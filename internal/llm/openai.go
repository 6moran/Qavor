package llm

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	einoOpenAI "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// openAIClient OpenAI 客户端实现
type openAIClient struct {
	model model.BaseChatModel
}

// openaiClientCache OpenAI 客户端缓存
// key 格式: "modelName:apiKey:baseURL"
var (
	openaiClientCache sync.Map
)

// openaiCacheKey 生成缓存 key
func openaiCacheKey(modelName, apiKey, baseURL string) string {
	return modelName + ":" + apiKey + ":" + baseURL
}

// newOpenAIClient 创建 OpenAI 客户端
// 同一 apiKey + model + baseURL 组合会复用已有的客户端
func newOpenAIClient(ctx context.Context, provider, modelName, apiKey, baseURL string, timeout int) (Client, error) {
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

	// 检查缓存，命中则直接返回
	key := openaiCacheKey(modelName, apiKey, baseURL)
	if cached, ok := openaiClientCache.Load(key); ok {
		return cached.(Client), nil
	}

	// 缓存未命中，创建新客户端
	duration := time.Duration(timeout) * time.Millisecond
	if duration == 0 {
		duration = 60 * time.Second
	}

	// 在 ChatModelConfig 内部直接配置 HTTP 客户端连接池
	m, err := einoOpenAI.NewChatModel(ctx, &einoOpenAI.ChatModelConfig{
		APIKey:  apiKey,
		Model:   modelName,
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout: duration,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				MaxConnsPerHost:     20,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	})
	if err != nil {
		return nil, err
	}

	client := &openAIClient{model: m}

	// 存入缓存
	actual, _ := openaiClientCache.LoadOrStore(key, client)
	return actual.(Client), nil
}

// Generate 同步生成回复
func (c *openAIClient) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	return c.model.Generate(ctx, input, opts...)
}

// Stream 流式生成回复
func (c *openAIClient) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return c.model.Stream(ctx, input, opts...)
}
