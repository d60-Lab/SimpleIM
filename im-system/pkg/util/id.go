// Package util 提供通用工具函数
package util

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// IDGenerator ID生成器接口
type IDGenerator interface {
	// NextID 生成下一个ID
	NextID() string
	// NextMessageID 生成消息ID
	NextMessageID() string
	// NextFileID 生成文件ID
	NextFileID() string
}

// SnowflakeConfig 雪花算法配置
type SnowflakeConfig struct {
	NodeID       int64 // 节点ID (0-1023)
	Epoch        int64 // 起始时间戳（毫秒）
	NodeBits     uint  // 节点ID位数
	SequenceBits uint  // 序列号位数
}

// DefaultSnowflakeConfig 默认雪花算法配置
var DefaultSnowflakeConfig = SnowflakeConfig{
	NodeID:       1,
	Epoch:        1704067200000, // 2024-01-01 00:00:00 UTC
	NodeBits:     10,
	SequenceBits: 12,
}

// Snowflake 雪花算法ID生成器
type Snowflake struct {
	mu           sync.Mutex
	nodeID       int64
	epoch        int64
	nodeBits     uint
	sequenceBits uint
	nodeMax      int64
	sequenceMax  int64
	timeShift    uint
	nodeShift    uint
	lastTime     int64
	sequence     int64
}

// NewSnowflake 创建雪花算法ID生成器
func NewSnowflake(config SnowflakeConfig) (*Snowflake, error) {
	nodeMax := int64(1<<config.NodeBits - 1)
	if config.NodeID < 0 || config.NodeID > nodeMax {
		return nil, fmt.Errorf("node ID must be between 0 and %d", nodeMax)
	}

	return &Snowflake{
		nodeID:       config.NodeID,
		epoch:        config.Epoch,
		nodeBits:     config.NodeBits,
		sequenceBits: config.SequenceBits,
		nodeMax:      nodeMax,
		sequenceMax:  int64(1<<config.SequenceBits - 1),
		timeShift:    config.NodeBits + config.SequenceBits,
		nodeShift:    config.SequenceBits,
		lastTime:     0,
		sequence:     0,
	}, nil
}

// NextID 生成下一个ID
func (s *Snowflake) NextID() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UnixMilli()

	if now < s.lastTime {
		// 时钟回拨，等待到上次时间
		now = s.waitUntil(s.lastTime)
	}

	if now == s.lastTime {
		s.sequence = (s.sequence + 1) & s.sequenceMax
		if s.sequence == 0 {
			// 序列号用完，等待下一毫秒
			now = s.waitUntil(now)
		}
	} else {
		s.sequence = 0
	}

	s.lastTime = now

	id := ((now - s.epoch) << s.timeShift) |
		(s.nodeID << s.nodeShift) |
		s.sequence

	return id
}

// waitUntil 等待直到指定时间
func (s *Snowflake) waitUntil(lastTime int64) int64 {
	for {
		now := time.Now().UnixMilli()
		if now > lastTime {
			return now
		}
		time.Sleep(time.Millisecond)
	}
}

// NextIDString 生成字符串格式的ID
func (s *Snowflake) NextIDString() string {
	return fmt.Sprintf("%d", s.NextID())
}

// defaultGenerator 默认ID生成器
var (
	defaultGenerator *Snowflake
	generatorOnce    sync.Once
)

// GetDefaultGenerator 获取默认ID生成器
func GetDefaultGenerator() *Snowflake {
	generatorOnce.Do(func() {
		var err error
		defaultGenerator, err = NewSnowflake(DefaultSnowflakeConfig)
		if err != nil {
			panic(fmt.Sprintf("failed to create default snowflake generator: %v", err))
		}
	})
	return defaultGenerator
}

// GenerateID 生成唯一ID
func GenerateID() string {
	return GetDefaultGenerator().NextIDString()
}

// GenerateUUID 生成UUID
func GenerateUUID() string {
	return uuid.New().String()
}

// GenerateShortUUID 生成短UUID（去掉横杠）
func GenerateShortUUID() string {
	return strings.ReplaceAll(uuid.New().String(), "-", "")
}

// GenerateMessageID 生成消息ID
// 格式: msg_<timestamp>_<random>
func GenerateMessageID() string {
	return fmt.Sprintf("msg_%d_%s", time.Now().UnixNano(), randomHex(8))
}

// GenerateFileID 生成文件ID
// 格式: file_<timestamp>_<random>
func GenerateFileID() string {
	return fmt.Sprintf("file_%d_%s", time.Now().UnixNano(), randomHex(8))
}

// GenerateGroupID 生成群组ID
// 格式: group_<uuid>
func GenerateGroupID() string {
	return "group_" + GenerateShortUUID()
}

