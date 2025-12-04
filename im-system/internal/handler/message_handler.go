// Package handler 消息相关API处理
package handler

import (
	"net/http"
	"strconv"

	"github.com/d60-lab/im-system/internal/service"
	"github.com/gin-gonic/gin"
)

// MessageHandler 消息处理器
type MessageHandler struct {
	messageService service.MessageService
}

// NewMessageHandler 创建消息处理器
func NewMessageHandler(messageService service.MessageService) *MessageHandler {
	return &MessageHandler{
		messageService: messageService,
	}
}

// RegisterRoutes 注册路由
func (h *MessageHandler) RegisterRoutes(router *gin.RouterGroup) {
	messages := router.Group("/messages")
	{
		messages.GET("/conversation/:conversation_id", h.GetConversationMessages)
		messages.GET("/group/:group_id", h.GetGroupMessages)
		messages.GET("/private/:user_id", h.GetPrivateMessages)
	}
}

// GetConversationMessages 获取会话消息历史
// @Summary		获取会话消息历史
// @Description	根据会话ID获取消息历史记录
// @Tags			消息
// @Accept			json
// @Produce		json
// @Security		BearerAuth
// @Param			conversation_id	path		string					true	"会话ID"
// @Param			last_seq		query		int						false	"上次消息序号"
// @Param			limit			query		int						false	"返回数量"	default(50)
// @Success		200				{object}	map[string]interface{}	"消息列表"
// @Failure		401				{object}	map[string]interface{}	"未授权"
// @Failure		500				{object}	map[string]interface{}	"服务器错误"
// @Router			/messages/conversation/{conversation_id} [get]
func (h *MessageHandler) GetConversationMessages(c *gin.Context) {
	userID := c.GetString("user_id")
	conversationID := c.Param("conversation_id")

	// 分页参数
	lastSeq, _ := strconv.ParseInt(c.DefaultQuery("last_seq", "0"), 10, 64)
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))

	if limit <= 0 || limit > 100 {
		limit = 50
	}

	messages, err := h.messageService.GetConversationMessages(c.Request.Context(), userID, conversationID, lastSeq, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"messages": messages,
			"has_more": len(messages) >= limit,
		},
	})
}

// GetGroupMessages 获取群聊消息历史
// @Summary		获取群聊消息历史
// @Description	根据群组ID获取群聊消息历史记录
// @Tags			消息
// @Accept			json
// @Produce		json
// @Security		BearerAuth
// @Param			group_id	path		string					true	"群组ID"
// @Param			last_seq	query		int						false	"上次消息序号"
// @Param			limit		query		int						false	"返回数量"	default(50)
// @Success		200			{object}	map[string]interface{}	"消息列表"
// @Failure		401			{object}	map[string]interface{}	"未授权"
// @Failure		500			{object}	map[string]interface{}	"服务器错误"
// @Router			/messages/group/{group_id} [get]
func (h *MessageHandler) GetGroupMessages(c *gin.Context) {
	userID := c.GetString("user_id")
	groupID := c.Param("group_id")

	// 分页参数
	lastSeq, _ := strconv.ParseInt(c.DefaultQuery("last_seq", "0"), 10, 64)
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))

	if limit <= 0 || limit > 100 {
		limit = 50
	}

	// 验证用户是否是群成员
	messages, err := h.messageService.GetGroupMessages(c.Request.Context(), userID, groupID, lastSeq, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"messages": messages,
			"has_more": len(messages) >= limit,
		},
	})
}

// GetPrivateMessages 获取私聊消息历史
// @Summary		获取私聊消息历史
// @Description	根据对方用户ID获取私聊消息历史记录
// @Tags			消息
// @Accept			json
// @Produce		json
// @Security		BearerAuth
// @Param			user_id		path		string					true	"对方用户ID"
// @Param			last_seq	query		int						false	"上次消息序号"
// @Param			limit		query		int						false	"返回数量"	default(50)
// @Success		200			{object}	map[string]interface{}	"消息列表"
// @Failure		401			{object}	map[string]interface{}	"未授权"
// @Failure		500			{object}	map[string]interface{}	"服务器错误"
// @Router			/messages/private/{user_id} [get]
func (h *MessageHandler) GetPrivateMessages(c *gin.Context) {
	userID := c.GetString("user_id")
	otherUserID := c.Param("user_id")

	// 分页参数
	lastSeq, _ := strconv.ParseInt(c.DefaultQuery("last_seq", "0"), 10, 64)
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))

	if limit <= 0 || limit > 100 {
		limit = 50
	}

	messages, err := h.messageService.GetPrivateMessages(c.Request.Context(), userID, otherUserID, lastSeq, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"messages": messages,
			"has_more": len(messages) >= limit,
		},
	})
}
