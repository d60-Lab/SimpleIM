// Package app 应用初始化
package app

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"gorm.io/gorm"

	_ "github.com/d60-lab/im-system/docs" // swagger docs
	"github.com/d60-lab/im-system/internal/gateway"
	"github.com/d60-lab/im-system/internal/handler"
	"github.com/d60-lab/im-system/internal/model"
	"github.com/d60-lab/im-system/internal/repository"
	"github.com/d60-lab/im-system/internal/service"
	"github.com/d60-lab/im-system/pkg/auth"
	"github.com/d60-lab/im-system/pkg/database"
)

// Server 应用服务器
type Server struct {
	config      *Config
	db          *gorm.DB
	redis       *redis.Client
	mongo       *database.MongoClient
	engine      *gin.Engine
	httpServer  *http.Server
	connManager *gateway.ConnectionManager
	dispatcher  gateway.MessageDispatcher
	messageRepo repository.MessageRepository
}

// NewServer 创建服务器
func NewServer(config *Config) (*Server, error) {
	// 初始化MySQL
	mysqlConfig := &database.MySQLConfig{
		Host:     config.MySQLHost,
		Port:     config.MySQLPort,
		User:     config.MySQLUser,
		Password: config.MySQLPassword,
		Database: config.MySQLDatabase,
	}
	db, err := database.NewMySQL(mysqlConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MySQL: %w", err)
	}
	log.Println("Connected to MySQL")

	// 自动迁移表结构
	if err := database.AutoMigrate(db,
		&model.User{},
		&model.Group{},
		&model.GroupMember{},
		&model.Message{},
		&model.OfflineMessage{},
		&model.Conversation{},
		&model.UserConversation{},
		&model.Device{},
		&model.File{},
	); err != nil {
		return nil, fmt.Errorf("failed to auto migrate: %w", err)
	}

	// 初始化Redis
	redisConfig := &database.RedisConfig{
		Host:     config.RedisHost,
		Port:     config.RedisPort,
		Password: config.RedisPassword,
		DB:       config.RedisDB,
	}
	redisClient, err := database.NewRedis(redisConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}
	log.Println("Connected to Redis")

	// 初始化MongoDB
	mongoConfig := &database.MongoConfig{
		URI:      config.MongoURI,
		Database: config.MongoDatabase,
	}
	mongoClient, err := database.NewMongoDB(mongoConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MongoDB: %w", err)
	}
	log.Println("Connected to MongoDB")

	// 创建消息仓库
	messageRepo := repository.NewMessageRepository(mongoClient)

	// 确保MongoDB索引
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := messageRepo.EnsureIndexes(ctx); err != nil {
		log.Printf("Warning: Failed to ensure MongoDB indexes: %v", err)
	}

	return &Server{
		config:      config,
		db:          db,
		redis:       redisClient,
		mongo:       mongoClient,
		messageRepo: messageRepo,
	}, nil
}

// Setup 初始化服务器组件
func (s *Server) Setup() error {
	// 初始化JWT管理器
	jwtConfig := &auth.JWTConfig{
		Secret:        s.config.JWTSecret,
		Issuer:        "im-system",
		Expire:        s.config.JWTExpire,
		RefreshExpire: s.config.JWTRefreshExp,
	}
	jwtManager := auth.NewJWTManager(jwtConfig)
	auth.InitDefaultManager(jwtConfig)

	// 初始化连接管理器
	connConfig := &gateway.ConnectionConfig{
		PingInterval: s.config.PingInterval,
		PongTimeout:  s.config.PongTimeout,
	}
	s.connManager = gateway.NewConnectionManager(s.config.NodeID, connConfig)

	// 初始化服务
	offlineService := service.NewOfflineService(s.db, s.redis, nil)
	offlineHandler := service.NewOfflineMessageHandler(offlineService)

	// 初始化消息分发器
	dispatcherConfig := &gateway.DispatcherConfig{
		NodeID:               s.config.NodeID,
		OnlineKeyExpire:      s.config.PongTimeout * 2,
		PublishChannelPrefix: "im:node:",
	}

	groupMemberGetter := &groupMemberGetterAdapter{}
	s.dispatcher = gateway.NewMessageDispatcher(
		dispatcherConfig,
		s.redis,
		groupMemberGetter,
		offlineHandler,
	)

	// 初始化群组服务
	groupService := service.NewGroupService(s.db, s.redis, &messageDispatcherAdapter{dispatcher: s.dispatcher})
	groupMemberGetter.groupService = groupService

	// 初始化消息服务（使用MongoDB）
	messageService := service.NewMessageService(s.messageRepo, groupService)
	messageSaver := &messageSaverAdapter{messageService: messageService}

	// 初始化文件存储服务
	storageConfig := &service.StorageConfig{
		Provider:  "minio",
		Endpoint:  s.config.MinioEndpoint,
		AccessKey: s.config.MinioAccessKey,
		SecretKey: s.config.MinioSecretKey,
		Bucket:    s.config.MinioBucket,
		UseSSL:    s.config.MinioUseSSL,
	}
	fileService, err := service.NewMinioStorageService(storageConfig, s.db, s.redis)
	if err != nil {
		log.Printf("Warning: Failed to initialize file storage service: %v", err)
		fileService = nil
	} else {
		log.Println("File storage service initialized")
	}

	// 初始化WebSocket处理器
	handlerConfig := &gateway.HandlerConfig{
		NodeID:       s.config.NodeID,
		PingInterval: s.config.PingInterval,
		PongTimeout:  s.config.PongTimeout,
	}
	wsHandler := gateway.NewWebSocketHandler(handlerConfig, s.connManager, s.dispatcher, jwtManager, messageSaver)

	// 创建Gin引擎
	gin.SetMode(gin.ReleaseMode)
	s.engine = gin.New()
	s.engine.Use(gin.Recovery())
	s.engine.Use(gin.Logger())

	// 注册路由
	s.registerRoutes(wsHandler, groupService, offlineService, messageService, fileService, jwtManager)

	// 创建HTTP服务器
	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      s.engine,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	return nil
}

