package service

import (
	"context"

	"github.com/cloudwego/eino/schema"
)

// ChatService 聊天服务接口
type ChatService interface {
	// Chat 发送消息并获取回复
	Chat(ctx context.Context, conversationID uint, agentSlug string, message string) (*ChatResult, error)

	// ChatStream 流式发送消息（预留，后续接入 SSE）
	ChatStream(ctx context.Context, conversationID uint, agentSlug string, message string) (<-chan *schema.Message, error)
}

// ChatResult 聊天结果
type ChatResult struct {
	MessageID      uint   `json:"message_id"`
	ConversationID uint   `json:"conversation_id"`
	Content        string `json:"content"`
	DeliveryStatus string `json:"delivery_status"`
}
