package repository

import (
	"context"
	"errors"
	"time"

	"Qavor/internal/model/entity"

	"gorm.io/gorm"
)

type documentProcessingJobRepository struct{ db *gorm.DB }

func NewDocumentProcessingJobRepository(db *gorm.DB) DocumentProcessingJobRepository {
	return &documentProcessingJobRepository{db: db}
}

func (r *documentProcessingJobRepository) Create(job *entity.DocumentProcessingJob) error {
	return r.db.Create(job).Error
}

func (r *documentProcessingJobRepository) FindByJobID(jobID string) (*entity.DocumentProcessingJob, error) {
	var job entity.DocumentProcessingJob
	err := r.db.Where("job_id = ?", jobID).First(&job).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &job, nil
}

func (r *documentProcessingJobRepository) ListRecent(ctx context.Context, limit int) ([]*entity.DocumentProcessingJob, error) {
	var jobs []*entity.DocumentProcessingJob
	if err := r.db.WithContext(ctx).Order("created_at DESC").Limit(limit).Find(&jobs).Error; err != nil {
		return nil, err
	}
	return jobs, nil
}

func (r *documentProcessingJobRepository) ClaimByJobID(ctx context.Context, jobID, workerID string) (*entity.DocumentProcessingJob, error) {
	return r.claimByJobID(ctx, jobID, workerID, []string{entity.JobPending}, true)
}

func (r *documentProcessingJobRepository) ReclaimByJobID(ctx context.Context, jobID, workerID string) (*entity.DocumentProcessingJob, error) {
	return r.claimByJobID(ctx, jobID, workerID, []string{entity.JobPending, entity.JobRunning}, false)
}

func (r *documentProcessingJobRepository) claimByJobID(ctx context.Context, jobID, workerID string, statuses []string, incrementAttempt bool) (*entity.DocumentProcessingJob, error) {
	now := time.Now()
	updates := map[string]any{
		"status":     entity.JobRunning,
		"worker_id":  workerID,
		"started_at": now,
	}
	if incrementAttempt {
		updates["attempt"] = gorm.Expr("attempt + 1")
	}
	result := r.db.WithContext(ctx).
		Model(&entity.DocumentProcessingJob{}).
		Where("job_id = ? AND status IN ?", jobID, statuses).
		Updates(updates)
	if result.Error != nil {
		return nil, result.Error
	}
	return r.FindByJobID(jobID)
}

func (r *documentProcessingJobRepository) ClaimNext(ctx context.Context, workerID string, lease time.Duration) (*entity.DocumentProcessingJob, error) {
	var claimed *entity.DocumentProcessingJob
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var job entity.DocumentProcessingJob
		err := tx.Raw("SELECT * FROM document_processing_jobs WHERE status = ? AND available_at <= NOW() ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT 1", entity.JobPending).Scan(&job).Error
		if err != nil {
			return err
		}
		if job.ID == 0 {
			return nil
		}
		now, expires := time.Now(), time.Now().Add(lease)
		if err := tx.Model(&job).Updates(map[string]any{"status": entity.JobRunning, "worker_id": workerID, "attempt": job.Attempt + 1, "started_at": now, "lease_expires_at": expires}).Error; err != nil {
			return err
		}
		job.Status, job.WorkerID, job.Attempt, job.StartedAt, job.LeaseExpiresAt = entity.JobRunning, workerID, job.Attempt+1, &now, &expires
		claimed = &job
		return nil
	})
	return claimed, err
}

func (r *documentProcessingJobRepository) MarkSucceeded(jobID string) error {
	now := time.Now()
	return r.db.Model(&entity.DocumentProcessingJob{}).Where("job_id = ?", jobID).Updates(map[string]any{"status": entity.JobSucceeded, "finished_at": now, "lease_expires_at": nil}).Error
}

func (r *documentProcessingJobRepository) MarkFailed(jobID, errorCode, errorMessage string) error {
	now := time.Now()
	return r.db.Model(&entity.DocumentProcessingJob{}).Where("job_id = ?", jobID).Updates(map[string]any{"status": entity.JobFailed, "error_code": errorCode, "error_message": errorMessage, "finished_at": now, "lease_expires_at": nil}).Error
}
