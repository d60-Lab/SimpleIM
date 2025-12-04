// Package model 定义IM系统的数据模型
package model

import (
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

// MessageType 消息类型
type MessageType int

const (
	// 基础消息类型
	MsgText       MessageType = 0 // 文本消息
	MsgSingleChat MessageType = 1 // 单聊消息
	MsgGroupChat  MessageType = 2 // 群聊消息
	MsgSystem     MessageType = 3 // 系统消息

	// 媒体消息类型
	MsgImage    MessageType = 4  // 图片消息
	MsgVoice    MessageType = 5  // 语音消息
	MsgVideo    MessageType = 6  // 视频消息
	MsgFile     MessageType = 7  // 文件消息
	MsgLocation MessageType = 8  // 位置消息
	MsgCard     MessageType = 9  // 名片消息
	MsgCustom   MessageType = 10 // 自定义消息

	// 群组事件消息
	MsgGroupCreated      MessageType = 20 // 群组创建
	MsgGroupMemberJoin   MessageType = 21 // 成员加入
	MsgGroupMemberLeave  MessageType = 22 // 成员离开
	MsgGroupMemberKicked MessageType = 23 // 成员被踢
	MsgGroupDismissed    MessageType = 24 // 群组解散
	MsgGroupInfoUpdate   MessageType = 25 // 群信息更新
	MsgGroupAdminChange  MessageType = 26 // 管理员变更
	MsgGroupMute         MessageType = 27 // 群禁言
	MsgGroupTransfer     MessageType = 28 // 群主转让

	// 消息状态类型
	MsgAck         MessageType = 30 // 消息确认
	MsgReadReceipt MessageType = 31 // 已读回执
	MsgRevoke      MessageType = 32 // 消息撤回
	MsgTyping      MessageType = 33 // 正在输入

	// 系统消息类型
	MsgHeartbeat     MessageType = 99  // 心跳消息
	MsgKickout       MessageType = 100 // 踢出下线
	MsgServerNotice  MessageType = 101 // 服务器通知
	MsgFriendRequest MessageType = 102 // 好友请求
	MsgFriendAccept  MessageType = 103 // 好友接受
)

// String 返回消息类型的字符串表示
func (t MessageType) String() string {
	switch t {
	case MsgText:
		return "text"
	case MsgSingleChat:
		return "single_chat"
	case MsgGroupChat:
		return "group_chat"
	case MsgSystem:
		return "system"
	case MsgImage:
		return "image"
	case MsgVoice:
		return "voice"
	case MsgVideo:
		return "video"
	case MsgFile:
		return "file"
	case MsgLocation:
		return "location"
	case MsgCard:
		return "card"
	case MsgCustom:
		return "custom"
	case MsgGroupCreated:
		return "group_created"
	case MsgGroupMemberJoin:
		return "group_member_join"
	case MsgGroupMemberLeave:
		return "group_member_leave"
	case MsgGroupMemberKicked:
		return "group_member_kicked"
	case MsgGroupDismissed:
		return "group_dismissed"
	case MsgGroupInfoUpdate:
		return "group_info_update"
	case MsgGroupAdminChange:
		return "group_admin_change"
	case MsgGroupMute:
		return "group_mute"
	case MsgGroupTransfer:
		return "group_transfer"
	case MsgAck:
		return "ack"
	case MsgReadReceipt:
		return "read_receipt"
	case MsgRevoke:
		return "revoke"
	case MsgTyping:
		return "typing"
	case MsgHeartbeat:
		return "heartbeat"
	case MsgKickout:
		return "kickout"
	case MsgServerNotice:
		return "server_notice"
	case MsgFriendRequest:
		return "friend_request"
	case MsgFriendAccept:
		return "friend_accept"
	default:
		return "unknown"
	}
}

// QoSLevel 消息质量等级
type QoSLevel int

const (
	QoSAtMostOnce  QoSLevel = 0 // 最多一次（不保证送达）
	QoSAtLeastOnce QoSLevel = 1 // 至少一次（保证送达，可能重复）
	QoSExactlyOnce QoSLevel = 2 // 恰好一次（保证送达且不重复）
)

// Message 消息主体结构
type Message struct {
	MessageID       string      `json:"message_id" gorm:"primaryKey;type:varchar(64)"`
	Type            MessageType `json:"type" gorm:"type:tinyint;not null"`
	From            string      `json:"from" gorm:"column:from_user_id;type:varchar(64);index"`
	To              string      `json:"to" gorm:"column:to_id;type:varchar(64);index"`
	Content         interface{} `json:"content" gorm:"-"`
	ContentRaw      string      `json:"-" gorm:"column:content;type:text"`
	Timestamp       int64       `json:"timestamp" gorm:"autoCreateTime:milli"`
	ClientTimestamp int64       `json:"client_timestamp,omitempty" gorm:"-"`
	QoS             QoSLevel    `json:"qos" gorm:"-"`
	ConversationID  string      `json:"conversation_id" gorm:"type:varchar(128);index:idx_conversation_seq"`
	Seq             int64       `json:"seq" gorm:"index:idx_conversation_seq"`
	Revoked         bool        `json:"revoked" gorm:"default:false"`
	CreatedAt       time.Time   `json:"created_at" gorm:"autoCreateTime"`
}

// TableName 指定消息表名
func (Message) TableName() string {
	return "messages"
}

// BeforeCreate 创建前序列化Content
func (m *Message) BeforeCreate(tx *gorm.DB) error {
	if m.Content != nil {
		data, err := json.Marshal(m.Content)
		if err != nil {
			return err
		}
		m.ContentRaw = string(data)
	}
	return nil
}

// AfterFind 查询后反序列化Content
func (m *Message) AfterFind(tx *gorm.DB) error {
	if m.ContentRaw != "" {
		var content interface{}
		if err := json.Unmarshal([]byte(m.ContentRaw), &content); err != nil {
			return err
		}
		m.Content = content
	}
	return nil
}

// MarshalBinary 序列化为二进制（用于Redis）
func (m *Message) MarshalBinary() ([]byte, error) {
	return json.Marshal(m)
}

// UnmarshalBinary 从二进制反序列化（用于Redis）
func (m *Message) UnmarshalBinary(data []byte) error {
	return json.Unmarshal(data, m)
}

// TextContent 文本消息内容
type TextContent struct {
	Text      string   `json:"text"`
	AtUserIDs []string `json:"at_user_ids,omitempty"` // @的用户ID列表
	AtAll     bool     `json:"at_all,omitempty"`      // 是否@所有人
}

// ImageContent 图片消息内容
type ImageContent struct {
	FileID       string `json:"file_id"`
	URL          string `json:"url"`
	ThumbnailURL string `json:"thumbnail_url,omitempty"`
	Width        int    `json:"width,omitempty"`
	Height       int    `json:"height,omitempty"`
	FileSize     int64  `json:"file_size,omitempty"`
	Format       string `json:"format,omitempty"` // jpeg, png, gif, webp
}

// VoiceContent 语音消息内容
type VoiceContent struct {
	FileID   string `json:"file_id"`
	URL      string `json:"url"`
	Duration int    `json:"duration"` // 时长（秒）
	FileSize int64  `json:"file_size,omitempty"`
	Format   string `json:"format,omitempty"` // mp3, amr, wav
}

// VideoContent 视频消息内容
type VideoContent struct {
	FileID       string `json:"file_id"`
	URL          string `json:"url"`
	ThumbnailURL string `json:"thumbnail_url,omitempty"`
	Duration     int    `json:"duration"` // 时长（秒）
	Width        int    `json:"width,omitempty"`
	Height       int    `json:"height,omitempty"`
	FileSize     int64  `json:"file_size,omitempty"`
	Format       string `json:"format,omitempty"` // mp4, mov, webm
}

// FileContent 文件消息内容
type FileContent struct {
	FileID   string `json:"file_id"`
	FileName string `json:"file_name"`
	FileSize int64  `json:"file_size"`
	FileExt  string `json:"file_ext,omitempty"`
	MimeType string `json:"mime_type,omitempty"`
	URL      string `json:"url"`
}

// LocationContent 位置消息内容
type LocationContent struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Name      string  `json:"name,omitempty"`
	Address   string  `json:"address,omitempty"`
	Zoom      int     `json:"zoom,omitempty"`
}

