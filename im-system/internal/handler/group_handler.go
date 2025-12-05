// Package handler 提供HTTP请求处理器
package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/d60-lab/im-system/internal/model"
	"github.com/d60-lab/im-system/internal/service"
)

// GroupHandler 群组处理器
type GroupHandler struct {
	groupService service.GroupService
}

// NewGroupHandler 创建群组处理器
func NewGroupHandler(groupService service.GroupService) *GroupHandler {
	return &GroupHandler{
		groupService: groupService,
	}
}

// RegisterRoutes 注册路由
func (h *GroupHandler) RegisterRoutes(r *gin.Engine) {
	group := r.Group("/api/groups")
	group.Use(AuthMiddleware())
	{
		group.POST("", h.CreateGroup)
		group.GET("/:group_id", h.GetGroupInfo)
		group.PUT("/:group_id", h.UpdateGroupInfo)
		group.DELETE("/:group_id", h.DismissGroup)

		group.POST("/:group_id/join", h.JoinGroup)
		group.POST("/:group_id/leave", h.LeaveGroup)
		group.POST("/:group_id/kick", h.KickMember)
		group.GET("/:group_id/members", h.GetGroupMembers)

		group.POST("/:group_id/admin", h.SetAdmin)
		group.POST("/:group_id/transfer", h.TransferOwner)
		group.POST("/:group_id/mute", h.MuteMember)
		group.POST("/:group_id/mute-all", h.SetMuteAll)
	}

	// 用户相关群组接口
	r.GET("/api/groups/my", AuthMiddleware(), h.GetUserGroups) //
}

// CreateGroup 创建群组
// @Summary		创建群组
// @Description	创建一个新的群组
// @Tags			群组
// @Accept			json
// @Produce		json
// @Security		BearerAuth
// @Param			request	body		object{name=string,avatar=string,description=string,member_ids=[]string}	true	"群组信息"
// @Success		200		{object}	map[string]interface{}													"创建成功"
// @Failure		400		{object}	map[string]interface{}													"参数错误"
// @Failure		401		{object}	map[string]interface{}													"未授权"
// @Failure		500		{object}	map[string]interface{}													"服务器错误"
// @Router			/groups [post]
func (h *GroupHandler) CreateGroup(c *gin.Context) {
	userID := c.GetString("user_id")

	var req struct {
		Name        string   `json:"name" binding:"required,max=128"`
		Avatar      string   `json:"avatar"`
		Description string   `json:"description" binding:"max=512"`
		MemberIDs   []string `json:"member_ids"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	createReq := &model.CreateGroupRequest{
		OwnerID:     userID,
		Name:        req.Name,
		Avatar:      req.Avatar,
		Description: req.Description,
		MemberIDs:   req.MemberIDs,
	}

	group, err := h.groupService.CreateGroup(c.Request.Context(), createReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    group,
	})
}

// GetGroupInfo 获取群信息
// @Summary		获取群组信息
// @Description	根据群组ID获取群组详细信息
// @Tags			群组
// @Accept			json
// @Produce		json
// @Security		BearerAuth
// @Param			group_id	path		string					true	"群组ID"
// @Success		200			{object}	map[string]interface{}	"群组信息"
// @Failure		401			{object}	map[string]interface{}	"未授权"
// @Failure		404			{object}	map[string]interface{}	"群组不存在"
// @Router			/groups/{group_id} [get]
func (h *GroupHandler) GetGroupInfo(c *gin.Context) {
	groupID := c.Param("group_id")

	group, err := h.groupService.GetGroupInfo(c.Request.Context(), groupID)
	if err != nil {
		if err == service.ErrGroupNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "group not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    group,
	})
}

// UpdateGroupInfo 更新群信息
// @Summary		更新群组信息
// @Description	更新群组的名称、头像、公告等信息
// @Tags			群组
// @Accept			json
// @Produce		json
// @Security		BearerAuth
// @Param			group_id	path		string					true	"群组ID"
// @Param			request		body		object					true	"更新信息"
// @Success		200			{object}	map[string]interface{}	"更新成功"
// @Failure		400			{object}	map[string]interface{}	"参数错误"
// @Failure		401			{object}	map[string]interface{}	"未授权"
// @Failure		403			{object}	map[string]interface{}	"无权限"
// @Router			/groups/{group_id} [put]
func (h *GroupHandler) UpdateGroupInfo(c *gin.Context) {
	userID := c.GetString("user_id")
	groupID := c.Param("group_id")

	var req struct {
		Name         *string `json:"name"`
		Avatar       *string `json:"avatar"`
		Announcement *string `json:"announcement"`
		Description  *string `json:"description"`
		JoinMode     *int    `json:"join_mode"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updateReq := &model.UpdateGroupRequest{
		GroupID:      groupID,
		OperatorID:   userID,
		Name:         req.Name,
		Avatar:       req.Avatar,
		Announcement: req.Announcement,
		Description:  req.Description,
		JoinMode:     req.JoinMode,
	}

	if err := h.groupService.UpdateGroupInfo(c.Request.Context(), updateReq); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
	})
}

