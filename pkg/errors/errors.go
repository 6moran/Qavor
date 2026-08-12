package errors

import "fmt"

// BizError 业务错误
type BizError struct {
	Code    int
	Message string
	Detail  string // 原始错误详情（如脱敏后的连接测试错误），JSON 序列化为 detail,omitempty
	Err     error
}

// Error 实现 error 接口
func (e *BizError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%d] %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("[%d] %s", e.Code, e.Message)
}

// New 创建新的业务错误
func New(code int, message string) *BizError {
	return &BizError{
		Code:    code,
		Message: message,
	}
}

// NewWithErr 创建带原始错误的业务错误
func NewWithErr(code int, message string, err error) *BizError {
	return &BizError{
		Code:    code,
		Message: message,
		Err:     err,
	}
}

// NewWithDetail 创建带原始错误详情的业务错误
func NewWithDetail(code int, message, detail string) *BizError {
	return &BizError{
		Code:    code,
		Message: message,
		Detail:  detail,
	}
}

// NewDefault 创建使用默认消息的业务错误
func NewDefault(code int) *BizError {
	return &BizError{
		Code:    code,
		Message: GetMessage(code),
	}
}

// IsBizError 判断错误是否是业务错误
func IsBizError(err error) bool {
	if _, ok := err.(*BizError); ok {
		return true
	}
	return false
}

// 预定义常用错误
var (
	ErrBadRequest         = NewDefault(CodeBadRequest)
	ErrUnauthorized       = NewDefault(CodeUnauthorized)
	ErrForbidden          = NewDefault(CodeForbidden)
	ErrNotFound           = NewDefault(CodeNotFound)
	ErrInternalError      = NewDefault(CodeInternalError)
	ErrUserNotFound       = NewDefault(CodeUserNotFound)
	ErrUserAlreadyExists  = NewDefault(CodeUserAlreadyExists)
	ErrUserDisabled       = NewDefault(CodeUserDisabled)
	ErrInvalidCredentials = NewDefault(CodeInvalidCredentials)
	ErrInvalidToken       = NewDefault(CodeInvalidToken)
	ErrTokenExpired       = NewDefault(CodeTokenExpired)
	ErrInvalidParam       = NewDefault(CodeInvalidParam)
	ErrMissingParam       = NewDefault(CodeMissingParam)

	// LLM 错误
	ErrLLMInternalError   = NewDefault(CodeLLMInternalError)
	ErrLLMConfigError     = NewDefault(CodeLLMConfigError)
	ErrLLMRequestFailed   = NewDefault(CodeLLMRequestFailed)
	ErrLLMResponseInvalid = NewDefault(CodeLLMResponseInvalid)
	ErrLLMTimeout         = NewDefault(CodeLLMTimeout)
	ErrLLMTokenLimit      = NewDefault(CodeLLMTokenLimit)
)
