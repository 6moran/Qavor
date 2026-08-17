package trace

import (
	"context"
)

// SpanContext 不可变 Span 上下文（当前 Span 的身份信息，通过 Go context 传播）
// 类似 OTel 的 SpanContext，但多了 RequestID/RunID 业务字段
// 所有下游调用通过 context 共享同一个 SpanContext，子 Span 从中继承 TraceID/RequestID/RunID
type SpanContext struct {
	TraceID   string // 所属 Trace 的 ID（全局唯一，贯穿整条调用链）
	SpanID    string // 当前 Span 的 ID（用于设置子 Span 的 ParentSpanID）
	RequestID string // 请求 ID（X-Request-Id，贯穿同一次 HTTP 请求）
	RunID     string // 业务 Run ID（agent_runs 表主键，贯穿同一次 Agent 执行）
	Sampled   bool   // 采样标志：false 时整条链路不记录（父不采样子直接 no-op）
}

// RunMeta 不可变 Run 元信息（业务执行上下文）
// 由异步 Worker 或同步 Controller 注入 context，Agent 读取并写入 agent.run span 的 attributes
// 让 Agent 执行时能拿到"这个 Run 是谁、第几次尝试、从哪恢复的"等业务维度的身份
type RunMeta struct {
	RunID            string // 业务 Run ID（agent_runs 表主键）
	AgentSlug        string // Agent slug（如 assistant / rag-bot）
	ConversationID   uint   // 会话 ID
	RequestID        string // 请求 ID
	Query            string // 用户原始问题（用于 span attributes）
	Mode             string // 执行模式：sync（同步）/ stream（流式）/ run（异步）
	Attempt          int    // 第几次尝试（从 1 开始，首次执行为 1）
	ResumeFromRunID  string // 恢复场景：上次的 RunID（审批中断后继续）
	ResumeFromSpanID string // 恢复场景：上次的 agent.run span ID（建立因果链）
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

// TraceIDFromContext 读取 TraceID（入队时透传使用），无则返回空串
// 从新 SpanContext 读取 TraceID
func TraceIDFromContext(ctx context.Context) string {
	if sc, ok := SpanContextFromContext(ctx); ok {
		return sc.TraceID
	}
	return ""
}
