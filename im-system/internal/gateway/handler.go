// Package gateway 提供网关核心功能
package gateway

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"github.com/d60-lab/im-system/internal/model"
	"github.com/d60-lab/im-system/pkg/auth"
	"github.com/d60-lab/im-system/pkg/util"
)

// MessageSaver 消息保存接口
type MessageSaver interface {
	SaveMessage(ctx context.Context, msg *model.Message) error
}

// WebSocketHandler WebSocket处理器
type WebSocketHandler struct {
	config       *HandlerConfig
	upgrader     websocket.Upgrader
	connMgr      *ConnectionManager
	dispatcher   MessageDispatcher
	jwtManager   *auth.JWTManager
	deduper      *MessageDeduper
	messageSaver MessageSaver

	// 消息处理回调
	onMessage func(ctx context.Context, conn *Connection, msg *model.Message) error
}

// HandlerConfig 处理器配置
type HandlerConfig struct {
	NodeID           string
	MaxMessageSize   int64
	PingInterval     time.Duration
	PongTimeout      time.Duration
	WriteTimeout     time.Duration
	ReadTimeout      time.Duration
	HandshakeTimeout time.Duration
	AllowOrigins     []string
}

// DefaultHandlerConfig 默认配置
func DefaultHandlerConfig() *HandlerConfig {
	return &HandlerConfig{
		NodeID:           "node1",
		MaxMessageSize:   65536, // 64KB
		PingInterval:     30 * time.Second,
		PongTimeout:      60 * time.Second,
		WriteTimeout:     10 * time.Second,
		ReadTimeout:      60 * time.Second,
		HandshakeTimeout: 10 * time.Second,
		AllowOrigins:     []string{"*"},
	}
}

// NewWebSocketHandler 创建WebSocket处理器
func NewWebSocketHandler(
	config *HandlerConfig,
	connMgr *ConnectionManager,
	dispatcher MessageDispatcher,
	jwtManager *auth.JWTManager,
	messageSaver MessageSaver,
) *WebSocketHandler {
	if config == nil {
		config = DefaultHandlerConfig()
	}

	h := &WebSocketHandler{
		config:       config,
		connMgr:      connMgr,
		dispatcher:   dispatcher,
		jwtManager:   jwtManager,
		deduper:      NewMessageDeduper(10000),
		messageSaver: messageSaver,
	}

	h.upgrader = websocket.Upgrader{
		ReadBufferSize:   1024,
		WriteBufferSize:  1024,
		HandshakeTimeout: config.HandshakeTimeout,
		CheckOrigin:      h.checkOrigin,
	}

	return h
}

// checkOrigin 检查请求来源
func (h *WebSocketHandler) checkOrigin(r *http.Request) bool {
	if len(h.config.AllowOrigins) == 0 {
		return true
	}

	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}

	for _, allowed := range h.config.AllowOrigins {
		if allowed == "*" || allowed == origin {
			return true
		}
	}

	return false
}

// SetOnMessage 设置消息处理回调
func (h *WebSocketHandler) SetOnMessage(fn func(ctx context.Context, conn *Connection, msg *model.Message) error) {
	h.onMessage = fn
}

// RegisterRoutes 注册路由
func (h *WebSocketHandler) RegisterRoutes(r *gin.Engine) {
	r.GET("/ws", h.HandleWebSocket)
	r.GET("/health", h.HandleHealth)
	r.GET("/stats", h.HandleStats)
}

// HandleWebSocket 处理WebSocket连接
func (h *WebSocketHandler) HandleWebSocket(c *gin.Context) {
	// 从查询参数或Header获取token
	token := c.Query("token")
	if token == "" {
		token = c.GetHeader("Authorization")
		if strings.HasPrefix(token, "Bearer ") {
			token = token[7:]
		}
	}

	// 验证token
	claims, err := h.jwtManager.ParseToken(token)
	if err != nil {
		log.Printf("Invalid token: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return
	}

	userID := claims.UserID
	platform := c.Query("platform")
	deviceID := c.Query("device_id")

	// 升级为WebSocket连接
	wsConn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	// 创建连接对象
	connID := util.GenerateUUID()
	conn := NewConnection(connID, userID, h.config.NodeID, wsConn, nil)
	conn.SetPlatform(platform)
	conn.SetDeviceID(deviceID)

	// 注册连接
	h.connMgr.Register(conn)

	log.Printf("User %s connected (connID: %s, platform: %s)", userID, connID, platform)

	// 启动读写协程
	go h.writePump(conn)
	go h.readPump(conn)
}

// readPump 读取消息协程
func (h *WebSocketHandler) readPump(conn *Connection) {
	defer func() {
		h.connMgr.Unregister(conn)
		conn.Close()
		log.Printf("User %s disconnected (connID: %s)", conn.UserID, conn.ID)
	}()

	// 设置读取限制
	conn.Conn.SetReadLimit(h.config.MaxMessageSize)
	conn.Conn.SetReadDeadline(time.Now().Add(h.config.PongTimeout))

	// 设置Pong处理器
	conn.Conn.SetPongHandler(func(string) error {
		conn.Conn.SetReadDeadline(time.Now().Add(h.config.PongTimeout))
		conn.UpdateLastActive()
		return nil
	})

	ctx := context.Background()

	for {
		_, data, err := conn.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket read error: %v", err)
			}
			break
		}

		// 重置读取超时（每收到消息都重置，不仅仅是 Pong）
		conn.Conn.SetReadDeadline(time.Now().Add(h.config.PongTimeout))
		conn.UpdateLastActive()

		// 解析消息
		var msg model.Message
		if err := json.Unmarshal(data, &msg); err != nil {
			log.Printf("Unmarshal message error: %v", err)
			h.sendError(conn, "invalid_message", "Invalid message format")
			continue
		}

		// 处理消息
		if err := h.handleMessage(ctx, conn, &msg); err != nil {
			log.Printf("Handle message error: %v", err)
			h.sendError(conn, "handle_error", err.Error())
		}
	}
}

