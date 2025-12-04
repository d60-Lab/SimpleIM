// Package auth 提供认证相关功能
package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrInvalidToken   = errors.New("invalid token")
	ErrExpiredToken   = errors.New("token has expired")
	ErrTokenNotActive = errors.New("token not active yet")
	ErrInvalidClaims  = errors.New("invalid token claims")
	ErrMissingUserID  = errors.New("missing user_id in token")
	ErrSigningMethod  = errors.New("unexpected signing method")
)

// JWTConfig JWT配置
type JWTConfig struct {
	Secret        string        `json:"secret"`
	Issuer        string        `json:"issuer"`
	Expire        time.Duration `json:"expire"`         // Access Token过期时间
	RefreshExpire time.Duration `json:"refresh_expire"` // Refresh Token过期时间
}

// DefaultJWTConfig 默认JWT配置
func DefaultJWTConfig() *JWTConfig {
	return &JWTConfig{
		Secret:        "im-system-jwt-secret-key-change-in-production",
		Issuer:        "im-system",
		Expire:        7 * 24 * time.Hour,  // 7天
		RefreshExpire: 30 * 24 * time.Hour, // 30天
	}
}

// Claims JWT声明
type Claims struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Platform string `json:"platform,omitempty"` // web, ios, android
	DeviceID string `json:"device_id,omitempty"`
	jwt.RegisteredClaims
}

// JWTManager JWT管理器
type JWTManager struct {
	config *JWTConfig
}

// NewJWTManager 创建JWT管理器
func NewJWTManager(config *JWTConfig) *JWTManager {
	if config == nil {
		config = DefaultJWTConfig()
	}
	return &JWTManager{config: config}
}

// GenerateToken 生成Access Token
func (m *JWTManager) GenerateToken(userID, username string) (string, error) {
	return m.GenerateTokenWithOptions(userID, username, "", "")
}

// GenerateTokenWithOptions 生成带选项的Token
func (m *JWTManager) GenerateTokenWithOptions(userID, username, platform, deviceID string) (string, error) {
	now := time.Now()
	claims := &Claims{
		UserID:   userID,
		Username: username,
		Platform: platform,
		DeviceID: deviceID,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.config.Issuer,
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.config.Expire)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(m.config.Secret))
}

// GenerateRefreshToken 生成Refresh Token
func (m *JWTManager) GenerateRefreshToken(userID string) (string, error) {
	now := time.Now()
	claims := &Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.config.Issuer,
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.config.RefreshExpire)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(m.config.Secret))
}

// GenerateTokenPair 生成Token对（Access Token + Refresh Token）
func (m *JWTManager) GenerateTokenPair(userID, username, platform, deviceID string) (accessToken, refreshToken string, expiresAt time.Time, err error) {
	accessToken, err = m.GenerateTokenWithOptions(userID, username, platform, deviceID)
	if err != nil {
		return "", "", time.Time{}, err
	}

	refreshToken, err = m.GenerateRefreshToken(userID)
	if err != nil {
		return "", "", time.Time{}, err
	}

	expiresAt = time.Now().Add(m.config.Expire)
	return accessToken, refreshToken, expiresAt, nil
}

// ParseToken 解析并验证Token
func (m *JWTManager) ParseToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// 验证签名方法
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrSigningMethod
		}
		return []byte(m.config.Secret), nil
	})

	if err != nil {
		// 检查具体错误类型
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		if errors.Is(err, jwt.ErrTokenNotValidYet) {
			return nil, ErrTokenNotActive
		}
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidClaims
	}

	if claims.UserID == "" {
		return nil, ErrMissingUserID
	}

	return claims, nil
}

// ValidateToken 验证Token并返回UserID
func (m *JWTManager) ValidateToken(tokenString string) (string, error) {
	claims, err := m.ParseToken(tokenString)
	if err != nil {
		return "", err
	}
	return claims.UserID, nil
}

// RefreshToken 使用Refresh Token刷新Access Token
func (m *JWTManager) RefreshToken(refreshToken string) (newAccessToken string, err error) {
	claims, err := m.ParseToken(refreshToken)
	if err != nil {
		return "", err
	}

	// 生成新的Access Token
	return m.GenerateTokenWithOptions(claims.UserID, claims.Username, claims.Platform, claims.DeviceID)
}

// GetExpiresAt 获取Token过期时间
func (m *JWTManager) GetExpiresAt(tokenString string) (time.Time, error) {
	claims, err := m.ParseToken(tokenString)
	if err != nil {
		return time.Time{}, err
	}
	return claims.ExpiresAt.Time, nil
}

// IsExpired 检查Token是否过期
func (m *JWTManager) IsExpired(tokenString string) bool {
	claims, err := m.ParseToken(tokenString)
	if err != nil {
		return true
	}
	return claims.ExpiresAt.Time.Before(time.Now())
}

// GetRemainingTime 获取Token剩余有效时间
func (m *JWTManager) GetRemainingTime(tokenString string) (time.Duration, error) {
	claims, err := m.ParseToken(tokenString)
	if err != nil {
		return 0, err
	}
	remaining := time.Until(claims.ExpiresAt.Time)
	if remaining < 0 {
		return 0, ErrExpiredToken
	}
	return remaining, nil
}

// ExtractUserID 从Token中提取UserID（不验证有效期）
func ExtractUserID(tokenString, secret string) (string, error) {
	token, _, err := new(jwt.Parser).ParseUnverified(tokenString, &Claims{})
	if err != nil {
		return "", ErrInvalidToken
	}

	claims, ok := token.Claims.(*Claims)
	if !ok {
		return "", ErrInvalidClaims
	}

	return claims.UserID, nil
}

// 全局JWT管理器实例（可选）
var defaultManager *JWTManager

// InitDefaultManager 初始化默认管理器
func InitDefaultManager(config *JWTConfig) {
	defaultManager = NewJWTManager(config)
}

// GetDefaultManager 获取默认管理器
func GetDefaultManager() *JWTManager {
	if defaultManager == nil {
		defaultManager = NewJWTManager(nil)
	}
	return defaultManager
}

// 便捷函数（使用默认管理器）

// GenerateAccessToken 生成Access Token
func GenerateAccessToken(userID, username string) (string, error) {
	return GetDefaultManager().GenerateToken(userID, username)
}

// ValidateAccessToken 验证Access Token
func ValidateAccessToken(tokenString string) (string, error) {
	return GetDefaultManager().ValidateToken(tokenString)
}

// ParseAccessToken 解析Access Token
func ParseAccessToken(tokenString string) (*Claims, error) {
	return GetDefaultManager().ParseToken(tokenString)
}
