package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"Qavor/internal/model/entity"
	"Qavor/internal/trace"
	pkgerrors "Qavor/pkg/errors"
)

// —— 测试 Fake ——

// fakeTraceRepo 实现 trace.TraceRepository，预置数据供 Service 查询
type fakeTraceRepo struct {
	records    map[string]*entity.TraceRecord
	spans      map[string][]*entity.TraceSpan
	summaries  []trace.TraceSummary
	total      int64
	runToTrace map[string]string
	err        error // 预置错误
}

func newFakeTraceRepo() *fakeTraceRepo {
	return &fakeTraceRepo{
		records:    map[string]*entity.TraceRecord{},
		spans:      map[string][]*entity.TraceSpan{},
		runToTrace: map[string]string{},
	}
}

func (f *fakeTraceRepo) CreateTrace(_ context.Context, r *entity.TraceRecord) error { return nil }
func (f *fakeTraceRepo) UpdateTraceMetadata(context.Context, string, trace.TraceMetadata) error {
	return nil
}
func (f *fakeTraceRepo) StartSpan(_ context.Context, s *entity.TraceSpan) error     { return nil }
func (f *fakeTraceRepo) EndSpan(_ context.Context, _ string, _ trace.SpanEnd) error { return nil }
func (f *fakeTraceRepo) GetTrace(_ context.Context, traceID string) (*entity.TraceRecord, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.records[traceID], nil
}
func (f *fakeTraceRepo) ListTraces(_ context.Context, _ trace.TraceFilter) ([]trace.TraceSummary, int64, error) {
	if f.err != nil {
		return nil, 0, f.err
	}
	return f.summaries, f.total, nil
}
func (f *fakeTraceRepo) ListSpans(_ context.Context, traceID string) ([]*entity.TraceSpan, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.spans[traceID], nil
}
func (f *fakeTraceRepo) GetSpan(_ context.Context, _ string) (*entity.TraceSpan, error) {
	if f.err != nil {
		return nil, f.err
	}
	return nil, nil
}
func (f *fakeTraceRepo) GetTraceIDByRunID(_ context.Context, runID string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	if tid, ok := f.runToTrace[runID]; ok {
		return tid, nil
	}
	return "", nil
}
func (f *fakeTraceRepo) GetAgentRunSpan(_ context.Context, runID string) (*trace.RunSpanRef, error) {
	if f.err != nil {
		return nil, f.err
	}
	if traceID, ok := f.runToTrace[runID]; ok {
		return &trace.RunSpanRef{TraceID: traceID, SpanID: "agent-span", Attempt: 1}, nil
	}
	return nil, nil
}
func (f *fakeTraceRepo) MarkTimeoutSpans(context.Context, time.Time) (int64, error) { return 0, nil }
func (f *fakeTraceRepo) DeleteExpired(context.Context, time.Time) (int64, error)    { return 0, nil }

// fakeRunRepo 实现 RunStatusReader，预置 AgentRun 数据
type fakeRunRepo struct {
	runs map[string]*entity.AgentRun
}

func newFakeRunRepo() *fakeRunRepo {
	return &fakeRunRepo{runs: map[string]*entity.AgentRun{}}
}

func (f *fakeRunRepo) GetByID(id string) (*entity.AgentRun, error) {
	return f.runs[id], nil
}

// —— 测试用例 ——

