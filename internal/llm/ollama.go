package llm

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	einoOllama "github.com/cloudwego/eino-ext/components/model/ollama"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// ollamaClient Ollama 客户端实现
type ollamaClient struct {
	model model.BaseChatModel
}

// ollamaClientCache Ollama 客户端缓存
// key 格式: "modelName:baseURL"
var (
	ollamaClientCache sync.Map
)

// ollamaCacheKey 生成缓存 key
func ollamaCacheKey(modelName, baseURL string) string {
	return modelName + ":" + baseURL
}

// newOllamaClient 创建 Ollama 客户端
// 同一 model + baseURL 组合会复用已有的客户端
func newOllamaClient(ctx context.Context, provider, modelName, apiKey, baseURL string, timeout int) (Client, error) {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}

	// 检查缓存，命中则直接返回
	key := ollamaCacheKey(modelName, baseURL)
	if cached, ok := ollamaClientCache.Load(key); ok {
		return cached.(Client), nil
	}

	// 缓存未命中，创建新客户端
	duration := time.Duration(timeout) * time.Millisecond
	if duration == 0 {
		duration = 120 * time.Second // Ollama 本地推理可能较慢
	}

	// 在 ChatModelConfig 内部直接配置 HTTP 客户端连接池
	m, err := einoOllama.NewChatModel(ctx, &einoOllama.ChatModelConfig{
		Model:   modelName,
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout: duration,
			Transport: &http.Transport{
				MaxIdleConns:        10,
				MaxIdleConnsPerHost: 5,
				MaxConnsPerHost:     10,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create ollama client: %w", err)
	}

	client := &ollamaClient{model: m}

	// 存入缓存
	actual, _ := ollamaClientCache.LoadOrStore(key, client)
	return actual.(Client), nil
}

// Generate 同步生成回复
func (c *ollamaClient) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	return c.model.Generate(ctx, input, opts...)
}

// Stream 流式生成回复
func (c *ollamaClient) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return c.model.Stream(ctx, input, opts...)
}
