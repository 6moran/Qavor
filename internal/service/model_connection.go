package service

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"Qavor/internal/embedding"
	"Qavor/internal/llm"
	"Qavor/internal/model/dto/request"
	dto "Qavor/internal/model/dto/response"
	"Qavor/internal/rag"
	qerrors "Qavor/pkg/errors"

	"github.com/cloudwego/eino/schema"
)

const defaultConnectionTestTimeoutMS = 60000

// TestConnection 测试未保存的模型配置是否能正常连接。
// 不会持久化任何变更，仅执行一次最小化的真实请求。
func (s *modelService) TestConnection(ctx context.Context, req *request.ModelConnectionTestRequest) (*dto.ModelConnectionTestResponse, error) {
	apiKey, err := s.resolveConnectionTestKey(req)
	if err != nil {
		return nil, err
	}

	timeout := req.Timeout
	if timeout <= 0 {
		timeout = defaultConnectionTestTimeoutMS
	}

	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Millisecond)
	defer cancel()

	start := time.Now()
	var result *dto.ModelConnectionTestResponse
	switch req.ModelType {
	case "embedding":
		result, err = s.testEmbeddingConnection(ctx, req, apiKey)
	case "chat":
		result, err = s.testChatConnection(ctx, req, apiKey)
	case "rerank":
		result, err = s.testRerankConnection(ctx, req, apiKey)
	default:
		return nil, qerrors.New(qerrors.CodeInvalidParam, "不支持的模型类型: "+req.ModelType)
	}
	latency := time.Since(start).Milliseconds()
	if latency < 1 {
		latency = 1
	}

	if err != nil {
		_, friendly := classifyConnectionTestError(err)
		return nil, qerrors.NewWithDetail(qerrors.CodeLLMRequestFailed, friendly, redactConnectionTestError(err, apiKey))
	}

	result.LatencyMS = latency
	result.ModelType = req.ModelType
	result.Connected = true
	return result, nil
}

// testRerankConnection 执行一次最小化的重排请求并校验结果。
// 与运行时 ResolveReranker 保持一致：复用自定义请求头与 Bearer API Key。
func (s *modelService) testRerankConnection(ctx context.Context, req *request.ModelConnectionTestRequest, apiKey string) (*dto.ModelConnectionTestResponse, error) {
	client, err := rag.NewHTTPReranker(rag.HTTPRerankerConfig{
		Model:   req.Name,
		BaseURL: req.BaseURL,
		APIKey:  apiKey,
		Headers: req.Headers,
		Timeout: time.Duration(req.Timeout) * time.Millisecond,
	})
	if err != nil {
		return nil, err
	}
	results, err := client.Rerank(ctx, "连接测试", []rag.RerankDocument{
		{ID: "relevant", Content: "相关内容"},
		{ID: "irrelevant", Content: "无关内容"},
	}, 1)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, qerrors.New(qerrors.CodeLLMResponseInvalid, "模型未返回有效重排结果")
	}
	return &dto.ModelConnectionTestResponse{ModelType: req.ModelType}, nil
}

// resolveConnectionTestKey 解析测试连接使用的 API Key。
// 优先使用请求中携带的 API Key，否则回退到已保存模型的解密 Key。
func (s *modelService) resolveConnectionTestKey(req *request.ModelConnectionTestRequest) (string, error) {
	if strings.TrimSpace(req.APIKey) != "" {
		return req.APIKey, nil
	}
	if req.ModelID == 0 {
		return "", qerrors.New(qerrors.CodeInvalidParam, "请填写 API Key")
	}
	saved, err := s.GetModelWithDecryptedKey(req.ModelID)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(saved.APIKey) == "" {
		return "", qerrors.New(qerrors.CodeInvalidParam, "已保存模型没有可用的 API Key")
	}
	return saved.APIKey, nil
}

// 连接测试错误分类标识
const (
	connectionTestTimeout           = "timeout"
	connectionTestConnectFailed     = "connection_failed"
	connectionTestInsufficientQuota = "insufficient_quota"
	connectionTestAuthFailed        = "auth_failed"
	connectionTestModelNotFound     = "model_not_found"
	connectionTestRateLimited       = "rate_limited"
	connectionTestUnknown           = "unknown"
)

// httpStatusPattern 匹配错误消息中的 HTTP 状态码（如 "status code: 401"、"返回 HTTP 402"）。
// 用上下文限定避免与 URL 端口号（如 :4404 中的 404）误匹配。
var httpStatusPattern = regexp.MustCompile(`(?:status code|status|http status|http)\D{0,6}([45]\d\d)`)