// GenerateUserID 生成用户ID
// 格式: user_<uuid>
func GenerateUserID() string {
	return "user_" + GenerateShortUUID()
}

// GenerateConversationID 生成会话ID
// 单聊: single_<小user_id>_<大user_id>
// 群聊: group_<group_id>
func GenerateConversationID(convType int, id1, id2 string) string {
	if convType == 1 { // 单聊
		if id1 < id2 {
			return fmt.Sprintf("single_%s_%s", id1, id2)
		}
		return fmt.Sprintf("single_%s_%s", id2, id1)
	}
	// 群聊
	return fmt.Sprintf("group_%s", id1)
}

// GenerateToken 生成随机Token
func GenerateToken(length int) string {
	return randomHex(length)
}

// GenerateDeviceID 生成设备ID
func GenerateDeviceID() string {
	return "device_" + randomHex(16)
}

// GenerateUploadID 生成上传ID
func GenerateUploadID() string {
	return fmt.Sprintf("upload_%d_%s", time.Now().UnixNano(), randomHex(8))
}

// randomHex 生成随机十六进制字符串
func randomHex(n int) string {
	bytes := make([]byte, n)
	if _, err := rand.Read(bytes); err != nil {
		// 如果随机数生成失败，使用时间戳作为备选
		return fmt.Sprintf("%x", time.Now().UnixNano())[:n*2]
	}
	return hex.EncodeToString(bytes)[:n*2]
}

// TimeFormat 时间格式化常量
const (
	TimeFormatDateTime     = "2006-01-02 15:04:05"
	TimeFormatDate         = "2006-01-02"
	TimeFormatTime         = "15:04:05"
	TimeFormatDateTimeNano = "2006-01-02 15:04:05.000000000"
	TimeFormatCompact      = "20060102150405"
	TimeFormatMonth        = "200601"
)

// FormatTime 格式化时间
func FormatTime(t time.Time, format string) string {
	return t.Format(format)
}

// ParseTime 解析时间
func ParseTime(s string, format string) (time.Time, error) {
	return time.Parse(format, s)
}

// NowMillis 获取当前时间戳（毫秒）
func NowMillis() int64 {
	return time.Now().UnixMilli()
}

// NowMicros 获取当前时间戳（微秒）
func NowMicros() int64 {
	return time.Now().UnixMicro()
}

// NowNanos 获取当前时间戳（纳秒）
func NowNanos() int64 {
	return time.Now().UnixNano()
}

// MillisToTime 毫秒时间戳转时间
func MillisToTime(millis int64) time.Time {
	return time.UnixMilli(millis)
}

// GetMessageTableName 获取消息分表表名
// 按月分表
func GetMessageTableName(t time.Time) string {
	return fmt.Sprintf("messages_%s", t.Format(TimeFormatMonth))
}

// GetCurrentMessageTableName 获取当前消息分表表名
func GetCurrentMessageTableName() string {
	return GetMessageTableName(time.Now())
}

// StringSliceContains 检查字符串切片是否包含指定元素
func StringSliceContains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// UniqueStrings 字符串切片去重
func UniqueStrings(slice []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(slice))
	for _, s := range slice {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}

// RemoveString 从切片中移除指定字符串
func RemoveString(slice []string, item string) []string {
	result := make([]string, 0, len(slice))
	for _, s := range slice {
		if s != item {
			result = append(result, s)
		}
	}
	return result
}

// Min 返回两个整数中的较小值
func Min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Max 返回两个整数中的较大值
func Max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// MinInt64 返回两个int64中的较小值
func MinInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

// MaxInt64 返回两个int64中的较大值
func MaxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

// Clamp 将值限制在指定范围内
func Clamp(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

// ClampInt64 将int64值限制在指定范围内
func ClampInt64(value, min, max int64) int64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

// Retry 重试执行函数
func Retry(attempts int, delay time.Duration, fn func() error) error {
	var lastErr error
	for i := 0; i < attempts; i++ {
		if err := fn(); err != nil {
			lastErr = err
			if i < attempts-1 {
				time.Sleep(delay)
			}
			continue
		}
		return nil
	}
	return lastErr
}

// RetryWithBackoff 带退避的重试
func RetryWithBackoff(attempts int, initialDelay time.Duration, maxDelay time.Duration, fn func() error) error {
	var lastErr error
	delay := initialDelay

	for i := 0; i < attempts; i++ {
		if err := fn(); err != nil {
			lastErr = err
			if i < attempts-1 {
				time.Sleep(delay)
				// 指数退避
				delay *= 2
				if delay > maxDelay {
					delay = maxDelay
				}
			}
			continue
		}
		return nil
	}
	return lastErr
}
