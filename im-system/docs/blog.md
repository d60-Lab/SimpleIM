# 从零构建千人在线IM系统：Go语言实战指南

> 一个周末就能跑起来的即时通讯系统，代码简洁、架构清晰、开箱即用。

![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)
![WebSocket](https://img.shields.io/badge/WebSocket-Real--time-brightgreen)
![Redis](https://img.shields.io/badge/Redis-7.0-DC382D?style=flat&logo=redis)
![MySQL](https://img.shields.io/badge/MySQL-8.0-4479A1?style=flat&logo=mysql)

## 📖 前言

IM（即时通讯）系统是现代应用的标配功能。市面上有很多成熟的云服务，但出于**数据安全**、**定制需求**或**成本控制**的考虑，自研IM仍是许多团队的选择。

本文将带你从零实现一个**支持千人同时在线**的轻量级IM系统，特点是：

- ✅ **代码简洁** - 核心逻辑不到2000行Go代码
- ✅ **开箱即用** - Docker一键启动，自带Web演示页面
- ✅ **功能完整** - 单聊、群聊、离线消息、心跳保活
- ✅ **易于扩展** - 清晰的分层架构，方便二次开发

---

## 🏗️ 架构设计

### 整体架构

```
┌─────────────────────────────────────────────────────────────┐
│                         客户端                               │
│              (Web / iOS / Android / 桌面)                    │
└───────────────────────────┬─────────────────────────────────┘
                            │ WebSocket + JSON
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                    Gateway 接入层                            │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐                  │
│  │  Node 1  │  │  Node 2  │  │  Node N  │   ← 无状态，可水平扩展
│  └────┬─────┘  └────┬─────┘  └────┬─────┘                  │
└───────┼─────────────┼─────────────┼─────────────────────────┘
        │             │             │
        └─────────────┼─────────────┘
                      │ Redis Pub/Sub（跨节点通信）
                      ▼
┌─────────────────────────────────────────────────────────────┐
│                      存储层                                  │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐                  │
│  │  MySQL   │  │  Redis   │  │  MinIO   │                  │
│  │ (业务数据) │  │ (缓存/状态) │  │ (文件存储) │                  │
│  └──────────┘  └──────────┘  └──────────┘                  │
└─────────────────────────────────────────────────────────────┘
```

### 为什么这样设计？

| 设计决策 | 原因 |
|---------|------|
| **WebSocket** | 双向实时通信，比轮询省资源，生态成熟 |
| **无状态Gateway** | 方便水平扩展，挂掉一台不影响整体 |
| **Redis Pub/Sub** | 轻量级跨节点消息路由，千人规模够用 |
| **JSON协议** | 调试方便，前端友好，性能够用（后期可换Protobuf） |

---

## 🔧 核心实现

### 1. 消息协议设计

```go
// 消息类型定义
const (
    MsgSingleChat   = 1   // 单聊消息
    MsgGroupChat    = 2   // 群聊消息
    MsgAck          = 30  // 消息确认
    MsgHeartbeat    = 99  // 心跳
)

// 统一消息结构
type Message struct {
    MessageID string      `json:"message_id"`
    Type      int         `json:"type"`
    From      string      `json:"from"`
    To        string      `json:"to"`
    GroupID   string      `json:"group_id,omitempty"`
    Content   interface{} `json:"content"`
    Timestamp int64       `json:"timestamp"`
}
```

### 2. 连接管理器

```go
type ConnectionManager struct {
    nodeID      string
    connections sync.Map  // userID -> *Connection
    config      *ConnectionConfig
}

// 注册新连接
func (m *ConnectionManager) Register(userID string, conn *Connection) {
    // 踢掉旧连接（单设备登录）
    if old, ok := m.connections.Load(userID); ok {
        old.(*Connection).Close("kicked")
    }
    m.connections.Store(userID, conn)
    
    // 记录在线状态到Redis
    m.redis.Set(ctx, fmt.Sprintf("online:%s", userID), m.nodeID, time.Hour)
}

// 发送消息给本地用户
func (m *ConnectionManager) SendToLocal(userID string, msg *Message) bool {
    if conn, ok := m.connections.Load(userID); ok {
        return conn.(*Connection).Send(msg)
    }
    return false
}
```

### 3. 跨节点消息路由

```go
func (d *Dispatcher) DispatchToUser(ctx context.Context, userID string, msg *Message) error {
    // 1. 尝试本地投递
    if d.connManager.SendToLocal(userID, msg) {
        return nil
    }
    
    // 2. 查询用户所在节点
    nodeID, err := d.redis.Get(ctx, fmt.Sprintf("online:%s", userID)).Result()
    if err == redis.Nil {
        // 用户离线，存储离线消息
        return d.offlineService.Save(userID, msg)
    }
    
    // 3. 发布到目标节点的频道
    data, _ := json.Marshal(msg)
    return d.redis.Publish(ctx, fmt.Sprintf("im:node:%s", nodeID), data).Err()
}
```

### 4. 心跳保活机制

```go
// 服务端：定期检查连接活性
func (c *Connection) StartHeartbeatChecker() {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()
    
    for range ticker.C {
        if time.Since(c.lastActive) > 90*time.Second {
            c.Close("heartbeat timeout")
            return
        }
    }
}

// 客户端：定期发送心跳
setInterval(() => {
    if (ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({ type: 99, timestamp: Date.now() }));
    }
}, 30000);
```

---

## 📡 API 接口

### 用户认证

```bash
# 注册
POST /api/register
{ "username": "alice", "password": "123456" }

# 登录
POST /api/login
{ "username": "alice", "password": "123456" }
# 返回: { "token": "eyJhbGc...", "user_id": "u_xxx" }
```

### WebSocket 连接

```javascript
const ws = new WebSocket(`ws://localhost:8080/ws?token=${token}`);

// 发送单聊消息
ws.send(JSON.stringify({
    type: 1,
    to: "user_bob",
    content: { text: "Hello!" },
    timestamp: Date.now()
}));

// 发送群聊消息
ws.send(JSON.stringify({
    type: 2,
    to: "group_123",
    group_id: "group_123",
    content: { text: "大家好!" },
    timestamp: Date.now()
}));
```

### 群组管理

```bash
# 创建群组
POST /api/groups
{ "name": "技术交流群", "member_ids": ["user_a", "user_b"] }

# 加入群组
POST /api/groups/{group_id}/join

# 获取群成员
GET /api/groups/{group_id}/members
```

---

## 🚀 快速开始

### 方式一：Docker Compose（推荐）

```bash
# 克隆项目
git clone https://github.com/d60-lab/SimpleIM.git
cd SimpleIM/im-system

# 一键启动
make up

# 查看日志
make logs

# 访问演示页面
open http://localhost:8080
```

### 方式二：本地开发

```bash
# 启动依赖服务
make deps

# 运行Gateway
make run

# 或者直接
go run cmd/gateway/main.go
```

### 验证服务

```bash
# 健康检查
curl http://localhost:8080/health

# 查看统计
curl http://localhost:8080/stats
```

---

## 📊 性能指标

在 4核8G 服务器上的测试结果：

| 指标 | 数值 |
|-----|------|
| 单节点并发连接 | 10,000+ |
| 消息延迟（P99） | < 50ms |
| 消息吞吐量 | 10,000+ QPS |
| 内存占用（1万连接） | ~500MB |

---

## 🔍 关键设计点

### 1. 消息可靠性

```
客户端                    服务端
   │                        │
   │─────── 发送消息 ───────>│
   │                        │
   │<────── ACK确认 ────────│
   │                        │
   │  (超时未收到ACK则重发)   │
```

### 2. 离线消息处理

```go
// 用户上线时拉取离线消息
func (h *Handler) OnConnect(userID string) {
    messages, _ := h.offlineService.Pull(userID, 100)
    for _, msg := range messages {
        h.connManager.SendToLocal(userID, msg)
    }
    h.offlineService.Ack(userID, messageIDs)
}
```

### 3. 群消息扩散

```go
func (d *Dispatcher) DispatchToGroup(ctx context.Context, groupID string, msg *Message) error {
    // 获取群成员列表（Redis缓存）
    memberIDs, _ := d.groupService.GetMemberIDs(ctx, groupID)
    
    // 并发投递给每个成员
    var wg sync.WaitGroup
    for _, userID := range memberIDs {
        if userID == msg.From {
            continue // 不发给自己
        }
        wg.Add(1)
        go func(uid string) {
            defer wg.Done()
            d.DispatchToUser(ctx, uid, msg)
        }(userID)
    }
    wg.Wait()
    return nil
}
```

---

## 🛣️ 扩展路线

当用户量增长时，可以按需升级：

```
千人级（当前）          万人级              十万人级
     │                   │                    │
     ▼                   ▼                    ▼
 单节点Gateway  →   多节点+负载均衡  →   独立路由层
 Redis Pub/Sub  →   Redis Cluster   →   Kafka/RocketMQ
 MySQL单库      →   读写分离        →   分库分表
 JSON协议       →   Protobuf        →   自定义二进制协议
```

---

## 📁 项目结构

```
im-system/
├── cmd/
│   └── gateway/main.go       # 服务入口
├── internal/
│   ├── gateway/              # 网关核心
│   │   ├── connection.go     # 连接管理
│   │   ├── dispatcher.go     # 消息分发
│   │   └── handler.go        # WebSocket处理
│   ├── service/              # 业务服务
│   │   ├── group_service.go  # 群组服务
│   │   └── offline_service.go# 离线消息
│   ├── handler/              # HTTP接口
│   └── model/                # 数据模型
├── pkg/auth/                 # JWT认证
├── web/chat-app/             # Vue3 前端应用
├── deploy/
│   ├── docker-compose.yml
│   └── Dockerfile
├── Makefile
└── README.md
```

---

## 🎯 总结

本文实现了一个**简单实用**的IM系统，核心特点：

1. **架构简洁** - 三层架构，没有过度设计
2. **技术主流** - Go + WebSocket + Redis + MySQL
3. **功能完整** - 单聊、群聊、离线消息、心跳保活
4. **易于扩展** - 清晰的接口设计，方便添加新功能

这个方案适合：
- 🏢 中小型应用的IM需求
- 📚 学习IM系统设计
- 🔧 作为更复杂系统的起点

**完整代码已开源**: [github.com/d60-lab/SimpleIM](https://github.com/d60-lab/SimpleIM)

---

## 📚 参考资料

- [WebSocket 协议规范 RFC 6455](https://tools.ietf.org/html/rfc6455)
- [Redis Pub/Sub 文档](https://redis.io/topics/pubsub)
- [Gorilla WebSocket](https://github.com/gorilla/websocket)

---

> 💡 **有问题或建议？** 欢迎提交 Issue 或 PR！