package trace

import (
	"context"

	"Qavor/pkg/logger"

	"go.uber.org/zap"
)

var (
	globalRepo    TraceRepository
	globalEnabled bool
	globalMaxLen  int
)

// Init 初始化 trace 包（进程启动时调用一次；enabled=false 时采集/收尾均为 no-op）
func Init(repo TraceRepository, enabled bool, maxLen int) {
	globalRepo = repo
	globalEnabled = enabled
	globalMaxLen = maxLen
}

// Enabled 返回采集开关
func Enabled() bool { return globalEnabled }

// truncate 按字符数截断（rune 安全）
func truncate(s string, n int) string {
	if n <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

// FinishTrace 在 Agent 执行收尾点调用，补齐根记录聚合字段（无上下文/未启用/已收尾时为 no-op）
func FinishTrace(ctx context.Context, status, errMsg string) {
	if !globalEnabled || globalRepo == nil {
		return
	}
	tc := FromContext(ctx)
	if tc == nil || tc.IsFinished() {
		return
	}
	tc.markFinished()
	if err := globalRepo.FinishTrace(ctx, tc.TraceID, status, truncate(errMsg, globalMaxLen)); err != nil {
		logger.Warn("trace: 收尾失败", zap.String("trace_id", tc.TraceID), zap.String("status", status), zap.Error(err))
	}
}