// writePump 发送消息协程
func (h *WebSocketHandler) writePump(conn *Connection) {
	ticker := time.NewTicker(h.config.PingInterval)
	defer func() {
		ticker.Stop()
		conn.Close()
	}()

	for {
		select {
		case data, ok := <-conn.Send:
			if !ok {
				// 通道已关闭
				conn.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			conn.Conn.SetWriteDeadline(time.Now().Add(h.config.WriteTimeout))

			if err := conn.Conn.WriteMessage(websocket.TextMessage, data); err != nil {
				log.Printf("WebSocket write error: %v", err)
				return
			}

		case <-ticker.C:
			conn.Conn.SetWriteDeadline(time.Now().Add(h.config.WriteTimeout))
			if err := conn.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}

		case <-conn.Done():
			return
		}
	}
}

// handleMessage 处理接收到的消息
func (h *WebSocketHandler) handleMessage(ctx context.Context, conn *Connection, msg *model.Message) error {
	// 设置消息来源
	msg.From = conn.UserID
	msg.Timestamp = time.Now().UnixMilli()

	// 生成消息ID（如果没有）
	if msg.MessageID == "" {
		msg.MessageID = util.GenerateMessageID()
	}

	// 消息去重
	if h.deduper.IsDuplicate(msg.MessageID) {
		log.Printf("Duplicate message: %s", msg.MessageID)
		return nil
	}

	// 根据消息类型处理
	switch msg.Type {
	case model.MsgHeartbeat:
		return h.handleHeartbeat(ctx, conn, msg)

	case model.MsgSingleChat, model.MsgText:
		return h.handleSingleChat(ctx, conn, msg)

	case model.MsgGroupChat:
		return h.handleGroupChat(ctx, conn, msg)

	case model.MsgAck:
		return h.handleAck(ctx, conn, msg)

	case model.MsgReadReceipt:
		return h.handleReadReceipt(ctx, conn, msg)

	case model.MsgTyping:
		return h.handleTyping(ctx, conn, msg)

	default:
		// 自定义消息处理
		if h.onMessage != nil {
			return h.onMessage(ctx, conn, msg)
		}
		return nil
	}
}

// handleHeartbeat 处理心跳消息
func (h *WebSocketHandler) handleHeartbeat(ctx context.Context, conn *Connection, msg *model.Message) error {
	// 返回心跳响应
	response := model.NewHeartbeatMessage()
	return conn.SendJSON(response)
}

// handleSingleChat 处理单聊消息
func (h *WebSocketHandler) handleSingleChat(ctx context.Context, conn *Connection, msg *model.Message) error {
	// 设置会话ID
	msg.ConversationID = model.GetSingleChatConversationID(msg.From, msg.To)

	// 保存消息到数据库
	if h.messageSaver != nil {
		if err := h.messageSaver.SaveMessage(ctx, msg); err != nil {
			log.Printf("Save message error: %v", err)
		}
	}

	// 发送ACK给发送者
	ack := model.NewAckMessage(msg.MessageID, 0)
	conn.SendJSON(ack)

	// 分发消息给接收者
	return h.dispatcher.DispatchToUsers(ctx, []string{msg.To}, msg)
}

