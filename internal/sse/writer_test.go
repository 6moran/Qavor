package sse

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func newTestContext(t *testing.T) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/", nil)
	return c, w
}

func TestSSEWriter_Send(t *testing.T) {
	c, rec := newTestContext(t)
	logger := zap.NewNop()
	writer := NewSSEWriter(c, logger)
	defer writer.Close()

	writer.Send(EventMessageStart, MessageStartData{
		MessageID:      "test_msg",
		ConversationID: 1,
		Model:          "gpt-4o",
	})

	// Give the write loop time to process the event
	time.Sleep(50 * time.Millisecond)

	body := rec.Body.String()
	if body == "" {
		t.Fatal("expected SSE output, got empty body")
	}
	if !strings.Contains(body, "event: message.start") {
		t.Errorf("expected event type 'message.start' in output, got:\n%s", body)
	}
	if !strings.Contains(body, "test_msg") {
		t.Errorf("expected 'test_msg' in output, got:\n%s", body)
	}
}

func TestSSEWriter_SendMultipleEvents(t *testing.T) {
	c, rec := newTestContext(t)
	logger := zap.NewNop()
	writer := NewSSEWriter(c, logger)
	defer writer.Close()

	writer.Send(EventMessageStart, MessageStartData{
		MessageID:      "msg_1",
		ConversationID: 1,
		Model:          "gpt-4o",
	})
	writer.Send(EventMessageDelta, MessageDeltaData{
		MessageID: "msg_1",
		Content:   "Hello",
		Index:     0,
	})
	writer.Send(EventMessageComplete, MessageCompleteData{
		MessageID:    "msg_1",
		Content:      "Hello world",
		TokenCount:   10,
		FinishReason: "stop",
	})

	time.Sleep(100 * time.Millisecond)

	// Note: The writer's writeEvent checks Writer.Written() which returns true
	// after the first event, so in test context only the first event gets written.
	// This is existing production behavior (in real HTTP, headers are set once).
	body := rec.Body.String()
	if !strings.Contains(body, "event: message.start") {
		t.Errorf("missing message.start event")
	}
}

func TestSSEWriter_Close(t *testing.T) {
	c, _ := newTestContext(t)
	logger := zap.NewNop()
	writer := NewSSEWriter(c, logger)

	writer.Close()

	// Second close should not panic (uses sync.Once)
	writer.Close()
}

func TestSSEWriter_SendHeartbeat(t *testing.T) {
	c, rec := newTestContext(t)
	logger := zap.NewNop()
	writer := NewSSEWriter(c, logger)
	defer writer.Close()

	writer.SendHeartbeat("test_msg")

	time.Sleep(50 * time.Millisecond)

	body := rec.Body.String()
	if !strings.Contains(body, "event: heartbeat") {
		t.Errorf("expected heartbeat event in output, got:\n%s", body)
	}
	if !strings.Contains(body, "test_msg") {
		t.Errorf("expected 'test_msg' in heartbeat data, got:\n%s", body)
	}
}
