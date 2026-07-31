package llm

import (
	"context"
	"fmt"
	"time"

	einoOpenAI "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// openAIClient OpenAI 客户端实现
type openAIClient struct {
	model model.BaseChatModel
}

// newOpenAIClient 创建 OpenAI 客户端
func newOpenAIClient(ctx context.Context, provider, modelName, apiKey, baseURL string, timeout int) (*openAIClient, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("API key is required for provider: %s", provider)
	}

	// DeepSeek 和 Moonshot 使用 OpenAI 兼容接口，可能需要不同的 BaseURL
	if baseURL == "" {
		switch provider {
		case "deepseek":
			baseURL = "https://api.deepseek.com"
		case "moonshot":
			baseURL = "https://api.moonshot.cn"
		}
	}

	duration := time.Duration(timeout) * time.Millisecond
	if duration == 0 {
		duration = 60 * time.Second
	}

	m, err := einoOpenAI.NewChatModel(ctx, &einoOpenAI.ChatModelConfig{
		APIKey:  apiKey,
		Model:   modelName,
		BaseURL: baseURL,
		Timeout: duration,
	})
	if err != nil {
		return nil, err
	}

	return &openAIClient{model: m}, nil
}

// Generate 同步生成回复
func (c *openAIClient) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	return c.model.Generate(ctx, input, opts...)
}

// Stream 流式生成回复
func (c *openAIClient) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return c.model.Stream(ctx, input, opts...)
}
