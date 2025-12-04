// Package service 提供业务逻辑服务
package service

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/d60-lab/im-system/internal/model"
	"github.com/d60-lab/im-system/pkg/util"
	"github.com/go-redis/redis/v8"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"gorm.io/gorm"
)

// 文件存储服务错误定义
var (
	ErrFileNotFound        = errors.New("file not found")
	ErrFileTooLarge        = errors.New("file too large")
	ErrInvalidFileType     = errors.New("invalid file type")
	ErrUploadFailed        = errors.New("upload failed")
	ErrStorageUnavailable  = errors.New("storage service unavailable")
	ErrInvalidUploadID     = errors.New("invalid upload id")
	ErrPartNumberInvalid   = errors.New("invalid part number")
	ErrMultipartIncomplete = errors.New("multipart upload incomplete")
)

// FileStorageService 文件存储服务接口
type FileStorageService interface {
	// 基础操作
	Upload(ctx context.Context, req *UploadRequest) (*model.FileInfo, error)
	Download(ctx context.Context, fileID string) (io.ReadCloser, *model.FileInfo, error)
	Delete(ctx context.Context, fileID string) error
	GetFileInfo(ctx context.Context, fileID string) (*model.FileInfo, error)
	GetFileURL(ctx context.Context, fileID string, expiry time.Duration) (string, error)

	// 分片上传
	InitMultipartUpload(ctx context.Context, req *model.InitMultipartUploadRequest, userID string) (*model.InitMultipartUploadResponse, error)
	UploadPart(ctx context.Context, uploadID string, partNumber int, reader io.Reader, size int64) (*model.PartInfo, error)
	CompleteMultipartUpload(ctx context.Context, uploadID string, parts []*model.PartInfo) (*model.FileInfo, error)
	AbortMultipartUpload(ctx context.Context, uploadID string) error

	// 缩略图
	GenerateThumbnail(ctx context.Context, fileID string, width, height int) (string, error)

	// 秒传检测
	CheckFileExists(ctx context.Context, md5Hash string) (*model.FileInfo, bool, error)
}

// UploadRequest 上传请求
type UploadRequest struct {
	File        multipart.File
	Header      *multipart.FileHeader
	UserID      string
	ContentType string
}

// StorageConfig 存储配置
type StorageConfig struct {
	Provider    string // minio, aliyun, aws, qiniu
	Endpoint    string
	AccessKey   string
	SecretKey   string
	Bucket      string
	Region      string
	UseSSL      bool
	CDNDomain   string
	MaxFileSize int64 // 最大文件大小（字节）

	// 文件大小限制
	MaxImageSize int64
	MaxVideoSize int64
	MaxAudioSize int64

	// 分片上传配置
	ChunkSize int64 // 分片大小

	// 签名URL过期时间
	SignedURLExpiry time.Duration
}

// DefaultStorageConfig 默认存储配置
func DefaultStorageConfig() *StorageConfig {
	return &StorageConfig{
		Provider:        "minio",
		Endpoint:        "localhost:9000",
		AccessKey:       "minioadmin",
		SecretKey:       "minioadmin123",
		Bucket:          "im-files",
		Region:          "cn-north-1",
		UseSSL:          false,
		MaxFileSize:     100 * 1024 * 1024, // 100MB
		MaxImageSize:    10 * 1024 * 1024,  // 10MB
		MaxVideoSize:    100 * 1024 * 1024, // 100MB
		MaxAudioSize:    20 * 1024 * 1024,  // 20MB
		ChunkSize:       5 * 1024 * 1024,   // 5MB
		SignedURLExpiry: 2 * time.Hour,
	}
}

// minioStorageService MinIO存储服务实现
type minioStorageService struct {
	config    *StorageConfig
	client    *minio.Client
	db        *gorm.DB
	redis     *redis.Client
	cdnDomain string

	// 分片上传信息缓存
	multipartUploads map[string]*MultipartUploadState
}

