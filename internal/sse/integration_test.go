package sse

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func TestStream_Integration(t *testing.T) {
	// 1. Setup test server
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/stream", nil)

	// 2. Create SSE Writer
	logger := zap.NewNop()
	writer := NewSSEWriter(c, logger)
	defer writer.Close()

	// 3. Send test events in sequence: message.start -> message.delta -> message.complete -> done
	writer.Send(EventMessageStart, MessageStartData{
		MessageID:      "integration_test_msg",
		ConversationID: 1,
		Model:          "gpt-4o",
	})

	writer.Send(EventMessageDelta, MessageDeltaData{
		MessageID: "integration_test_msg",
		Content:   "Hello, world!",
		Index:     0,
	})

	writer.Send(EventMessageComplete, MessageCompleteData{
		MessageID:    "integration_test_msg",
		Content:      "Hello, world!",
		TokenCount:   5,
		FinishReason: "stop",
	})

	writer.Send(EventDone, nil)

	// 4. Wait for the write loop to process events
	time.Sleep(200 * time.Millisecond)

	// 5. Verify response body contains the expected SSE events
	body := w.Body.String()

	if !strings.Contains(body, "event: message.start") {
		t.Errorf("expected 'event: message.start' in output, got:\n%s", body)
	}
	if !strings.Contains(body, "integration_test_msg") {
		t.Errorf("expected message ID 'integration_test_msg' in output, got:\n%s", body)
	}
	if !strings.Contains(body, "gpt-4o") {
		t.Errorf("expected model 'gpt-4o' in output, got:\n%s", body)
	}
}

func TestStream_AllEventTypes(t *testing.T) {
	// Test that all defined event types can be serialized and sent without errors.
	// Note: httptest.ResponseRecorder.Written() returns true after the first write,
	// so we create a fresh writer per event type to test each independently.
	tests := []struct {
		name      string
		eventType EventType
		data      interface{}
		expect    string
	}{
		{
			name:      "message.start",
			eventType: EventMessageStart,
			data: MessageStartData{
				MessageID:      "msg_001",
				ConversationID: 1,
				Model:          "gpt-4o",
			},
			expect: "event: message.start",
		},
		{
			name:      "tool_call.start",
			eventType: EventToolCallStart,
			data: ToolCallStartData{
				MessageID:  "msg_001",
				ToolName:   "search",
				ToolCallID: "tc_001",
			},
			expect: "event: tool_call.start",
		},
		{
			name:      "tool_call.end",
			eventType: EventToolCallEnd,
			data: ToolCallEndData{
				MessageID:  "msg_001",
				ToolName:   "search",
				ToolCallID: "tc_001",
				Success:    true,
			},
			expect: "event: tool_call.end",
		},
		{
			name:      "rag.start",
			eventType: EventRAGStart,
			data: RAGStartData{
				Query:           "test query",
				KnowledgeBaseID: 1,
			},
			expect: "event: rag.start",
		},
		{
			name:      "rag.done",
			eventType: EventRAGDone,
			data: RAGDoneData{
				Query:       "test query",
				ResultCount: 3,
				ChunksUsed:  2,
			},
			expect: "event: rag.done",
		},
		{
			name:      "file.upload_start",
			eventType: EventFileUploadStart,
			data: FileUploadStartData{
				FileID:   1,
				FileName: "test.pdf",
				FileSize: 1024,
				FileType: "document",
			},
			expect: "event: file.upload_start",
		},
		{
			name:      "file.upload_complete",
			eventType: EventFileUploadComplete,
			data: FileUploadCompleteData{
				FileID:   1,
				FileName: "test.pdf",
				FileSize: 1024,
				FileType: "document",
				FileURL:  "https://example.com/test.pdf",
			},
			expect: "event: file.upload_complete",
		},
		{
			name:      "file.process_done",
			eventType: EventFileProcessDone,
			data: FileProcessDoneData{
				FileID:      1,
				FileName:    "test.pdf",
				Content:     "extracted text",
				TokenCount:  100,
				ProcessTime: 500,
			},
			expect: "event: file.process_done",
		},
		{
			name:      "message.complete",
			eventType: EventMessageComplete,
			data: MessageCompleteData{
				MessageID:    "msg_001",
				Content:      "final content",
				TokenCount:   50,
				FinishReason: "stop",
			},
			expect: "event: message.complete",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("POST", "/stream", nil)

			logger := zap.NewNop()
			writer := NewSSEWriter(c, logger)

			writer.Send(tt.eventType, tt.data)
			writer.Send(EventDone, nil)

			time.Sleep(100 * time.Millisecond)
			writer.Close()

			body := w.Body.String()
			if !strings.Contains(body, tt.expect) {
				t.Errorf("expected '%s' in output, got:\n%s", tt.expect, body)
			}
		})
	}
}

func TestStream_ErrorEvents(t *testing.T) {
	// Test error and cancellation event types
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/stream", nil)

	logger := zap.NewNop()
	writer := NewSSEWriter(c, logger)
	defer writer.Close()

	writer.Send(EventMessageError, ErrorData{
		Code:    "LLM_INIT_FAILED",
		Message: "初始化LLM失败",
	})

	writer.Send(EventDone, nil)

	time.Sleep(100 * time.Millisecond)

	body := w.Body.String()
	if !strings.Contains(body, "event: message.error") {
		t.Errorf("expected 'event: message.error' in output, got:\n%s", body)
	}
	if !strings.Contains(body, "LLM_INIT_FAILED") {
		t.Errorf("expected error code 'LLM_INIT_FAILED' in output, got:\n%s", body)
	}
}

func TestController_GenerateTaskID(t *testing.T) {
	// Test task ID generation
	id1 := GenerateTaskID()
	id2 := GenerateTaskID()

	if id1 == "" {
		t.Error("Task ID should not be empty")
	}

	if id2 == "" {
		t.Error("Task ID should not be empty")
	}

	if id1 == id2 {
		t.Error("Task IDs should be unique")
	}

	// Verify format: task_<8chars>_<6digits>
	if !strings.HasPrefix(id1, "task_") {
		t.Errorf("Task ID should start with 'task_', got: %s", id1)
	}

	parts := strings.Split(id1, "_")
	if len(parts) != 3 {
		t.Errorf("Task ID should have 3 parts separated by '_', got: %s", id1)
	}

	if len(parts[1]) != 8 {
		t.Errorf("Task ID middle part should be 8 chars, got %d chars: %s", len(parts[1]), parts[1])
	}

	if len(parts[2]) != 6 {
		t.Errorf("Task ID last part should be 6 digits, got %d chars: %s", len(parts[2]), parts[2])
	}
}

func TestStream_ConcurrentSend(t *testing.T) {
	// Test that sending events concurrently does not panic
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/stream", nil)

	logger := zap.NewNop()
	writer := NewSSEWriter(c, logger)
	defer writer.Close()

	done := make(chan struct{})

	// Send events from multiple goroutines
	for i := 0; i < 5; i++ {
		go func(idx int) {
			defer func() { done <- struct{}{} }()
			for j := 0; j < 10; j++ {
				writer.Send(EventMessageDelta, MessageDeltaData{
					MessageID: "concurrent_msg",
					Content:   "chunk",
					Index:     idx*10 + j,
				})
			}
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 5; i++ {
		<-done
	}

	writer.Send(EventDone, nil)

	time.Sleep(200 * time.Millisecond)

	// If we get here without panicking, the test passes
	body := w.Body.String()
	if body == "" {
		t.Error("expected some SSE output from concurrent sends")
	}
}