// CardContent 名片消息内容
type CardContent struct {
	UserID   string `json:"user_id"`
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar,omitempty"`
}

// GroupEventContent 群组事件内容
type GroupEventContent struct {
	GroupID    string            `json:"group_id"`
	OperatorID string            `json:"operator_id"`
	TargetIDs  []string          `json:"target_ids,omitempty"`
	Extra      map[string]string `json:"extra,omitempty"`
}

// GroupInfoUpdateContent 群资料变更内容
type GroupInfoUpdateContent struct {
	GroupID    string `json:"group_id"`
	OperatorID string `json:"operator_id"`
	Field      string `json:"field"`     // name, avatar, announcement, description
	OldValue   string `json:"old_value"` // 旧值
	NewValue   string `json:"new_value"` // 新值
}

// AckContent ACK消息内容
type AckContent struct {
	MessageID string `json:"message_id"` // 被确认的消息ID
	Status    int    `json:"status"`     // 0-已接收 1-已存储
}

// ReadReceiptContent 已读回执内容
type ReadReceiptContent struct {
	ConversationID string   `json:"conversation_id"`
	MessageIDs     []string `json:"message_ids,omitempty"` // 已读的消息ID列表
	LastReadSeq    int64    `json:"last_read_seq"`         // 最后已读序列号
}