func TestTraceServiceListUsesAgentRunAsStatus(t *testing.T) {
	traceRepo := newFakeTraceRepo()
	traceRepo.summaries = []trace.TraceSummary{{
		TraceID: "t1", RunID: "r1", AgentStatus: trace.SpanStatusError,
		BusinessRunStatus: entity.StatusCompleted, QuerySummary: "q",
	}}
	traceRepo.total = 1
	runRepo := newFakeRunRepo()
	runRepo.runs["r1"] = &entity.AgentRun{ID: "r1", Status: entity.StatusCompleted}

	svc := NewTraceService(traceRepo, runRepo)
	items, total, err := svc.ListTraces(context.Background(), TraceListFilter{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(items) != 1 {
		t.Fatalf("total=%d items=%d", total, len(items))
	}
	if !items[0].StatusMismatch {
		t.Fatalf("expected StatusMismatch=true, item=%+v", items[0])
	}
	if items[0].AgentStatus != trace.SpanStatusError || items[0].BusinessRunStatus != entity.StatusCompleted {
		t.Fatalf("statuses = agent:%s business:%s", items[0].AgentStatus, items[0].BusinessRunStatus)
	}
}

func TestTraceServiceListMismatchOnly(t *testing.T) {
	traceRepo := newFakeTraceRepo()
	// Repository 负责在分页前应用 mismatch_only，仅返回匹配项。
	traceRepo.summaries = []trace.TraceSummary{
		{TraceID: "t2", RunID: "r2", AgentStatus: trace.SpanStatusError, BusinessRunStatus: entity.StatusCompleted},
	}
	traceRepo.total = 1
	runRepo := newFakeRunRepo()
	runRepo.runs["r1"] = &entity.AgentRun{ID: "r1", Status: entity.StatusCompleted}
	runRepo.runs["r2"] = &entity.AgentRun{ID: "r2", Status: entity.StatusCompleted}

	svc := NewTraceService(traceRepo, runRepo)
	items, _, err := svc.ListTraces(context.Background(), TraceListFilter{Page: 1, PageSize: 20, MismatchOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].TraceID != "t2" {
		t.Fatalf("mismatch_only should return only mismatched, items=%+v", items)
	}
}

func TestTraceServiceDoesNotFilterAfterRepositoryPagination(t *testing.T) {
	traceRepo := newFakeTraceRepo()
	traceRepo.summaries = []trace.TraceSummary{{
		TraceID: "repo-selected", RunID: "r1", AgentStatus: trace.SpanStatusOK,
	}}
	traceRepo.total = 1
	runRepo := newFakeRunRepo()
	runRepo.runs["r1"] = &entity.AgentRun{ID: "r1", Status: entity.StatusCompleted}

	svc := NewTraceService(traceRepo, runRepo)
	items, total, err := svc.ListTraces(context.Background(), TraceListFilter{
		Page: 1, PageSize: 20, MismatchOnly: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(items) != 1 || items[0].TraceID != "repo-selected" {
		t.Fatalf("service post-filtered repository page: total=%d items=%+v", total, items)
	}
}

func TestTraceServiceListAggregates(t *testing.T) {
	traceRepo := newFakeTraceRepo()
	traceRepo.summaries = []trace.TraceSummary{{
		TraceID: "t1", RunID: "r1", AgentStatus: trace.SpanStatusOK,
		BusinessRunStatus: entity.StatusCompleted, DurationMs: 1500,
		QueueWaitMs: 200, LLMCount: 3, ToolCount: 2, TotalTokens: 500,
		FirstError: "", StartedAt: time.Now(),
	}}
	traceRepo.total = 1
	runRepo := newFakeRunRepo()
	runRepo.runs["r1"] = &entity.AgentRun{ID: "r1", Status: entity.StatusCompleted}

	svc := NewTraceService(traceRepo, runRepo)
	items, _, err := svc.ListTraces(context.Background(), TraceListFilter{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	it := items[0]
	if it.LLMCount != 3 || it.ToolCount != 2 || it.TotalTokens != 500 || it.QueueWaitMs != 200 || it.DurationMs != 1500 {
		t.Fatalf("aggregates = %+v", it)
	}
}

func TestIsStatusMismatchTimeout(t *testing.T) {
	// timeout span + cancelled run：超时被取消属于异常，判为不一致
	if !isStatusMismatch(trace.SpanStatusTimeout, entity.StatusCancelled) {
		t.Fatal("timeout/cancelled should be mismatch")
	}
	// timeout span + failed run：超时以失败收尾，互认一致
	if isStatusMismatch(trace.SpanStatusTimeout, entity.StatusFailed) {
		t.Fatal("timeout/failed should be consistent")
	}
	// 其余互认映射保持不变
	if isStatusMismatch(trace.SpanStatusCancelled, entity.StatusInterrupted) {
		t.Fatal("cancelled/interrupted should be consistent")
	}
	if isStatusMismatch(trace.SpanStatusInterrupted, entity.StatusCancelled) {
		t.Fatal("interrupted/cancelled should be consistent")
	}
	if !isStatusMismatch(trace.SpanStatusError, entity.StatusCompleted) {
		t.Fatal("error/completed should be mismatch")
	}
}

func TestTraceServiceDetailSortedByStartedAt(t *testing.T) {
	traceRepo := newFakeTraceRepo()
	t1 := time.Now()
	t2 := t1.Add(time.Second)
	t3 := t2.Add(time.Second)
	traceRepo.records["trace-1"] = &entity.TraceRecord{TraceID: "trace-1", QuerySummary: "q"}
	// 故意打乱顺序，验证 service 输出按 started_at 升序
	traceRepo.spans["trace-1"] = []*entity.TraceSpan{
		{SpanID: "s2", TraceID: "trace-1", Operation: "tool.execute", StartedAt: t2},
		{SpanID: "s1", TraceID: "trace-1", Operation: "agent.run", StartedAt: t1},
		{SpanID: "s3", TraceID: "trace-1", Operation: "llm.generate", StartedAt: t3},
	}

	svc := NewTraceService(traceRepo, newFakeRunRepo())
	detail, err := svc.GetTraceDetail(context.Background(), "trace-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Spans) != 3 {
		t.Fatalf("spans=%d", len(detail.Spans))
	}
	if detail.Spans[0].SpanID != "s1" || detail.Spans[1].SpanID != "s2" || detail.Spans[2].SpanID != "s3" {
		t.Fatalf("order = %s %s %s", detail.Spans[0].SpanID, detail.Spans[1].SpanID, detail.Spans[2].SpanID)
	}
}

func TestTraceServiceDetailToolTriggeredBySpanID(t *testing.T) {
	traceRepo := newFakeTraceRepo()
	t1 := time.Now()
	t2 := t1.Add(time.Second)
	traceRepo.records["trace-1"] = &entity.TraceRecord{TraceID: "trace-1"}
	// LLM span 输出含 tool_call_id，Tool span 输入含相同 tool_call_id
	traceRepo.spans["trace-1"] = []*entity.TraceSpan{
		{SpanID: "llm-1", TraceID: "trace-1", Kind: "llm", Operation: "llm.generate", StartedAt: t1,
			Attributes: entity.JSON{"tool_call_ids": []string{"call-1"}}},
		{SpanID: "tool-1", TraceID: "trace-1", Kind: "tool", Operation: "tool.execute", StartedAt: t2,
			Attributes: entity.JSON{"tool_call_id": "call-1"}},
	}

	svc := NewTraceService(traceRepo, newFakeRunRepo())
	detail, err := svc.GetTraceDetail(context.Background(), "trace-1")
	if err != nil {
		t.Fatal(err)
	}
	var toolSpan *TraceSpanItem
	for i := range detail.Spans {
		if detail.Spans[i].SpanID == "tool-1" {
			toolSpan = &detail.Spans[i]
		}
	}
	if toolSpan == nil {
		t.Fatal("tool-1 span not found")
	}
	if toolSpan.TriggeredBySpanID != "llm-1" {
		t.Fatalf("triggered_by_span_id = %q, want llm-1", toolSpan.TriggeredBySpanID)
	}
}

func TestTraceServiceDetailToolTriggeredBySpanIDFromJSONBArray(t *testing.T) {
	traceRepo := newFakeTraceRepo()
	traceRepo.records["trace-jsonb"] = &entity.TraceRecord{TraceID: "trace-jsonb"}
	traceRepo.spans["trace-jsonb"] = []*entity.TraceSpan{
		{
			SpanID: "llm-jsonb", TraceID: "trace-jsonb", Kind: entity.SpanKindLLM,
			Operation: "llm.generate", Attributes: entity.JSON{"tool_call_ids": []any{"call-1", "call-2"}},
		},
		{
			SpanID: "tool-jsonb", TraceID: "trace-jsonb", Kind: entity.SpanKindTool,
			Operation: "tool.execute", Attributes: entity.JSON{"tool_call_id": "call-2"},
		},
	}

	detail, err := NewTraceService(traceRepo, nil).GetTraceDetail(context.Background(), "trace-jsonb")
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Spans) != 2 || detail.Spans[1].TriggeredBySpanID != "llm-jsonb" {
		t.Fatalf("tool link = %+v", detail.Spans)
	}
}

func TestTraceServiceDetailDiagnosticsRunningSpan(t *testing.T) {
	traceRepo := newFakeTraceRepo()
	traceRepo.records["trace-1"] = &entity.TraceRecord{TraceID: "trace-1"}
	traceRepo.spans["trace-1"] = []*entity.TraceSpan{
		{SpanID: "s1", TraceID: "trace-1", Operation: "agent.run", StartedAt: time.Now(), Status: trace.SpanStatusRunning},
	}

	svc := NewTraceService(traceRepo, newFakeRunRepo())
	detail, err := svc.GetTraceDetail(context.Background(), "trace-1")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, d := range detail.Diagnostics {
		if d.Code == "running_span" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected running_span diagnostic, got %+v", detail.Diagnostics)
	}
}

func TestTraceServiceDetailNotFound(t *testing.T) {
	traceRepo := newFakeTraceRepo()
	svc := NewTraceService(traceRepo, newFakeRunRepo())
	_, err := svc.GetTraceDetail(context.Background(), "missing")
	if err == nil {
		t.Fatal("expected error for missing trace")
	}
	if !pkgerrors.IsBizError(err) {
		t.Fatalf("expected BizError, got %v", err)
	}
}

func TestTraceServiceGetTraceByRunIDNotFound(t *testing.T) {
	traceRepo := newFakeTraceRepo()
	svc := NewTraceService(traceRepo, newFakeRunRepo())
	_, err := svc.GetTraceByRunID(context.Background(), "missing-run")
	if err == nil {
		t.Fatal("expected error for missing run")
	}
	if !pkgerrors.IsBizError(err) {
		t.Fatalf("expected BizError, got %v", err)
	}
}

func TestTraceServiceGetTraceByRunIDFound(t *testing.T) {
	traceRepo := newFakeTraceRepo()
	traceRepo.runToTrace["run-1"] = "trace-1"
	svc := NewTraceService(traceRepo, newFakeRunRepo())
	traceID, err := svc.GetTraceByRunID(context.Background(), "run-1")
	if err != nil {
		t.Fatal(err)
	}
	if traceID != "trace-1" {
		t.Fatalf("traceID = %q, want trace-1", traceID)
	}
}

func TestTraceServiceListRepoError(t *testing.T) {
	traceRepo := newFakeTraceRepo()
	traceRepo.err = errors.New("db down")
	svc := NewTraceService(traceRepo, newFakeRunRepo())
	_, _, err := svc.ListTraces(context.Background(), TraceListFilter{Page: 1, PageSize: 20})
	if err == nil {
		t.Fatal("expected error")
	}
}
