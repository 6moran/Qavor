package agent

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"Qavor/internal/model/entity"
	"Qavor/internal/trace"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
)

// fakeSpanWriter 仅实现 trace.SpanWriter，记录 CreateTrace/StartSpan/EndSpan 调用
type fakeSpanWriter struct {
	mu      sync.Mutex
	records []*entity.TraceRecord
	started []*entity.TraceSpan
	ends    []fakeEnd
}

type fakeEnd struct {
	spanID string
	end    trace.SpanEnd
}

func (f *fakeSpanWriter) CreateTrace(_ context.Context, r *entity.TraceRecord) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := *r
	f.records = append(f.records, &cp)
	return nil
}
func (f *fakeSpanWriter) UpdateTraceMetadata(context.Context, string, trace.TraceMetadata) error {
	return nil
}

func (f *fakeSpanWriter) StartSpan(_ context.Context, s *entity.TraceSpan) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := *s
	f.started = append(f.started, &cp)
	return nil
}

func (f *fakeSpanWriter) EndSpan(_ context.Context, spanID string, end trace.SpanEnd) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ends = append(f.ends, fakeEnd{spanID: spanID, end: end})
	return nil
}

func (f *fakeSpanWriter) snapshot() ([]*entity.TraceSpan, []fakeEnd) {
	f.mu.Lock()
	defer f.mu.Unlock()
	started := make([]*entity.TraceSpan, len(f.started))
	copy(started, f.started)
	ends := make([]fakeEnd, len(f.ends))
	copy(ends, f.ends)
	return started, ends
}

// newAgentRuntimeTestTracer 构造测试用 Tracer + fakeSpanWriter
func newAgentRuntimeTestTracer() (*trace.Tracer, *fakeSpanWriter) {
	repo := &fakeSpanWriter{}
	tracer := trace.NewTracer(repo, trace.Config{Enabled: true, MaxContentLength: 500})
	return tracer, repo
}

// findRunSpan 从 started 中找到 operation=agent.run 的 Span
func findRunSpan(t *testing.T, started []*entity.TraceSpan) *entity.TraceSpan {
	t.Helper()
	for _, s := range started {
		if s.Operation == "agent.run" {
			return s
		}
	}
	t.Fatalf("no agent.run span found in %d started spans", len(started))
	return nil
}

func findRunEnd(t *testing.T, started []*entity.TraceSpan, ends []fakeEnd) fakeEnd {
	t.Helper()
	span := findRunSpan(t, started)
	for _, e := range ends {
		if e.spanID == span.SpanID {
			return e
		}
	}
	t.Fatalf("no end for agent.run span %s", span.SpanID)
	return fakeEnd{}
}

// —— Run 同步包装器测试 ——

func TestAgentRuntimeRunSuccess(t *testing.T) {
	tracer, repo := newAgentRuntimeTestTracer()
	rt := &AgentRuntime{Tracer: tracer}
	meta := trace.RunMeta{RunID: "run-1", AgentSlug: "assistant", Mode: "sync", Query: "hello"}

	err := rt.Run(context.Background(), meta, func(context.Context) error { return nil })
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	started, ends := repo.snapshot()
	if len(started) == 0 {
		t.Fatal("no span started")
	}
	span := findRunSpan(t, started)
	if span.RunID != "run-1" {
		t.Fatalf("run_id = %q, want run-1", span.RunID)
	}
	if len(ends) != 1 {
		t.Fatalf("expected 1 end, got %d", len(ends))
	}
	end := findRunEnd(t, started, ends)
	if end.end.Status != trace.SpanStatusOK {
		t.Fatalf("status = %q, want %q", end.end.Status, trace.SpanStatusOK)
	}
}

func TestAgentRuntimeRunEndsFromError(t *testing.T) {
	tracer, repo := newAgentRuntimeTestTracer()
	rt := &AgentRuntime{Tracer: tracer}
	wantErr := errors.New("model failed")
	meta := trace.RunMeta{RunID: "run-2", AgentSlug: "assistant", Mode: "async"}

	err := rt.Run(context.Background(), meta, func(context.Context) error { return wantErr })
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
	started, ends := repo.snapshot()
	end := findRunEnd(t, started, ends)
	if end.end.Status != trace.SpanStatusError {
		t.Fatalf("status = %q, want %q", end.end.Status, trace.SpanStatusError)
	}
	if end.end.ErrorMessage != wantErr.Error() {
		t.Fatalf("error_message = %q, want %q", end.end.ErrorMessage, wantErr.Error())
	}
}

