package trace

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"Qavor/internal/model/entity"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

func TestHandlerDoesNotCreateRootTrace(t *testing.T) {
	repo := newFakeRepository()
	tracer := NewTracer(repo, Config{Enabled: true, MaxContentLength: 500})
	h := NewHandler(tracer)
	ctx := WithSpanContext(context.Background(), SpanContext{TraceID: "t1", SpanID: "agent-span", RunID: "run-1", Sampled: true})
	info := &callbacks.RunInfo{Name: "ChatModel", Component: components.ComponentOfChatModel}
	ctx = h.OnStart(ctx, info, &model.CallbackInput{Config: &model.Config{Model: "qwen"}})
	h.OnEnd(ctx, info, &model.CallbackOutput{TokenUsage: &model.TokenUsage{PromptTokens: 3, CompletionTokens: 2}})
	if len(repo.records) != 0 {
		t.Fatalf("callback created trace records: %d", len(repo.records))
	}
	if len(repo.started) == 0 || repo.started[0].ParentSpanID != "agent-span" {
		t.Fatalf("parent = %+v", repo.started)
	}
	if repo.started[0].Kind != "llm" || repo.started[0].Operation != "llm.generate" {
		t.Fatalf("span kind/operation = %+v", repo.started[0])
	}
}

func TestHandlerLLMTokenAndOutput(t *testing.T) {
	repo := newFakeRepository()
	tracer := NewTracer(repo, Config{Enabled: true, ContentMode: "summary", MaxContentLength: 500})
	h := NewHandler(tracer)
	ctx := WithSpanContext(context.Background(), SpanContext{TraceID: "t2", SpanID: "agent-span", Sampled: true})
	info := &callbacks.RunInfo{Name: "ChatModel", Component: components.ComponentOfChatModel}
	ctx = h.OnStart(ctx, info, &model.CallbackInput{
		Messages: []*schema.Message{{Role: schema.User, Content: "hello"}},
		Config:   &model.Config{Model: "gpt-4o"},
	})
	h.OnEnd(ctx, info, &model.CallbackOutput{
		Message:    &schema.Message{Role: schema.Assistant, Content: "answer"},
		TokenUsage: &model.TokenUsage{PromptTokens: 10, CompletionTokens: 5, CompletionTokensDetails: model.CompletionTokensDetails{ReasoningTokens: 2}},
	})
	if len(repo.ends) != 1 {
		t.Fatalf("expected 1 end, got %d", len(repo.ends))
	}
	end := repo.ends[0].end
	if end.TokensIn != 10 || end.TokensOut != 5 || end.ReasoningTokens != 2 {
		t.Fatalf("tokens = in:%d out:%d reasoning:%d", end.TokensIn, end.TokensOut, end.ReasoningTokens)
	}
	if end.OutputSummary != "answer" {
		t.Fatalf("output_summary = %q", end.OutputSummary)
	}
	if end.Status != SpanStatusOK {
		t.Fatalf("status = %q", end.Status)
	}
}

func TestHandlerLLMNonStreamEndsWithoutUsage(t *testing.T) {
	repo := newFakeRepository()
	tracer := NewTracer(repo, Config{Enabled: true, ContentMode: "summary", MaxContentLength: 500})
	h := NewHandler(tracer)
	ctx := WithSpanContext(context.Background(), SpanContext{TraceID: "t-ns", SpanID: "agent-span", Sampled: true})
	info := &callbacks.RunInfo{Name: "ChatModel", Component: components.ComponentOfChatModel}
	ctx = h.OnStart(ctx, info, &model.CallbackInput{Config: &model.Config{Model: "gpt-4o"}})

	// 非流式 Generate 仅调用一次 OnEnd，即使缺少 usage / finish_reason 也应正常结束并落库输出，
	// 否则 span 会卡在 running、OutputSummary 永不写入。
	h.OnEnd(ctx, info, &model.CallbackOutput{
		Message: &schema.Message{Role: schema.Assistant, Content: "answer"},
	})
	if len(repo.ends) != 1 {
		t.Fatalf("expected 1 end, got %d", len(repo.ends))
	}
	end := repo.ends[0].end
	if end.OutputSummary != "answer" {
		t.Fatalf("output_summary = %q", end.OutputSummary)
	}
	if end.Status != SpanStatusOK {
		t.Fatalf("status = %q", end.Status)
	}
}

