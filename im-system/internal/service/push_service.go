// Package service 提供业务逻辑服务
package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/d60-lab/im-system/internal/model"
	"github.com/d60-lab/im-system/pkg/util"
	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

// 推送服务错误定义
var (
	ErrDeviceNotFound    = errors.New("device not found")
	ErrInvalidPlatform   = errors.New("invalid platform")
	ErrPushFailed        = errors.New("push notification failed")
	ErrPushDisabled      = errors.New("push notification disabled")
	ErrInvalidToken      = errors.New("invalid device token")
	ErrRateLimitExceeded = errors.New("push rate limit exceeded")
)

// PushService 推送服务接口
type PushService interface {
	// 推送操作
	PushToUser(ctx context.Context, userID string, notification *model.PushNotification) error
	PushToUsers(ctx context.Context, userIDs []string, notification *model.PushNotification) (*model.BatchPushResult, error)
	PushToDevice(ctx context.Context, device *model.Device, notification *model.PushNotification) (*model.PushResult, error)

	// 设备管理
	RegisterDevice(ctx context.Context, userID string, req *model.RegisterDeviceRequest) error
	UnregisterDevice(ctx context.Context, userID, deviceToken string) error
	GetUserDevices(ctx context.Context, userID string) ([]*model.Device, error)
	UpdateDeviceToken(ctx context.Context, oldToken, newToken string) error

	// 推送Worker
	StartPushWorker(ctx context.Context) error
	StopPushWorker() error

	// 统计
	GetPushStats(ctx context.Context) (*PushStats, error)
}

// APNsClient APNs客户端接口
type APNsClient interface {
	Push(ctx context.Context, deviceToken string, notification *model.PushNotification) error
}

// FCMClient FCM客户端接口
type FCMClient interface {
	Push(ctx context.Context, deviceToken string, notification *model.PushNotification) error
	PushMulticast(ctx context.Context, tokens []string, notification *model.PushNotification) ([]error, error)
}

// PushConfig 推送配置
type PushConfig struct {
	WorkerCount     int           // Worker数量
	BatchSize       int           // 批量推送大小
	MaxRetries      int           // 最大重试次数
	RetryDelay      time.Duration // 重试延迟
	MergeEnabled    bool          // 是否启用推送合并
	MergeWindow     time.Duration // 合并窗口
	QueueSize       int           // 队列大小
	RateLimitPerSec int           // 每秒限制推送数
}

// DefaultPushConfig 默认推送配置
func DefaultPushConfig() *PushConfig {
	return &PushConfig{
		WorkerCount:     10,
		BatchSize:       100,
		MaxRetries:      3,
		RetryDelay:      5 * time.Second,
		MergeEnabled:    true,
		MergeWindow:     5 * time.Second,
		QueueSize:       10000,
		RateLimitPerSec: 1000,
	}
}

// PushStats 推送统计
type PushStats struct {
	TotalPushed   int64     `json:"total_pushed"`
	TotalSuccess  int64     `json:"total_success"`
	TotalFailed   int64     `json:"total_failed"`
	PendingCount  int64     `json:"pending_count"`
	IOSCount      int64     `json:"ios_count"`
	AndroidCount  int64     `json:"android_count"`
	LastPushTime  time.Time `json:"last_push_time"`
	AvgLatencyMs  float64   `json:"avg_latency_ms"`
	InvalidTokens int64     `json:"invalid_tokens"`
}

// pushServiceImpl 推送服务实现
type pushServiceImpl struct {
	config         *PushConfig
	db             *gorm.DB
	redis          *redis.Client
	apnsClient     APNsClient
	fcmClient      FCMClient
	offlineService PushOfflineService

	// 推送队列
	pushQueue chan *PushTask
	stopChan  chan struct{}
	wg        sync.WaitGroup

	// 统计
	stats   *PushStats
	statsMu sync.RWMutex

	// 状态
	running bool
	runMu   sync.Mutex
}

// PushTask 推送任务
type PushTask struct {
	ID           string
	UserID       string
	Devices      []*model.Device
	Notification *model.PushNotification
	Retries      int
	CreatedAt    time.Time
	ScheduledAt  time.Time
}

