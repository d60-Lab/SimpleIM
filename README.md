# SimpleIM

一个轻量级的即时通讯系统，基于 Go 语言实现，支持千人同时在线。

[![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

## ✨ 特性

- 🚀 **轻量高效** - 核心代码简洁，单节点支持万级连接
- 💬 **功能完整** - 单聊、群聊、离线消息、消息已读、消息撤回
- 📦 **开箱即用** - Docker 一键启动，自带 Web 演示页面
- 🔧 **易于扩展** - 清晰的分层架构，方便二次开发

## 🚀 快速开始

```bash
cd im-system

# 启动所有服务
make up

# 访问演示页面
open http://localhost:8080
```

## 📁 项目结构

```
SimpleIM/
└── im-system/              # 主项目目录
    ├── cmd/gateway/        # 服务入口
    ├── internal/           # 内部实现
    │   ├── gateway/        # 网关核心
    │   ├── service/        # 业务服务
    │   ├── handler/        # HTTP 接口
    │   └── model/          # 数据模型
    ├── pkg/                # 公共包
    ├── web/                # 前端演示页面
    ├── deploy/             # 部署配置
    ├── docs/               # 文档
    │   ├── blog.md         # 技术博文
    │   └── design.md       # 详细设计文档
    ├── Makefile            # 构建脚本
    └── README.md
```

## 📖 文档

- [README](im-system/README.md) - 项目说明和使用指南
- [技术博文](im-system/docs/blog.md) - 从零构建千人在线IM系统
- [设计文档](im-system/docs/design.md) - 完整的系统设计文档

## 🛠️ 技术栈

- **后端**: Go 1.21+ / Gin / gorilla/websocket
- **存储**: MySQL 8.0 / Redis 7 / MinIO
- **部署**: Docker / Docker Compose

## 📄 License

MIT License