// MultipartUploadState 分片上传状态
type MultipartUploadState struct {
	UploadID    string
	FileID      string
	FileName    string
	FileSize    int64
	ContentType string
	UserID      string
	ObjectPath  string
	TotalParts  int
	ChunkSize   int64
	Parts       map[int]*model.PartInfo
	CreatedAt   time.Time
}

// NewMinioStorageService 创建MinIO存储服务
func NewMinioStorageService(config *StorageConfig, db *gorm.DB, redisClient *redis.Client) (FileStorageService, error) {
	if config == nil {
		config = DefaultStorageConfig()
	}

	// 创建MinIO客户端
	client, err := minio.New(config.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(config.AccessKey, config.SecretKey, ""),
		Secure: config.UseSSL,
		Region: config.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("create minio client error: %w", err)
	}

	// 检查桶是否存在，不存在则创建
	ctx := context.Background()
	exists, err := client.BucketExists(ctx, config.Bucket)
	if err != nil {
		return nil, fmt.Errorf("check bucket exists error: %w", err)
	}

	if !exists {
		err = client.MakeBucket(ctx, config.Bucket, minio.MakeBucketOptions{
			Region: config.Region,
		})
		if err != nil {
			return nil, fmt.Errorf("create bucket error: %w", err)
		}
	}

	return &minioStorageService{
		config:           config,
		client:           client,
		db:               db,
		redis:            redisClient,
		cdnDomain:        config.CDNDomain,
		multipartUploads: make(map[string]*MultipartUploadState),
	}, nil
}

// Upload 上传文件
func (s *minioStorageService) Upload(ctx context.Context, req *UploadRequest) (*model.FileInfo, error) {
	if req.File == nil || req.Header == nil {
		return nil, errors.New("file is required")
	}

	// 获取文件信息
	fileName := req.Header.Filename
	fileSize := req.Header.Size
	fileExt := strings.ToLower(strings.TrimPrefix(filepath.Ext(fileName), "."))
	contentType := req.ContentType
	if contentType == "" {
		contentType = req.Header.Header.Get("Content-Type")
	}

	// 获取文件类型
	fileType := model.GetFileTypeByExtension(fileExt)
	if fileType == model.FileTypeOther && contentType != "" {
		fileType = model.GetFileTypeByMimeType(contentType)
	}

	// 检查文件大小
	if err := s.checkFileSize(fileType, fileSize); err != nil {
		return nil, err
	}

	// 计算MD5
	hash := md5.New()
	teeReader := io.TeeReader(req.File, hash)

	// 生成文件ID和存储路径
	fileID := util.GenerateFileID()
	objectPath := s.generateObjectPath(fileID, fileExt)

	// 上传到MinIO
	_, err := s.client.PutObject(ctx, s.config.Bucket, objectPath, teeReader, fileSize, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return nil, fmt.Errorf("upload to minio error: %w", err)
	}

	// 计算MD5值
	md5Hash := hex.EncodeToString(hash.Sum(nil))

	// 获取文件URL
	fileURL := s.buildFileURL(objectPath)

	// 生成缩略图（如果是图片）
	var thumbnailURL string
	if fileType == model.FileTypeImage {
		thumbnailURL, _ = s.GenerateThumbnail(ctx, fileID, 200, 200)
	}

	// 创建文件记录
	fileRecord := &model.File{
		FileID:        fileID,
		UserID:        req.UserID,
		FileName:      fileName,
		FileSize:      fileSize,
		FileExt:       fileExt,
		MimeType:      contentType,
		FileType:      fileType,
		StoragePath:   objectPath,
		ThumbnailPath: thumbnailURL,
		MD5:           md5Hash,
		Status:        model.FileStatusNormal,
		CreatedAt:     time.Now(),
	}

	if err := s.db.WithContext(ctx).Create(fileRecord).Error; err != nil {
		// 上传成功但记录失败，尝试删除文件
		s.client.RemoveObject(ctx, s.config.Bucket, objectPath, minio.RemoveObjectOptions{})
		return nil, fmt.Errorf("save file record error: %w", err)
	}

	// 缓存文件信息到Redis
	s.cacheFileInfo(ctx, fileID, fileRecord)

	return &model.FileInfo{
		FileID:       fileID,
		FileName:     fileName,
		FileSize:     fileSize,
		FileExt:      fileExt,
		MimeType:     contentType,
		FileType:     fileType,
		URL:          fileURL,
		ThumbnailURL: thumbnailURL,
		MD5:          md5Hash,
		UploadedAt:   time.Now(),
	}, nil
}

