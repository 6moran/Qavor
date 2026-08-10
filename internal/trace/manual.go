package trace

import (
	"context"
	"time"

	"Qavor/internal/model/entity"
	"Qavor/pkg/logger"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// SpanHandle 手动 span 句柄，End 时补全 span 记录
// nil 句柄表示 trace 未启用或上下文缺失，调用方安全 nil-check 后 defer End 即可
type SpanHandle struct {
	traceID   string
	spanID    string
	startedAt time.Time
}

// StartSpan 手动创建一个 span，返回携带 span 状态的 ctx 和句柄。
//
// 用法：
//
//	ctx, span := trace.StartSpan(ctx, entity.SpanKindContext, "FetchContext", "conv=123")
//	defer span.End(ctx, entity.SpanStatusSuccess, "12 msgs", "", 1024, 0)
//
// 当 trace 未启用、repo 未初始化或 ctx 无 TraceContext 时，返回原 ctx 和 nil。
// 调用方对返回的 *SpanHandle 做 nil-check 后再使用，nil 时 End 为 no-op。
//
// 父子关系：自动读取 ctx 中已有的 span（由 eino callback 或上一次 StartSpan 注入）作为 parent。
func StartSpan(ctx context.Context, kind, name, inputSummary string) (context.Context, *SpanHandle) {
	if !globalEnabled || globalRepo == nil {
		return ctx, nil
	}
	tc := FromContext(ctx)
	if tc == nil {
		return ctx, nil
	}
	tc.ensureRoot(ctx, globalRepo, globalMaxLen)

	now := time.Now()
	span := &entity.AgentTraceSpan{
		TraceID:      tc.TraceID,
		SpanID:       uuid.New().String(),
		Kind:         kind,
		Name:         name,
		Status:       entity.SpanStatusRunning,
		StartedAt:    now,
		InputSummary: truncate(inputSummary, globalMaxLen),
		CreatedAt:    now,
	}
	if ps := SpanFromContext(ctx); ps != nil {
		span.ParentSpanID = ps.ID
	}
	if err := globalRepo.CreateSpan(ctx, span); err != nil {
		logger.Warn("trace: 创建手动 span 失败",
			zap.String("trace_id", tc.TraceID),
			zap.String("kind", kind),
			zap.String("name", name),
			zap.Error(err))
		return ctx, nil
	}
	return WithSpan(ctx, &spanState{ID: span.SpanID, StartedAt: now}), &SpanHandle{
		traceID:   tc.TraceID,
		spanID:    span.SpanID,
		startedAt: now,
	}
}

// End 补全 span：写入 status / 输出摘要 / 错误 / token / 耗时。
// h 为 nil 时直接返回（no-op），便于 defer span.End(...) 无条件调用。
func (h *SpanHandle) End(ctx context.Context, status, outputSummary, errMsg string, tokensIn, tokensOut int) {
	if h == nil || !globalEnabled || globalRepo == nil {
		return
	}
	now := time.Now()
	upd := &entity.AgentTraceSpan{
		TraceID:       h.traceID,
		SpanID:        h.spanID,
		Status:        status,
		EndedAt:       &now,
		DurationMs:    now.Sub(h.startedAt).Milliseconds(),
		OutputSummary: truncate(outputSummary, globalMaxLen),
		ErrorMessage:  truncate(errMsg, globalMaxLen),
		TokensIn:      tokensIn,
		TokensOut:     tokensOut,
	}
	if err := globalRepo.UpdateSpan(ctx, upd); err != nil {
		logger.Warn("trace: 更新手动 span 失败",
			zap.String("trace_id", h.traceID),
			zap.String("span_id", h.spanID),
			zap.Error(err))
	}
}
