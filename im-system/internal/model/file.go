// Package model 定义IM系统的数据模型
package model

import (
	"time"
)

// FileType 文件类型
type FileType int

const (
	FileTypeImage    FileType = 1 // 图片
	FileTypeVideo    FileType = 2 // 视频
	FileTypeAudio    FileType = 3 // 音频
	FileTypeDocument FileType = 4 // 文档
	FileTypeArchive  FileType = 5 // 压缩包
	FileTypeOther    FileType = 6 // 其他
)

// String 返回文件类型的字符串表示
func (t FileType) String() string {
	switch t {
	case FileTypeImage:
		return "image"
	case FileTypeVideo:
		return "video"
	case FileTypeAudio:
		return "audio"
	case FileTypeDocument:
		return "document"
	case FileTypeArchive:
		return "archive"
	default:
		return "other"
	}
}

// FileStatus 文件状态
type FileStatus int

const (
	FileStatusNormal  FileStatus = 1 // 正常
	FileStatusDeleted FileStatus = 0 // 已删除
)

// File 文件记录
type File struct {
	ID            uint       `json:"id" gorm:"primaryKey;autoIncrement"`
	FileID        string     `json:"file_id" gorm:"type:varchar(64);uniqueIndex;not null"`
	UserID        string     `json:"user_id" gorm:"type:varchar(64);index;not null"`
	FileName      string     `json:"file_name" gorm:"type:varchar(256);not null"`
	FileSize      int64      `json:"file_size" gorm:"not null"`
	FileExt       string     `json:"file_ext" gorm:"type:varchar(32)"`
	MimeType      string     `json:"mime_type" gorm:"type:varchar(128)"`
	FileType      FileType   `json:"file_type" gorm:"type:tinyint"`
	StoragePath   string     `json:"storage_path" gorm:"type:varchar(512);not null"`
	ThumbnailPath string     `json:"thumbnail_path" gorm:"type:varchar(512)"`
	MD5           string     `json:"md5" gorm:"type:varchar(64)"`
	Width         int        `json:"width" gorm:"default:0"`    // 图片/视频宽度
	Height        int        `json:"height" gorm:"default:0"`   // 图片/视频高度
	Duration      int        `json:"duration" gorm:"default:0"` // 音视频时长(秒)
	Status        FileStatus `json:"status" gorm:"default:1"`   // 状态
	CreatedAt     time.Time  `json:"created_at" gorm:"autoCreateTime;index"`
}

// TableName 指定表名
func (File) TableName() string {
	return "files"
}

// FileMessage 文件消息
type FileMessage struct {
	FileID       string   `json:"file_id"`
	FileType     FileType `json:"file_type"`
	FileName     string   `json:"file_name"`
	FileSize     int64    `json:"file_size"`
	FileExt      string   `json:"file_ext,omitempty"`
	MimeType     string   `json:"mime_type,omitempty"`
	URL          string   `json:"url"`
	ThumbnailURL string   `json:"thumbnail_url,omitempty"`
	Width        int      `json:"width,omitempty"`
	Height       int      `json:"height,omitempty"`
	Duration     int      `json:"duration,omitempty"`
	MD5          string   `json:"md5,omitempty"`
	UploadStatus string   `json:"upload_status,omitempty"` // uploading, completed, failed
}

// ImageMessage 图片消息
type ImageMessage struct {
	FileID       string `json:"file_id"`
	URL          string `json:"url"`
	ThumbnailURL string `json:"thumbnail_url,omitempty"`
	Width        int    `json:"width,omitempty"`
	Height       int    `json:"height,omitempty"`
	FileSize     int64  `json:"file_size,omitempty"`
	Format       string `json:"format,omitempty"` // jpeg, png, gif, webp
}

// VideoMessage 视频消息
type VideoMessage struct {
	FileID       string `json:"file_id"`
	URL          string `json:"url"`
	ThumbnailURL string `json:"thumbnail_url,omitempty"`
	Duration     int    `json:"duration"` // 时长（秒）
	Width        int    `json:"width,omitempty"`
	Height       int    `json:"height,omitempty"`
	FileSize     int64  `json:"file_size,omitempty"`
	Format       string `json:"format,omitempty"` // mp4, mov, webm
}

