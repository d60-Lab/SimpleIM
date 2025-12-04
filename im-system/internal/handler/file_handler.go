// Package handler 文件上传处理
package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/d60-lab/im-system/internal/model"
	"github.com/d60-lab/im-system/internal/service"
	"github.com/gin-gonic/gin"
)

// FileHandler 文件处理器
type FileHandler struct {
	fileService service.FileStorageService
}

// NewFileHandler 创建文件处理器
func NewFileHandler(fileService service.FileStorageService) *FileHandler {
	return &FileHandler{
		fileService: fileService,
	}
}

// RegisterRoutes 注册路由
func (h *FileHandler) RegisterRoutes(r *gin.Engine) {
	file := r.Group("/api/file")
	file.Use(AuthMiddleware())
	{
		file.POST("/upload", h.Upload)
		file.GET("/info/:file_id", h.GetFileInfo)
		file.GET("/url/:file_id", h.GetFileURL)
		file.GET("/download/:file_id", h.Download)
		file.DELETE("/:file_id", h.Delete)

		// 分片上传
		file.POST("/multipart/init", h.InitMultipartUpload)
		file.POST("/multipart/upload", h.UploadPart)
		file.POST("/multipart/complete", h.CompleteMultipartUpload)
		file.POST("/multipart/abort", h.AbortMultipartUpload)
	}
}

// Upload 上传文件
// @Summary		上传文件
// @Description	上传图片、文档等文件
// @Tags			文件
// @Accept			multipart/form-data
// @Produce		json
// @Security		BearerAuth
// @Param			file	formData	file					true	"文件"
// @Success		200		{object}	map[string]interface{}	"上传成功"
// @Failure		400		{object}	map[string]interface{}	"参数错误"
// @Failure		401		{object}	map[string]interface{}	"未授权"
// @Failure		500		{object}	map[string]interface{}	"上传失败"
// @Router			/file/upload [post]
func (h *FileHandler) Upload(c *gin.Context) {
	userID := c.GetString("user_id")

	// 获取上传的文件
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "文件上传失败: " + err.Error(),
		})
		return
	}
	defer file.Close()

	// 获取内容类型
	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// 上传文件
	req := &service.UploadRequest{
		File:        file,
		Header:      header,
		UserID:      userID,
		ContentType: contentType,
	}

	fileInfo, err := h.fileService.Upload(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "文件上传失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    fileInfo,
	})
}

// GetFileInfo 获取文件信息
// @Summary		获取文件信息
// @Description	根据文件ID获取文件详细信息
// @Tags			文件
// @Accept			json
// @Produce		json
// @Security		BearerAuth
// @Param			file_id	path		string					true	"文件ID"
// @Success		200		{object}	map[string]interface{}	"文件信息"
// @Failure		401		{object}	map[string]interface{}	"未授权"
// @Failure		404		{object}	map[string]interface{}	"文件不存在"
// @Router			/file/info/{file_id} [get]
func (h *FileHandler) GetFileInfo(c *gin.Context) {
	fileID := c.Param("file_id")

	fileInfo, err := h.fileService.GetFileInfo(c.Request.Context(), fileID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "文件不存在",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    fileInfo,
	})
}

// GetFileURL 获取文件URL
// @Summary		获取文件访问URL
// @Description	获取文件的临时访问URL
// @Tags			文件
// @Accept			json
// @Produce		json
// @Security		BearerAuth
// @Param			file_id	path		string					true	"文件ID"
// @Param			expiry	query		int						false	"过期时间(秒)"	default(3600)
// @Success		200		{object}	map[string]interface{}	"文件URL"
// @Failure		401		{object}	map[string]interface{}	"未授权"
// @Failure		404		{object}	map[string]interface{}	"文件不存在"
// @Router			/file/url/{file_id} [get]
func (h *FileHandler) GetFileURL(c *gin.Context) {
	fileID := c.Param("file_id")

	// 过期时间，默认1小时
	expiry := time.Hour
	if expiryStr := c.Query("expiry"); expiryStr != "" {
		if seconds, err := strconv.Atoi(expiryStr); err == nil {
			expiry = time.Duration(seconds) * time.Second
		}
	}

	url, err := h.fileService.GetFileURL(c.Request.Context(), fileID, expiry)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "文件不存在",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"url":       url,
			"expire_at": time.Now().Add(expiry).Unix(),
		},
	})
}

// Download 下载文件
// @Summary		下载文件
// @Description	下载指定文件
// @Tags			文件
// @Accept			json
// @Produce		octet-stream
// @Security		BearerAuth
// @Param			file_id	path	string	true	"文件ID"
// @Success		200		"文件内容"
// @Failure		401		{object}	map[string]interface{}	"未授权"
// @Failure		404		{object}	map[string]interface{}	"文件不存在"
// @Router			/file/download/{file_id} [get]
func (h *FileHandler) Download(c *gin.Context) {
	fileID := c.Param("file_id")

	reader, fileInfo, err := h.fileService.Download(c.Request.Context(), fileID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "文件不存在",
		})
		return
	}
	defer reader.Close()

	// 设置响应头
	c.Header("Content-Type", fileInfo.MimeType)
	c.Header("Content-Disposition", "attachment; filename=\""+fileInfo.FileName+"\"")
	c.Header("Content-Length", strconv.FormatInt(fileInfo.FileSize, 10))

	c.DataFromReader(http.StatusOK, fileInfo.FileSize, fileInfo.MimeType, reader, nil)
}

