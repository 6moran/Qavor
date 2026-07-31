package repository

import (
	"context"
	"time"

	"Qavor/internal/model/entity"
)

type DocumentProcessingJobRepository interface {
	Create(job *entity.DocumentProcessingJob) error
	FindByJobID(jobID string) (*entity.DocumentProcessingJob, error)
	ListRecent(ctx context.Context, limit int) ([]*entity.DocumentProcessingJob, error)
	ClaimByJobID(ctx context.Context, jobID, workerID string) (*entity.DocumentProcessingJob, error)
	ReclaimByJobID(ctx context.Context, jobID, workerID string) (*entity.DocumentProcessingJob, error)
	ClaimNext(ctx context.Context, workerID string, lease time.Duration) (*entity.DocumentProcessingJob, error)
	MarkSucceeded(jobID string) error
	MarkFailed(jobID, errorCode, errorMessage string) error
}