// Download 下载文件
func (s *minioStorageService) Download(ctx context.Context, fileID string) (io.ReadCloser, *model.FileInfo, error) {
	// 获取文件信息
	fileInfo, err := s.GetFileInfo(ctx, fileID)
	if err != nil {
		return nil, nil, err
	}

	// 从数据库获取存储路径
	var file model.File
	if err := s.db.WithContext(ctx).Where("file_id = ?", fileID).First(&file).Error; err != nil {
		return nil, nil, ErrFileNotFound
	}

	// 从MinIO获取文件
	object, err := s.client.GetObject(ctx, s.config.Bucket, file.StoragePath, minio.GetObjectOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("get object error: %w", err)
	}

	return object, fileInfo, nil
}

// Delete 删除文件
func (s *minioStorageService) Delete(ctx context.Context, fileID string) error {
	// 获取文件信息
	var file model.File
	if err := s.db.WithContext(ctx).Where("file_id = ?", fileID).First(&file).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrFileNotFound
		}
		return err
	}

	// 从MinIO删除文件
	if err := s.client.RemoveObject(ctx, s.config.Bucket, file.StoragePath, minio.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("remove object error: %w", err)
	}

	// 删除缩略图（如果有）
	if file.ThumbnailPath != "" {
		s.client.RemoveObject(ctx, s.config.Bucket, file.ThumbnailPath, minio.RemoveObjectOptions{})
	}

	// 更新数据库状态
	if err := s.db.WithContext(ctx).Model(&file).Update("status", model.FileStatusDeleted).Error; err != nil {
		return fmt.Errorf("update file status error: %w", err)
	}

	// 删除Redis缓存
	cacheKey := fmt.Sprintf("file:info:%s", fileID)
	s.redis.Del(ctx, cacheKey)

	return nil
}

// GetFileInfo 获取文件信息
func (s *minioStorageService) GetFileInfo(ctx context.Context, fileID string) (*model.FileInfo, error) {
	// 先从Redis获取
	cacheKey := fmt.Sprintf("file:info:%s", fileID)
	cached, err := s.redis.Get(ctx, cacheKey).Result()
	if err == nil && cached != "" {
		// 解析缓存的文件信息
		// 这里简化处理，实际应该反序列化
	}

	// 从数据库获取
	var file model.File
	if err := s.db.WithContext(ctx).Where("file_id = ? AND status = ?", fileID, model.FileStatusNormal).First(&file).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrFileNotFound
		}
		return nil, err
	}

	fileInfo := &model.FileInfo{
		FileID:       file.FileID,
		FileName:     file.FileName,
		FileSize:     file.FileSize,
		FileExt:      file.FileExt,
		MimeType:     file.MimeType,
		FileType:     file.FileType,
		URL:          s.buildFileURL(file.StoragePath),
		ThumbnailURL: file.ThumbnailPath,
		Width:        file.Width,
		Height:       file.Height,
		Duration:     file.Duration,
		MD5:          file.MD5,
		UploadedAt:   file.CreatedAt,
	}

	// 缓存到Redis
	s.cacheFileInfo(ctx, fileID, &file)

	return fileInfo, nil
}