// registerRoutes 注册所有路由
func (s *Server) registerRoutes(
	wsHandler *gateway.WebSocketHandler,
	groupService service.GroupService,
	offlineService service.OfflineService,
	messageService service.MessageService,
	fileService service.FileStorageService,
	jwtManager *auth.JWTManager,
) {
	// WebSocket路由
	wsHandler.RegisterRoutes(s.engine)

	// 群组API
	groupHandler := handler.NewGroupHandler(groupService)
	groupHandler.RegisterRoutes(s.engine)

	// 离线消息API
	offlineAPIHandler := handler.NewOfflineHandler(offlineService)
	offlineAPIHandler.RegisterRoutes(s.engine)

	// 用户API
	userHandler := handler.NewUserHandler(s.db, jwtManager)
	userHandler.RegisterRoutes(s.engine)

	// 消息历史API
	messageHandler := handler.NewMessageHandler(messageService)
	messageHandler.RegisterRoutes(s.engine.Group("/api", handler.AuthMiddleware()))

	// 文件上传API
	if fileService != nil {
		fileHandler := handler.NewFileHandler(fileService)
		fileHandler.RegisterRoutes(s.engine)
	}

	// Swagger文档
	s.engine.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// 静态文件服务
	s.setupStaticFiles()
}

// setupStaticFiles 设置静态文件服务
func (s *Server) setupStaticFiles() {
	candidates := []string{
		"web",
		"/app/web",
		filepath.Join(filepath.Dir(executablePath()), "web"),
		filepath.Join(filepath.Dir(executablePath()), "..", "web"),
	}

	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			log.Printf("Found web directory at: %s", candidate)
			s.engine.Static("/web", candidate)
			return
		}
	}

	log.Println("Warning: web directory not found, static file serving disabled")
}

// executablePath 获取可执行文件路径
func executablePath() string {
	p, _ := os.Executable()
	return p
}

// Run 启动服务器
func (s *Server) Run(ctx context.Context) error {
	// 启动后台任务
	s.startBackgroundTasks(ctx)

	// 启动HTTP服务器
	log.Printf("IM Gateway listening on %s", s.httpServer.Addr)
	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	return nil
}

// startBackgroundTasks 启动后台任务
func (s *Server) startBackgroundTasks(ctx context.Context) {
	// 启动指标服务
	go s.startMetricsServer()

	// 启动消息订阅
	if err := s.dispatcher.SubscribeNodeMessages(ctx); err != nil {
		log.Printf("Warning: Failed to subscribe node messages: %v", err)
	}

	// 启动心跳检查
	go s.connManager.StartHeartbeatChecker(ctx, time.Minute, s.config.PongTimeout*2)

	// 注册节点
	if err := database.RegisterNode(ctx, s.redis, s.config.NodeID); err != nil {
		log.Printf("Warning: Failed to register node: %v", err)
	}
}

// startMetricsServer 启动指标服务器
func (s *Server) startMetricsServer() {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	addr := fmt.Sprintf(":%d", s.config.MetricsPort)
	log.Printf("Metrics server listening on %s", addr)

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Printf("Metrics server error: %v", err)
	}
}

// Shutdown 优雅关闭服务器
func (s *Server) Shutdown(ctx context.Context) error {
	log.Println("Shutting down server...")

	// 注销节点
	if err := database.UnregisterNode(ctx, s.redis, s.config.NodeID); err != nil {
		log.Printf("Warning: Failed to unregister node: %v", err)
	}

	// 关闭所有连接
	s.connManager.CloseAll()

	// 关闭分发器
	s.dispatcher.Close()

	// 关闭HTTP服务器
	if err := s.httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shutdown http server: %w", err)
	}

	// 关闭MongoDB连接
	if s.mongo != nil {
		if err := s.mongo.Close(ctx); err != nil {
			log.Printf("Warning: Failed to close MongoDB connection: %v", err)
		}
	}

	log.Println("Server exited")
	return nil
}

// Config 获取配置
func (s *Server) Config() *Config {
	return s.config
}

// DB 获取数据库连接
func (s *Server) DB() *gorm.DB {
	return s.db
}

// Redis 获取Redis客户端
func (s *Server) Redis() *redis.Client {
	return s.redis
}

// Engine 获取Gin引擎
func (s *Server) Engine() *gin.Engine {
	return s.engine
}

// ConnManager 获取连接管理器
func (s *Server) ConnManager() *gateway.ConnectionManager {
	return s.connManager
}

// Dispatcher 获取消息分发器
func (s *Server) Dispatcher() gateway.MessageDispatcher {
	return s.dispatcher
}

// Mongo 获取MongoDB客户端
func (s *Server) Mongo() *database.MongoClient {
	return s.mongo
}

// MessageRepo 获取消息仓库
func (s *Server) MessageRepo() repository.MessageRepository {
	return s.messageRepo
}
