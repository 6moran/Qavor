package chat

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"Qavor/internal/model/entity"
	"Qavor/internal/service"
	"Qavor/internal/trace"

	"github.com/gin-gonic/gin"
)

type traceTestChatService struct{}

func (traceTestChatService) Chat(context.Context, uint, string, string) (*service.ChatResult, error) {
	return &service.ChatResult{}, nil
}
func (traceTestChatService) ChatStream(context.Context, uint, string, string) error { return nil }

type traceMetadataWriter struct {
	traceID string
	meta    trace.TraceMetadata
}

func (w *traceMetadataWriter) CreateTrace(context.Context, *entity.TraceRecord) error { return nil }
func (w *traceMetadataWriter) UpdateTraceMetadata(_ context.Context, traceID string, meta trace.TraceMetadata) error {
	w.traceID = traceID
	w.meta = meta
	return nil
}
func (w *traceMetadataWriter) StartSpan(context.Context, *entity.TraceSpan) error   { return nil }
func (w *traceMetadataWriter) EndSpan(context.Context, string, trace.SpanEnd) error { return nil }

func TestChatControllerUpdatesTraceMetadataAfterParsingBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	writer := &traceMetadataWriter{}
	tracer := trace.NewTracer(writer, trace.Config{Enabled: true, ContentMode: "summary", MaxContentLength: 500})
	ctrl := NewController(traceTestChatService{}, tracer)

	request := httptest.NewRequest(http.MethodPost, "/api/v1/chat", strings.NewReader(`{"conversation_id":42,"agent_slug":"assistant","message":"hello trace"}`))
	request.Header.Set("Content-Type", "application/json")
	request = request.WithContext(trace.WithSpanContext(request.Context(), trace.SpanContext{
		TraceID: "11111111-1111-1111-1111-111111111111", SpanID: "http-span", Sampled: true,
	}))
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = request

	ctrl.Chat(ctx)

	if writer.traceID != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("trace_id=%q", writer.traceID)
	}
	if writer.meta.ConversationID != 42 || writer.meta.QuerySummary != "hello trace" || writer.meta.EntryType != "sync" {
		t.Fatalf("metadata=%+v", writer.meta)
	}
}
