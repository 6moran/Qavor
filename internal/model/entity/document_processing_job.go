package entity

import "time"

const (
	JobPending   = "pending"
	JobRunning   = "running"
	JobSucceeded = "succeeded"
	JobFailed    = "failed"
	JobCancelled = "cancelled"
)

// 分阶段处理的任务类型常量。
const (
	JobTypeParse = "parse"
	JobTypeIndex = "index"
)

// QueuedFileStatusForJobType 将任务类型映射到其对应的文件排队状态。
func QueuedFileStatusForJobType(jobType string) string {
	switch jobType {
	case JobTypeParse:
		return FileParseQueued
	case JobTypeIndex:
		return FileIndexQueued
	default:
		return ""
	}
}

// CanTransitionJob 报告处理任务是否可以移动到目标状态。
func CanTransitionJob(current, target string) bool {
	switch current {
	case JobPending:
		return target == JobRunning || target == JobCancelled
	case JobRunning:
		return target == JobSucceeded || target == JobFailed || target == JobPending || target == JobCancelled
	default:
		return false
	}
}

// DocumentProcessingJob 表示一次异步文档入库尝试。
type DocumentProcessingJob struct {
	BaseEntity
	JobID            string     `gorm:"type:varchar(64);uniqueIndex;not null" json:"job_id"`
	KBID             string     `gorm:"type:varchar(80);not null;index" json:"kb_id"`
	FileID           string     `gorm:"type:varchar(64);not null;index" json:"file_id"`
	JobType          string     `gorm:"type:varchar(16);not null;default:parse;index" json:"job_type"`
	ProcessingParams JSON       `gorm:"type:json" json:"processing_params,omitempty"`
	Status           string     `gorm:"type:varchar(32);not null;default:pending;index" json:"status"`
	Attempt          int        `gorm:"not null;default:0" json:"attempt"`
	MaxAttempts      int        `gorm:"not null;default:1" json:"max_attempts"`
	AvailableAt      time.Time  `gorm:"not null;index" json:"available_at"`
	WorkerID         string     `gorm:"type:varchar(128)" json:"worker_id,omitempty"`
	LeaseExpiresAt   *time.Time `gorm:"index" json:"lease_expires_at,omitempty"`
	ErrorCode        string     `gorm:"type:varchar(64)" json:"error_code,omitempty"`
	ErrorMessage     string     `gorm:"type:text" json:"error_message,omitempty"`
	StartedAt        *time.Time `json:"started_at,omitempty"`
	FinishedAt       *time.Time `json:"finished_at,omitempty"`
}

func (DocumentProcessingJob) TableName() string {
	return "document_processing_jobs"
}
