// Package gateway 提供网关核心功能
package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/d60-lab/im-system/internal/model"
	"github.com/go-redis/redis/v8"
)

// MessageDispatcher 消息分发器接口
type MessageDispatcher interface {
	// DispatchToUsers 分发消息给指定用户
	DispatchToUsers(ctx context.Context, userIDs []string, msg *model.Message) error

	// DispatchToConversation 分发消息到会话（根据会话类型分发）
	DispatchToConversation(ctx context.Context, conversationID string, msg *model.Message, excludeUserID string) error

	// SubscribeNodeMessages 订阅本节点的消息
	SubscribeNodeMessages(ctx context.Context) error

	// RegisterConnection 注册用户连接
	RegisterConnection(userID string, conn Conn) error

	// UnregisterConnection 注销用户连接
	UnregisterConnection(userID string) error

	// IsUserOnline 检查用户是否在线
	IsUserOnline(ctx context.Context, userID string) (bool, error)

	// GetUserNode 获取用户所在节点
	GetUserNode(ctx context.Context, userID string) (string, error)

	// Close 关闭分发器
	Close() error
}

// Conn 连接接口（用于消息分发）
type Conn interface {
	// SendData 发送消息
	SendData(data []byte) error
	// CloseConn 关闭连接
	CloseConn() error
	// GetUserID 获取用户ID
	GetUserID() string
}

// GroupMemberGetter 群成员获取接口
type GroupMemberGetter interface {
	// GetGroupMemberIDs 获取群成员ID列表
	GetGroupMemberIDs(ctx context.Context, groupID string) ([]string, error)
}

// OfflineMessageSaver 离线消息保存接口
type OfflineMessageSaver interface {
	// SaveOfflineMessage 保存离线消息
	SaveOfflineMessage(ctx context.Context, userID string, msg *model.Message) error
}

// DispatcherConfig 分发器配置
type DispatcherConfig struct {
	NodeID                 string        // 节点ID
	OnlineKeyExpire        time.Duration // 在线状态过期时间
	PublishChannelPrefix   string        // 发布频道前缀
	SubscribeChannelPrefix string        // 订阅频道前缀
}

// DefaultDispatcherConfig 默认配置
func DefaultDispatcherConfig() *DispatcherConfig {
	return &DispatcherConfig{
		NodeID:                 "node1",
		OnlineKeyExpire:        time.Hour,
		PublishChannelPrefix:   "im:node:",
		SubscribeChannelPrefix: "im:node:",
	}
}

// messageDispatcherImpl 消息分发器实现
type messageDispatcherImpl struct {
	config            *DispatcherConfig
	redis             *redis.Client
	localConns        map[string]Conn // 本节点的连接 userID -> Conn
	connMutex         sync.RWMutex
	groupMemberGetter GroupMemberGetter
	offlineSaver      OfflineMessageSaver
	pubsub            *redis.PubSub
	stopChan          chan struct{}
	wg                sync.WaitGroup
}

// NewMessageDispatcher 创建消息分发器
func NewMessageDispatcher(
	config *DispatcherConfig,
	redisClient *redis.Client,
	groupMemberGetter GroupMemberGetter,
	offlineSaver OfflineMessageSaver,
) MessageDispatcher {
	if config == nil {
		config = DefaultDispatcherConfig()
	}

	return &messageDispatcherImpl{
		config:            config,
		redis:             redisClient,
		localConns:        make(map[string]Conn),
		groupMemberGetter: groupMemberGetter,
		offlineSaver:      offlineSaver,
		stopChan:          make(chan struct{}),
	}
}

// RegisterConnection 注册用户连接
func (d *messageDispatcherImpl) RegisterConnection(userID string, conn Conn) error {
	d.connMutex.Lock()
	d.localConns[userID] = conn
	d.connMutex.Unlock()

	// 在Redis中记录用户在线状态
	ctx := context.Background()
	onlineKey := fmt.Sprintf("online:%s", userID)
	nodeInfo := fmt.Sprintf("%s:%d", d.config.NodeID, time.Now().Unix())

	return d.redis.SetEX(ctx, onlineKey, nodeInfo, d.config.OnlineKeyExpire).Err()
}

