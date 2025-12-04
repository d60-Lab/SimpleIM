// Package database MongoDB连接
package database

import (
	"context"
	"fmt"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

// MongoConfig MongoDB配置
type MongoConfig struct {
	URI            string
	Database       string
	ConnectTimeout time.Duration
	MaxPoolSize    uint64
	MinPoolSize    uint64
}

// DefaultMongoConfig 默认MongoDB配置
func DefaultMongoConfig() *MongoConfig {
	return &MongoConfig{
		URI:            "mongodb://localhost:27017",
		Database:       "im_db",
		ConnectTimeout: 10 * time.Second,
		MaxPoolSize:    100,
		MinPoolSize:    10,
	}
}

// MongoClient MongoDB客户端封装
type MongoClient struct {
	client   *mongo.Client
	database *mongo.Database
}

// NewMongoDB 创建MongoDB连接
func NewMongoDB(config *MongoConfig) (*MongoClient, error) {
	if config == nil {
		config = DefaultMongoConfig()
	}

	// Apply defaults for zero values
	connectTimeout := config.ConnectTimeout
	if connectTimeout == 0 {
		connectTimeout = 10 * time.Second
	}
	maxPoolSize := config.MaxPoolSize
	if maxPoolSize == 0 {
		maxPoolSize = 100
	}
	minPoolSize := config.MinPoolSize
	if minPoolSize == 0 {
		minPoolSize = 10
	}

	log.Printf("Connecting to MongoDB at %s (database: %s)...", config.URI, config.Database)

	ctx, cancel := context.WithTimeout(context.Background(), connectTimeout)
	defer cancel()

	// 设置客户端选项
	clientOptions := options.Client().
		ApplyURI(config.URI).
		SetMaxPoolSize(maxPoolSize).
		SetMinPoolSize(minPoolSize).
		SetConnectTimeout(connectTimeout).
		SetServerSelectionTimeout(connectTimeout)

	// 连接MongoDB
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to mongodb: %w", err)
	}

	// 验证连接
	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		return nil, fmt.Errorf("failed to ping mongodb: %w", err)
	}

	log.Printf("Successfully connected to MongoDB (database: %s)", config.Database)

	return &MongoClient{
		client:   client,
		database: client.Database(config.Database),
	}, nil
}

// Client 获取原始MongoDB客户端
func (m *MongoClient) Client() *mongo.Client {
	return m.client
}

// Database 获取数据库
func (m *MongoClient) Database() *mongo.Database {
	return m.database
}

// Collection 获取集合
func (m *MongoClient) Collection(name string) *mongo.Collection {
	return m.database.Collection(name)
}

// Close 关闭连接
func (m *MongoClient) Close(ctx context.Context) error {
	if m.client != nil {
		return m.client.Disconnect(ctx)
	}
	return nil
}

// CreateIndexes 创建索引
func (m *MongoClient) CreateIndexes(ctx context.Context, collection string, indexes []mongo.IndexModel) error {
	_, err := m.Collection(collection).Indexes().CreateMany(ctx, indexes)
	if err != nil {
		return fmt.Errorf("failed to create indexes: %w", err)
	}
	return nil
}
