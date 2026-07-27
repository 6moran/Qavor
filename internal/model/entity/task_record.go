package entity

import "time"

// TaskRecord 任务记录实体
type TaskRecord struct {
	ID              string     `gorm:"type:varchar(32);primarykey;comment:任务ID" json:"id"`
	Name            string     `gorm:"type:varchar(255);not null;comment:任务名称" json:"name"`
	Type            string     `gorm:"type:varchar(64);not null;index;comment:任务类型" json:"type"`
	Status          string     `gorm:"type:varchar(32);not null;index;default:pending;comment:状态" json:"status"`
	Progress        float64    `gorm:"not null;default:0;comment:进度（0-1）" json:"progress"`
	Message         string     `gorm:"type:text;not null;default:'';comment:状态消息" json:"message"`
	Payload         JSON       `gorm:"type:json;comment:任务参数" json:"payload,omitempty"`
	Result          JSON       `gorm:"type:json;comment:执行结果" json:"result,omitempty"`
	Error           string     `gorm:"type:text;comment:错误信息" json:"error,omitempty"`
	CancelRequested int        `gorm:"not null;default:0;comment:是否请求取消" json:"cancel_requested"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	StartedAt       *time.Time `gorm:"comment:开始时间" json:"started_at,omitempty"`
	CompletedAt     *time.Time `gorm:"comment:完成时间" json:"completed_at,omitempty"`
}

// TableName 指定表名
func (TaskRecord) TableName() string {
	return "task_records"
}
