package llm

import (
	"context"
	"errors"
	"math"
	"math/rand"
	"net"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	pkgerrors "Qavor/pkg/errors"
)

// RetryConfig 重试配置，控制重试行为的各项参数
type RetryConfig struct {
	// MaxRetries 最大重试次数（不含首次请求），默认 3 次
	MaxRetries int
	// InitialBackoff 初始退避时间，默认 1 秒
	InitialBackoff time.Duration
	// MaxBackoff 最大退避时间上限，默认 30 秒
	MaxBackoff time.Duration
	// BackoffFactor 退避因子，每次重试退避时间按此因子倍增，默认 2.0
	BackoffFactor float64
}

// DefaultRetryConfig 返回默认的重试配置
// 配置策略：最多重试 3 次，初始退避 1 秒，最大退避 30 秒，退避因子 2.0
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxRetries:     3,
		InitialBackoff: 1 * time.Second,
		MaxBackoff:     30 * time.Second,
		BackoffFactor:  2.0,
	}
}

// RetryableClient 支持自动重试的 LLM 客户端包装器
// 在原始 Client 基础上增加了重试逻辑，支持可配置的退避策略
type RetryableClient struct {
	*Client                           // 嵌入原始 LLM 客户端
	retryConfig  *RetryConfig         // 重试配置
}

// NewRetryableClient 创建支持重试的客户端
// 参数:
//   - client: 原始 LLM 客户端实例
//   - retryConfig: 重试配置，为 nil 时使用默认配置
//
// 返回: 包装后的支持重试的客户端
func NewRetryableClient(client *Client, retryConfig *RetryConfig) *RetryableClient {
	if retryConfig == nil {
		retryConfig = DefaultRetryConfig()
	}
	return &RetryableClient{
		Client:      client,
		retryConfig: retryConfig,
	}
}

// GenerateWithRetry 带重试的同步生成
// 在调用 LLM 生成接口时自动重试，支持指数退避和随机抖动
// 参数:
//   - ctx: 上下文，用于控制超时和取消
//   - input: 输入消息列表
//   - opts: 可选的模型选项
//
// 返回: 生成的消息或错误
func (c *RetryableClient) GenerateWithRetry(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	var lastErr error

	// 循环执行重试，attempt 从 0 开始表示首次请求
	for attempt := 0; attempt <= c.retryConfig.MaxRetries; attempt++ {
		// 非首次请求时，先等待退避时间再进行重试
		if attempt > 0 {
			backoff := c.calculateBackoff(attempt)
			// 使用 select 监听上下文取消和退避计时器
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		// 调用原始客户端的生成方法
		result, err := c.Generate(ctx, input, opts...)
		if err == nil {
			return result, nil
		}

		lastErr = err

		// 判断错误是否可重试，不可重试则直接返回错误
		if !c.isRetryableError(err) {
			return nil, err
		}
	}

	// 所有重试次数用尽后，返回最大重试次数超过的错误
	return nil, pkgerrors.NewWithErr(pkgerrors.CodeLLMRequestFailed,
		"max retries exceeded", lastErr)
}

// StreamWithRetry 带重试的流式生成
// 在调用 LLM 流式接口时自动重试，逻辑与 GenerateWithRetry 类似
// 参数:
//   - ctx: 上下文，用于控制超时和取消
//   - input: 输入消息列表
//   - opts: 可选的模型选项
//
// 返回: 流式消息读取器或错误
func (c *RetryableClient) StreamWithRetry(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	var lastErr error

	// 循环执行重试
	for attempt := 0; attempt <= c.retryConfig.MaxRetries; attempt++ {
		// 非首次请求时执行退避等待
		if attempt > 0 {
			backoff := c.calculateBackoff(attempt)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		// 调用原始客户端的流式方法
		result, err := c.Stream(ctx, input, opts...)
		if err == nil {
			return result, nil
		}

		lastErr = err

		// 不可重试的错误直接返回
		if !c.isRetryableError(err) {
			return nil, err
		}
	}

	// 重试次数耗尽
	return nil, pkgerrors.NewWithErr(pkgerrors.CodeLLMRequestFailed,
		"max retries exceeded", lastErr)
}

// calculateBackoff 计算指定重试次数的退避时间
// 使用指数退避算法：退避时间 = 初始退避 * 退避因子 ^ (重试次数 - 1)
// 并添加 10% 以内的随机抖动以避免惊群效应
// 参数:
//   - attempt: 当前重试次数（从 1 开始）
//
// 返回: 计算后的退避持续时间
func (c *RetryableClient) calculateBackoff(attempt int) time.Duration {
	// 计算指数退避值
	backoff := float64(c.retryConfig.InitialBackoff) * math.Pow(c.retryConfig.BackoffFactor, float64(attempt-1))

	// 添加 10% 以内的随机抖动，防止多个客户端同时重试造成服务端压力
	jitter := rand.Float64() * 0.1 * backoff
	backoff += jitter

	// 限制退避时间不超过配置的最大值
	if time.Duration(backoff) > c.retryConfig.MaxBackoff {
		backoff = float64(c.retryConfig.MaxBackoff)
	}

	return time.Duration(backoff)
}

// isRetryableError 判断给定的错误是否应该进行重试
// 可重试的错误包括：
//   - 网络超时错误（net.Error 且 Timeout() 返回 true）
//   - 网络连接被重置（ECONNRESET）
//   - 连接被对端关闭（ECONNREFUSED）
//   - LLM 请求失败（如限流 429、服务端 5xx 错误等）
//
// 不可重试的错误包括：
//   - 认证错误（401）
//   - 参数错误（400）
//   - 配置错误
//   - Token 限制等
func (c *RetryableClient) isRetryableError(err error) bool {
	// 检查网络超时错误：实现 net.Error 接口且 Timeout() 返回 true 的错误
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}

	// 检查网络连接相关错误（如连接被重置、拒绝等）
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return true
	}

	// 检查业务错误码，判断是否为可重试的 LLM 错误
	var bizErr *pkgerrors.BizError
	if errors.As(err, &bizErr) {
		switch bizErr.Code {
		// LLM 请求失败（包含限流 429、服务端 5xx 等场景），可重试
		case pkgerrors.CodeLLMRequestFailed:
			return true
		// LLM 超时错误，可重试
		case pkgerrors.CodeLLMTimeout:
			return true
		// 服务不可用错误，可重试
		case pkgerrors.CodeServiceUnavailable:
			return true
		// 请求超时错误，可重试
		case pkgerrors.CodeRequestTimeout:
			return true
		// 以下错误码不可重试，直接返回 false
		// CodeLLMConfigError（配置错误）、CodeLLMResponseInvalid（响应无效）、
		// CodeLLMTokenLimit（Token 超限）等
		default:
			return false
		}
	}

	// 默认情况下不可重试，避免误重试不可恢复的错误
	return false
}