// DismissGroup 解散群组
// @Summary		解散群组
// @Description	解散指定群组，仅群主可操作
// @Tags			群组
// @Accept			json
// @Produce		json
// @Security		BearerAuth
// @Param			group_id	path		string					true	"群组ID"
// @Success		200			{object}	map[string]interface{}	"解散成功"
// @Failure		401			{object}	map[string]interface{}	"未授权"
// @Failure		403			{object}	map[string]interface{}	"无权限"
// @Failure		404			{object}	map[string]interface{}	"群组不存在"
// @Router			/groups/{group_id} [delete]
func (h *GroupHandler) DismissGroup(c *gin.Context) {
	userID := c.GetString("user_id")
	groupID := c.Param("group_id")

	if err := h.groupService.DismissGroup(c.Request.Context(), groupID, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
	})
}

// JoinGroup 加入群组
// @Summary		加入群组
// @Description	申请加入指定群组
// @Tags			群组
// @Accept			json
// @Produce		json
// @Security		BearerAuth
// @Param			group_id	path		string					true	"群组ID"
// @Success		200			{object}	map[string]interface{}	"加入成功"
// @Failure		400			{object}	map[string]interface{}	"已在群中"
// @Failure		401			{object}	map[string]interface{}	"未授权"
// @Failure		404			{object}	map[string]interface{}	"群组不存在"
// @Router			/groups/{group_id}/join [post]
func (h *GroupHandler) JoinGroup(c *gin.Context) {
	userID := c.GetString("user_id")
	groupID := c.Param("group_id")

	if err := h.groupService.JoinGroup(c.Request.Context(), groupID, userID, ""); err != nil {
		if err == service.ErrAlreadyInGroup {
			c.JSON(http.StatusBadRequest, gin.H{"error": "already in group"})
			return
		}
		if err == service.ErrGroupFull {
			c.JSON(http.StatusBadRequest, gin.H{"error": "group is full"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
	})
}

// LeaveGroup 退出群组
// @Summary		退出群组
// @Description	退出指定群组
// @Tags			群组
// @Accept			json
// @Produce		json
// @Security		BearerAuth
// @Param			group_id	path		string					true	"群组ID"
// @Success		200			{object}	map[string]interface{}	"退出成功"
// @Failure		401			{object}	map[string]interface{}	"未授权"
// @Failure		403			{object}	map[string]interface{}	"群主不能退出"
// @Failure		404			{object}	map[string]interface{}	"群组不存在"
// @Router			/groups/{group_id}/leave [post]
func (h *GroupHandler) LeaveGroup(c *gin.Context) {
	userID := c.GetString("user_id")
	groupID := c.Param("group_id")

	if err := h.groupService.LeaveGroup(c.Request.Context(), groupID, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
	})
}

// KickMember 踢出成员
// @Summary		踢出群成员
// @Description	将指定成员踢出群组
// @Tags			群组
// @Accept			json
// @Produce		json
// @Security		BearerAuth
// @Param			group_id	path		string					true	"群组ID"
// @Param			user_id		path		string					true	"被踢用户ID"
// @Success		200			{object}	map[string]interface{}	"踢出成功"
// @Failure		401			{object}	map[string]interface{}	"未授权"
// @Failure		403			{object}	map[string]interface{}	"无权限"
// @Failure		404			{object}	map[string]interface{}	"用户不在群中"
// @Router			/groups/{group_id}/members/{user_id} [delete]
func (h *GroupHandler) KickMember(c *gin.Context) {
	userID := c.GetString("user_id")
	groupID := c.Param("group_id")

	var req struct {
		TargetIDs []string `json:"target_ids" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.groupService.KickMember(c.Request.Context(), groupID, userID, req.TargetIDs); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
	})
}

// GetGroupMembers 获取群成员列表
// @Summary		获取群成员列表
// @Description	获取指定群组的成员列表
// @Tags			群组
// @Accept			json
// @Produce		json
// @Security		BearerAuth
// @Param			group_id	path		string					true	"群组ID"
// @Success		200			{object}	map[string]interface{}	"成员列表"
// @Failure		401			{object}	map[string]interface{}	"未授权"
// @Failure		404			{object}	map[string]interface{}	"群组不存在"
// @Router			/groups/{group_id}/members [get]
func (h *GroupHandler) GetGroupMembers(c *gin.Context) {
	groupID := c.Param("group_id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	members, total, err := h.groupService.GetGroupMembers(c.Request.Context(), groupID, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"total":   total,
			"members": members,
		},
	})
}

// SetAdmin 设置/取消管理员
func (h *GroupHandler) SetAdmin(c *gin.Context) {
	userID := c.GetString("user_id")
	groupID := c.Param("group_id")

	var req struct {
		TargetID string `json:"target_id" binding:"required"`
		IsAdmin  bool   `json:"is_admin"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.groupService.SetAdmin(c.Request.Context(), groupID, userID, req.TargetID, req.IsAdmin); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
	})
}

