package trace

import (
	"context"
	"fmt"
	"strings"
	"time"

	"Qavor/internal/model/entity"
	"Qavor/pkg/logger"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	callbackstpl "github.com/cloudwego/eino/utils/callbacks"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// collector 公共 span 生命周期管理（各组件 handler 复用）
type collector struct {
	maxLen int
}

func newCollector() *collector { return &collector{maxLen: globalMaxLen} }

// start 创建 span 并注入 ctx，返回携带 span 状态的 ctx
func (c *collector) start(ctx context.Context, info *callbacks.RunInfo, kind, name string, summary string) context.Context {
	if !globalEnabled || globalRepo == nil || info == nil {
		return ctx
	}
	tc := FromContext(ctx)
	if tc == nil {
		return ctx
	}
	tc.ensureRoot(ctx, globalRepo, globalMaxLen)

	now := time.Now()
	span := &entity.AgentTraceSpan{
		TraceID:      tc.TraceID,
		SpanID:       uuid.New().String(),
		Kind:         kind,
		Name:         name,
		Status:       entity.SpanStatusRunning,
		StartedAt:    now,
		InputSummary: truncate(summary, globalMaxLen),
		CreatedAt:    now,
	}
	if ps := SpanFromContext(ctx); ps != nil {
		span.ParentSpanID = ps.ID
	}
	if span.Name == "" {
		span.Name = info.Name
	}
	if err := globalRepo.CreateSpan(ctx, span); err != nil {
		logger.Warn("trace: 创建 span 失败", zap.String("trace_id", tc.TraceID), zap.String("kind", kind), zap.Error(err))
		return ctx
	}
	return WithSpan(ctx, &spanState{ID: span.SpanID, StartedAt: now})
}

// complete 补全 span（status/结果摘要/token/耗时），返回更新后的实体
func (c *collector) complete(ctx context.Context, status, summary, errMsg string, tokensIn, tokensOut, reasoningTokens int) {
	if !globalEnabled || globalRepo == nil {
		return
	}
	st := SpanFromContext(ctx)
	tc := FromContext(ctx)
	if st == nil || tc == nil {
		return
	}
	now := time.Now()
	upd := &entity.AgentTraceSpan{
		TraceID:         tc.TraceID,
		SpanID:          st.ID,
		Status:          status,
		EndedAt:         &now,
		DurationMs:      now.Sub(st.StartedAt).Milliseconds(),
		OutputSummary:   truncate(summary, globalMaxLen),
		ErrorMessage:    truncate(errMsg, globalMaxLen),
		TokensIn:        tokensIn,
		TokensOut:       tokensOut,
		ReasoningTokens: reasoningTokens,
	}
	if err := globalRepo.UpdateSpan(ctx, upd); err != nil {
		logger.Warn("trace: 更新 span 失败", zap.String("trace_id", tc.TraceID), zap.String("span_id", st.ID), zap.Error(err))
	}
}

// NewHandler 创建全局采集器（进程启动时经 callbacks.AppendGlobalHandlers 注册一次）。
// 采用 eino 官方 utils/callbacks.HandlerHelper 按组件类型分发。
func NewHandler() callbacks.Handler {
	col := newCollector()

	modelH := &callbackstpl.ModelCallbackHandler{
		OnStart: func(ctx context.Context, info *callbacks.RunInfo, input *model.CallbackInput) context.Context {
			name, summary := "", ""
			if input != nil {
				if input.Config != nil {
					name = input.Config.Model
				}
				summary = promptSummary(input.Messages)
			}
			return col.start(ctx, info, entity.SpanKindLLM, name, summary)
		},
		OnEnd: func(ctx context.Context, info *callbacks.RunInfo, output *model.CallbackOutput) context.Context {
			summary := ""
			var tin, tout, treason int
			if output != nil {
				summary = messageText(output.Message)
				if tu := output.TokenUsage; tu != nil {
					tin = tu.PromptTokens
					tout = tu.CompletionTokens
					treason = tu.CompletionTokensDetails.ReasoningTokens
				}
			}
			col.complete(ctx, entity.SpanStatusSuccess, summary, "", tin, tout, treason)
			return ctx
		},
		OnError: func(ctx context.Context, info *callbacks.RunInfo, err error) context.Context {
			col.complete(ctx, entity.SpanStatusError, "", err.Error(), 0, 0, 0)
			return ctx
		},
	}

	toolH := &callbackstpl.ToolCallbackHandler{
		OnStart: func(ctx context.Context, info *callbacks.RunInfo, input *tool.CallbackInput) context.Context {
			summary := ""
			if input != nil {
				summary = input.ArgumentsInJSON
			}
			return col.start(ctx, info, entity.SpanKindTool, "", summary)
		},
		OnEnd: func(ctx context.Context, info *callbacks.RunInfo, output *tool.CallbackOutput) context.Context {
			summary := ""
			if output != nil {
				summary = output.Response
			}
			col.complete(ctx, entity.SpanStatusSuccess, summary, "", 0, 0, 0)
			return ctx
		},
		OnError: func(ctx context.Context, info *callbacks.RunInfo, err error) context.Context {
			col.complete(ctx, entity.SpanStatusError, "", err.Error(), 0, 0, 0)
			return ctx
		},
	}

	retrieverH := &callbackstpl.RetrieverCallbackHandler{
		OnStart: func(ctx context.Context, info *callbacks.RunInfo, input *retriever.CallbackInput) context.Context {
			summary := ""
			if input != nil {
				summary = input.Query
			}
			return col.start(ctx, info, entity.SpanKindRetriever, "", summary)
		},
		OnEnd: func(ctx context.Context, info *callbacks.RunInfo, output *retriever.CallbackOutput) context.Context {
			summary := ""
			if output != nil {
				summary = docsSummary(output.Docs)
			}
			col.complete(ctx, entity.SpanStatusSuccess, summary, "", 0, 0, 0)
			return ctx
		},
		OnError: func(ctx context.Context, info *callbacks.RunInfo, err error) context.Context {
			col.complete(ctx, entity.SpanStatusError, "", err.Error(), 0, 0, 0)
			return ctx
		},
	}

	// Agent OnEnd 的 output 是异步 Events 迭代器，只标记成功不消费
	agentH := &callbackstpl.AgentCallbackHandler{
		OnStart: func(ctx context.Context, info *callbacks.RunInfo, input *adk.AgentCallbackInput) context.Context {
			summary := ""
			if input != nil {
				summary = agentInputSummary(input)
			}
			return col.start(ctx, info, entity.SpanKindAgent, info.Name, summary)
		},
		OnEnd: func(ctx context.Context, info *callbacks.RunInfo, output *adk.AgentCallbackOutput) context.Context {
			col.complete(ctx, entity.SpanStatusSuccess, "", "", 0, 0, 0)
			return ctx
		},
	}

	// Graph 需要完整 callbacks.Handler（composeTemplates 接受）
	graphH := &graphHandler{col: col}

	return callbackstpl.NewHandlerHelper().
		ChatModel(modelH).
		Tool(toolH).
		Retriever(retrieverH).
		Agent(agentH).
		Graph(graphH).
		Handler()
}

// graphHandler Graph 组件的通用 handler（kind=agent）
type graphHandler struct {
	col *collector
}

func (g *graphHandler) OnStart(ctx context.Context, info *callbacks.RunInfo, input callbacks.CallbackInput) context.Context {
	return g.col.start(ctx, info, entity.SpanKindAgent, "", "")
}

func (g *graphHandler) OnEnd(ctx context.Context, info *callbacks.RunInfo, output callbacks.CallbackOutput) context.Context {
	g.col.complete(ctx, entity.SpanStatusSuccess, "", "", 0, 0, 0)
	return ctx
}

func (g *graphHandler) OnError(ctx context.Context, info *callbacks.RunInfo, err error) context.Context {
	g.col.complete(ctx, entity.SpanStatusError, "", err.Error(), 0, 0, 0)
	return ctx
}

func (g *graphHandler) OnStartWithStreamInput(ctx context.Context, info *callbacks.RunInfo, input *schema.StreamReader[callbacks.CallbackInput]) context.Context {
	return ctx
}

func (g *graphHandler) OnEndWithStreamOutput(ctx context.Context, info *callbacks.RunInfo, output *schema.StreamReader[callbacks.CallbackOutput]) context.Context {
	return ctx
}

func (g *graphHandler) Needed(_ context.Context, _ *callbacks.RunInfo, timing callbacks.CallbackTiming) bool {
	return timing == callbacks.TimingOnStart || timing == callbacks.TimingOnEnd || timing == callbacks.TimingOnError
}

// promptSummary 拼接主要消息文本
func promptSummary(msgs []*schema.Message) string {
	if len(msgs) == 0 {
		return ""
	}
	var sb strings.Builder
	for _, m := range msgs {
		if m == nil {
			continue
		}
		if text := messageText(m); text != "" {
			sb.WriteString(text)
			sb.WriteString("\n")
		}
	}
	return strings.TrimSpace(sb.String())
}

// messageText 提取消息文本（Content / MultiContent / AssistantGenMultiContent 中的 text/reasoning part）
func messageText(m *schema.Message) string {
	if m == nil {
		return ""
	}
	if m.Content != "" {
		return m.Content
	}
	var sb strings.Builder
	for _, part := range m.MultiContent {
		if part.Type == schema.ChatMessagePartTypeText {
			sb.WriteString(part.Text)
		}
	}
	for _, part := range m.AssistantGenMultiContent {
		switch part.Type {
		case schema.ChatMessagePartTypeText:
			sb.WriteString(part.Text)
		case schema.ChatMessagePartTypeReasoning:
			if part.Reasoning != nil {
				sb.WriteString(part.Reasoning.Text)
			}
		}
	}
	return sb.String()
}

// docsSummary 文档摘要：拼接前 3 条内容
func docsSummary(docs []*schema.Document) string {
	if len(docs) == 0 {
		return ""
	}
	limit := 3
	if len(docs) < limit {
		limit = len(docs)
	}
	parts := make([]string, 0, limit)
	for i := 0; i < limit; i++ {
		parts = append(parts, docs[i].Content)
	}
	s := strings.Join(parts, "\n---\n")
	if len(docs) > limit {
		s += fmt.Sprintf("\n...（共 %d 条）", len(docs))
	}
	return s
}

// agentInputSummary 提取 Agent 输入中的用户消息摘要（AgentInput 为 TypedAgentInput[*schema.Message] 的类型别名）
func agentInputSummary(input *adk.AgentCallbackInput) string {
	if input == nil || input.Input == nil {
		return ""
	}
	return promptSummary(input.Input.Messages)
}
