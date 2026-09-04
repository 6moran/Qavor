package service

import (
	"context"
)

// ChatService 聊天服务接口
type ChatService interface {
	// Chat 发送消息并获取回复（同步）
	Chat(ctx context.Context, conversationID uint, agentSlug string, message string) (*ChatResult, error)
}

// ChatResult 聊天结果
type ChatResult struct {
	MessageID      uint   `json:"message_id"`
	ConversationID uint   `json:"conversation_id"`
	Content        string `json:"content"`
	DeliveryStatus string `json:"delivery_status"`
}