func TestHandlerLLMStreamOutput(t *testing.T) {
	repo := newFakeRepository()
	tracer := NewTracer(repo, Config{Enabled: true, ContentMode: "summary", MaxContentLength: 500})
	h := NewHandler(tracer)
	ctx := WithSpanContext(context.Background(), SpanContext{TraceID: "t-stream", SpanID: "agent-span", Sampled: true})
	info := &callbacks.RunInfo{Name: "ChatModel", Component: components.ComponentOfChatModel}
	ctx = h.OnStart(ctx, info, &model.CallbackInput{Config: &model.Config{Model: "gpt-4o"}})

	sr, sw := schema.Pipe[callbacks.CallbackOutput](1)
	// 真实流程中模型侧在独立 goroutine 发送 chunk；此处同样异步发送，
	// 避免 cap=1 的 pipe 在 handler 读取 goroutine 启动前阻塞。
	go func() {
		// openai 语义：每个 chunk 的 Message 为累计全文
		sw.Send(&model.CallbackOutput{Message: &schema.Message{Role: schema.Assistant, Content: "Hello"}}, nil)
		sw.Send(&model.CallbackOutput{Message: &schema.Message{Role: schema.Assistant, Content: "Hello world"}}, nil)
		// 末尾 usage-only 空消息（openai 把 usage 放在无内容的最后一块）
		sw.Send(&model.CallbackOutput{
			Message:    &schema.Message{Role: schema.Assistant, ResponseMeta: &schema.ResponseMeta{Usage: &schema.TokenUsage{PromptTokens: 23, CompletionTokens: 7, TotalTokens: 30}}},
			TokenUsage: &model.TokenUsage{PromptTokens: 23, CompletionTokens: 7, TotalTokens: 30},
		}, nil)
		sw.Close()
	}()

	h.OnEndWithStreamOutput(ctx, info, sr)

	// 流式 drain 在 goroutine 中异步完成，轮询等待落库
	deadline := time.Now().Add(2 * time.Second)
	for len(repo.ends) == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if len(repo.ends) != 1 {
		t.Fatalf("expected 1 end, got %d", len(repo.ends))
	}
	end := repo.ends[0].end
	// 累计全文不应被重复拼接
	if end.OutputSummary != "Hello world" {
		t.Fatalf("output_summary = %q (want %q)", end.OutputSummary, "Hello world")
	}
	if end.TokensIn != 23 || end.TokensOut != 7 {
		t.Fatalf("tokens = in:%d out:%d", end.TokensIn, end.TokensOut)
	}
	if end.Status != SpanStatusOK {
		t.Fatalf("status = %q", end.Status)
	}
}

func TestHandlerLLMReasoningNotInOutput(t *testing.T) {
	repo := newFakeRepository()
	tracer := NewTracer(repo, Config{Enabled: true, ContentMode: "summary", MaxContentLength: 500})
	h := NewHandler(tracer)
	ctx := WithSpanContext(context.Background(), SpanContext{TraceID: "t3", SpanID: "agent-span", Sampled: true})
	info := &callbacks.RunInfo{Name: "ChatModel", Component: components.ComponentOfChatModel}
	ctx = h.OnStart(ctx, info, &model.CallbackInput{Config: &model.Config{Model: "r1"}})
	h.OnEnd(ctx, info, &model.CallbackOutput{
		Message: &schema.Message{
			Role:    schema.Assistant,
			Content: "",
			MultiContent: []schema.ChatMessagePart{
				{Type: schema.ChatMessagePartTypeText, Text: "visible"},
			},
			AssistantGenMultiContent: []schema.MessageOutputPart{
				{Type: schema.ChatMessagePartTypeReasoning, Reasoning: &schema.MessageOutputReasoning{Text: "secret reasoning"}},
			},
		},
	})
	end := repo.ends[0].end
	if strings.Contains(end.OutputSummary, "secret reasoning") {
		t.Fatalf("reasoning leaked into output: %q", end.OutputSummary)
	}
	if !strings.Contains(end.OutputSummary, "visible") {
		t.Fatalf("text missing from output: %q", end.OutputSummary)
	}
}

