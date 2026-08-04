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
	// CreateForFileTransition 原子性地检查活跃任务、转换文件状态并插入任务。
	// 成功返回 (true, nil)，状态冲突（无行变更）返回 (false, nil)，数据库错误返回 (false, err)。
	CreateForFileTransition(ctx context.Context, job *entity.DocumentProcessingJob, fromStatuses []string, queuedStatus string) (bool, error)
	// FindActiveByFileAndType 查找给定文件和任务类型的待处理或运行中的任务。
	FindActiveByFileAndType(ctx context.Context, kbID, fileID, jobType string) (*entity.DocumentProcessingJob, error)
	// CancelPendingByFile 取消给定文件的待处理（非运行中）任务。
	CancelPendingByFile(ctx context.Context, kbID, fileID string) error
}
