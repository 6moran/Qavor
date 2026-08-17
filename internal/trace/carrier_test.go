package trace

import (
	"context"
	"testing"
)

func TestValidTraceID(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"", false},
		{"not-a-uuid", false},
		{"11111111-1111-1111-1111-111111111111", true},
		{"1111111111111111111111111111111111111111111111111111111111111111-1", false}, // > 64 chars
	}
	for _, c := range cases {
		if got := ValidTraceID(c.in); got != c.want {
			t.Errorf("ValidTraceID(%q) = %v; want %v", c.in, got, c.want)
		}
	}
}

func TestCarrierFromContextWithoutSpanContext(t *testing.T) {
	carrier, ok := CarrierFromContext(context.Background())
	if ok {
		t.Fatalf("expected no carrier from empty context, got %+v", carrier)
	}
}

func TestCarrierFromContextExtractsSpan(t *testing.T) {
	ctx := WithSpanContext(context.Background(), SpanContext{
		TraceID:   "11111111-1111-1111-1111-111111111111",
		SpanID:    "span-1",
		RequestID: "req-1",
		RunID:     "run-1",
		Sampled:   true,
	})
	carrier, ok := CarrierFromContext(ctx)
	if !ok {
		t.Fatal("expected carrier from context with SpanContext")
	}
	if carrier.TraceID != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("TraceID = %q", carrier.TraceID)
	}
	if carrier.ParentSpanID != "span-1" {
		t.Fatalf("ParentSpanID = %q", carrier.ParentSpanID)
	}
	if carrier.RequestID != "req-1" {
		t.Fatalf("RequestID = %q", carrier.RequestID)
	}
	if !carrier.Sampled {
		t.Fatal("Sampled should be true")
	}
}

func TestTraceCarrierRestoresParent(t *testing.T) {
	carrier := TraceCarrier{
		TraceID:      "11111111-1111-1111-1111-111111111111",
		ParentSpanID: "parent-1",
		RequestID:    "req-1",
		Sampled:      true,
	}
	ctx := ContextFromCarrier(context.Background(), carrier)
	got, ok := SpanContextFromContext(ctx)
	if !ok || got.TraceID != carrier.TraceID || got.SpanID != carrier.ParentSpanID || got.RequestID != carrier.RequestID {
		t.Fatalf("restored context = %+v, %v", got, ok)
	}
}

func TestContextFromCarrierPreservesParentOnSubsequentStart(t *testing.T) {
	// 恢复后父 Span 不变：从 carrier 恢复的 SpanID 作为 ParentSpanID
	carrier := TraceCarrier{
		TraceID:      "11111111-1111-1111-1111-111111111111",
		ParentSpanID: "parent-1",
		Sampled:      true,
	}
	ctx := ContextFromCarrier(context.Background(), carrier)
	got, ok := SpanContextFromContext(ctx)
	if !ok {
		t.Fatal("expected SpanContext after carrier restore")
	}
	// 再次读取应得到相同的父 Span，不会被改写
	got2, ok2 := SpanContextFromContext(ctx)
	if !ok2 || got2 != got {
		t.Fatalf("second read = %+v, %v; want %+v, true", got2, ok2, got)
	}
}
