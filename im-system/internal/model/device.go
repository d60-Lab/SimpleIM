// Package model 定义数据模型
package model

import (
	"time"
)

// Platform 设备平台
type Platform string

const (
	PlatformIOS     Platform = "ios"
	PlatformAndroid Platform = "android"
	PlatformWeb     Platform = "web"
)

// Device 设备信息（用于推送）
type Device struct {
	ID          uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	UserID      string    `json:"user_id" gorm:"type:varchar(64);index;not null"`
	DeviceToken string    `json:"device_token" gorm:"type:varchar(256);uniqueIndex;not null"`
	Platform    Platform  `json:"platform" gorm:"type:varchar(16);not null"` // ios, android, web
	AppVersion  string    `json:"app_version" gorm:"type:varchar(32)"`
	DeviceInfo  string    `json:"device_info" gorm:"type:varchar(256)"` // 设备型号等信息
	PushEnabled bool      `json:"push_enabled" gorm:"default:true"`     // 是否开启推送
	UpdatedAt   time.Time `json:"updated_at" gorm:"autoUpdateTime"`
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`
}

// TableName 指定表名
func (Device) TableName() string {
	return "devices"
}

// IsIOS 判断是否为iOS设备
func (d *Device) IsIOS() bool {
	return d.Platform == PlatformIOS
}

// IsAndroid 判断是否为Android设备
func (d *Device) IsAndroid() bool {
	return d.Platform == PlatformAndroid
}

// RegisterDeviceRequest 注册设备请求
type RegisterDeviceRequest struct {
	DeviceToken string   `json:"device_token" binding:"required"`
	Platform    Platform `json:"platform" binding:"required,oneof=ios android web"`
	AppVersion  string   `json:"app_version"`
	DeviceInfo  string   `json:"device_info"`
}

// UnregisterDeviceRequest 注销设备请求
type UnregisterDeviceRequest struct {
	DeviceToken string `json:"device_token" binding:"required"`
}

// PushNotification 推送通知
type PushNotification struct {
	Title       string            `json:"title,omitempty"`
	Body        string            `json:"body"`
	Badge       int               `json:"badge,omitempty"`        // iOS角标数
	Sound       string            `json:"sound,omitempty"`        // 提示音
	Data        map[string]string `json:"data,omitempty"`         // 自定义数据
	Category    string            `json:"category,omitempty"`     // 通知类别
	ThreadID    string            `json:"thread_id,omitempty"`    // 会话ID（iOS消息分组）
	MessageID   string            `json:"message_id,omitempty"`   // 关联的消息ID
	CollapseKey string            `json:"collapse_key,omitempty"` // 折叠键（同一键的通知会被合并）
	Priority    PushPriority      `json:"priority,omitempty"`     // 推送优先级
	TTL         int               `json:"ttl,omitempty"`          // 有效期（秒）
}

// PushPriority 推送优先级
type PushPriority int

const (
	PushPriorityNormal PushPriority = 0 // 普通优先级
	PushPriorityHigh   PushPriority = 1 // 高优先级
)

// BatchPushRequest 批量推送请求
type BatchPushRequest struct {
	IOSTokens     []string          `json:"ios_tokens"`
	AndroidTokens []string          `json:"android_tokens"`
	Notification  *PushNotification `json:"notification"`
}

// PushResult 推送结果
type PushResult struct {
	Success      bool   `json:"success"`
	DeviceToken  string `json:"device_token"`
	Platform     string `json:"platform"`
	Error        string `json:"error,omitempty"`
	InvalidToken bool   `json:"invalid_token,omitempty"` // 标记无效Token，需要清理
	RetryAfter   int    `json:"retry_after,omitempty"`   // 需要等待多少秒后重试
}

// BatchPushResult 批量推送结果
type BatchPushResult struct {
	TotalCount   int           `json:"total_count"`
	SuccessCount int           `json:"success_count"`
	FailedCount  int           `json:"failed_count"`
	Results      []*PushResult `json:"results,omitempty"`
}

// PushTask 推送任务
type PushTask struct {
	ID           string            `json:"id"`
	UserID       string            `json:"user_id"`
	MessageID    string            `json:"message_id"`
	Notification *PushNotification `json:"notification"`
	Devices      []*Device         `json:"devices"`
	Retries      int               `json:"retries"`
	CreatedAt    time.Time         `json:"created_at"`
	ScheduledAt  time.Time         `json:"scheduled_at"` // 计划执行时间（用于延迟推送）
}

// DeviceInfo 设备详细信息
type DeviceInfo struct {
	UserID      string    `json:"user_id"`
	DeviceToken string    `json:"device_token"`
	Platform    Platform  `json:"platform"`
	AppVersion  string    `json:"app_version"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// APNsConfig APNs配置
type APNsConfig struct {
	Production bool   `json:"production"` // true=生产环境, false=沙盒环境
	BundleID   string `json:"bundle_id"`
	KeyFile    string `json:"key_file"` // .p8文件路径
	KeyID      string `json:"key_id"`
	TeamID     string `json:"team_id"`
}

// FCMConfig FCM配置
type FCMConfig struct {
	ProjectID       string `json:"project_id"`
	CredentialsFile string `json:"credentials_file"` // Firebase凭证文件路径
	ServerKey       string `json:"server_key"`       // Legacy API服务器密钥（可选）
}

// PushConfig 推送配置
type PushConfig struct {
	APNs         *APNsConfig `json:"apns"`
	FCM          *FCMConfig  `json:"fcm"`
	WorkerCount  int         `json:"worker_count"`  // 推送Worker数量
	BatchSize    int         `json:"batch_size"`    // 批量推送大小
	MaxRetries   int         `json:"max_retries"`   // 最大重试次数
	RetryDelay   int         `json:"retry_delay"`   // 重试延迟（秒）
	MergeEnabled bool        `json:"merge_enabled"` // 是否启用推送合并
	MergeWindow  int         `json:"merge_window"`  // 合并窗口（秒）
}

// NewPushNotification 创建推送通知
func NewPushNotification(title, body string) *PushNotification {
	return &PushNotification{
		Title:    title,
		Body:     body,
		Sound:    "default",
		Priority: PushPriorityHigh,
	}
}

// WithBadge 设置角标
func (n *PushNotification) WithBadge(badge int) *PushNotification {
	n.Badge = badge
	return n
}

// WithData 设置自定义数据
func (n *PushNotification) WithData(data map[string]string) *PushNotification {
	n.Data = data
	return n
}

// WithCollapseKey 设置折叠键
func (n *PushNotification) WithCollapseKey(key string) *PushNotification {
	n.CollapseKey = key
	return n
}

// WithCategory 设置通知类别
func (n *PushNotification) WithCategory(category string) *PushNotification {
	n.Category = category
	return n
}

// WithThreadID 设置会话ID
func (n *PushNotification) WithThreadID(threadID string) *PushNotification {
	n.ThreadID = threadID
	return n
}

// WithTTL 设置有效期
func (n *PushNotification) WithTTL(ttl int) *PushNotification {
	n.TTL = ttl
	return n
}