// NewPushService 创建推送服务
func NewPushService(
	config *PushConfig,
	db *gorm.DB,
	redisClient *redis.Client,
	apnsClient APNsClient,
	fcmClient FCMClient,
	offlineService PushOfflineService,
) PushService {
	if config == nil {
		config = DefaultPushConfig()
	}

	return &pushServiceImpl{
		config:         config,
		db:             db,
		redis:          redisClient,
		apnsClient:     apnsClient,
		fcmClient:      fcmClient,
		offlineService: offlineService,
		pushQueue:      make(chan *PushTask, config.QueueSize),
		stopChan:       make(chan struct{}),
		stats:          &PushStats{},
	}
}

// RegisterDevice 注册设备
func (s *pushServiceImpl) RegisterDevice(ctx context.Context, userID string, req *model.RegisterDeviceRequest) error {
	if req.DeviceToken == "" {
		return ErrInvalidToken
	}

	// 验证平台
	if req.Platform != model.PlatformIOS && req.Platform != model.PlatformAndroid && req.Platform != model.PlatformWeb {
		return ErrInvalidPlatform
	}

	device := &model.Device{
		UserID:      userID,
		DeviceToken: req.DeviceToken,
		Platform:    req.Platform,
		AppVersion:  req.AppVersion,
		DeviceInfo:  req.DeviceInfo,
		PushEnabled: true,
		UpdatedAt:   time.Now(),
		CreatedAt:   time.Now(),
	}

	// 使用 upsert 操作
	result := s.db.WithContext(ctx).Where("device_token = ?", req.DeviceToken).
		Assign(model.Device{
			UserID:     userID,
			Platform:   req.Platform,
			AppVersion: req.AppVersion,
			DeviceInfo: req.DeviceInfo,
			UpdatedAt:  time.Now(),
		}).
		FirstOrCreate(device)

	if result.Error != nil {
		return fmt.Errorf("register device error: %w", result.Error)
	}

	// 缓存到Redis
	deviceKey := fmt.Sprintf("device:%s", userID)
	s.redis.SAdd(ctx, deviceKey, req.DeviceToken)
	s.redis.Expire(ctx, deviceKey, 30*24*time.Hour)

	return nil
}

// UnregisterDevice 注销设备
func (s *pushServiceImpl) UnregisterDevice(ctx context.Context, userID, deviceToken string) error {
	result := s.db.WithContext(ctx).
		Where("user_id = ? AND device_token = ?", userID, deviceToken).
		Delete(&model.Device{})

	if result.Error != nil {
		return fmt.Errorf("unregister device error: %w", result.Error)
	}

	// 从Redis删除
	deviceKey := fmt.Sprintf("device:%s", userID)
	s.redis.SRem(ctx, deviceKey, deviceToken)

	return nil
}

// GetUserDevices 获取用户设备列表
func (s *pushServiceImpl) GetUserDevices(ctx context.Context, userID string) ([]*model.Device, error) {
	var devices []*model.Device

	if err := s.db.WithContext(ctx).
		Where("user_id = ? AND push_enabled = ?", userID, true).
		Find(&devices).Error; err != nil {
		return nil, fmt.Errorf("get user devices error: %w", err)
	}

	return devices, nil
}