// Delete 删除文件
// @Summary		删除文件
// @Description	删除指定文件
// @Tags			文件
// @Accept			json
// @Produce		json
// @Security		BearerAuth
// @Param			file_id	path		string					true	"文件ID"
// @Success		200		{object}	map[string]interface{}	"删除成功"
// @Failure		401		{object}	map[string]interface{}	"未授权"
// @Failure		404		{object}	map[string]interface{}	"文件不存在"
// @Failure		500		{object}	map[string]interface{}	"删除失败"
// @Router			/file/{file_id} [delete]
func (h *FileHandler) Delete(c *gin.Context) {
	fileID := c.Param("file_id")

	if err := h.fileService.Delete(c.Request.Context(), fileID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "删除失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
	})
}

// InitMultipartUpload 初始化分片上传
// @Summary		初始化分片上传
// @Description	初始化大文件分片上传
// @Tags			文件
// @Accept			json
// @Produce		json
// @Security		BearerAuth
// @Param			request	body		model.InitMultipartUploadRequest	true	"上传信息"
// @Success		200		{object}	map[string]interface{}				"初始化成功"
// @Failure		400		{object}	map[string]interface{}				"参数错误"
// @Failure		401		{object}	map[string]interface{}				"未授权"
// @Failure		500		{object}	map[string]interface{}				"初始化失败"
// @Router			/file/multipart/init [post]
func (h *FileHandler) InitMultipartUpload(c *gin.Context) {
	userID := c.GetString("user_id")

	var req model.InitMultipartUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "参数错误: " + err.Error(),
		})
		return
	}

	resp, err := h.fileService.InitMultipartUpload(c.Request.Context(), &req, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "初始化分片上传失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    resp,
	})
}

// UploadPart 上传分片
// @Summary		上传分片
// @Description	上传文件分片
// @Tags			文件
// @Accept			multipart/form-data
// @Produce		json
// @Security		BearerAuth
// @Param			upload_id	formData	string					true	"上传ID"
// @Param			part_number	formData	int						true	"分片序号"
// @Param			file		formData	file					true	"分片文件"
// @Success		200			{object}	map[string]interface{}	"上传成功"
// @Failure		400			{object}	map[string]interface{}	"参数错误"
// @Failure		401			{object}	map[string]interface{}	"未授权"
// @Failure		500			{object}	map[string]interface{}	"上传失败"
// @Router			/file/multipart/upload [post]
func (h *FileHandler) UploadPart(c *gin.Context) {
	uploadID := c.PostForm("upload_id")
	partNumberStr := c.PostForm("part_number")

	if uploadID == "" || partNumberStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "缺少upload_id或part_number",
		})
		return
	}

	partNumber, err := strconv.Atoi(partNumberStr)
	if err != nil || partNumber < 1 {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的part_number",
		})
		return
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "获取文件失败: " + err.Error(),
		})
		return
	}
	defer file.Close()

	partInfo, err := h.fileService.UploadPart(c.Request.Context(), uploadID, partNumber, file, header.Size)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "上传分片失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    partInfo,
	})
}

// CompleteMultipartUpload 完成分片上传
// @Summary		完成分片上传
// @Description	完成分片上传，合并文件
// @Tags			文件
// @Accept			json
// @Produce		json
// @Security		BearerAuth
// @Param			request	body		object{upload_id=string,parts=[]object}	true	"分片信息"
// @Success		200		{object}	map[string]interface{}					"完成成功"
// @Failure		400		{object}	map[string]interface{}					"参数错误"
// @Failure		401		{object}	map[string]interface{}					"未授权"
// @Failure		500		{object}	map[string]interface{}					"完成失败"
// @Router			/file/multipart/complete [post]
func (h *FileHandler) CompleteMultipartUpload(c *gin.Context) {
	var req struct {
		UploadID string            `json:"upload_id" binding:"required"`
		Parts    []*model.PartInfo `json:"parts" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "参数错误: " + err.Error(),
		})
		return
	}

	fileInfo, err := h.fileService.CompleteMultipartUpload(c.Request.Context(), req.UploadID, req.Parts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "完成分片上传失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    fileInfo,
	})
}

// AbortMultipartUpload 取消分片上传
// @Summary		取消分片上传
// @Description	取消分片上传，清理已上传分片
// @Tags			文件
// @Accept			json
// @Produce		json
// @Security		BearerAuth
// @Param			request	body		object{upload_id=string}	true	"上传ID"
// @Success		200		{object}	map[string]interface{}		"取消成功"
// @Failure		400		{object}	map[string]interface{}		"参数错误"
// @Failure		401		{object}	map[string]interface{}		"未授权"
// @Failure		500		{object}	map[string]interface{}		"取消失败"
// @Router			/file/multipart/abort [post]
func (h *FileHandler) AbortMultipartUpload(c *gin.Context) {
	var req struct {
		UploadID string `json:"upload_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "参数错误: " + err.Error(),
		})
		return
	}

	if err := h.fileService.AbortMultipartUpload(c.Request.Context(), req.UploadID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "取消分片上传失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
	})
}
