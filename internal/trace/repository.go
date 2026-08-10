package trace

import (
	"context"
	"time"

	"Qavor/internal/model/entity"
)

// —— 新架构类型（Task 2 引入）——

// Span 终态常量（新表 trace_spans 使用）
const (
	SpanStatusRunning     = "running"
	SpanStatusOK          = "ok"
	SpanStatusError       = "error"
	SpanStatusCancelled   = "cancelled"
	SpanStatusInterrupted = "interrupted"
	SpanStatusTimeout     = "timeout"
)

// SpanEnd Span 结束时的补充数据
type SpanEnd struct {
	Status          string
	EndedAt         time.Time
	OutputSummary   string
	ErrorType       string
	ErrorMessage    string
	TokensIn        int
	TokensOut       int
	ReasoningTokens int
	Attributes      entity.JSON
}

// Config Tracer 配置
type Config struct {
	Enabled          bool
	ContentMode      string
	MaxContentLength int
	Retention        time.Duration
	TracedRoutes     []string
}

// SpanSpec 创建 Span 的规格
type SpanSpec struct {
	TraceID      string
	ParentSpanID string
	RunID        string
	RequestID    string
	Kind         string
	Operation    string
	DisplayName  string
	InputSummary string
	Attributes   entity.JSON
}

// RequestMeta HTTP 请求元信息（Middleware 创建 HTTP Span 时使用）
type RequestMeta struct {
	TraceID        string
	RequestID      string
	ConversationID uint
	QuerySummary   string
	EntryType      string
	Method         string
	Path           string
}

// TraceMetadata 是请求体解析后用于补全 TraceRecord 的非空字段集合。
type TraceMetadata struct {
	ConversationID uint
	QuerySummary   string
	EntryType      string
}

// SpanWriter 异步写入接口（Tracer 依赖，不包含查询方法）
type SpanWriter interface {
	CreateTrace(ctx context.Context, record *entity.TraceRecord) error
	UpdateTraceMetadata(ctx context.Context, traceID string, meta TraceMetadata) error
	StartSpan(ctx context.Context, span *entity.TraceSpan) error
	EndSpan(ctx context.Context, spanID string, end SpanEnd) error
}

// TraceFilter 新表列表筛选条件
type TraceFilter struct {
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

// TraceSummary 列表项聚合摘要
type TraceSummary struct {
	TraceID           string
	RunID             string
	RequestID         string
	AgentSlug         string
	QuerySummary      string
	AgentStatus       string
	BusinessRunStatus string
	DurationMs        int64
	QueueWaitMs       int64
	LLMCount          int
	ToolCount         int
	TotalTokens       int
	FirstError        string
	StartedAt         time.Time
}

// RunSpanRef 指向一次已记录的 agent.run Span，用于恢复/重试建立因果关系。
type RunSpanRef struct {
	TraceID string
	SpanID  string
	Attempt int
}

// TraceRepository 新表数据访问接口（由 repository 包实现）
type TraceRepository interface {
	SpanWriter
	GetTrace(ctx context.Context, traceID string) (*entity.TraceRecord, error)
	ListTraces(ctx context.Context, filter TraceFilter) ([]TraceSummary, int64, error)
	ListSpans(ctx context.Context, traceID string) ([]*entity.TraceSpan, error)
	GetTraceIDByRunID(ctx context.Context, runID string) (string, error)
	GetAgentRunSpan(ctx context.Context, runID string) (*RunSpanRef, error)
	MarkTimeoutSpans(ctx context.Context, before time.Time) (int64, error)
	DeleteExpired(ctx context.Context, before time.Time) (int64, error)
}

// —— 旧表接口（迁移期保留，Task 7-9 逐步删除）——

// LegacyTraceFilter 旧表列表筛选条件
type LegacyTraceFilter struct {
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

// LegacyTraceRepository 旧表数据访问接口，由 repository 包实现（迁移期保留）
type LegacyTraceRepository interface {
	CreateTrace(ctx context.Context, t *entity.AgentTrace) error
	CreateSpan(ctx context.Context, s *entity.AgentTraceSpan) error
	UpdateSpan(ctx context.Context, s *entity.AgentTraceSpan) error
	GetTrace(ctx context.Context, traceID string) (*entity.AgentTrace, error)
	ListTraces(ctx context.Context, filter LegacyTraceFilter) ([]*entity.AgentTrace, int64, error)
	ListSpans(ctx context.Context, traceID string) ([]*entity.AgentTraceSpan, error)
	MarkTimeoutTraces(ctx context.Context, before time.Time) (int64, error)
	DeleteExpired(ctx context.Context, before time.Time) (int64, error)
}