func TestHandlerToolError(t *testing.T) {
	repo := newFakeRepository()
	tracer := NewTracer(repo, Config{Enabled: true, MaxContentLength: 500})
	h := NewHandler(tracer)
	ctx := WithSpanContext(context.Background(), SpanContext{TraceID: "t4", SpanID: "agent-span", Sampled: true})
	info := &callbacks.RunInfo{Name: "query_kb", Component: components.ComponentOfTool}
	ctx = h.OnStart(ctx, info, &tool.CallbackInput{ArgumentsInJSON: `{"query":"文档"}`})
	h.OnError(ctx, info, context.DeadlineExceeded)
	if len(repo.ends) != 1 {
		t.Fatalf("expected 1 end, got %d", len(repo.ends))
	}
	end := repo.ends[0].end
	if end.Status != SpanStatusError || end.ErrorMessage == "" {
		t.Fatalf("error end = %+v", end)
	}
}

func TestHandlerToolJSONSensitiveRedacted(t *testing.T) {
	repo := newFakeRepository()
	tracer := NewTracer(repo, Config{Enabled: true, ContentMode: "summary", MaxContentLength: 500})
	h := NewHandler(tracer)
	ctx := WithSpanContext(context.Background(), SpanContext{TraceID: "t5", SpanID: "agent-span", Sampled: true})
	info := &callbacks.RunInfo{Name: "search", Component: components.ComponentOfTool}
	ctx = h.OnStart(ctx, info, &tool.CallbackInput{ArgumentsInJSON: `{"api_key":"sk-secret","query":"test"}`})
	h.OnEnd(ctx, info, &tool.CallbackOutput{Response: `{"result":"ok","token":"leaked"}`})
	if len(repo.started) == 0 {
		t.Fatal("no span started")
	}
	if strings.Contains(repo.started[0].InputSummary, "sk-secret") {
		t.Fatalf("input api_key not redacted: %q", repo.started[0].InputSummary)
	}
	if len(repo.ends) == 0 {
		t.Fatal("no span ended")
	}
	if strings.Contains(repo.ends[0].end.OutputSummary, "leaked") {
		t.Fatalf("output token not redacted: %q", repo.ends[0].end.OutputSummary)
	}
}

func TestHandlerRetrieverHitCount(t *testing.T) {
	repo := newFakeRepository()
	tracer := NewTracer(repo, Config{Enabled: true, MaxContentLength: 500})
	h := NewHandler(tracer)
	ctx := WithSpanContext(context.Background(), SpanContext{TraceID: "t6", SpanID: "agent-span", Sampled: true})
	info := &callbacks.RunInfo{Name: "Retriever", Component: components.ComponentOfRetriever}
	ctx = h.OnStart(ctx, info, &retriever.CallbackInput{Query: "检索词", TopK: 3})
	h.OnEnd(ctx, info, &retriever.CallbackOutput{Docs: []*schema.Document{{Content: "d1"}, {Content: "d2"}, {Content: "d3"}}})
	if len(repo.started) == 0 {
		t.Fatal("no span started")
	}
	sp := repo.started[0]
	if sp.Kind != "retriever" || sp.Operation != "retriever.search" {
		t.Fatalf("kind/operation = %s/%s", sp.Kind, sp.Operation)
	}
	if sp.InputSummary != "检索词" {
		t.Fatalf("input = %q", sp.InputSummary)
	}
	end := repo.ends[0].end
	hitCount, _ := end.Attributes["retriever.hit_count"].(int)
	if hitCount != 3 {
		t.Fatalf("hit_count = %v, want 3", end.Attributes["retriever.hit_count"])
	}
}

func TestHandlerEmbedding(t *testing.T) {
	repo := newFakeRepository()
	tracer := NewTracer(repo, Config{Enabled: true, ContentMode: "summary", MaxContentLength: 500})
	h := NewHandler(tracer)
	ctx := WithSpanContext(context.Background(), SpanContext{TraceID: "t-emb", SpanID: "retriever-span", Sampled: true})
	info := &callbacks.RunInfo{Name: "OpenAI", Component: components.ComponentOfEmbedding}
	ctx = h.OnStart(ctx, info, &embedding.CallbackInput{
		Texts:  []string{"查询文本"},
		Config: &embedding.Config{Model: "text-embedding-3"},
	})
	h.OnEnd(ctx, info, &embedding.CallbackOutput{
		TokenUsage: &embedding.TokenUsage{PromptTokens: 9, TotalTokens: 9},
	})
	if len(repo.started) == 0 {
		t.Fatal("no span started")
	}
	sp := repo.started[0]
	if sp.Kind != "embedding" || sp.Operation != "embedding.generate" {
		t.Fatalf("kind/operation = %s/%s", sp.Kind, sp.Operation)
	}
	if sp.DisplayName != "text-embedding-3" {
		t.Fatalf("display_name = %q", sp.DisplayName)
	}
	if sp.ParentSpanID != "retriever-span" {
		t.Fatalf("parent = %q", sp.ParentSpanID)
	}
	count, _ := sp.Attributes["embedding.text_count"].(int)
	if count != 1 {
		t.Fatalf("text_count = %v, want 1", sp.Attributes["embedding.text_count"])
	}
	end := repo.ends[0].end
	if end.TokensIn != 9 {
		t.Fatalf("tokens_in = %d, want 9", end.TokensIn)
	}
	if end.Status != SpanStatusOK {
		t.Fatalf("status = %q", end.Status)
	}
}