// classifyConnectionTestError 识别连接测试常见错误类别并返回友好提示。
// 检测顺序：超时 → 连接失败 → 余额不足 → 认证失败 → 模型不存在 → 限流 → 兜底。
func classifyConnectionTestError(err error) (category, friendly string) {
	if err == nil {
		return connectionTestUnknown, "连接测试失败"
	}
	message := strings.ToLower(err.Error())

	switch {
	case isConnectionTestTimeout(err, message):
		return connectionTestTimeout, "请求超时，请检查网络连接或适当增大超时时间"
	case containsAny(message, "no such host", "lookup", "connection refused", "dial tcp", "connection reset", "network is unreachable", "tls handshake", "tls: failed", "x509", "certificate"):
		return connectionTestConnectFailed, "无法访问该地址，请检查 Base URL 是否正确、网络是否可达"
	case containsAny(message, "insufficient_quota", "quota", "out of credits", "insufficient balance", "no balance", "余额"):
		return connectionTestInsufficientQuota, "账户余额不足或额度已用尽，请前往服务商控制台充值或检查配额"
	}

	if status := extractHTTPStatus(message); status > 0 {
		switch status {
		case http.StatusPaymentRequired:
			return connectionTestInsufficientQuota, "账户余额不足或额度已用尽，请前往服务商控制台充值或检查配额"
		case http.StatusUnauthorized, http.StatusForbidden:
			return connectionTestAuthFailed, "API Key 无效或未授权，请检查 API Key 是否正确"
		case http.StatusNotFound:
			return connectionTestModelNotFound, "模型不存在，请检查模型名称是否正确"
		case http.StatusTooManyRequests:
			return connectionTestRateLimited, "请求过于频繁被限流，请稍后重试"
		}
	}

	if containsAny(message, "invalid api key", "invalid_api_key", "authentication", "unauthorized", "forbidden", "permission denied") {
		return connectionTestAuthFailed, "API Key 无效或未授权，请检查 API Key 是否正确"
	}
	if containsAny(message, "model not found", "model_not_found", "no such model", "does not exist") {
		return connectionTestModelNotFound, "模型不存在，请检查模型名称是否正确"
	}
	if containsAny(message, "rate limit", "rate_limit", "too many requests", "throttl") {
		return connectionTestRateLimited, "请求过于频繁被限流，请稍后重试"
	}
	return connectionTestUnknown, "连接测试失败"
}

// isConnectionTestTimeout 判断错误是否为超时（优先 context 状态，其次错误文本）。
func isConnectionTestTimeout(err error, message string) bool {
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

// redactConnectionTestError 从错误信息中移除敏感数据（如 API Key）。
func redactConnectionTestError(err error, secrets ...string) string {
	if err == nil {
		return ""
	}
	message := err.Error()
	for _, secret := range secrets {
		if secret != "" {
			message = strings.ReplaceAll(message, secret, "[REDACTED]")
		}
	}
	return message
}

// testEmbeddingConnection 执行一次最小化的 embedding 请求并构造响应。
func (s *modelService) testEmbeddingConnection(ctx context.Context, req *request.ModelConnectionTestRequest, apiKey string) (*dto.ModelConnectionTestResponse, error) {
	client, err := embedding.NewOpenAIClient(ctx, apiKey, req.Name, req.BaseURL, req.Timeout)
	if err != nil {
		return nil, err
	}
	vectors, err := client.EmbedStrings(ctx, []string{"Qavor model connection test"})
	if err != nil {
		return nil, err
	}
	if len(vectors) == 0 {
		return nil, qerrors.New(qerrors.CodeLLMResponseInvalid, "模型未返回任何向量")
	}
	dimension := len(vectors[0])
	if dimension == 0 {
		return nil, qerrors.New(qerrors.CodeLLMResponseInvalid, "模型返回了空向量")
	}
	return &dto.ModelConnectionTestResponse{
		ModelType: req.ModelType,
		Dimension: dimension,
	}, nil
}

// testChatConnection 执行一次最小化的 chat 请求并构造响应。
func (s *modelService) testChatConnection(ctx context.Context, req *request.ModelConnectionTestRequest, apiKey string) (*dto.ModelConnectionTestResponse, error) {
	client, err := llm.NewClient(ctx, req.Protocol, req.Name, apiKey, req.BaseURL, req.Timeout)
	if err != nil {
		return nil, err
	}
	reply, err := client.Generate(ctx, []*schema.Message{{
		Role:    schema.User,
		Content: "Reply with OK.",
	}})
	if err != nil {
		return nil, err
	}
	if reply == nil || strings.TrimSpace(reply.Content) == "" {
		return nil, qerrors.New(qerrors.CodeLLMResponseInvalid, "模型未返回有效回复")
	}
	return &dto.ModelConnectionTestResponse{
		ModelType: req.ModelType,
	}, nil
}
