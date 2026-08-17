package service

import (
	"context"
	"testing"
	"time"

	"Qavor/internal/model/entity"
	documentqueue "Qavor/internal/queue"
	"Qavor/internal/repository"
)

type retryJobRepository struct {
	repository.DocumentProcessingJobRepository
	original *entity.DocumentProcessingJob
	created  *entity.DocumentProcessingJob
}

func (r *retryJobRepository) FindByJobID(string) (*entity.DocumentProcessingJob, error) {
	return r.original, nil
}

func (r *retryJobRepository) CreateForFileTransition(_ context.Context, job *entity.DocumentProcessingJob, _ []string, _ string) (bool, error) {
	r.created = job
	return true, nil
}

type retryFileRepository struct {
	repository.KnowledgeFileRepository
}

func (retryFileRepository) FindByKBIDAndFileID(_, _ string) (*entity.KnowledgeFile, error) {
	return &entity.KnowledgeFile{FileID: "file-1", KBID: "kb-1", Filename: "file.md"}, nil
}

func (retryFileRepository) TransitionStatus(context.Context, string, string, []string, string, map[string]any) (bool, error) {
	return true, nil
}

type retryQueue struct{ published []documentqueue.Message }

func (q *retryQueue) EnsureGroup(context.Context) error { return nil }
func (q *retryQueue) Publish(_ context.Context, message documentqueue.Message) error {
	q.published = append(q.published, message)
	return nil
}
func (q *retryQueue) Consume(context.Context, string, time.Duration) (*documentqueue.Message, error) {
	return nil, nil
}
func (q *retryQueue) Ack(context.Context, string) error { return nil }
func (q *retryQueue) ClaimStale(context.Context, string, time.Duration, int64) ([]documentqueue.Message, error) {
	return nil, nil
}

func TestRetryIndexJobPreservesProcessingParams(t *testing.T) {
	jobs := &retryJobRepository{original: &entity.DocumentProcessingJob{
		JobID: "job-1", KBID: "kb-1", FileID: "file-1", JobType: entity.JobTypeIndex,
		Status: entity.JobFailed, MaxAttempts: 1,
		ProcessingParams: entity.JSON{"chunk_token_num": 900, "overlapped_percent": 20},
	}}
	svc := NewProcessingJobService(jobs, retryFileRepository{}, &retryQueue{})

	if _, err := svc.Retry("job-1"); err != nil {
		t.Fatal(err)
	}
	if jobs.created == nil || jobs.created.ProcessingParams["chunk_token_num"] != 900 {
		t.Fatalf("retry params=%v", jobs.created.ProcessingParams)
	}
}
