package trace

import (
	"context"
	"testing"
)

func TestSpanContextFromEmptyContext(t *testing.T) {
	_, ok := SpanContextFromContext(context.Background())
	if ok {
		t.Fatal("empty context should not yield SpanContext")
	}
}

func TestRunMetaFromEmptyContext(t *testing.T) {
	_, ok := RunMetaFromContext(context.Background())
	if ok {
		t.Fatal("empty context should not yield RunMeta")
	}
}

func TestSpanContextRoundTrip(t *testing.T) {
	want := SpanContext{
		TraceID:   "11111111-1111-1111-1111-111111111111",
		SpanID:    "span-1",
		RequestID: "req-1",
		RunID:     "run-1",
		Sampled:   true,
	}
	ctx := WithSpanContext(context.Background(), want)
	got, ok := SpanContextFromContext(ctx)
	if !ok || got != want {
		t.Fatalf("SpanContextFromContext() = %+v, %v; want %+v, true", got, ok, want)
	}
}

func TestRunMetaRoundTrip(t *testing.T) {
	want := RunMeta{
		RunID:          "run-1",
		AgentSlug:      "assistant",
		ConversationID: 42,
		RequestID:      "req-1",
		Query:          "hello",
		Mode:           "async",
		Attempt:        1,
	}
	ctx := WithRunMeta(context.Background(), want)
	got, ok := RunMetaFromContext(ctx)
	if !ok || got != want {
		t.Fatalf("RunMetaFromContext() = %+v, %v; want %+v, true", got, ok, want)
	}
}
