package worker

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"Qavor/internal/ingestion"
	"Qavor/internal/model/entity"
	documentqueue "Qavor/internal/queue"
	"Qavor/internal/rag"
	"Qavor/internal/repository"
	"Qavor/internal/service"
)

// --- fakes for worker tests ---

type wFakeQueue struct{}

func (wFakeQueue) EnsureGroup(_ context.Context) error { return nil }
func (wFakeQueue) Publish(_ context.Context, _ documentqueue.Message) error {
	return nil
}
func (wFakeQueue) Consume(_ context.Context, _ string, _ time.Duration) (*documentqueue.Message, error) {
	return nil, nil
}
func (wFakeQueue) Ack(_ context.Context, _ string) error { return nil }
func (wFakeQueue) ClaimStale(_ context.Context, _ string, _ time.Duration, _ int64) ([]documentqueue.Message, error) {
	return nil, nil
}

type wFakeJobs struct {
	repository.DocumentProcessingJobRepository
	mu      sync.Mutex
	status  string
	jobType string
}

func (f *wFakeJobs) ClaimByJobID(_ context.Context, jobID, workerID string) (*entity.DocumentProcessingJob, error) {
	return &entity.DocumentProcessingJob{
		JobID:    jobID,
		WorkerID: workerID,
		Status:   entity.JobRunning,
		JobType:  f.jobType,
		Attempt:  1,
	}, nil
}

func (f *wFakeJobs) MarkSucceeded(string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.status = entity.JobSucceeded
	return nil
}
func (f *wFakeJobs) MarkFailed(string, string, string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.status = entity.JobFailed
	return nil
}

type wFakeFiles struct {
	repository.KnowledgeFileRepository
	mu            sync.Mutex
	status        string
	markdownPath  string
	failOnStatus  string
	transitionErr error
}

func (f *wFakeFiles) FindByKBIDAndFileID(string, string) (*entity.KnowledgeFile, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return &entity.KnowledgeFile{
		FileID:           "file-1",
		KBID:             "kb-1",
		Path:             "original.txt",
		OriginalFilename: "test.txt",
		MarkdownFile:     f.markdownPath,
		Status:           f.status,
	}, nil
}

func (f *wFakeFiles) TransitionStatus(_ context.Context, _, _ string, _ []string, to string, updates map[string]any) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if to == f.failOnStatus {
		return false, f.transitionErr
	}
	f.status = to
	if v, ok := updates["markdown_file"]; ok {
		f.markdownPath = v.(string)
	}
	return true, nil
}

type wFakeStorage struct {
	service.ObjectStorage
	content string
}

func (f *wFakeStorage) Read(path string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader(f.content)), nil
}

func (f *wFakeStorage) UploadReader(_, _, _ string, reader io.Reader, _ int64) (*service.UploadedObject, error) {
	_, _ = io.ReadAll(reader)
	return &service.UploadedObject{Path: "derived/normalized.md"}, nil
}

func (f *wFakeStorage) Delete(string) error { return nil }

type wFakeIndexer struct {
	calls int
}

func (i *wFakeIndexer) Index(_ context.Context, in rag.IndexInput) (*rag.IndexOutput, error) {
	i.calls++
	return &rag.IndexOutput{
		Chunks: []rag.IndexedChunk{
			{ChunkID: "chunk-1", Content: in.Markdown, TokenCount: 10},
			{ChunkID: "chunk-2", Content: in.Markdown, TokenCount: 10},
		},
	}, nil
}

type failingIndexer struct{}

func (f *failingIndexer) Index(_ context.Context, _ rag.IndexInput) (*rag.IndexOutput, error) {
	return nil, io.ErrUnexpectedEOF
}

type concurrentQueue struct {
	jobs            chan documentqueue.Message
	consumerIDs     chan string
	recoveryStarted chan struct{}
	recoveryOnce    sync.Once
	recoveryCalls   atomic.Int32
}

func newConcurrentQueue(messages ...documentqueue.Message) *concurrentQueue {
	q := &concurrentQueue{
		jobs:            make(chan documentqueue.Message, len(messages)),
		consumerIDs:     make(chan string, 8),
		recoveryStarted: make(chan struct{}),
	}
	for _, message := range messages {
		q.jobs <- message
	}
	return q
}