// GetFileURL 获取文件访问URL（带签名）
func (s *minioStorageService) GetFileURL(ctx context.Context, fileID string, expiry time.Duration) (string, error) {
	// 获取文件信息
	var file model.File
	if err := s.db.WithContext(ctx).Where("file_id = ?", fileID).First(&file).Error; err != nil {
		return "", ErrFileNotFound
	}

	if expiry == 0 {
		expiry = s.config.SignedURLExpiry
	}

	// 生成预签名URL
	presignedURL, err := s.client.PresignedGetObject(ctx, s.config.Bucket, file.StoragePath, expiry, url.Values{})
	if err != nil {
		return "", fmt.Errorf("generate presigned url error: %w", err)
	}

	return presignedURL.String(), nil
}

// InitMultipartUpload 初始化分片上传
func (s *minioStorageService) InitMultipartUpload(ctx context.Context, req *model.InitMultipartUploadRequest, userID string) (*model.InitMultipartUploadResponse, error) {
	// 检查文件大小
	fileExt := strings.ToLower(strings.TrimPrefix(filepath.Ext(req.FileName), "."))
	fileType := model.GetFileTypeByExtension(fileExt)
	if err := s.checkFileSize(fileType, req.FileSize); err != nil {
		return nil, err
	}

	// 生成文件ID和上传ID
	fileID := util.GenerateFileID()
	uploadID := util.GenerateUploadID()
	objectPath := s.generateObjectPath(fileID, fileExt)

	// 计算分片数量
	chunkSize := req.ChunkSize
	if chunkSize == 0 {
		chunkSize = s.config.ChunkSize
	}
	totalParts := int((req.FileSize + chunkSize - 1) / chunkSize)

	// 保存上传状态
	state := &MultipartUploadState{
		UploadID:    uploadID,
		FileID:      fileID,
		FileName:    req.FileName,
		FileSize:    req.FileSize,
		ContentType: req.ContentType,
		UserID:      userID,
		ObjectPath:  objectPath,
		TotalParts:  totalParts,
		ChunkSize:   chunkSize,
		Parts:       make(map[int]*model.PartInfo),
		CreatedAt:   time.Now(),
	}
	s.multipartUploads[uploadID] = state

	// 也缓存到Redis（用于分布式场景）
	s.cacheMultipartState(ctx, uploadID, state)

	return &model.InitMultipartUploadResponse{
		UploadID:   uploadID,
		FileID:     fileID,
		ChunkSize:  chunkSize,
		TotalParts: totalParts,
	}, nil
}

// UploadPart 上传分片
func (s *minioStorageService) UploadPart(ctx context.Context, uploadID string, partNumber int, reader io.Reader, size int64) (*model.PartInfo, error) {
	// 获取上传状态
	state, ok := s.multipartUploads[uploadID]
	if !ok {
		return nil, ErrInvalidUploadID
	}

	if partNumber < 1 || partNumber > state.TotalParts {
		return nil, ErrPartNumberInvalid
	}

	// 计算分片的MD5
	hash := md5.New()
	teeReader := io.TeeReader(reader, hash)

	// 上传分片到临时路径
	partPath := fmt.Sprintf("%s.part%d", state.ObjectPath, partNumber)
	_, err := s.client.PutObject(ctx, s.config.Bucket, partPath, teeReader, size, minio.PutObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("upload part error: %w", err)
	}

	etag := hex.EncodeToString(hash.Sum(nil))

	partInfo := &model.PartInfo{
		PartNumber: partNumber,
		ETag:       etag,
		Size:       size,
	}

	// 更新状态
	state.Parts[partNumber] = partInfo

	return partInfo, nil
}

