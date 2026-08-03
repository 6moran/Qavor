package request

// StreamRequest 流式对话请求
type StreamRequest struct {
	ConversationID uint   `json:"conversation_id" binding:"required"`
	Content        string `json:"content" binding:"required"`
	AgentSlug      string `json:"agent_slug"` // 可选：指定 Agent
	ModelName      string `json:"model_name"` // 可选：指定模型
	FileIDs        []uint `json:"file_ids"`   // 可选：关联的文件ID列表
}

// FileUploadRequest 文件上传请求
type FileUploadRequest struct {
	ConversationID uint   `json:"conversation_id" binding:"required"`
	FileType       string `json:"file_type" binding:"required,oneof=image document code audio video"`
	FileName       string `json:"file_name" binding:"required"`
	FileSize       int64  `json:"file_size" binding:"required"`
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

// ProcessFileRequest 处理文件请求
type ProcessFileRequest struct {
	FileID uint `json:"file_id" binding:"required"`
}
