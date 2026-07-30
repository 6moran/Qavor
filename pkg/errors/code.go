package errors

// 错误码定义
const (
	// 成功
	CodeSuccess = 0

	// 通用错误 4xx
	CodeBadRequest       = 400
	CodeUnauthorized     = 401
	CodeForbidden        = 403
	CodeNotFound         = 404
	CodeMethodNotAllowed = 405
	CodeRequestTimeout   = 408
	CodeConflict         = 409

	// 服务器错误 5xx
	CodeInternalError      = 500
	CodeNotImplemented     = 501
	CodeServiceUnavailable = 503

	// 业务错误 1xxx
	CodeUserNotFound       = 1001
	CodeUserAlreadyExists  = 1002
	CodeInvalidCredentials = 1003
	CodeUserDisabled       = 1004
	CodeInvalidToken       = 1005
	CodeTokenExpired       = 1006

	// 密码重置相关错误 1007-1012
	CodeInvalidResetCode  = 1007
	CodeResetCodeExpired  = 1008
	CodeResetCodeSent     = 1009
	CodeEmailNotVerified  = 1010
	CodeEmailNotExists    = 1011
	CodeResetTokenInvalid = 1012

	// 参数错误 2xxx
	CodeInvalidParam     = 2001
	CodeMissingParam     = 2002
	CodeParamFormatError = 2003

	// 资源错误 3xxx
	CodeResourceNotFound      = 3001
	CodeResourceAlreadyExists = 3002
	CodeResourceLocked        = 3003

	// LLM 错误 4xxx
	CodeLLMInternalError   = 4001
	CodeLLMConfigError     = 4002
	CodeLLMRequestFailed   = 4003
	CodeLLMResponseInvalid = 4004
	CodeLLMTimeout         = 4005
	CodeLLMTokenLimit      = 4006

	// 模型提供商错误 5xxx
	CodeModelProviderNotFound      = 5001
	CodeModelProviderAlreadyExists = 5002
	CodeModelProviderDisabled      = 5003
	CodeModelProviderAPIKeyMissing = 5004

	// 会话错误 40xxx
	CodeConversationNotFound      = 40001
	CodeConversationAccessDenied  = 40002
	CodeConversationStatusInvalid = 40003

	// 消息错误 400xx
	CodeMessageNotFound     = 40011
	CodeMessageAccessDenied = 40012
	CodeMessageRoleInvalid  = 40013
)

// 错误码对应的文本消息
var codeMessages = map[int]string{
	CodeSuccess:               "成功",
	CodeBadRequest:            "请求参数错误",
	CodeUnauthorized:          "未授权",
	CodeForbidden:             "禁止访问",
	CodeNotFound:              "资源不存在",
	CodeMethodNotAllowed:      "方法不允许",
	CodeRequestTimeout:        "请求超时",
	CodeConflict:              "资源冲突",
	CodeInternalError:         "服务器内部错误",
	CodeNotImplemented:        "功能未实现",
	CodeServiceUnavailable:    "服务不可用",
	CodeUserNotFound:          "用户不存在",
	CodeUserAlreadyExists:     "用户已存在",
	CodeInvalidCredentials:    "用户名或密码错误",
	CodeUserDisabled:          "用户已被禁用",
	CodeInvalidToken:          "无效的令牌",
	CodeTokenExpired:          "令牌已过期",
	CodeInvalidResetCode:      "验证码错误",
	CodeResetCodeExpired:      "验证码已过期",
	CodeResetCodeSent:         "验证码已发送",
	CodeEmailNotVerified:      "邮箱未验证",
	CodeEmailNotExists:        "邮箱不存在",
	CodeResetTokenInvalid:     "重置令牌无效",
	CodeInvalidParam:          "参数错误",
	CodeMissingParam:          "缺少必要参数",
	CodeParamFormatError:      "参数格式错误",
	CodeResourceNotFound:      "资源不存在",
	CodeResourceAlreadyExists: "资源已存在",
	CodeResourceLocked:        "资源已被锁定",

	// LLM 错误消息
	CodeLLMInternalError:   "LLM 内部错误",
	CodeLLMConfigError:     "LLM 配置错误",
	CodeLLMRequestFailed:   "LLM 请求失败",
	CodeLLMResponseInvalid: "LLM 响应无效",
	CodeLLMTimeout:         "LLM 请求超时",
	CodeLLMTokenLimit:      "超出 token 限制",

	// 模型提供商错误消息
	CodeModelProviderNotFound:      "模型提供商不存在",
	CodeModelProviderAlreadyExists: "模型提供商已存在",
	CodeModelProviderDisabled:      "模型提供商已禁用",
	CodeModelProviderAPIKeyMissing: "API Key 未配置",

	// 会话错误消息
	CodeConversationNotFound:      "会话不存在",
	CodeConversationAccessDenied:  "无权访问会话",
	CodeConversationStatusInvalid: "会话状态无效",

	// 消息错误消息
	CodeMessageNotFound:     "消息不存在",
	CodeMessageAccessDenied: "无权访问消息",
	CodeMessageRoleInvalid:  "消息角色无效",
}

// GetMessage 获取错误码对应的文本消息
func GetMessage(code int) string {
	if msg, ok := codeMessages[code]; ok {
		return msg
	}
	return "未知错误"
}
