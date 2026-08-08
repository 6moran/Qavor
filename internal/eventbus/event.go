package eventbus

import "encoding/json"

// 事件类型，对应 SSE 的 event: 字段
const (
	EventMetadata = "metadata" // Run 元信息，首发事件
	EventMessage  = "message"  // 内容事件（token / 工具调用 / 消息片段）
	EventEnd      = "end"      // Run 终态（completed / interrupted / cancelled）
	EventError    = "error"    // 错误终态
)

// Run 终态 status 值（用于 end 事件 payload.status）
const (
	StatusCompleted   = "completed"
	StatusInterrupted = "interrupted"
	StatusCancelled   = "cancelled"
	StatusFailed      = "failed"
)

// Event Redis Stream 中存储的 Run 事件
type Event struct {
	EventType string          `json:"event"` // metadata / message / end / error
	RunID     string          `json:"run_id"`
	ThreadID  string          `json:"thread_id"`
	RequestID string          `json:"request_id"`
	Payload   json.RawMessage `json:"payload"` // 事件负载（chunk / items / status 等）
}

// StreamEntry Subscriber 读取到的一条 Stream 记录
type StreamEntry struct {
	ID    string // Redis Stream 的 {timestamp}-{seq}
	Event Event
}

// MetadataPayload metadata 事件负载
type MetadataPayload struct {
	RunType string `json:"run_type"` // chat / resume / subagent
	Source  string `json:"source"`   // chat
}

// ChunkPayload message 事件的 chunk 负载
// Type 取值：
//   - text_delta:  AI 文本流增量（同一段输出共享 MessageID）
//   - tool_call:   AI 发起工具调用
//   - tool_result: 工具返回结果
//   - message_end: 一段消息输出结束（流式边界信号）
type ChunkPayload struct {
	MessageID string        `json:"message_id"`          // 聚合同一条消息的 token
	Type      string        `json:"type"`                // text_delta / tool_call / tool_result / message_end
	Role      string        `json:"role,omitempty"`      // assistant / tool
	Content   string        `json:"content,omitempty"`   // 文本内容
	ToolCall  *ToolCallInfo `json:"tool_call,omitempty"` // 工具调用结构化字段
}

// ToolCallInfo 工具调用信息（tool_call / tool_result 事件携带）
type ToolCallInfo struct {
	ID    string `json:"id"`              // tool_call_id（tool_call 必填，tool_result 可空）
	Name  string `json:"name"`            // 工具名
	Args  string `json:"args,omitempty"`  // 参数（JSON 字符串，tool_call 携带）
	Index int    `json:"index,omitempty"` // chunk 拼接索引（流式工具调用）
}

// ItemsPayload message 事件的批量 chunk 负载
type ItemsPayload struct {
	Items []ChunkPayload `json:"items"`
}

// EndPayload end 事件负载
type EndPayload struct {
	Status   string           `json:"status"`             // completed / interrupted / cancelled
	Approval *ApprovalPayload `json:"approval,omitempty"` // 审批信息（仅 interrupted 时填充）
}

// ApprovalPayload 审批信息（嵌入 EndPayload）
type ApprovalPayload struct {
	ActionRequests []ApprovalActionRequest `json:"action_requests"`
	ReviewConfigs  []ApprovalReviewConfig  `json:"review_configs"`
}

// ApprovalActionRequest 待审批的工具调用（前端展示用）
type ApprovalActionRequest struct {
	Name string `json:"name"`
	Args string `json:"args"`
}

// ApprovalReviewConfig 审批配置（前端展示用）
type ApprovalReviewConfig struct {
	ToolName string `json:"tool_name"`
	Args     string `json:"args"`
	Reason   string `json:"reason"`
}

// ErrorPayload error 事件负载
type ErrorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Envelope SSE 推送给前端的信封（data 字段内容）
type Envelope struct {
	Event     string          `json:"event"`
	RunID     string          `json:"run_id"`
	ThreadID  string          `json:"thread_id,omitempty"`
	RequestID string          `json:"request_id,omitempty"`
	Payload   json.RawMessage `json:"payload"`
}

// NewEnvelope 从 Event 构造 SSE 信封
func NewEnvelope(e Event) Envelope {
	return Envelope{
		Event:     e.EventType,
		RunID:     e.RunID,
		ThreadID:  e.ThreadID,
		RequestID: e.RequestID,
		Payload:   e.Payload,
	}
}

// streamKey 返回 Run 事件流的 Redis key
func streamKey(runID string) string {
	return "qavor:run:" + runID + ":events"
}
