// Package service 提供业务逻辑服务
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/d60-lab/im-system/internal/model"
	"github.com/d60-lab/im-system/pkg/util"
	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

// 离线消息配置
const (
	DefaultOfflineMessageExpire = 7 * 24 * time.Hour // 默认离线消息过期时间：7天
	DefaultMaxOfflineMessages   = 1000               // 默认最大离线消息数
)

// 离线消息服务错误定义
var (
	ErrOfflineMessageNotFound = errors.New("offline message not found")
	ErrTooManyOfflineMessages = errors.New("too many offline messages")
)

// OfflineServiceConfig 离线消息服务配置
type OfflineServiceConfig struct {
	MaxMessages   int           // 每用户最大离线消息数
	ExpireDays    int           // 过期天数
	CleanInterval time.Duration // 清理任务间隔
}

// DefaultOfflineServiceConfig 默认配置
func DefaultOfflineServiceConfig() *OfflineServiceConfig {
	return &OfflineServiceConfig{
		MaxMessages:   1000,
		ExpireDays:    7,
		CleanInterval: time.Hour,
	}
}

// OfflineService 离线消息服务接口
type OfflineService interface {
	// SaveOfflineMessage 保存离线消息
	SaveOfflineMessage(ctx context.Context, userID string, msg *model.Message) error

	// PullOfflineMessages 拉取离线消息
	PullOfflineMessages(ctx context.Context, userID string, lastSeq int64, limit int) ([]*model.OfflineMessage, error)

	// MarkAsPushed 标记消息已推送
	MarkAsPushed(ctx context.Context, messageIDs []string) error

	// DeleteOfflineMessages 删除离线消息
	DeleteOfflineMessages(ctx context.Context, userID string, messageIDs []string) error

	// GetUnpushedMessages 获取未推送的消息
	GetUnpushedMessages(ctx context.Context, userID string, limit int) ([]*model.OfflineMessage, error)

	// CleanExpiredMessages 清理过期消息
	CleanExpiredMessages(ctx context.Context) (int64, error)

	// GetOfflineMessageCount 获取离线消息数量
	GetOfflineMessageCount(ctx context.Context, userID string) (int64, error)

	// StartCleanupTask 启动清理任务
	StartCleanupTask(ctx context.Context)
}

// offlineServiceImpl 离线消息服务实现
type offlineServiceImpl struct {
	db     *gorm.DB
	redis  *redis.Client
	config *OfflineServiceConfig
}

// NewOfflineService 创建离线消息服务
func NewOfflineService(db *gorm.DB, redisClient *redis.Client, config *OfflineServiceConfig) OfflineService {
	if config == nil {
		config = DefaultOfflineServiceConfig()
	}
	return &offlineServiceImpl{
		db:     db,
		redis:  redisClient,
		config: config,
	}
}

// SaveOfflineMessage 保存离线消息
func (s *offlineServiceImpl) SaveOfflineMessage(ctx context.Context, userID string, msg *model.Message) error {
	// 检查离线消息数量是否超限
	count, err := s.GetOfflineMessageCount(ctx, userID)
	if err != nil {
		return fmt.Errorf("get offline message count error: %w", err)
	}

	if count >= int64(s.config.MaxMessages) {
		// 删除最旧的消息
		if err := s.deleteOldestMessages(ctx, userID, int(count-int64(s.config.MaxMessages)+1)); err != nil {
			return fmt.Errorf("delete oldest messages error: %w", err)
		}
	}

	// 序列化消息内容
	contentBytes, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal message error: %w", err)
	}

	// 计算过期时间
	expireAt := time.Now().Add(time.Duration(s.config.ExpireDays) * 24 * time.Hour)

	// 创建离线消息记录
	offlineMsg := &model.OfflineMessage{
		UserID:         userID,
		MessageID:      msg.MessageID,
		ConversationID: msg.ConversationID,
		Content:        string(contentBytes),
		Pushed:         false,
		CreatedAt:      time.Now(),
		ExpireAt:       expireAt,
	}

	// 保存到数据库
	if err := s.db.WithContext(ctx).Create(offlineMsg).Error; err != nil {
		return fmt.Errorf("save offline message error: %w", err)
	}

	// 同时保存到Redis（用于快速查询）
	redisKey := fmt.Sprintf("offline:msgs:%s", userID)
	member := redis.Z{
		Score:  float64(msg.Timestamp),
		Member: msg.MessageID,
	}
	if err := s.redis.ZAdd(ctx, redisKey, &member).Err(); err != nil {
		// Redis保存失败不影响主流程，只记录日志
		fmt.Printf("save offline message to redis error: %v\n", err)
	}

	// 设置Redis键过期时间
	s.redis.Expire(ctx, redisKey, time.Duration(s.config.ExpireDays)*24*time.Hour)

	// 更新未读消息计数
	countKey := fmt.Sprintf("offline:count:%s", userID)
	s.redis.Incr(ctx, countKey)
	s.redis.Expire(ctx, countKey, time.Duration(s.config.ExpireDays)*24*time.Hour)

	return nil
}