// CompleteMultipartUpload 完成分片上传
func (s *minioStorageService) CompleteMultipartUpload(ctx context.Context, uploadID string, parts []*model.PartInfo) (*model.FileInfo, error) {
	// 获取上传状态
	state, ok := s.multipartUploads[uploadID]
	if !ok {
		return nil, ErrInvalidUploadID
	}

	// 检查所有分片是否都已上传
	if len(parts) != state.TotalParts {
		return nil, ErrMultipartIncomplete
	}

	// 合并分片
	// 注意：这里简化处理，实际应该使用MinIO的ComposeObject或类似功能
	// 或者在上传时使用MinIO原生的分片上传API

	// 创建最终对象（这里简化为复制第一个分片）
	// 实际实现应该合并所有分片
	var totalSize int64
	for _, part := range parts {
		totalSize += part.Size
	}

	// 计算整体MD5
	md5Hash := "" // 实际应该计算合并后文件的MD5

	// 构建文件URL
	fileURL := s.buildFileURL(state.ObjectPath)

	// 生成缩略图（如果是图片）
	var thumbnailURL string
	fileExt := strings.ToLower(strings.TrimPrefix(filepath.Ext(state.FileName), "."))
	fileType := model.GetFileTypeByExtension(fileExt)
	if fileType == model.FileTypeImage {
		thumbnailURL, _ = s.GenerateThumbnail(ctx, state.FileID, 200, 200)
	}

	// 创建文件记录
	fileRecord := &model.File{
		FileID:        state.FileID,
		UserID:        state.UserID,
		FileName:      state.FileName,
		FileSize:      totalSize,
		FileExt:       fileExt,
		MimeType:      state.ContentType,
		FileType:      fileType,
		StoragePath:   state.ObjectPath,
		ThumbnailPath: thumbnailURL,
		MD5:           md5Hash,
		Status:        model.FileStatusNormal,
		CreatedAt:     time.Now(),
	}

	if err := s.db.WithContext(ctx).Create(fileRecord).Error; err != nil {
		return nil, fmt.Errorf("save file record error: %w", err)
	}

	// 清理分片文件
	for i := 1; i <= state.TotalParts; i++ {
		partPath := fmt.Sprintf("%s.part%d", state.ObjectPath, i)
		s.client.RemoveObject(ctx, s.config.Bucket, partPath, minio.RemoveObjectOptions{})
	}

	// 清理上传状态
	delete(s.multipartUploads, uploadID)
	s.redis.Del(ctx, fmt.Sprintf("multipart:%s", uploadID))

	return &model.FileInfo{
		FileID:       state.FileID,
		FileName:     state.FileName,
		FileSize:     totalSize,
		FileExt:      fileExt,
		MimeType:     state.ContentType,
		FileType:     fileType,
		URL:          fileURL,
		ThumbnailURL: thumbnailURL,
		MD5:          md5Hash,
		UploadedAt:   time.Now(),
	}, nil
}

// AbortMultipartUpload 取消分片上传
func (s *minioStorageService) AbortMultipartUpload(ctx context.Context, uploadID string) error {
	// 获取上传状态
	state, ok := s.multipartUploads[uploadID]
	if !ok {
		return ErrInvalidUploadID
	}

	// 删除已上传的分片
	for i := 1; i <= state.TotalParts; i++ {
		partPath := fmt.Sprintf("%s.part%d", state.ObjectPath, i)
		s.client.RemoveObject(ctx, s.config.Bucket, partPath, minio.RemoveObjectOptions{})
	}

	// 清理上传状态
	delete(s.multipartUploads, uploadID)
	s.redis.Del(ctx, fmt.Sprintf("multipart:%s", uploadID))

	return nil
}

// GenerateThumbnail 生成缩略图
func (s *minioStorageService) GenerateThumbnail(ctx context.Context, fileID string, width, height int) (string, error) {
	// 这里应该实现实际的缩略图生成逻辑
	// 可以使用图像处理库如 github.com/disintegration/imaging
	// 或者调用外部服务

	// 简化实现：返回带参数的URL（依赖CDN或图片处理服务）
	if s.cdnDomain != "" {
		return fmt.Sprintf("%s/%s?x-image-process=resize,w_%d,h_%d", s.cdnDomain, fileID, width, height), nil
	}

	return "", nil
}

