package response

import (
	"Qavor/internal/model/entity"
	"time"
)

// AgentRunResponse Agent运行任务响应
type AgentRunResponse struct {
	ID                       string      `json:"id"`
	ConversationThreadID     string      `json:"conversation_thread_id"`
	AgentSlug                string      `json:"agent_slug"`
	Status                   string      `json:"status"`
	RequestID                string      `json:"request_id"`
	ConversationID           *uint       `json:"conversation_id,omitempty"`
	CreatedByRunID           string      `json:"created_by_run_id,omitempty"`
	SubagentThreadRelationID *uint       `json:"subagent_thread_relation_id,omitempty"`
	RunType                  string      `json:"run_type"`
	InputMessageID           *uint       `json:"input_message_id,omitempty"`
	OutputMessageID          *uint       `json:"output_message_id,omitempty"`
	LastEventID              string      `json:"last_event_id,omitempty"`
	InputPayload             entity.JSON `json:"input_payload"`
	ErrorType                string      `json:"error_type,omitempty"`
	ErrorMessage             string      `json:"error_message,omitempty"`
	StartedAt                *time.Time  `json:"started_at,omitempty"`
	FinishedAt               *time.Time  `json:"finished_at,omitempty"`
	CreatedAt                time.Time   `json:"created_at"`
	UpdatedAt                time.Time   `json:"updated_at"`
}

// AgentRunListResponse Agent运行任务列表响应
type AgentRunListResponse struct {
	Total int64              `json:"total"`
	Items []AgentRunResponse `json:"items"`
}
