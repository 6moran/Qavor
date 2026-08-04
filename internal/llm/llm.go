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

// ClientFactory 客户端工厂函数类型
type ClientFactory func(ctx context.Context, provider, model, apiKey, baseURL string, timeout int) (Client, error)

// providerRegistry provider 注册表，存储不同 provider 的工厂函数
var providerRegistry = map[string]ClientFactory{}

// RegisterProvider 注册 provider 工厂函数
func RegisterProvider(provider string, factory ClientFactory) {
	providerRegistry[provider] = factory
}

// GetProvider 获取 provider 的工厂函数
func GetProvider(provider string) (ClientFactory, bool) {
	factory, ok := providerRegistry[provider]
	return factory, ok
}

func init() {
	// 注册所有支持的 provider
	// OpenAI 兼容协议
	RegisterProvider("openai", newOpenAIClient)
	RegisterProvider("deepseek", newOpenAIClient)
	RegisterProvider("moonshot", newOpenAIClient)
	RegisterProvider("zhipu", newOpenAIClient)
	RegisterProvider("alibaba", newOpenAIClient)
	RegisterProvider("tencent", newOpenAIClient)
	RegisterProvider("minimax", newOpenAIClient)
	RegisterProvider("groq", newOpenAIClient)
	RegisterProvider("siliconflow", newOpenAIClient)

	// Ollama 本地部署
	RegisterProvider("ollama", newOllamaClient)
}

// NewClient 根据 provider 类型创建对应的 LLM 客户端
// 缓存由各 provider 实现内部管理（openai.go / ollama.go）
func NewClient(ctx context.Context, provider, model, apiKey, baseURL string, timeout int) (Client, error) {
	// 默认使用 openai
	if provider == "" {
		provider = "openai"
	}

	factory, ok := GetProvider(provider)
	if !ok {
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}

	return factory(ctx, provider, model, apiKey, baseURL, timeout)
}
