package trace

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"Qavor/internal/model/entity"
)

// fakeRepository 完整实现 TraceRepository，用于 Tracer/Span 测试。
// 写入方法复制值到切片/映射，查询方法返回预置数据。
type fakeRepository struct {
	records map[string]*entity.TraceRecord
	started []*entity.TraceSpan
	ends    []struct {
		spanID string
		end    SpanEnd
	}
	// 查询预置数据
	traceByRunID map[string]string
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{
		records:      map[string]*entity.TraceRecord{},
		traceByRunID: map[string]string{},
	}
}

func (f *fakeRepository) CreateTrace(_ context.Context, record *entity.TraceRecord) error {
	cp := *record
	f.records[cp.TraceID] = &cp
	return nil
}

func (f *fakeRepository) UpdateTraceMetadata(_ context.Context, traceID string, meta TraceMetadata) error {
	record := f.records[traceID]
	if record == nil {
		return nil
	}
	if meta.ConversationID > 0 {
		record.ConversationID = meta.ConversationID
	}
	if meta.QuerySummary != "" {
		record.QuerySummary = meta.QuerySummary
	}
	if meta.EntryType != "" {
		record.EntryType = meta.EntryType
	}
	return nil
}

func (f *fakeRepository) StartSpan(_ context.Context, span *entity.TraceSpan) error {
	cp := *span
	f.started = append(f.started, &cp)
	return nil
}

func (f *fakeRepository) EndSpan(_ context.Context, spanID string, end SpanEnd) error {
	f.ends = append(f.ends, struct {
		spanID string
		end    SpanEnd
	}{spanID: spanID, end: end})
	return nil
}

func (f *fakeRepository) GetTrace(_ context.Context, traceID string) (*entity.TraceRecord, error) {
	return f.records[traceID], nil
}

func (f *fakeRepository) ListTraces(context.Context, TraceFilter) ([]TraceSummary, int64, error) {
	return nil, 0, nil
}

func (f *fakeRepository) ListSpans(context.Context, string) ([]*entity.TraceSpan, error) {
	return nil, nil
}

func (f *fakeRepository) GetSpan(_ context.Context, spanID string) (*entity.TraceSpan, error) {
	for _, s := range f.started {
		if s.SpanID == spanID {
			cp := *s
			return &cp, nil
		}
	}
	return nil, nil
}

func (f *fakeRepository) GetTraceIDByRunID(_ context.Context, runID string) (string, error) {
	if tid, ok := f.traceByRunID[runID]; ok {
		return tid, nil
	}
	return "", errTraceNotFound
}

func (f *fakeRepository) GetAgentRunSpan(_ context.Context, runID string) (*RunSpanRef, error) {
	if traceID, ok := f.traceByRunID[runID]; ok {
		return &RunSpanRef{TraceID: traceID, SpanID: "agent-span", Attempt: 1}, nil
	}
	return nil, nil
}

func (f *fakeRepository) MarkTimeoutSpans(context.Context, time.Time) (int64, error) {
	return 0, nil
}

func (f *fakeRepository) DeleteExpired(context.Context, time.Time) (int64, error) {
	return 0, nil
}

var errTraceNotFound = errors.New("trace not found")

func TestSpanEndOnlyOnce(t *testing.T) {
	repo := newFakeRepository()
	tracer := NewTracer(repo, Config{Enabled: true, MaxContentLength: 500})
	ctx, span := tracer.StartSpan(context.Background(), SpanSpec{TraceID: "t1", Kind: "agent", Operation: "agent.run"})
	_ = ctx
	span.End(SpanEnd{Status: SpanStatusOK})
	span.End(SpanEnd{Status: SpanStatusError, ErrorMessage: "late"})
	if len(repo.ends) != 1 || repo.ends[0].end.Status != SpanStatusOK {
		t.Fatalf("ends = %+v", repo.ends)
	}
}

func TestStartSpanUsesCurrentParent(t *testing.T) {
	repo := newFakeRepository()
	tracer := NewTracer(repo, Config{Enabled: true})
	parentCtx := WithSpanContext(context.Background(), SpanContext{TraceID: "t1", SpanID: "parent", Sampled: true})
	_, child := tracer.StartSpan(parentCtx, SpanSpec{Kind: "llm", Operation: "llm.generate"})
	if child.Record().ParentSpanID != "parent" || child.Record().TraceID != "t1" {
		t.Fatalf("child = %+v", child.Record())
	}
}

