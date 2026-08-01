package sse

// 错误码常量
const (
	// 客户端错误 (4xx)
	ErrCodeInvalidRequest      = "INVALID_REQUEST"       // 请求参数无效
	ErrCodeUnauthorized        = "UNAUTHORIZED"           // 未授权
	ErrCodeForbidden           = "FORBIDDEN"              // 无权限
	ErrCodeNotFound            = "NOT_FOUND"              // 资源不存在
	ErrCodeTooManyRequests     = "TOO_MANY_REQUESTS"      // 请求过多

	// 服务端错误 (5xx)
	ErrCodeInternalError       = "INTERNAL_ERROR"         // 内部错误
	ErrCodeContextBuildFailed  = "CONTEXT_BUILD_FAILED"   // 上下文构建失败
	ErrCodeLLMInitFailed       = "LLM_INIT_FAILED"        // LLM初始化失败
	ErrCodeLLMStreamFailed     = "LLM_STREAM_FAILED"      // LLM流式调用失败
	ErrCodeStreamReadFailed    = "STREAM_READ_FAILED"     // 流式读取失败
	ErrCodePersistFailed       = "PERSIST_FAILED"         // 持久化失败
	ErrCodeTaskNotFound        = "TASK_NOT_FOUND"         // 任务不存在
	ErrCodeTaskCancelled       = "TASK_CANCELLED"         // 任务已取消
	ErrCodeTimeout             = "TIMEOUT"                // 超时

	// 文件错误
	ErrCodeFileTooLarge        = "FILE_TOO_LARGE"         // 文件大小超限
	ErrCodeUnsupportedFileType = "UNSUPPORTED_FILE_TYPE"  // 不支持的文件类型
	ErrCodeFileParseFailed     = "FILE_PARSE_FAILED"      // 文件解析失败
	ErrCodeOCRFailed           = "OCR_FAILED"             // OCR识别失败
	ErrCodeStorageUnavailable  = "STORAGE_UNAVAILABLE"    // 存储服务不可用
	ErrCodeFileProcessFailed   = "FILE_PROCESS_FAILED"    // 文件处理失败

	// 业务错误
	ErrCodeConversationNotFound    = "CONVERSATION_NOT_FOUND"    // 会话不存在
	ErrCodeConversationClosed      = "CONVERSATION_CLOSED"       // 会话已关闭
	ErrCodeMessageNotFound         = "MESSAGE_NOT_FOUND"         // 消息不存在
)

// ErrorResponse 错误响应
type ErrorResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// NewErrorResponse 创建错误响应
func NewErrorResponse(code int, message string) *ErrorResponse {
	return &ErrorResponse{
		Code:    code,
		Message: message,
	}
}

// SuccessResponse 成功响应
type SuccessResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// NewSuccessResponse 创建成功响应
func NewSuccessResponse(data interface{}) *SuccessResponse {
	return &SuccessResponse{
		Code:    200,
		Message: "success",
		Data:    data,
	}
}

// 错误码与HTTP状态码映射
var errorCodeToHTTPStatus = map[string]int{
	ErrCodeInvalidRequest:      400,
	ErrCodeUnauthorized:        401,
	ErrCodeForbidden:           403,
	ErrCodeNotFound:            404,
	ErrCodeTooManyRequests:     429,
	ErrCodeInternalError:       500,
	ErrCodeContextBuildFailed:  500,
	ErrCodeLLMInitFailed:       500,
	ErrCodeLLMStreamFailed:     500,
	ErrCodeStreamReadFailed:    500,
	ErrCodePersistFailed:       500,
	ErrCodeTaskNotFound:        404,
	ErrCodeTaskCancelled:       499,  // 自定义状态码
	ErrCodeTimeout:             504,
	ErrCodeFileTooLarge:        413,  // Payload Too Large
	ErrCodeUnsupportedFileType: 415,  // Unsupported Media Type
	ErrCodeFileParseFailed:     422,  // Unprocessable Entity
	ErrCodeOCRFailed:           422,
	ErrCodeStorageUnavailable:  503,  // Service Unavailable
	ErrCodeFileProcessFailed:   422,
	ErrCodeConversationNotFound: 404,
	ErrCodeConversationClosed:  400,
	ErrCodeMessageNotFound:     404,
}

// GetHTTPStatus 获取错误码对应的HTTP状态码
func GetHTTPStatus(errCode string) int {
	if status, ok := errorCodeToHTTPStatus[errCode]; ok {
		return status
	}
	return 500
}
