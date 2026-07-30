package llm

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// Client 统一的 LLM 客户端接口
// 所有厂商实现都必须满足此接口
type Client interface {
	// Generate 同步生成回复
	Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error)
	// Stream 流式生成回复
	Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error)
}

// NewClient 根据 provider 类型创建对应的 LLM 客户端
func NewClient(ctx context.Context, provider, model, apiKey, baseURL string, timeout int) (Client, error) {
	switch provider {
	case "openai", "deepseek", "moonshot", "": // 默认使用 OpenAI
		return newOpenAIClient(ctx, provider, model, apiKey, baseURL, timeout)
	case "ollama":
		return newOllamaClient(ctx, model, baseURL, timeout)
	default:
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}
}