func TestHandlerEmbeddingError(t *testing.T) {
	repo := newFakeRepository()
	tracer := NewTracer(repo, Config{Enabled: true})
	h := NewHandler(tracer)
	ctx := WithSpanContext(context.Background(), SpanContext{TraceID: "t-emb-err", SpanID: "retriever-span", Sampled: true})
	info := &callbacks.RunInfo{Name: "OpenAI", Component: components.ComponentOfEmbedding}
	ctx = h.OnStart(ctx, info, &embedding.CallbackInput{Texts: []string{"x"}})
	h.OnError(ctx, info, context.DeadlineExceeded)
	if len(repo.ends) != 1 {
		t.Fatalf("expected 1 end, got %d", len(repo.ends))
	}
	end := repo.ends[0].end
	if end.Status != SpanStatusError || end.ErrorMessage == "" {
		t.Fatalf("error end = %+v", end)
	}
}

func TestHandlerNoSpanContextSkip(t *testing.T) {
	repo := newFakeRepository()
	tracer := NewTracer(repo, Config{Enabled: true})
	h := NewHandler(tracer)
	ctx := h.OnStart(context.Background(), &callbacks.RunInfo{Name: "x", Component: components.ComponentOfChatModel}, nil)
	if len(repo.started) != 0 {
		t.Fatal("should not create span without SpanContext")
	}
	// ctx should be unchanged
	_, ok := SpanContextFromContext(ctx)
	if ok {
		t.Fatal("should not inject SpanContext")
	}
}

func TestHandlerDoesNotEndParentSpan(t *testing.T) {
	repo := newFakeRepository()
	tracer := NewTracer(repo, Config{Enabled: true})
	h := NewHandler(tracer)
	ctx := WithSpanContext(context.Background(), SpanContext{TraceID: "t7", SpanID: "parent-span", Sampled: true})
	info := &callbacks.RunInfo{Name: "ChatModel", Component: components.ComponentOfChatModel}
	ctx = h.OnStart(ctx, info, &model.CallbackInput{Config: &model.Config{Model: "m"}})
	h.OnEnd(ctx, info, &model.CallbackOutput{})
	// 只有 LLM span 应该被结束，而不是父 span
	if len(repo.ends) != 1 {
		t.Fatalf("expected 1 end, got %d", len(repo.ends))
	}
	if repo.ends[0].spanID == "parent-span" {
		t.Fatal("parent span was ended by callback")
	}
}

func TestHandlerNilTracerNoOp(t *testing.T) {
	h := NewHandler(nil)
	ctx := h.OnStart(context.Background(), &callbacks.RunInfo{Name: "x", Component: components.ComponentOfChatModel}, nil)
	h.OnEnd(ctx, &callbacks.RunInfo{}, &model.CallbackOutput{})
	// should not panic
}

func TestHandlerLLMToolCallIDs(t *testing.T) {
	repo := newFakeRepository()
	tracer := NewTracer(repo, Config{Enabled: true, MaxContentLength: 500})
	h := NewHandler(tracer)
	ctx := WithSpanContext(context.Background(), SpanContext{TraceID: "t8", SpanID: "agent-span", Sampled: true})
	info := &callbacks.RunInfo{Name: "ChatModel", Component: components.ComponentOfChatModel}
	ctx = h.OnStart(ctx, info, &model.CallbackInput{Config: &model.Config{Model: "m"}})
	h.OnEnd(ctx, info, &model.CallbackOutput{
		Message: &schema.Message{
			Role: schema.Assistant,
			ToolCalls: []schema.ToolCall{
				{ID: "call-1", Function: schema.FunctionCall{Name: "search"}},
			},
		},
	})
	end := repo.ends[0].end
	ids, _ := end.Attributes["tool_call_ids"].([]string)
	if len(ids) != 1 || ids[0] != "call-1" {
		t.Fatalf("tool_call_ids = %v", end.Attributes["tool_call_ids"])
	}
}

