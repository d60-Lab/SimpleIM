// Package handler 提供HTTP请求处理器
package handler

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/d60-lab/im-system/internal/model"
	"github.com/d60-lab/im-system/pkg/auth"
	"github.com/d60-lab/im-system/pkg/util"
)

// UserHandler 用户处理器
type UserHandler struct {
	db         *gorm.DB
	jwtManager *auth.JWTManager
}

// NewUserHandler 创建用户处理器
func NewUserHandler(db *gorm.DB, jwtManager *auth.JWTManager) *UserHandler {
	return &UserHandler{
		db:         db,
		jwtManager: jwtManager,
	}
}

// RegisterRoutes 注册路由
func (h *UserHandler) RegisterRoutes(r *gin.Engine) {
	// 公开接口
	r.POST("/api/register", h.Register)
	r.POST("/api/login", h.Login)
	r.POST("/api/refresh-token", h.RefreshToken)

	// 需要认证的接口
	auth := r.Group("/api/user")
	auth.Use(AuthMiddleware())
	{
		auth.GET("/info", h.GetUserInfo)
		auth.PUT("/info", h.UpdateUserInfo)
		auth.POST("/change-password", h.ChangePassword)
		auth.POST("/logout", h.Logout)
	}

	// 用户查询接口
	r.GET("/api/users/:user_id", AuthMiddleware(), h.GetUserByID)
	r.GET("/api/users", AuthMiddleware(), h.SearchUsers)
}

// Register 用户注册
// @Summary		用户注册
// @Description	创建新用户账号
// @Tags			用户
// @Accept			json
// @Produce		json
// @Param			request	body		model.RegisterRequest	true	"注册信息"
// @Success		200		{object}	map[string]interface{}	"注册成功"
// @Failure		400		{object}	map[string]interface{}	"参数错误或用户名已存在"
// @Failure		500		{object}	map[string]interface{}	"服务器错误"
// @Router			/register [post]
func (h *UserHandler) Register(c *gin.Context) {
	var req model.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 检查用户名是否已存在
	var existingUser model.User
	if err := h.db.Where("username = ?", req.Username).First(&existingUser).Error; err == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username already exists"})
		return
	}

	// 哈希密码
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to hash password"})
		return
	}

	// 创建用户
	user := &model.User{
		UserID:       util.GenerateUserID(),
		Username:     req.Username,
		Nickname:     req.Nickname,
		PasswordHash: string(hashedPassword),
		Status:       model.UserStatusNormal,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if user.Nickname == "" {
		user.Nickname = req.Username
	}

	if err := h.db.Create(user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create user"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"user_id":  user.UserID,
			"username": user.Username,
			"nickname": user.Nickname,
		},
	})
}

// Login 用户登录
// @Summary		用户登录
// @Description	使用用户名密码登录，返回JWT令牌
// @Tags			用户
// @Accept			json
// @Produce		json
// @Param			request	body		model.LoginRequest		true	"登录信息"
// @Success		200		{object}	map[string]interface{}	"登录成功，返回token"
// @Failure		400		{object}	map[string]interface{}	"参数错误"
// @Failure		401		{object}	map[string]interface{}	"用户名或密码错误"
// @Router			/login [post]
func (h *UserHandler) Login(c *gin.Context) {
	var req model.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 查找用户
	var user model.User
	if err := h.db.Where("username = ?", req.Username).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid username or password"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to query user"})
		return
	}

	// 验证密码
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid username or password"})
		return
	}

	// 检查用户状态
	if user.Status != model.UserStatusNormal {
		c.JSON(http.StatusForbidden, gin.H{"error": "user is disabled"})
		return
	}

	// 获取平台信息
	platform := c.GetHeader("X-Platform")
	deviceID := c.GetHeader("X-Device-ID")

	// 生成Token
	accessToken, refreshToken, expiresAt, err := h.jwtManager.GenerateTokenPair(user.UserID, user.Username, platform, deviceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	// 获取WebSocket URL
	wsURL := getWebSocketURL(c)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": model.LoginResponse{
			UserID:       user.UserID,
			Username:     user.Username,
			Nickname:     user.Nickname,
			Avatar:       user.Avatar,
			Token:        accessToken,
			RefreshToken: refreshToken,
			ExpiresAt:    expiresAt,
			WebSocketURL: wsURL,
		},
	})
}

// RefreshToken 刷新Token
func (h *UserHandler) RefreshToken(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 验证并刷新Token
	newAccessToken, err := h.jwtManager.RefreshToken(req.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid refresh token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"token":      newAccessToken,
			"expires_at": time.Now().Add(7 * 24 * time.Hour),
		},
	})
}

// GetUserInfo 获取当前用户信息
// @Summary		获取当前用户信息
// @Description	获取当前登录用户的详细信息
// @Tags			用户
// @Accept			json
// @Produce		json
// @Security		BearerAuth
// @Success		200	{object}	map[string]interface{}	"用户信息"
// @Failure		401	{object}	map[string]interface{}	"未授权"
// @Failure		404	{object}	map[string]interface{}	"用户不存在"
// @Router			/user/info [get]
func (h *UserHandler) GetUserInfo(c *gin.Context) {
	userID := c.GetString("user_id")

	var user model.User
	if err := h.db.Where("user_id = ?", userID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    user.ToUserInfo(),
	})
}