// UpdateDeviceToken 更新设备Token
func (s *pushServiceImpl) UpdateDeviceToken(ctx context.Context, oldToken, newToken string) error {
	result := s.db.WithContext(ctx).Model(&model.Device{}).
		Where("device_token = ?", oldToken).
		Update("device_token", newToken)

	if result.Error != nil {
		return fmt.Errorf("update device token error: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return ErrDeviceNotFound
	}

	return nil
}

// PushToUser 推送给单个用户
func (s *pushServiceImpl) PushToUser(ctx context.Context, userID string, notification *model.PushNotification) error {
	devices, err := s.GetUserDevices(ctx, userID)
	if err != nil {
		return err
	}

	if len(devices) == 0 {
		return nil // 用户没有注册设备，不需要推送
	}

	// 创建推送任务
	task := &PushTask{
		ID:           util.GenerateUUID(),
		UserID:       userID,
		Devices:      devices,
		Notification: notification,
		Retries:      0,
		CreatedAt:    time.Now(),
		ScheduledAt:  time.Now(),
	}

	// 加入推送队列
	select {
	case s.pushQueue <- task:
		return nil
	default:
		// 队列已满，直接推送
		return s.executePushTask(ctx, task)
	}
}

// PushToUsers 批量推送给多个用户
func (s *pushServiceImpl) PushToUsers(ctx context.Context, userIDs []string, notification *model.PushNotification) (*model.BatchPushResult, error) {
	result := &model.BatchPushResult{
		TotalCount: len(userIDs),
		Results:    make([]*model.PushResult, 0),
	}

	for _, userID := range userIDs {
		err := s.PushToUser(ctx, userID, notification)
		if err != nil {
			result.FailedCount++
			result.Results = append(result.Results, &model.PushResult{
				Success:     false,
				DeviceToken: "",
				Platform:    "",
				Error:       err.Error(),
			})
		} else {
			result.SuccessCount++
		}
	}

	return result, nil
}

// PushToDevice 推送到单个设备
func (s *pushServiceImpl) PushToDevice(ctx context.Context, device *model.Device, notification *model.PushNotification) (*model.PushResult, error) {
	result := &model.PushResult{
		DeviceToken: device.DeviceToken,
		Platform:    string(device.Platform),
	}

	var err error

	switch device.Platform {
	case model.PlatformIOS:
		if s.apnsClient != nil {
			err = s.apnsClient.Push(ctx, device.DeviceToken, notification)
		} else {
			err = errors.New("APNs client not configured")
		}
	case model.PlatformAndroid:
		if s.fcmClient != nil {
			err = s.fcmClient.Push(ctx, device.DeviceToken, notification)
		} else {
			err = errors.New("FCM client not configured")
		}
	default:
		err = ErrInvalidPlatform
	}

	if err != nil {
		result.Success = false
		result.Error = err.Error()

		// 检查是否为无效Token错误
		if isInvalidTokenError(err) {
			result.InvalidToken = true
			// 删除无效设备
			s.db.WithContext(ctx).Where("device_token = ?", device.DeviceToken).Delete(&model.Device{})
			s.updateStats(func(stats *PushStats) {
				stats.InvalidTokens++
			})
		}

		return result, err
	}

	result.Success = true
	return result, nil
}

// StartPushWorker 启动推送Worker
func (s *pushServiceImpl) StartPushWorker(ctx context.Context) error {
	s.runMu.Lock()
	if s.running {
		s.runMu.Unlock()
		return errors.New("push worker already running")
	}
	s.running = true
	s.runMu.Unlock()

	// 启动多个Worker
	for i := 0; i < s.config.WorkerCount; i++ {
		s.wg.Add(1)
		go s.pushWorker(ctx, i)
	}

	// 启动待推送消息处理
	s.wg.Add(1)
	go s.processPendingPush(ctx)

	log.Printf("Push worker started with %d workers", s.config.WorkerCount)
	return nil
}

// StopPushWorker 停止推送Worker
func (s *pushServiceImpl) StopPushWorker() error {
	s.runMu.Lock()
	if !s.running {
		s.runMu.Unlock()
		return nil
	}
	s.running = false
	s.runMu.Unlock()

	close(s.stopChan)
	s.wg.Wait()

	log.Println("Push worker stopped")
	return nil
}

// pushWorker 推送Worker协程
func (s *pushServiceImpl) pushWorker(ctx context.Context, workerID int) {
	defer s.wg.Done()

	log.Printf("Push worker %d started", workerID)

	for {
		select {
		case <-s.stopChan:
			log.Printf("Push worker %d stopping", workerID)
			return
		case <-ctx.Done():
			return
		case task, ok := <-s.pushQueue:
			if !ok {
				return
			}

			// 检查是否需要延迟执行
			if task.ScheduledAt.After(time.Now()) {
				time.Sleep(time.Until(task.ScheduledAt))
			}

			// 执行推送
			if err := s.executePushTask(ctx, task); err != nil {
				log.Printf("Push task %s failed: %v", task.ID, err)

				// 重试逻辑
				if task.Retries < s.config.MaxRetries {
					task.Retries++
					task.ScheduledAt = time.Now().Add(s.config.RetryDelay * time.Duration(task.Retries))

					select {
					case s.pushQueue <- task:
					default:
						log.Printf("Push queue full, dropping retry task %s", task.ID)
					}
				}
			}
		}
	}
}

// executePushTask 执行推送任务
func (s *pushServiceImpl) executePushTask(ctx context.Context, task *PushTask) error {
	start := time.Now()

	var lastErr error
	successCount := 0
	failedCount := 0

	for _, device := range task.Devices {
		result, err := s.PushToDevice(ctx, device, task.Notification)
		if err != nil {
			lastErr = err
			failedCount++
			log.Printf("Push to device %s failed: %v", device.DeviceToken, err)
		} else if result.Success {
			successCount++
		}
	}

	// 更新统计
	latency := time.Since(start).Milliseconds()
	s.updateStats(func(stats *PushStats) {
		stats.TotalPushed += int64(len(task.Devices))
		stats.TotalSuccess += int64(successCount)
		stats.TotalFailed += int64(failedCount)
		stats.LastPushTime = time.Now()

		// 计算平均延迟（简单移动平均）
		if stats.AvgLatencyMs == 0 {
			stats.AvgLatencyMs = float64(latency)
		} else {
			stats.AvgLatencyMs = (stats.AvgLatencyMs*0.9 + float64(latency)*0.1)
		}
	})

	if failedCount > 0 && successCount == 0 {
		return lastErr
	}

	return nil
}

// processPendingPush 处理待推送消息
func (s *pushServiceImpl) processPendingPush(ctx context.Context) {
	defer s.wg.Done()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopChan:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.processUnpushedMessages(ctx)
		}
	}
}