// handleGroupChat 处理群聊消息
func (h *WebSocketHandler) handleGroupChat(ctx context.Context, conn *Connection, msg *model.Message) error {
	// 设置会话ID
	msg.ConversationID = model.GetGroupChatConversationID(msg.To)

	// 保存消息到数据库
	if h.messageSaver != nil {
		if err := h.messageSaver.SaveMessage(ctx, msg); err != nil {
			log.Printf("Save group message error: %v", err)
		}
	}

	// 发送ACK给发送者
	ack := model.NewAckMessage(msg.MessageID, 0)
	conn.SendJSON(ack)

	// 分发消息给群成员（排除发送者）
	return h.dispatcher.DispatchToConversation(ctx, msg.ConversationID, msg, msg.From)
}

// handleAck 处理消息确认
func (h *WebSocketHandler) handleAck(ctx context.Context, conn *Connection, msg *model.Message) error {
	// 这里可以实现消息确认逻辑
	// 例如：更新消息状态、停止重发等
	log.Printf("ACK received from %s for message %v", conn.UserID, msg.Content)
	return nil
}

// handleReadReceipt 处理已读回执
func (h *WebSocketHandler) handleReadReceipt(ctx context.Context, conn *Connection, msg *model.Message) error {
	// 转发已读回执给消息发送者
	content, ok := msg.Content.(*model.ReadReceiptContent)
	if !ok {
		// 尝试从map解析
		if contentMap, ok := msg.Content.(map[string]interface{}); ok {
			content = &model.ReadReceiptContent{
				ConversationID: getString(contentMap, "conversation_id"),
				LastReadSeq:    getInt64(contentMap, "last_read_seq"),
			}
		} else {
			return nil
		}
	}

	// 如果是单聊，发送给对方
	if strings.HasPrefix(content.ConversationID, "single_") {
		parts := content.ConversationID[7:]
		for i := len(parts) - 1; i >= 0; i-- {
			if parts[i] == '_' {
				user1, user2 := parts[:i], parts[i+1:]
				targetUser := user1
				if user1 == conn.UserID {
					targetUser = user2
				}
				return h.dispatcher.DispatchToUsers(ctx, []string{targetUser}, msg)
			}
		}
	}

	return nil
}

// handleTyping 处理正在输入
func (h *WebSocketHandler) handleTyping(ctx context.Context, conn *Connection, msg *model.Message) error {
	// 转发输入状态给对方
	if msg.To != "" {
		return h.dispatcher.DispatchToUsers(ctx, []string{msg.To}, msg)
	}
	return nil
}

// sendError 发送错误消息
func (h *WebSocketHandler) sendError(conn *Connection, code, message string) {
	errMsg := &model.Message{
		Type: model.MsgSystem,
		Content: map[string]string{
			"error":   code,
			"message": message,
		},
		Timestamp: time.Now().UnixMilli(),
	}
	conn.SendJSON(errMsg)
}

// HandleHealth 健康检查接口
func (h *WebSocketHandler) HandleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"node_id": h.config.NodeID,
		"time":    time.Now().Format(time.RFC3339),
	})
}

// HandleStats 统计信息接口
func (h *WebSocketHandler) HandleStats(c *gin.Context) {
	stats := h.connMgr.GetStats()
	c.JSON(http.StatusOK, stats)
}

// MessageDeduper 消息去重器
type MessageDeduper struct {
	cache map[string]int64
	mu    sync.RWMutex
	size  int
}

// NewMessageDeduper 创建消息去重器
func NewMessageDeduper(size int) *MessageDeduper {
	d := &MessageDeduper{
		cache: make(map[string]int64),
		size:  size,
	}

	// 启动清理协程
	go d.cleanup()

	return d
}

// IsDuplicate 检查消息是否重复
func (d *MessageDeduper) IsDuplicate(messageID string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	if _, exists := d.cache[messageID]; exists {
		return true
	}

	// 如果缓存已满，清理旧数据
	if len(d.cache) >= d.size {
		d.evictOldest()
	}

	d.cache[messageID] = time.Now().Unix()
	return false
}

// evictOldest 清理最旧的数据
func (d *MessageDeduper) evictOldest() {
	oldest := time.Now().Unix()
	var oldestKey string

	for k, v := range d.cache {
		if v < oldest {
			oldest = v
			oldestKey = k
		}
	}

	if oldestKey != "" {
		delete(d.cache, oldestKey)
	}
}

// cleanup 定期清理过期数据
func (d *MessageDeduper) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		d.mu.Lock()
		now := time.Now().Unix()
		expireTime := int64(300) // 5分钟过期

		for k, v := range d.cache {
			if now-v > expireTime {
				delete(d.cache, k)
			}
		}
		d.mu.Unlock()
	}
}

// 辅助函数：从map中获取字符串
func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// 辅助函数：从map中获取int64
func getInt64(m map[string]interface{}, key string) int64 {
	if v, ok := m[key]; ok {
		switch n := v.(type) {
		case float64:
			return int64(n)
		case int64:
			return n
		case int:
			return int64(n)
		}
	}
	return 0
}
