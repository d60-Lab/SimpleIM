# 从零构建即时通讯系统：Go + Vue3 实战指南

> 一个周末就能跑起来的 IM 系统，代码简洁、架构清晰、开箱即用。

![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)
![Vue](https://img.shields.io/badge/Vue-3.x-4FC08D?style=flat&logo=vue.js)
![WebSocket](https://img.shields.io/badge/WebSocket-Real--time-brightgreen)
![Redis](https://img.shields.io/badge/Redis-7.0-DC382D?style=flat&logo=redis)
![MySQL](https://img.shields.io/badge/MySQL-8.0-4479A1?style=flat&logo=mysql)

---

## 目录

- [前言](#-前言)
- [架构设计](#️-架构设计)
- [技术选型](#-技术选型)
- [核心实现](#-核心实现)
- [前后端交互协议](#-前后端交互协议)
- [API 接口](#-api-接口)
- [快速开始](#-快速开始)
- [安全设计](#-安全设计)
- [性能优化](#-性能优化)
- [踩坑指南](#-踩坑指南)
- [扩展路线](#️-扩展路线)
- [总结](#-总结)

---

## 📖 前言

IM（即时通讯）系统是现代应用的标配功能。市面上有很多成熟的云服务（如融云、环信、腾讯云 IM），但出于以下原因，自研 IM 仍是许多团队的选择：

| 考量因素 | 说明 |
|---------|------|
| 🔒 **数据安全** | 敏感数据不出企业内网 |
| 🎨 **定制需求** | 深度定制消息格式、业务流程 |
| 💰 **成本控制** | 高并发场景下自建更划算 |
| 📚 **技术积累** | 掌握核心技术，不受第三方限制 |

本文将带你从零实现一个**生产可用**的轻量级 IM 系统，特点是：

- ✅ **代码简洁** — 核心逻辑不到 2000 行 Go 代码
- ✅ **全栈完整** — 后端 Go + 前端 Vue3，开箱即用
- ✅ **功能丰富** — 单聊、群聊、文件传输、离线消息、心跳保活
- ✅ **易于扩展** — 清晰的分层架构，方便二次开发

---

## 🏗️ 架构设计

### 整体架构

```
┌─────────────────────────────────────────────────────────────────┐
│                           客户端                                 │
│                (Web / iOS / Android / Desktop)                  │
└────────────────────────────┬────────────────────────────────────┘
                             │
              ┌──────────────┴──────────────┐
              │                             │
              ▼                             ▼
        HTTP REST API              WebSocket 长连接
     (登录/注册/历史消息)            (实时消息收发)
              │                             │
              └──────────────┬──────────────┘
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│                      Gateway 接入层                              │
│  ┌───────────┐  ┌───────────┐  ┌───────────┐                   │
│  │  Node 1   │  │  Node 2   │  │  Node N   │  ← 无状态，水平扩展 │
│  │  (WS+HTTP)│  │  (WS+HTTP)│  │  (WS+HTTP)│                   │
│  └─────┬─────┘  └─────┬─────┘  └─────┬─────┘                   │
└────────┼──────────────┼──────────────┼──────────────────────────┘
         │              │              │
         └──────────────┼──────────────┘
                        │ Redis Pub/Sub (跨节点消息路由)
                        ▼
┌─────────────────────────────────────────────────────────────────┐
│                         存储层                                   │
│                                                                 │
│  ┌───────────┐  ┌───────────┐  ┌───────────┐  ┌───────────┐   │
│  │   MySQL   │  │  MongoDB  │  │   Redis   │  │   MinIO   │   │
│  │  用户/群组 │  │  消息存储  │  │ 在线状态   │  │  文件存储  │   │
│  │  离线消息  │  │  历史记录  │  │ 会话缓存   │  │  图片/视频 │   │
│  └───────────┘  └───────────┘  └───────────┘  └───────────┘   │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### 前端架构

```
┌─────────────────────────────────────────────────────────────────┐
│                     Vue3 + Pinia 前端                            │
│                                                                 │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                      Views (页面)                        │   │
│  │   LoginView  │  ChatView  │  GroupView  │  SettingView  │   │
│  └─────────────────────────────────────────────────────────┘   │
│                              │                                  │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                    Stores (状态管理)                      │   │
│  │        auth.js         │         chat.js                │   │
│  │    (登录态/Token)       │    (消息/会话/群组)             │   │
│  └─────────────────────────────────────────────────────────┘   │
│                              │                                  │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                  Composables (组合式函数)                 │   │
│  │   useWebSocket.js  │  useFileUpload.js  │  useToast.js  │   │
│  └─────────────────────────────────────────────────────────┘   │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### 数据流转

> 详细的前后端交互图请参考 [frontend-backend-interaction.excalidraw](./frontend-backend-interaction.excalidraw)

```
用户操作 → Store Action → API/WebSocket → 后端处理 → 响应 → Store 更新 → UI 刷新
```

---

## 🔧 技术选型

### 为什么选择这套技术栈？

| 组件 | 选型 | 理由 |
|-----|------|------|
| **后端语言** | Go | 高并发、低延迟、goroutine 轻量、部署简单 |
| **Web框架** | Gin | 性能优秀、生态成熟、学习曲线平缓 |
| **通信协议** | WebSocket | 双向实时、省资源、浏览器原生支持 |
| **消息格式** | JSON | 调试方便、前端友好、后期可换 Protobuf |
| **缓存** | Redis | 在线状态、Pub/Sub、会话缓存一站式解决 |
| **关系数据库** | MySQL | 用户/群组数据，事务支持好 |
| **文档数据库** | MongoDB | 消息存储，Schema 灵活，写入性能高 |
| **文件存储** | MinIO | S3 兼容、自托管、免费开源 |
| **前端框架** | Vue3 + Pinia | 组合式 API、响应式、轻量级状态管理 |
| **认证方案** | JWT | 无状态、易扩展、跨域友好 |

### 没有选择的方案

| 方案 | 不选择的原因 |
|-----|-------------|
| gRPC + Protobuf | 调试复杂，前端需要额外处理，后期可升级 |
| Kafka | 千人规模用 Redis Pub/Sub 足够，避免过度设计 |
| Cassandra | MongoDB 足够应对中等规模，运维更简单 |
| 自定义二进制协议 | 开发调试成本高，JSON 在中小规模够用 |

---

## 🔧 核心实现

### 1. 消息协议设计

```go
// 消息类型定义 (internal/model/message.go)
const (
    MsgText       = 0   // 文本消息
    MsgSingleChat = 1   // 单聊消息
    MsgGroupChat  = 2   // 群聊消息
    MsgSystem     = 3   // 系统消息
    MsgImage      = 4   // 图片消息
    MsgVoice      = 5   // 语音消息
    MsgVideo      = 6   // 视频消息
    MsgFile       = 7   // 文件消息
    MsgAck        = 30  // 消息确认
    MsgReadReceipt= 31  // 已读回执
    MsgTyping     = 33  // 正在输入
    MsgHeartbeat  = 99  // 心跳消息
)

// 统一消息结构
type Message struct {
    MessageID      string      `json:"message_id"`
    Type           MessageType `json:"type"`
    From           string      `json:"from"`
    To             string      `json:"to"`
    GroupID        string      `json:"group_id,omitempty"`
    Content        interface{} `json:"content"`
    Timestamp      int64       `json:"timestamp"`
    ConversationID string      `json:"conversation_id,omitempty"`
}
```

### 2. WebSocket 连接管理

```go
// 连接管理器 (internal/gateway/connection.go)
type ConnectionManager struct {
    nodeID      string
    connections sync.Map        // userID -> *Connection
    dispatcher  MessageDispatcher
}

// 注册新连接
func (m *ConnectionManager) Register(conn *Connection) {
    userID := conn.UserID
    
    // 踢掉旧连接（单设备登录策略）
    if old, ok := m.connections.Load(userID); ok {
        old.(*Connection).Close()
        log.Printf("Kicked old connection for user: %s", userID)
    }
    
    m.connections.Store(userID, conn)
    
    // 注册到消息分发器（记录在线状态到 Redis）
    m.dispatcher.RegisterConnection(userID, conn)
}

// 注销连接
func (m *ConnectionManager) Unregister(conn *Connection) {
    m.connections.Delete(conn.UserID)
    m.dispatcher.UnregisterConnection(conn.UserID)
}
```

### 3. 消息分发器

```go
func (d *messageDispatcherImpl) DispatchToUsers(ctx context.Context, userIDs []string, msg *Message) error {
    for _, userID := range userIDs {
        go func(uid string) {
            // 1. 尝试本地投递
            if d.pushToLocalUser(uid, data) {
                return
            }
            
            // 2. 查询用户所在节点
            nodeID, _ := d.GetUserNode(ctx, uid)
            
            if nodeID != "" && nodeID != d.config.NodeID {
                // 3. 通过 Redis Pub/Sub 发送到其他节点
                d.publishToNode(ctx, nodeID, uid, msg)
            } else {
                // 4. 用户离线，存储离线消息
                d.offlineSaver.SaveOfflineMessage(ctx, uid, msg)
            }
        }(userID)
    }
    return nil
}

// 群聊消息分发
func (d *messageDispatcherImpl) DispatchToConversation(ctx context.Context, conversationID string, msg *Message, excludeUserID string) error {
    // 获取群成员列表
    memberIDs, _ := d.groupMemberGetter.GetGroupMemberIDs(ctx, groupID)
    
    // 过滤掉发送者
    targetIDs := filterOut(memberIDs, excludeUserID)
    
    // 分发给所有目标用户
    return d.DispatchToUsers(ctx, targetIDs, msg)
}
```

### 4. 心跳保活机制

**后端实现：**

```go
// WebSocket 处理器 (internal/gateway/handler.go)
func (h *WebSocketHandler) readPump(conn *Connection) {
    // 设置 Pong 超时
    conn.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
    
    // Pong 处理器：收到 Pong 时重置超时
    conn.Conn.SetPongHandler(func(string) error {
        conn.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
        conn.UpdateLastActive()
        return nil
    })
    
    // 读取消息循环
    for {
        _, data, err := conn.Conn.ReadMessage()
        if err != nil {
            break
        }
        conn.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
        // 处理消息...
    }
}

func (h *WebSocketHandler) writePump(conn *Connection) {
    ticker := time.NewTicker(30 * time.Second) // 每 30 秒发送 Ping
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            conn.Conn.WriteMessage(websocket.PingMessage, nil)
        case data := <-conn.Send:
            conn.Conn.WriteMessage(websocket.TextMessage, data)
        }
    }
}
```

**前端实现：**

```javascript
// composables/useWebSocket.js
function startHeartbeat() {
  // 每 25 秒发送心跳（小于后端 30 秒的 Ping 间隔）
  heartbeatTimer = setInterval(() => {
    if (ws.value?.readyState === WebSocket.OPEN) {
      ws.value.send(JSON.stringify({
        type: 99,  // MsgHeartbeat
        content: { timestamp: Date.now() }
      }));
    }
  }, 25000);
}

// 断线重连（指数退避）
function scheduleReconnect() {
  reconnectAttempts.value++;
  const delay = Math.min(1000 * Math.pow(2, reconnectAttempts.value), 30000);
  
  setTimeout(() => {
    if (authStore.token) {
      connect();
    }
  }, delay);
}
```

---

## 📡 前后端交互协议

### 连接建立流程

```
┌─────────┐                                      ┌─────────┐
│  前端   │                                      │  后端   │
└────┬────┘                                      └────┬────┘
     │                                                │
     │  1. POST /api/login                            │
     │───────────────────────────────────────────────>│
     │                                                │
     │  2. { token, user_id, websocket_url }          │
     │<───────────────────────────────────────────────│
     │                                                │
     │  3. WebSocket 连接 /ws?token=xxx               │
     │═══════════════════════════════════════════════>│
     │                                                │ JWT 验证
     │  4. 连接成功                                    │ 注册连接
     │<═══════════════════════════════════════════════│ 记录在线状态
     │                                                │
     │  5. 心跳 (每25秒)                               │
     │═══════════════════════════════════════════════>│
     │                                                │
```

### 消息格式

**通用消息结构：**

```json
{
  "message_id": "msg_xxx",
  "type": 1,
  "from": "user_alice",
  "to": "user_bob",
  "content": { "text": "Hello!" },
  "timestamp": 1699999999999
}
```

**各类型内容格式：**

| 类型 | Content 结构 |
|-----|-------------|
| 文本 | `{ "text": "消息内容", "at_user_ids": ["user_1"] }` |
| 图片 | `{ "file_id": "xxx", "url": "...", "thumbnail_url": "..." }` |
| 文件 | `{ "file_id": "xxx", "file_name": "doc.pdf", "file_size": 1024 }` |
| 群事件 | `{ "group_id": "xxx", "operator_id": "xxx", "target_ids": [...] }` |

### 单聊消息流程

```
发送者                    Gateway                   接收者
  │                         │                         │
  │  1. 发送消息 (type=1)    │                         │
  │────────────────────────>│                         │
  │                         │  2. 保存到数据库         │
  │                         │                         │
  │  3. ACK (type=30)       │                         │
  │<────────────────────────│                         │
  │                         │  4. 分发消息             │
  │                         │────────────────────────>│
  │                         │                         │
```

### 群聊消息流程

```
发送者                    Gateway                  群成员(N人)
  │                         │                         │
  │  1. 发送群消息 (type=2)  │                         │
  │────────────────────────>│                         │
  │                         │  2. 保存到数据库         │
  │                         │                         │
  │  3. ACK (type=30)       │                         │
  │<────────────────────────│                         │
  │                         │  4. 获取群成员列表       │
  │                         │  5. 并发分发给所有成员   │
  │                         │────────────────────────>│ (成员1)
  │                         │────────────────────────>│ (成员2)
  │                         │────────────────────────>│ (成员N)
  │                         │    (排除发送者自己)      │
```

---

## 📋 API 接口

### 用户认证

```bash
# 注册
POST /api/register
Content-Type: application/json

{
  "username": "alice",
  "nickname": "Alice",
  "password": "123456"
}

# Response
{
  "code": 0,
  "message": "success",
  "data": {
    "user_id": "user_xxx",
    "username": "alice"
  }
}
```

```bash
# 登录
POST /api/login
Content-Type: application/json

{
  "username": "alice",
  "password": "123456"
}

# Response
{
  "code": 0,
  "data": {
    "user_id": "user_xxx",
    "token": "eyJhbGc...",
    "refresh_token": "eyJhbGc...",
    "websocket_url": "ws://localhost:8080/ws"
  }
}
```

### 群组管理

```bash
# 创建群组
POST /api/groups
Authorization: Bearer {token}

{
  "name": "技术交流群",
  "description": "讨论技术问题",
  "member_ids": ["user_a", "user_b"]
}

# 获取我的群组
GET /api/groups/my
Authorization: Bearer {token}

# 加入群组
POST /api/groups/{group_id}/join
Authorization: Bearer {token}

# 退出群组
POST /api/groups/{group_id}/leave
Authorization: Bearer {token}

# 获取群成员
GET /api/groups/{group_id}/members
Authorization: Bearer {token}
```

### 消息历史

```bash
# 私聊历史
GET /api/messages/private/{user_id}?limit=50&last_seq=0
Authorization: Bearer {token}

# 群聊历史
GET /api/messages/group/{group_id}?limit=50&last_seq=0
Authorization: Bearer {token}
```

### WebSocket 消息收发

```javascript
// 连接
const ws = new WebSocket(`ws://localhost:8080/ws?token=${token}`);

// 发送单聊消息
ws.send(JSON.stringify({
  type: 1,
  to: "user_bob",
  content: { text: "Hello!" }
}));

// 发送群聊消息
ws.send(JSON.stringify({
  type: 2,
  to: "group_123",
  group_id: "group_123",
  content: { text: "大家好!" }
}));

// 发送心跳
ws.send(JSON.stringify({
  type: 99,
  content: { timestamp: Date.now() }
}));
```

---

## 🚀 快速开始

### 方式一：Docker Compose（推荐）

```bash
# 克隆项目
git clone https://github.com/d60-lab/SimpleIM.git
cd SimpleIM/im-system

# 一键启动所有服务
make up

# 查看日志
make logs

# 访问演示页面
open http://localhost:8080
```

### 方式二：本地开发

```bash
# 1. 启动依赖服务（MySQL、Redis、MinIO）
make deps

# 2. 启动后端
make run

# 3. 启动前端（新终端）
cd web/chat-app
npm install
npm run dev

# 4. 访问
open http://localhost:5173
```

### 验证服务

```bash
# 健康检查
curl http://localhost:8080/health
# {"status":"ok","node_id":"node1","time":"2024-..."}

# 查看连接统计
curl http://localhost:8080/stats
# {"total_connections":0,"users_online":0}
```

---

## 🔒 安全设计

### JWT 认证

```go
// Token 生成 (pkg/auth/jwt.go)
func (m *JWTManager) GenerateTokenPair(userID, username, platform, deviceID string) (accessToken, refreshToken string, expiresAt time.Time, err error) {
    // Access Token: 7天有效
    accessToken, _ = m.generateToken(userID, username, 7*24*time.Hour)
    // Refresh Token: 30天有效
    refreshToken, _ = m.generateToken(userID, username, 30*24*time.Hour)
    return
}
```

### 请求认证

```go
// HTTP 接口认证中间件
func AuthMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        token := c.GetHeader("Authorization")
        if strings.HasPrefix(token, "Bearer ") {
            token = token[7:]
        }
        
        claims, err := auth.ParseAccessToken(token)
        if err != nil {
            c.JSON(401, gin.H{"error": "invalid token"})
            c.Abort()
            return
        }
        
        c.Set("user_id", claims.UserID)
        c.Next()
    }
}

// WebSocket 连接认证
func (h *WebSocketHandler) HandleWebSocket(c *gin.Context) {
    token := c.Query("token")
    claims, err := h.jwtManager.ParseToken(token)
    if err != nil {
        c.JSON(401, gin.H{"error": "invalid token"})
        return
    }
    // 升级连接...
}
```

### 消息去重

```go
// 防止消息重复处理 (internal/gateway/handler.go)
type MessageDeduper struct {
    cache map[string]int64  // messageID -> timestamp
    mu    sync.RWMutex
    size  int
}

func (d *MessageDeduper) IsDuplicate(messageID string) bool {
    d.mu.Lock()
    defer d.mu.Unlock()
    
    if _, exists := d.cache[messageID]; exists {
        return true  // 重复消息
    }
    
    d.cache[messageID] = time.Now().Unix()
    return false
}
```

---

## 📊 性能优化

### 性能指标

在 4核8G 服务器上的测试结果：

| 指标 | 数值 |
|-----|------|
| 单节点并发连接 | 10,000+ |
| 消息延迟（P99） | < 50ms |
| 消息吞吐量 | 10,000+ msg/s |
| 内存占用（1万连接） | ~500MB |
| MongoDB 写入 | 5,000+ ops/s |

### 优化技巧

**1. 连接池复用**

```go
// Redis 连接池
redis.NewClient(&redis.Options{
    PoolSize:     100,
    MinIdleConns: 10,
})

// MongoDB 连接池
mongoClient, _ := mongo.Connect(ctx, options.Client().
    ApplyURI(uri).
    SetMaxPoolSize(100).
    SetMinPoolSize(10))
```

**2. 并发消息分发**

```go
// 群消息并发投递
var wg sync.WaitGroup
for _, userID := range memberIDs {
    wg.Add(1)
    go func(uid string) {
        defer wg.Done()
        d.DispatchToUser(ctx, uid, msg)
    }(userID)
}
wg.Wait()
```

**3. MongoDB 索引优化**

```go
// 消息仓库索引 (internal/repository/message_repo.go)
func (r *messageRepository) EnsureIndexes(ctx context.Context) error {
    indexes := []mongo.IndexModel{
        // 消息ID唯一索引
        {
            Keys:    bson.D{{Key: "message_id", Value: 1}},
            Options: options.Index().SetUnique(true),
        },
        // 会话ID + 序号复合索引（用于分页查询）
        {
            Keys: bson.D{
                {Key: "conversation_id", Value: 1},
                {Key: "seq", Value: -1},
            },
        },
        // 发送者索引
        {
            Keys: bson.D{{Key: "from_user_id", Value: 1}},
        },
    }
    _, err := r.collection.Indexes().CreateMany(ctx, indexes)
    return err
}
```

---

## 🐛 踩坑指南

### 1. WebSocket 连接频繁断开

**问题**：客户端连接几分钟后自动断开

**原因**：Nginx/负载均衡器默认 60 秒超时

**解决**：

```nginx
# Nginx 配置
location /ws {
    proxy_pass http://backend;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
    proxy_read_timeout 3600s;  # 增加超时时间
    proxy_send_timeout 3600s;
}
```

### 2. 群消息丢失

**问题**：群成员收不到部分消息

**原因**：用户在线状态过期，被误判为离线

**解决**：

```go
// 定期刷新在线状态
func (d *Dispatcher) refreshOnlineStatus(ctx context.Context, userID string) {
    key := fmt.Sprintf("online:%s", userID)
    d.redis.Expire(ctx, key, time.Hour)  // 续期
}
```

### 3. 消息顺序错乱

**问题**：消息显示顺序与发送顺序不一致

**原因**：使用客户端时间戳，各设备时间不同步

**解决**：

```go
// 使用服务端时间戳 + 序列号
msg.Timestamp = time.Now().UnixMilli()
msg.Seq = d.getNextSeq(conversationID)
```

### 4. 前端心跳失效

**问题**：页面切到后台后心跳停止

**原因**：浏览器节流后台页面的 setInterval

**解决**：

```javascript
// 使用 Web Worker 发送心跳
const heartbeatWorker = new Worker('heartbeat-worker.js');

// 或者使用 visibilitychange 事件
document.addEventListener('visibilitychange', () => {
  if (document.visibilityState === 'visible') {
    sendHeartbeat();  // 页面恢复时立即发送心跳
  }
});
```

### 5. 跨域问题

**问题**：WebSocket 连接被 CORS 拦截

**解决**：

```go
// WebSocket 允许所有来源
upgrader := websocket.Upgrader{
    CheckOrigin: func(r *http.Request) bool {
        return true  // 生产环境应该检查来源
    },
}
```

---

## 🛣️ 扩展路线

当用户量增长时，按需升级：

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   千人级（当前）  │ →  │     万人级       │ →  │     十万人级     │
└─────────────────┘    └─────────────────┘    └─────────────────┘
        │                      │                      │
        ▼                      ▼                      ▼
   单节点 Gateway        多节点 + LB            独立路由层
   Redis Pub/Sub       Redis Cluster        Kafka/RocketMQ
   MySQL 单库            读写分离              分库分表
   MongoDB 单节点       MongoDB 副本集        MongoDB 分片
   JSON 协议            Protobuf            自定义二进制
   MinIO 单节点         MinIO 集群            CDN 加速
```

### 功能扩展建议

| 功能 | 实现思路 |
|-----|---------|
| **消息撤回** | 发送撤回消息(type=32)，客户端收到后删除本地消息 |
| **消息已读** | 定时上报已读位置，服务端计算未读数 |
| **@提醒** | 消息 content 中添加 at_user_ids，客户端高亮显示 |
| **消息搜索** | 接入 Elasticsearch，全文检索消息内容 |
| **音视频通话** | 集成 WebRTC，服务端做信令转发 |
| **多端同步** | 每个设备独立连接，消息广播给同一用户的所有设备 |

---

## 📁 项目结构

```
im-system/
├── cmd/
│   └── gateway/main.go           # 服务入口
│
├── internal/
│   ├── gateway/                  # 网关核心
│   │   ├── connection.go         # 连接对象
│   │   ├── connection_manager.go # 连接管理
│   │   ├── dispatcher.go         # 消息分发
│   │   └── handler.go            # WebSocket 处理
│   │
│   ├── handler/                  # HTTP 接口
│   │   ├── user_handler.go       # 用户接口
│   │   ├── group_handler.go      # 群组接口
│   │   ├── message_handler.go    # 消息接口
│   │   └── file_handler.go       # 文件接口
│   │
│   ├── service/                  # 业务逻辑
│   │   ├── group_service.go      # 群组服务
│   │   ├── message_service.go    # 消息服务
│   │   └── user_service.go       # 用户服务
│   │
│   ├── repository/               # 数据访问
│   └── model/                    # 数据模型
│
├── pkg/
│   ├── auth/                     # JWT 认证
│   └── util/                     # 工具函数
│
├── web/chat-app/                 # Vue3 前端
│   ├── src/
│   │   ├── views/                # 页面
│   │   ├── components/           # 组件
│   │   ├── stores/               # Pinia 状态
│   │   └── composables/          # 组合式函数
│   └── package.json
│
├── deploy/
│   ├── docker-compose.yml        # 容器编排
│   └── Dockerfile
│
├── docs/
│   ├── frontend-backend-interaction.excalidraw  # 架构图
│   └── frontend-backend-interaction.md          # 交互文档
│
├── Makefile
└── README.md
```

---

## 🎯 总结

本文实现了一个**简单实用**的全栈 IM 系统：

### 技术亮点

1. **架构简洁** — 三层架构（接入层/业务层/存储层），没有过度设计
2. **技术主流** — Go + Vue3 + WebSocket + Redis + MySQL + MongoDB
3. **功能完整** — 单聊、群聊、文件传输、离线消息、心跳保活
4. **生产可用** — JWT 认证、消息去重、断线重连、错误处理
5. **存储分离** — MySQL 存用户/群组，MongoDB 存消息，各取所长

### 适用场景

- 🏢 中小型应用的 IM 需求（电商客服、社区聊天）
- 📚 学习 IM 系统设计和全栈开发
- 🔧 作为更复杂系统的起点

### 不适用场景

- ❌ 超大规模（百万级在线）—— 需要更复杂的架构
- ❌ 强一致性要求 —— 需要引入消息队列
- ❌ 金融级可靠性 —— 需要更完善的容灾方案

---

**🔗 完整代码**: [github.com/d60-lab/SimpleIM](https://github.com/d60-lab/SimpleIM)

---

## 📚 参考资料

- [WebSocket 协议规范 RFC 6455](https://tools.ietf.org/html/rfc6455)
- [Redis Pub/Sub 文档](https://redis.io/topics/pubsub)
- [Gorilla WebSocket](https://github.com/gorilla/websocket)
- [Gin Web Framework](https://gin-gonic.com/)
- [Vue 3 文档](https://vuejs.org/)
- [Pinia 状态管理](https://pinia.vuejs.org/)

---

> 💡 **有问题或建议？** 欢迎提交 [Issue](https://github.com/d60-lab/SimpleIM/issues) 或 [PR](https://github.com/d60-lab/SimpleIM/pulls)！