// UnregisterConnection 注销用户连接
func (d *messageDispatcherImpl) UnregisterConnection(userID string) error {
	d.connMutex.Lock()
	delete(d.localConns, userID)
	d.connMutex.Unlock()

	// 从Redis中删除用户在线状态
	ctx := context.Background()
	onlineKey := fmt.Sprintf("online:%s", userID)

	return d.redis.Del(ctx, onlineKey).Err()
}

// IsUserOnline 检查用户是否在线
func (d *messageDispatcherImpl) IsUserOnline(ctx context.Context, userID string) (bool, error) {
	onlineKey := fmt.Sprintf("online:%s", userID)
	exists, err := d.redis.Exists(ctx, onlineKey).Result()
	if err != nil {
		return false, err
	}
	return exists > 0, nil
}

// GetUserNode 获取用户所在节点
func (d *messageDispatcherImpl) GetUserNode(ctx context.Context, userID string) (string, error) {
	onlineKey := fmt.Sprintf("online:%s", userID)
	val, err := d.redis.Get(ctx, onlineKey).Result()
	if err == redis.Nil {
		return "", nil // 用户不在线
	}
	if err != nil {
		return "", err
	}

	// 解析节点信息 (格式: nodeID:timestamp)
	var nodeID string
	fmt.Sscanf(val, "%[^:]", &nodeID)
	return nodeID, nil
}

// DispatchToUsers 分发消息给指定用户
func (d *messageDispatcherImpl) DispatchToUsers(ctx context.Context, userIDs []string, msg *model.Message) error {
	if len(userIDs) == 0 {
		return nil
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal message error: %w", err)
	}

	var wg sync.WaitGroup
	errChan := make(chan error, len(userIDs))

	for _, userID := range userIDs {
		wg.Add(1)
		go func(uid string) {
			defer wg.Done()

			// 尝试本地推送
			if d.pushToLocalUser(uid, data) {
				return
			}

			// 检查用户是否在其他节点
			nodeID, err := d.GetUserNode(ctx, uid)
			if err != nil {
				errChan <- fmt.Errorf("get user node error for %s: %w", uid, err)
				return
			}

			if nodeID != "" && nodeID != d.config.NodeID {
				// 用户在其他节点，通过Redis发布消息
				if err := d.publishToNode(ctx, nodeID, uid, msg); err != nil {
					errChan <- fmt.Errorf("publish to node error: %w", err)
				}
			} else {
				// 用户不在线，保存离线消息
				if d.offlineSaver != nil {
					if err := d.offlineSaver.SaveOfflineMessage(ctx, uid, msg); err != nil {
						errChan <- fmt.Errorf("save offline message error: %w", err)
					}
				}
			}
		}(userID)
	}

	wg.Wait()
	close(errChan)

	// 收集错误
	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return fmt.Errorf("dispatch errors: %v", errs)
	}

	return nil
}

