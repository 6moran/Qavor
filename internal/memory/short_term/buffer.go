package shortterm

import (
	"time"

	"Qavor/pkg/utils"

	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

// RecentMessagesManager 最近消息管理器
// 定位：维护一个滚动窗口，保留最近 N 条消息供摘要生成和上下文注入
type RecentMessagesManager struct {
	logger  *zap.Logger
	maxSize int // 最大消息数
}

// NewRecentMessagesManager 创建最近消息管理器
func NewRecentMessagesManager(logger *zap.Logger, maxSize int) *RecentMessagesManager {
	if maxSize <= 0 {
		maxSize = 20
	}
	return &RecentMessagesManager{
		logger:  logger,
		maxSize: maxSize,
	}
}

// AddMessage 添加消息到最近消息列表
func (m *RecentMessagesManager) AddMessage(messages *[]Message, msg *schema.Message) {
	*messages = append(*messages, Message{
		Role:      string(msg.Role),
		Content:   msg.Content,
		Timestamp: time.Now(),
		Tokens:    utils.CountTokens(msg.Content),
	})

	// 超出上限时裁剪最早的
	if len(*messages) > m.maxSize {
		*messages = (*messages)[len(*messages)-m.maxSize:]
	}
}

// SlideWindow 滑动窗口：取前半部分消息用于摘要生成，保留后半部分
// 返回被摘走的消息列表，调用方用它生成摘要
func (m *RecentMessagesManager) SlideWindow(messages *[]Message) []Message {
	if messages == nil || len(*messages) == 0 {
		return nil
	}

	half := len(*messages) / 2
	if half == 0 {
		return nil
	}

	summarized := (*messages)[:half]
	*messages = (*messages)[half:]
	return summarized
}

// GetMessagesByTokens 按 Token 数获取最近消息（从新到旧，尽可能多地返回）
func (m *RecentMessagesManager) GetMessagesByTokens(messages []Message, maxTokens int) []Message {
	var result []Message
	totalTokens := 0

	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		if totalTokens+msg.Tokens > maxTokens {
			break
		}
		result = append([]Message{msg}, result...)
		totalTokens += msg.Tokens
	}

	return result
}

// TotalTokens 计算消息列表的总 Token 数
func (m *RecentMessagesManager) TotalTokens(messages []Message) int {
	total := 0
	for _, msg := range messages {
		total += msg.Tokens
	}
	return total
}
