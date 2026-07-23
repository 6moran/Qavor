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
}

// GetMessage 获取错误码对应的文本消息
func GetMessage(code int) string {
	if msg, ok := codeMessages[code]; ok {
		return msg
	}
	return "未知错误"
}
