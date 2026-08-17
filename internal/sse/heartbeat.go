package sse

import (
	"context"
	"time"

	"go.uber.org/zap"
)

// HeartbeatConfig 心跳配置
type HeartbeatConfig struct {
	Interval         time.Duration // 连接保活心跳间隔
	BusinessInterval time.Duration // 业务心跳间隔
	Timeout          time.Duration // 心跳超时时间
}

// DefaultHeartbeatConfig 默认心跳配置
func DefaultHeartbeatConfig() *HeartbeatConfig {
	return &HeartbeatConfig{
		Interval:         30 * time.Second, // 连接保活心跳：30秒
		BusinessInterval: 15 * time.Second, // 业务心跳：15秒
		Timeout:          60 * time.Second, // 超时：60秒
	}
}

// HeartbeatManager 心跳管理器
type HeartbeatManager struct {
	config *HeartbeatConfig
	logger *zap.Logger
}

// NewHeartbeatManager 创建心跳管理器
func NewHeartbeatManager(config *HeartbeatConfig, logger *zap.Logger) *HeartbeatManager {
	if config == nil {
		config = DefaultHeartbeatConfig()
	}
	return &HeartbeatManager{
		config: config,
		logger: logger,
	}
}

// Start 启动心跳（连接保活 + 业务心跳）
func (hm *HeartbeatManager) Start(ctx context.Context, conn *UserConnection) {
	// 启动连接保活心跳
	go hm.startKeepaliveHeartbeat(ctx, conn)
	// 启动业务心跳
	go hm.startBusinessHeartbeat(ctx, conn)
}

// startKeepaliveHeartbeat 连接保活心跳
// 用于保持 TCP 连接活跃，防止代理/防火墙超时
func (hm *HeartbeatManager) startKeepaliveHeartbeat(ctx context.Context, conn *UserConnection) {
	ticker := time.NewTicker(hm.config.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-conn.Done:
			return
		case <-ticker.C:
			conn.Writer.SendHeartbeat()
			if !conn.Writer.IsAlive() {
				hm.logger.Info("SSE 连接已断开（保活心跳检测）",
					zap.String("conn_id", conn.ConnID),
				)
				return
			}
		}
	}
}

// startBusinessHeartbeat 业务心跳
// 用于前端展示连接状态，更新最后活跃时间
func (hm *HeartbeatManager) startBusinessHeartbeat(ctx context.Context, conn *UserConnection) {
	ticker := time.NewTicker(hm.config.BusinessInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-conn.Done:
			return
		case <-ticker.C:
			conn.Writer.SendBusinessHeartbeat()
			conn.LastActive = time.Now()
			if !conn.Writer.IsAlive() {
				return
			}
		}
	}
}
