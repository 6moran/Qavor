package sse

import "time"

const (
	// MaxConcurrentTasksPerUser 单用户最大并发任务数
	MaxConcurrentTasksPerUser = 5
)

// SSEConfig SSE 配置
type SSEConfig struct {
	MaxStreamTime     time.Duration // 单次流式最大时长
	HeartbeatInterval time.Duration // 流式过程中心跳间隔
}

// StreamRequest 流式对话请求
type StreamRequest struct {
	ConversationID uint   `json:"conversation_id" binding:"required"`
	Content        string `json:"content" binding:"required"`
	AgentSlug      string `json:"agent_slug"`   // 可选：指定 Agent
	ModelName      string `json:"model_name"`   // 可选：指定模型
	FileIDs        []uint `json:"file_ids"`     // 可选：关联的文件ID列表
}

// FileUploadRequest 文件上传请求
type FileUploadRequest struct {
	ConversationID uint   `json:"conversation_id" binding:"required"`
	FileType       string `json:"file_type" binding:"required,oneof=image document code audio video"`
	FileName       string `json:"file_name" binding:"required"`
	FileSize       int64  `json:"file_size" binding:"required"`
}

// 支持的文件类型
const (
	FileTypeImage    = "image"    // 图片：jpg, png, gif, webp
	FileTypeDocument = "document" // 文档：pdf, doc, docx, txt, md
	FileTypeCode     = "code"     // 代码：js, ts, py, go, java, etc.
	FileTypeAudio    = "audio"    // 音频：mp3, wav, m4a
	FileTypeVideo    = "video"    // 视频：mp4, webm, avi
)

// 文件大小限制
const (
	MaxFileSizeImage    = 10 * 1024 * 1024  // 10MB
	MaxFileSizeDocument = 20 * 1024 * 1024  // 20MB
	MaxFileSizeCode     = 1 * 1024 * 1024   // 1MB
	MaxFileSizeAudio    = 50 * 1024 * 1024  // 50MB
	MaxFileSizeVideo    = 100 * 1024 * 1024 // 100MB
)

// FileUploadResponse 文件上传响应
type FileUploadResponse struct {
	FileID    uint   `json:"file_id"`
	FileName  string `json:"file_name"`
	FileSize  int64  `json:"file_size"`
	FileType  string `json:"file_type"`
	UploadURL string `json:"upload_url"` // 预签名上传URL
}

// CancelRequest 取消请求
type CancelRequest struct {
	TaskID string `json:"task_id" binding:"required"`
}

// RegenerateRequest 重新生成请求
type RegenerateRequest struct {
	ConversationID uint `json:"conversation_id" binding:"required"`
}

// EditRequest 编辑消息请求
type EditRequest struct {
	ConversationID uint   `json:"conversation_id" binding:"required"`
	MessageID      uint   `json:"message_id" binding:"required"`  // 原始消息ID
	NewContent     string `json:"new_content" binding:"required"` // 新的消息内容
}

// StreamResponse 流式对话响应（非SSE，用于错误场景）
type StreamResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}