func TestTruncate(t *testing.T) {
	s := strings.Repeat("a", 1000)
	if got := truncate(s, 500); len(got) != 500 {
		t.Fatalf("truncate 长度=%d want 500", len(got))
	}
	if got := truncate("短", 500); got != "短" {
		t.Fatalf("短字符串不应截断: %q", got)
	}
}

func TestHandlerLLMMessagesStructured(t *testing.T) {
	repo := newFakeRepository()
	tracer := NewTracer(repo, Config{Enabled: true, ContentMode: "summary", MaxContentLength: 500})
	h := NewHandler(tracer)
	ctx := WithSpanContext(context.Background(), SpanContext{TraceID: "t-msgs", SpanID: "agent-span", Sampled: true})
	info := &callbacks.RunInfo{Name: "ChatModel", Component: components.ComponentOfChatModel}
	ctx = h.OnStart(ctx, info, &model.CallbackInput{
		Messages: []*schema.Message{
			{Role: schema.System, Content: "你是助手，密钥 Bearer sk-secret-1 勿泄露"},
			{Role: schema.User, Content: "你好"},
			{Role: schema.Assistant, Content: "你好！"},
			{Role: schema.User, Content: "请查一下付涛"},
		},
		Config: &model.Config{Model: "m"},
	})
	sp := repo.started[0]
	// input_summary 应取最后一条 user 消息（不被长 system prompt 顶掉）
	if sp.InputSummary != "请查一下付涛" {
		t.Fatalf("input_summary = %q, want 最后一条 user", sp.InputSummary)
	}
	raw, ok := sp.Attributes["llm.messages"].([]map[string]any)
	if !ok {
		t.Fatalf("llm.messages 类型 = %T, attrs=%+v", sp.Attributes["llm.messages"], sp.Attributes)
	}
	if len(raw) != 4 {
		t.Fatalf("messages 条数 = %d, want 4", len(raw))
	}
	if raw[0]["role"] != "system" || !strings.Contains(raw[0]["content"].(string), "你是助手") {
		t.Fatalf("system 消息 = %+v", raw[0])
	}
	if strings.Contains(raw[0]["content"].(string), "sk-secret-1") {
		t.Fatalf("system content 未脱敏: %v", raw[0]["content"])
	}
	if raw[3]["role"] != "user" || raw[3]["content"] != "请查一下付涛" {
		t.Fatalf("最后一条消息 = %+v", raw[3])
	}
}

func TestHandlerLLMMessagesTruncated(t *testing.T) {
	// 超过 structuredMessageMax 条时保留首条（system）+ 最近 N-1 条
	repo := newFakeRepository()
	tracer := NewTracer(repo, Config{Enabled: true, ContentMode: "summary", MaxContentLength: 500})
	h := NewHandler(tracer)
	ctx := WithSpanContext(context.Background(), SpanContext{TraceID: "t-many", SpanID: "agent-span", Sampled: true})
	info := &callbacks.RunInfo{Name: "ChatModel", Component: components.ComponentOfChatModel}
	msgs := []*schema.Message{{Role: schema.System, Content: "system-prompt"}}
	for i := 0; i < 25; i++ {
		msgs = append(msgs, &schema.Message{Role: schema.User, Content: fmt.Sprintf("msg-%d", i)})
	}
	ctx = h.OnStart(ctx, info, &model.CallbackInput{Messages: msgs, Config: &model.Config{Model: "m"}})
	sp := repo.started[0]
	raw, ok := sp.Attributes["llm.messages"].([]map[string]any)
	if !ok {
		t.Fatalf("llm.messages 类型 = %T", sp.Attributes["llm.messages"])
	}
	if len(raw) != structuredMessageMax {
		t.Fatalf("保留条数 = %d, want %d", len(raw), structuredMessageMax)
	}
	if raw[0]["content"] != "system-prompt" {
		t.Fatalf("首条 = %v, want system-prompt", raw[0])
	}
	if raw[len(raw)-1]["content"] != "msg-24" {
		t.Fatalf("末条 = %v, want msg-24", raw[len(raw)-1])
	}
	// input_summary 同样取最后一条 user
	if sp.InputSummary != "msg-24" {
		t.Fatalf("input_summary = %q, want msg-24", sp.InputSummary)
	}
}

// 保留旧 fakeRepo 供 init.go 的 FinishTrace 兼容入口编译（迁移期）
var _ = time.Time{}
var _ = entity.AgentTrace{}
