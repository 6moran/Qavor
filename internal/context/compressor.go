package context

import (
	"unicode/utf8"

	"github.com/cloudwego/eino/schema"
)

// ContextTokenizer Token 计数与裁剪器
type ContextTokenizer struct {
	maxTokens     int // 模型最大 Token
	reserveTokens int // 预留给回复的 Token
}

// NewContextTokenizer 创建裁剪器
func NewContextTokenizer(maxTokens, reserveTokens int) *ContextTokenizer {
	return &ContextTokenizer{
		maxTokens:     maxTokens,
		reserveTokens: reserveTokens,
	}
}

// EstimateTokens 估算单条消息的 Token 数
func (t *ContextTokenizer) EstimateTokens(msg *schema.Message) int {
	content := msg.Content
	if content == "" {
		return 0
	}

	byteCount := len(content)
	charCount := utf8.RuneCountInString(content)

	ratio := float64(byteCount) / float64(charCount)

	var tokens float64
	if ratio > 2.0 {
		tokens = float64(charCount) / 1.5
	} else {
		tokens = float64(charCount) / 4.0
	}

	return int(tokens) + 4
}

// TrimMessages 裁剪消息列表以适应 Token 窗口
func (t *ContextTokenizer) TrimMessages(messages []*schema.Message, systemTokens int) []*schema.Message {
	availableTokens := t.maxTokens - t.reserveTokens - systemTokens

	if availableTokens <= 0 {
		if len(messages) > 0 {
			return messages[len(messages)-1:]
		}
		return nil
	}

	totalTokens := 0
	keepStart := 0

	for i := len(messages) - 1; i >= 0; i-- {
		msgTokens := t.EstimateTokens(messages[i])
		if totalTokens+msgTokens > availableTokens {
			keepStart = i + 1
			break
		}
		totalTokens += msgTokens
		keepStart = i
	}

	keepStart = t.adjustForToolCallingPairs(messages, keepStart)

	return messages[keepStart:]
}

// adjustForToolCallingPairs 保护 Tool Calling 消息对不被拆开
func (t *ContextTokenizer) adjustForToolCallingPairs(messages []*schema.Message, keepStart int) int {
	if keepStart <= 0 {
		return keepStart
	}

	for i := keepStart; i < len(messages); i++ {
		msg := messages[i]
		if msg.Role == schema.Tool && i > 0 {
			prevMsg := messages[i-1]
			if prevMsg.Role == schema.Assistant && len(prevMsg.ToolCalls) > 0 {
				if keepStart == i {
					keepStart = i - 1
				}
			}
		}
	}

	return keepStart
}

// CountAllTokens 计算消息列表总 Token 数
func (t *ContextTokenizer) CountAllTokens(messages []*schema.Message) int {
	total := 0
	for _, msg := range messages {
		total += t.EstimateTokens(msg)
	}
	return total
}
