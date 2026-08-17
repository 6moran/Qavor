package trace

import (
	"context"
	"time"

	"Qavor/pkg/logger"

	"go.uber.org/zap"
)

// Janitor 兜底清理器，解决两个问题：
//  1. "永远 running"的悬挂 span：正常流程 span 都会被 End，但进程崩溃或极端 bug 会导致悬挂
//     Janitor 把 running 超过 timeout（默认 30 分钟）的 span 标记为 timeout
//  2. 数据无限膨胀：按 retention（默认 7 天）物理删除过期的 trace_records，表不会无限增长
//
// 定时执行（默认 5 分钟一次），阻塞直到 ctx 取消（优雅关闭时停掉）
type Janitor struct {
	repo     TraceRepository // 数据访问接口
	interval time.Duration   // 清理间隔（默认 5 分钟）
	timeout  time.Duration   // running 超时时长（默认 30 分钟）
}

// NewJanitor 创建清理器
func NewJanitor(repo TraceRepository, interval, timeout time.Duration) *Janitor {
	return &Janitor{repo: repo, interval: interval, timeout: timeout}
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
	if n, err := j.repo.MarkTimeoutSpans(ctx, time.Now().Add(-j.timeout)); err != nil {
		logger.Warn("trace janitor 超时标记失败", zap.Error(err))
	} else if n > 0 {
		logger.Info("trace janitor 超时标记", zap.Int64("count", n))
	}
	// DeleteExpired 按 trace_records.expires_at < now 删除，retention 仅用于估算 expires_at
	if n, err := j.repo.DeleteExpired(ctx, time.Now()); err != nil {
		logger.Warn("trace janitor 过期清理失败", zap.Error(err))
	} else if n > 0 {
		logger.Info("trace janitor 过期清理", zap.Int64("count", n))
	}
}