// PullOfflineMessages 拉取离线消息
func (s *offlineServiceImpl) PullOfflineMessages(ctx context.Context, userID string, lastSeq int64, limit int) ([]*model.OfflineMessage, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}

	var messages []*model.OfflineMessage

	// 从数据库查询
	query := s.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Where("expire_at > ?", time.Now())

	if lastSeq > 0 {
		query = query.Where("id > ?", lastSeq)
	}

	if err := query.
		Order("id ASC").
		Limit(limit).
		Find(&messages).Error; err != nil {
		return nil, fmt.Errorf("query offline messages error: %w", err)
	}

	return messages, nil
}

// MarkAsPushed 标记消息已推送
func (s *offlineServiceImpl) MarkAsPushed(ctx context.Context, messageIDs []string) error {
	if len(messageIDs) == 0 {
		return nil
	}

	now := time.Now()
	if err := s.db.WithContext(ctx).
		Model(&model.OfflineMessage{}).
		Where("message_id IN ?", messageIDs).
		Updates(map[string]interface{}{
			"pushed":    true,
			"pushed_at": now,
		}).Error; err != nil {
		return fmt.Errorf("mark messages as pushed error: %w", err)
	}

	return nil
}

// DeleteOfflineMessages 删除离线消息
func (s *offlineServiceImpl) DeleteOfflineMessages(ctx context.Context, userID string, messageIDs []string) error {
	if len(messageIDs) == 0 {
		return nil
	}

	// 从数据库删除
	if err := s.db.WithContext(ctx).
		Where("user_id = ? AND message_id IN ?", userID, messageIDs).
		Delete(&model.OfflineMessage{}).Error; err != nil {
		return fmt.Errorf("delete offline messages error: %w", err)
	}

	// 从Redis删除
	redisKey := fmt.Sprintf("offline:msgs:%s", userID)
	for _, msgID := range messageIDs {
		s.redis.ZRem(ctx, redisKey, msgID)
	}

	// 更新计数
	countKey := fmt.Sprintf("offline:count:%s", userID)
	s.redis.DecrBy(ctx, countKey, int64(len(messageIDs)))

	return nil
}

// GetUnpushedMessages 获取未推送的消息
func (s *offlineServiceImpl) GetUnpushedMessages(ctx context.Context, userID string, limit int) ([]*model.OfflineMessage, error) {
	if limit <= 0 {
		limit = 100
	}

	var messages []*model.OfflineMessage

	if err := s.db.WithContext(ctx).
		Where("user_id = ? AND pushed = ? AND expire_at > ?", userID, false, time.Now()).
		Order("created_at ASC").
		Limit(limit).
		Find(&messages).Error; err != nil {
		return nil, fmt.Errorf("query unpushed messages error: %w", err)
	}

	return messages, nil
}

