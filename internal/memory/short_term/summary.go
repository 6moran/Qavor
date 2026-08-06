package shortterm

import (
	"context"
	"fmt"
	"strings"

	"go.uber.org/zap"
)

// SummaryGenerator 摘要生成器
// 注意：实际的 LLM 摘要压缩由 eino 的 summarization middleware 处理
// 这里只提供简单的降级方案
type SummaryGenerator struct {
	logger *zap.Logger
	config *SummaryConfig
}

// NewSummaryGenerator 创建摘要生成器
func NewSummaryGenerator(logger *zap.Logger, config *SummaryConfig) *SummaryGenerator {
	if config == nil {
		config = DefaultSummaryConfig()
	}
	return &SummaryGenerator{
		logger: logger,
		config: config,
	}
}

// ShouldGenerate 判断是否需要生成摘要
func (g *SummaryGenerator) ShouldGenerate(buffer *MessageBuffer) bool {
	if buffer == nil {
		return false
	}
	return len(buffer.Messages) >= g.config.MessageThreshold ||
		buffer.TotalTokens >= g.config.TokenThreshold
}

// GenerateSummary 生成摘要（简单规则，作为降级方案）
// 实际的 LLM 摘要压缩由 eino summarization middleware 在 Agent 层处理
func (g *SummaryGenerator) GenerateSummary(_ context.Context, buffer *MessageBuffer) (string, error) {
	if buffer == nil || len(buffer.Messages) == 0 {
		return "", nil
	}

	var summaryParts []string

	// 获取最近5条消息
	recentMessages := buffer.Messages
	if len(recentMessages) > 5 {
		recentMessages = recentMessages[len(recentMessages)-5:]
	}

	for _, msg := range recentMessages {
		content := msg.Content
		if len(content) > 20 {
			content = content[:20] + "..."
		}
		summaryParts = append(summaryParts, fmt.Sprintf("[%s] %s", msg.Role, content))
	}

	summary := strings.Join(summaryParts, "\n")
	return summary, nil
}

// GenerateSummaryAsync 异步生成摘要
func (g *SummaryGenerator) GenerateSummaryAsync(ctx context.Context, buffer *MessageBuffer, callback func(string)) {
	go func() {
		summary, err := g.GenerateSummary(ctx, buffer)
		if err != nil {
			g.logger.Error("生成摘要失败", zap.Error(err))
			return
		}
		if callback != nil {
			callback(summary)
		}
	}()
}
