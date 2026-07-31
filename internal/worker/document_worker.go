package worker

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"Qavor/internal/ingestion"
	"Qavor/internal/model/entity"
	documentqueue "Qavor/internal/queue"
	"Qavor/internal/repository"
	"Qavor/internal/service"
	"Qavor/pkg/logger"

	"go.uber.org/zap"
)

type DocumentWorkerOptions struct {
	ReadBlock        time.Duration
	PendingCheck     time.Duration
	PendingMinIdle   time.Duration
	PendingClaimSize int64
}

type DocumentWorker struct {
	queue   documentqueue.DocumentQueue
	jobs    repository.DocumentProcessingJobRepository
	files   repository.KnowledgeFileRepository
	storage service.ObjectStorage
	parser  *ingestion.Parser
}

func NewDocumentWorker(queue documentqueue.DocumentQueue, jobs repository.DocumentProcessingJobRepository, files repository.KnowledgeFileRepository, storage service.ObjectStorage, parser *ingestion.Parser) *DocumentWorker {
	return &DocumentWorker{queue: queue, jobs: jobs, files: files, storage: storage, parser: parser}
}

func (w *DocumentWorker) processMessage(ctx context.Context, message documentqueue.Message, workerID string, reclaimed bool) (bool, error) {
	if message.InvalidReason != "" || message.JobID == "" {
		return true, nil
	}
	var (
		job *entity.DocumentProcessingJob
		err error
	)
	if reclaimed {
		job, err = w.jobs.ReclaimByJobID(ctx, message.JobID, workerID)
	} else {
		job, err = w.jobs.ClaimByJobID(ctx, message.JobID, workerID)
	}
	if err != nil {
		return false, err
	}
	if job == nil {
		return true, nil
	}
	switch job.Status {
	case entity.JobSucceeded, entity.JobFailed, entity.JobCancelled:
		return true, nil
	}
	if job.Status != entity.JobRunning || job.WorkerID != workerID {
		return false, nil
	}
	if job.MaxAttempts > 0 && job.Attempt > job.MaxAttempts {
		if err := w.jobs.MarkFailed(job.JobID, "MAX_ATTEMPTS_EXCEEDED", "文档处理超过最大尝试次数"); err != nil {
			return false, err
		}
		return true, nil
	}

	file, err := w.files.FindByKBIDAndFileID(job.KBID, job.FileID)
	if err != nil {
		return false, err
	}
	if file == nil {
		if err := w.jobs.MarkFailed(job.JobID, "FILE_NOT_FOUND", "文件不存在"); err != nil {
			return false, err
		}
		return true, nil
	}
	if err := w.files.UpdateProcessingResult(job.KBID, job.FileID, "processing", "", ""); err != nil {
		return false, err
	}
	reader, err := w.storage.Read(file.Path)
	if err != nil {
		return w.failJob(job, "STORAGE_READ_FAILED", "读取原文件失败")
	}
	content, readErr := io.ReadAll(reader)
	closeErr := reader.Close()
	if readErr != nil {
		return w.failJob(job, "STORAGE_READ_FAILED", "读取原文件失败")
	}
	if closeErr != nil {
		return w.failJob(job, "STORAGE_READ_FAILED", "关闭原文件失败")
	}
	parsed, err := w.parser.Parse(ctx, ingestion.ParseInput{
		Filename: file.OriginalFilename,
		Content:  content,
		Path:     file.Path,
	})
	if err != nil {
		return w.failJob(job, "PARSER_FAILED", "文档解析失败")
	}
	object, err := w.storage.UploadReader(
		fmt.Sprintf("knowledge-internal/%s/%s/derived", job.KBID, job.FileID),
		"normalized.md",
		"text/markdown",
		bytes.NewReader([]byte(parsed.Markdown)),
		int64(len(parsed.Markdown)),
	)
	if err != nil {
		return w.failJob(job, "STORAGE_WRITE_FAILED", "保存解析结果失败")
	}
	if err := w.files.UpdateProcessingResult(job.KBID, job.FileID, "ready", object.Path, ""); err != nil {
		return false, err
	}
	if err := w.jobs.MarkSucceeded(job.JobID); err != nil {
		return false, err
	}
	return true, nil
}

