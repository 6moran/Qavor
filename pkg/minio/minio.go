package minio

import (
	"context"
	"fmt"

	"Qavor/pkg/config"
	"Qavor/pkg/logger"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"go.uber.org/zap"
)

// MinIOClient MinIO 客户端封装
type MinIOClient struct {
	client    *minio.Client
	cfg       *config.MinIOConfig
	publicURL string // 拼接好的公共访问 URL 前缀
}

var globalClient *MinIOClient

// Init 初始化 MinIO 客户端
func Init(cfg *config.MinIOConfig) error {
	// 设置默认值
	if cfg.MaxFileSize == 0 {
		cfg.MaxFileSize = 50 << 20 // 50MB
	}
	if cfg.PublicEndpoint == "" {
		cfg.PublicEndpoint = cfg.Endpoint
	}

	// 构建 MinIO 客户端
	minioClient, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
		Region: cfg.Region,
	})
	if err != nil {
		logger.Error("MinIO 客户端创建失败", zap.Error(err))
		return fmt.Errorf("MinIO 客户端创建失败: %w", err)
	}

	// 确保存储桶存在
	ctx := context.Background()
	exists, err := minioClient.BucketExists(ctx, cfg.Bucket)
	if err != nil {
		logger.Error("MinIO 检查存储桶失败", zap.Error(err))
		return fmt.Errorf("MinIO 检查存储桶失败: %w", err)
	}

	if !exists {
		err = minioClient.MakeBucket(ctx, cfg.Bucket, minio.MakeBucketOptions{Region: cfg.Region})
		if err != nil {
			logger.Error("MinIO 创建存储桶失败", zap.Error(err))
			return fmt.Errorf("MinIO 创建存储桶失败: %w", err)
		}
		logger.Info("MinIO 存储桶已创建", zap.String("bucket", cfg.Bucket))

		// 设置公开读取策略（可选，按需启用）
		// 如果需要公开读取，需要在 MinIO 控制台或通过 API 设置 bucket policy
	} else {
		logger.Info("MinIO 存储桶已存在", zap.String("bucket", cfg.Bucket))
	}

	// 构建公共访问 URL 前缀: http://endpoint/bucket
	scheme := "http"
	if cfg.UseSSL {
		scheme = "https"
	}
	publicURL := fmt.Sprintf("%s://%s/%s", scheme, cfg.PublicEndpoint, cfg.Bucket)

	globalClient = &MinIOClient{
		client:    minioClient,
		cfg:       cfg,
		publicURL: publicURL,
	}

	logger.Info("MinIO 连接成功",
		zap.String("endpoint", cfg.Endpoint),
		zap.String("bucket", cfg.Bucket),
		zap.String("public_endpoint", cfg.PublicEndpoint),
	)

	return nil
}

// Get 获取 MinIO 客户端单例
func Get() *MinIOClient {
	if globalClient == nil {
		panic("MinIO 未初始化，请先调用 Init()")
	}
	return globalClient
}

// Close 关闭 MinIO 客户端（minio-go 无需显式关闭，保留接口一致性）
func Close() error {
	globalClient = nil
	return nil
}

// Client 返回底层 minio.Client，供高级操作使用
func (c *MinIOClient) Client() *minio.Client {
	return c.client
}

// Config 返回 MinIO 配置
func (c *MinIOClient) Config() *config.MinIOConfig {
	return c.cfg
}
