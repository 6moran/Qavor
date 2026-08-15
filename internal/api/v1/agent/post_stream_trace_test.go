package agent

import (
	"context"
	"testing"

	"Qavor/internal/trace"
)

func TestResumeTraceContextUsesParentAgentSpan(t *testing.T) {
	base := trace.WithSpanContext(context.Background(), trace.SpanContext{
		TraceID:   "new-http-trace",
		SpanID:    "new-http-span",
		RequestID: "new-request",
		Sampled:   true,
	})
	ref := &trace.RunSpanRef{
		TraceID: "original-trace",
		SpanID:  "interrupted-agent-span",
		Attempt: 2,
	}

	gotCtx, attempt, parentSpanID := resumeTraceContext(base, "resume-request", ref)
	got, ok := trace.SpanContextFromContext(gotCtx)
	if !ok {
		t.Fatal("resume context has no SpanContext")
	}
	if got.TraceID != ref.TraceID || got.SpanID != ref.SpanID {
		t.Fatalf("resume context = %+v, want trace=%q span=%q", got, ref.TraceID, ref.SpanID)
	}
	if got.RequestID != "resume-request" || !got.Sampled {
		t.Fatalf("resume request context = %+v", got)
	}
	if attempt != 3 || parentSpanID != ref.SpanID {
		t.Fatalf("attempt=%d parent=%q, want 3/%q", attempt, parentSpanID, ref.SpanID)
	}
}

func TestResumeTraceContextFallsBackWithoutParentTrace(t *testing.T) {
	base := context.Background()
	gotCtx, attempt, parentSpanID := resumeTraceContext(base, "resume-request", nil)
	if gotCtx != base || attempt != 1 || parentSpanID != "" {
		t.Fatalf("fallback = ctx:%v attempt:%d parent:%q", gotCtx, attempt, parentSpanID)
	}
}
