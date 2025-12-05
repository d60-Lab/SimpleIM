# SimpleIM 前后端交互逻辑文档

## 概述

SimpleIM 是一个即时通讯系统，前端使用 Vue3 + Pinia 构建，后端使用 Go + Gin + WebSocket 实现。本文档详细描述了前后端的交互逻辑。

## 架构图

详细架构图请参考同目录下的 `frontend-backend-interaction.excalidraw` 文件。

## 前端架构

### 核心模块

| 模块 | 文件路径 | 职责 |
|------|----------|------|
| 认证状态管理 | `stores/auth.js` | 用户登录/注册、Token 管理 |
| 聊天状态管理 | `stores/chat.js` | 消息、会话、群组数据管理 |
| WebSocket 通信 | `composables/useWebSocket.js` | WebSocket 连接、消息收发 |

### 前端状态流

```
用户操作 → Store Action → API/WebSocket → 后端处理 → 响应 → Store 更新 → UI 刷新
```

## 后端架构

### 核心模块

| 模块 | 文件路径 | 职责 |
|------|----------|------|
| WebSocket 网关 | `internal/gateway/handler.go` | WS 连接管理、消息处理 |
| 消息分发器 | `internal/gateway/dispatcher.go` | 消息路由、多节点分发 |
| 用户处理器 | `internal/handler/user_handler.go` | 用户相关 API |
| 群组处理器 | `internal/handler/group_handler.go` | 群组管理 API |
| 消息处理器 | `internal/handler/message_handler.go` | 消息历史查询 API |
| 消息仓库 | `internal/repository/message_repo.go` | MongoDB 消息存储 |

---

## 一、HTTP REST API 交互

### 1.1 用户认证

#### 注册
```
POST /api/register
Content-Type: application/json

Request:
{
  "username": "string",
  "nickname": "string",
  "password": "string"
}

Response:
{
  "code": 0,
  "message": "success",
  "data": {
    "user_id": "user_xxx",
    "username": "string",
    "nickname": "string"
  }
}
```

#### 登录
```
POST /api/login
Content-Type: application/json

Request:
{
  "username": "string",
  "password": "string"
}

Response:
{
  "code": 0,
  "message": "success",
  "data": {
    "user_id": "user_xxx",
    "username": "string",
    "nickname": "string",
    "token": "jwt_token",
    "refresh_token": "refresh_token",
    "expires_at": "timestamp",
    "websocket_url": "ws://host/ws"
  }
}
```

### 1.2 群组管理

#### 获取我的群组列表
```
GET /api/groups/my
Authorization: Bearer {token}

Response:
{
  "code": 0,
  "data": {
    "total": 2,
    "groups": [
      {
        "group_id": "group_xxx",
        "name": "群组名",
        "avatar": "url",
        "member_count": 10
      }
    ]
  }
}
```

#### 创建群组
```
POST /api/groups
Authorization: Bearer {token}
Content-Type: application/json

Request:
{
  "name": "群组名称",
  "description": "群组描述",
  "member_ids": ["user_1", "user_2"]
}
```

#### 加入/退出群组
```
POST /api/groups/{group_id}/join
POST /api/groups/{group_id}/leave
Authorization: Bearer {token}
```

### 1.3 消息历史

#### 私聊历史
```
GET /api/messages/private/{user_id}?limit=50&last_seq=0
Authorization: Bearer {token}

Response:
{
  "code": 0,
  "data": {
    "messages": [...],
    "has_more": true
  }
}
```

#### 群聊历史
```
GET /api/messages/group/{group_id}?limit=50&last_seq=0
Authorization: Bearer {token}
```

---

## 二、WebSocket 实时通信

### 2.1 连接建立

#### 连接流程
```
1. 前端获取 token（登录后）
2. 建立 WebSocket 连接: ws://host/ws?token={jwt_token}
3. 后端验证 token（JWT 解析）
4. 后端创建 Connection 对象，注册到 ConnectionManager
5. 后端在 Redis 中记录用户在线状态
6. 启动 readPump 和 writePump 协程
```

