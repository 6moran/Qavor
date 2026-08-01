package sse

// EventType SSE 事件类型
type EventType string

const (
	// 消息事件
	EventMessageStart     EventType = "message.start"     // 消息开始
	EventMessageDelta     EventType = "message.delta"     // 增量内容
	EventMessageComplete  EventType = "message.complete"  // 消息完成
	EventMessageError     EventType = "message.error"     // 消息错误
	EventMessageCancelled EventType = "message.cancelled" // 消息取消（用户中断）

	// 文件事件
	EventFileUploadStart    EventType = "file.upload_start"    // 文件上传开始
	EventFileUploadProgress EventType = "file.upload_progress" // 文件上传进度
	EventFileUploadComplete EventType = "file.upload_complete" // 文件上传完成
	EventFileUploadError    EventType = "file.upload_error"    // 文件上传失败
	EventFileProcessStart   EventType = "file.process_start"   // 文件处理开始
	EventFileProcessDone    EventType = "file.process_done"    // 文件处理完成
	EventFileProcessError   EventType = "file.process_error"   // 文件处理失败

	// 工具调用事件
	EventToolCallStart EventType = "tool_call.start" // 工具调用开始
	EventToolCallEnd   EventType = "tool_call.end"   // 工具调用结束

	// RAG 事件
	EventRAGStart EventType = "rag.start" // 检索开始
	EventRAGDone  EventType = "rag.done"  // 检索完成

	// 心跳事件
	EventHeartbeat EventType = "heartbeat" // 心跳

	// 流结束
	EventDone EventType = "done" // 流结束
)

// SSEEvent SSE 事件
type SSEEvent struct {
	Type EventType   `json:"type"`
	Data interface{} `json:"data"`
}

// --- 消息事件数据 ---

// MessageStartData 消息开始事件
type MessageStartData struct {
	MessageID      string `json:"message_id"`
	ConversationID uint   `json:"conversation_id"`
	Model          string `json:"model"`
}

// MessageDeltaData 增量内容事件
type MessageDeltaData struct {
	MessageID string `json:"message_id"`
	Content   string `json:"content"` // 增量内容片段
	Index     int    `json:"index"`   // 片段序号
}

// MessageCompleteData 消息完成事件
type MessageCompleteData struct {
	MessageID    string `json:"message_id"`
	Content      string `json:"content"`       // 完整内容
	TokenCount   int    `json:"token_count"`   // Token 消耗
	FinishReason string `json:"finish_reason"` // 结束原因
}

// MessageCancelledData 消息取消事件
type MessageCancelledData struct {
	MessageID string `json:"message_id"`
	Reason    string `json:"reason"` // 取消原因：user_cancel / new_message
}

// --- 文件事件数据 ---

// FileUploadStartData 文件上传开始事件
type FileUploadStartData struct {
	FileID   uint   `json:"file_id"`
	FileName string `json:"file_name"`
	FileSize int64  `json:"file_size"`
	FileType string `json:"file_type"`
}

// FileUploadProgressData 文件上传进度事件
type FileUploadProgressData struct {
	FileID   uint    `json:"file_id"`
	Progress float64 `json:"progress"` // 0.0 - 1.0
	Loaded   int64   `json:"loaded"`
	Total    int64   `json:"total"`
}

// FileUploadCompleteData 文件上传完成事件
type FileUploadCompleteData struct {
	FileID     uint   `json:"file_id"`
	FileName   string `json:"file_name"`
	FileSize   int64  `json:"file_size"`
	FileType   string `json:"file_type"`
	FileURL    string `json:"file_url"`
	PreviewURL string `json:"preview_url"`
}

// FileProcessStartData 文件处理开始事件
type FileProcessStartData struct {
	FileID   uint   `json:"file_id"`
	FileName string `json:"file_name"`
	Process  string `json:"process"` // 处理类型：ocr / parse / extract
}

// FileProcessDoneData 文件处理完成事件
type FileProcessDoneData struct {
	FileID      uint   `json:"file_id"`
	FileName    string `json:"file_name"`
	Content     string `json:"content"`      // 提取的文本内容
	TokenCount  int    `json:"token_count"`  // 内容的Token数
	ProcessTime int64  `json:"process_time"` // 处理耗时（ms）
}

// FileProcessErrorData 文件处理失败事件
type FileProcessErrorData struct {
	FileID   uint   `json:"file_id"`
	FileName string `json:"file_name"`
	Error    string `json:"error"`
	Code     string `json:"code"`
}

// --- 工具调用事件数据 ---

// ToolCallStartData 工具调用开始事件
type ToolCallStartData struct {
	MessageID  string `json:"message_id"`
	ToolName   string `json:"tool_name"`
	ToolCallID string `json:"tool_call_id"`
}

// ToolCallEndData 工具调用结束事件
type ToolCallEndData struct {
	MessageID  string `json:"message_id"`
	ToolName   string `json:"tool_name"`
	ToolCallID string `json:"tool_call_id"`
	Success    bool   `json:"success"`
}

// --- RAG 事件数据 ---

// RAGStartData RAG检索开始事件
type RAGStartData struct {
	Query           string `json:"query"`
	KnowledgeBaseID uint   `json:"knowledge_base_id"`
}

// RAGDoneData RAG检索完成事件
type RAGDoneData struct {
	Query       string `json:"query"`
	ResultCount int    `json:"result_count"`
	ChunksUsed  int    `json:"chunks_used"`
}

// --- 心跳事件数据 ---

// HeartbeatData 心跳事件
type HeartbeatData struct {
	MessageID string `json:"message_id"`
	Timestamp int64  `json:"timestamp"`
}

// --- 错误事件数据 ---

// ErrorData 错误事件
type ErrorData struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
