// Package gateway 提供IM网关核心功能
package gateway

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/d60-lab/im-system/internal/model"
)

// ConnectionState 连接状态
type ConnectionState int

const (
	StateConnecting ConnectionState = iota // 连接中
	StateConnected                         // 已连接
	StateClosing                           // 关闭中
	StateClosed                            // 已关闭
)

// Connection WebSocket连接封装
type Connection struct {
	ID         string          // 连接ID
	UserID     string          // 用户ID
	Conn       *websocket.Conn // WebSocket连接
	Send       chan []byte     // 发送消息通道
	NodeID     string          // 所在节点ID
	Platform   string          // 平台: web, ios, android
	DeviceID   string          // 设备ID
	State      ConnectionState // 连接状态
	LastActive time.Time       // 最后活跃时间
	CreatedAt  time.Time       // 创建时间

	mu       sync.RWMutex
	closed   bool
	closedCh chan struct{}
}

// ConnectionConfig 连接配置
type ConnectionConfig struct {
	ReadBufferSize   int
	WriteBufferSize  int
	MaxMessageSize   int64
	PingInterval     time.Duration
	PongTimeout      time.Duration
	WriteTimeout     time.Duration
	ReadTimeout      time.Duration
	SendChannelSize  int
	HandshakeTimeout time.Duration
}

// DefaultConnectionConfig 默认连接配置
var DefaultConnectionConfig = ConnectionConfig{
	ReadBufferSize:   1024,
	WriteBufferSize:  1024,
	MaxMessageSize:   65536, // 64KB
	PingInterval:     30 * time.Second,
	PongTimeout:      60 * time.Second,
	WriteTimeout:     10 * time.Second,
	ReadTimeout:      60 * time.Second,
	SendChannelSize:  256,
	HandshakeTimeout: 10 * time.Second,
}

// NewConnection 创建新连接
func NewConnection(id, userID, nodeID string, conn *websocket.Conn, config *ConnectionConfig) *Connection {
	if config == nil {
		config = &DefaultConnectionConfig
	}

	return &Connection{
		ID:         id,
		UserID:     userID,
		Conn:       conn,
		Send:       make(chan []byte, config.SendChannelSize),
		NodeID:     nodeID,
		State:      StateConnected,
		LastActive: time.Now(),
		CreatedAt:  time.Now(),
		closedCh:   make(chan struct{}),
	}
}

// Close 关闭连接
func (c *Connection) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	c.State = StateClosed
	close(c.closedCh)
	close(c.Send)
	c.mu.Unlock()

	return c.Conn.Close()
}

// IsClosed 检查连接是否已关闭
func (c *Connection) IsClosed() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.closed
}

// SendMessage 发送消息
func (c *Connection) SendMessage(data []byte) error {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return ErrConnectionClosed
	}
	c.mu.RUnlock()

	select {
	case c.Send <- data:
		return nil
	default:
		return ErrSendBufferFull
	}
}

// SendJSON 发送JSON消息
func (c *Connection) SendJSON(v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return c.SendMessage(data)
}

// UpdateLastActive 更新最后活跃时间
func (c *Connection) UpdateLastActive() {
	c.mu.Lock()
	c.LastActive = time.Now()
	c.mu.Unlock()
}

// GetLastActive 获取最后活跃时间
func (c *Connection) GetLastActive() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.LastActive
}

// SetPlatform 设置平台信息
func (c *Connection) SetPlatform(platform string) {
	c.mu.Lock()
	c.Platform = platform
	c.mu.Unlock()
}

// SetDeviceID 设置设备ID
func (c *Connection) SetDeviceID(deviceID string) {
	c.mu.Lock()
	c.DeviceID = deviceID
	c.mu.Unlock()
}

// Done 返回关闭信号通道
func (c *Connection) Done() <-chan struct{} {
	return c.closedCh
}

