package service

import (
	"context"
	"strings"
	"time"

	"Qavor/internal/embedding"
	"Qavor/internal/llm"
	"Qavor/internal/model/dto/request"
	dto "Qavor/internal/model/dto/response"
	"Qavor/internal/rag"
	"Qavor/pkg/errors"

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
		return nil, errors.New(errors.CodeInvalidParam, "不支持的模型类型: "+req.ModelType)
	}
	latency := time.Since(start).Milliseconds()
	if latency < 1 {
		latency = 1
	}

	if err != nil {
		return nil, errors.New(errors.CodeLLMRequestFailed,
			"连接测试失败: "+redactConnectionTestError(err, apiKey))
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
		return nil, errors.New(errors.CodeLLMResponseInvalid, "模型未返回有效重排结果")
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
		return "", errors.New(errors.CodeInvalidParam, "请填写 API Key")
	}
	saved, err := s.GetModelWithDecryptedKey(req.ModelID)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(saved.APIKey) == "" {
		return "", errors.New(errors.CodeInvalidParam, "已保存模型没有可用的 API Key")
	}
	return saved.APIKey, nil
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
		return nil, errors.New(errors.CodeLLMResponseInvalid, "模型未返回任何向量")
	}
	dimension := len(vectors[0])
	if dimension == 0 {
		return nil, errors.New(errors.CodeLLMResponseInvalid, "模型返回了空向量")
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
		return nil, errors.New(errors.CodeLLMResponseInvalid, "模型未返回有效回复")
	}
	return &dto.ModelConnectionTestResponse{
		ModelType: req.ModelType,
	}, nil
}
