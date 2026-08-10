package trace

import (
	"context"
)

// SpanContext 不可变 Span 上下文（当前 Span 的身份信息，通过 context 传播）
type SpanContext struct {
	TraceID   string
	SpanID    string
	RequestID string
	RunID     string
	Sampled   bool
}

// RunMeta 不可变 Run 元信息（异步 Worker 注入，Agent 读取）
type RunMeta struct {
	RunID            string
	AgentSlug        string
	ConversationID   uint
	RequestID        string
	Query            string
	Mode             string
	Attempt          int
	ResumeFromRunID  string
	ResumeFromSpanID string
}

type spanContextKey struct{}
type runMetaKey struct{}

// WithSpanContext 注入不可变 SpanContext
func WithSpanContext(ctx context.Context, sc SpanContext) context.Context {
	return context.WithValue(ctx, spanContextKey{}, sc)
}

// SpanContextFromContext 读取 SpanContext，无则返回零值和 false
func SpanContextFromContext(ctx context.Context) (SpanContext, bool) {
	sc, ok := ctx.Value(spanContextKey{}).(SpanContext)
	return sc, ok
}

// WithRunMeta 注入不可变 RunMeta
func WithRunMeta(ctx context.Context, meta RunMeta) context.Context {
	return context.WithValue(ctx, runMetaKey{}, meta)
}

// RunMetaFromContext 读取 RunMeta，无则返回零值和 false
func RunMetaFromContext(ctx context.Context) (RunMeta, bool) {
	meta, ok := ctx.Value(runMetaKey{}).(RunMeta)
	return meta, ok
}

// TraceIDFromContext 读取 TraceID（入队时透传使用），无则返回空串。
// 从新 SpanContext 读取 TraceID。
func TraceIDFromContext(ctx context.Context) string {
	if sc, ok := SpanContextFromContext(ctx); ok {
		return sc.TraceID
	}
	return ""
}