// RevokeContent 撤回消息内容
type RevokeContent struct {
	MessageID string `json:"message_id"` // 被撤回的消息ID
	Operator  string `json:"operator"`   // 撤回操作人
}

// TypingContent 正在输入内容
type TypingContent struct {
	ConversationID string `json:"conversation_id"`
}

// HeartbeatContent 心跳消息内容
type HeartbeatContent struct {
	Timestamp int64 `json:"timestamp"`
}

// KickoutContent 踢出下线内容
type KickoutContent struct {
	Reason   string `json:"reason"`              // 踢出原因
	DeviceID string `json:"device_id,omitempty"` // 新登录的设备ID
}

// ServerNoticeContent 服务器通知内容
type ServerNoticeContent struct {
	Title   string `json:"title,omitempty"`
	Content string `json:"content"`
	Action  string `json:"action,omitempty"` // 动作类型
	Data    string `json:"data,omitempty"`   // 附加数据
}

// FriendRequestContent 好友请求内容
type FriendRequestContent struct {
	FromUserID string `json:"from_user_id"`
	Nickname   string `json:"nickname"`
	Avatar     string `json:"avatar,omitempty"`
	Message    string `json:"message,omitempty"` // 申请留言
}

// CustomContent 自定义消息内容
type CustomContent struct {
	CustomType string                 `json:"custom_type"` // 自定义类型
	Data       map[string]interface{} `json:"data"`        // 自定义数据
}

// MessageRecord 消息记录（用于持久化存储）
type MessageRecord struct {
	MessageID      string    `json:"message_id" gorm:"primaryKey;type:varchar(64)"`
	ConversationID string    `json:"conversation_id" gorm:"type:varchar(128);index:idx_conv_seq"`
	Type           int       `json:"type" gorm:"type:tinyint;not null"`
	FromUserID     string    `json:"from_user_id" gorm:"type:varchar(64);index"`
	ToUserID       string    `json:"to_user_id" gorm:"type:varchar(64);index"`
	GroupID        string    `json:"group_id" gorm:"type:varchar(64);index"`
	Content        string    `json:"content" gorm:"type:text"`
	Seq            int64     `json:"seq" gorm:"index:idx_conv_seq"`
	Status         int       `json:"status" gorm:"type:tinyint;default:1"` // 1-已发送 2-已送达 3-已读
	Revoked        bool      `json:"revoked" gorm:"default:false"`
	CreatedAt      time.Time `json:"created_at" gorm:"autoCreateTime"`
}

// TableName 指定消息记录表名
func (MessageRecord) TableName() string {
	return "messages"
}

// OfflineMessage 离线消息
type OfflineMessage struct {
	ID             uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	UserID         string    `json:"user_id" gorm:"type:varchar(64);index:idx_user_created"`
	MessageID      string    `json:"message_id" gorm:"type:varchar(64);uniqueIndex"`
	ConversationID string    `json:"conversation_id" gorm:"type:varchar(128)"`
	Content        string    `json:"content" gorm:"type:text"`
	Pushed         bool      `json:"pushed" gorm:"default:false;index"`
	PushedAt       time.Time `json:"pushed_at,omitempty"`
	CreatedAt      time.Time `json:"created_at" gorm:"autoCreateTime;index:idx_user_created"`
	ExpireAt       time.Time `json:"expire_at" gorm:"index"`
}

// TableName 指定离线消息表名
func (OfflineMessage) TableName() string {
	return "offline_messages"
}

