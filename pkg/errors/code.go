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

	// SSE 流式服务错误 6xxx
	CodeSSEInvalidRequest      = 6001 // 请求参数无效
	CodeSSETooManyRequests     = 6002 // 请求过多（并发限制）
	CodeSSEContextBuildFailed  = 6003 // 上下文构建失败
	CodeSSELLMInitFailed       = 6004 // LLM初始化失败
	CodeSSELLMStreamFailed     = 6005 // LLM流式调用失败
	CodeSSEStreamReadFailed    = 6006 // 流式读取失败
	CodeSSEPersistFailed       = 6007 // 持久化失败
	CodeSSETaskNotFound        = 6008 // 任务不存在
	CodeSSETaskCancelled       = 6009 // 任务已取消
	CodeSSETimeout             = 6010 // 超时
	CodeSSEFileTooLarge        = 6011 // 文件大小超限
	CodeSSEUnsupportedFileType = 6012 // 不支持的文件类型
	CodeSSEFileProcessFailed   = 6013 // 文件处理失败
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

	// SSE 流式服务错误消息
	CodeSSEInvalidRequest:      "请求参数无效",
	CodeSSETooManyRequests:     "请求过多，请稍后再试",
	CodeSSEContextBuildFailed:  "构建上下文失败",
	CodeSSELLMInitFailed:       "LLM初始化失败",
	CodeSSELLMStreamFailed:     "LLM流式调用失败",
	CodeSSEStreamReadFailed:    "读取流式输出失败",
	CodeSSEPersistFailed:       "持久化失败",
	CodeSSETaskNotFound:        "任务不存在或已完成",
	CodeSSETaskCancelled:       "任务已取消",
	CodeSSETimeout:             "请求超时",
	CodeSSEFileTooLarge:        "文件大小超过限制",
	CodeSSEUnsupportedFileType: "不支持的文件类型",
	CodeSSEFileProcessFailed:   "文件处理失败",
}

// GetMessage 获取错误码对应的文本消息
func GetMessage(code int) string {
	if msg, ok := codeMessages[code]; ok {
		return msg
	}
	return "未知错误"
}
