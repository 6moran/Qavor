package service

import (
	"context"
	"time"
)

// TraceItem Trace 列表项
type TraceItem struct {
	TraceID      string     `json:"trace_id"`
	Source       string     `json:"source"`
	AgentSlug    string     `json:"agent_slug"`
	Query        string     `json:"query"`
	Status       string     `json:"status"`
	ErrorMessage string     `json:"error_message,omitempty"`
	DurationMs   int64      `json:"duration_ms"`
	ModelName    string     `json:"model_name"`
	TotalTokens  int        `json:"total_tokens"`
	StartedAt    time.Time  `json:"started_at"`
	EndedAt      *time.Time `json:"ended_at,omitempty"`
}

// TraceSpanItem span 明细（平铺，前端按 parent_span_id 组装树）
type TraceSpanItem struct {
	SpanID          string     `json:"span_id"`
	ParentSpanID    string     `json:"parent_span_id"`
	Kind            string     `json:"kind"`
	Name            string     `json:"name"`
	Status          string     `json:"status"`
	StartedAt       time.Time  `json:"started_at"`
	EndedAt         *time.Time `json:"ended_at,omitempty"`
	DurationMs      int64      `json:"duration_ms"`
	InputSummary    string     `json:"input_summary"`
	OutputSummary   string     `json:"output_summary"`
	TokensIn        int        `json:"tokens_in"`
	TokensOut       int        `json:"tokens_out"`
	ReasoningTokens int        `json:"reasoning_tokens"`
	ErrorMessage    string     `json:"error_message"`
}

// TraceDetail Trace 详情
type TraceDetail struct {
	Trace TraceItem       `json:"trace"`
	Spans []TraceSpanItem `json:"spans"`
}

// TraceService 链路追踪服务接口
type TraceService interface {
	ListTraces(ctx context.Context, filter TraceListFilter) ([]TraceItem, int64, error)
	GetTraceDetail(ctx context.Context, traceID string) (*TraceDetail, error)
}

// TraceListFilter 列表筛选（controller 解析后传入）
type TraceListFilter struct {
	Keyword        string
	AgentSlug      string
	ConversationID uint
	Status         string
	Source         string
	From           time.Time
	To             time.Time
	Page           int
	PageSize       int
}
