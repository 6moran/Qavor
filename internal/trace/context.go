package trace

import (
	"context"
	"sync"
	"time"

	"Qavor/internal/model/entity"
	"Qavor/pkg/logger"

	"go.uber.org/zap"
)

type traceCtxKey struct{}
type spanCtxKey struct{}

// TraceContext 一次 Agent 执行（一次 Trace）的上下文状态
type TraceContext struct {
	TraceID        string
	Source         string // sync / stream / run
	AgentSlug      string
	ConversationID uint
	RunID          string
	RequestID      string
	Query          string

	mu          sync.Mutex
	rootCreated bool // 根记录是否已懒创建
	finished    bool // 是否已收尾（幂等）
}

// WithTraceContext 注入 trace 上下文
func WithTraceContext(ctx context.Context, tc *TraceContext) context.Context {
	return context.WithValue(ctx, traceCtxKey{}, tc)
}

// FromContext 读取 trace 上下文，无则返回 nil
func FromContext(ctx context.Context) *TraceContext {
	tc, _ := ctx.Value(traceCtxKey{}).(*TraceContext)
	return tc
}

// TraceIDFromContext 读取 TraceID（异步透传入队时使用），无则返回空串
func TraceIDFromContext(ctx context.Context) string {
	tc := FromContext(ctx)
	if tc == nil {
		return ""
	}
	return tc.TraceID
}

// spanState 单个 span 的内存态：Handler OnStart 注入，OnEnd/OnError 读取
type spanState struct {
	ID        string
	StartedAt time.Time
}

// WithSpan 注入当前 span 状态（OnStart 返回值传给同 Handler 的 OnEnd）
func WithSpan(ctx context.Context, st *spanState) context.Context {
	return context.WithValue(ctx, spanCtxKey{}, st)
}

// SpanFromContext 读取当前 span 状态，无则返回 nil
func SpanFromContext(ctx context.Context) *spanState {
	st, _ := ctx.Value(spanCtxKey{}).(*spanState)
	return st
}

// ensureRoot 懒创建根记录：首次组件调用时插入 agent_traces（status=running）
func (tc *TraceContext) ensureRoot(ctx context.Context, repo TraceRepository, maxLen int) {
	tc.mu.Lock()
	if tc.rootCreated {
		tc.mu.Unlock()
		return
	}
	tc.rootCreated = true
	tc.mu.Unlock()

	t := &entity.AgentTrace{
		TraceID:        tc.TraceID,
		Source:         tc.Source,
		AgentSlug:      tc.AgentSlug,
		ConversationID: tc.ConversationID,
		RunID:          tc.RunID,
		RequestID:      tc.RequestID,
		Query:          truncate(tc.Query, maxLen),
		Status:         entity.TraceStatusRunning,
		StartedAt:      time.Now(),
		CreatedAt:      time.Now(),
	}
	if err := repo.CreateTrace(ctx, t); err != nil {
		logger.Warn("trace: 创建根记录失败", zap.String("trace_id", tc.TraceID), zap.Error(err))
	}
}

func (tc *TraceContext) markFinished() {
	tc.mu.Lock()
	tc.finished = true
	tc.mu.Unlock()
}

// IsFinished 是否已收尾
func (tc *TraceContext) IsFinished() bool {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	return tc.finished
}
