package minio

import (
	"context"
	"fmt"
	"time"

	"github.com/minio/minio-go/v7"
)

// GetURL 根据相对路径生成完整访问 URL
// relativePath: 相对路径，如 "avatars/a1b2c3d4.jpg"
// 返回: "http://minio.example.com/qavor/avatars/a1b2c3d4.jpg"
func (c *MinIOClient) GetURL(relativePath string) string {
	return fmt.Sprintf("%s/%s", c.publicURL, relativePath)
}

// GetPresignedURL 生成预签名 URL（带有效期）
// relativePath: 相对路径
// expiry: 有效期，如 15 * time.Minute
func (c *MinIOClient) GetPresignedURL(relativePath string, expiry time.Duration) (string, error) {
	ctx := context.Background()

	// 检查文件是否存在
	exists, err := c.Exists(relativePath)
	if err != nil {
		return "", fmt.Errorf("检查文件是否存在失败: %w", err)
	}
	if !exists {
		return "", fmt.Errorf("文件不存在: %s", relativePath)
	}

	// 生成预签名 URL
	presignedURL, err := c.client.PresignedGetObject(ctx, c.cfg.Bucket, relativePath, expiry, nil)
	if err != nil {
		return "", fmt.Errorf("生成预签名 URL 失败: %w", err)
	}

	return presignedURL.String(), nil
}

// Exists 检查文件是否存在
func (c *MinIOClient) Exists(relativePath string) (bool, error) {
	ctx := context.Background()
	_, err := c.client.StatObject(ctx, c.cfg.Bucket, relativePath, minio.StatObjectOptions{})
	if err != nil {
		// minio 判断是否为"不存在"错误
		if isNotFoundError(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// isNotFoundError 判断是否为 MinIO 对象不存在错误
func isNotFoundError(err error) bool {
	// minio-go 中，对象不存在时返回的错误包含 "NoSuchKey"
	return err != nil && (err.Error() == "The specified key does not exist." ||
		err.Error() == "NoSuchKey" ||
		fmt.Sprintf("%v", err) == "NoSuchKey")
}