// processUnpushedMessages 处理未推送的离线消息
func (s *pushServiceImpl) processUnpushedMessages(ctx context.Context) {
	if s.offlineService == nil {
		return
	}

	// 获取所有用户的未推送消息（简化实现：遍历待处理队列）
	// 实际生产环境应该有更高效的方式获取待推送用户列表
	// 这里先跳过，由外部调用 PushToUser 触发推送
	return
}

// processUnpushedMessagesForUser 处理指定用户未推送的离线消息
func (s *pushServiceImpl) processUnpushedMessagesForUser(ctx context.Context, userID string) {
	if s.offlineService == nil {
		return
	}

	// 获取未推送的消息
	messages, err := s.offlineService.GetUnpushedMessages(ctx, userID, s.config.BatchSize)
	if err != nil {
		log.Printf("Get unpushed messages error: %v", err)
		return
	}

	// 按用户分组
	userMessages := make(map[string][]*model.OfflineMessage)
	for _, msg := range messages {
		userMessages[msg.UserID] = append(userMessages[msg.UserID], msg)
	}

	// 为每个用户发送推送
	notification := s.buildNotification(messages)

	if err := s.PushToUser(ctx, userID, notification); err != nil {
		log.Printf("Push to user %s error: %v", userID, err)
		return
	}

	// 标记为已推送
	messageIDs := make([]string, len(messages))
	for i, msg := range messages {
		messageIDs[i] = msg.MessageID
	}

	if err := s.offlineService.MarkAsPushed(ctx, messageIDs); err != nil {
		log.Printf("Mark as pushed error: %v", err)
	}
}

// buildNotification 根据离线消息构建推送通知
func (s *pushServiceImpl) buildNotification(messages []*model.OfflineMessage) *model.PushNotification {
	if len(messages) == 0 {
		return nil
	}

	notification := &model.PushNotification{
		Sound:    "default",
		Badge:    len(messages),
		Priority: model.PushPriorityHigh,
	}

	if len(messages) == 1 {
		// 单条消息
		notification.Title = s.getNotificationTitle(messages[0])
		notification.Body = s.getNotificationBody(messages[0])
		notification.MessageID = messages[0].MessageID
		notification.ThreadID = messages[0].ConversationID
	} else {
		// 多条消息，合并显示
		notification.Title = "您有新消息"
		notification.Body = fmt.Sprintf("您有 %d 条未读消息", len(messages))
		notification.CollapseKey = "new_messages"
	}

	// 添加自定义数据
	notification.Data = map[string]string{
		"type":            "new_message",
		"conversation_id": messages[0].ConversationID,
		"count":           fmt.Sprintf("%d", len(messages)),
	}

	return notification
}

