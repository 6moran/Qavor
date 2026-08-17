package context

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

// Summarizer 摘要生成器接口
type Summarizer interface {
	// Generate 生成摘要
	Generate(ctx context.Context, messages []*schema.Message) (string, error)
}

// LLMSummarizer LLM 摘要生成器
type LLMSummarizer struct {
	logger *zap.Logger
	client LLMClient
}

// LLMClient LLM 客户端接口
type LLMClient interface {
	Generate(ctx context.Context, input []*schema.Message) (*schema.Message, error)
}

// NewLLMSummarizer 创建 LLM 摘要生成器
func NewLLMSummarizer(logger *zap.Logger, client LLMClient) *LLMSummarizer {
	return &LLMSummarizer{
		logger: logger,
		client: client,
	}
}

// Generate 生成摘要
func (s *LLMSummarizer) Generate(ctx context.Context, messages []*schema.Message) (string, error) {
	if len(messages) == 0 {
		return "", nil
	}

	// 构建摘要 prompt
	prompt := s.buildSummaryPrompt(messages)

	// 调用 LLM
	resp, err := s.client.Generate(ctx, prompt)
	if err != nil {
		return "", fmt.Errorf("LLM 生成摘要失败: %w", err)
	}

	summary := strings.TrimSpace(resp.Content)
	if summary == "" {
		return "", fmt.Errorf("LLM 返回空摘要")
	}

	return summary, nil
}

// buildSummaryPrompt 构建摘要生成的 prompt
func (s *LLMSummarizer) buildSummaryPrompt(messages []*schema.Message) []*schema.Message {
	systemPrompt := `你是一个对话摘要助手。请根据提供的对话历史，生成一段简洁的摘要。
要求：
1. 保留关键信息：用户意图、讨论主题、重要结论、提及的实体（人名、地点、数字等）
2. 摘要长度控制在 300 字以内
3. 用简洁的叙述风格，不要使用列表格式
4. 保留对话的逻辑流程`

	// 构建对话内容
	var conversationParts []string
	for _, msg := range messages {
		role := "用户"
		if msg.Role == schema.Assistant {
			role = "助手"
		}
		conversationParts = append(conversationParts, fmt.Sprintf("[%s] %s", role, msg.Content))
	}
	conversation := strings.Join(conversationParts, "\n")

	userContent := fmt.Sprintf("对话内容：\n%s\n\n请生成摘要：", conversation)

	return []*schema.Message{
		{Role: schema.System, Content: systemPrompt},
		{Role: schema.User, Content: userContent},
	}
}
