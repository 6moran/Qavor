package run

import (
	"context"
	"testing"

	"Qavor/internal/eventbus"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
)

// intPtr 返回指向 v 的指针（ToolCall.Index 需要指针）
func intPtr(v int) *int { return &v }

// toolCallStreamChunk 构造一个带流式工具调用片段的 chunk
func toolCallStreamChunk(index int, id, name, argsDelta string) *schema.Message {
	return &schema.Message{
		Role: schema.Assistant,
		ToolCalls: []schema.ToolCall{{
			Index:    intPtr(index),
			ID:       id,
			Type:     "function",
			Function: schema.FunctionCall{Name: name, Arguments: argsDelta},
		}},
	}
}

// runEmitAssistant 执行 emitAssistant 并返回全部发出的事件
func runEmitAssistant(t *testing.T, mv *adk.TypedMessageVariant[*schema.Message]) []StreamEvent {
	t.Helper()
	e := &agentExecutor{}
	var events []StreamEvent
	emit := func(ev StreamEvent) { events = append(events, ev) }
	e.emitAssistant(context.Background(), mv, emit)
	return events
}

// TestEmitAssistantStreamingToolCall 流式模式下工具调用事件必须发出（回归：此前 chunk.ToolCalls 被丢弃）
func TestEmitAssistantStreamingToolCall(t *testing.T) {
	chunks := []*schema.Message{
		toolCallStreamChunk(0, "call_1", "query_kb", `{"query":"`),
		toolCallStreamChunk(0, "", "", `付涛"}`),
	}
	mv := &adk.TypedMessageVariant[*schema.Message]{
		IsStreaming:   true,
		MessageStream: schema.StreamReaderFromArray(chunks),
		Role:          schema.Assistant,
	}

	events := runEmitAssistant(t, mv)

	var toolCallEvents []StreamEvent
	for _, ev := range events {
		if ev.Type == "tool_call" {
			toolCallEvents = append(toolCallEvents, ev)
		}
	}
	if len(toolCallEvents) != 1 {
		t.Fatalf("期望发出 1 个 tool_call 事件，实际 %d 个：%+v", len(toolCallEvents), events)
	}
	tc := toolCallEvents[0].ToolCall
	if tc == nil {
		t.Fatal("tool_call 事件缺少 ToolCall 信息")
	}
	if tc.ID != "call_1" || tc.Name != "query_kb" || tc.Index != 0 {
		t.Errorf("工具调用信息不完整: %+v", tc)
	}
	if tc.Args != `{"query":"付涛"}` {
		t.Errorf("args 增量未正确拼接, 期望 %q, 实际 %q", `{"query":"付涛"}`, tc.Args)
	}

	// 流结束后应发出 message_end
	hasEnd := false
	for _, ev := range events {
		if ev.Type == "message_end" {
			hasEnd = true
		}
	}
	if !hasEnd {
		t.Error("缺少 message_end 事件")
	}
}

// TestEmitAssistantStreamingParallelToolCalls 并发工具调用（多 index）各自独立聚合
func TestEmitAssistantStreamingParallelToolCalls(t *testing.T) {
	chunks := []*schema.Message{
		toolCallStreamChunk(0, "call_a", "query_kb", `{"query":"a"}`),
		toolCallStreamChunk(1, "call_b", "task", `{"desc":"`),
		toolCallStreamChunk(1, "", "", `b"}`),
	}
	mv := &adk.TypedMessageVariant[*schema.Message]{
		IsStreaming:   true,
		MessageStream: schema.StreamReaderFromArray(chunks),
		Role:          schema.Assistant,
	}

	events := runEmitAssistant(t, mv)

	byIndex := map[int]*eventbus.ToolCallInfo{}
	for _, ev := range events {
		if ev.Type == "tool_call" && ev.ToolCall != nil {
			byIndex[ev.ToolCall.Index] = ev.ToolCall
		}
	}
	if len(byIndex) != 2 {
		t.Fatalf("期望 2 个工具调用事件，实际 %d 个", len(byIndex))
	}
	if byIndex[0].Name != "query_kb" || byIndex[0].Args != `{"query":"a"}` {
		t.Errorf("index 0 聚合错误: %+v", byIndex[0])
	}
	if byIndex[1].Name != "task" || byIndex[1].Args != `{"desc":"b"}` {
		t.Errorf("index 1 聚合错误: %+v", byIndex[1])
	}
}

// TestEmitAssistantNonStreamingToolCall 非流式工具调用仍正常发出（回归保护）
func TestEmitAssistantNonStreamingToolCall(t *testing.T) {
	idx := 0
	msg := &schema.Message{
		Role:    schema.Assistant,
		Content: "",
		ToolCalls: []schema.ToolCall{{
			Index:    &idx,
			ID:       "call_ns",
			Type:     "function",
			Function: schema.FunctionCall{Name: "query_kb", Arguments: `{"query":"x"}`},
		}},
	}
	mv := &adk.TypedMessageVariant[*schema.Message]{
		IsStreaming: false,
		Message:     msg,
		Role:        schema.Assistant,
	}

	events := runEmitAssistant(t, mv)

	var toolCallEvents []StreamEvent
	for _, ev := range events {
		if ev.Type == "tool_call" {
			toolCallEvents = append(toolCallEvents, ev)
		}
	}
	if len(toolCallEvents) != 1 {
		t.Fatalf("期望 1 个 tool_call 事件，实际 %d 个", len(toolCallEvents))
	}
	if toolCallEvents[0].ToolCall.Args != `{"query":"x"}` {
		t.Errorf("非流式 args 错误: %+v", toolCallEvents[0].ToolCall)
	}
}
