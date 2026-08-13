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
// 包含终态信息（状态、时间）、输出摘要、错误详情、token 用量和自定义属性
// 由调用方在 span.End() 时传入，Writer 异步写入 trace_spans 表
type SpanEnd struct {
	Status          string      // 终态：ok / error / cancelled / interrupted / timeout
	EndedAt         time.Time   // 结束时间戳（零值时自动填充当前时间）
	OutputSummary   string      // 输出摘要（脱敏截断后的模型回复/工具结果）
	ErrorType       string      // 错误类型（panic / queue_enqueue 等）
	ErrorMessage    string      // 错误信息（脱敏截断后）
	TokensIn        int         // 输入 token 数（仅 LLM/embedding span）
	TokensOut       int         // 输出 token 数（仅 LLM span）
	ReasoningTokens int         // 推理 token 数（reasoning 模型如 o1）
	Attributes      entity.JSON // 自定义属性（jsonb，如 tool_call_ids / http.status_code）
}

// Config Tracer 配置，从 config.yaml 的 trace 节点读取
// 控制链路追踪的开关、内容模式、截断长度、数据保留策略和路由白名单
type Config struct {
	Enabled          bool          // 总开关：false 时所有埋点为 no-op，业务零侵入
	ContentMode      string        // 内容模式："summary"（截断+脱敏）或 "none"（不存任何内容）
	MaxContentLength int           // 内容字段最大字符数（默认 500），用于截断 input/output summary
	Retention        time.Duration // 数据保留时长（默认 7 天），过期由 Janitor 物理删除
	TracedRoutes     []string      // 追踪的路由白名单（格式："METHOD /path"，如 "POST /api/v1/chat"）
}

// SpanSpec 创建 Span 的规格（类似 OTel 的 SpanConfig）
// 由埋点代码填充，Tracer.StartSpan() 据此构建 TraceSpan 实体并异步写入
// TraceID/ParentSpanID/RunID/RequestID 优先级：spec > 父 SpanContext
type SpanSpec struct {
	TraceID      string      // 所属 Trace 的 ID（为空时 Tracer 自动生成新 UUID，成为新 Trace 的根）
	ParentSpanID string      // 父 Span ID（为空时从父 SpanContext 自动继承）
	RunID        string      // 业务 Run ID（agent_runs 表主键，异步任务必填）
	RequestID    string      // 请求 ID（X-Request-Id 头或生成）
	Kind         string      // 组件类型：llm / tool / retriever / embedding / agent / http / context
	Operation    string      // 操作名（语义化，如 llm.generate / tool.execute / http.server）
	DisplayName  string      // 展示名（面向人，如模型名 / 工具名 / Agent slug）
	InputSummary string      // 输入摘要（脱敏截断后，如 prompt 摘要 / 工具参数 / 检索 query）
	Attributes   entity.JSON // 自定义属性（jsonb，如 temperature / model / http.method）
}

// RequestMeta HTTP 请求元信息（Middleware 创建 HTTP Span 时使用）
// 由 gin 中间件从请求头和路由信息中提取，用于构建 http.server span 和 TraceRecord
type RequestMeta struct {
	TraceID        string // 外部传入的 TraceID（X-Trace-Id 头，必须是合法 UUID），为空时自动生成
	RequestID      string // 请求 ID（X-Request-Id 头或生成）
	RunID          string // 业务 Run ID（同步场景为空，异步场景由 Worker 注入）
	ConversationID uint   // 会话 ID（中间件阶段为空，由 Controller 调用 UpdateRequestMetadata 补全）
	QuerySummary   string // 用户问题摘要（中间件阶段为空，由 Controller 补全）
	EntryType      string // 入口类型：http / agent
	Method         string // HTTP 方法（GET / POST / PUT / DELETE）
	Path           string // 请求路径（如 /api/v1/chat）
}

// TraceMetadata 请求体解析后用于补全 TraceRecord 的非空字段集合
// 由 Controller 在解析完请求体后调用 tracer.UpdateRequestMetadata() 补全
// 解决了"中间件不读请求体"的设计约束
type TraceMetadata struct {
	ConversationID uint   // 会话 ID
	QuerySummary   string // 用户问题摘要（脱敏截断后）
	EntryType      string // 入口类型：http / agent
}

