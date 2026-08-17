package service

import (
	"context"
	"time"

	"Qavor/internal/model/entity"
)

// TraceItem Trace 列表项（基于 agent.run Span 聚合）
type TraceItem struct {
	TraceID           string    `json:"trace_id"`
	RunID             string    `json:"run_id,omitempty"`
	RequestID         string    `json:"request_id,omitempty"`
	AgentSlug         string    `json:"agent_slug,omitempty"`
	QuerySummary      string    `json:"query_summary,omitempty"`
	AgentStatus       string    `json:"agent_status"`
	BusinessRunStatus string    `json:"business_run_status,omitempty"`
	StatusMismatch    bool      `json:"status_mismatch"`
	DurationMs        int64     `json:"duration_ms"`
	QueueWaitMs       int64     `json:"queue_wait_ms,omitempty"`
	LLMCount          int       `json:"llm_count"`
	ToolCount         int       `json:"tool_count"`
	TotalTokens       int       `json:"total_tokens"`
	FirstError        string    `json:"first_error,omitempty"`
	StartedAt         time.Time `json:"started_at"`
}

// TraceSpanItem span 明细（平铺，前端按 parent_span_id 组装树）
type TraceSpanItem struct {
	SpanID            string      `json:"span_id"`
	ParentSpanID      string      `json:"parent_span_id,omitempty"`
	Kind              string      `json:"kind"`
	Operation         string      `json:"operation"`
	DisplayName       string      `json:"display_name,omitempty"`
	RunID             string      `json:"run_id,omitempty"`
	RequestID         string      `json:"request_id,omitempty"`
	Status            string      `json:"status"`
	StartedAt         time.Time   `json:"started_at"`
	EndedAt           *time.Time  `json:"ended_at,omitempty"`
	DurationMs        int64       `json:"duration_ms"`
	InputSummary      string      `json:"input_summary,omitempty"`
	OutputSummary     string      `json:"output_summary,omitempty"`
	TokensIn          int         `json:"tokens_in,omitempty"`
	TokensOut         int         `json:"tokens_out,omitempty"`
	ReasoningTokens   int         `json:"reasoning_tokens,omitempty"`
	ErrorType         string      `json:"error_type,omitempty"`
	ErrorMessage      string      `json:"error_message,omitempty"`
	Attributes        entity.JSON `json:"attributes,omitempty"`
	TriggeredBySpanID string      `json:"triggered_by_span_id,omitempty"`
}

// TraceRunSummary 关联 Run 摘要（业务状态真相源）
type TraceRunSummary struct {
	RunID      string     `json:"run_id,omitempty"`
	Status     string     `json:"status,omitempty"`
	ErrorType  string     `json:"error_type,omitempty"`
	StartedAt  *time.Time `json:"started_at,omitempty"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
}

// TraceDiagnostic 诊断提示（用于详情页顶部展示异常）
type TraceDiagnostic struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	SpanID  string `json:"span_id,omitempty"`
}

// TraceDetail Trace 详情
type TraceDetail struct {
	Trace       entity.TraceRecord `json:"trace"`
	Run         *TraceRunSummary   `json:"run,omitempty"`
	Spans       []TraceSpanItem    `json:"spans"`
	SpanTotal   int64              `json:"span_total"`
	Diagnostics []TraceDiagnostic  `json:"diagnostics,omitempty"`
}

// TraceService 链路追踪服务接口
type TraceService interface {
	ListTraces(ctx context.Context, filter TraceListFilter) ([]TraceItem, int64, error)
	GetTraceDetail(ctx context.Context, traceID string) (*TraceDetail, error)
	GetSpanDetail(ctx context.Context, spanID string) (*TraceSpanItem, error)
	GetTraceByRunID(ctx context.Context, runID string) (string, error)
}

// TraceListFilter 列表筛选（controller 解析后传入）
type TraceListFilter struct {
	Keyword        string
	AgentSlug      string
	ConversationID uint
	Status         string
	Model          string
	Tool           string
	ErrorOnly      bool
	MismatchOnly   bool
	From           time.Time
	To             time.Time
	Page           int
	PageSize       int
}

// RunStatusReader 读取 AgentRun 业务状态（由 repository.AgentRunRepository 实现）
// 仅依赖 GetByID，避免 Service 直接依赖完整 AgentRunRepository 接口。
type RunStatusReader interface {
	GetByID(id string) (*entity.AgentRun, error)
}
