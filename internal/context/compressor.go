package context

import (
	"Qavor/pkg/utils"

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

// EstimateTokens 计算单条消息的 Token 数（正文用 tiktoken 精确计算）。
// 除 Content 外，同时计入 ToolCalls JSON 结构、tool_call_id 等此前漏算的部分：
// 工具调用助手消息的 Content 常为空，漏算会导致输入预算被低估、上下文裁剪过量。
func (t *ContextTokenizer) EstimateTokens(msg *schema.Message) int {
	if msg == nil {
		return 0
	}
	tokens := 0
	if msg.Content != "" {
		tokens += utils.CountTokens(msg.Content)
	}
	if msg.ReasoningContent != "" {
		tokens += utils.CountTokens(msg.ReasoningContent)
	}
	// 助手消息的工具调用（tool_calls 数组），按 OpenAI 计数规则近似：
	// 每个调用固定结构开销 + id/name/arguments 内容 token
	for _, tc := range msg.ToolCalls {
		tokens += 4 // {"id":...,"type":...,"function":{...}} 键名与引号
		if tc.ID != "" {
			tokens += utils.CountTokens(tc.ID)
		}
		tokens += 2 // "name": / "arguments":
		if tc.Function.Name != "" {
			tokens += utils.CountTokens(tc.Function.Name)
		}
		if tc.Function.Arguments != "" {
			tokens += 1 + utils.CountTokens(tc.Function.Arguments)
		}
	}
	// 工具结果消息的 tool_call_id
	if msg.ToolCallID != "" {
		tokens += 1 + utils.CountTokens(msg.ToolCallID)
	}
	return tokens
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
