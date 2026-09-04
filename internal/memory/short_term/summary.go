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

// SummaryGenerator 任务可恢复摘要生成器
// 核心问题：如果 Agent 中断后重新接手，它需要知道什么才能继续工作？
type SummaryGenerator struct {
	logger        *zap.Logger
	config        *SummaryConfig
	modelResolver ModelResolver
}

// NewSummaryGenerator 创建摘要生成器
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
func (g *SummaryGenerator) ShouldGenerate(messages []Message) bool {
	if len(messages) == 0 {
		return false
	}
	totalTokens := 0
	for _, msg := range messages {
		totalTokens += msg.Tokens
	}
	return len(messages) >= g.config.MessageThreshold || totalTokens >= g.config.TokenThreshold
}

// GenerateSummary 生成任务可恢复摘要
func (g *SummaryGenerator) GenerateSummary(ctx context.Context, messages []Message, oldSummary string, modelID uint) (string, error) {
	if len(messages) == 0 {
		return oldSummary, nil
	}

	if g.modelResolver != nil && modelID > 0 {
		summary, err := g.generateWithLLM(ctx, messages, oldSummary, modelID)
		if err != nil {
			g.logger.Warn("LLM 摘要生成失败，降级为规则式摘要", zap.Error(err))
		} else {
			return summary, nil
		}
	}

	return g.generateRuleBased(messages, oldSummary), nil
}

// generateWithLLM 使用 LLM 生成任务可恢复摘要
func (g *SummaryGenerator) generateWithLLM(ctx context.Context, messages []Message, oldSummary string, modelID uint) (string, error) {
	client, err := g.modelResolver.CreateLLMClient(ctx, modelID)
	if err != nil {
		return "", fmt.Errorf("创建 LLM 客户端失败: %w", err)
	}

	prompt := g.buildSummaryPrompt(messages, oldSummary)
	resp, err := client.Generate(ctx, prompt)
	if err != nil {
		return "", fmt.Errorf("LLM 生成摘要失败: %w", err)
	}

	summary := strings.TrimSpace(resp.Content)
	if summary == "" {
		return "", fmt.Errorf("LLM 返回空摘要")
	}
	return summary, nil
}

// buildSummaryPrompt 构建任务可恢复摘要的 prompt
func (g *SummaryGenerator) buildSummaryPrompt(messages []Message, oldSummary string) []*schema.Message {
	systemPrompt := `你是一个任务恢复助手。请根据对话历史和已有摘要，生成一段"任务可恢复摘要"。

这个摘要的目的是：如果 Agent 中断后重新接手，它需要知道什么才能继续工作？

必须包含：
1. 当前任务目标（用户要完成什么）
2. 已做的决策和采用的方案
3. 关键技术上下文（涉及的文件、框架、工具、代码位置）
4. 当前进度（做到哪一步了，下一步该做什么）
5. 遇到的问题或阻塞点

不要包含：
- 闲聊内容
- 已经完成且不再相关的历史对话
- 重复的信息

摘要长度控制在 300 字以内，用简洁叙述风格。`

	var parts []string
	for _, msg := range messages {
		role := "用户"
		if msg.Role == "assistant" {
			role = "助手"
		}
		parts = append(parts, fmt.Sprintf("[%s] %s", role, msg.Content))
	}
	conversation := strings.Join(parts, "\n")

	var userContent string
	if oldSummary != "" {
		userContent = fmt.Sprintf("已有摘要：\n%s\n\n新对话内容：\n%s\n\n请生成合并后的任务恢复摘要：", oldSummary, conversation)
	} else {
		userContent = fmt.Sprintf("对话内容：\n%s\n\n请生成任务恢复摘要：", conversation)
	}

	return []*schema.Message{
		{Role: schema.System, Content: systemPrompt},
		{Role: schema.User, Content: userContent},
	}
}

// generateRuleBased 规则式降级摘要
// 提取最近消息中的关键技术术语和决策关键词
func (g *SummaryGenerator) generateRuleBased(messages []Message, oldSummary string) string {
	var parts []string

	if oldSummary != "" {
		parts = append(parts, "[之前的摘要] "+oldSummary)
	}

	// 提取最近5条消息的关键信息
	recent := messages
	if len(recent) > 5 {
		recent = recent[len(recent)-5:]
	}
	for _, msg := range recent {
		content := msg.Content
		if len(content) > 50 {
			content = content[:50] + "..."
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
func (g *SummaryGenerator) GenerateSummaryAsync(ctx context.Context, messages []Message, oldSummary string, modelID uint, callback func(string)) {
	go func() {
		summaryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		summary, err := g.GenerateSummary(summaryCtx, messages, oldSummary, modelID)
		if err != nil {
			g.logger.Error("生成摘要失败", zap.Error(err))
			return
		}
		if callback != nil {
			callback(summary)
		}
	}()
}