func TestStartSpanNoOpWhenDisabled(t *testing.T) {
	repo := newFakeRepository()
	tracer := NewTracer(repo, Config{Enabled: false})
	ctx, span := tracer.StartSpan(context.Background(), SpanSpec{TraceID: "t1", Kind: "agent", Operation: "agent.run"})
	_ = ctx
	span.End(SpanEnd{Status: SpanStatusOK})
	if len(repo.started) != 0 || len(repo.ends) != 0 {
		t.Fatalf("disabled tracer should not write, started=%d ends=%d", len(repo.started), len(repo.ends))
	}
}

func TestStartSpanNoOpWhenNotSampled(t *testing.T) {
	repo := newFakeRepository()
	tracer := NewTracer(repo, Config{Enabled: true})
	parentCtx := WithSpanContext(context.Background(), SpanContext{TraceID: "t1", SpanID: "parent", Sampled: false})
	_, span := tracer.StartSpan(parentCtx, SpanSpec{Kind: "llm", Operation: "llm.generate"})
	span.End(SpanEnd{Status: SpanStatusOK})
	if len(repo.started) != 0 {
		t.Fatalf("non-sampled should not write, started=%d", len(repo.started))
	}
}

func TestStartSpanInjectsSpanContext(t *testing.T) {
	repo := newFakeRepository()
	tracer := NewTracer(repo, Config{Enabled: true})
	ctx, span := tracer.StartSpan(context.Background(), SpanSpec{TraceID: "t1", Kind: "agent", Operation: "agent.run"})
	sc, ok := SpanContextFromContext(ctx)
	if !ok {
		t.Fatal("StartSpan should inject SpanContext")
	}
	if sc.SpanID != span.Record().SpanID {
		t.Fatalf("injected SpanID = %q, want %q", sc.SpanID, span.Record().SpanID)
	}
	if sc.TraceID != "t1" {
		t.Fatalf("TraceID = %q", sc.TraceID)
	}
	if !sc.Sampled {
		t.Fatal("should be sampled")
	}
}

func TestStartRequestCreatesTraceRecord(t *testing.T) {
	repo := newFakeRepository()
	tracer := NewTracer(repo, Config{Enabled: true})
	meta := RequestMeta{TraceID: "t1", RequestID: "req-1", ConversationID: 5, QuerySummary: "hello", EntryType: entity.EntryTypeHTTP, Method: "POST", Path: "/api/v1/chat"}
	ctx, span := tracer.StartRequest(context.Background(), meta)
	_ = ctx
	if repo.records["t1"] == nil {
		t.Fatal("StartRequest should create TraceRecord")
	}
	if span.Record().Operation != "http.server" {
		t.Fatalf("operation = %q", span.Record().Operation)
	}
}

func TestShouldTrace(t *testing.T) {
	tracer := NewTracer(newFakeRepository(), Config{Enabled: true, TracedRoutes: []string{"POST /api/v1/chat"}})
	if !tracer.ShouldTrace("POST", "/api/v1/chat") {
		t.Fatal("should trace configured route")
	}
	if tracer.ShouldTrace("GET", "/api/v1/health") {
		t.Fatal("should not trace unconfigured route")
	}
}

func TestShouldTraceDisabled(t *testing.T) {
	tracer := NewTracer(newFakeRepository(), Config{Enabled: false, TracedRoutes: []string{"POST /api/v1/chat"}})
	if tracer.ShouldTrace("POST", "/api/v1/chat") {
		t.Fatal("disabled tracer should not trace")
	}
}

