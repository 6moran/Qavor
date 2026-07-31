package entity

import "time"

const (
	JobPending   = "pending"
	JobRunning   = "running"
	JobSucceeded = "succeeded"
	JobFailed    = "failed"
	JobCancelled = "cancelled"
)

// CanTransitionJob reports whether a processing job can move to target.
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

// DocumentProcessingJob represents one asynchronous document-ingestion attempt.
type DocumentProcessingJob struct {
	BaseEntity
	JobID          string     `gorm:"type:varchar(64);uniqueIndex;not null" json:"job_id"`
	KBID           string     `gorm:"type:varchar(80);not null;index" json:"kb_id"`
	FileID         string     `gorm:"type:varchar(64);not null;index" json:"file_id"`
	Status         string     `gorm:"type:varchar(32);not null;default:pending;index" json:"status"`
	Attempt        int        `gorm:"not null;default:0" json:"attempt"`
	MaxAttempts    int        `gorm:"not null;default:1" json:"max_attempts"`
	AvailableAt    time.Time  `gorm:"not null;index" json:"available_at"`
	WorkerID       string     `gorm:"type:varchar(128)" json:"worker_id,omitempty"`
	LeaseExpiresAt *time.Time `gorm:"index" json:"lease_expires_at,omitempty"`
	ErrorCode      string     `gorm:"type:varchar(64)" json:"error_code,omitempty"`
	ErrorMessage   string     `gorm:"type:text" json:"error_message,omitempty"`
	StartedAt      *time.Time `json:"started_at,omitempty"`
	FinishedAt     *time.Time `json:"finished_at,omitempty"`
}

func (DocumentProcessingJob) TableName() string {
	return "document_processing_jobs"
}
