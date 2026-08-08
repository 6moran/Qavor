package trace

import (
	"context"
	"time"

	"Qavor/pkg/logger"

	"go.uber.org/zap"
)

// Janitor 兜底清理：running 超时标记 timeout + 过期数据物理删除
type Janitor struct {
	repo      TraceRepository
	interval  time.Duration
	timeout   time.Duration
	retention time.Duration
}

// NewJanitor 创建清理器
func NewJanitor(repo TraceRepository, interval, timeout, retention time.Duration) *Janitor {
	return &Janitor{repo: repo, interval: interval, timeout: timeout, retention: retention}
}

// Run 周期执行清理，阻塞直到 ctx 取消
func (j *Janitor) Run(ctx context.Context) {
	ticker := time.NewTicker(j.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			j.sweep(ctx)
		}
	}
}

func (j *Janitor) sweep(ctx context.Context) {
	if n, err := j.repo.MarkTimeoutTraces(ctx, time.Now().Add(-j.timeout)); err != nil {
		logger.Warn("trace janitor 超时标记失败", zap.Error(err))
	} else if n > 0 {
		logger.Info("trace janitor 超时标记", zap.Int64("count", n))
	}
	if n, err := j.repo.DeleteExpired(ctx, time.Now().Add(-j.retention)); err != nil {
		logger.Warn("trace janitor 过期清理失败", zap.Error(err))
	} else if n > 0 {
		logger.Info("trace janitor 过期清理", zap.Int64("count", n))
	}
}
