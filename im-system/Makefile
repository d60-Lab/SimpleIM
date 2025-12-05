# SimpleIM Makefile
# 用于管理 IM 系统的构建、启动、停止等操作

# 变量定义
PROJECT_NAME := imdemo
COMPOSE_FILE := deploy/docker-compose.yml
DOCKER_COMPOSE := docker-compose -f $(COMPOSE_FILE) -p $(PROJECT_NAME)

# 默认目标
.DEFAULT_GOAL := help

# 颜色定义
GREEN  := \033[0;32m
YELLOW := \033[0;33m
BLUE   := \033[0;34m
NC     := \033[0m # No Color

##@ 帮助

.PHONY: help
help: ## 显示帮助信息
	@echo ""
	@echo "$(BLUE)SimpleIM - 轻量级即时通讯系统$(NC)"
	@echo ""
	@echo "$(YELLOW)使用方法:$(NC)"
	@echo "  make <target>"
	@echo ""
	@awk 'BEGIN {FS = ":.*##"; printf ""} /^[a-zA-Z_-]+:.*?##/ { printf "  $(GREEN)%-15s$(NC) %s\n", $$1, $$2 } /^##@/ { printf "\n$(YELLOW)%s$(NC)\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ 开发

.PHONY: run
run: ## 本地运行 Gateway 服务
	@echo "$(GREEN)启动 Gateway 服务...$(NC)"
	go run cmd/gateway/main.go

.PHONY: build-go
build-go: ## 编译 Go 二进制文件
	@echo "$(GREEN)编译 Gateway...$(NC)"
	go build -o bin/gateway ./cmd/gateway
	@echo "$(GREEN)编译完成: bin/gateway$(NC)"

.PHONY: test
test: ## 运行测试
	@echo "$(GREEN)运行测试...$(NC)"
	go test -v ./...

.PHONY: swagger
swagger: ## 生成 Swagger 文档
	@echo "$(GREEN)生成 Swagger 文档...$(NC)"
	swag init -g cmd/gateway/main.go -o docs --parseDependency --parseInternal
	@echo "$(GREEN)Swagger 文档已生成到 docs/ 目录$(NC)"
	@echo "$(YELLOW)访问 http://localhost:8080/swagger/index.html 查看文档$(NC)"

.PHONY: swagger-install
swagger-install: ## 安装 swag 工具
	@echo "$(GREEN)安装 swag 工具...$(NC)"
	go install github.com/swaggo/swag/cmd/swag@latest

.PHONY: lint
lint: ## 代码检查
	@echo "$(GREEN)运行代码检查...$(NC)"
	golangci-lint run ./...

.PHONY: fmt
fmt: ## 格式化代码
	@echo "$(GREEN)格式化代码...$(NC)"
	go fmt ./...

.PHONY: tidy
tidy: ## 整理依赖
	@echo "$(GREEN)整理 Go 依赖...$(NC)"
	go mod tidy

##@ Docker

.PHONY: build
build: ## 构建 Docker 镜像
	@echo "$(GREEN)构建 Docker 镜像...$(NC)"
	$(DOCKER_COMPOSE) build

.PHONY: build-gateway
build-gateway: ## 仅构建 Gateway 镜像
	@echo "$(GREEN)构建 Gateway 镜像...$(NC)"
	$(DOCKER_COMPOSE) build im-gateway

.PHONY: rebuild
rebuild: ## 重新构建并启动 Gateway
	@echo "$(GREEN)重新构建 Gateway...$(NC)"
	$(DOCKER_COMPOSE) build im-gateway
	$(DOCKER_COMPOSE) up -d im-gateway
	@echo "$(GREEN)Gateway 已重新部署$(NC)"

##@ 服务管理

.PHONY: up
up: ## 启动所有服务
	@echo "$(GREEN)启动所有服务...$(NC)"
	$(DOCKER_COMPOSE) up -d
	@echo ""
	@echo "$(GREEN)服务已启动!$(NC)"
	@echo "  - Web Demo: http://localhost:8080/web/chat.html"
	@echo "  - MinIO 控制台: http://localhost:9001"
	@echo ""

.PHONY: down
down: ## 停止所有服务
	@echo "$(YELLOW)停止所有服务...$(NC)"
	$(DOCKER_COMPOSE) down

.PHONY: restart
restart: down up ## 重启所有服务

.PHONY: deps
deps: ## 仅启动依赖服务 (MySQL, Redis, MinIO)
	@echo "$(GREEN)启动依赖服务...$(NC)"
	$(DOCKER_COMPOSE) up -d mysql redis minio
	@echo ""
	@echo "$(GREEN)依赖服务已启动!$(NC)"
	@echo "  - MySQL: localhost:13306"
	@echo "  - Redis: localhost:16379"
	@echo "  - MinIO: localhost:9000"
	@echo ""

.PHONY: stop-deps
stop-deps: ## 停止依赖服务
	@echo "$(YELLOW)停止依赖服务...$(NC)"
	$(DOCKER_COMPOSE) stop mysql redis minio

##@ 监控

.PHONY: ps
ps: ## 查看服务状态
	@$(DOCKER_COMPOSE) ps

.PHONY: logs
logs: ## 查看所有日志
	$(DOCKER_COMPOSE) logs -f

.PHONY: logs-gateway
logs-gateway: ## 查看 Gateway 日志
	$(DOCKER_COMPOSE) logs -f im-gateway

.PHONY: logs-mysql
logs-mysql: ## 查看 MySQL 日志
	$(DOCKER_COMPOSE) logs -f mysql

.PHONY: logs-redis
logs-redis: ## 查看 Redis 日志
	$(DOCKER_COMPOSE) logs -f redis

.PHONY: stats
stats: ## 查看容器资源使用
	@docker stats --no-stream $(shell $(DOCKER_COMPOSE) ps -q)

.PHONY: health
health: ## 健康检查
	@echo "$(GREEN)检查服务健康状态...$(NC)"
	@curl -s http://localhost:8080/health | jq . || echo "Gateway 未响应"

##@ 清理

.PHONY: clean
clean: ## 停止并删除所有容器、网络、卷
	@echo "$(YELLOW)清理所有资源...$(NC)"
	$(DOCKER_COMPOSE) down -v --remove-orphans
	@echo "$(GREEN)清理完成$(NC)"

.PHONY: clean-images
clean-images: clean ## 同时删除镜像
	@echo "$(YELLOW)删除镜像...$(NC)"
	$(DOCKER_COMPOSE) down --rmi local

.PHONY: clean-all
clean-all: clean-images ## 彻底清理（包括构建缓存）
	@echo "$(YELLOW)清理构建缓存...$(NC)"
	docker builder prune -f
	rm -rf bin/

##@ 数据库

.PHONY: mysql-cli
mysql-cli: ## 连接 MySQL 命令行
	@echo "$(GREEN)连接 MySQL...$(NC)"
	docker exec -it $(PROJECT_NAME)-mysql-1 mysql -uim_user -pim_password im_db

.PHONY: redis-cli
redis-cli: ## 连接 Redis 命令行
	@echo "$(GREEN)连接 Redis...$(NC)"
	docker exec -it $(PROJECT_NAME)-redis-1 redis-cli

.PHONY: mongo-cli
mongo-cli: ## 连接 MongoDB 命令行
	@echo "$(GREEN)连接 MongoDB...$(NC)"
	docker exec -it $(PROJECT_NAME)-mongodb-1 mongosh -u root -p password123 --authenticationDatabase admin im_db

##@ 多节点部署

.PHONY: up-multi
up-multi: ## 启动多节点模式
	@echo "$(GREEN)启动多节点模式...$(NC)"
	$(DOCKER_COMPOSE) --profile multi-node up -d
	@echo "$(GREEN)多节点模式已启动$(NC)"
	@echo "  - Node 1: http://localhost:8080"
	@echo "  - Node 2: http://localhost:8081"

.PHONY: up-monitoring
up-monitoring: ## 启动监控服务 (Prometheus + Grafana)
	@echo "$(GREEN)启动监控服务...$(NC)"
	$(DOCKER_COMPOSE) --profile monitoring up -d
	@echo "$(GREEN)监控服务已启动$(NC)"
	@echo "  - Prometheus: http://localhost:9092"
	@echo "  - Grafana: http://localhost:3000 (admin/admin)"

##@ 快捷方式

.PHONY: dev
dev: deps ## 开发模式：启动依赖 + 本地运行
	@sleep 3
	@$(MAKE) run

.PHONY: demo
demo: up ## 演示模式：启动所有服务并打开浏览器
	@sleep 2
	@open http://localhost:8080/web/chat.html 2>/dev/null || xdg-open http://localhost:8080/web/chat.html 2>/dev/null || echo "请手动打开: http://localhost:8080/web/chat.html"
