package minio

import (
	"context"
	"fmt"

	"Qavor/pkg/logger"

	minio "github.com/minio/minio-go/v7"
	"go.uber.org/zap"
)

// Delete 删除文件
// relativePath: 相对路径，如 "avatars/a1b2c3d4.jpg"
func (c *MinIOClient) Delete(relativePath string) error {
	ctx := context.Background()
	err := c.client.RemoveObject(ctx, c.cfg.Bucket, relativePath, minio.RemoveObjectOptions{})
	if err != nil {
		logger.Error("MinIO 文件删除失败",
			zap.String("path", relativePath),
			zap.Error(err),
		)
		return fmt.Errorf("文件删除失败: %w", err)
	}

	logger.Info("MinIO 文件删除成功", zap.String("path", relativePath))
	return nil
}

// DeleteBatch 批量删除文件
// relativePaths: 相对路径列表
func (c *MinIOClient) DeleteBatch(relativePaths []string) error {
	ctx := context.Background()
	objectsCh := make(chan minio.ObjectInfo, len(relativePaths))

	// 发送待删除对象
	go func() {
		defer close(objectsCh)
		for _, path := range relativePaths {
			objectsCh <- minio.ObjectInfo{Key: path}
		}
	}()

	// 执行批量删除
	errorCh := c.client.RemoveObjects(ctx, c.cfg.Bucket, objectsCh, minio.RemoveObjectsOptions{})
	var errs []error
	for e := range errorCh {
		logger.Error("MinIO 批量删除对象失败",
			zap.String("key", e.ObjectName),
			zap.Error(e.Err),
		)
		errs = append(errs, fmt.Errorf("删除 %s 失败: %w", e.ObjectName, e.Err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("批量删除完成，%d 个文件删除失败", len(errs))
	}

	logger.Info("MinIO 批量删除完成", zap.Int("count", len(relativePaths)))
	return nil
}
