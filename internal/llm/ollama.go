package llm

import (
	"context"
	"time"

	einoOllama "github.com/cloudwego/eino-ext/components/model/ollama"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// ollamaClient Ollama 客户端实现
type ollamaClient struct {
	model model.BaseChatModel
}

// newOllamaClient 创建 Ollama 客户端
func newOllamaClient(ctx context.Context, modelName, baseURL string, timeout int) (*ollamaClient, error) {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}

	duration := time.Duration(timeout) * time.Millisecond
	if duration == 0 {
		duration = 60 * time.Second
	}

	m, err := einoOllama.NewChatModel(ctx, &einoOllama.ChatModelConfig{
		Model:   modelName,
		BaseURL: baseURL,
		Timeout: duration,
	})
	if err != nil {
		return nil, err
	}

	return &ollamaClient{model: m}, nil
}

// Generate 同步生成回复
func (c *ollamaClient) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	return c.model.Generate(ctx, input, opts...)
}

// Stream 流式生成回复
func (c *ollamaClient) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return c.model.Stream(ctx, input, opts...)
}
