// Package app 应用初始化
package app

import (
	"flag"
	"os"
	"time"
)

// Config 应用配置
type Config struct {
	// 服务配置
	Host   string
	Port   int
	NodeID string

	// MySQL配置
	MySQLHost     string
	MySQLPort     int
	MySQLUser     string
	MySQLPassword string
	MySQLDatabase string

	// Redis配置
	RedisHost     string
	RedisPort     int
	RedisPassword string
	RedisDB       int

	// MongoDB配置
	MongoURI      string
	MongoDatabase string

	// MinIO配置
	MinioEndpoint  string
	MinioAccessKey string
	MinioSecretKey string
	MinioBucket    string
	MinioUseSSL    bool

	// JWT配置
	JWTSecret     string
	JWTExpire     time.Duration
	JWTRefreshExp time.Duration

	// WebSocket配置
	PingInterval time.Duration
	PongTimeout  time.Duration

	// 指标端口
	MetricsPort int
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		Host:           "0.0.0.0",
		Port:           8080,
		NodeID:         getEnv("NODE_ID", "node1"),
		MySQLHost:      getEnv("MYSQL_HOST", "localhost"),
		MySQLPort:      3306,
		MySQLUser:      getEnv("MYSQL_USER", "root"),
		MySQLPassword:  getEnv("MYSQL_PASSWORD", "password"),
		MySQLDatabase:  getEnv("MYSQL_DATABASE", "im_db"),
		RedisHost:      getEnv("REDIS_HOST", "localhost"),
		RedisPort:      6379,
		RedisPassword:  getEnv("REDIS_PASSWORD", ""),
		RedisDB:        0,
		MongoURI:       getEnv("MONGO_URI", "mongodb://localhost:27017"),
		MongoDatabase:  getEnv("MONGO_DATABASE", "im_db"),
		MinioEndpoint:  getEnv("MINIO_ENDPOINT", "localhost:9000"),
		MinioAccessKey: getEnv("MINIO_ACCESS_KEY", "minioadmin"),
		MinioSecretKey: getEnv("MINIO_SECRET_KEY", "minioadmin123"),
		MinioBucket:    getEnv("MINIO_BUCKET", "im-files"),
		MinioUseSSL:    getEnv("MINIO_USE_SSL", "false") == "true",
		JWTSecret:      getEnv("JWT_SECRET", "im-system-jwt-secret-key"),
		JWTExpire:      7 * 24 * time.Hour,
		JWTRefreshExp:  30 * 24 * time.Hour,
		PingInterval:   30 * time.Second,
		PongTimeout:    60 * time.Second,
		MetricsPort:    9090,
	}
}

// ParseFlags 解析命令行参数
func (c *Config) ParseFlags() {
	flag.StringVar(&c.Host, "host", c.Host, "Server host")
	flag.IntVar(&c.Port, "port", c.Port, "Server port")
	flag.StringVar(&c.NodeID, "node-id", c.NodeID, "Node ID")
	flag.StringVar(&c.MySQLHost, "mysql-host", c.MySQLHost, "MySQL host")
	flag.IntVar(&c.MySQLPort, "mysql-port", c.MySQLPort, "MySQL port")
	flag.StringVar(&c.MySQLUser, "mysql-user", c.MySQLUser, "MySQL user")
	flag.StringVar(&c.MySQLPassword, "mysql-password", c.MySQLPassword, "MySQL password")
	flag.StringVar(&c.MySQLDatabase, "mysql-database", c.MySQLDatabase, "MySQL database")
	flag.StringVar(&c.RedisHost, "redis-host", c.RedisHost, "Redis host")
	flag.IntVar(&c.RedisPort, "redis-port", c.RedisPort, "Redis port")
	flag.StringVar(&c.RedisPassword, "redis-password", c.RedisPassword, "Redis password")
	flag.StringVar(&c.MongoURI, "mongo-uri", c.MongoURI, "MongoDB URI")
	flag.StringVar(&c.MongoDatabase, "mongo-database", c.MongoDatabase, "MongoDB database")
	flag.StringVar(&c.MinioEndpoint, "minio-endpoint", c.MinioEndpoint, "MinIO endpoint")
	flag.StringVar(&c.MinioAccessKey, "minio-access-key", c.MinioAccessKey, "MinIO access key")
	flag.StringVar(&c.MinioSecretKey, "minio-secret-key", c.MinioSecretKey, "MinIO secret key")
	flag.StringVar(&c.MinioBucket, "minio-bucket", c.MinioBucket, "MinIO bucket")
	flag.IntVar(&c.MetricsPort, "metrics-port", c.MetricsPort, "Metrics port")
	flag.Parse()
}

// getEnv 获取环境变量，如果不存在返回默认值
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
