// Package handler 提供HTTP请求处理器
package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/d60-lab/im-system/internal/model"
	"github.com/d60-lab/im-system/internal/service"
)

// OfflineHandler 离线消息处理器
type OfflineHandler struct {
	offlineService service.OfflineService
}

// NewOfflineHandler 创建离线消息处理器
func NewOfflineHandler(offlineService service.OfflineService) *OfflineHandler {
	return &OfflineHandler{
		offlineService: offlineService,
	}
}

// RegisterRoutes 注册路由
func (h *OfflineHandler) RegisterRoutes(r *gin.Engine) {
	offline := r.Group("/api/offline")
	offline.Use(AuthMiddleware())
	{
		offline.GET("/messages", h.PullMessages)
		offline.POST("/ack", h.AckMessages)
		offline.GET("/count", h.GetMessageCount)
		offline.GET("/summary", h.GetMessageSummary)
	}
}

// PullMessages 拉取离线消息
func (h *OfflineHandler) PullMessages(c *gin.Context) {
	userID := c.GetString("user_id")

	// 解析请求参数
	lastSeq, _ := strconv.ParseInt(c.DefaultQuery("last_seq", "0"), 10, 64)
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))

	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}

	// 拉取离线消息
	messages, err := h.offlineService.PullOfflineMessages(c.Request.Context(), userID, lastSeq, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 计算是否还有更多消息
	hasMore := len(messages) >= limit

	// 获取最后的序列号
	var newLastSeq int64 = lastSeq
	if len(messages) > 0 {
		newLastSeq = int64(messages[len(messages)-1].ID)
	}

	// 解析消息内容
	parsedMessages := make([]map[string]interface{}, 0, len(messages))
	for _, msg := range messages {
		parsedMsg, err := service.ParseOfflineMessage(msg)
		if err != nil {
			continue
		}
		parsedMessages = append(parsedMessages, map[string]interface{}{
			"id":              msg.ID,
			"message_id":      msg.MessageID,
			"conversation_id": msg.ConversationID,
			"message":         parsedMsg,
			"created_at":      msg.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"messages": parsedMessages,
			"has_more": hasMore,
			"last_seq": newLastSeq,
		},
	})
}

// AckMessages 确认离线消息（删除）
func (h *OfflineHandler) AckMessages(c *gin.Context) {
	userID := c.GetString("user_id")

	var req struct {
		MessageIDs []string `json:"message_ids" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if len(req.MessageIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "message_ids is required"})
		return
	}

	if len(req.MessageIDs) > 100 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "too many message_ids, max is 100"})
		return
	}

	// 删除离线消息
	if err := h.offlineService.DeleteOfflineMessages(c.Request.Context(), userID, req.MessageIDs); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
	})
}

// GetMessageCount 获取离线消息数量
func (h *OfflineHandler) GetMessageCount(c *gin.Context) {
	userID := c.GetString("user_id")

	count, err := h.offlineService.GetOfflineMessageCount(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"count": count,
		},
	})
}

// GetMessageSummary 获取离线消息摘要
func (h *OfflineHandler) GetMessageSummary(c *gin.Context) {
	userID := c.GetString("user_id")

	// 获取总数
	count, err := h.offlineService.GetOfflineMessageCount(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 获取未推送数量
	unpushedMessages, err := h.offlineService.GetUnpushedMessages(c.Request.Context(), userID, 1)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	unpushedCount := int64(len(unpushedMessages))

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"user_id":        userID,
			"total_count":    count,
			"unpushed_count": unpushedCount,
		},
	})
}

// RegisterDeviceHandler 设备注册处理器
type RegisterDeviceHandler struct {
	pushService service.PushService
}

// NewRegisterDeviceHandler 创建设备注册处理器
func NewRegisterDeviceHandler(pushService service.PushService) *RegisterDeviceHandler {
	return &RegisterDeviceHandler{
		pushService: pushService,
	}
}

// RegisterRoutes 注册路由
func (h *RegisterDeviceHandler) RegisterRoutes(r *gin.Engine) {
	device := r.Group("/api/device")
	device.Use(AuthMiddleware())
	{
		device.POST("/register", h.RegisterDevice)
		device.POST("/unregister", h.UnregisterDevice)
	}
}

// RegisterDevice 注册设备
func (h *RegisterDeviceHandler) RegisterDevice(c *gin.Context) {
	userID := c.GetString("user_id")

	var req model.RegisterDeviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.pushService.RegisterDevice(c.Request.Context(), userID, &req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
	})
}

// UnregisterDevice 注销设备
func (h *RegisterDeviceHandler) UnregisterDevice(c *gin.Context) {
	userID := c.GetString("user_id")

	var req model.UnregisterDeviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.pushService.UnregisterDevice(c.Request.Context(), userID, req.DeviceToken); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
	})
}