func TestSpanContentModeNoneDropsAllContent(t *testing.T) {
	repo := newFakeRepository()
	tracer := NewTracer(repo, Config{Enabled: true, ContentMode: "none", MaxContentLength: 500})
	_, span := tracer.StartSpan(context.Background(), SpanSpec{
		TraceID:      "trace-none",
		Kind:         "agent",
		Operation:    "agent.run",
		InputSummary: "secret query",
	})
	span.End(SpanEnd{
		Status:        SpanStatusError,
		OutputSummary: "secret output",
		ErrorMessage:  "Authorization: Bearer sk-secret",
	})

	if len(repo.started) != 1 || repo.started[0].InputSummary != "" {
		t.Fatalf("started=%+v, want empty input_summary", repo.started)
	}
	if len(repo.ends) != 1 || repo.ends[0].end.OutputSummary != "" || repo.ends[0].end.ErrorMessage != "" {
		t.Fatalf("ends=%+v, want empty output/error summaries", repo.ends)
	}
}

func TestSpanSummaryModeRedactsSensitiveText(t *testing.T) {
	repo := newFakeRepository()
	tracer := NewTracer(repo, Config{Enabled: true, ContentMode: "summary", MaxContentLength: 500})
	_, span := tracer.StartSpan(context.Background(), SpanSpec{
		TraceID:      "trace-summary",
		InputSummary: "Authorization: Bearer sk-input-secret",
	})
	span.End(SpanEnd{Status: SpanStatusError, ErrorMessage: "Bearer sk-error-secret"})

	if strings.Contains(repo.started[0].InputSummary, "sk-input-secret") {
		t.Fatalf("input secret leaked: %q", repo.started[0].InputSummary)
	}
	if strings.Contains(repo.ends[0].end.ErrorMessage, "sk-error-secret") {
		t.Fatalf("error secret leaked: %q", repo.ends[0].end.ErrorMessage)
	}
}

func TestStartRequestContentModeNoneDropsQuerySummary(t *testing.T) {
	repo := newFakeRepository()
	tracer := NewTracer(repo, Config{Enabled: true, ContentMode: "none", MaxContentLength: 500})
	tracer.StartRequest(context.Background(), RequestMeta{
		TraceID: "trace-request-none", QuerySummary: "private question", Method: "POST", Path: "/api/v1/chat",
	})
	if got := repo.records["trace-request-none"].QuerySummary; got != "" {
		t.Fatalf("query_summary=%q, want empty", got)
	}
}

func TestUpdateRequestMetadataFillsTraceRecordAfterBodyParsing(t *testing.T) {
	repo := newFakeRepository()
	tracer := NewTracer(repo, Config{Enabled: true, ContentMode: "summary", MaxContentLength: 500})
	ctx, _ := tracer.StartRequest(context.Background(), RequestMeta{
		TraceID: "trace-meta", RequestID: "req-meta", Method: "POST", Path: "/api/v1/agent/runs",
	})

	tracer.UpdateRequestMetadata(ctx, 42, "user question", "async")

	record := repo.records["trace-meta"]
	if record.ConversationID != 42 || record.QuerySummary != "user question" || record.EntryType != "async" {
		t.Fatalf("trace metadata = %+v", record)
	}
}

func TestUpdateRequestMetadataDoesNotOverwriteQueryWithEmptyResumeValue(t *testing.T) {
	repo := newFakeRepository()
	tracer := NewTracer(repo, Config{Enabled: true, ContentMode: "summary", MaxContentLength: 500})
	ctx, _ := tracer.StartRequest(context.Background(), RequestMeta{
		TraceID: "trace-resume-meta", QuerySummary: "original question", Method: "POST", Path: "/api/v1/agent/runs",
	})

	tracer.UpdateRequestMetadata(ctx, 42, "", "resume")

	record := repo.records["trace-resume-meta"]
	if record.QuerySummary != "original question" || record.EntryType != "resume" {
		t.Fatalf("trace metadata = %+v", record)
	}
}

func TestStartRequestUsesConfiguredRetention(t *testing.T) {
	repo := newFakeRepository()
	retention := 30 * 24 * time.Hour
	tracer := NewTracer(repo, Config{Enabled: true, Retention: retention})
	before := time.Now()
	tracer.StartRequest(context.Background(), RequestMeta{
		TraceID: "trace-retention", Method: "POST", Path: "/api/v1/chat",
	})
	record := repo.records["trace-retention"]
	if record.ExpiresAt.Before(before.Add(retention-time.Second)) || record.ExpiresAt.After(before.Add(retention+time.Second)) {
		t.Fatalf("expires_at=%v, want about %v", record.ExpiresAt, before.Add(retention))
	}
}
