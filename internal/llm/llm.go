package llm

import (
	"context"
	"github.com/cloudwego/eino/schema"
	"time"

	pkgerrors "Qavor/pkg/errors"
	einoOpenAI "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
)

// Config LLM 配置
type Config struct {
	// Model 模型名称
	Model string `mapstructure:"model"`

	// APIKey API 密钥
	APIKey string `mapstructure:"api_key"`

	// BaseURL API 基础地址 (可选)
	BaseURL string `mapstructure:"base_url"`

	// Timeout 请求超时时间 (可选)
	Timeout time.Duration `mapstructure:"timeout"`

	// Temperature 温度参数 (0-2)
	Temperature *float32 `mapstructure:"temperature"`

	// MaxCompletionTokens 最大 token 数（包含推理 token）
	MaxCompletionTokens *int `mapstructure:"max_completion_tokens"`

	// TopP Top P 采样
	TopP *float32 `mapstructure:"top_p"`

	// Stop 停止词
	Stop []string `mapstructure:"stop"`

	// PresencePenalty 存在惩罚 (-2.0 到 2.0)
	PresencePenalty *float32 `mapstructure:"presence_penalty"`

	// FrequencyPenalty 频率惩罚 (-2.0 到 2.0)
	FrequencyPenalty *float32 `mapstructure:"frequency_penalty"`

	// ResponseFormat 响应格式
	ResponseFormat *einoOpenAI.ChatCompletionResponseFormat `mapstructure:"response_format"`

	// Seed 随机种子（用于可重现的结果）
	Seed *int `mapstructure:"seed"`

	// User 用户标识
	User *string `mapstructure:"user"`

	// ExtraFields 额外字段（用于实验性功能）
	ExtraFields map[string]any `mapstructure:"extra_fields"`
}

// Client LLM 客户端
type Client struct {
	model  model.BaseChatModel
	config *Config
}

// NewClient 创建新的 LLM 客户端
func NewClient(ctx context.Context, cfg *Config) (*Client, error) {
	if cfg == nil {
		return nil, pkgerrors.ErrLLMConfigError
	}

	if cfg.APIKey == "" {
		return nil, pkgerrors.NewWithErr(pkgerrors.CodeLLMConfigError, "API key is required", nil)
	}

	// 转换配置到 eino-ext
	einoCfg := &einoOpenAI.ChatModelConfig{
		APIKey:              cfg.APIKey,
		Model:               cfg.Model,
		BaseURL:             cfg.BaseURL,
		Timeout:             cfg.Timeout,
		Temperature:         cfg.Temperature,
		MaxCompletionTokens: cfg.MaxCompletionTokens,
		TopP:                cfg.TopP,
		Stop:                cfg.Stop,
		PresencePenalty:     cfg.PresencePenalty,
		FrequencyPenalty:    cfg.FrequencyPenalty,
		ResponseFormat:      cfg.ResponseFormat,
		Seed:                cfg.Seed,
		User:                cfg.User,
		ExtraFields:         cfg.ExtraFields,
	}

	// 使用 eino-ext 官方 OpenAI 包
	m, err := einoOpenAI.NewChatModel(ctx, einoCfg)
	if err != nil {
		return nil, pkgerrors.NewWithErr(pkgerrors.CodeLLMRequestFailed, "failed to create OpenAI model", err)
	}

	return &Client{
		model:  m,
		config: cfg,
	}, nil
}

// Generate 同步生成回复
func (c *Client) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	if c.model == nil {
		return nil, pkgerrors.ErrLLMInternalError
	}

	return c.model.Generate(ctx, input, opts...)
}

// Stream 流式生成回复
func (c *Client) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	if c.model == nil {
		return nil, pkgerrors.ErrLLMInternalError
	}

	return c.model.Stream(ctx, input, opts...)
}

// GetModel 获取底层 eino BaseChatModel
func (c *Client) GetModel() model.BaseChatModel {
	return c.model
}

// GetConfig 获取配置
func (c *Client) GetConfig() *Config {
	return c.config
}