// getNotificationTitle 获取通知标题
func (s *pushServiceImpl) getNotificationTitle(msg *model.OfflineMessage) string {
	// 根据会话类型返回不同标题
	if len(msg.ConversationID) > 6 && msg.ConversationID[:6] == "group_" {
		return "群消息"
	}
	return "新消息"
}

// getNotificationBody 获取通知内容
func (s *pushServiceImpl) getNotificationBody(msg *model.OfflineMessage) string {
	// 这里可以解析消息内容，返回适当的预览文本
	// 为了简化，直接返回通用文本
	return "您收到一条新消息"
}

// updateStats 更新统计信息
func (s *pushServiceImpl) updateStats(fn func(*PushStats)) {
	s.statsMu.Lock()
	defer s.statsMu.Unlock()
	fn(s.stats)
}

// GetPushStats 获取推送统计
func (s *pushServiceImpl) GetPushStats(ctx context.Context) (*PushStats, error) {
	s.statsMu.RLock()
	defer s.statsMu.RUnlock()

	// 复制统计数据
	stats := &PushStats{
		TotalPushed:   s.stats.TotalPushed,
		TotalSuccess:  s.stats.TotalSuccess,
		TotalFailed:   s.stats.TotalFailed,
		LastPushTime:  s.stats.LastPushTime,
		AvgLatencyMs:  s.stats.AvgLatencyMs,
		InvalidTokens: s.stats.InvalidTokens,
	}

	// 获取队列中待处理数量
	stats.PendingCount = int64(len(s.pushQueue))

	// 获取设备统计
	var iosCount, androidCount int64
	s.db.WithContext(ctx).Model(&model.Device{}).Where("platform = ?", model.PlatformIOS).Count(&iosCount)
	s.db.WithContext(ctx).Model(&model.Device{}).Where("platform = ?", model.PlatformAndroid).Count(&androidCount)
	stats.IOSCount = iosCount
	stats.AndroidCount = androidCount

	return stats, nil
}

// isInvalidTokenError 检查是否为无效Token错误
func isInvalidTokenError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()
	invalidTokenErrors := []string{
		"InvalidToken",
		"BadDeviceToken",
		"Unregistered",
		"NotRegistered",
		"InvalidRegistration",
		"MismatchSenderId",
	}

	for _, e := range invalidTokenErrors {
		if contains(errStr, e) {
			return true
		}
	}

	return false
}

// contains 检查字符串是否包含子串
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsImpl(s, substr))
}

func containsImpl(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// PushOfflineService 离线消息服务接口（供推送服务使用）
type PushOfflineService interface {
	GetUnpushedMessages(ctx context.Context, userID string, limit int) ([]*model.OfflineMessage, error)
	MarkAsPushed(ctx context.Context, messageIDs []string) error
}

// MockAPNsClient APNs模拟客户端（用于测试）
type MockAPNsClient struct{}

func (c *MockAPNsClient) Push(ctx context.Context, deviceToken string, notification *model.PushNotification) error {
	log.Printf("[MockAPNs] Push to %s: %s", deviceToken, notification.Body)
	return nil
}

// MockFCMClient FCM模拟客户端（用于测试）
type MockFCMClient struct{}

func (c *MockFCMClient) Push(ctx context.Context, deviceToken string, notification *model.PushNotification) error {
	log.Printf("[MockFCM] Push to %s: %s", deviceToken, notification.Body)
	return nil
}

func (c *MockFCMClient) PushMulticast(ctx context.Context, tokens []string, notification *model.PushNotification) ([]error, error) {
	log.Printf("[MockFCM] Multicast push to %d devices: %s", len(tokens), notification.Body)
	return make([]error, len(tokens)), nil
}