#### 前端代码
```javascript
const WS_URL = `${protocol}://${host}/ws`;
ws.value = new WebSocket(`${WS_URL}?token=${authStore.token}`);
```

#### 后端代码
```go
// 验证 token
claims, err := h.jwtManager.ParseToken(token)
// 升级 WebSocket
wsConn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
// 注册连接
h.connMgr.Register(conn)
```

### 2.2 消息类型定义

```go
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
```

### 2.3 消息格式

#### 通用消息结构
```json
{
  "message_id": "msg_xxx",
  "type": 1,
  "from": "user_sender",
  "to": "user_receiver",
  "group_id": "group_xxx",
  "content": {},
  "timestamp": 1699999999999,
  "conversation_id": "single:user1:user2"
}
```

#### 文本消息内容
```json
{
  "text": "消息内容",
  "at_user_ids": ["user_1"],
  "at_all": false
}
```

#### 图片/文件消息内容
```json
{
  "file_id": "file_xxx",
  "file_name": "image.jpg",
  "file_size": 102400,
  "file_type": "image",
  "mime_type": "image/jpeg",
  "url": "https://...",
  "thumbnail_url": "https://..."
}
```

### 2.4 单聊消息流程

```
┌─────────┐     ┌─────────┐     ┌─────────────┐     ┌─────────┐
│  发送者  │────▶│ Gateway │────▶│ Dispatcher  │────▶│  接收者  │
└─────────┘     └─────────┘     └─────────────┘     └─────────┘
     │               │                 │                  │
     │  1.发送消息    │                 │                  │
     │──────────────▶│                 │                  │
     │               │ 2.保存消息到DB   │                  │
     │               │────────────────▶│                  │
     │               │ 3.返回ACK       │                  │
     │◀──────────────│                 │                  │
     │               │ 4.分发消息       │                  │
     │               │────────────────▶│                  │
     │               │                 │ 5.推送给接收者    │
     │               │                 │─────────────────▶│
```

**代码流程：**

前端发送：
```javascript
function sendPrivateMessage(to, text) {
  sendMessage(1, to, { text });  // type=1 单聊
  // 同时添加到本地 store
  chatStore.addMessage(chatKey, messageData);
}
```

后端处理：
```go
func (h *WebSocketHandler) handleSingleChat(ctx, conn, msg) error {
    // 1. 设置会话ID
    msg.ConversationID = model.GetSingleChatConversationID(msg.From, msg.To)
    
    // 2. 保存消息到数据库
    h.messageSaver.SaveMessage(ctx, msg)
    
    // 3. 发送ACK给发送者
    ack := model.NewAckMessage(msg.MessageID, 0)
    conn.SendJSON(ack)
    
    // 4. 分发消息给接收者
    return h.dispatcher.DispatchToUsers(ctx, []string{msg.To}, msg)
}
```

### 2.5 群聊消息流程

```
┌─────────┐     ┌─────────┐     ┌─────────────┐     ┌──────────────┐
│  发送者  │────▶│ Gateway │────▶│ Dispatcher  │────▶│ 所有群成员    │
└─────────┘     └─────────┘     └─────────────┘     └──────────────┘
     │               │                 │                    │
     │ 1.发送群消息   │                 │                    │
     │──────────────▶│                 │                    │
     │               │ 2.保存消息       │                    │
     │               │────────────────▶│                    │
     │               │ 3.返回ACK       │                    │
     │◀──────────────│                 │                    │
     │               │ 4.获取群成员     │                    │
     │               │────────────────▶│                    │
     │               │                 │ 5.分发给所有成员    │
     │               │                 │───────────────────▶│
     │               │                 │   (排除发送者)      │