// TransferOwner 转让群主
func (h *GroupHandler) TransferOwner(c *gin.Context) {
	userID := c.GetString("user_id")
	groupID := c.Param("group_id")

	var req struct {
		NewOwnerID string `json:"new_owner_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.groupService.TransferOwner(c.Request.Context(), groupID, userID, req.NewOwnerID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
	})
}

// MuteMember 禁言成员
func (h *GroupHandler) MuteMember(c *gin.Context) {
	userID := c.GetString("user_id")
	groupID := c.Param("group_id")

	var req struct {
		TargetID string `json:"target_id" binding:"required"`
		Duration int    `json:"duration"` // 秒，0表示取消禁言
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	duration := time.Duration(0)
	if req.Duration > 0 {
		duration = time.Duration(req.Duration) * time.Second
	}

	if err := h.groupService.MuteMember(c.Request.Context(), groupID, userID, req.TargetID, duration); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
	})
}

// SetMuteAll 设置全员禁言
func (h *GroupHandler) SetMuteAll(c *gin.Context) {
	userID := c.GetString("user_id")
	groupID := c.Param("group_id")

	var req struct {
		MuteAll bool `json:"mute_all"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.groupService.SetMuteAll(c.Request.Context(), groupID, userID, req.MuteAll); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
	})
}

// GetUserGroups 获取用户的群组列表
// @Summary		获取我的群组列表
// @Description	获取当前用户加入的所有群组
// @Tags			群组
// @Accept			json
// @Produce		json
// @Security		BearerAuth
// @Success		200	{object}	map[string]interface{}	"群组列表"
// @Failure		401	{object}	map[string]interface{}	"未授权"
// @Router			/user/groups [get]
func (h *GroupHandler) GetUserGroups(c *gin.Context) {
	userID := c.GetString("user_id")

	groups, err := h.groupService.GetUserGroups(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"total":  len(groups),
			"groups": groups,
		},
	})
}
