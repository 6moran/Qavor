package service

import (
	"context"

	"Qavor/internal/sse"

	"github.com/cloudwego/eino/schema"
)

// SSEService SSE 流式服务接口
type SSEService interface {
	// Stream 处理流式对话
	Stream(ctx context.Context, req *StreamRequest) error

	// Cancel 取消任务
	Cancel(taskID string) error

	// UploadFile 上传文件
	UploadFile(userID uint, req *FileUploadRequest) (*FileUploadResponse, error)

	// ProcessFile 处理文件
	ProcessFile(userID uint, fileID uint) error
}

// StreamRequest 流式对话请求
type StreamRequest struct {
	TaskID         string
	UserID         uint
	ConversationID uint
	Content        string
	AgentSlug      string
	ModelName      string
	FileIDs        []uint
	Writer         *sse.SSEWriter
}

// FileUploadRequest 文件上传请求
type FileUploadRequest struct {
	ConversationID uint   `json:"conversation_id" binding:"required"`
	FileType       string `json:"file_type" binding:"required,oneof=image document code audio video"`
	FileName       string `json:"file_name" binding:"required"`
	FileSize       int64  `json:"file_size" binding:"required"`
}

// FileUploadResponse 文件上传响应
type FileUploadResponse struct {
	FileID    uint   `json:"file_id"`
	FileName  string `json:"file_name"`
	FileSize  int64  `json:"file_size"`
	FileType  string `json:"file_type"`
	UploadURL string `json:"upload_url"`
}

// SSEMessageStream SSE 消息流接口
type SSEMessageStream interface {
	Send(eventType sse.EventType, data interface{})
	SendHeartbeat(messageID string)
}

// ContextManager 上下文管理器接口（简化版）
type ContextManager interface {
	FetchContext(ctx context.Context, query *ContextHistoryQuery) (*ContextWindow, error)
	BuildPrompt(ctx context.Context, window *ContextWindow, userMessage *schema.Message) []*schema.Message
	PersistUserMessage(ctx context.Context, conversationID uint, userMsg *schema.Message) (uint, error)
	PersistAssistantMessage(ctx context.Context, conversationID uint, assistantMsg *schema.Message) error
}

// ContextHistoryQuery 上下文历史查询
type ContextHistoryQuery struct {
	ConversationID uint
	Limit          int
}

// ContextWindow 上下文窗口
type ContextWindow struct {
	Messages []*schema.Message
}