func (w *DocumentWorker) failJob(job *entity.DocumentProcessingJob, code, message string) (bool, error) {
	if err := w.files.UpdateProcessingResult(job.KBID, job.FileID, "failed", "", message); err != nil {
		return false, err
	}
	if err := w.jobs.MarkFailed(job.JobID, code, message); err != nil {
		return false, err
	}
	return true, nil
}

func (w *DocumentWorker) handleMessage(ctx context.Context, message documentqueue.Message, workerID string, reclaimed bool) error {
	ack, err := w.processMessage(ctx, message, workerID, reclaimed)
	if err != nil {
		return err
	}
	if !ack {
		return nil
	}
	return w.queue.Ack(ctx, message.ID)
}

// Run blocks on Redis for new document jobs and starts a Redis-only Pending recovery loop.
func (w *DocumentWorker) Run(ctx context.Context, workerID string, options DocumentWorkerOptions) {
	options.applyDefaults()
	if err := w.queue.EnsureGroup(ctx); err != nil {
		logger.Error("文档处理消费者组初始化失败", zap.Error(err))
		return
	}
	if logger.Initialized() {
		logger.Info(
			"文档处理 Worker 已启动",
			zap.String("worker_id", workerID),
			zap.Duration("read_block", options.ReadBlock),
			zap.Duration("pending_check", options.PendingCheck),
			zap.Duration("pending_min_idle", options.PendingMinIdle),
			zap.Int64("pending_claim_size", options.PendingClaimSize),
		)
	}

	recoveryCtx, cancelRecovery := context.WithCancel(ctx)
	var recoveryWG sync.WaitGroup
	recoveryWG.Add(1)
	go func() {
		defer recoveryWG.Done()
		w.runPendingRecovery(recoveryCtx, workerID, options)
	}()

	defer func() {
		cancelRecovery()
		recoveryWG.Wait()
	}()

	for {
		message, err := w.queue.Consume(ctx, workerID, options.ReadBlock)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			logger.Warn("读取文档处理队列失败", zap.Error(err))
			if !waitForContext(ctx, time.Second) {
				return
			}
			continue
		}
		if message == nil {
			if ctx.Err() != nil {
				return
			}
			continue
		}
		if err := w.handleMessage(ctx, *message, workerID, false); err != nil {
			logger.Warn("文档处理消息执行失败", zap.String("job_id", message.JobID), zap.Error(err))
		}
	}
}

func (w *DocumentWorker) runPendingRecovery(ctx context.Context, workerID string, options DocumentWorkerOptions) {
	ticker := time.NewTicker(options.PendingCheck)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			messages, err := w.queue.ClaimStale(ctx, workerID, options.PendingMinIdle, options.PendingClaimSize)
			if err != nil {
				if ctx.Err() == nil {
					logger.Warn("领取超时文档消息失败", zap.Error(err))
				}
				continue
			}
			for _, message := range messages {
				if err := w.handleMessage(ctx, message, workerID, true); err != nil {
					logger.Warn("补偿文档处理消息失败", zap.String("job_id", message.JobID), zap.Error(err))
				}
			}
		}
	}
}

func (o *DocumentWorkerOptions) applyDefaults() {
	if o.ReadBlock <= 0 {
		o.ReadBlock = 5 * time.Second
	}
	if o.PendingCheck <= 0 {
		o.PendingCheck = time.Minute
	}
	if o.PendingMinIdle <= 0 {
		o.PendingMinIdle = 30 * time.Minute
	}
	if o.PendingClaimSize <= 0 {
		o.PendingClaimSize = 10
	}
}

func waitForContext(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