// ConnectionManager 连接管理器
type ConnectionManager struct {
	connections sync.Map // map[userID]*Connection
	connByID    sync.Map // map[connID]*Connection
	nodeID      string
	config      *ConnectionConfig
	mu          sync.RWMutex

	// 连接统计
	totalConnections int64
	activeUsers      int64

	// 回调函数
	onConnect    func(*Connection)
	onDisconnect func(*Connection)
	onMessage    func(*Connection, []byte)
}

// NewConnectionManager 创建连接管理器
func NewConnectionManager(nodeID string, config *ConnectionConfig) *ConnectionManager {
	if config == nil {
		config = &DefaultConnectionConfig
	}
	return &ConnectionManager{
		nodeID: nodeID,
		config: config,
	}
}

// Register 注册连接
func (m *ConnectionManager) Register(conn *Connection) {
	// 检查是否已存在该用户的连接（踢出旧连接）
	if old, loaded := m.connections.LoadAndDelete(conn.UserID); loaded {
		oldConn := old.(*Connection)
		// 发送踢出消息
		kickMsg := &model.Message{
			Type: model.MsgKickout,
			Content: &model.KickoutContent{
				Reason:   "您的账号在其他设备登录",
				DeviceID: conn.DeviceID,
			},
			Timestamp: time.Now().UnixMilli(),
		}
		oldConn.SendJSON(kickMsg)
		oldConn.Close()
		m.connByID.Delete(oldConn.ID)
	}

	// 注册新连接
	m.connections.Store(conn.UserID, conn)
	m.connByID.Store(conn.ID, conn)

	// 更新统计
	m.mu.Lock()
	m.totalConnections++
	m.activeUsers = m.countActiveUsers()
	m.mu.Unlock()

	// 触发回调
	if m.onConnect != nil {
		go m.onConnect(conn)
	}
}

// Unregister 注销连接
func (m *ConnectionManager) Unregister(conn *Connection) {
	// 只有当前存储的连接ID匹配时才删除
	if current, ok := m.connections.Load(conn.UserID); ok {
		if current.(*Connection).ID == conn.ID {
			m.connections.Delete(conn.UserID)
		}
	}
	m.connByID.Delete(conn.ID)

	// 更新统计
	m.mu.Lock()
	m.activeUsers = m.countActiveUsers()
	m.mu.Unlock()

	// 触发回调
	if m.onDisconnect != nil {
		go m.onDisconnect(conn)
	}
}

// GetConnection 根据用户ID获取连接
func (m *ConnectionManager) GetConnection(userID string) (*Connection, bool) {
	if conn, ok := m.connections.Load(userID); ok {
		return conn.(*Connection), true
	}
	return nil, false
}

// GetConnectionByID 根据连接ID获取连接
func (m *ConnectionManager) GetConnectionByID(connID string) (*Connection, bool) {
	if conn, ok := m.connByID.Load(connID); ok {
		return conn.(*Connection), true
	}
	return nil, false
}

// GetConnections 获取多个用户的连接
func (m *ConnectionManager) GetConnections(userIDs []string) []*Connection {
	conns := make([]*Connection, 0, len(userIDs))
	for _, userID := range userIDs {
		if conn, ok := m.GetConnection(userID); ok {
			conns = append(conns, conn)
		}
	}
	return conns
}

// GetAllConnections 获取所有连接
func (m *ConnectionManager) GetAllConnections() []*Connection {
	var conns []*Connection
	m.connections.Range(func(key, value interface{}) bool {
		conns = append(conns, value.(*Connection))
		return true
	})
	return conns
}

// GetOnlineUserIDs 获取所有在线用户ID
func (m *ConnectionManager) GetOnlineUserIDs() []string {
	var userIDs []string
	m.connections.Range(func(key, value interface{}) bool {
		userIDs = append(userIDs, key.(string))
		return true
	})
	return userIDs
}