// AudioMessage 音频消息
type AudioMessage struct {
	FileID   string `json:"file_id"`
	URL      string `json:"url"`
	Duration int    `json:"duration"` // 时长（秒）
	FileSize int64  `json:"file_size,omitempty"`
	Format   string `json:"format,omitempty"` // mp3, amr, wav, ogg
}

// DocumentMessage 文档消息
type DocumentMessage struct {
	FileID     string `json:"file_id"`
	FileName   string `json:"file_name"`
	FileSize   int64  `json:"file_size"`
	FileExt    string `json:"file_ext,omitempty"`
	MimeType   string `json:"mime_type,omitempty"`
	URL        string `json:"url"`
	PreviewURL string `json:"preview_url,omitempty"` // 预览URL
	Pages      int    `json:"pages,omitempty"`       // 页数（PDF等）
}

// UploadFileRequest 上传文件请求
type UploadFileRequest struct {
	FileName    string `json:"file_name" binding:"required"`
	FileSize    int64  `json:"file_size" binding:"required,min=1"`
	ContentType string `json:"content_type"`
	MD5         string `json:"md5,omitempty"` // 可选，用于秒传检测
}

// UploadFileResponse 上传文件响应
type UploadFileResponse struct {
	FileID       string `json:"file_id"`
	UploadURL    string `json:"upload_url,omitempty"`    // 直传URL（用于大文件）
	ThumbnailURL string `json:"thumbnail_url,omitempty"` // 缩略图URL
	URL          string `json:"url,omitempty"`           // 文件访问URL
	Exists       bool   `json:"exists,omitempty"`        // 秒传：文件已存在
}

// MultipartUploadInfo 分片上传信息
type MultipartUploadInfo struct {
	UploadID   string `json:"upload_id"`
	FileID     string `json:"file_id"`
	ChunkSize  int64  `json:"chunk_size"`
	TotalParts int    `json:"total_parts"`
}

// InitMultipartUploadRequest 初始化分片上传请求
type InitMultipartUploadRequest struct {
	FileName    string `json:"file_name" binding:"required"`
	FileSize    int64  `json:"file_size" binding:"required,min=1"`
	ContentType string `json:"content_type"`
	ChunkSize   int64  `json:"chunk_size,omitempty"` // 分片大小，默认5MB
}

// InitMultipartUploadResponse 初始化分片上传响应
type InitMultipartUploadResponse struct {
	UploadID   string `json:"upload_id"`
	FileID     string `json:"file_id"`
	ChunkSize  int64  `json:"chunk_size"`
	TotalParts int    `json:"total_parts"`
}

// PartInfo 分片信息
type PartInfo struct {
	PartNumber int    `json:"part_number"`
	ETag       string `json:"etag"`
	Size       int64  `json:"size"`
}

// UploadPartRequest 上传分片请求
type UploadPartRequest struct {
	UploadID   string `json:"upload_id" binding:"required"`
	PartNumber int    `json:"part_number" binding:"required,min=1"`
}

// UploadPartResponse 上传分片响应
type UploadPartResponse struct {
	PartNumber int    `json:"part_number"`
	ETag       string `json:"etag"`
	Size       int64  `json:"size"`
}

// CompleteMultipartUploadRequest 完成分片上传请求
type CompleteMultipartUploadRequest struct {
	UploadID string      `json:"upload_id" binding:"required"`
	Parts    []*PartInfo `json:"parts" binding:"required"`
}

// CompleteMultipartUploadResponse 完成分片上传响应
type CompleteMultipartUploadResponse struct {
	FileID       string `json:"file_id"`
	URL          string `json:"url"`
	ThumbnailURL string `json:"thumbnail_url,omitempty"`
	FileSize     int64  `json:"file_size"`
}

// FileInfo 文件信息
type FileInfo struct {
	FileID       string    `json:"file_id"`
	FileName     string    `json:"file_name"`
	FileSize     int64     `json:"file_size"`
	FileExt      string    `json:"file_ext,omitempty"`
	MimeType     string    `json:"mime_type,omitempty"`
	FileType     FileType  `json:"file_type"`
	URL          string    `json:"url"`
	ThumbnailURL string    `json:"thumbnail_url,omitempty"`
	Width        int       `json:"width,omitempty"`
	Height       int       `json:"height,omitempty"`
	Duration     int       `json:"duration,omitempty"`
	MD5          string    `json:"md5,omitempty"`
	UploaderID   string    `json:"uploader_id,omitempty"`
	UploadedAt   time.Time `json:"uploaded_at"`
}

