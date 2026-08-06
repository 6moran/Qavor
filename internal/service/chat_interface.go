package service

import (
	"context"
)

// ChatService 聊天服务接口
type ChatService interface {
	// Chat 发送消息并获取回复（同步）
	Chat(ctx context.Context, conversationID uint, agentSlug string, message string) (*ChatResult, error)

	// ChatStream 流式发送消息，通过 SSE 推送事件
	// 前端通过 POST /api/v1/chat/stream 触发，结果通过已建立的 SSE 连接推送
	ChatStream(ctx context.Context, conversationID uint, agentSlug string, message string) error
}

// ChatResult 聊天结果
type ChatResult struct {
	MessageID      uint   `json:"message_id"`
	ConversationID uint   `json:"conversation_id"`
	Content        string `json:"content"`
	DeliveryStatus string `json:"delivery_status"`
}