// DispatchToConversation 分发消息到会话
func (d *messageDispatcherImpl) DispatchToConversation(ctx context.Context, conversationID string, msg *model.Message, excludeUserID string) error {
	var targetUserIDs []string

	// 根据会话类型获取目标用户
	// 支持两种格式: "group:group_xxx" 或 "group_xxx"
	var groupID string
	if len(conversationID) > 6 && conversationID[:6] == "group:" {
		// 格式: group:group_xxx
		groupID = conversationID[6:]
	} else if len(conversationID) > 6 && conversationID[:6] == "group_" {
		// 格式: group_xxx
		groupID = conversationID
	}

	if groupID != "" {
		// 群聊会话
		if d.groupMemberGetter != nil {
			memberIDs, err := d.groupMemberGetter.GetGroupMemberIDs(ctx, groupID)
			if err != nil {
				return fmt.Errorf("get group members error: %w", err)
			}
			targetUserIDs = memberIDs
		} else {
			// 从Redis获取群成员
			groupKey := fmt.Sprintf("group:members:%s", groupID)
			members, err := d.redis.SMembers(ctx, groupKey).Result()
			if err != nil {
				return fmt.Errorf("get group members from redis error: %w", err)
			}
			targetUserIDs = members
		}
	}

	if len(conversationID) > 7 && conversationID[:7] == "single_" {
		// 单聊会话，提取两个用户ID
		parts := conversationID[7:] // 去掉 "single_" 前缀
		// 格式: userID1_userID2
		for i := len(parts) - 1; i >= 0; i-- {
			if parts[i] == '_' {
				targetUserIDs = []string{parts[:i], parts[i+1:]}
				break
			}
		}
	}

	// 排除发送者
	if excludeUserID != "" {
		filtered := make([]string, 0, len(targetUserIDs))
		for _, uid := range targetUserIDs {
			if uid != excludeUserID {
				filtered = append(filtered, uid)
			}
		}
		targetUserIDs = filtered
	}

	return d.DispatchToUsers(ctx, targetUserIDs, msg)
}

// pushToLocalUser 推送消息给本地用户
func (d *messageDispatcherImpl) pushToLocalUser(userID string, data []byte) bool {
	d.connMutex.RLock()
	conn, ok := d.localConns[userID]
	d.connMutex.RUnlock()

	if !ok {
		return false
	}

	if err := conn.SendData(data); err != nil {
		log.Printf("send to user %s error: %v", userID, err)
		return false
	}

	return true
}

// publishToNode 发布消息到指定节点
func (d *messageDispatcherImpl) publishToNode(ctx context.Context, nodeID, targetUserID string, msg *model.Message) error {
	channel := fmt.Sprintf("%s%s", d.config.PublishChannelPrefix, nodeID)

	routeMsg := &RouteMessage{
		TargetUsers: []string{targetUserID},
		Message:     msg,
	}

	data, err := json.Marshal(routeMsg)
	if err != nil {
		return err
	}

	return d.redis.Publish(ctx, channel, data).Err()
}

// SubscribeNodeMessages 订阅本节点的消息
func (d *messageDispatcherImpl) SubscribeNodeMessages(ctx context.Context) error {
	channel := fmt.Sprintf("%s%s", d.config.SubscribeChannelPrefix, d.config.NodeID)

	d.pubsub = d.redis.Subscribe(ctx, channel)

	// 等待订阅确认
	_, err := d.pubsub.Receive(ctx)
	if err != nil {
		return fmt.Errorf("subscribe error: %w", err)
	}

	log.Printf("Subscribed to channel: %s", channel)

	// 启动消息处理协程
	d.wg.Add(1)
	go d.handleSubscribedMessages(ctx)

	return nil
}

// handleSubscribedMessages 处理订阅的消息
func (d *messageDispatcherImpl) handleSubscribedMessages(ctx context.Context) {
	defer d.wg.Done()

	ch := d.pubsub.Channel()

	for {
		select {
		case <-d.stopChan:
			return
		case <-ctx.Done():
			return
		case redisMsg, ok := <-ch:
			if !ok {
				return
			}

			var routeMsg RouteMessage
			if err := json.Unmarshal([]byte(redisMsg.Payload), &routeMsg); err != nil {
				log.Printf("unmarshal route message error: %v", err)
				continue
			}

			// 处理路由消息
			d.handleRouteMessage(&routeMsg)
		}
	}
}

// handleRouteMessage 处理路由消息
func (d *messageDispatcherImpl) handleRouteMessage(routeMsg *RouteMessage) {
	data, err := json.Marshal(routeMsg.Message)
	if err != nil {
		log.Printf("marshal message error: %v", err)
		return
	}

	for _, userID := range routeMsg.TargetUsers {
		if !d.pushToLocalUser(userID, data) {
			log.Printf("user %s not found on this node", userID)
		}
	}
}

