package sse

import "time"

// EventType SSE 事件类型
type EventType string

const (
	// SSE 自身事件
	EventConnected         EventType = "connected"          // 连接建立
	EventHeartbeat         EventType = "heartbeat"          // 连接保活心跳
	EventBusinessHeartbeat EventType = "business_heartbeat" // 业务心跳

	// Runtime 事件（SSE 只负责透传）
	EventMessageStart     EventType = "message.start"     // 消息开始
	EventMessageDelta     EventType = "message.delta"     // 增量内容
	EventMessageComplete  EventType = "message.complete"  // 消息完成
	EventMessageError     EventType = "message.error"     // 消息错误
	EventMessageCancelled EventType = "message.cancelled" // 消息取消

	// 流结束
	EventDone EventType = "done" // 流结束

	// 工具调用事件
	EventToolCall EventType = "tool.call" // 工具调用
)

// SSEEvent SSE 事件
type SSEEvent struct {
	ID             string      `json:"id"`                        // 事件唯一ID，用于断线恢复
	Type           EventType   `json:"event"`                     // 事件类型
	ConversationID uint        `json:"conversation_id,omitempty"` // 会话标识
	MessageID      string      `json:"message_id,omitempty"`      // 消息标识
	TaskID         string      `json:"task_id,omitempty"`         // 任务标识
	Timestamp      int64       `json:"timestamp"`                 // 事件时间戳
	Data           interface{} `json:"data"`                      // 事件数据
}

// NewSSEEvent 创建 SSE 事件
func NewSSEEvent(eventType EventType, data interface{}) SSEEvent {
	return SSEEvent{
		ID:        generateEventID(),
		Type:      eventType,
		Timestamp: time.Now().Unix(),
		Data:      data,
	}
}

// --- 连接事件数据 ---

// ConnectedData 连接建立数据
type ConnectedData struct {
	ConnID string `json:"conn_id"`
}

// HeartbeatData 心跳数据
type HeartbeatData struct {
	Timestamp int64 `json:"timestamp"`
}

// ErrorData 错误数据
type ErrorData struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// --- 消息事件数据 ---

// MessageStartData 消息开始数据
type MessageStartData struct {
	MessageID      string `json:"message_id"`
	ConversationID uint   `json:"conversation_id"`
	Model          string `json:"model"`
}

// MessageDeltaData 增量内容数据
type MessageDeltaData struct {
	MessageID string `json:"message_id"`
	Content   string `json:"content"`
	Reasoning string `json:"reasoning,omitempty"`
	Index     int    `json:"index"`
}

// MessageCompleteData 消息完成数据
type MessageCompleteData struct {
	MessageID    string `json:"message_id"`
	Content      string `json:"content"`
	TokenCount   int    `json:"token_count"`
	FinishReason string `json:"finish_reason"`
}

// MessageCancelledData 消息取消数据
type MessageCancelledData struct {
	MessageID string `json:"message_id"`
	Reason    string `json:"reason"`
}

// ToolCallData 工具调用数据
type ToolCallData struct {
	MessageID string `json:"message_id"`
	ToolName  string `json:"tool_name"`
	Arguments string `json:"arguments"`
	Result    string `json:"result,omitempty"`
}

// --- 工具函数 ---

// generateEventID 生成事件ID
func generateEventID() string {
	return time.Now().Format("20060102150405.000000000")
}