func (q *concurrentQueue) EnsureGroup(context.Context) error                    { return nil }
func (q *concurrentQueue) Publish(context.Context, documentqueue.Message) error { return nil }
func (q *concurrentQueue) Consume(ctx context.Context, consumer string, _ time.Duration) (*documentqueue.Message, error) {
	select {
	case q.consumerIDs <- consumer:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	select {
	case message := <-q.jobs:
		return &message, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
func (q *concurrentQueue) Ack(context.Context, string) error { return nil }
func (q *concurrentQueue) ClaimStale(ctx context.Context, _ string, _ time.Duration, _ int64) ([]documentqueue.Message, error) {
	q.recoveryCalls.Add(1)
	q.recoveryOnce.Do(func() { close(q.recoveryStarted) })
	<-ctx.Done()
	return nil, ctx.Err()
}

type blockingStorage struct {
	service.ObjectStorage
	started chan struct{}
	release <-chan struct{}
}

func (s *blockingStorage) Read(string) (io.ReadCloser, error) {
	s.started <- struct{}{}
	<-s.release
	return io.NopCloser(strings.NewReader("content")), nil
}

func (s *blockingStorage) UploadReader(_, _, _ string, reader io.Reader, _ int64) (*service.UploadedObject, error) {
	_, _ = io.ReadAll(reader)
	return &service.UploadedObject{Path: "derived/normalized.md"}, nil
}

type quotaQueue struct {
	normal           chan documentqueue.Message
	allowRecovery    chan struct{}
	recoveryClaimed  chan struct{}
	recoveryReturned atomic.Bool
}

func (q *quotaQueue) EnsureGroup(context.Context) error                    { return nil }
func (q *quotaQueue) Publish(context.Context, documentqueue.Message) error { return nil }
func (q *quotaQueue) Consume(ctx context.Context, _ string, _ time.Duration) (*documentqueue.Message, error) {
	select {
	case message := <-q.normal:
		return &message, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
func (q *quotaQueue) Ack(context.Context, string) error { return nil }
func (q *quotaQueue) ClaimStale(ctx context.Context, _ string, _ time.Duration, _ int64) ([]documentqueue.Message, error) {
	select {
	case <-q.allowRecovery:
		if q.recoveryReturned.CompareAndSwap(false, true) {
			close(q.recoveryClaimed)
			return []documentqueue.Message{{ID: "recovery-message", JobID: "recovery-job"}}, nil
		}
		return nil, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

type quotaJobs struct {
	repository.DocumentProcessingJobRepository
}

func (quotaJobs) ClaimByJobID(_ context.Context, jobID, workerID string) (*entity.DocumentProcessingJob, error) {
	return quotaJob(jobID, workerID), nil
}

func (quotaJobs) ReclaimByJobID(_ context.Context, jobID, workerID string) (*entity.DocumentProcessingJob, error) {
	return quotaJob(jobID, workerID), nil
}

func quotaJob(jobID, workerID string) *entity.DocumentProcessingJob {
	return &entity.DocumentProcessingJob{
		JobID:    jobID,
		KBID:     "kb-1",
		FileID:   jobID,
		WorkerID: workerID,
		Status:   entity.JobRunning,
		JobType:  entity.JobTypeParse,
		Attempt:  1,
	}
}

func (quotaJobs) MarkSucceeded(string) error              { return nil }
func (quotaJobs) MarkFailed(string, string, string) error { return nil }

type quotaFiles struct {
	repository.KnowledgeFileRepository
	mu     sync.Mutex
	status map[string]string
}

func (f *quotaFiles) FindByKBIDAndFileID(_ string, fileID string) (*entity.KnowledgeFile, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return &entity.KnowledgeFile{
		KBID:             "kb-1",
		FileID:           fileID,
		Path:             fileID + ".txt",
		OriginalFilename: fileID + ".txt",
		Status:           f.status[fileID],
	}, nil
}

func (f *quotaFiles) TransitionStatus(_ context.Context, _ string, fileID string, from []string, to string, _ map[string]any) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, expected := range from {
		if f.status[fileID] == expected {
			f.status[fileID] = to
			return true, nil
		}
	}
	return false, nil
}

type quotaStorage struct {
	service.ObjectStorage
	started chan string
	release <-chan struct{}
	active  atomic.Int32
	max     atomic.Int32
}

type duplicateRecoveryQueue struct {
	claimCalls  atomic.Int32
	claimedMany chan struct{}
	claimOnce   sync.Once
	acked       atomic.Int32
}

func (q *duplicateRecoveryQueue) EnsureGroup(context.Context) error                    { return nil }
func (q *duplicateRecoveryQueue) Publish(context.Context, documentqueue.Message) error { return nil }
func (q *duplicateRecoveryQueue) Consume(ctx context.Context, _ string, _ time.Duration) (*documentqueue.Message, error) {
	select {
	case <-time.After(time.Millisecond):
		return nil, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
func (q *duplicateRecoveryQueue) Ack(context.Context, string) error {
	q.acked.Add(1)
	return nil
}
func (q *duplicateRecoveryQueue) ClaimStale(context.Context, string, time.Duration, int64) ([]documentqueue.Message, error) {
	calls := q.claimCalls.Add(1)
	if calls >= 3 {
		q.claimOnce.Do(func() { close(q.claimedMany) })
	}
	if calls > 3 {
		return nil, nil
	}
	return []documentqueue.Message{{ID: "duplicate-recovery-message", JobID: "duplicate-recovery-job"}}, nil
}

type duplicateRecoveryJobs struct {
	repository.DocumentProcessingJobRepository
	reclaimed atomic.Int32
}

func (j *duplicateRecoveryJobs) ReclaimByJobID(_ context.Context, jobID, workerID string) (*entity.DocumentProcessingJob, error) {
	j.reclaimed.Add(1)
	return quotaJob(jobID, workerID), nil
}
func (*duplicateRecoveryJobs) MarkSucceeded(string) error              { return nil }
func (*duplicateRecoveryJobs) MarkFailed(string, string, string) error { return nil }

func (s *quotaStorage) Read(path string) (io.ReadCloser, error) {
	active := s.active.Add(1)
	for {
		currentMax := s.max.Load()
		if active <= currentMax || s.max.CompareAndSwap(currentMax, active) {
			break
		}
	}
	s.started <- path
	<-s.release
	s.active.Add(-1)
	return io.NopCloser(strings.NewReader("content")), nil
}

func (s *quotaStorage) UploadReader(_, _, _ string, reader io.Reader, _ int64) (*service.UploadedObject, error) {
	_, _ = io.ReadAll(reader)
	return &service.UploadedObject{Path: "derived/normalized.md"}, nil
}

// --- helpers ---

func parseWorkerFixture() *DocumentWorker {
	return &DocumentWorker{
		queue:   wFakeQueue{},
		jobs:    &wFakeJobs{jobType: entity.JobTypeParse},
		files:   &wFakeFiles{status: entity.FileParseQueued},
		storage: &wFakeStorage{content: "original file content"},
		parser:  ingestion.NewParser(nil), // handles .txt without python parser
		indexer: nil,
	}
}

func indexWorkerFixture() *DocumentWorker {
	return &DocumentWorker{
		queue:   wFakeQueue{},
		jobs:    &wFakeJobs{jobType: entity.JobTypeIndex},
		files:   &wFakeFiles{status: entity.FileIndexQueued, markdownPath: "derived/normalized.md"},
		storage: &wFakeStorage{content: "# heading\nparsed content"},
		parser:  ingestion.NewParser(nil),
		indexer: &wFakeIndexer{},
	}
}

// --- tests ---

func TestParseJobStopsAtParsedWithoutIndexing(t *testing.T) {
	w := parseWorkerFixture()
	indexer := &wFakeIndexer{}
	w.indexer = indexer

	ack, err := w.processMessage(context.Background(), documentqueue.Message{JobID: "parse-1"}, "w-1", false)
	if err != nil || !ack {
		t.Fatalf("ack=%v err=%v", ack, err)
	}
	if indexer.calls != 0 {
		t.Fatalf("index calls=%d", indexer.calls)
	}
	if w.files.(*wFakeFiles).status != entity.FileParsed {
		t.Fatalf("file status=%q want %q", w.files.(*wFakeFiles).status, entity.FileParsed)
	}
}

func TestIndexJobUsesMarkdownWithoutParsing(t *testing.T) {
	w := indexWorkerFixture()

	// We track parse calls by checking that the MarkdownFile is read, not the original path.
	// The fake storage always returns the same content, so we verify via the indexer call.
	ack, err := w.processMessage(context.Background(), documentqueue.Message{JobID: "index-1"}, "w-1", false)
	if err != nil || !ack {
		t.Fatalf("ack=%v err=%v", ack, err)
	}
	if w.files.(*wFakeFiles).status != entity.FileIndexed {
		t.Fatalf("file status=%q want %q", w.files.(*wFakeFiles).status, entity.FileIndexed)
	}
	if w.indexer.(*wFakeIndexer).calls != 1 {
		t.Fatalf("index calls=%d want 1", w.indexer.(*wFakeIndexer).calls)
	}
}

func TestIndexFailureTransitionsFileToIndexFailed(t *testing.T) {
	w := indexWorkerFixture()
	w.indexer = &failingIndexer{}

	ack, err := w.processMessage(context.Background(), documentqueue.Message{JobID: "index-fail"}, "w-1", false)
	if err != nil || !ack {
		t.Fatalf("ack=%v err=%v", ack, err)
	}
	if w.files.(*wFakeFiles).status != entity.FileIndexFailed {
		t.Fatalf("file status=%q want %q", w.files.(*wFakeFiles).status, entity.FileIndexFailed)
	}
}

func TestIndexMissingMarkdownFails(t *testing.T) {
	w := indexWorkerFixture()
	w.files.(*wFakeFiles).markdownPath = ""

	ack, err := w.processMessage(context.Background(), documentqueue.Message{JobID: "index-nomd"}, "w-1", false)
	if err != nil || !ack {
		t.Fatalf("ack=%v err=%v", ack, err)
	}
	if w.files.(*wFakeFiles).status != entity.FileIndexFailed {
		t.Fatalf("file status=%q want %q", w.files.(*wFakeFiles).status, entity.FileIndexFailed)
	}
}

func TestUnknownJobTypeFails(t *testing.T) {
	w := &DocumentWorker{
		queue:   wFakeQueue{},
		jobs:    &wFakeJobs{jobType: "unknown"},
		files:   &wFakeFiles{status: entity.FileUploaded},
		storage: &wFakeStorage{content: ""},
		parser:  ingestion.NewParser(nil),
		indexer: nil,
	}

	ack, err := w.processMessage(context.Background(), documentqueue.Message{JobID: "unknown-1"}, "w-1", false)
	if err != nil || !ack {
		t.Fatalf("ack=%v err=%v", ack, err)
	}
}

func TestIndexFailureDoesNotAckWhenFailureStateCannotPersist(t *testing.T) {
	w := indexWorkerFixture()
	w.indexer = &failingIndexer{}
	files := w.files.(*wFakeFiles)
	files.failOnStatus = entity.FileIndexFailed
	files.transitionErr = errors.New("database unavailable")

	ack, err := w.processMessage(context.Background(), documentqueue.Message{JobID: "index-db-fail"}, "w-1", false)
	if err == nil || ack {
		t.Fatalf("ack=%v err=%v", ack, err)
	}
	if w.jobs.(*wFakeJobs).status == entity.JobFailed {
		t.Fatal("job must remain recoverable when file failure state cannot persist")
	}
}

func TestRunStartsBoundedConsumersAndOneRecoveryCoordinator(t *testing.T) {
	queue := newConcurrentQueue(
		documentqueue.Message{ID: "message-1", JobID: "job-1"},
		documentqueue.Message{ID: "message-2", JobID: "job-2"},
	)
	release := make(chan struct{})
	storage := &blockingStorage{started: make(chan struct{}, 2), release: release}
	worker := &DocumentWorker{
		queue:   queue,
		jobs:    &wFakeJobs{jobType: entity.JobTypeParse},
		files:   &wFakeFiles{status: entity.FileParseQueued},
		storage: storage,
		parser:  ingestion.NewParser(nil),
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		worker.Run(ctx, "consumer", DocumentWorkerOptions{
			ConsumerCount: 2,
			PendingCheck:  time.Millisecond,
		})
	}()

	for range 2 {
		select {
		case <-storage.started:
		case <-time.After(time.Second):
			t.Fatal("two parse jobs did not enter processing concurrently")
		}
	}

	consumerIDs := map[string]bool{}
	for range 2 {
		select {
		case consumerID := <-queue.consumerIDs:
			consumerIDs[consumerID] = true
		case <-time.After(time.Second):
			t.Fatal("consumer did not start")
		}
	}
	if !consumerIDs["consumer-0"] || !consumerIDs["consumer-1"] {
		t.Fatalf("consumer IDs = %v, want consumer-0 and consumer-1", consumerIDs)
	}
	select {
	case <-queue.recoveryStarted:
	case <-time.After(time.Second):
		t.Fatal("pending recovery coordinator did not start")
	}
	if calls := queue.recoveryCalls.Load(); calls != 1 {
		t.Fatalf("pending recovery loops = %d, want 1", calls)
	}

	close(release)
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("worker did not stop after context cancellation")
	}
}

func TestRunLimitsNormalAndRecoveredMessagesToConsumerCount(t *testing.T) {
	queue := &quotaQueue{
		normal:          make(chan documentqueue.Message, 1),
		allowRecovery:   make(chan struct{}),
		recoveryClaimed: make(chan struct{}),
	}
	queue.normal <- documentqueue.Message{ID: "normal-message", JobID: "normal-job"}
	release := make(chan struct{})
	storage := &quotaStorage{started: make(chan string, 2), release: release}
	worker := &DocumentWorker{
		queue:   queue,
		jobs:    quotaJobs{},
		files:   &quotaFiles{status: map[string]string{"normal-job": entity.FileParseQueued, "recovery-job": entity.FileParseQueued}},
		storage: storage,
		parser:  ingestion.NewParser(nil),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer close(release)
	done := make(chan struct{})
	go func() {
		defer close(done)
		worker.Run(ctx, "consumer", DocumentWorkerOptions{
			ConsumerCount: 1,
			PendingCheck:  time.Millisecond,
		})
	}()

	select {
	case path := <-storage.started:
		if path != "normal-job.txt" {
			t.Fatalf("first parsed path = %q, want normal-job.txt", path)
		}
	case <-time.After(time.Second):
		t.Fatal("normal message did not enter processing")
	}
	close(queue.allowRecovery)
	select {
	case <-queue.recoveryClaimed:
	case <-time.After(time.Second):
		t.Fatal("recovery message was not claimed")
	}

	select {
	case path := <-storage.started:
		t.Fatalf("recovered message %q began while the sole consumer handled normal work", path)
	case <-time.After(25 * time.Millisecond):
	}
	if got := storage.max.Load(); got != 1 {
		t.Fatalf("maximum concurrent handlers = %d, want 1", got)
	}

	release <- struct{}{}
	select {
	case path := <-storage.started:
		if path != "recovery-job.txt" {
			t.Fatalf("second parsed path = %q, want recovery-job.txt", path)
		}
	case <-time.After(time.Second):
		t.Fatal("recovered message was not handled after the consumer became free")
	}
	if got := storage.max.Load(); got != 1 {
		t.Fatalf("maximum concurrent handlers = %d, want 1", got)
	}

	release <- struct{}{}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("worker did not stop after context cancellation")
	}
}

func TestRunSkipsStaleClaimsForRecoveryMessageAlreadyInFlight(t *testing.T) {
	queue := &duplicateRecoveryQueue{claimedMany: make(chan struct{})}
	jobs := &duplicateRecoveryJobs{}
	release := make(chan struct{})
	storage := &blockingStorage{started: make(chan struct{}, 2), release: release}
	worker := &DocumentWorker{
		queue:   queue,
		jobs:    jobs,
		files:   &quotaFiles{status: map[string]string{"duplicate-recovery-job": entity.FileParseQueued}},
		storage: storage,
		parser:  ingestion.NewParser(nil),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		worker.Run(ctx, "consumer", DocumentWorkerOptions{
			ConsumerCount:  1,
			PendingCheck:   time.Millisecond,
			PendingMinIdle: time.Millisecond,
		})
	}()

	select {
	case <-storage.started:
	case <-time.After(time.Second):
		t.Fatal("recovered message did not enter processing")
	}
	select {
	case <-queue.claimedMany:
	case <-time.After(time.Second):
		t.Fatal("recovery did not perform repeated stale claims while processing was blocked")
	}
	if got := jobs.reclaimed.Load(); got != 1 {
		t.Fatalf("recovered handler entries while first attempt is in flight = %d, want 1", got)
	}

	release <- struct{}{}
	select {
	case <-storage.started:
		t.Fatal("same recovered message was processed twice")
	case <-time.After(25 * time.Millisecond):
	}
	if got := jobs.reclaimed.Load(); got != 1 {
		t.Fatalf("recovered handler entries after completion = %d, want 1", got)
	}
	if got := queue.acked.Load(); got != 1 {
		t.Fatalf("acks = %d, want 1", got)
	}

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("worker did not stop after context cancellation")
	}
}

func TestDocumentWorkerOptionsDefaultConsumerCount(t *testing.T) {
	options := DocumentWorkerOptions{}
	options.applyDefaults()
	if options.ConsumerCount != 1 {
		t.Fatalf("ConsumerCount = %d, want 1", options.ConsumerCount)
	}
}
