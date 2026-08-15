package run

import (
	"context"
	"errors"
	"testing"
	"time"

	"Qavor/internal/eventbus"
	"Qavor/internal/model/entity"
	"Qavor/internal/trace"

	"go.uber.org/zap"
)

type recordingRunRepository struct {
	statusCalls int
	lastRunID   string
	lastStatus  string
	lastEventID string
}

func (r *recordingRunRepository) Create(*entity.AgentRun) error                   { return nil }
func (r *recordingRunRepository) GetByID(string) (*entity.AgentRun, error)        { return nil, nil }
func (r *recordingRunRepository) GetByRequestID(string) (*entity.AgentRun, error) { return nil, nil }
func (r *recordingRunRepository) Update(*entity.AgentRun) error                   { return nil }
func (r *recordingRunRepository) UpdateStatus(_ context.Context, runID, status, eventID string) error {
	r.statusCalls++
	r.lastRunID = runID
	r.lastStatus = status
	r.lastEventID = eventID
	return nil
}
func (r *recordingRunRepository) ListByThread(string, int, int) ([]entity.AgentRun, int64, error) {
	return nil, 0, nil
}
func (r *recordingRunRepository) ListByStatus(string, int, int) ([]entity.AgentRun, int64, error) {
	return nil, 0, nil
}
func (r *recordingRunRepository) ListSubagentThreadsByParent(uint) ([]entity.SubagentThread, error) {
	return nil, nil
}

type recordingSpanWriter struct {
	ends []trace.SpanEnd
}

func (w *recordingSpanWriter) CreateTrace(context.Context, *entity.TraceRecord) error { return nil }
func (w *recordingSpanWriter) UpdateTraceMetadata(context.Context, string, trace.TraceMetadata) error {
	return nil
}
func (w *recordingSpanWriter) StartSpan(context.Context, *entity.TraceSpan) error { return nil }
func (w *recordingSpanWriter) EndSpan(_ context.Context, _ string, end trace.SpanEnd) error {
	w.ends = append(w.ends, end)
	return nil
}

type fakeEventPublisher struct {
	err error
}

func (p *fakeEventPublisher) PublishPayload(context.Context, string, string, string, string, any) (string, error) {
	if p.err != nil {
		return "", p.err
	}
	return "event-1", nil
}

func failingPublisher() EventPublisher {
	return &fakeEventPublisher{err: errors.New("redis unavailable")}
}

func TestWorkerFinishPublishFailureStillUpdatesRunStatus(t *testing.T) {
	runs := &recordingRunRepository{}
	spanWriter := &recordingSpanWriter{}
	tracer := trace.NewTracer(spanWriter, trace.Config{Enabled: true})
	w := &Worker{
		pub:     failingPublisher(),
		runRepo: runs,
		logger:  zap.NewNop(),
		tracer:  tracer,
	}
	run := &entity.AgentRun{
		ID:                   "run-1",
		ConversationThreadID: "1",
		RequestID:            "req-1",
	}
	ctx := trace.WithSpanContext(context.Background(), trace.SpanContext{
		TraceID: "11111111-1111-1111-1111-111111111111",
		SpanID:  "queue-consume-1",
		Sampled: true,
	})

	w.finish(ctx, run, eventbus.StatusCompleted, entity.StatusCompleted)

	if runs.statusCalls != 1 {
		t.Fatalf("UpdateStatus calls = %d, want 1", runs.statusCalls)
	}
	if runs.lastRunID != run.ID || runs.lastStatus != entity.StatusCompleted {
		t.Fatalf("UpdateStatus = (%q, %q), want (%q, %q)", runs.lastRunID, runs.lastStatus, run.ID, entity.StatusCompleted)
	}
	if runs.lastEventID != "" {
		t.Fatalf("last_event_id = %q, want empty after publish failure", runs.lastEventID)
	}
	if len(spanWriter.ends) != 1 || spanWriter.ends[0].Status != trace.SpanStatusError {
		t.Fatalf("event.publish end = %+v, want one error span", spanWriter.ends)
	}
}

func TestWorkerFinishPublishFailureMatchesTraceDisabledBehavior(t *testing.T) {
	for _, tc := range []struct {
		name   string
		tracer *trace.Tracer
	}{
		{name: "trace disabled", tracer: nil},
		{name: "trace enabled", tracer: trace.NewTracer(&recordingSpanWriter{}, trace.Config{Enabled: true})},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runs := &recordingRunRepository{}
			w := &Worker{pub: failingPublisher(), runRepo: runs, logger: zap.NewNop(), tracer: tc.tracer}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			if tc.tracer != nil {
				ctx = trace.WithSpanContext(ctx, trace.SpanContext{TraceID: "22222222-2222-2222-2222-222222222222", SpanID: "parent", Sampled: true})
			}
			w.finish(ctx, &entity.AgentRun{ID: "run-2"}, eventbus.StatusFailed, entity.StatusFailed)
			if runs.statusCalls != 1 || runs.lastStatus != entity.StatusFailed {
				t.Fatalf("UpdateStatus calls=%d status=%q", runs.statusCalls, runs.lastStatus)
			}
		})
	}
}

func TestQueueRunMetaPreservesBusinessRunIDWithoutTrace(t *testing.T) {
	run := &entity.AgentRun{ID: "business-run-1"}
	item := &QueueItem{
		AgentSlug:        "assistant",
		Query:            "hello",
		Attempt:          0,
		ResumeFromRunID:  "old-run",
		ResumeFromSpanID: "old-span",
	}

	meta := queueRunMeta(run, item, 42, "request-1")
	if meta.RunID != run.ID {
		t.Fatalf("run_id = %q, want %q", meta.RunID, run.ID)
	}
	if meta.Attempt != 1 {
		t.Fatalf("attempt = %d, want 1", meta.Attempt)
	}
	if meta.ResumeFromRunID != item.ResumeFromRunID || meta.ResumeFromSpanID != item.ResumeFromSpanID {
		t.Fatalf("resume metadata = %+v", meta)
	}
}
