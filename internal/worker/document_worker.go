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
	"Qavor/internal/rag"
	"Qavor/internal/repository"
	"Qavor/internal/service"
	"Qavor/pkg/logger"

	"go.uber.org/zap"
)

// DocumentWorkerOptions 文档处理 Worker 的配置选项。
type DocumentWorkerOptions struct {
	ReadBlock        time.Duration // 从 Redis Stream 读取新消息的阻塞时间
	PendingCheck     time.Duration // 定期检查 Pending 状态任务的时间间隔
	PendingMinIdle   time.Duration // Pending 任务被重新领取前的最小空闲时间
	PendingClaimSize int64         // 每次领取 Pending 任务的数量上限
}

// DocumentWorker 文档处理 Worker，负责从 Redis 队列消费文档处理任务。
// 处理流程：解析文档 -> 上传 Markdown -> RAG 索引（可选）-> 标记任务完成。
type DocumentWorker struct {
	queue   documentqueue.DocumentQueue                // 文档处理任务队列（Redis Stream）
	jobs    repository.DocumentProcessingJobRepository // 任务状态存储
	files   repository.KnowledgeFileRepository         // 知识文件存储
	storage service.ObjectStorage                      // 对象存储（MinIO）
	parser  *ingestion.Parser                          // 文档解析器
	indexer rag.DocumentIndexer                        // RAG 索引器（可选，Embedding 未配置时为 nil）
}

// NewDocumentWorker 创建文档处理 Worker 实例。
func NewDocumentWorker(queue documentqueue.DocumentQueue, jobs repository.DocumentProcessingJobRepository, files repository.KnowledgeFileRepository, storage service.ObjectStorage, parser *ingestion.Parser, indexer rag.DocumentIndexer) *DocumentWorker {
	return &DocumentWorker{queue: queue, jobs: jobs, files: files, storage: storage, parser: parser, indexer: indexer}
}

// processMessage 处理单条文档处理消息，返回 (是否确认消费, 错误)。
// 流程：领取/恢复任务 -> 验证任务状态 -> 按 JobType 分发。
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

	switch job.JobType {
	case entity.JobTypeParse:
		return w.processParseJob(ctx, job, file)
	case entity.JobTypeIndex:
		return w.processIndexJob(ctx, job, file)
	default:
		return w.failJob(job, "INVALID_JOB_TYPE", "未知的文档处理任务类型")
	}
}

// processParseJob 处理解析任务：parse_queued -> parsing -> 读取原文件 -> 解析 -> 上传 Markdown -> parsed。
func (w *DocumentWorker) processParseJob(ctx context.Context, job *entity.DocumentProcessingJob, file *entity.KnowledgeFile) (bool, error) {
	ok, err := w.files.TransitionStatus(ctx, job.KBID, job.FileID, []string{entity.FileParseQueued}, entity.FileParsing, nil)
	if err != nil {
		return false, err
	}
	if !ok {
		return w.failJob(job, "STATE_CONFLICT", "文件状态冲突，无法开始解析")
	}

	reader, err := w.storage.Read(file.Path)
	if err != nil {
		return w.failParseJob(job, "STORAGE_READ_FAILED", "读取原文件失败")
	}
	content, readErr := io.ReadAll(reader)
	closeErr := reader.Close()
	if readErr != nil {
		return w.failParseJob(job, "STORAGE_READ_FAILED", "读取原文件失败")
	}
	if closeErr != nil {
		return w.failParseJob(job, "STORAGE_READ_FAILED", "关闭原文件失败")
	}

	parsed, err := w.parser.Parse(ctx, ingestion.ParseInput{
		Filename: file.OriginalFilename,
		Content:  content,
		Path:     file.Path,
	})
	if err != nil {
		return w.failParseJob(job, "PARSER_FAILED", "文档解析失败")
	}

	object, err := w.storage.UploadReader(
		fmt.Sprintf("knowledge-internal/%s/%s/derived", job.KBID, job.FileID),
		"normalized.md",
		"text/markdown",
		bytes.NewReader([]byte(parsed.Markdown)),
		int64(len(parsed.Markdown)),
	)
	if err != nil {
		return w.failParseJob(job, "STORAGE_WRITE_FAILED", "保存解析结果失败")
	}

	ok, err = w.files.TransitionStatus(ctx, job.KBID, job.FileID, []string{entity.FileParsing}, entity.FileParsed, map[string]any{"markdown_file": object.Path, "error_message": ""})
	if err != nil {
		return false, err
	}
	if !ok {
		return w.failParseJob(job, "STATE_CONFLICT", "文件状态冲突，无法完成解析")
	}

	if err := w.jobs.MarkSucceeded(job.JobID); err != nil {
		return false, err
	}
	return true, nil
}