func TestAgentRuntimeRunEndsFromContextCancel(t *testing.T) {
	tracer, repo := newAgentRuntimeTestTracer()
	rt := &AgentRuntime{Tracer: tracer}
	meta := trace.RunMeta{RunID: "run-3", AgentSlug: "assistant", Mode: "async"}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 进入 Run 前 ctx 已取消

	err := rt.Run(ctx, meta, func(c context.Context) error {
		return c.Err() // 返回 context.Canceled
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want %v", err, context.Canceled)
	}
	started, ends := repo.snapshot()
	end := findRunEnd(t, started, ends)
	if end.end.Status != trace.SpanStatusCancelled {
		t.Fatalf("status = %q, want %q", end.end.Status, trace.SpanStatusCancelled)
	}
}

func TestAgentRuntimeRunEndsFromPanic(t *testing.T) {
	tracer, repo := newAgentRuntimeTestTracer()
	rt := &AgentRuntime{Tracer: tracer}
	meta := trace.RunMeta{RunID: "run-4", AgentSlug: "assistant", Mode: "sync"}

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("panic not re-propagated")
		}
		if r != "boom" {
			t.Fatalf("recovered = %v, want boom", r)
		}
		started, ends := repo.snapshot()
		end := findRunEnd(t, started, ends)
		if end.end.Status != trace.SpanStatusError {
			t.Fatalf("status = %q, want %q", end.end.Status, trace.SpanStatusError)
		}
		if end.end.ErrorType != "panic" {
			t.Fatalf("error_type = %q, want panic", end.end.ErrorType)
		}
	}()

	_ = rt.Run(context.Background(), meta, func(context.Context) error {
		panic("boom")
	})
}

func TestAgentRuntimeRunNoTracerNoOp(t *testing.T) {
	// Tracer 为 nil 时 Run 仍执行 execute，但不写任何 Span
	rt := &AgentRuntime{}
	called := false
	err := rt.Run(context.Background(), trace.RunMeta{RunID: "run-5"}, func(context.Context) error {
		called = true
		return nil
	})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !called {
		t.Fatal("execute not called")
	}
}

func TestAgentRuntimeRunStartsExactlyOneSpan(t *testing.T) {
	tracer, repo := newAgentRuntimeTestTracer()
	rt := &AgentRuntime{Tracer: tracer}
	meta := trace.RunMeta{RunID: "run-6", AgentSlug: "assistant", Mode: "sync"}

	_ = rt.Run(context.Background(), meta, func(context.Context) error { return nil })
	started, ends := repo.snapshot()
	runCount := 0
	for _, s := range started {
		if s.Operation == "agent.run" {
			runCount++
		}
	}
	if runCount != 1 {
		t.Fatalf("agent.run span count = %d, want 1", runCount)
	}
	if len(ends) != 1 {
		t.Fatalf("end count = %d, want 1", len(ends))
	}
}

// —— StartRun + 流迭代器测试 ——

// fakeEventIter 用于测试的迭代器，按顺序返回预置事件
type fakeEventIter struct {
	events []*adk.AgentEvent
	idx    int
}

func (f *fakeEventIter) Next() (*adk.AgentEvent, bool) {
	if f.idx >= len(f.events) {
		return nil, false
	}
	ev := f.events[f.idx]
	f.idx++
	return ev, true
}

func newAgentAndTracer(t *testing.T) (*Agent, *trace.Tracer, *fakeSpanWriter) {
	t.Helper()
	tracer, repo := newAgentRuntimeTestTracer()
	a := &Agent{
		agent:   nil, // 测试不调用 agent.Run，直接测试迭代器包装
		config:  &AgentConfig{Slug: "assistant", Name: "Assistant"},
		runtime: &AgentRuntime{Tracer: tracer},
	}
	return a, tracer, repo
}

func TestTracedIteratorEndsOnExhausted(t *testing.T) {
	a, _, repo := newAgentAndTracer(t)
	ctx, span := a.runtime.StartRun(context.Background(), trace.RunMeta{
		RunID: "run-7", AgentSlug: "assistant", Mode: "stream", Query: "hi",
	})
	inner := &fakeEventIter{events: []*adk.AgentEvent{
		{Output: &adk.AgentOutput{MessageOutput: &adk.MessageVariant{Message: &schema.Message{Content: "ok"}}}},
	}}
	it := newTracedIterator(ctx, span, inner)

	// 消费所有事件
	for {
		ev, ok := it.Next()
		if !ok {
			break
		}
		_ = ev
	}

	started, ends := repo.snapshot()
	end := findRunEnd(t, started, ends)
	if end.end.Status != trace.SpanStatusOK {
		t.Fatalf("status = %q, want %q", end.end.Status, trace.SpanStatusOK)
	}
}