// IsOnline 检查用户是否在线
func (m *ConnectionManager) IsOnline(userID string) bool {
	_, ok := m.connections.Load(userID)
	return ok
}

// Broadcast 广播消息给所有连接
func (m *ConnectionManager) Broadcast(data []byte) {
	m.connections.Range(func(key, value interface{}) bool {
		conn := value.(*Connection)
		conn.SendMessage(data)
		return true
	})
}

// BroadcastToUsers 广播消息给指定用户
func (m *ConnectionManager) BroadcastToUsers(userIDs []string, data []byte) {
	for _, userID := range userIDs {
		if conn, ok := m.GetConnection(userID); ok {
			conn.SendMessage(data)
		}
	}
}

// BroadcastJSON 广播JSON消息给所有连接
func (m *ConnectionManager) BroadcastJSON(v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	m.Broadcast(data)
	return nil
}

// Count 获取当前连接数
func (m *ConnectionManager) Count() int {
	count := 0
	m.connections.Range(func(key, value interface{}) bool {
		count++
		return true
	})
	return count
}

// countActiveUsers 计算活跃用户数（内部使用）
func (m *ConnectionManager) countActiveUsers() int64 {
	count := int64(0)
	m.connections.Range(func(key, value interface{}) bool {
		count++
		return true
	})
	return count
}

// GetStats 获取连接统计信息
func (m *ConnectionManager) GetStats() ConnectionStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return ConnectionStats{
		NodeID:           m.nodeID,
		TotalConnections: m.totalConnections,
		ActiveUsers:      m.activeUsers,
		CurrentCount:     int64(m.Count()),
	}
}

// ConnectionStats 连接统计信息
type ConnectionStats struct {
	NodeID           string `json:"node_id"`
	TotalConnections int64  `json:"total_connections"` // 历史总连接数
	ActiveUsers      int64  `json:"active_users"`      // 当前活跃用户数
	CurrentCount     int64  `json:"current_count"`     // 当前连接数
}

// SetOnConnect 设置连接建立回调
func (m *ConnectionManager) SetOnConnect(fn func(*Connection)) {
	m.onConnect = fn
}

// SetOnDisconnect 设置连接断开回调
func (m *ConnectionManager) SetOnDisconnect(fn func(*Connection)) {
	m.onDisconnect = fn
}

// SetOnMessage 设置消息接收回调
func (m *ConnectionManager) SetOnMessage(fn func(*Connection, []byte)) {
	m.onMessage = fn
}

// CleanIdleConnections 清理空闲连接
func (m *ConnectionManager) CleanIdleConnections(idleTimeout time.Duration) int {
	cleaned := 0
	now := time.Now()

	m.connections.Range(func(key, value interface{}) bool {
		conn := value.(*Connection)
		if now.Sub(conn.GetLastActive()) > idleTimeout {
			m.Unregister(conn)
			conn.Close()
			cleaned++
		}
		return true
	})

	return cleaned
}

// StartHeartbeatChecker 启动心跳检查器
func (m *ConnectionManager) StartHeartbeatChecker(ctx context.Context, checkInterval, idleTimeout time.Duration) {
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.CleanIdleConnections(idleTimeout)
		}
	}
}

// CloseAll 关闭所有连接
func (m *ConnectionManager) CloseAll() {
	m.connections.Range(func(key, value interface{}) bool {
		conn := value.(*Connection)
		conn.Close()
		return true
	})
}

// 错误定义
var (
	ErrConnectionClosed = &ConnectionError{Code: 1001, Message: "connection closed"}
	ErrSendBufferFull   = &ConnectionError{Code: 1002, Message: "send buffer full"}
	ErrInvalidMessage   = &ConnectionError{Code: 1003, Message: "invalid message"}
	ErrUserNotOnline    = &ConnectionError{Code: 1004, Message: "user not online"}
)

// ConnectionError 连接错误
type ConnectionError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *ConnectionError) Error() string {
	return e.Message
}
