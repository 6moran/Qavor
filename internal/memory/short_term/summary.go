package shortterm

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

// ModelResolver 模型解析接口（避免直接依赖 service.ModelService）
type ModelResolver interface {
	CreateLLMClient(ctx context.Context, modelID uint) (LLMClient, error)
}

// LLMClient LLM 客户端接口（从 llm.Client 抽象出来，方便测试）
type LLMClient interface {
	Generate(ctx context.Context, input []*schema.Message) (*schema.Message, error)
}

// SummaryGenerator 摘要生成器
// 当缓冲区达到阈值时，取前半部分消息 + 旧摘要，通过 LLM 生成新摘要
// 缓冲区保留后半部分（滑动窗口），实现长期记忆压缩
type SummaryGenerator struct {
	logger        *zap.Logger
	config        *SummaryConfig
	modelResolver ModelResolver
}

// NewSummaryGenerator 创建摘要生成器
// modelResolver 为可选参数，不传入时降级为规则式摘要；modelID 在调用时按参数传入
func NewSummaryGenerator(logger *zap.Logger, config *SummaryConfig, modelResolver ModelResolver, _ uint) *SummaryGenerator {
	if config == nil {
		config = DefaultSummaryConfig()
	}
	return &SummaryGenerator{
		logger:        logger,
		config:        config,
		modelResolver: modelResolver,
	}
}

// ShouldGenerate 判断是否需要生成摘要
// 达到阈值后，还需检查有效消息数（重要性 > 0.3）是否足够
// 避免大量寒暄消息凑数触发无意义的摘要
func (g *SummaryGenerator) ShouldGenerate(buffer *MessageBuffer) bool {
	if buffer == nil {
		return false
	}
	if len(buffer.Messages) >= g.config.MessageThreshold ||
		buffer.TotalTokens >= g.config.TokenThreshold {
		// 检查有效消息数：至少 5 条有意义消息才值得摘要
		meaningfulCount := 0
		for _, msg := range buffer.Messages {
			if msg.Importance > 0.3 {
				meaningfulCount++
			}
		}
		return meaningfulCount >= 5
	}
	return false
}

// GenerateSummary 生成摘要
// 取缓冲区前半部分消息 + 旧摘要，通过 LLM 生成新摘要
// modelID 指定使用的模型，0 则降级为规则式摘要
func (g *SummaryGenerator) GenerateSummary(ctx context.Context, buffer *MessageBuffer, oldSummary string, modelID uint) (string, error) {
	if buffer == nil || len(buffer.Messages) == 0 {
		return oldSummary, nil
	}

	// 尝试 LLM 摘要
	if g.modelResolver != nil && modelID > 0 {
		summary, err := g.generateWithLLM(ctx, buffer, oldSummary, modelID)
		if err != nil {
			g.logger.Warn("LLM 摘要生成失败，降级为规则式摘要", zap.Error(err))
		} else {
			return summary, nil
		}
	}

	// 降级：规则式摘要
	return g.generateRuleBased(buffer, oldSummary), nil
}

// generateWithLLM 使用 LLM 生成摘要
func (g *SummaryGenerator) generateWithLLM(ctx context.Context, buffer *MessageBuffer, oldSummary string, modelID uint) (string, error) {
	client, err := g.modelResolver.CreateLLMClient(ctx, modelID)
	if err != nil {
		return "", fmt.Errorf("创建 LLM 客户端失败: %w", err)
	}

	// 构建摘要 prompt
	messages := g.buildSummaryPrompt(buffer, oldSummary)

	resp, err := client.Generate(ctx, messages)
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
func (g *SummaryGenerator) buildSummaryPrompt(buffer *MessageBuffer, oldSummary string) []*schema.Message {
	systemPrompt := `你是一个对话摘要助手。请根据提供的对话历史和已有摘要，生成一段简洁的摘要。
要求：
1. 保留关键信息：用户意图、讨论主题、重要结论、提及的实体（人名、地点、数字等）
2. 合并已有摘要和新对话内容，不要丢失早期的重要信息
3. 摘要长度控制在 300 字以内
4. 用简洁的叙述风格，不要使用列表格式
5. 如果已有摘要中有新对话未涉及的内容，应当保留`

	// 构建对话内容
	var conversationParts []string
	for _, msg := range buffer.Messages {
		role := "用户"
		if msg.Role == "assistant" {
			role = "助手"
		}
		conversationParts = append(conversationParts, fmt.Sprintf("[%s] %s", role, msg.Content))
	}
	conversation := strings.Join(conversationParts, "\n")

	userContent := ""
	if oldSummary != "" {
		userContent = fmt.Sprintf("已有摘要：\n%s\n\n新对话内容：\n%s\n\n请生成合并后的新摘要：", oldSummary, conversation)
	} else {
		userContent = fmt.Sprintf("对话内容：\n%s\n\n请生成摘要：", conversation)
	}

	return []*schema.Message{
		{Role: schema.System, Content: systemPrompt},
		{Role: schema.User, Content: userContent},
	}
}

// generateRuleBased 规则式降级摘要
func (g *SummaryGenerator) generateRuleBased(buffer *MessageBuffer, oldSummary string) string {
	var parts []string

	// 保留旧摘要
	if oldSummary != "" {
		parts = append(parts, "[之前的摘要] "+oldSummary)
	}

	// 取最近 5 条消息截取
	recent := buffer.Messages
	if len(recent) > 5 {
		recent = recent[len(recent)-5:]
	}
	for _, msg := range recent {
		content := msg.Content
		if len(content) > 30 {
			content = content[:30] + "..."
		}
		role := "用户"
		if msg.Role == "assistant" {
			role = "助手"
		}
		parts = append(parts, fmt.Sprintf("[%s] %s", role, content))
	}

	return strings.Join(parts, "\n")
}

// GenerateSummaryAsync 异步生成摘要
func (g *SummaryGenerator) GenerateSummaryAsync(ctx context.Context, buffer *MessageBuffer, oldSummary string, modelID uint, callback func(string)) {
	go func() {
		// 使用超时 context，防止 LLM 调用卡死
		summaryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		summary, err := g.GenerateSummary(summaryCtx, buffer, oldSummary, modelID)
		if err != nil {
			g.logger.Error("生成摘要失败", zap.Error(err))
			return
		}
		if callback != nil {
			callback(summary)
		}
	}()
}