// Conversation 会话
type Conversation struct {
	ConversationID string    `json:"conversation_id" gorm:"primaryKey;type:varchar(128)"`
	Type           int       `json:"type" gorm:"type:tinyint;not null"` // 1-单聊 2-群聊
	LastMessageID  string    `json:"last_message_id,omitempty" gorm:"type:varchar(64)"`
	LastMessageAt  time.Time `json:"last_message_at,omitempty"`
	CreatedAt      time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt      time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName 指定会话表名
func (Conversation) TableName() string {
	return "conversations"
}

// ConversationType 会话类型
const (
	ConversationTypeSingle = 1 // 单聊
	ConversationTypeGroup  = 2 // 群聊
)

// UserConversation 用户会话关系
type UserConversation struct {
	ID             uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	UserID         string    `json:"user_id" gorm:"type:varchar(64);uniqueIndex:idx_user_conv"`
	ConversationID string    `json:"conversation_id" gorm:"type:varchar(128);uniqueIndex:idx_user_conv"`
	UnreadCount    int       `json:"unread_count" gorm:"default:0"`
	LastReadSeq    int64     `json:"last_read_seq" gorm:"default:0"`
	Muted          bool      `json:"muted" gorm:"default:false"`
	Pinned         bool      `json:"pinned" gorm:"default:false"`
	Deleted        bool      `json:"deleted" gorm:"default:false"`
	UpdatedAt      time.Time `json:"updated_at" gorm:"autoUpdateTime;index:idx_user_updated"`
	CreatedAt      time.Time `json:"created_at" gorm:"autoCreateTime"`
}

// TableName 指定用户会话表名
func (UserConversation) TableName() string {
	return "user_conversations"
}

// MessageReadStatus 消息已读状态
type MessageReadStatus struct {
	ID             uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	ConversationID string    `json:"conversation_id" gorm:"type:varchar(128);uniqueIndex:idx_conv_user"`
	UserID         string    `json:"user_id" gorm:"type:varchar(64);uniqueIndex:idx_conv_user"`
	LastReadSeq    int64     `json:"last_read_seq" gorm:"default:0"`
	LastReadAt     time.Time `json:"last_read_at" gorm:"autoUpdateTime"`
}

// TableName 指定消息已读状态表名
func (MessageReadStatus) TableName() string {
	return "message_read_status"
}

// NewTextMessage 创建文本消息
func NewTextMessage(from, to string, msgType MessageType, text string) *Message {
	return &Message{
		Type: msgType,
		From: from,
		To:   to,
		Content: &TextContent{
			Text: text,
		},
		Timestamp: time.Now().UnixMilli(),
		QoS:       QoSAtLeastOnce,
	}
}

// NewImageMessage 创建图片消息
func NewImageMessage(from, to string, msgType MessageType, content *ImageContent) *Message {
	return &Message{
		Type:      msgType,
		From:      from,
		To:        to,
		Content:   content,
		Timestamp: time.Now().UnixMilli(),
		QoS:       QoSAtLeastOnce,
	}
}

// NewGroupEventMessage 创建群组事件消息
func NewGroupEventMessage(eventType MessageType, groupID, operatorID string, targetIDs []string) *Message {
	return &Message{
		Type: eventType,
		From: operatorID,
		To:   groupID,
		Content: &GroupEventContent{
			GroupID:    groupID,
			OperatorID: operatorID,
			TargetIDs:  targetIDs,
		},
		Timestamp: time.Now().UnixMilli(),
		QoS:       QoSAtLeastOnce,
	}
}

// NewHeartbeatMessage 创建心跳消息
func NewHeartbeatMessage() *Message {
	return &Message{
		Type: MsgHeartbeat,
		Content: &HeartbeatContent{
			Timestamp: time.Now().UnixMilli(),
		},
		Timestamp: time.Now().UnixMilli(),
	}
}

// NewAckMessage 创建ACK消息
func NewAckMessage(messageID string, status int) *Message {
	return &Message{
		Type: MsgAck,
		Content: &AckContent{
			MessageID: messageID,
			Status:    status,
		},
		Timestamp: time.Now().UnixMilli(),
	}
}

// GetSingleChatConversationID 获取单聊会话ID
func GetSingleChatConversationID(userID1, userID2 string) string {
	if userID1 < userID2 {
		return "single:" + userID1 + ":" + userID2
	}
	return "single:" + userID2 + ":" + userID1
}

// GetGroupChatConversationID 获取群聊会话ID
func GetGroupChatConversationID(groupID string) string {
	return "group:" + groupID
}