// SpanWriter 异步写入接口（Tracer 依赖，不包含查询方法）
type SpanWriter interface {
	CreateTrace(ctx context.Context, record *entity.TraceRecord) error
	UpdateTraceMetadata(ctx context.Context, traceID string, meta TraceMetadata) error
	StartSpan(ctx context.Context, span *entity.TraceSpan) error
	EndSpan(ctx context.Context, spanID string, end SpanEnd) error
}

// TraceFilter 新表列表筛选条件（对应 GET /api/v1/traces 的查询参数）
// 用于 TraceRepository.ListTraces() 的分页和筛选，支持关键词、Agent、状态、时间范围等多维度过滤
type TraceFilter struct {
	Keyword        string    // 关键词（模糊匹配 query_summary / agent_slug）
	AgentSlug      string    // Agent slug（精确匹配）
	ConversationID uint      // 会话 ID（精确匹配）
	Status         string    // Agent 执行状态（ok / error / cancelled 等）
	Model          string    // 模型名称（模糊匹配 llm.generate span 的 attributes.model）
	Tool           string    // 工具名称（模糊匹配 tool.execute span 的 display_name）
	ErrorOnly      bool      // 只看错误（只返回含 error span 的 trace）
	MismatchOnly   bool      // 只看不匹配（trace 状态与业务状态不一致）
	From           time.Time // 起始时间（created_at >= from）
	To             time.Time // 结束时间（created_at < to）
	Page           int       // 页码（从 1 开始）
	PageSize       int       // 每页数量（默认 20，最大 100）
}

// TraceSummary 列表项聚合摘要（对应 TraceListView 的一行数据）
// 由 TraceRepository.ListTraces() 聚合计算，消除了 N+1 查询问题
// 只为含 agent.run span 的 trace 生成列表项（过滤掉没有根的零碎 span）
type TraceSummary struct {
	TraceID           string    // Trace ID（主键）
	RunID             string    // 业务 Run ID（agent_runs 表主键）
	RequestID         string    // 请求 ID
	AgentSlug         string    // Agent slug（如 assistant / rag-bot）
	QuerySummary      string    // 用户问题摘要（脱敏截断后）
	AgentStatus       string    // Agent 执行状态（从 agent.run span 取：ok / error / cancelled 等）
	BusinessRunStatus string    // 业务 Run 状态（从 agent_runs 表取：pending / running / success / failed）
	DurationMs        int64     // 总耗时（根到最深 span 的时间跨度）
	QueueWaitMs       int64     // 队列等待时长（入队到消费的间隔，从 queue.enqueue/consume 计算）
	LLMCount          int       // LLM 调用次数（llm.generate span 数量）
	ToolCount         int       // 工具调用次数（tool.execute span 数量）
	TotalTokens       int       // 总 token 用量（所有 LLM span 的 TokensIn + TokensOut 之和）
	FirstError        string    // 首个错误消息（只取第一个 error span 的 ErrorMessage）
	StartedAt         time.Time // 开始时间（http.server span 的 StartedAt）
}

// RunSpanRef 指向一次已记录的 agent.run Span 的引用
// 用于恢复/重试场景建立因果链：resume 时新 Run 会带上 ResumeFromRunID + ResumeFromSpanID
// 前端可追溯"这次执行是从哪次中断的继续"
type RunSpanRef struct {
	TraceID string // 所属 Trace 的 ID
	SpanID  string // agent.run span 的 ID
	Attempt int    // 第几次尝试（从 1 开始，首次执行为 1）
}

// TraceRepository 新表数据访问接口（由 repository 包实现）
type TraceRepository interface {
	SpanWriter
	GetTrace(ctx context.Context, traceID string) (*entity.TraceRecord, error)
	ListTraces(ctx context.Context, filter TraceFilter) ([]TraceSummary, int64, error)
	ListSpans(ctx context.Context, traceID string) ([]*entity.TraceSpan, error)
	GetSpan(ctx context.Context, spanID string) (*entity.TraceSpan, error)
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