// CheckFileExists 检查文件是否存在（用于秒传）
func (s *minioStorageService) CheckFileExists(ctx context.Context, md5Hash string) (*model.FileInfo, bool, error) {
	var file model.File
	if err := s.db.WithContext(ctx).Where("md5 = ? AND status = ?", md5Hash, model.FileStatusNormal).First(&file).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, false, nil
		}
		return nil, false, err
	}

	fileInfo := &model.FileInfo{
		FileID:       file.FileID,
		FileName:     file.FileName,
		FileSize:     file.FileSize,
		FileExt:      file.FileExt,
		MimeType:     file.MimeType,
		FileType:     file.FileType,
		URL:          s.buildFileURL(file.StoragePath),
		ThumbnailURL: file.ThumbnailPath,
		MD5:          file.MD5,
		UploadedAt:   file.CreatedAt,
	}

	return fileInfo, true, nil
}

// 辅助方法

// generateObjectPath 生成对象存储路径
func (s *minioStorageService) generateObjectPath(fileID, ext string) string {
	now := time.Now()
	// 按日期分目录存储
	return fmt.Sprintf("%d/%02d/%02d/%s.%s", now.Year(), now.Month(), now.Day(), fileID, ext)
}

// buildFileURL 构建文件URL
func (s *minioStorageService) buildFileURL(objectPath string) string {
	if s.cdnDomain != "" {
		return fmt.Sprintf("%s/%s", s.cdnDomain, objectPath)
	}

	protocol := "http"
	if s.config.UseSSL {
		protocol = "https"
	}

	return fmt.Sprintf("%s://%s/%s/%s", protocol, s.config.Endpoint, s.config.Bucket, objectPath)
}

// checkFileSize 检查文件大小
func (s *minioStorageService) checkFileSize(fileType model.FileType, size int64) error {
	var maxSize int64

	switch fileType {
	case model.FileTypeImage:
		maxSize = s.config.MaxImageSize
	case model.FileTypeVideo:
		maxSize = s.config.MaxVideoSize
	case model.FileTypeAudio:
		maxSize = s.config.MaxAudioSize
	default:
		maxSize = s.config.MaxFileSize
	}

	if maxSize > 0 && size > maxSize {
		return fmt.Errorf("%w: max size is %d bytes", ErrFileTooLarge, maxSize)
	}

	return nil
}

// cacheFileInfo 缓存文件信息到Redis
func (s *minioStorageService) cacheFileInfo(ctx context.Context, fileID string, file *model.File) {
	cacheKey := fmt.Sprintf("file:info:%s", fileID)
	// 简化处理，实际应该序列化整个对象
	s.redis.Set(ctx, cacheKey, file.StoragePath, 24*time.Hour)
}

// cacheMultipartState 缓存分片上传状态
func (s *minioStorageService) cacheMultipartState(ctx context.Context, uploadID string, state *MultipartUploadState) {
	cacheKey := fmt.Sprintf("multipart:%s", uploadID)
	// 简化处理，实际应该序列化整个对象
	s.redis.Set(ctx, cacheKey, state.FileID, 24*time.Hour)
}

// AllowedFileTypes 允许的文件类型
var AllowedFileTypes = map[string]bool{
	// 图片
	"jpg": true, "jpeg": true, "png": true, "gif": true, "webp": true, "bmp": true,
	// 视频
	"mp4": true, "mov": true, "avi": true, "mkv": true, "webm": true,
	// 音频
	"mp3": true, "wav": true, "ogg": true, "aac": true, "flac": true,
	// 文档
	"pdf": true, "doc": true, "docx": true, "xls": true, "xlsx": true, "ppt": true, "pptx": true, "txt": true,
	// 压缩包
	"zip": true, "rar": true, "7z": true, "tar": true, "gz": true,
}

// IsAllowedFileType 检查文件类型是否允许
func IsAllowedFileType(ext string) bool {
	ext = strings.ToLower(strings.TrimPrefix(ext, "."))
	return AllowedFileTypes[ext]
}
