package llm

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

// 错误分类常量。
const (
	CategoryTimeout          = "timeout"
	CategoryConnectFailed    = "connect_failed"
	CategoryInsufficientQuota = "insufficient_quota"
	CategoryAuthFailed       = "auth_failed"
	CategoryModelNotFound    = "model_not_found"
	CategoryRateLimited      = "rate_limited"
	CategoryUnknown          = "unknown"
)

// ClassifiedError 携带分类与友好提示的 LLM 调用错误。
type ClassifiedError struct {
	Category string // 错误分类（见 Category* 常量）
	Friendly string // 可直接展示给用户的中文提示
	Err      error  // 原始错误
}

// Error 实现 error 接口。
func (e *ClassifiedError) Error() string {
	if e == nil {
		return ""
	}
	if e.Err != nil {
		return e.Friendly + ": " + e.Err.Error()
	}
	return e.Friendly
}

// Unwrap 支持 errors.Is/As 解包原始错误。
func (e *ClassifiedError) Unwrap() error { return e.Err }

// httpStatusPattern 匹配错误消息中的 HTTP 状态码（如 "status code: 401"、"返回 HTTP 402"）。
// 用上下文限定避免与 URL 端口号（如 :4404 中的 404）误匹配。
var httpStatusPattern = regexp.MustCompile(`(?:status code|status|http status|http)\D{0,6}([45]\d\d)`)

// ClassifyError 识别 LLM/连接常见错误类别并返回友好提示。
// 检测顺序：超时 → 连接失败 → 余额不足 → 认证失败 → 模型不存在 → 限流 → 兜底。
// 当 err 已经是 *ClassifiedError 时直接返回自身。
func ClassifyError(err error) *ClassifiedError {
	if err == nil {
		return &ClassifiedError{Category: CategoryUnknown, Friendly: "模型调用失败"}
	}
	var classified *ClassifiedError
	if errors.As(err, &classified) {
		return classified
	}
	message := strings.ToLower(err.Error())

	switch {
	case isTimeout(err, message):
		return &ClassifiedError{Category: CategoryTimeout, Friendly: "请求超时，请检查网络连接或适当增大超时时间", Err: err}
	case containsAny(message, "no such host", "lookup", "connection refused", "dial tcp", "connection reset", "network is unreachable", "tls handshake", "tls: failed", "x509", "certificate"):
		return &ClassifiedError{Category: CategoryConnectFailed, Friendly: "无法访问该地址，请检查 Base URL 是否正确、网络是否可达", Err: err}
	case containsAny(message, "insufficient_quota", "quota", "out of credits", "insufficient balance", "no balance", "余额"):
		return &ClassifiedError{Category: CategoryInsufficientQuota, Friendly: "账户余额不足或额度已用尽，请前往服务商控制台充值或检查配额", Err: err}
	}

	if status := extractHTTPStatus(message); status > 0 {
		switch status {
		case http.StatusPaymentRequired:
			return &ClassifiedError{Category: CategoryInsufficientQuota, Friendly: "账户余额不足或额度已用尽，请前往服务商控制台充值或检查配额", Err: err}
		case http.StatusUnauthorized, http.StatusForbidden:
			return &ClassifiedError{Category: CategoryAuthFailed, Friendly: "API Key 无效或未授权，请检查 API Key 是否正确", Err: err}
		case http.StatusNotFound:
			return &ClassifiedError{Category: CategoryModelNotFound, Friendly: "模型不存在，请检查模型名称是否正确", Err: err}
		case http.StatusTooManyRequests:
			return &ClassifiedError{Category: CategoryRateLimited, Friendly: "请求过于频繁被限流，请稍后重试", Err: err}
		}
	}

	if containsAny(message, "invalid api key", "invalid_api_key", "authentication", "unauthorized", "forbidden", "permission denied") {
		return &ClassifiedError{Category: CategoryAuthFailed, Friendly: "API Key 无效或未授权，请检查 API Key 是否正确", Err: err}
	}
	if containsAny(message, "model not found", "model_not_found", "no such model", "does not exist") {
		return &ClassifiedError{Category: CategoryModelNotFound, Friendly: "模型不存在，请检查模型名称是否正确", Err: err}
	}
	if containsAny(message, "rate limit", "rate_limit", "too many requests", "throttl") {
		return &ClassifiedError{Category: CategoryRateLimited, Friendly: "请求过于频繁被限流，请稍后重试", Err: err}
	}
	return &ClassifiedError{Category: CategoryUnknown, Friendly: "模型调用失败", Err: err}
}

// isTimeout 判断错误是否为超时（优先 context 状态，其次错误文本）。
func isTimeout(err error, message string) bool {
	return errors.Is(err, context.DeadlineExceeded) ||
		containsAny(message, "context deadline exceeded", "deadline exceeded", "i/o timeout", "timed out", "timeout")
}

// extractHTTPStatus 从错误消息中提取 HTTP 状态码，未匹配时返回 0。
func extractHTTPStatus(message string) int {
	m := httpStatusPattern.FindStringSubmatch(message)
	if len(m) != 2 {
		return 0
	}
	status, _ := strconv.Atoi(m[1])
	return status
}

// containsAny 检查消息是否包含任一关键词（调用方需确保 message 已小写化）。
func containsAny(message string, keywords ...string) bool {
	for _, keyword := range keywords {
		if strings.Contains(message, keyword) {
			return true
		}
	}
	return false
}
