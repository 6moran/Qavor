package entity

import "time"

// Trace 状态常量
const (
	TraceStatusRunning   = "running"
	TraceStatusSuccess   = "success"
	TraceStatusFailed    = "failed"
	TraceStatusCancelled = "cancelled"
	TraceStatusTimeout   = "timeout"
)

// TraceSource 执行来源常量
const (
	TraceSourceSync   = "sync"
	TraceSourceStream = "stream"
	TraceSourceRun    = "run"
)

// SpanKind span 类型常量
const (
	SpanKindLLM       = "llm"
	SpanKindTool      = "tool"
	SpanKindRetriever = "retriever"
	SpanKindAgent     = "agent"
	SpanKindContext   = "context" // 上下文管理（FetchContext/CompressContext/BuildPrompt 等）
)

// SpanStatus span 状态常量
const (
	SpanStatusRunning = "running"
	SpanStatusSuccess = "success"
	SpanStatusError   = "error"
)

// AgentTrace 对话级聚合记录
type AgentTrace struct {
	ID             uint       `gorm:"primarykey;comment:自增主键" json:"id"`
	TraceID        string     `gorm:"type:varchar(64);uniqueIndex;not null;comment:TraceID（UUID），span 关联键" json:"trace_id"`
	Source         string     `gorm:"type:varchar(16);not null;default:sync;index;comment:来源：sync/stream/run" json:"source"`
	AgentSlug      string     `gorm:"type:varchar(64);index;comment:使用的 Agent" json:"agent_slug,omitempty"`
	ConversationID uint       `gorm:"index;comment:会话 ID，0=无" json:"conversation_id"`
	RunID          string     `gorm:"type:varchar(64);comment:异步 run_id，同步路径为空" json:"run_id,omitempty"`
	RequestID      string     `gorm:"type:varchar(64);comment:请求 ID" json:"request_id,omitempty"`
	Query          string     `gorm:"type:text;comment:用户问题（截断）" json:"query,omitempty"`
	Status         string     `gorm:"type:varchar(16);not null;default:running;index;comment:running/success/failed/cancelled/timeout" json:"status"`
	ErrorMessage   string     `gorm:"type:text;comment:失败原因（截断）" json:"error_message,omitempty"`
	StartedAt      time.Time  `gorm:"comment:首次组件调用时间" json:"started_at"`
	EndedAt        *time.Time `gorm:"comment:完成时间" json:"ended_at,omitempty"`
	DurationMs     int64      `gorm:"comment:总耗时（毫秒）" json:"duration_ms"`
	ModelName      string     `gorm:"type:varchar(128);comment:主模型名（最后完成的 LLM span）" json:"model_name,omitempty"`
	TotalTokens    int        `gorm:"comment:全部 LLM 调用 Token 之和" json:"total_tokens"`
	CreatedAt      time.Time  `json:"created_at"`
}

// TableName 指定表名
func (AgentTrace) TableName() string { return "agent_traces" }

// AgentTraceSpan span 明细
type AgentTraceSpan struct {
	ID              uint       `gorm:"primarykey;comment:自增主键" json:"id"`
	TraceID         string     `gorm:"type:varchar(64);not null;index;comment:关联 agent_traces.trace_id" json:"trace_id"`
	SpanID          string     `gorm:"type:varchar(64);not null;comment:UUID，span 唯一标识" json:"span_id"`
	ParentSpanID    string     `gorm:"type:varchar(64);comment:父 span，空=根 span" json:"parent_span_id,omitempty"`
	Kind            string     `gorm:"type:varchar(16);not null;comment:llm/tool/retriever/agent" json:"kind"`
	Name            string     `gorm:"type:varchar(128);comment:模型名/工具名/节点名" json:"name,omitempty"`
	Status          string     `gorm:"type:varchar(16);not null;default:running;comment:running/success/error" json:"status"`
	StartedAt       time.Time  `gorm:"comment:开始时间" json:"started_at"`
	EndedAt         *time.Time `gorm:"comment:结束时间" json:"ended_at,omitempty"`
	DurationMs      int64      `gorm:"comment:耗时（毫秒）" json:"duration_ms"`
	InputSummary    string     `gorm:"type:text;comment:prompt 摘要/工具参数（截断）" json:"input_summary,omitempty"`
	OutputSummary   string     `gorm:"type:text;comment:回复摘要/工具结果（截断）" json:"output_summary,omitempty"`
	TokensIn        int        `gorm:"comment:输入 Token" json:"tokens_in"`
	TokensOut       int        `gorm:"comment:输出 Token" json:"tokens_out"`
	ReasoningTokens int        `gorm:"comment:推理 Token（reasoning 模型）" json:"reasoning_tokens"`
	ErrorMessage    string     `gorm:"type:text;comment:错误信息（截断）" json:"error_message,omitempty"`
	OrderIndex      int        `gorm:"comment:同层顺序，前端渲染排序" json:"order_index"`
	Extra           JSON       `gorm:"type:jsonb;comment:扩展元数据（预留）" json:"extra,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

// TableName 指定表名
func (AgentTraceSpan) TableName() string { return "agent_trace_spans" }
