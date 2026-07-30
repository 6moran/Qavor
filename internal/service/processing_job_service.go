package service

import (
	"context"
	"time"

	"Qavor/internal/model/dto/response"
	"Qavor/internal/model/entity"
	documentqueue "Qavor/internal/queue"
	"Qavor/internal/repository"
	bizerrors "Qavor/pkg/errors"

	"github.com/google/uuid"
)

type processingJobService struct {
	repo     repository.DocumentProcessingJobRepository
	fileRepo repository.KnowledgeFileRepository
	queue    documentqueue.DocumentQueue
}

func NewProcessingJobService(repo repository.DocumentProcessingJobRepository, fileRepo repository.KnowledgeFileRepository, queue documentqueue.DocumentQueue) ProcessingJobService {
	return &processingJobService{repo: repo, fileRepo: fileRepo, queue: queue}
}

func (s *processingJobService) Get(jobID string) (*response.DocumentProcessingJobResponse, error) {
	job, err := s.repo.FindByJobID(jobID)
	if err != nil {
		return nil, err
	}
	if job == nil {
		return nil, bizerrors.New(bizerrors.CodeResourceNotFound, "解析任务不存在")
	}
	return s.documentProcessingJobResponse(job), nil
}

func (s *processingJobService) List(limit int) (*response.DocumentProcessingJobListResponse, error) {
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	jobs, err := s.repo.ListRecent(context.Background(), limit)
	if err != nil {
		return nil, err
	}
	items := make([]response.DocumentProcessingJobResponse, 0, len(jobs))
	for _, job := range jobs {
		if job != nil {
			items = append(items, *s.documentProcessingJobResponse(job))
		}
	}
	return &response.DocumentProcessingJobListResponse{
		Total: int64(len(items)),
		Items: items,
	}, nil
}

func (s *processingJobService) Retry(jobID string) (*response.DocumentProcessingJobResponse, error) {
	if s.queue == nil {
		return nil, bizerrors.New(bizerrors.CodeServiceUnavailable, "文档处理队列暂不可用")
	}
	job, err := s.repo.FindByJobID(jobID)
	if err != nil {
		return nil, err
	}
	if job == nil {
		return nil, bizerrors.New(bizerrors.CodeResourceNotFound, "解析任务不存在")
	}
	if job.Status != entity.JobFailed {
		return nil, bizerrors.New(bizerrors.CodeConflict, "仅失败任务可以重试")
	}
	retry := &entity.DocumentProcessingJob{JobID: uuid.NewString(), KBID: job.KBID, FileID: job.FileID, Status: entity.JobPending, MaxAttempts: job.MaxAttempts, AvailableAt: time.Now()}
	if err := s.repo.Create(retry); err != nil {
		return nil, err
	}
	if err := s.queue.Publish(context.Background(), documentqueue.Message{
		JobID:     retry.JobID,
		KBID:      retry.KBID,
		FileID:    retry.FileID,
		CreatedAt: time.Now(),
		Schema:    1,
	}); err != nil {
		_ = s.repo.MarkFailed(retry.JobID, "QUEUE_ENQUEUE_FAILED", "文档处理任务投递失败")
		return nil, bizerrors.NewWithErr(bizerrors.CodeServiceUnavailable, "文档处理队列暂不可用", err)
	}
	return s.documentProcessingJobResponse(retry), nil
}

func (s *processingJobService) documentProcessingJobResponse(job *entity.DocumentProcessingJob) *response.DocumentProcessingJobResponse {
	resp := &response.DocumentProcessingJobResponse{
		JobID:        job.JobID,
		KBID:         job.KBID,
		FileID:       job.FileID,
		Status:       job.Status,
		Attempt:      job.Attempt,
		MaxAttempts:  job.MaxAttempts,
		ErrorCode:    job.ErrorCode,
		ErrorMessage: job.ErrorMessage,
		CreatedAt:    job.CreatedAt,
		StartedAt:    job.StartedAt,
		FinishedAt:   job.FinishedAt,
	}

	// 尝试查询源文件名
	if s.fileRepo != nil && job.FileID != "" {
		file, err := s.fileRepo.FindByKBIDAndFileID(job.KBID, job.FileID)
		if err == nil && file != nil {
			// 优先使用 OriginalFilename，其次 Filename
			if file.OriginalFilename != "" {
				resp.Filename = file.OriginalFilename
			} else if file.Filename != "" {
				resp.Filename = file.Filename
			}
		}
	}

	return resp
}