```

**代码流程：**

前端发送：
```javascript
function sendGroupMessage(groupId, text) {
  sendMessage(2, groupId, { text }, groupId);  // type=2 群聊
}
```

后端处理：
```go
func (h *WebSocketHandler) handleGroupChat(ctx, conn, msg) error {
    // 1. 设置会话ID
    msg.ConversationID = model.GetGroupChatConversationID(msg.To)
    
    // 2. 保存消息
    h.messageSaver.SaveMessage(ctx, msg)
    
    // 3. 发送ACK
    conn.SendJSON(model.NewAckMessage(msg.MessageID, 0))
    
    // 4. 分发给群成员（排除发送者）
    return h.dispatcher.DispatchToConversation(ctx, msg.ConversationID, msg, msg.From)
}
```

### 2.6 心跳保活机制

#### 前端心跳
```javascript
// 每25秒发送心跳
heartbeatTimer = setInterval(() => {
  ws.send(JSON.stringify({
    type: 99,  // MsgHeartbeat
    content: { timestamp: Date.now() }
  }));
}, 25000);
```

#### 后端心跳检测
```go
// Ping 间隔 30秒
PingInterval: 30 * time.Second
// Pong 超时 60秒
PongTimeout: 60 * time.Second

// Pong 处理器
conn.Conn.SetPongHandler(func(string) error {
    conn.Conn.SetReadDeadline(time.Now().Add(h.config.PongTimeout))
    conn.UpdateLastActive()
    return nil
})
```

### 2.7 断线重连

```javascript
function scheduleReconnect() {
  reconnectAttempts.value++;
  // 指数退避：1s, 2s, 4s, 8s... 最大30s
  const delay = Math.min(1000 * Math.pow(2, reconnectAttempts.value), 30000);
  
  reconnectTimer = setTimeout(() => {
    if (authStore.token) {
      connect();
    }
  }, delay);
}
```

---

## 三、消息分发器 (Dispatcher)

### 3.1 本地分发

```go
func (d *messageDispatcherImpl) pushToLocalUser(userID string, data []byte) bool {
    conn, ok := d.localConns[userID]
    if !ok {
        return false
    }
    return conn.SendData(data) == nil
}
```

### 3.2 跨节点分发（通过 Redis Pub/Sub）

```go
func (d *messageDispatcherImpl) DispatchToUsers(ctx, userIDs, msg) error {
    for _, userID := range userIDs {
        // 尝试本地推送
        if d.pushToLocalUser(userID, data) {
            continue
        }
        
        // 检查用户在哪个节点
        nodeID, _ := d.GetUserNode(ctx, userID)
        
        if nodeID != "" && nodeID != d.config.NodeID {
            // 通过 Redis 发布到其他节点
            d.publishToNode(ctx, nodeID, userID, msg)
        } else {
            // 用户离线，保存离线消息
            d.offlineSaver.SaveOfflineMessage(ctx, userID, msg)
        }
    }
}
```

### 3.3 在线状态管理

```go
// 注册连接时
func (d *messageDispatcherImpl) RegisterConnection(userID string, conn Conn) error {
    d.localConns[userID] = conn
    // Redis 记录在线状态，格式：online:{userID} -> {nodeID}:{timestamp}
    onlineKey := fmt.Sprintf("online:%s", userID)
    return d.redis.SetEX(ctx, onlineKey, nodeInfo, d.config.OnlineKeyExpire).Err()
}

