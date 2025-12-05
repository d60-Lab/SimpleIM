# IM Gateway 服务 Dockerfile
# 多阶段构建

# ==================== 前端构建阶段 ====================
FROM node:20-alpine AS frontend-builder

WORKDIR /app/web/chat-app

# 复制前端依赖文件
COPY web/chat-app/package*.json ./

# 安装依赖
RUN npm ci

# 复制前端源码
COPY web/chat-app/ ./

# 构建前端
RUN npm run build

# ==================== 后端构建阶段 ====================
FROM golang:1.21-alpine AS backend-builder

# 设置工作目录
WORKDIR /app

# 安装必要的构建工具
RUN apk add --no-cache git ca-certificates tzdata

# 设置Go环境变量
ENV GO111MODULE=on \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64 \
    GOPROXY=https://goproxy.cn,direct

# 复制go.mod和go.sum，先下载依赖（利用缓存）
COPY go.mod go.sum ./
RUN go mod download

# 复制源代码
COPY . .

# 构建应用
RUN go build -ldflags="-s -w" -o /app/bin/gateway ./cmd/gateway

# ==================== 运行阶段 ====================
FROM alpine:3.19

# 设置工作目录
WORKDIR /app

# 安装运行时依赖
RUN apk add --no-cache ca-certificates tzdata curl

# 设置时区
ENV TZ=Asia/Shanghai

# 从后端构建阶段复制二进制文件
COPY --from=backend-builder /app/bin/gateway /app/gateway

# 从前端构建阶段复制构建产物
COPY --from=frontend-builder /app/web/chat-app/dist /app/web/chat-app/dist

# 创建非root用户
RUN addgroup -g 1000 -S appgroup && \
    adduser -u 1000 -S appuser -G appgroup && \
    chown -R appuser:appgroup /app

# 切换到非root用户
USER appuser

# 暴露端口
# 8080: HTTP/WebSocket服务端口
# 9090: Prometheus指标端口
EXPOSE 8080 9090

# 健康检查
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8080/health || exit 1

# 启动命令
ENTRYPOINT ["/app/gateway"]

# 默认参数（可被docker run覆盖）
CMD ["--host", "0.0.0.0", "--port", "8080"]
