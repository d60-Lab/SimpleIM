// Package main Gateway服务入口
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/d60-lab/im-system/internal/app"
)

func main() {
	// 加载配置
	config := app.DefaultConfig()
	config.ParseFlags()

	log.Printf("Starting IM Gateway (NodeID: %s)...", config.NodeID)

	// 创建服务器
	server, err := app.NewServer(config)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	// 初始化服务器组件
	if err := server.Setup(); err != nil {
		log.Fatalf("Failed to setup server: %v", err)
	}

	// 启动服务器
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := server.Run(ctx); err != nil {
		log.Fatalf("Failed to run server: %v", err)
	}

	// 等待退出信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	// 优雅关闭
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}
}