// 注销连接时
func (d *messageDispatcherImpl) UnregisterConnection(userID string) error {
    delete(d.localConns, userID)
    return d.redis.Del(ctx, onlineKey).Err()
}
```

---

## 四、数据存储

### 4.1 Redis 用途

| Key 模式 | 用途 |
|----------|------|
| `online:{userID}` | 用户在线状态及所在节点 |
| `im:node:{nodeID}` | 节点消息订阅频道 |
| `im:nodes` | 节点列表 |
| `group:members:{groupID}` | 群成员缓存 |

### 4.2 数据库存储

| 数据类型 | 存储位置 | 说明 |
|----------|----------|------|
| 用户信息 | MySQL | users 表 |
| 群组信息 | MySQL | groups 表 |
| 群成员关系 | MySQL | group_members 表 |
| 消息记录 | **MongoDB** | messages 集合，Schema 灵活，写入性能高 |
| 离线消息 | MySQL | offline_messages 表 |

### 4.3 为什么消息用 MongoDB？

| 优势 | 说明 |
|------|------|
| **Schema 灵活** | 不同消息类型的 content 结构不同，MongoDB 无需预定义 |
| **写入性能高** | 追加写入场景，MongoDB 性能优于 MySQL |
| **水平扩展** | 后期可方便地使用分片集群 |
| **查询友好** | 支持丰富的查询语法，适合消息检索 |

```go
// MongoDB 消息文档结构 (internal/repository/message_repo.go)
type MessageDocument struct {
    ID             primitive.ObjectID     `bson:"_id,omitempty"`
    MessageID      string                 `bson:"message_id"`
    ConversationID string                 `bson:"conversation_id"`
    FromUserID     string                 `bson:"from_user_id"`
    ToUserID       string                 `bson:"to_user_id"`
    GroupID        string                 `bson:"group_id,omitempty"`
    MsgType        int                    `bson:"msg_type"`
    Content        map[string]interface{} `bson:"content"`
    Seq            int64                  `bson:"seq"`
    CreatedAt      time.Time              `bson:"created_at"`
}
```

---

## 五、前端消息处理

### 5.1 消息接收处理

```javascript
function handleMessage(msg) {
  switch (msg.type) {
    case 0: // 系统消息
    case 1: // 私聊消息
      handlePrivateMessage(msg);
      break;
    case 2: // 群聊消息
      handleGroupMessage(msg);
      break;
    case 100: // 心跳响应
      break;
  }
}

function handlePrivateMessage(msg) {
  const fromUser = msg.from;
  const isFromMe = fromUser === authStore.userId;
  const chatId = isFromMe ? msg.to : fromUser;
  const chatKey = `user:${chatId}`;

  // 添加到消息列表
  chatStore.addMessage(chatKey, messageData);

  // 更新未读计数
  if (!isFromMe && !isCurrentChat) {
    chatStore.incrementUnreadCount('user', chatId);
  }
}
```

### 5.2 消息内容解析

```javascript
function getContentType(msgType, content) {
  if (msgType === 4 || content?.file_type === 'image') return 'image';
  if (msgType === 7 || (content?.file_id && content?.file_type !== 'image')) return 'file';
  if (msgType === 5) return 'voice';
  if (msgType === 6) return 'video';
  return 'text';
}
```

---

## 六、安全机制

### 6.1 JWT 认证

- 登录成功后返回 `access_token` 和 `refresh_token`
- WebSocket 连接时通过 URL 参数传递 token
- HTTP API 通过 `Authorization: Bearer {token}` 头传递
- Token 过期后使用 refresh_token 刷新

### 6.2 消息去重

```go
type MessageDeduper struct {
    cache map[string]int64  // messageID -> timestamp
    size  int               // 最大缓存数量
}

func (d *MessageDeduper) IsDuplicate(messageID string) bool {
    if _, exists := d.cache[messageID]; exists {
        return true
    }
    d.cache[messageID] = time.Now().Unix()
    return false
}
```

---

## 附录：会话 ID 生成规则

```go
// 单聊会话ID：按字母序排列两个用户ID
func GetSingleChatConversationID(userID1, userID2 string) string {
    if userID1 < userID2 {
        return "single:" + userID1 + ":" + userID2
    }
    return "single:" + userID2 + ":" + userID1
}

// 群聊会话ID
func GetGroupChatConversationID(groupID string) string {
    return "group:" + groupID
}
```
