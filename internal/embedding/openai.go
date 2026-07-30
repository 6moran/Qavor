package embedding

import (
	"context"
	"errors"
	"time"

	einoOpenAI "github.com/cloudwego/eino-ext/components/embedding/openai"
)

var (
	// ErrAPIKeyRequired API Key 必需
	ErrAPIKeyRequired = errors.New("API key is required")
)

// openaiClient OpenAI Embedding 客户端
type openaiClient struct {
	embedder *einoOpenAI.Embedder
}

// NewOpenAIClient 创建 OpenAI Embedding 客户端
func NewOpenAIClient(ctx context.Context, apiKey, model, baseURL string, timeout int) (Client, error) {
	if apiKey == "" {
		return nil, ErrAPIKeyRequired
	}

	duration := time.Duration(timeout) * time.Millisecond
	if duration == 0 {
		duration = 60 * time.Second
	}

	embedder, err := einoOpenAI.NewEmbedder(ctx, &einoOpenAI.EmbeddingConfig{
		APIKey:  apiKey,
		Model:   model,
		BaseURL: baseURL,
		Timeout: duration,
	})
	if err != nil {
		return nil, err
	}

	return &openaiClient{embedder: embedder}, nil
}

// EmbedStrings 实现 Client 接口
func (c *openaiClient) EmbedStrings(ctx context.Context, input []string) ([][]float64, error) {
	return c.embedder.EmbedStrings(ctx, input)
}