// CleanExpiredMessages 清理过期消息
func (s *offlineServiceImpl) CleanExpiredMessages(ctx context.Context) (int64, error) {
	result := s.db.WithContext(ctx).
		Where("expire_at < ?", time.Now()).
		Delete(&model.OfflineMessage{})

	if result.Error != nil {
		return 0, fmt.Errorf("clean expired messages error: %w", result.Error)
	}

	return result.RowsAffected, nil
}

// GetOfflineMessageCount 获取离线消息数量
func (s *offlineServiceImpl) GetOfflineMessageCount(ctx context.Context, userID string) (int64, error) {
	// 先从Redis获取
	countKey := fmt.Sprintf("offline:count:%s", userID)
	count, err := s.redis.Get(ctx, countKey).Int64()
	if err == nil {
		return count, nil
	}

	// Redis没有，从数据库查询
	var dbCount int64
	if err := s.db.WithContext(ctx).
		Model(&model.OfflineMessage{}).
		Where("user_id = ? AND expire_at > ?", userID, time.Now()).
		Count(&dbCount).Error; err != nil {
		return 0, err
	}

	// 缓存到Redis
	s.redis.Set(ctx, countKey, dbCount, time.Duration(s.config.ExpireDays)*24*time.Hour)

	return dbCount, nil
}

// StartCleanupTask 启动清理任务
func (s *offlineServiceImpl) StartCleanupTask(ctx context.Context) {
	ticker := time.NewTicker(s.config.CleanInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			count, err := s.CleanExpiredMessages(ctx)
			if err != nil {
				fmt.Printf("clean expired messages error: %v\n", err)
			} else if count > 0 {
				fmt.Printf("cleaned %d expired offline messages\n", count)
			}
		}
	}
}

// deleteOldestMessages 删除最旧的消息
func (s *offlineServiceImpl) deleteOldestMessages(ctx context.Context, userID string, count int) error {
	// 查询最旧的消息ID
	var messageIDs []string
	if err := s.db.WithContext(ctx).
		Model(&model.OfflineMessage{}).
		Where("user_id = ?", userID).
		Order("created_at ASC").
		Limit(count).
		Pluck("message_id", &messageIDs).Error; err != nil {
		return err
	}

	if len(messageIDs) == 0 {
		return nil
	}

	return s.DeleteOfflineMessages(ctx, userID, messageIDs)
}

// ParseOfflineMessage 解析离线消息内容
func ParseOfflineMessage(offlineMsg *model.OfflineMessage) (*model.Message, error) {
	var msg model.Message
	if err := json.Unmarshal([]byte(offlineMsg.Content), &msg); err != nil {
		return nil, fmt.Errorf("unmarshal message error: %w", err)
	}
	return &msg, nil
}

// OfflineMessageSummary 离线消息摘要
type OfflineMessageSummary struct {
	UserID        string                        `json:"user_id"`
	TotalCount    int64                         `json:"total_count"`
	UnpushedCount int64                         `json:"unpushed_count"`
	Conversations []*ConversationOfflineSummary `json:"conversations"`
}

// ConversationOfflineSummary 会话离线消息摘要
type ConversationOfflineSummary struct {
	ConversationID string `json:"conversation_id"`
	Count          int64  `json:"count"`
	LastMessageAt  int64  `json:"last_message_at"`
}

// GetOfflineMessageSummary 获取离线消息摘要
func (s *offlineServiceImpl) GetOfflineMessageSummary(ctx context.Context, userID string) (*OfflineMessageSummary, error) {
	summary := &OfflineMessageSummary{
		UserID: userID,
	}

	// 获取总数
	total, err := s.GetOfflineMessageCount(ctx, userID)
	if err != nil {
		return nil, err
	}
	summary.TotalCount = total

	// 获取未推送数
	var unpushedCount int64
	if err := s.db.WithContext(ctx).
		Model(&model.OfflineMessage{}).
		Where("user_id = ? AND pushed = ? AND expire_at > ?", userID, false, time.Now()).
		Count(&unpushedCount).Error; err != nil {
		return nil, err
	}
	summary.UnpushedCount = unpushedCount

	// 按会话分组统计
	type conversationStat struct {
		ConversationID string
		Count          int64
		LastCreatedAt  time.Time
	}

	var stats []conversationStat
	if err := s.db.WithContext(ctx).
		Model(&model.OfflineMessage{}).
		Select("conversation_id, COUNT(*) as count, MAX(created_at) as last_created_at").
		Where("user_id = ? AND expire_at > ?", userID, time.Now()).
		Group("conversation_id").
		Scan(&stats).Error; err != nil {
		return nil, err
	}

	summary.Conversations = make([]*ConversationOfflineSummary, 0, len(stats))
	for _, stat := range stats {
		summary.Conversations = append(summary.Conversations, &ConversationOfflineSummary{
			ConversationID: stat.ConversationID,
			Count:          stat.Count,
			LastMessageAt:  stat.LastCreatedAt.UnixMilli(),
		})
	}

	return summary, nil
}

