package run

import (
	"testing"

	"Qavor/internal/eventbus"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
)

func TestToolResultEventPreservesToolCallID(t *testing.T) {
	mv := &adk.TypedMessageVariant[*schema.Message]{
		Role:     schema.Tool,
		ToolName: "query_kb",
		Message: &schema.Message{
			Role:       schema.Tool,
			ToolName:   "query_kb",
			ToolCallID: "call-kb-1",
			Content:    `{"query_text":"测试","chunks":[{"content":"命中"}]}`,
		},
	}

	// 与 executor.go 生产路径一致：先 GetMessage() 合并完整消息，再传入 toolResultEvent。
	// （toolResultEvent 双参签名：mv 提供 ToolName，msg 提供 Content/ToolCallID）
	msg, _ := mv.GetMessage()
	event := toolResultEvent(mv, msg)
	if event.ToolCall == nil {
		t.Fatal("tool_result 事件缺少 ToolCall 信息")
	}
	if event.ToolCall.ID != "call-kb-1" {
		t.Fatalf("tool_call_id = %q, want %q", event.ToolCall.ID, "call-kb-1")
	}
	if event.ToolCall.Name != "query_kb" {
		t.Fatalf("tool name = %q, want query_kb", event.ToolCall.Name)
	}
	if event.Content != `{"query_text":"测试","chunks":[{"content":"命中"}]}` {
		t.Fatalf("tool output = %q", event.Content)
	}
}

func TestBuildPersistedToolCallsIncludesMatchingResult(t *testing.T) {
	toolCalls := []schema.ToolCall{{
		ID: "call-kb-1",
		Function: schema.FunctionCall{
			Name:      "query_kb",
			Arguments: `{"query_text":"测试"}`,
		},
	}}
	results := map[string]toolExecutionResult{
		"call-kb-1": {
			Output: `{"query_text":"测试","chunks":[{"content":"命中"}]}`,
			Status: "success",
		},
	}

	persisted := buildPersistedToolCalls(toolCalls, results)
	if len(persisted) != 1 {
		t.Fatalf("persisted calls = %d, want 1", len(persisted))
	}
	got := persisted[0]
	if got.LanggraphToolCallID != "call-kb-1" || got.ToolName != "query_kb" {
		t.Fatalf("unexpected persisted call: %+v", got)
	}
	if got.ToolInput["query_text"] != "测试" {
		t.Fatalf("tool input = %#v", got.ToolInput)
	}
	if got.ToolOutput != results["call-kb-1"].Output || got.Status != "success" {
		t.Fatalf("tool result not persisted: %+v", got)
	}
}

func TestBuildPersistedToolCallsDoesNotAttachUnmatchedResult(t *testing.T) {
	toolCalls := []schema.ToolCall{{
		ID:       "call-kb-1",
		Function: schema.FunctionCall{Name: "query_kb", Arguments: `{}`},
	}}
	results := map[string]toolExecutionResult{
		"call-other": {Output: "wrong", Status: "success"},
	}

	persisted := buildPersistedToolCalls(toolCalls, results)
	if len(persisted) != 1 {
		t.Fatalf("persisted calls = %d, want 1", len(persisted))
	}
	if persisted[0].ToolOutput != "" || persisted[0].Status != "pending" {
		t.Fatalf("unmatched result was attached: %+v", persisted[0])
	}
}

func TestRecordToolExecutionResultUsesCallID(t *testing.T) {
	results := map[string]toolExecutionResult{}
	recordToolExecutionResult(results, StreamEvent{
		Type:    "tool_result",
		Content: "result",
		ToolCall: &eventbus.ToolCallInfo{
			ID:   "call-kb-1",
			Name: "query_kb",
		},
	})

	got, ok := results["call-kb-1"]
	if !ok || got.Output != "result" || got.Status != "success" {
		t.Fatalf("result map = %#v", results)
	}
}
