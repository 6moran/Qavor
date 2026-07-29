package llm

import (
	"context"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// TimeoutConfig 超时配置，定义同步生成和流式生成的超时时间
type TimeoutConfig struct {
	// GenerateTimeout 同步生成（Generate）的超时时间
	GenerateTimeout time.Duration
	// StreamTimeout 流式生成（Stream）的超时时间，
	// 通常设置得更长，因为流式响应会持续较长时间
	StreamTimeout time.Duration
}

// DefaultTimeoutConfig 返回默认的超时配置
// 同步生成默认 60 秒，流式生成默认 120 秒
func DefaultTimeoutConfig() *TimeoutConfig {
	return &TimeoutConfig{
		GenerateTimeout: 60 * time.Second,
		StreamTimeout:   120 * time.Second,
	}
}

// TimeoutClient 支持超时控制的 LLM 客户端，包装了基础 Client
// 通过在 context 上设置 deadline 来实现超时控制
type TimeoutClient struct {
	*Client                      // 内嵌基础客户端
	timeoutConfig *TimeoutConfig // 超时配置
}

// NewTimeoutClient 创建支持超时控制的客户端
// 如果 timeoutConfig 为 nil，则使用默认超时配置

func NewTimeoutClient(client *Client, timeoutConfig *TimeoutConfig) *TimeoutClient {
	if timeoutConfig == nil {
		timeoutConfig = DefaultTimeoutConfig()
	}
	return &TimeoutClient{
		Client:        client,
		timeoutConfig: timeoutConfig,
	}
}

// GenerateWithTimeout 带超时控制的同步生成
// 会在原始 context 上叠加超时 deadline，超时后调用会返回 context.DeadlineExceeded 错误
func (c *TimeoutClient) GenerateWithTimeout(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	// 创建带有超时的子 context
	ctx, cancel := context.WithTimeout(ctx, c.timeoutConfig.GenerateTimeout)
	defer cancel()

	// 调用基础客户端的 Generate 方法
	return c.Generate(ctx, input, opts...)
}

// StreamWithTimeout 带超时控制的流式生成
// 注意：此超时控制的是 Stream 调用本身（获取 StreamReader）的超时，
// 而非整个流式读取过程的超时
func (c *TimeoutClient) StreamWithTimeout(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	// 创建带有超时的子 context
	ctx, cancel := context.WithTimeout(ctx, c.timeoutConfig.StreamTimeout)
	defer cancel()

	// 调用基础客户端的 Stream 方法
	return c.Stream(ctx, input, opts...)
}
