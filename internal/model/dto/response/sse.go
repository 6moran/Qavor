package response

// FileUploadResponse 文件上传响应
type FileUploadResponse struct {
	FileID    uint   `json:"file_id"`
	FileName  string `json:"file_name"`
	FileSize  int64  `json:"file_size"`
	FileType  string `json:"file_type"`
	UploadURL string `json:"upload_url"` // 预签名上传URL
}

// SSEStreamResponse SSE 流式对话响应（非SSE，用于错误场景）
type SSEStreamResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// SSECancelResponse SSE 取消响应
type SSECancelResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// SSEProcessFileResponse SSE 文件处理响应
type SSEProcessFileResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}
