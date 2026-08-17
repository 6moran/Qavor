package service

import (
	"context"
	"sync"
	"time"

	"Qavor/internal/model/entity"
	"Qavor/pkg/logger"

	"go.uber.org/zap"
)

// evaluationRunner 评估模块后台执行器。
// 轮询数据库中的待处理任务（pending/running 的数据集生成与评估运行），
// 启动时把进程遗留的 running 任务标记为 failed（生成任务可手动恢复）。
type evaluationRunner struct {
	svc    *evaluationService
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}

	mu      sync.Mutex
	running map[string]bool // 正在执行的任务 key：ds:<dataset_id> / run:<run_id>
}

// newEvaluationRunner 创建后台执行器。
func newEvaluationRunner(svc *evaluationService) *evaluationRunner {
	return &evaluationRunner{
		svc:     svc,
		running: make(map[string]bool),
	}
}

// Start 启动轮询循环。ctx 取消时退出。
func (r *evaluationRunner) Start(ctx context.Context) {
	r.ctx, r.cancel = context.WithCancel(ctx)
	r.done = make(chan struct{})
	go r.loop()
}

// Stop 停止轮询循环并等待退出。
func (r *evaluationRunner) Stop() {
	if r.cancel != nil {
		r.cancel()
	}
	if r.done != nil {
		<-r.done
	}
}

// loop 轮询待处理任务并调度执行。
func (r *evaluationRunner) loop() {
	defer close(r.done)
	r.recoverStaleTasks()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.ctx.Done():
			return
		case <-ticker.C:
			r.poll()
		}
	}
}

// recoverStaleTasks 启动时把遗留的 running 任务标记为 failed（进程重启后无法恢复内存态）。
// pending 任务保留，等待正常执行；生成任务支持通过 resume 接口重新提交。
func (r *evaluationRunner) recoverStaleTasks() {
	datasets, err := r.svc.repo.ListPendingDatasets()
	if err == nil {
		for _, d := range datasets {
			if d.BuildMetadata["status"] == datasetBuildRunning {
				r.markDatasetFailed(d.DatasetID, "任务因服务重启中断，可点击继续生成恢复")
			}
		}
	} else {
		logEvalError("扫描数据集任务失败", err)
	}

	runs, err := r.svc.repo.ListPendingRuns()
	if err == nil {
		for _, run := range runs {
			r.markRunFailed(run.RunID, "任务因服务重启中断")
		}
	} else {
		logEvalError("扫描评估运行失败", err)
	}
}

// poll 扫描并调度待处理任务。
func (r *evaluationRunner) poll() {
	datasets, err := r.svc.repo.ListPendingDatasets()
	if err != nil {
		logEvalError("扫描数据集任务失败", err)
	} else {
		for _, d := range datasets {
			key := "ds:" + d.DatasetID
			if r.tryAcquire(key) {
				go func(dataset *entity.EvaluationDataset) {
					defer r.release(key)
					r.svc.runGenerateTask(r.ctx, dataset)
				}(d)
			}
		}
	}

	runs, err := r.svc.repo.ListPendingRuns()
	if err != nil {
		logEvalError("扫描评估运行失败", err)
	} else {
		for _, run := range runs {
			key := "run:" + run.RunID
			if r.tryAcquire(key) {
				go func(run *entity.EvaluationRun) {
					defer r.release(key)
					r.svc.runEvaluationTask(r.ctx, run)
				}(run)
			}
		}
	}
}

// tryAcquire 尝试占用任务执行槽，避免重复调度。
func (r *evaluationRunner) tryAcquire(key string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.running[key] {
		return false
	}
	r.running[key] = true
	return true
}

// release 释放任务执行槽。
func (r *evaluationRunner) release(key string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.running, key)
}

// markDatasetFailed 将数据集生成任务标记为失败。
func (r *evaluationRunner) markDatasetFailed(datasetID, message string) {
	dataset, err := r.svc.repo.FindDatasetByID(datasetID)
	if err != nil || dataset == nil {
		return
	}
	metadata := dataset.BuildMetadata
	metadata["status"] = datasetBuildFailed
	metadata["message"] = message
	dataset.BuildMetadata = metadata
	if err := r.svc.repo.UpdateDataset(dataset); err != nil {
		logEvalError("标记数据集任务失败", err)
	}
}

// markRunFailed 将评估运行标记为失败。
func (r *evaluationRunner) markRunFailed(runID, message string) {
	run, err := r.svc.repo.FindRunByID(runID)
	if err != nil || run == nil {
		return
	}
	now := time.Now()
	run.Status = runStatusFailed
	run.CompletedAt = &now
	run.Message = message
	if err := r.svc.repo.UpdateRun(run); err != nil {
		logEvalError("标记评估运行失败", err)
	}
}

// logRunner 记录执行器调度日志。
func logRunner(msg string, fields ...zap.Field) {
	if logger.Initialized() {
		logger.Info(msg, fields...)
	}
}
