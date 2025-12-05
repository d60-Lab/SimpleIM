# Vue3 聊天应用

基于 Vue3 + Vite + Pinia + TailwindCSS 构建的即时通讯前端应用。

## 技术栈

- **Vue 3** - 渐进式 JavaScript 框架
- **Vite** - 下一代前端构建工具
- **Pinia** - Vue 官方状态管理库
- **Vue Router** - Vue 官方路由
- **TailwindCSS** - 原子化 CSS 框架

## 项目结构

```
chat-app/
├── src/
│   ├── components/          # 组件
│   │   ├── ContactList.vue      # 联系人列表
│   │   ├── GroupList.vue        # 群组列表
│   │   ├── MessageList.vue      # 消息列表
│   │   ├── MessageContent.vue   # 消息内容渲染
│   │   ├── MessageInput.vue     # 消息输入框
│   │   ├── GroupInfoPanel.vue   # 群组信息面板
│   │   └── CreateGroupModal.vue # 创建群组弹窗
│   ├── composables/         # 组合式函数
│   │   ├── useWebSocket.js      # WebSocket 连接管理
│   │   ├── useFileUpload.js     # 文件上传
│   │   └── useToast.js          # 消息提示
│   ├── stores/              # Pinia 状态管理
│   │   ├── auth.js              # 认证状态
│   │   └── chat.js              # 聊天状态
│   ├── views/               # 页面视图
│   │   ├── LoginView.vue        # 登录/注册页
│   │   └── ChatView.vue         # 聊天主页
│   ├── router/              # 路由配置
│   │   └── index.js
│   ├── App.vue              # 根组件
│   ├── main.js              # 入口文件
│   └── style.css            # 全局样式
├── index.html
├── vite.config.js           # Vite 配置
├── package.json
└── dev.sh                   # 开发脚本
```

## 快速开始

### 安装依赖

```bash
npm install
# 或
./dev.sh install
```

### 开发模式

```bash
npm run dev
# 或
./dev.sh dev
```

开发服务器会运行在 `http://localhost:3000`，并自动代理 API 请求到后端 (`localhost:8080`)。

### 构建生产版本

```bash
npm run build
# 或
./dev.sh build
```

构建产物会输出到 `dist/` 目录。

### 预览构建

```bash
npm run preview
# 或
./dev.sh preview
```

## 开发说明

### 后端代理配置

`vite.config.js` 中配置了开发代理：

```javascript
server: {
  port: 3000,
  proxy: {
    "/api": {
      target: "http://localhost:8080",
      changeOrigin: true,
    },
    "/ws": {
      target: "ws://localhost:8080",
      ws: true,
    },
    "/uploads": {
      target: "http://localhost:8080",
      changeOrigin: true,
    },
  },
}
```

### 状态管理

使用 Pinia 进行状态管理：

- **auth store** - 用户认证状态、登录/注册/登出
- **chat store** - 消息、联系人、群组、当前会话

### WebSocket

WebSocket 连接管理在 `useWebSocket.js` 中实现：

- 自动重连（指数退避）
- 心跳保活
- 消息处理

### 文件上传

文件上传在 `useFileUpload.js` 中实现：

- 文件选择与预览
- 上传进度显示
- 支持图片和普通文件

## 部署

### 集成到后端

构建后的 `dist/` 目录会被后端自动识别并服务。后端会：

1. 静态资源服务 (`/assets/*`)
2. SPA 路由支持 (`/login`, `/chat` 等)
3. NoRoute 处理（返回 `index.html`）

### 独立部署

如需独立部署，可以使用 Nginx：

```nginx
server {
    listen 80;
    server_name your-domain.com;
    root /path/to/dist;
    index index.html;

    # SPA 路由支持
    location / {
        try_files $uri $uri/ /index.html;
    }

    # API 代理
    location /api {
        proxy_pass http://backend:8080;
    }

    # WebSocket 代理
    location /ws {
        proxy_pass http://backend:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }
}
```

## 与旧版对比

| 特性 | 旧版 (chat.html) | 新版 (Vue3) |
|------|------------------|-------------|
| 代码行数 | ~2000行单文件 | 多文件组件化 |
| 状态管理 | 全局变量 | Pinia |
| UI 更新 | 手动 DOM 操作 | Vue 响应式 |
| 可维护性 | 低 | 高 |
| 调试 | 困难 | Vue DevTools |
| 复用性 | 低 | 组件可复用 |

## 功能特性

- ✅ 用户登录/注册
- ✅ 私聊消息
- ✅ 群组聊天
- ✅ 创建/加入/退出群组
- ✅ 文件/图片发送
- ✅ 消息历史加载
- ✅ WebSocket 自动重连
- ✅ 未读消息计数
- ✅ 响应式布局