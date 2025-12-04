// Package database 数据库初始化
package database

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/go-redis/redis/v8"
)

// RedisConfig Redis配置
type RedisConfig struct {
	Host         string
	Port         int
	Password     string
	DB           int
	PoolSize     int
	MinIdleConns int
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

// DefaultRedisConfig 默认Redis配置
func DefaultRedisConfig() *RedisConfig {
	return &RedisConfig{
		Host:         "localhost",
		Port:         6379,
		Password:     "",
		DB:           0,
		PoolSize:     100,
		MinIdleConns: 10,
		DialTimeout:  10 * time.Second,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}
}

// NewRedis 创建Redis连接
func NewRedis(config *RedisConfig) (*redis.Client, error) {
	if config == nil {
		config = DefaultRedisConfig()
	}

	// Apply defaults for zero values
	dialTimeout := config.DialTimeout
	if dialTimeout == 0 {
		dialTimeout = 10 * time.Second
	}
	readTimeout := config.ReadTimeout
	if readTimeout == 0 {
		readTimeout = 5 * time.Second
	}
	writeTimeout := config.WriteTimeout
	if writeTimeout == 0 {
		writeTimeout = 5 * time.Second
	}
	poolSize := config.PoolSize
	if poolSize == 0 {
		poolSize = 100
	}
	minIdleConns := config.MinIdleConns
	if minIdleConns == 0 {
		minIdleConns = 10
	}

	addr := fmt.Sprintf("%s:%d", config.Host, config.Port)
	log.Printf("Connecting to Redis at %s...", addr)

	client := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     config.Password,
		DB:           config.DB,
		PoolSize:     poolSize,
		MinIdleConns: minIdleConns,
		DialTimeout:  dialTimeout,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
	})

	// Use the dial timeout for the ping context
	ctx, cancel := context.WithTimeout(context.Background(), dialTimeout)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	log.Printf("Successfully connected to Redis at %s", addr)
	return client, nil
}

// RegisterNode 注册节点到Redis
func RegisterNode(ctx context.Context, client *redis.Client, nodeID string) error {
	nodesKey := "im:nodes"
	if err := client.SAdd(ctx, nodesKey, nodeID).Err(); err != nil {
		return fmt.Errorf("failed to register node: %w", err)
	}

	// 设置节点信息
	nodeInfoKey := fmt.Sprintf("im:node:info:%s", nodeID)
	if err := client.HSet(ctx, nodeInfoKey,
		"start_time", time.Now().Format(time.RFC3339),
		"status", "running",
	).Err(); err != nil {
		return fmt.Errorf("failed to set node info: %w", err)
	}

	client.Expire(ctx, nodeInfoKey, 24*time.Hour)
	return nil
}

// UnregisterNode 注销节点
func UnregisterNode(ctx context.Context, client *redis.Client, nodeID string) error {
	nodesKey := "im:nodes"
	client.SRem(ctx, nodesKey, nodeID)

	nodeInfoKey := fmt.Sprintf("im:node:info:%s", nodeID)
	client.Del(ctx, nodeInfoKey)
	return nil
}
