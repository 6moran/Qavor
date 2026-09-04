package context

import (
	"fmt"
	"time"

	"github.com/cloudwego/eino/schema"
)

// ContextBuilder Prompt 组装器
type ContextBuilder struct {
	config *ContextConfig
}

// NewContextBuilder 创建组装器
func NewContextBuilder(config *ContextConfig) *ContextBuilder {
	return &ContextBuilder{config: config}
}

// BuildPrompt 组装最终的 Prompt 列表
func (b *ContextBuilder) BuildPrompt(
	window *ContextWindow,
	userMessage *schema.Message,
) []*schema.Message {

	result := make([]*schema.Message, 0, len(window.Messages)+2)

	systemContent := b.buildSystemPrompt(window)
	result = append(result, &schema.Message{
		Role:    schema.System,
		Content: systemContent,
	})

	result = append(result, window.Messages...)
	result = append(result, userMessage)

	return result
}

// buildSystemPrompt 构建系统提示词
func (b *ContextBuilder) buildSystemPrompt(window *ContextWindow) string {
	content := b.config.SystemPrompt

	content += fmt.Sprintf("\n\n当前时间：%s", time.Now().Format("2006-01-02 15:04:05"))

	// 注入任务可恢复摘要
	if window.ShortTermSummary != "" {
		content += fmt.Sprintf("\n\n[任务恢复摘要]\n%s", window.ShortTermSummary)
	}

	// 注入任务状态（目标/进度/技术上下文）
	if window.ShortTermState != "" {
		content += fmt.Sprintf("\n\n[任务状态]\n%s", window.ShortTermState)
	}

	if window.MemoryContext != "" {
		content += fmt.Sprintf("\n\n[用户记忆]\n%s", window.MemoryContext)
	}

	if window.RAGContext != "" {
		content += fmt.Sprintf("\n\n[相关知识]\n%s", window.RAGContext)
	}

	return content
}

// EstimateSystemTokens 估算 System Prompt 的 Token 数
func (b *ContextBuilder) EstimateSystemTokens(window *ContextWindow) int {
	systemContent := b.buildSystemPrompt(window)
	tokenizer := NewContextTokenizer(0, 0)
	return tokenizer.EstimateTokens(&schema.Message{
		Role:    schema.System,
		Content: systemContent,
	})
}
