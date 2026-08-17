package trace

import (
	"context"

	"github.com/google/uuid"
)

// TraceCarrier 不可变追踪载体，用于跨进程边界（如 Redis 队列）传播追踪上下文
// 只携带父 Span 身份，不携带可变聚合状态
type TraceCarrier struct {
	TraceID      string `json:"trace_id"`
	ParentSpanID string `json:"parent_span_id,omitempty"`
	RequestID    string `json:"request_id,omitempty"`
	Sampled      bool   `json:"sampled"`
}

// CarrierFromContext 从当前 context 提取 TraceCarrier（基于当前 SpanContext）
// 没有 SpanContext 时返回零值和 false
func CarrierFromContext(ctx context.Context) (TraceCarrier, bool) {
	sc, ok := SpanContextFromContext(ctx)
	if !ok {
		return TraceCarrier{}, false
	}
	return TraceCarrier{
		TraceID:      sc.TraceID,
		ParentSpanID: sc.SpanID,
		RequestID:    sc.RequestID,
		Sampled:      sc.Sampled,
	}, true
}

// ContextFromCarrier 将 TraceCarrier 恢复为 context 中的 SpanContext
// 恢复后的 SpanID 即为 carrier 的 ParentSpanID，后续新建的 Span 将以此作为父 Span
func ContextFromCarrier(ctx context.Context, carrier TraceCarrier) context.Context {
	return WithSpanContext(ctx, SpanContext{
		TraceID:   carrier.TraceID,
		SpanID:    carrier.ParentSpanID,
		RequestID: carrier.RequestID,
		Sampled:   carrier.Sampled,
	})
}

// ValidTraceID 校验 TraceID 是否为合法 UUID，空值和超过 64 字符的值返回 false
func ValidTraceID(value string) bool {
	if value == "" || len(value) > 64 {
		return false
	}
	if _, err := uuid.Parse(value); err != nil {
		return false
	}
	return true
}
