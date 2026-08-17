package shortterm

import (
	"strings"
	"time"

	"Qavor/pkg/utils"

	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

// MessageBufferManager 消息缓冲区管理器
type MessageBufferManager struct {
	logger  *zap.Logger
	maxSize int // 缓冲区最大消息数
}

// NewMessageBufferManager 创建缓冲区管理器
func NewMessageBufferManager(logger *zap.Logger, maxSize int) *MessageBufferManager {
	if maxSize <= 0 {
		maxSize = 20 // 默认缓冲20条消息
	}
	return &MessageBufferManager{
		logger:  logger,
		maxSize: maxSize,
	}
}

// AddMessage 添加消息到缓冲区
func (m *MessageBufferManager) AddMessage(buffer *MessageBuffer, msg *schema.Message, messageID string) {
	bufMsg := BufferMessage{
		MessageID:  messageID,
		Role:       string(msg.Role),
		Content:    msg.Content,
		Timestamp:  time.Now(),
		Tokens:     utils.CountTokens(msg.Content),
		Importance: calculateImportance(msg),
	}

	buffer.Messages = append(buffer.Messages, bufMsg)
	buffer.TotalTokens += bufMsg.Tokens
}

// calculateImportance 计算消息重要性
func calculateImportance(msg *schema.Message) float64 {
	score := 0.5 // 基础分
	content := msg.Content

	// 包含实体/数字：+0.2
	if containsEntityOrNumber(content) {
		score += 0.2
	}

	// 包含决策/结论：+0.2
	if containsDecisionOrConclusion(content) {
		score += 0.2
	}

	// 用户明确要求记住：+0.3
	if containsRememberRequest(content) {
		score += 0.3
	}

	// 纯寒暄/确认：-0.2
	if isGreetingOrConfirmation(content) {
		score -= 0.2
	}

	// 限制在 0-1 范围内
	if score < 0 {
		score = 0
	}
	if score > 1 {
		score = 1
	}

	return score
}

// containsEntityOrNumber 检查是否包含实体或数字
func containsEntityOrNumber(content string) bool {
	// 简单检查：包含数字或常见实体
	digits := []string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9"}
	for _, d := range digits {
		if strings.Contains(content, d) {
			return true
		}
	}

	// 检查常见实体
	entities := []string{"张三", "李四", "北京", "上海", "Python", "Go", "Java"}
	for _, e := range entities {
		if strings.Contains(content, e) {
			return true
		}
	}

	return false
}

// containsDecisionOrConclusion 检查是否包含决策或结论
func containsDecisionOrConclusion(content string) bool {
	keywords := []string{"决定", "选择", "确认", "同意", "拒绝", "最终", "结论", "方案"}
	for _, k := range keywords {
		if strings.Contains(content, k) {
			return true
		}
	}
	return false
}

// containsRememberRequest 检查是否包含记住请求
func containsRememberRequest(content string) bool {
	keywords := []string{"记住", "记下", "记录", "保存", "备忘"}
	for _, k := range keywords {
		if strings.Contains(content, k) {
			return true
		}
	}
	return false
}

// isGreetingOrConfirmation 检查是否为寒暄或确认
func isGreetingOrConfirmation(content string) bool {
	keywords := []string{"好的", "嗯", "是的", "对", "ok", "OK", "收到", "谢谢", "感谢"}
	for _, k := range keywords {
		if strings.EqualFold(content, k) {
			return true
		}
	}
	return false
}

// SlideWindow 滑动窗口：取前半部分消息用于摘要生成，保留后半部分
// 返回被摘走的消息列表，调用方用它生成摘要
func (m *MessageBufferManager) SlideWindow(buffer *MessageBuffer) []BufferMessage {
	if buffer == nil || len(buffer.Messages) == 0 {
		return nil
	}

	half := len(buffer.Messages) / 2
	if half == 0 {
		return nil
	}

	// 取前半部分（用于摘要）
	summarized := buffer.Messages[:half]
	// 保留后半部分
	buffer.Messages = buffer.Messages[half:]

	// 重新计算 token
	buffer.TotalTokens = 0
	for _, msg := range buffer.Messages {
		buffer.TotalTokens += msg.Tokens
	}

	return summarized
}

// GetMessagesByTokens 按 Token 数获取消息
func (m *MessageBufferManager) GetMessagesByTokens(buffer *MessageBuffer, maxTokens int) []BufferMessage {
	var result []BufferMessage
	totalTokens := 0

	// 从最新消息开始，尽可能多地返回
	for i := len(buffer.Messages) - 1; i >= 0; i-- {
		msg := buffer.Messages[i]
		if totalTokens+msg.Tokens > maxTokens {
			break
		}
		result = append([]BufferMessage{msg}, result...)
		totalTokens += msg.Tokens
	}

	return result
}