// UpdateUserInfo 更新用户信息
// @Summary		更新用户信息
// @Description	更新当前用户的昵称、头像等信息
// @Tags			用户
// @Accept			json
// @Produce		json
// @Security		BearerAuth
// @Param			request	body		map[string]string		true	"更新信息"
// @Success		200		{object}	map[string]interface{}	"更新成功"
// @Failure		400		{object}	map[string]interface{}	"参数错误"
// @Failure		401		{object}	map[string]interface{}	"未授权"
// @Router			/user/info [put]
func (h *UserHandler) UpdateUserInfo(c *gin.Context) {
	userID := c.GetString("user_id")

	var req model.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updates := make(map[string]interface{})
	if req.Nickname != nil {
		updates["nickname"] = *req.Nickname
	}
	if req.Avatar != nil {
		updates["avatar"] = *req.Avatar
	}

	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no fields to update"})
		return
	}

	updates["updated_at"] = time.Now()

	if err := h.db.Model(&model.User{}).Where("user_id = ?", userID).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update user"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
	})
}

// ChangePassword 修改密码
// @Summary		修改密码
// @Description	修改当前用户的登录密码
// @Tags			用户
// @Accept			json
// @Produce		json
// @Security		BearerAuth
// @Param			request	body		map[string]string		true	"新旧密码"
// @Success		200		{object}	map[string]interface{}	"修改成功"
// @Failure		400		{object}	map[string]interface{}	"参数错误或旧密码错误"
// @Failure		401		{object}	map[string]interface{}	"未授权"
// @Router			/user/password [put]
func (h *UserHandler) ChangePassword(c *gin.Context) {
	userID := c.GetString("user_id")

	var req model.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 获取用户
	var user model.User
	if err := h.db.Where("user_id = ?", userID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	// 验证旧密码
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.OldPassword)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "incorrect old password"})
		return
	}

	// 哈希新密码
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to hash password"})
		return
	}

	// 更新密码
	if err := h.db.Model(&user).Updates(map[string]interface{}{
		"password_hash": string(hashedPassword),
		"updated_at":    time.Now(),
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update password"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
	})
}

// Logout 登出
func (h *UserHandler) Logout(c *gin.Context) {
	// 这里可以实现Token黑名单等逻辑
	// 简化实现：客户端直接删除Token即可

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
	})
}

// GetUserByID 根据ID获取用户信息
// @Summary		根据ID获取用户
// @Description	根据用户ID获取用户公开信息
// @Tags			用户
// @Accept			json
// @Produce		json
// @Security		BearerAuth
// @Param			user_id	path		string					true	"用户ID"
// @Success		200		{object}	map[string]interface{}	"用户信息"
// @Failure		401		{object}	map[string]interface{}	"未授权"
// @Failure		404		{object}	map[string]interface{}	"用户不存在"
// @Router			/users/{user_id} [get]
func (h *UserHandler) GetUserByID(c *gin.Context) {
	targetUserID := c.Param("user_id")

	var user model.User
	if err := h.db.Where("user_id = ?", targetUserID).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to query user"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    user.ToUserInfo(),
	})
}

// SearchUsers 搜索用户
// @Summary		搜索用户
// @Description	根据关键词搜索用户
// @Tags			用户
// @Accept			json
// @Produce		json
// @Security		BearerAuth
// @Param			keyword	query		string					true	"搜索关键词"
// @Param			limit	query		int						false	"返回数量限制"	default(20)
// @Success		200		{object}	map[string]interface{}	"用户列表"
// @Failure		400		{object}	map[string]interface{}	"参数错误"
// @Failure		401		{object}	map[string]interface{}	"未授权"
// @Router			/users [get]
func (h *UserHandler) SearchUsers(c *gin.Context) {
	keyword := c.Query("keyword")
	if keyword == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "keyword is required"})
		return
	}

	var users []model.User
	if err := h.db.Where("username LIKE ? OR nickname LIKE ?", "%"+keyword+"%", "%"+keyword+"%").
		Limit(20).
		Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to search users"})
		return
	}

	userInfos := make([]*model.UserInfo, 0, len(users))
	for _, user := range users {
		userInfos = append(userInfos, user.ToUserInfo())
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"total": len(userInfos),
			"users": userInfos,
		},
	})
}

// AuthMiddleware 认证中间件
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 获取Token
		token := c.GetHeader("Authorization")
		if token == "" {
			token = c.Query("token")
		}

		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing token"})
			c.Abort()
			return
		}

		// 去掉Bearer前缀
		if strings.HasPrefix(token, "Bearer ") {
			token = token[7:]
		}

		// 验证Token
		claims, err := auth.ParseAccessToken(token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			c.Abort()
			return
		}

		// 将用户信息存入上下文
		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)

		c.Next()
	}
}

// getWebSocketURL 获取WebSocket连接URL
func getWebSocketURL(c *gin.Context) string {
	scheme := "ws"
	if c.Request.TLS != nil {
		scheme = "wss"
	}

	host := c.Request.Host
	return scheme + "://" + host + "/ws"
}
