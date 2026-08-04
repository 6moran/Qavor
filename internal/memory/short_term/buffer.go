package shortterm

import (
	"time"

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
		MessageID: messageID,
		Role:      string(msg.Role),
		Content:   msg.Content,
		Timestamp: time.Now(),
		Tokens:    estimateTokens(msg.Content),
	}

	buffer.Messages = append(buffer.Messages, bufMsg)
	buffer.TotalTokens += bufMsg.Tokens

	// 如果缓冲区满，移除最旧的消息
	for len(buffer.Messages) > m.maxSize {
		removed := buffer.Messages[0]
		buffer.Messages = buffer.Messages[1:]
		buffer.TotalTokens -= removed.Tokens
	}
}

// GetRecentMessages 获取最近的消息
func (m *MessageBufferManager) GetRecentMessages(buffer *MessageBuffer, count int) []BufferMessage {
	if count <= 0 || count > len(buffer.Messages) {
		count = len(buffer.Messages)
	}
	return buffer.Messages[len(buffer.Messages)-count:]
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

// estimateTokens 估算 Token 数
func estimateTokens(content string) int {
	// 简单估算：中文约1.5字符/token，英文约4字符/token
	// 这里简化为：每10个字符约1个token
	if len(content) == 0 {
		return 0
	}
	return (len(content) + 9) / 10
}