// StorageConfig 存储配置
type StorageConfig struct {
	Provider  string `json:"provider"` // minio, aliyun, aws, qiniu
	Endpoint  string `json:"endpoint"`
	AccessKey string `json:"access_key"`
	SecretKey string `json:"secret_key"`
	Bucket    string `json:"bucket"`
	Region    string `json:"region"`
	UseSSL    bool   `json:"use_ssl"`
	CDNDomain string `json:"cdn_domain,omitempty"`
}

// FileAccessControl 文件访问控制
type FileAccessControl struct {
	// 签名URL过期时间（秒）
	SignedURLExpiry int `json:"signed_url_expiry"`

	// Referer白名单
	RefererWhitelist []string `json:"referer_whitelist,omitempty"`

	// 是否启用病毒扫描
	EnableVirusScan bool `json:"enable_virus_scan"`

	// 是否启用内容审核
	EnableContentScan bool `json:"enable_content_scan"`
}

// GetFileTypeByMimeType 根据MIME类型判断文件类型
func GetFileTypeByMimeType(mimeType string) FileType {
	switch {
	case isImageMimeType(mimeType):
		return FileTypeImage
	case isVideoMimeType(mimeType):
		return FileTypeVideo
	case isAudioMimeType(mimeType):
		return FileTypeAudio
	case isDocumentMimeType(mimeType):
		return FileTypeDocument
	case isArchiveMimeType(mimeType):
		return FileTypeArchive
	default:
		return FileTypeOther
	}
}

// GetFileTypeByExtension 根据扩展名判断文件类型
func GetFileTypeByExtension(ext string) FileType {
	imageExts := map[string]bool{"jpg": true, "jpeg": true, "png": true, "gif": true, "webp": true, "bmp": true, "svg": true}
	videoExts := map[string]bool{"mp4": true, "mov": true, "avi": true, "mkv": true, "webm": true, "flv": true}
	audioExts := map[string]bool{"mp3": true, "wav": true, "ogg": true, "aac": true, "flac": true, "amr": true}
	documentExts := map[string]bool{"pdf": true, "doc": true, "docx": true, "xls": true, "xlsx": true, "ppt": true, "pptx": true, "txt": true}
	archiveExts := map[string]bool{"zip": true, "rar": true, "7z": true, "tar": true, "gz": true}

	switch {
	case imageExts[ext]:
		return FileTypeImage
	case videoExts[ext]:
		return FileTypeVideo
	case audioExts[ext]:
		return FileTypeAudio
	case documentExts[ext]:
		return FileTypeDocument
	case archiveExts[ext]:
		return FileTypeArchive
	default:
		return FileTypeOther
	}
}

func isImageMimeType(mimeType string) bool {
	imageMimes := map[string]bool{
		"image/jpeg": true, "image/png": true, "image/gif": true,
		"image/webp": true, "image/bmp": true, "image/svg+xml": true,
	}
	return imageMimes[mimeType]
}

func isVideoMimeType(mimeType string) bool {
	videoMimes := map[string]bool{
		"video/mp4": true, "video/quicktime": true, "video/x-msvideo": true,
		"video/x-matroska": true, "video/webm": true, "video/x-flv": true,
	}
	return videoMimes[mimeType]
}

func isAudioMimeType(mimeType string) bool {
	audioMimes := map[string]bool{
		"audio/mpeg": true, "audio/wav": true, "audio/ogg": true,
		"audio/aac": true, "audio/flac": true, "audio/amr": true,
	}
	return audioMimes[mimeType]
}

func isDocumentMimeType(mimeType string) bool {
	docMimes := map[string]bool{
		"application/pdf":    true,
		"application/msword": true,
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document": true,
		"application/vnd.ms-excel": true,
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":         true,
		"application/vnd.ms-powerpoint":                                             true,
		"application/vnd.openxmlformats-officedocument.presentationml.presentation": true,
		"text/plain": true,
	}
	return docMimes[mimeType]
}

func isArchiveMimeType(mimeType string) bool {
	archiveMimes := map[string]bool{
		"application/zip":              true,
		"application/x-rar-compressed": true,
		"application/x-7z-compressed":  true,
		"application/x-tar":            true,
		"application/gzip":             true,
	}
	return archiveMimes[mimeType]
}
