package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/textproto"
	"strings"
	"testing"
	"time"

	"Qavor/internal/model/entity"
	documentqueue "Qavor/internal/queue"
	"Qavor/internal/repository"
)

// --- fakes ---

type fakeKBRepo struct {
	repository.KnowledgeBaseRepository // embed to satisfy interface without implementing every method
	found                              bool
}

func (f *fakeKBRepo) FindByKBID(string) (*entity.KnowledgeBase, error) {
	if f.found {
		return &entity.KnowledgeBase{}, nil
	}
	return nil, nil
}

type fakeFileRepo struct {
	repository.KnowledgeFileRepository
	created           *entity.KnowledgeFile
	status            string
	found             *entity.KnowledgeFile
	deletedWithChunks bool
}

func (f *fakeFileRepo) Create(file *entity.KnowledgeFile) error {
	f.created = file
	f.status = file.Status
	return nil
}

func (f *fakeFileRepo) DeleteByKBIDAndFileID(string, string) error { return nil }

func (f *fakeFileRepo) FindByKBIDAndFileID(string, string) (*entity.KnowledgeFile, error) {
	return f.found, nil
}

func (f *fakeFileRepo) DeleteWithChunks(context.Context, string, string) error {
	f.deletedWithChunks = true
	return nil
}

func (f *fakeFileRepo) TransitionStatus(_ context.Context, _, _ string, _ []string, to string, _ map[string]any) (bool, error) {
	f.status = to
	return true, nil
}

type fakeJobRepo struct {
	repository.DocumentProcessingJobRepository
	created   []*entity.DocumentProcessingJob
	cancelled []string
	cancelErr error
}

func (f *fakeJobRepo) CreateForFileTransition(_ context.Context, job *entity.DocumentProcessingJob, _ []string, queuedStatus string) (bool, error) {
	f.created = append(f.created, job)
	return true, nil
}

func (f *fakeJobRepo) MarkFailed(string, string, string) error { return nil }

func (f *fakeJobRepo) CancelPendingByFile(_ context.Context, _, fileID string) error {
	f.cancelled = append(f.cancelled, fileID)
	return f.cancelErr
}

func (f *fakeJobRepo) FindActiveByFileAndType(_ context.Context, _, _, _ string) (*entity.DocumentProcessingJob, error) {
	return nil, nil
}

type fakeStorage struct {
	ObjectStorage // embed unused interface
}

func (f *fakeStorage) Upload(string, *multipart.FileHeader) (*UploadedObject, error) {
	return &UploadedObject{Path: "test/path.txt", URL: "http://minio/test/path.txt", Filename: "guide.txt", Size: 5, ContentType: "text/plain"}, nil
}

func (f *fakeStorage) Delete(string) error { return nil }

type fakePreviewStorage struct {
	content string
}

func (f *fakePreviewStorage) Upload(string, *multipart.FileHeader) (*UploadedObject, error) {
	return nil, nil
}

func (f *fakePreviewStorage) UploadReader(string, string, string, io.Reader, int64) (*UploadedObject, error) {
	return nil, nil
}

func (f *fakePreviewStorage) Delete(string) error { return nil }

func (f *fakePreviewStorage) Read(string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader(f.content)), nil
}

type fakeChunkRepo struct {
	repository.KnowledgeChunkRepository
	chunks []*entity.KnowledgeChunk
}

func (f *fakeChunkRepo) FindByFileID(context.Context, string, string) ([]*entity.KnowledgeChunk, error) {
	return f.chunks, nil
}

type fakeQueue struct {
	published []documentqueue.Message
}

func (f *fakeQueue) EnsureGroup(_ context.Context) error { return nil }

func (f *fakeQueue) Publish(_ context.Context, msg documentqueue.Message) error {
	f.published = append(f.published, msg)
	return nil
}

func (f *fakeQueue) Consume(_ context.Context, _ string, _ time.Duration) (*documentqueue.Message, error) {
	return nil, nil
}

func (f *fakeQueue) Ack(_ context.Context, _ string) error { return nil }

func (f *fakeQueue) ClaimStale(_ context.Context, _ string, _ time.Duration, _ int64) ([]documentqueue.Message, error) {
	return nil, nil
}

// --- helpers ---

