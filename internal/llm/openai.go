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
	model model.ToolCallingChatModel
}

// GetToolCallingModel 返回支持 Tool Calling 的模型
func (c *openAIClient) GetToolCallingModel() model.ToolCallingChatModel {
	return c.model
}

// openaiClientCache OpenAI 客户端缓存
// key 格式: "modelName:apiKey:baseURL"
var (
	openaiClientCache sync.Map
)

// ClearOpenAICache 清除 OpenAI 客户端缓存
func ClearOpenAICache() {
	openaiClientCache.Range(func(key, value interface{}) bool {
		openaiClientCache.Delete(key)
		return true
	})
}

// ClearOpenAICacheByKey 根据 key 清除特定的 OpenAI 客户端缓存
func ClearOpenAICacheByKey(modelName, apiKey, baseURL string) {
	key := openaiCacheKey(modelName, apiKey, baseURL)
	openaiClientCache.Delete(key)
}

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

	// OpenAI 兼容接口的供应商默认 BaseURL 配置
	if baseURL == "" {
		switch provider {
		case "openai":
			baseURL = "https://api.openai.com/v1"
		case "deepseek":
			baseURL = "https://api.deepseek.com"
		case "moonshot":
			baseURL = "https://api.moonshot.cn"
		case "zhipu":
			baseURL = "https://open.bigmodel.cn/api/paas/v4"
		case "alibaba":
			baseURL = "https://dashscope.aliyuncs.com/compatible-mode/v1"
		case "tencent":
			baseURL = "https://api.hunyuan.cloud.tencent.com/v1"
		case "minimax":
			baseURL = "https://api.minimax.chat/v1"
		case "groq":
			baseURL = "https://api.groq.com/openai/v1"
		case "siliconflow":
			baseURL = "https://api.siliconflow.cn/v1"
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