// BatchSaveOfflineMessages 批量保存离线消息
func (s *offlineServiceImpl) BatchSaveOfflineMessages(ctx context.Context, userIDs []string, msg *model.Message) error {
	if len(userIDs) == 0 {
		return nil
	}

	// 序列化消息内容
	contentBytes, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal message error: %w", err)
	}

	// 计算过期时间
	expireAt := time.Now().Add(time.Duration(s.config.ExpireDays) * 24 * time.Hour)
	content := string(contentBytes)
	now := time.Now()

	// 构建批量插入数据
	offlineMessages := make([]*model.OfflineMessage, 0, len(userIDs))
	for _, userID := range userIDs {
		offlineMessages = append(offlineMessages, &model.OfflineMessage{
			UserID:         userID,
			MessageID:      fmt.Sprintf("%s_%s", msg.MessageID, userID), // 为每个用户生成唯一ID
			ConversationID: msg.ConversationID,
			Content:        content,
			Pushed:         false,
			CreatedAt:      now,
			ExpireAt:       expireAt,
		})
	}

	// 批量插入
	if err := s.db.WithContext(ctx).CreateInBatches(offlineMessages, 100).Error; err != nil {
		return fmt.Errorf("batch save offline messages error: %w", err)
	}

	// 更新Redis计数
	pipe := s.redis.Pipeline()
	for _, userID := range userIDs {
		countKey := fmt.Sprintf("offline:count:%s", userID)
		pipe.Incr(ctx, countKey)
		pipe.Expire(ctx, countKey, time.Duration(s.config.ExpireDays)*24*time.Hour)
	}
	_, _ = pipe.Exec(ctx)

	return nil
}

// OfflineMessageHandler 离线消息处理器（实现 OfflineMessageSaver 接口）
type OfflineMessageHandler struct {
	service OfflineService
}

// NewOfflineMessageHandler 创建离线消息处理器
func NewOfflineMessageHandler(service OfflineService) *OfflineMessageHandler {
	return &OfflineMessageHandler{
		service: service,
	}
}

// SaveOfflineMessage 实现 OfflineMessageSaver 接口
func (h *OfflineMessageHandler) SaveOfflineMessage(ctx context.Context, userID string, msg *model.Message) error {
	return h.service.SaveOfflineMessage(ctx, userID, msg)
}

// PullOfflineMessagesRequest 拉取离线消息请求
type PullOfflineMessagesRequest struct {
	LastSeq int64 `json:"last_seq" form:"last_seq"`
	Limit   int   `json:"limit" form:"limit" binding:"max=500"`
}

// PullOfflineMessagesResponse 拉取离线消息响应
type PullOfflineMessagesResponse struct {
	Messages []*model.OfflineMessage `json:"messages"`
	HasMore  bool                    `json:"has_more"`
	LastSeq  int64                   `json:"last_seq"`
}

// AckOfflineMessagesRequest 确认离线消息请求
type AckOfflineMessagesRequest struct {
	MessageIDs []string `json:"message_ids" binding:"required"`
}

// 生成唯一的离线消息ID
func generateOfflineMessageID(userID, messageID string) string {
	return fmt.Sprintf("%s_%s_%d", userID, messageID, util.NowNanos())
}