func multipartHeader(t *testing.T, filename, content string) *multipart.FileHeader {
	t.Helper()
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="file"; filename="`+filename+`"`)
	h.Set("Content-Type", "text/plain")
	return &multipart.FileHeader{
		Filename: filename,
		Header:   h,
		Size:     int64(len(content)),
	}
}

// --- test ---

func TestUploadQueuesParseJob(t *testing.T) {
	jobs := &fakeJobRepo{}
	queue := &fakeQueue{}
	svc := NewKnowledgeFileService(&fakeKBRepo{found: true}, &fakeFileRepo{}, jobs, &fakeStorage{}, queue)

	got, err := svc.Upload("kb-1", "", multipartHeader(t, "guide.txt", "hello"))
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != entity.FileParseQueued {
		t.Fatalf("status=%q want %q", got.Status, entity.FileParseQueued)
	}
	if len(jobs.created) != 1 || jobs.created[0].JobType != entity.JobTypeParse {
		t.Fatalf("jobs=%+v", jobs.created)
	}
	if len(queue.published) != 1 {
		t.Fatalf("messages=%d", len(queue.published))
	}
}

func TestDeleteIndexedFileDeletesChunksWithFileRecord(t *testing.T) {
	files := &fakeFileRepo{found: &entity.KnowledgeFile{
		FileID: "file-1", KBID: "kb-1", Status: entity.FileIndexed,
		Path: "original.pdf", MarkdownFile: "derived.md",
	}}
	svc := NewKnowledgeFileService(&fakeKBRepo{found: true}, files, &fakeJobRepo{}, &fakeStorage{}, &fakeQueue{})

	if err := svc.Delete("kb-1", "file-1"); err != nil {
		t.Fatal(err)
	}
	if !files.deletedWithChunks {
		t.Fatal("expected chunks and file record to be deleted together")
	}
}

func TestDeleteQueuedFileStopsWhenCancellationFails(t *testing.T) {
	files := &fakeFileRepo{found: &entity.KnowledgeFile{
		FileID: "file-1", KBID: "kb-1", Status: entity.FileIndexQueued, Path: "original.pdf",
	}}
	jobs := &fakeJobRepo{cancelErr: errors.New("database unavailable")}
	svc := NewKnowledgeFileService(&fakeKBRepo{found: true}, files, jobs, &fakeStorage{}, &fakeQueue{})

	if err := svc.Delete("kb-1", "file-1"); err == nil {
		t.Fatal("expected cancellation error")
	}
	if files.deletedWithChunks {
		t.Fatal("must not delete file data when pending job cancellation fails")
	}
}

func TestPreviewIncludesPersistedChunks(t *testing.T) {
	files := &fakeFileRepo{found: &entity.KnowledgeFile{
		FileID: "file-1", KBID: "kb-1", Path: "original.txt", MarkdownFile: "derived.md",
	}}
	chunks := &fakeChunkRepo{chunks: []*entity.KnowledgeChunk{
		{ChunkID: "chunk-1", ChunkIndex: 0, Content: "第一块", TokenCount: 2},
		{ChunkID: "chunk-2", ChunkIndex: 1, Content: "第二块", TokenCount: 2},
	}}
	svc := NewKnowledgeFileService(&fakeKBRepo{found: true}, files, &fakeJobRepo{}, &fakePreviewStorage{content: "# 文档"}, &fakeQueue{}, chunks)

	got, err := svc.Preview("kb-1", "file-1")
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(payload), `"chunks"`) {
		t.Fatalf("preview response does not include chunks: %s", payload)
	}
}

func TestRetryParseAllowsIndexedFile(t *testing.T) {
	queue := &fakeQueue{}
	jobs := &fakeJobRepo{}
	files := &fakeFileRepo{found: &entity.KnowledgeFile{
		FileID: "file-1", KBID: "kb-1", Status: entity.FileIndexed,
	}}
	svc := NewKnowledgeFileService(&fakeKBRepo{found: true}, files, jobs, &fakeStorage{}, queue)

	result, err := svc.RetryParse(context.Background(), "kb-1", "file-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.FileID != "file-1" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if len(jobs.created) != 1 || jobs.created[0].JobType != entity.JobTypeParse {
		t.Fatalf("expected one parse job, got %+v", jobs.created)
	}
	if len(queue.published) != 1 {
		t.Fatalf("messages=%d", len(queue.published))
	}
}

func TestRetryParseAllowsParsedFile(t *testing.T) {
	queue := &fakeQueue{}
	jobs := &fakeJobRepo{}
	files := &fakeFileRepo{found: &entity.KnowledgeFile{
		FileID: "file-1", KBID: "kb-1", Status: entity.FileParsed,
	}}
	svc := NewKnowledgeFileService(&fakeKBRepo{found: true}, files, jobs, &fakeStorage{}, queue)

	result, err := svc.RetryParse(context.Background(), "kb-1", "file-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.FileID != "file-1" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if len(jobs.created) != 1 || jobs.created[0].JobType != entity.JobTypeParse {
		t.Fatalf("expected one parse job, got %+v", jobs.created)
	}
	if len(queue.published) != 1 {
		t.Fatalf("messages=%d", len(queue.published))
	}
}

func TestRetryParseAllowsIndexFailedFile(t *testing.T) {
	queue := &fakeQueue{}
	jobs := &fakeJobRepo{}
	files := &fakeFileRepo{found: &entity.KnowledgeFile{
		FileID: "file-1", KBID: "kb-1", Status: entity.FileIndexFailed,
	}}
	svc := NewKnowledgeFileService(&fakeKBRepo{found: true}, files, jobs, &fakeStorage{}, queue)

	result, err := svc.RetryParse(context.Background(), "kb-1", "file-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.FileID != "file-1" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if len(jobs.created) != 1 || jobs.created[0].JobType != entity.JobTypeParse {
		t.Fatalf("expected one parse job, got %+v", jobs.created)
	}
	if len(queue.published) != 1 {
		t.Fatalf("messages=%d", len(queue.published))
	}
}

func TestRetryParseRejectsProcessingState(t *testing.T) {
	queue := &fakeQueue{}
	jobs := &fakeJobRepo{}
	files := &fakeFileRepo{found: &entity.KnowledgeFile{
		FileID: "file-1", KBID: "kb-1", Status: entity.FileParsing,
	}}
	svc := NewKnowledgeFileService(&fakeKBRepo{found: true}, files, jobs, &fakeStorage{}, queue)

	_, err := svc.RetryParse(context.Background(), "kb-1", "file-1")
	if err == nil {
		t.Fatal("expected error for parsing state")
	}
	if len(queue.published) != 0 {
		t.Fatalf("expected no published messages, got %d", len(queue.published))
	}
}