// processIndexJob 处理索引任务：index_queued -> indexing -> 读取 Markdown -> RAG 索引 -> indexed。
func (w *DocumentWorker) processIndexJob(ctx context.Context, job *entity.DocumentProcessingJob, file *entity.KnowledgeFile) (bool, error) {
	if w.indexer == nil {
		return w.failIndexJob(job, "EMBEDDING_NOT_CONFIGURED", "Embedding 模型未配置，无法入库")
	}

	ok, err := w.files.TransitionStatus(ctx, job.KBID, job.FileID, []string{entity.FileIndexQueued}, entity.FileIndexing, nil)
	if err != nil {
		return false, err
	}
	if !ok {
		return w.failIndexJob(job, "STATE_CONFLICT", "文件状态冲突，无法开始入库")
	}

	if file.MarkdownFile == "" {
		return w.failIndexJob(job, "MARKDOWN_MISSING", "文件未解析，无法入库")
	}

	mdReader, err := w.storage.Read(file.MarkdownFile)
	if err != nil {
		return w.failIndexJob(job, "STORAGE_READ_FAILED", "读取 Markdown 文件失败")
	}
	markdown, readErr := io.ReadAll(mdReader)
	closeErr := mdReader.Close()
	if readErr != nil {
		return w.failIndexJob(job, "STORAGE_READ_FAILED", "读取 Markdown 文件失败")
	}
	if closeErr != nil {
		return w.failIndexJob(job, "STORAGE_READ_FAILED", "关闭 Markdown 文件失败")
	}

	// 从任务的 ProcessingParams 中提取分块参数。
	chunkTokens, overlapTokens, chunkPreset := w.parseChunkParams(job)

	out, err := w.indexer.Index(ctx, rag.IndexInput{
		KBID:          job.KBID,
		FileID:        job.FileID,
		Filename:      file.OriginalFilename,
		Markdown:      string(markdown),
		ChunkTokens:   chunkTokens,
		OverlapTokens: overlapTokens,
		ChunkPreset:   chunkPreset,
	})
	if err != nil {
		if logger.Initialized() {
			logger.Warn("RAG 文档索引失败",
				zap.String("job_id", job.JobID),
				zap.String("kb_id", job.KBID),
				zap.String("file_id", job.FileID),
				zap.Error(err),
			)
		}
		return w.failIndexJob(job, "RAG_INDEX_FAILED", fmt.Sprintf("RAG 索引失败: %v", err))
	}

	// 从索引输出中更新分块数量和 Token 数量。
	chunkCount := len(out.Chunks)
	var tokenCount int64
	for _, c := range out.Chunks {
		tokenCount += int64(c.TokenCount)
	}
	ok, err = w.files.TransitionStatus(ctx, job.KBID, job.FileID, []string{entity.FileIndexing}, entity.FileIndexed, map[string]any{"chunk_count": chunkCount, "token_count": tokenCount, "error_message": ""})
	if err != nil {
		return false, err
	}
	if !ok {
		return w.failIndexJob(job, "STATE_CONFLICT", "文件状态冲突，无法完成入库")
	}

	if err := w.jobs.MarkSucceeded(job.JobID); err != nil {
		return false, err
	}
	return true, nil
}

// parseChunkParams 从任务的 ProcessingParams 中提取分块参数。
// 返回 chunkTokens、overlapTokens（-1 表示未设置）和 chunkPreset（空表示未设置）。
func (w *DocumentWorker) parseChunkParams(job *entity.DocumentProcessingJob) (chunkTokens, overlapTokens int, chunkPreset string) {
	if job.ProcessingParams == nil {
		return 0, -1, ""
	}
	params := map[string]any(job.ProcessingParams)
	if v, ok := params["chunk_preset_id"]; ok {
		if s, isStr := v.(string); isStr {
			chunkPreset = s
		}
	}
	if v, ok := params["chunk_token_num"]; ok {
		switch n := v.(type) {
		case float64:
			chunkTokens = int(n)
		case int:
			chunkTokens = n
		}
	}
	if v, ok := params["overlapped_percent"]; ok {
		switch p := v.(type) {
		case float64:
			if chunkTokens > 0 {
				overlapTokens = chunkTokens * int(p) / 100
			}
		case int:
			if chunkTokens > 0 {
				overlapTokens = chunkTokens * p / 100
			}
		}
	}
	return chunkTokens, overlapTokens, chunkPreset
}

// failJob 标记任务失败但不更新文件状态（用于通用错误）。
func (w *DocumentWorker) failJob(job *entity.DocumentProcessingJob, code, message string) (bool, error) {
	if err := w.jobs.MarkFailed(job.JobID, code, message); err != nil {
		return false, err
	}
	return true, nil
}

// failParseJob 标记解析任务失败并将文件状态转为 parse_failed。
func (w *DocumentWorker) failParseJob(job *entity.DocumentProcessingJob, code, message string) (bool, error) {
	transitioned, err := w.files.TransitionStatus(context.Background(), job.KBID, job.FileID, []string{entity.FileParseQueued, entity.FileParsing}, entity.FileParseFailed, map[string]any{"error_message": message})
	if err != nil {
		return false, err
	}
	if !transitioned {
		return false, fmt.Errorf("保存解析失败状态：文件状态冲突")
	}
	if err := w.jobs.MarkFailed(job.JobID, code, message); err != nil {
		return false, err
	}
	return true, nil
}

// failIndexJob 标记索引任务失败并将文件状态转为 index_failed。不清理 Markdown。
func (w *DocumentWorker) failIndexJob(job *entity.DocumentProcessingJob, code, message string) (bool, error) {
	transitioned, err := w.files.TransitionStatus(context.Background(), job.KBID, job.FileID, []string{entity.FileIndexQueued, entity.FileIndexing}, entity.FileIndexFailed, map[string]any{"error_message": message})
	if err != nil {
		return false, err
	}
	if !transitioned {
		return false, fmt.Errorf("保存索引失败状态：文件状态冲突")
	}
	if err := w.jobs.MarkFailed(job.JobID, code, message); err != nil {
		return false, err
	}
	return true, nil
}

// handleMessage 处理消息并在成功后确认消费。
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

// Run 启动文档处理 Worker，监听 Redis 队列中的新任务，并启动 Pending 状态任务的恢复循环。
// 当 Embedding 未配置时，RAG 索引步骤会被跳过，文档仅完成解析标记为成功。
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

// runPendingRecovery 定期检查并回收处于 Pending 状态超过指定时间的任务。
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

// applyDefaults 应用默认配置值。
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

// waitForContext 等待指定时间或上下文取消，返回是否超时。
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
