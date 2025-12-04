// Package model 定义数据模型
package model

import (
	"time"
)

// UserStatus 用户状态
type UserStatus int

const (
	UserStatusNormal   UserStatus = 1 // 正常
	UserStatusDisabled UserStatus = 0 // 禁用
)

// User 用户模型
type User struct {
	UserID       string     `json:"user_id" gorm:"primaryKey;type:varchar(64)"`
	Username     string     `json:"username" gorm:"type:varchar(64);uniqueIndex;not null"`
	Nickname     string     `json:"nickname" gorm:"type:varchar(64)"`
	Avatar       string     `json:"avatar" gorm:"type:varchar(512)"`
	PasswordHash string     `json:"-" gorm:"type:varchar(256);not null"` // 密码哈希，JSON序列化时忽略
	Status       UserStatus `json:"status" gorm:"default:1"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// TableName 指定表名
func (User) TableName() string {
	return "users"
}

// UserInfo 用户信息（对外暴露）
type UserInfo struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
	Online   bool   `json:"online"`
}

// ToUserInfo 转换为用户信息
func (u *User) ToUserInfo() *UserInfo {
	return &UserInfo{
		UserID:   u.UserID,
		Username: u.Username,
		Nickname: u.Nickname,
		Avatar:   u.Avatar,
		Online:   false, // 需要从Redis查询
	}
}

// RegisterRequest 注册请求
type RegisterRequest struct {
	Username string `json:"username" binding:"required,min=3,max=32"`
	Password string `json:"password" binding:"required,min=6,max=32"`
	Nickname string `json:"nickname" binding:"max=32"`
}

// LoginRequest 登录请求
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginResponse 登录响应
type LoginResponse struct {
	UserID       string    `json:"user_id"`
	Username     string    `json:"username"`
	Nickname     string    `json:"nickname"`
	Avatar       string    `json:"avatar"`
	Token        string    `json:"token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	WebSocketURL string    `json:"websocket_url"`
}

// UpdateUserRequest 更新用户信息请求
type UpdateUserRequest struct {
	Nickname *string `json:"nickname" binding:"omitempty,max=32"`
	Avatar   *string `json:"avatar" binding:"omitempty,max=512"`
}

// ChangePasswordRequest 修改密码请求
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=6,max=32"`
}

// TokenClaims JWT令牌声明
type TokenClaims struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
}

// OnlineStatus 在线状态
type OnlineStatus struct {
	UserID     string    `json:"user_id"`
	NodeID     string    `json:"node_id"`
	Platform   string    `json:"platform"` // web, ios, android
	LoginAt    time.Time `json:"login_at"`
	LastSeenAt time.Time `json:"last_seen_at"`
}
