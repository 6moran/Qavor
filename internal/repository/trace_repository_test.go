package repository

import (
	"context"
	"strings"
	"testing"
	"time"

	"Qavor/internal/model/entity"
	"Qavor/internal/trace"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func traceTestID(prefix string) string {
	return prefix + "-" + uuid.NewString()
}

func TestMergeTraceAttributesPreservesStartAndEndValues(t *testing.T) {
	start := entity.JSON{"http.method": "POST", "http.path": "/api/v1/chat", "shared": "start"}
	end := entity.JSON{"http.status_code": 200, "shared": "end"}
	got := mergeTraceAttributes(start, end)
	if got["http.method"] != "POST" || got["http.path"] != "/api/v1/chat" || got["http.status_code"] != 200 {
		t.Fatalf("merged attributes = %+v", got)
	}
	if got["shared"] != "end" {
		t.Fatalf("end value must win, got %+v", got)
	}
}

func TestQueueWaitMillisecondsReadsCanonicalAndLegacyKey(t *testing.T) {
	if got := queueWaitMilliseconds(entity.JSON{"queue_wait_ms": float64(321)}); got != 321 {
		t.Fatalf("canonical queue wait = %d", got)
	}
	if got := queueWaitMilliseconds(entity.JSON{"queue.wait_ms": float64(654)}); got != 654 {
		t.Fatalf("legacy queue wait = %d", got)
	}
	if got := queueWaitMilliseconds(entity.JSON{"queue_wait_ms": int64(777), "queue.wait_ms": float64(1)}); got != 777 {
		t.Fatalf("canonical key must win, got %d", got)
	}
}

func openTraceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	d := dsn(t)
	if d == "" {
		t.Skip("QAVOR_TEST_POSTGRES_DSN not set")
	}
	db, err := gorm.Open(postgres.Open(d), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	var databaseName string
	if err := db.Raw("SELECT current_database()").Scan(&databaseName).Error; err != nil {
		t.Fatalf("read current database: %v", err)
	}
	if !strings.Contains(strings.ToLower(databaseName), "test") {
		t.Fatalf("refusing trace integration tests on non-test database %q", databaseName)
	}
	if err := db.AutoMigrate(
		&entity.AgentTrace{}, &entity.AgentTraceSpan{},
		&entity.TraceRecord{}, &entity.TraceSpan{}, &entity.AgentRun{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatalf("begin test transaction: %v", tx.Error)
	}
	t.Cleanup(func() { _ = tx.Rollback().Error })
	return tx
}

// startTestSpan 创建一个 running Span 并返回其 spanID
func startTestSpan(t *testing.T, repo trace.TraceRepository, traceID, spanID string) *entity.TraceSpan {
	t.Helper()
	now := time.Now()
	rec := &entity.TraceRecord{
		TraceID:   traceID,
		EntryType: entity.EntryTypeAgent,
		CreatedAt: now,
		ExpiresAt: now.Add(7 * 24 * time.Hour),
	}
	if err := repo.CreateTrace(context.Background(), rec); err != nil {
		t.Fatalf("create trace: %v", err)
	}
	span := &entity.TraceSpan{
		SpanID:    spanID,
		TraceID:   traceID,
		Kind:      "agent",
		Operation: "agent.run",
		Status:    trace.SpanStatusRunning,
		StartedAt: now,
	}
	if err := repo.StartSpan(context.Background(), span); err != nil {
		t.Fatalf("start span: %v", err)
	}
	return span
}

func loadSpan(t *testing.T, db *gorm.DB, spanID string) *entity.TraceSpan {
	t.Helper()
	var s entity.TraceSpan
	if err := db.Where("span_id = ?", spanID).First(&s).Error; err != nil {
		t.Fatalf("load span %s: %v", spanID, err)
	}
	return &s
}

// TestTraceRepositoryCreateTraceConflictIgnore TraceRecord 冲突忽略
func TestTraceRepositoryCreateTraceConflictIgnore(t *testing.T) {
	db := openTraceTestDB(t)
	repo := NewTraceSpanRepository(db)
	ctx := context.Background()
	traceID := traceTestID("trace-conflict")
	now := time.Now()

	rec1 := &entity.TraceRecord{TraceID: traceID, EntryType: entity.EntryTypeAgent, CreatedAt: now, ExpiresAt: now.Add(7 * 24 * time.Hour), QuerySummary: "first"}
	if err := repo.CreateTrace(ctx, rec1); err != nil {
		t.Fatalf("create trace 1: %v", err)
	}
	// 第二次创建同一 trace_id，应被忽略（OnConflict DoNothing）
	rec2 := &entity.TraceRecord{TraceID: traceID, EntryType: entity.EntryTypeAgent, CreatedAt: now, ExpiresAt: now.Add(7 * 24 * time.Hour), QuerySummary: "second"}
	if err := repo.CreateTrace(ctx, rec2); err != nil {
		t.Fatalf("create trace 2: %v", err)
	}
	got, err := repo.GetTrace(ctx, traceID)
	if err != nil || got == nil {
		t.Fatalf("get trace: %v %v", got, err)
	}
	if got.QuerySummary != "first" {
		t.Fatalf("conflict should not overwrite, got query=%q", got.QuerySummary)
	}
}

// TestTraceRepositoryStartSpanConflictIgnore Span Start 冲突忽略
func TestTraceRepositoryStartSpanConflictIgnore(t *testing.T) {
	db := openTraceTestDB(t)
	repo := NewTraceSpanRepository(db)
	ctx := context.Background()
	traceID := traceTestID("trace-span-conflict")
	spanID := traceTestID("span-dup")
	now := time.Now()

	repo.CreateTrace(ctx, &entity.TraceRecord{TraceID: traceID, EntryType: entity.EntryTypeAgent, CreatedAt: now, ExpiresAt: now.Add(7 * 24 * time.Hour)})

	span1 := &entity.TraceSpan{SpanID: spanID, TraceID: traceID, Kind: "llm", Operation: "llm.generate", Status: trace.SpanStatusRunning, StartedAt: now, InputSummary: "first"}
	if err := repo.StartSpan(ctx, span1); err != nil {
		t.Fatalf("start span 1: %v", err)
	}
	span2 := &entity.TraceSpan{SpanID: spanID, TraceID: traceID, Kind: "llm", Operation: "llm.generate", Status: trace.SpanStatusRunning, StartedAt: now, InputSummary: "second"}
	if err := repo.StartSpan(ctx, span2); err != nil {
		t.Fatalf("start span 2: %v", err)
	}
	got := loadSpan(t, db, spanID)
	if got.InputSummary != "first" {
		t.Fatalf("conflict should not overwrite, got input=%q", got.InputSummary)
	}
}

// TestTraceRepositoryEndSpanFirstTerminalWins 第一次 End 成功，第二次 End 不覆盖
func TestTraceRepositoryEndSpanFirstTerminalWins(t *testing.T) {
	db := openTraceTestDB(t)
	repo := NewTraceSpanRepository(db)
	ctx := context.Background()
	traceID := traceTestID("trace-end")
	spanID := traceTestID("span-end")

	startTestSpan(t, repo, traceID, spanID)

	// 第一次 End: ok
	if err := repo.EndSpan(ctx, spanID, trace.SpanEnd{Status: trace.SpanStatusOK, EndedAt: time.Now(), OutputSummary: "success"}); err != nil {
		t.Fatalf("end span 1: %v", err)
	}
	// 第二次 End: error，不应覆盖
	if err := repo.EndSpan(ctx, spanID, trace.SpanEnd{Status: trace.SpanStatusError, EndedAt: time.Now(), ErrorMessage: "late"}); err != nil {
		t.Fatalf("end span 2: %v", err)
	}
	got := loadSpan(t, db, spanID)
	if got.Status != trace.SpanStatusOK || got.ErrorMessage != "" || got.OutputSummary != "success" {
		t.Fatalf("first terminal should win: status=%s err=%q out=%q", got.Status, got.ErrorMessage, got.OutputSummary)
	}
}

// TestTraceRepositoryEndSpanDurationFromDB duration_ms 由数据库 started_at 计算
func TestTraceRepositoryEndSpanDurationFromDB(t *testing.T) {
	db := openTraceTestDB(t)
	repo := NewTraceSpanRepository(db)
	ctx := context.Background()
	traceID := traceTestID("trace-dur")
	spanID := traceTestID("span-dur")

	now := time.Now()
	repo.CreateTrace(ctx, &entity.TraceRecord{TraceID: traceID, EntryType: entity.EntryTypeAgent, CreatedAt: now, ExpiresAt: now.Add(7 * 24 * time.Hour)})
	span := &entity.TraceSpan{SpanID: spanID, TraceID: traceID, Kind: "llm", Operation: "llm.generate", Status: trace.SpanStatusRunning, StartedAt: now}
	repo.StartSpan(ctx, span)

	endAt := now.Add(150 * time.Millisecond)
	if err := repo.EndSpan(ctx, spanID, trace.SpanEnd{Status: trace.SpanStatusOK, EndedAt: endAt}); err != nil {
		t.Fatalf("end span: %v", err)
	}
	got := loadSpan(t, db, spanID)
	if got.DurationMs < 100 || got.DurationMs > 500 {
		t.Fatalf("duration_ms should be ~150ms, got %d", got.DurationMs)
	}
}

// TestTraceRepositoryListSpansOrdered 按 started_at 排序
func TestTraceRepositoryListSpansOrdered(t *testing.T) {
	db := openTraceTestDB(t)
	repo := NewTraceSpanRepository(db)
	ctx := context.Background()
	traceID := traceTestID("trace-list")
	now := time.Now()

	repo.CreateTrace(ctx, &entity.TraceRecord{TraceID: traceID, EntryType: entity.EntryTypeAgent, CreatedAt: now, ExpiresAt: now.Add(7 * 24 * time.Hour)})

	// 逆序插入，验证查询按 started_at ASC 排序
	spanIDs := []string{traceTestID("span-a"), traceTestID("span-b"), traceTestID("span-c")}
	for i, offset := range []time.Duration{200 * time.Millisecond, 0, 100 * time.Millisecond} {
		s := &entity.TraceSpan{
			SpanID:    spanIDs[i],
			TraceID:   traceID,
			Kind:      "llm",
			Operation: "llm.generate",
			Status:    trace.SpanStatusOK,
			StartedAt: now.Add(offset),
		}
		if err := repo.StartSpan(ctx, s); err != nil {
			t.Fatalf("start span %d: %v", i, err)
		}
	}
	spans, err := repo.ListSpans(ctx, traceID)
	if err != nil || len(spans) != 3 {
		t.Fatalf("list spans: %v len=%d", err, len(spans))
	}
	if spans[0].SpanID != spanIDs[1] || spans[1].SpanID != spanIDs[2] || spans[2].SpanID != spanIDs[0] {
		t.Fatalf("order wrong: %s %s %s", spans[0].SpanID, spans[1].SpanID, spans[2].SpanID)
	}
}

// TestTraceRepositoryGetTraceIDByRunID run_id 反查 trace_id
func TestTraceRepositoryGetTraceIDByRunID(t *testing.T) {
	db := openTraceTestDB(t)
	repo := NewTraceSpanRepository(db)
	ctx := context.Background()
	traceID := traceTestID("trace-runid")
	runID := traceTestID("run-lookup")
	spanID := traceTestID("span-run")
	now := time.Now()

	repo.CreateTrace(ctx, &entity.TraceRecord{TraceID: traceID, EntryType: entity.EntryTypeAgent, CreatedAt: now, ExpiresAt: now.Add(7 * 24 * time.Hour)})
	span := &entity.TraceSpan{
		SpanID:    spanID,
		TraceID:   traceID,
		RunID:     runID,
		Kind:      "agent",
		Operation: "agent.run",
		Status:    trace.SpanStatusOK,
		StartedAt: now,
	}
	repo.StartSpan(ctx, span)

	got, err := repo.GetTraceIDByRunID(ctx, runID)
	if err != nil || got != traceID {
		t.Fatalf("GetTraceIDByRunID: got=%q err=%v", got, err)
	}
	// 不存在的 run_id 返回空串
	got2, err := repo.GetTraceIDByRunID(ctx, traceTestID("run-nonexistent"))
	if err != nil || got2 != "" {
		t.Fatalf("nonexistent run_id should return empty: got=%q err=%v", got2, err)
	}
}

// TestTraceRepositoryMarkTimeoutSpans 超时只影响 running
func TestTraceRepositoryMarkTimeoutSpans(t *testing.T) {
	db := openTraceTestDB(t)
	repo := NewTraceSpanRepository(db)
	ctx := context.Background()
	traceID := traceTestID("trace-timeout")
	runningSpanID := traceTestID("span-running")
	doneSpanID := traceTestID("span-done")
	now := time.Now()

	repo.CreateTrace(ctx, &entity.TraceRecord{TraceID: traceID, EntryType: entity.EntryTypeAgent, CreatedAt: now, ExpiresAt: now.Add(7 * 24 * time.Hour)})

	// running Span（旧时间，应被标记 timeout）
	oldStart := now.AddDate(-200, 0, 0)
	runningSpan := &entity.TraceSpan{SpanID: runningSpanID, TraceID: traceID, Kind: "agent", Operation: "agent.run", Status: trace.SpanStatusRunning, StartedAt: oldStart}
	repo.StartSpan(ctx, runningSpan)

	// 已结束 Span（旧时间，不应被影响）
	doneSpan := &entity.TraceSpan{SpanID: doneSpanID, TraceID: traceID, Kind: "llm", Operation: "llm.generate", Status: trace.SpanStatusOK, StartedAt: oldStart}
	repo.StartSpan(ctx, doneSpan)
	repo.EndSpan(ctx, doneSpanID, trace.SpanEnd{Status: trace.SpanStatusOK, EndedAt: oldStart.Add(time.Second)})

	// before = 1 小时前，running 且 started_at < before 的会被标记
	n, err := repo.MarkTimeoutSpans(ctx, now.AddDate(-100, 0, 0))
	if err != nil {
		t.Fatalf("mark timeout: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 timeout, got %d", n)
	}

	runningGot := loadSpan(t, db, runningSpanID)
	if runningGot.Status != trace.SpanStatusTimeout {
		t.Fatalf("running span should be timeout, got %s", runningGot.Status)
	}
	doneGot := loadSpan(t, db, doneSpanID)
	if doneGot.Status != trace.SpanStatusOK {
		t.Fatalf("done span should remain ok, got %s", doneGot.Status)
	}
}

// TestTraceRepositoryDeleteExpired 过期时先删 Span 再删 Record
func TestTraceRepositoryDeleteExpired(t *testing.T) {
	db := openTraceTestDB(t)
	repo := NewTraceSpanRepository(db)
	ctx := context.Background()
	traceID := traceTestID("trace-expired")
	spanID := traceTestID("span-expired")
	now := time.Now()

	// 过期 record
	repo.CreateTrace(ctx, &entity.TraceRecord{TraceID: traceID, EntryType: entity.EntryTypeAgent, CreatedAt: now.AddDate(-200, 0, 0), ExpiresAt: now.AddDate(-150, 0, 0)})
	span := &entity.TraceSpan{SpanID: spanID, TraceID: traceID, Kind: "agent", Operation: "agent.run", Status: trace.SpanStatusOK, StartedAt: now.AddDate(-200, 0, 0)}
	repo.StartSpan(ctx, span)

	// 未过期 record
	freshTraceID := traceTestID("trace-fresh")
	freshSpanID := traceTestID("span-fresh")
	repo.CreateTrace(ctx, &entity.TraceRecord{TraceID: freshTraceID, EntryType: entity.EntryTypeAgent, CreatedAt: now, ExpiresAt: now.Add(7 * 24 * time.Hour)})
	freshSpan := &entity.TraceSpan{SpanID: freshSpanID, TraceID: freshTraceID, Kind: "agent", Operation: "agent.run", Status: trace.SpanStatusOK, StartedAt: now}
	repo.StartSpan(ctx, freshSpan)

	n, err := repo.DeleteExpired(ctx, now.AddDate(-100, 0, 0))
	if err != nil {
		t.Fatalf("delete expired: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 deleted record, got %d", n)
	}

	// 过期 trace 的 span 应被删除
	var count int64
	db.Model(&entity.TraceSpan{}).Where("trace_id = ?", traceID).Count(&count)
	if count != 0 {
		t.Fatalf("expired trace spans should be deleted, got %d", count)
	}
	// 过期 trace 的 record 应被删除（GetTrace 返回 nil, nil）
	gotExpired, err := repo.GetTrace(ctx, traceID)
	if err != nil || gotExpired != nil {
		t.Fatalf("expired trace should be deleted: got=%v err=%v", gotExpired, err)
	}

	// 未过期 trace 仍存在
	got, err := repo.GetTrace(ctx, freshTraceID)
	if err != nil || got == nil {
		t.Fatalf("fresh trace should still exist: %v %v", got, err)
	}
}

func TestTraceRepositoryGetAgentRunSpan(t *testing.T) {
	db := openTraceTestDB(t)
	repo := NewTraceSpanRepository(db)
	ctx := context.Background()
	traceID := traceTestID("trace-resume")
	runID := traceTestID("run-resume")
	now := time.Now()

	if err := repo.CreateTrace(ctx, &entity.TraceRecord{
		TraceID: traceID, EntryType: entity.EntryTypeAgent,
		CreatedAt: now, ExpiresAt: now.Add(24 * time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	oldSpanID := traceTestID("agent-old")
	newSpanID := traceTestID("agent-new")
	for _, span := range []*entity.TraceSpan{
		{SpanID: oldSpanID, TraceID: traceID, RunID: runID, Kind: entity.SpanKindAgent, Operation: "agent.run", Status: trace.SpanStatusInterrupted, StartedAt: now, Attributes: entity.JSON{"attempt": 1}},
		{SpanID: newSpanID, TraceID: traceID, RunID: runID, Kind: entity.SpanKindAgent, Operation: "agent.run", Status: trace.SpanStatusInterrupted, StartedAt: now.Add(time.Second), Attributes: entity.JSON{"attempt": 2}},
	} {
		if err := repo.StartSpan(ctx, span); err != nil {
			t.Fatal(err)
		}
	}

	got, err := repo.GetAgentRunSpan(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.TraceID != traceID || got.SpanID != newSpanID || got.Attempt != 2 {
		t.Fatalf("GetAgentRunSpan = %+v", got)
	}
}

func TestTraceRepositoryListFiltersBeforePagination(t *testing.T) {
	db := openTraceTestDB(t)
	repo := NewTraceSpanRepository(db)
	ctx := context.Background()
	base := time.Now().Add(-time.Hour)
	wantTraceIDs := map[string]bool{}

	for i := 0; i < 25; i++ {
		traceID := traceTestID("trace-page")
		runID := traceTestID("run-page")
		requestID := traceTestID("request-page")
		createdAt := base.Add(time.Duration(i) * time.Second)
		isMatch := i < 2 // 两条最旧记录不在未过滤列表第一页。
		agentStatus := trace.SpanStatusOK
		businessStatus := entity.StatusCompleted
		errorMessage := ""
		if isMatch {
			agentStatus = trace.SpanStatusError
			businessStatus = entity.StatusCompleted // error/completed 构成 mismatch。
			errorMessage = "model failed"
			wantTraceIDs[traceID] = true
		}

		if err := db.Create(&entity.AgentRun{
			ID: runID, ConversationThreadID: traceTestID("thread"), AgentSlug: "assistant",
			Status: businessStatus, RequestID: requestID, RunType: "chat", InputPayload: entity.JSON{},
			CreatedAt: createdAt, UpdatedAt: createdAt,
		}).Error; err != nil {
			t.Fatal(err)
		}
		if err := repo.CreateTrace(ctx, &entity.TraceRecord{
			TraceID: traceID, RequestID: requestID, EntryType: entity.EntryTypeAgent,
			QuerySummary: "pagination", CreatedAt: createdAt, ExpiresAt: createdAt.Add(24 * time.Hour),
		}); err != nil {
			t.Fatal(err)
		}
		if err := repo.StartSpan(ctx, &entity.TraceSpan{
			SpanID: traceTestID("agent-page"), TraceID: traceID, RunID: runID,
			Kind: entity.SpanKindAgent, Operation: "agent.run", DisplayName: "assistant",
			Status: agentStatus, ErrorMessage: errorMessage, StartedAt: createdAt,
			Attributes: entity.JSON{"agent_slug": "assistant"},
		}); err != nil {
			t.Fatal(err)
		}
	}
	// 未知 Agent Span 状态按 Service 语义不算 mismatch，不能混入筛选结果。
	unknownTraceID := traceTestID("trace-unknown-status")
	unknownRunID := traceTestID("run-unknown-status")
	unknownRequestID := traceTestID("request-unknown-status")
	if err := db.Create(&entity.AgentRun{
		ID: unknownRunID, ConversationThreadID: traceTestID("thread"), AgentSlug: "assistant",
		Status: entity.StatusCompleted, RequestID: unknownRequestID, RunType: "chat", InputPayload: entity.JSON{},
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := repo.CreateTrace(ctx, &entity.TraceRecord{
		TraceID: unknownTraceID, RequestID: unknownRequestID, EntryType: entity.EntryTypeAgent,
		CreatedAt: base.Add(30 * time.Second), ExpiresAt: base.Add(24 * time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	if err := repo.StartSpan(ctx, &entity.TraceSpan{
		SpanID: traceTestID("agent-unknown-status"), TraceID: unknownTraceID, RunID: unknownRunID,
		Kind: entity.SpanKindAgent, Operation: "agent.run", Status: "future_status",
		ErrorMessage: "unknown status error", StartedAt: base.Add(30 * time.Second),
	}); err != nil {
		t.Fatal(err)
	}

	items, total, err := repo.ListTraces(ctx, trace.TraceFilter{
		ErrorOnly: true, MismatchOnly: true, Page: 1, PageSize: 20,
	})
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || len(items) != 2 {
		t.Fatalf("filtered page len=%d total=%d, want 2/2", len(items), total)
	}
	for _, item := range items {
		if !wantTraceIDs[item.TraceID] {
			t.Fatalf("unexpected trace after filtering: %s", item.TraceID)
		}
	}
}

func TestTraceRepositoryMismatchOnlyTimeoutCancelled(t *testing.T) {
	db := openTraceTestDB(t)
	repo := NewTraceSpanRepository(db)
	ctx := context.Background()
	base := time.Now().Add(-time.Hour)

	// timeout span + cancelled run：超时被取消属于异常，应判为不一致并筛出
	timeoutCancelledID := traceTestID("trace-timeout-cancelled")
	// timeout span + failed run：超时以失败收尾，互认一致，不应筛出
	timeoutFailedID := traceTestID("trace-timeout-failed")
	// ok span + completed run：一致，不应筛出
	okCompletedID := traceTestID("trace-ok-completed")

	cases := []struct {
		traceID, runID, requestID, agentStatus, businessStatus string
	}{
		{timeoutCancelledID, traceTestID("run-timeout-cancelled"), traceTestID("req-timeout-cancelled"), trace.SpanStatusTimeout, entity.StatusCancelled},
		{timeoutFailedID, traceTestID("run-timeout-failed"), traceTestID("req-timeout-failed"), trace.SpanStatusTimeout, entity.StatusFailed},
		{okCompletedID, traceTestID("run-ok-completed"), traceTestID("req-ok-completed"), trace.SpanStatusOK, entity.StatusCompleted},
	}
	for i, c := range cases {
		if err := db.Create(&entity.AgentRun{
			ID: c.runID, ConversationThreadID: traceTestID("thread"), AgentSlug: "assistant",
			Status: c.businessStatus, RequestID: c.requestID, RunType: "chat", InputPayload: entity.JSON{},
			CreatedAt: base.Add(time.Duration(i) * time.Second), UpdatedAt: base.Add(time.Duration(i) * time.Second),
		}).Error; err != nil {
			t.Fatal(err)
		}
		if err := repo.CreateTrace(ctx, &entity.TraceRecord{
			TraceID: c.traceID, RequestID: c.requestID, EntryType: entity.EntryTypeAgent,
			CreatedAt: base.Add(time.Duration(i) * time.Second), ExpiresAt: base.Add(24 * time.Hour),
		}); err != nil {
			t.Fatal(err)
		}
		if err := repo.StartSpan(ctx, &entity.TraceSpan{
			SpanID: traceTestID("agent"), TraceID: c.traceID, RunID: c.runID,
			Kind: entity.SpanKindAgent, Operation: "agent.run", DisplayName: "assistant",
			Status: c.agentStatus, StartedAt: base.Add(time.Duration(i) * time.Second),
			Attributes: entity.JSON{"agent_slug": "assistant"},
		}); err != nil {
			t.Fatal(err)
		}
	}

	items, total, err := repo.ListTraces(ctx, trace.TraceFilter{MismatchOnly: true, Page: 1, PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(items) != 1 || items[0].TraceID != timeoutCancelledID {
		t.Fatalf("mismatch_only should return only timeout/cancelled, total=%d items=%+v", total, items)
	}
}
