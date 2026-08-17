package minio

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"path/filepath"
	"strings"

	"Qavor/pkg/logger"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"go.uber.org/zap"
)

// UploadResult 上传结果
type UploadResult struct {
	RelativePath string // 相对路径，存数据库用: "avatars/a1b2c3d4.jpg"
	FullURL      string // 完整 URL，返回前端用: "http://minio.example.com/qavor/avatars/a1b2c3d4.jpg"
	FileName     string // 原始文件名
	FileSize     int64
	ContentType  string
}

// Upload 从 multipart 文件上传
// folder: 存储文件夹，如 "avatars", "attachments", "knowledge"
// fileHeader: 从 c.FormFile("file") 获取的 FileHeader
func (c *MinIOClient) Upload(folder string, fileHeader *multipart.FileHeader) (*UploadResult, error) {
	// 打开文件
	file, err := fileHeader.Open()
	if err != nil {
		return nil, fmt.Errorf("打开上传文件失败: %w", err)
	}
	defer file.Close()

	// 读取前 512 字节用于检测 MIME 类型
	buf := make([]byte, 512)
	n, err := file.Read(buf)
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("读取文件内容失败: %w", err)
	}
	buf = buf[:n]

	// 检测真实 Content-Type（不信任客户端传值）
	contentType := DetectContentType(buf)

	// 校验文件类型
	if err := ValidateFileType(contentType, c.cfg.AllowedTypes); err != nil {
		return nil, err
	}

	// 校验文件大小
	if err := ValidateFileSize(fileHeader.Size, c.cfg.MaxFileSize); err != nil {
		return nil, err
	}

	// 回到文件开头
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("文件 seek 失败: %w", err)
	}

	// 生成 UUID 文件名
	ext := filepath.Ext(fileHeader.Filename)
	if ext == "" {
		// 从 Content-Type 推断扩展名
		ext = extFromContentType(contentType)
	}
	uuidName := uuid.New().String() + ext

	// 组装相对路径
	relativePath := fmt.Sprintf("%s/%s", strings.Trim(folder, "/"), uuidName)

	// 上传到 MinIO
	ctx := context.Background()
	_, err = c.client.PutObject(ctx, c.cfg.Bucket, relativePath, file, fileHeader.Size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		logger.Error("MinIO 文件上传失败",
			zap.String("path", relativePath),
			zap.Error(err),
		)
		return nil, fmt.Errorf("文件上传失败: %w", err)
	}

	logger.Info("MinIO 文件上传成功",
		zap.String("path", relativePath),
		zap.Int64("size", fileHeader.Size),
		zap.String("type", contentType),
	)

	return &UploadResult{
		RelativePath: relativePath,
		FullURL:      c.GetURL(relativePath),
		FileName:     fileHeader.Filename,
		FileSize:     fileHeader.Size,
		ContentType:  contentType,
	}, nil
}

// UploadFromReader 从 io.Reader 上传（适用于已读取在内存或流式数据）
// folder: 存储文件夹
// filename: 原始文件名（用于推断扩展名）
// contentType: MIME 类型
// reader: 数据源
// size: 数据大小（-1 表示未知）
func (c *MinIOClient) UploadFromReader(folder, filename, contentType string, reader io.Reader, size int64) (*UploadResult, error) {
	// 校验文件类型
	if err := ValidateFileType(contentType, c.cfg.AllowedTypes); err != nil {
		return nil, err
	}

	// 校验文件大小
	if err := ValidateFileSize(size, c.cfg.MaxFileSize); err != nil {
		return nil, err
	}

	// 生成 UUID 文件名
	ext := filepath.Ext(filename)
	if ext == "" {
		ext = extFromContentType(contentType)
	}
	uuidName := uuid.New().String() + ext

	// 组装相对路径
	relativePath := fmt.Sprintf("%s/%s", strings.Trim(folder, "/"), uuidName)

	// 上传到 MinIO
	ctx := context.Background()
	opts := minio.PutObjectOptions{ContentType: contentType}
	_, err := c.client.PutObject(ctx, c.cfg.Bucket, relativePath, reader, size, opts)
	if err != nil {
		logger.Error("MinIO 文件上传失败",
			zap.String("path", relativePath),
			zap.Error(err),
		)
		return nil, fmt.Errorf("文件上传失败: %w", err)
	}

	logger.Info("MinIO 文件上传成功",
		zap.String("path", relativePath),
		zap.Int64("size", size),
		zap.String("type", contentType),
	)

	return &UploadResult{
		RelativePath: relativePath,
		FullURL:      c.GetURL(relativePath),
		FileName:     filename,
		FileSize:     size,
		ContentType:  contentType,
	}, nil
}

// extFromContentType 根据 Content-Type 返回文件扩展名
func extFromContentType(ct string) string {
	switch {
	case strings.HasPrefix(ct, "image/"):
		return ".png" // 默认图片格式
	case ct == "application/pdf":
		return ".pdf"
	case strings.HasPrefix(ct, "video/"):
		return ".mp4"
	case strings.HasPrefix(ct, "audio/"):
		return ".mp3"
	case ct == "application/zip":
		return ".zip"
	case ct == "application/gzip":
		return ".gz"
	case strings.Contains(ct, "json"):
		return ".json"
	case strings.Contains(ct, "xml"):
		return ".xml"
	case strings.HasPrefix(ct, "text/"):
		return ".txt"
	default:
		return ".bin"
	}
}
