package embedding

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"sync"
	"time"

	einoArk "github.com/cloudwego/eino-ext/components/embedding/ark"
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

type arkClient struct {
	embedder *einoArk.Embedder
}

type arkEndpointClient struct {
	mu         sync.Mutex
	standard   Client
	multimodal Client
	useMulti   bool
}

func newArkEndpointClient(standard, multimodal Client) Client {
	return &arkEndpointClient{standard: standard, multimodal: multimodal}
}

func shouldTryArkMultimodal(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return (strings.Contains(message, "does not support") && strings.Contains(message, "api")) ||
		(strings.Contains(message, "not support") && strings.Contains(message, "api")) ||
		strings.Contains(message, "500 internal server error")
}

func (c *arkEndpointClient) EmbedStrings(ctx context.Context, input []string) ([][]float64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.useMulti {
		return c.multimodal.EmbedStrings(ctx, input)
	}
	vectors, err := c.standard.EmbedStrings(ctx, input)
	if !shouldTryArkMultimodal(err) {
		return vectors, err
	}
	vectors, err = c.multimodal.EmbedStrings(ctx, input)
	if err == nil {
		c.useMulti = true
	}
	return vectors, err
}

func isArkMultimodalModel(model string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(model)), "doubao-embedding-vision")
}

func isArkMultimodalRequest(model, baseURL string) bool {
	baseURL = strings.TrimRight(strings.ToLower(strings.TrimSpace(baseURL)), "/")
	return isArkMultimodalModel(model) || strings.HasSuffix(baseURL, "/embeddings/multimodal")
}

func isArkEndpointID(model, baseURL string) bool {
	if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(model)), "ep-") {
		return false
	}
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return false
	}
	return strings.EqualFold(parsed.Hostname(), "ark.cn-beijing.volces.com") &&
		strings.HasPrefix(strings.TrimRight(parsed.Path, "/"), "/api/v3")
}

func normalizeArkMultimodalBaseURL(baseURL string) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	baseURL = strings.TrimSuffix(baseURL, "/embeddings/multimodal")
	baseURL = strings.TrimSuffix(baseURL, "/embeddings")
	return baseURL
}

func arkMultimodalConcurrency() int {
	return 1
}

func newArkMultimodalClient(ctx context.Context, apiKey, model, baseURL string, duration time.Duration) (Client, error) {
	apiType := einoArk.APITypeMultiModal
	maxConcurrentRequests := arkMultimodalConcurrency()
	embedder, err := einoArk.NewEmbedder(ctx, &einoArk.EmbeddingConfig{
		APIKey:                apiKey,
		Model:                 model,
		BaseURL:               normalizeArkMultimodalBaseURL(baseURL),
		Timeout:               &duration,
		APIType:               &apiType,
		MaxConcurrentRequests: &maxConcurrentRequests,
	})
	if err != nil {
		return nil, err
	}
	return &arkClient{embedder: embedder}, nil
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

	if isArkMultimodalRequest(model, baseURL) {
		return newArkMultimodalClient(ctx, apiKey, model, baseURL, duration)
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
	standard := &openaiClient{embedder: embedder}
	if isArkEndpointID(model, baseURL) {
		multimodal, err := newArkMultimodalClient(ctx, apiKey, model, baseURL, duration)
		if err != nil {
			return nil, err
		}
		return newArkEndpointClient(standard, multimodal), nil
	}

	return standard, nil
}

// EmbedStrings 实现 Client 接口
func (c *openaiClient) EmbedStrings(ctx context.Context, input []string) ([][]float64, error) {
	return c.embedder.EmbedStrings(ctx, input)
}

func (c *arkClient) EmbedStrings(ctx context.Context, input []string) ([][]float64, error) {
	return c.embedder.EmbedStrings(ctx, input)
}