func TestTracedIteratorEndsOnEventError(t *testing.T) {
	a, _, repo := newAgentAndTracer(t)
	ctx, span := a.runtime.StartRun(context.Background(), trace.RunMeta{
		RunID: "run-8", AgentSlug: "assistant", Mode: "stream",
	})
	wantErr := errors.New("model stream failed")
	inner := &fakeEventIter{events: []*adk.AgentEvent{
		{Err: wantErr},
	}}
	it := newTracedIterator(ctx, span, inner)

	_, _ = it.Next()

	started, ends := repo.snapshot()
	end := findRunEnd(t, started, ends)
	if end.end.Status != trace.SpanStatusError {
		t.Fatalf("status = %q, want %q", end.end.Status, trace.SpanStatusError)
	}
	if end.end.ErrorMessage != wantErr.Error() {
		t.Fatalf("error_message = %q, want %q", end.end.ErrorMessage, wantErr.Error())
	}
}

func TestTracedIteratorEndsOnContextCancel(t *testing.T) {
	a, _, repo := newAgentAndTracer(t)
	ctx, cancel := context.WithCancel(context.Background())
	ctx, span := a.runtime.StartRun(ctx, trace.RunMeta{
		RunID: "run-9", AgentSlug: "assistant", Mode: "stream",
	})
	inner := &fakeEventIter{events: []*adk.AgentEvent{}}
	it := newTracedIterator(ctx, span, inner)

	cancel() // 模拟客户端取消
	_, _ = it.Next()

	started, ends := repo.snapshot()
	end := findRunEnd(t, started, ends)
	if end.end.Status != trace.SpanStatusCancelled {
		t.Fatalf("status = %q, want %q", end.end.Status, trace.SpanStatusCancelled)
	}
}

func TestTracedIteratorEndsOnInterrupt(t *testing.T) {
	a, _, repo := newAgentAndTracer(t)
	ctx, span := a.runtime.StartRun(context.Background(), trace.RunMeta{
		RunID: "run-10", AgentSlug: "assistant", Mode: "stream",
	})
	inner := &fakeEventIter{events: []*adk.AgentEvent{
		{Action: &adk.AgentAction{Interrupted: &adk.InterruptInfo{Data: "need approval"}}},
	}}
	it := newTracedIterator(ctx, span, inner)

	_, _ = it.Next()

	started, ends := repo.snapshot()
	end := findRunEnd(t, started, ends)
	if end.end.Status != trace.SpanStatusInterrupted {
		t.Fatalf("status = %q, want %q", end.end.Status, trace.SpanStatusInterrupted)
	}
}

func TestTracedIteratorEndsOnlyOnce(t *testing.T) {
	a, _, repo := newAgentAndTracer(t)
	ctx, span := a.runtime.StartRun(context.Background(), trace.RunMeta{
		RunID: "run-11", AgentSlug: "assistant", Mode: "stream",
	})
	inner := &fakeEventIter{events: []*adk.AgentEvent{}}
	it := newTracedIterator(ctx, span, inner)

	// 多次调用 Next 在耗尽后不应重复结束 Span
	for i := 0; i < 3; i++ {
		_, _ = it.Next()
	}

	started, ends := repo.snapshot()
	span0 := findRunSpan(t, started)
	count := 0
	for _, e := range ends {
		if e.spanID == span0.SpanID {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("agent.run end count = %d, want 1", count)
	}
}

// —— RunMeta 构造测试 ——

func TestBuildRunMetaUsesContextRunID(t *testing.T) {
	// Worker 注入的 RunMeta 必须被保留，不被 Agent 覆盖
	ctx := trace.WithRunMeta(context.Background(), trace.RunMeta{
		RunID:     "worker-run-id",
		AgentSlug: "from-worker",
		Mode:      "async",
		Attempt:   2,
	})
	meta := buildRunMeta(ctx, &AgentConfig{Slug: "assistant"}, "user query", "sync")
	if meta.RunID != "worker-run-id" {
		t.Fatalf("run_id = %q, want worker-run-id（不得覆盖 Worker 注入）", meta.RunID)
	}
	if meta.AgentSlug != "from-worker" {
		t.Fatalf("agent_slug = %q, want from-worker", meta.AgentSlug)
	}
	if meta.Attempt != 2 {
		t.Fatalf("attempt = %d, want 2", meta.Attempt)
	}
	if meta.Mode != "async" {
		t.Fatalf("mode = %q, want async", meta.Mode)
	}
}

func TestBuildRunMetaFallbackForSyncCall(t *testing.T) {
	// 同步调用无 RunMeta 时，生成 UUID run_id，并用 Agent 配置和参数补齐
	meta := buildRunMeta(context.Background(), &AgentConfig{Slug: "assistant"}, "hi", "sync")
	if meta.RunID == "" {
		t.Fatal("run_id should be generated for sync call")
	}
	if meta.AgentSlug != "assistant" {
		t.Fatalf("agent_slug = %q, want assistant", meta.AgentSlug)
	}
	if meta.Query != "hi" {
		t.Fatalf("query = %q, want hi", meta.Query)
	}
	if meta.Mode != "sync" {
		t.Fatalf("mode = %q, want sync", meta.Mode)
	}
}

// 防止测试因 fakeEventIter 未使用 time 而报 unused import
var _ = time.Second
