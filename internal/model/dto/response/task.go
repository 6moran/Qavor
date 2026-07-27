package response

import (
	"Qavor/internal/model/entity"
	"time"
)

// TaskRecordResponse 任务记录响应
type TaskRecordResponse struct {
	ID              string      `json:"id"`
	Name            string      `json:"name"`
	Type            string      `json:"type"`
	Status          string      `json:"status"`
	Progress        float64     `json:"progress"`
	Message         string      `json:"message"`
	Payload         entity.JSON `json:"payload,omitempty"`
	Result          entity.JSON `json:"result,omitempty"`
	Error           string      `json:"error,omitempty"`
	CancelRequested int         `json:"cancel_requested"`
	CreatedAt       time.Time   `json:"created_at"`
	UpdatedAt       time.Time   `json:"updated_at"`
	StartedAt       *time.Time  `json:"started_at,omitempty"`
	CompletedAt     *time.Time  `json:"completed_at,omitempty"`
}

// TaskRecordListResponse 任务记录列表响应
type TaskRecordListResponse struct {
	Total int64                `json:"total"`
	Items []TaskRecordResponse `json:"items"`
}
