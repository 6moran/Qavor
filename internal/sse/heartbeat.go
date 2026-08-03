package sse

import (
	"context"
	"time"

	"go.uber.org/zap"
)

// HeartbeatManager 心跳管理器
type HeartbeatManager struct {
	interval time.Duration
	logger   *zap.Logger
}

// NewHeartbeatManager 创建心跳管理器
func NewHeartbeatManager(interval time.Duration, logger *zap.Logger) *HeartbeatManager {
	return &HeartbeatManager{
		interval: interval,
		logger:   logger,
	}
}

// Start 启动心跳
// 返回一个停止信号 channel
func (hm *HeartbeatManager) Start(ctx context.Context, writer *SSEWriter, messageID string) chan struct{} {
	stop := make(chan struct{})

	go func() {
		ticker := time.NewTicker(hm.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				// 上下文取消，停止心跳
				return
			case <-stop:
				// 外部停止信号
				return
			case <-ticker.C:
				// 发送心跳事件（通过 SSEWriter，线程安全）
				writer.SendHeartbeat(messageID)
			}
		}
	}()

	return stop
}

// Stop 停止心跳
func (hm *HeartbeatManager) Stop(stop chan struct{}) {
	select {
	case stop <- struct{}{}:
	default:
	}
}