// Close 关闭分发器
func (d *messageDispatcherImpl) Close() error {
	close(d.stopChan)

	if d.pubsub != nil {
		if err := d.pubsub.Close(); err != nil {
			return err
		}
	}

	d.wg.Wait()

	// 清理所有本地连接
	d.connMutex.Lock()
	for userID, conn := range d.localConns {
		conn.CloseConn()
		delete(d.localConns, userID)
	}
	d.connMutex.Unlock()

	return nil
}

// RouteMessage 路由消息
type RouteMessage struct {
	TargetUsers []string       `json:"target_users"`
	Message     *model.Message `json:"message"`
}

// RefreshOnlineStatus 刷新用户在线状态
func (d *messageDispatcherImpl) RefreshOnlineStatus(ctx context.Context, userID string) error {
	onlineKey := fmt.Sprintf("online:%s", userID)
	return d.redis.Expire(ctx, onlineKey, d.config.OnlineKeyExpire).Err()
}

// GetOnlineUsers 获取在线用户列表
func (d *messageDispatcherImpl) GetOnlineUsers() []string {
	d.connMutex.RLock()
	defer d.connMutex.RUnlock()

	users := make([]string, 0, len(d.localConns))
	for userID := range d.localConns {
		users = append(users, userID)
	}
	return users
}

// GetConnectionCount 获取本节点连接数
func (d *messageDispatcherImpl) GetConnectionCount() int {
	d.connMutex.RLock()
	defer d.connMutex.RUnlock()
	return len(d.localConns)
}

// BroadcastToAll 广播消息给所有本地用户
func (d *messageDispatcherImpl) BroadcastToAll(msg *model.Message) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	d.connMutex.RLock()
	defer d.connMutex.RUnlock()

	for userID, conn := range d.localConns {
		if err := conn.SendData(data); err != nil {
			log.Printf("broadcast to user %s error: %v", userID, err)
		}
	}

	return nil
}

// BroadcastToAllNodes 广播消息给所有节点的所有用户
func (d *messageDispatcherImpl) BroadcastToAllNodes(ctx context.Context, msg *model.Message) error {
	// 首先广播给本地用户
	if err := d.BroadcastToAll(msg); err != nil {
		log.Printf("broadcast to local users error: %v", err)
	}

	// 获取所有节点并发布广播消息
	// 这里假设节点列表存储在Redis的Set中
	nodesKey := "im:nodes"
	nodes, err := d.redis.SMembers(ctx, nodesKey).Result()
	if err != nil {
		return fmt.Errorf("get nodes error: %w", err)
	}

	routeMsg := &RouteMessage{
		TargetUsers: []string{"*"}, // 特殊标记，表示广播
		Message:     msg,
	}

	data, err := json.Marshal(routeMsg)
	if err != nil {
		return err
	}

	for _, nodeID := range nodes {
		if nodeID == d.config.NodeID {
			continue // 跳过本节点
		}

		channel := fmt.Sprintf("%s%s", d.config.PublishChannelPrefix, nodeID)
		if err := d.redis.Publish(ctx, channel, data).Err(); err != nil {
			log.Printf("publish to node %s error: %v", nodeID, err)
		}
	}

	return nil
}

// RegisterNode 注册节点
func (d *messageDispatcherImpl) RegisterNode(ctx context.Context) error {
	nodesKey := "im:nodes"
	return d.redis.SAdd(ctx, nodesKey, d.config.NodeID).Err()
}

// UnregisterNode 注销节点
func (d *messageDispatcherImpl) UnregisterNode(ctx context.Context) error {
	nodesKey := "im:nodes"
	return d.redis.SRem(ctx, nodesKey, d.config.NodeID).Err()
}

// SendDirectMessage 发送直连消息（跳过Redis路由，仅用于本地用户）
func (d *messageDispatcherImpl) SendDirectMessage(userID string, msg *model.Message) error {
	msgData, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	if !d.pushToLocalUser(userID, msgData) {
		return fmt.Errorf("user %s not connected to this node", userID)
	}

	return nil
}
