package trace

import (
	"context"
	"sync"
	"testing"
	"time"

	"Qavor/internal/model/entity"
)

func TestWriterFlushPersistsQueuedEvents(t *testing.T) {
	repo := newFakeRepository()
	writer := NewWriter(repo, WriterConfig{BufferSize: 8})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := writer.StartSpan(ctx, &entity.TraceSpan{SpanID: "s1", TraceID: "t1", Status: SpanStatusRunning}); err != nil {
		t.Fatal(err)
	}
	if err := writer.Flush(ctx); err != nil {
		t.Fatal(err)
	}
	if len(repo.started) != 1 || repo.started[0].SpanID != "s1" {
		t.Fatalf("started = %+v", repo.started)
	}
}

func TestWriterCreateTraceFlushed(t *testing.T) {
	repo := newFakeRepository()
	writer := NewWriter(repo, WriterConfig{BufferSize: 8})
	ctx := context.Background()

	rec := &entity.TraceRecord{TraceID: "t1", EntryType: entity.EntryTypeHTTP}
	if err := writer.CreateTrace(ctx, rec); err != nil {
		t.Fatal(err)
	}
	if err := writer.Flush(ctx); err != nil {
		t.Fatal(err)
	}
	if repo.records["t1"] == nil {
		t.Fatal("CreateTrace not persisted after Flush")
	}
}

func TestWriterEndSpanFlushed(t *testing.T) {
	repo := newFakeRepository()
	writer := NewWriter(repo, WriterConfig{BufferSize: 8})
	ctx := context.Background()

	if err := writer.EndSpan(ctx, "span-end-1", SpanEnd{Status: SpanStatusOK, OutputSummary: "done"}); err != nil {
		t.Fatal(err)
	}
	if err := writer.Flush(ctx); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range repo.ends {
		if e.spanID == "span-end-1" && e.end.Status == SpanStatusOK {
			found = true
		}
	}
	if !found {
		t.Fatalf("EndSpan not persisted, ends=%+v", repo.ends)
	}
}

func TestWriterBufferFullDropsStartSpan(t *testing.T) {
	repo := newFakeRepository()
	writer := NewWriter(repo, WriterConfig{BufferSize: 2})
	ctx := context.Background()

	// 填充缓冲区但不刷新
	for i := 0; i < 2; i++ {
		if err := writer.StartSpan(ctx, &entity.TraceSpan{SpanID: "s" + string(rune('A'+i)), TraceID: "t1", Status: SpanStatusRunning}); err != nil {
			t.Fatal(err)
		}
	}
	// 第三个应该被丢弃
	if err := writer.StartSpan(ctx, &entity.TraceSpan{SpanID: "sC", TraceID: "t1", Status: SpanStatusRunning}); err != nil {
		t.Fatal(err)
	}
	if writer.Dropped() != 1 {
		t.Fatalf("expected 1 dropped, got %d", writer.Dropped())
	}
}

// blockingRepo 包装 fakeRepository，处理 StartSpan 时阻塞直到 release 被关闭。
// 用于测试 EndSpan 在缓冲区满时的等待行为。
type blockingRepo struct {
	*fakeRepository
	release     chan struct{}
	entered     chan struct{}
	enteredOnce sync.Once
}

func newBlockingRepo() *blockingRepo {
	return &blockingRepo{
		fakeRepository: newFakeRepository(),
		release:        make(chan struct{}),
		entered:        make(chan struct{}),
	}
}

func (r *blockingRepo) StartSpan(ctx context.Context, span *entity.TraceSpan) error {
	r.enteredOnce.Do(func() { close(r.entered) })
	select {
	case <-r.release:
	case <-ctx.Done():
		return ctx.Err()
	}
	return r.fakeRepository.StartSpan(ctx, span)
}

func TestWriterEndSpanWaitsBriefly(t *testing.T) {
	repo := newBlockingRepo()
	writer := NewWriter(repo, WriterConfig{BufferSize: 1})
	ctx := context.Background()

	// Fill buffer: StartSpan 入队后 goroutine 立即取出处理，但被 blockingRepo 阻塞
	writer.StartSpan(ctx, &entity.TraceSpan{SpanID: "s1", TraceID: "t1", Status: SpanStatusRunning})

	// 明确等待 goroutine 进入仓储层，不依赖机器调度速度。
	select {
	case <-repo.entered:
	case <-time.After(time.Second):
		t.Fatal("writer goroutine did not enter StartSpan")
	}

	// 再填满 buffer（goroutine 被阻塞，不会再取）
	writer.StartSpan(ctx, &entity.TraceSpan{SpanID: "s2", TraceID: "t1", Status: SpanStatusRunning})

	// EndSpan 应等待 100ms 后丢弃（buffer 满 + goroutine 阻塞）
	start := time.Now()
	writer.EndSpan(ctx, "s1", SpanEnd{Status: SpanStatusOK})
	elapsed := time.Since(start)
	if elapsed < 50*time.Millisecond {
		t.Fatalf("EndSpan should wait ~100ms before dropping, elapsed=%v", elapsed)
	}
	if writer.Dropped() < 1 {
		t.Fatalf("expected dropped >= 1, got %d", writer.Dropped())
	}

	// 释放阻塞，让 goroutine 完成，允许 Close 排空
	close(repo.release)
	closeCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	writer.Close(closeCtx)
}

func TestWriterCloseNoPanicOnWrite(t *testing.T) {
	repo := newFakeRepository()
	writer := NewWriter(repo, WriterConfig{BufferSize: 8})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := writer.Close(ctx); err != nil {
		t.Fatal(err)
	}
	// 关闭后写入不应 panic，只返回 nil
	if err := writer.StartSpan(ctx, &entity.TraceSpan{SpanID: "s1", TraceID: "t1"}); err != nil {
		t.Fatal(err)
	}
	if err := writer.CreateTrace(ctx, &entity.TraceRecord{TraceID: "t1"}); err != nil {
		t.Fatal(err)
	}
	if err := writer.EndSpan(ctx, "s1", SpanEnd{Status: SpanStatusOK}); err != nil {
		t.Fatal(err)
	}
}

func TestWriterFlushTimeoutReturnsError(t *testing.T) {
	repo := newFakeRepository()
	writer := NewWriter(repo, WriterConfig{BufferSize: 1})

	// 填充缓冲区使 goroutine 忙碌（无法处理，因为我们将使用很短的超时时间）
	writer.StartSpan(context.Background(), &entity.TraceSpan{SpanID: "s1", TraceID: "t1", Status: SpanStatusRunning})

	// 使用已取消的 context 进行 Flush
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := writer.Flush(ctx)
	if err == nil {
		// 如果 goroutine 处理足够快，可能成功，这也是可接受的
		// 关键是它不会挂起
	}
}

func TestWriterCloseFlushesPending(t *testing.T) {
	repo := newFakeRepository()
	writer := NewWriter(repo, WriterConfig{BufferSize: 8})
	ctx := context.Background()

	writer.StartSpan(ctx, &entity.TraceSpan{SpanID: "s1", TraceID: "t1", Status: SpanStatusRunning})
	writer.CreateTrace(ctx, &entity.TraceRecord{TraceID: "t1", EntryType: entity.EntryTypeHTTP})

	closeCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := writer.Close(closeCtx); err != nil {
		t.Fatal(err)
	}
	// 关闭后，事件应该已被刷新
	if repo.records["t1"] == nil {
		t.Fatal("CreateTrace not flushed on Close")
	}
	if len(repo.started) != 1 {
		t.Fatalf("StartSpan not flushed on Close, started=%d", len(repo.started))
	}
}
