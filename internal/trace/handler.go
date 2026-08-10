package trace

import (
	"context"
	"strings"

	"Qavor/internal/model/entity"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	callbackstpl "github.com/cloudwego/eino/utils/callbacks"
)

// callbackSpanKey 用于在 context 中存储当前组件 Span 句柄（OnStart 注入，OnEnd/OnError 读取）
type callbackSpanKey struct{}

func withCallbackSpan(ctx context.Context, span *Span) context.Context {
	return context.WithValue(ctx, callbackSpanKey{}, span)
}

func callbackSpanFromContext(ctx context.Context) *Span {
	span, _ := ctx.Value(callbackSpanKey{}).(*Span)
	return span
}

// callbackCollector 基于 Tracer 的组件 Span 采集器。
// 只创建组件 Span（llm/tool/retriever/agent），不创建根 TraceRecord，不结束父 Span。
type callbackCollector struct {
	tracer    *Tracer
	sanitizer Sanitizer
}

func newCallbackCollector(tracer *Tracer) *callbackCollector {
	mode := "summary"
	maxLen := 500
	if tracer != nil {
		mode = tracer.ContentMode()
		maxLen = tracer.MaxContentLength()
	}
	return &callbackCollector{
		tracer:    tracer,
		sanitizer: Sanitizer{Mode: mode, MaxRunes: maxLen},
	}
}

// start 创建组件 Span 并注入 ctx。没有 SpanContext 时跳过（不创建根 Trace）。
func (c *callbackCollector) start(ctx context.Context, info *callbacks.RunInfo, kind, operation, displayName, inputSummary string, attrs entity.JSON) context.Context {
	if c.tracer == nil {
		return ctx
	}
	if _, ok := SpanContextFromContext(ctx); !ok {
		return ctx
	}
	if info != nil && displayName == "" {
		displayName = info.Name
	}
	spec := SpanSpec{
		Kind:         kind,
		Operation:    operation,
		DisplayName:  displayName,
		InputSummary: inputSummary,
		Attributes:   attrs,
	}
	newCtx, span := c.tracer.StartSpan(ctx, spec)
	return withCallbackSpan(newCtx, span)
}

// end 结束组件 Span（幂等，仅第一次生效）。
func (c *callbackCollector) end(ctx context.Context, end SpanEnd) {
	span := callbackSpanFromContext(ctx)
	if span == nil {
		return
	}
	span.End(end)
}

// NewHandler 创建全局采集器（进程启动时经 callbacks.AppendGlobalHandlers 注册一次）。
// tracer 为 nil 时所有回调为 no-op。
func NewHandler(tracer *Tracer) callbacks.Handler {
	col := newCallbackCollector(tracer)

	modelH := &callbackstpl.ModelCallbackHandler{
		OnStart: func(ctx context.Context, info *callbacks.RunInfo, input *model.CallbackInput) context.Context {
			name, summary := "", ""
			attrs := entity.JSON{}
			if input != nil {
				if input.Config != nil {
					cfg := input.Config
					name = cfg.Model
					// 记录模型调用参数（提示词本身已写入 InputSummary）
					if cfg.Temperature != 0 {
						attrs["temperature"] = cfg.Temperature
					}
					if cfg.TopP != 0 {
						attrs["top_p"] = cfg.TopP
					}
					if cfg.MaxTokens != 0 {
						attrs["max_tokens"] = cfg.MaxTokens
					}
					if len(cfg.Stop) > 0 {
						attrs["stop"] = cfg.Stop
					}
				}
				if len(input.Tools) > 0 {
					attrs["tool_count"] = len(input.Tools)
				}
				if input.ToolChoice != nil {
					attrs["tool_choice"] = string(*input.ToolChoice)
				}
				if len(attrs) == 0 {
					attrs = nil
				}
				summary = col.sanitizer.Text(promptSummary(input.Messages))
			}
			return col.start(ctx, info, "llm", "llm.generate", name, summary, attrs)
		},
		OnEnd: func(ctx context.Context, info *callbacks.RunInfo, output *model.CallbackOutput) context.Context {
			// 非流式 Generate 只会调用一次 OnEnd（流式走 OnEndWithStreamOutput），
			// 此处直接结束 span，避免缺少 finish_reason/usage 时 span 永远停在 running、输出不落库。
			col.end(ctx, col.buildModelSpanEnd(output))
			return ctx
		},
		// OnEndWithStreamOutput 处理流式模型调用：框架为各 handler 提供独立的 stream 副本，
		// 此处必须在 goroutine 中消费副本——框架会在各 handler 返回后才把业务 reader 交出去，
		// 若同步 drain 会阻塞业务流式输出（用户侧首 token 需等模型整段生成完）。
		// 缺失该钩子会导致流式路径下 Needed(TimingOnEndWithStreamOutput) 为假，
		// handler 被过滤，token 永远记为 0 且 LLM span 不被正常结束。
		OnEndWithStreamOutput: func(ctx context.Context, info *callbacks.RunInfo, output *schema.StreamReader[*model.CallbackOutput]) context.Context {
			if output == nil {
				return ctx
			}
			go func() {
				defer output.Close()
				var (
					lastChunk   *model.CallbackOutput
					lastContent *schema.Message
					prevText    string
					fullText    strings.Builder
				)
				for {
					chunk, err := output.Recv()
					if err != nil {
						break
					}
					lastChunk = chunk
					if chunk == nil || chunk.Message == nil {
						continue
					}
					text := MessageTextWithoutReasoning(chunk.Message)
					if text != "" {
						// openai 适配器每个 chunk 的 Message 为累计全文，ollama 为增量 delta。
						// 用前缀去重：累计流只追加新增后缀（避免重复 N 遍），增量流整段追加（得到全文）。
						if strings.HasPrefix(text, prevText) {
							fullText.WriteString(text[len(prevText):])
						} else {
							fullText.WriteString(text)
						}
						prevText = text
						lastContent = chunk.Message
					} else if len(chunk.Message.ToolCalls) > 0 {
						// 仅工具调用的 chunk 也保留 message，以捕获 tool_call_ids。
						lastContent = chunk.Message
					}
				}
				// 用量从最后一个 chunk 取（openai 的 usage 在末尾空消息里），
				// 文本内容取合并后的全文，工具调用取自最后一个含内容的 message。
				merged := &model.CallbackOutput{Message: lastContent}
				if lastChunk != nil {
					merged.TokenUsage = lastChunk.TokenUsage
				}
				end := col.buildModelSpanEnd(merged)
				end.OutputSummary = col.sanitizer.Text(fullText.String())
				col.end(ctx, end)
			}()
			return ctx
		},
		OnError: func(ctx context.Context, info *callbacks.RunInfo, err error) context.Context {
			col.end(ctx, SpanEnd{Status: SpanStatusError, ErrorMessage: err.Error()})
			return ctx
		},
	}

	toolH := &callbackstpl.ToolCallbackHandler{
		OnStart: func(ctx context.Context, info *callbacks.RunInfo, input *tool.CallbackInput) context.Context {
			summary := ""
			attrs := entity.JSON{}
			if input != nil {
				summary = col.sanitizer.JSON(input.ArgumentsInJSON)
				if input.Extra != nil {
					if id, ok := input.Extra["tool_call_id"].(string); ok && id != "" {
						attrs["tool_call_id"] = id
					}
				}
			}
			return col.start(ctx, info, "tool", "tool.execute", "", summary, attrs)
		},
		OnEnd: func(ctx context.Context, info *callbacks.RunInfo, output *tool.CallbackOutput) context.Context {
			end := SpanEnd{Status: SpanStatusOK}
			if output != nil {
				end.OutputSummary = col.sanitizer.JSON(output.Response)
			}
			col.end(ctx, end)
			return ctx
		},
		OnError: func(ctx context.Context, info *callbacks.RunInfo, err error) context.Context {
			col.end(ctx, SpanEnd{Status: SpanStatusError, ErrorMessage: err.Error()})
			return ctx
		},
	}

	retrieverH := &callbackstpl.RetrieverCallbackHandler{
		OnStart: func(ctx context.Context, info *callbacks.RunInfo, input *retriever.CallbackInput) context.Context {
			summary := ""
			if input != nil {
				summary = col.sanitizer.Text(input.Query)
			}
			return col.start(ctx, info, "retriever", "retriever.search", "", summary, nil)
		},
		OnEnd: func(ctx context.Context, info *callbacks.RunInfo, output *retriever.CallbackOutput) context.Context {
			end := SpanEnd{Status: SpanStatusOK}
			if output != nil && len(output.Docs) > 0 {
				if end.Attributes == nil {
					end.Attributes = entity.JSON{}
				}
				end.Attributes["retriever.hit_count"] = len(output.Docs)
			}
			col.end(ctx, end)
			return ctx
		},
		OnError: func(ctx context.Context, info *callbacks.RunInfo, err error) context.Context {
			col.end(ctx, SpanEnd{Status: SpanStatusError, ErrorMessage: err.Error()})
			return ctx
		},
	}

	// Agent OnEnd 的 output 是异步 Events 迭代器，只标记成功不消费
	agentH := &callbackstpl.AgentCallbackHandler{
		OnStart: func(ctx context.Context, info *callbacks.RunInfo, input *adk.AgentCallbackInput) context.Context {
			summary := ""
			if input != nil {
				summary = col.sanitizer.Text(agentInputSummary(input))
			}
			return col.start(ctx, info, "agent", "agent.invoke", info.Name, summary, nil)
		},
		OnEnd: func(ctx context.Context, info *callbacks.RunInfo, output *adk.AgentCallbackOutput) context.Context {
			col.end(ctx, SpanEnd{Status: SpanStatusOK})
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

// callbackTokenUsage reads usage from the callback payload first and falls back
// to Message.ResponseMeta for adapters that only populate the message metadata.
func callbackTokenUsage(output *model.CallbackOutput) *model.TokenUsage {
	if output == nil {
		return nil
	}
	if output.TokenUsage != nil {
		return output.TokenUsage
	}
	if output.Message == nil || output.Message.ResponseMeta == nil || output.Message.ResponseMeta.Usage == nil {
		return nil
	}
	usage := output.Message.ResponseMeta.Usage
	return &model.TokenUsage{
		PromptTokens:     usage.PromptTokens,
		CompletionTokens: usage.CompletionTokens,
		TotalTokens:      usage.TotalTokens,
		CompletionTokensDetails: model.CompletionTokensDetails{
			ReasoningTokens: usage.CompletionTokensDetails.ReasoningTokens,
		},
	}
}

// buildModelSpanEnd 从模型回调输出构造 Span 结束数据（token 用量、输出摘要、工具调用 id）。
// 非流式 OnEnd 与流式 OnEndWithStreamOutput 共用，避免重复逻辑。
func (c *callbackCollector) buildModelSpanEnd(output *model.CallbackOutput) SpanEnd {
	end := SpanEnd{Status: SpanStatusOK}
	if output == nil {
		return end
	}
	end.OutputSummary = c.sanitizer.Text(MessageTextWithoutReasoning(output.Message))
	if tu := callbackTokenUsage(output); tu != nil {
		end.TokensIn = tu.PromptTokens
		end.TokensOut = tu.CompletionTokens
		end.ReasoningTokens = tu.CompletionTokensDetails.ReasoningTokens
	}
	if output.Message != nil && len(output.Message.ToolCalls) > 0 {
		ids := make([]string, 0, len(output.Message.ToolCalls))
		for _, tc := range output.Message.ToolCalls {
			ids = append(ids, tc.ID)
		}
		if end.Attributes == nil {
			end.Attributes = entity.JSON{}
		}
		end.Attributes["tool_call_ids"] = ids
	}
	return end
}

// graphHandler Graph 组件的通用 handler（kind=agent, operation=agent.graph）
type graphHandler struct {
	col *callbackCollector
}

func (g *graphHandler) OnStart(ctx context.Context, info *callbacks.RunInfo, input callbacks.CallbackInput) context.Context {
	return g.col.start(ctx, info, "agent", "agent.graph", "", "", nil)
}

func (g *graphHandler) OnEnd(ctx context.Context, info *callbacks.RunInfo, output callbacks.CallbackOutput) context.Context {
	g.col.end(ctx, SpanEnd{Status: SpanStatusOK})
	return ctx
}

func (g *graphHandler) OnError(ctx context.Context, info *callbacks.RunInfo, err error) context.Context {
	g.col.end(ctx, SpanEnd{Status: SpanStatusError, ErrorMessage: err.Error()})
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
		if text := MessageTextWithoutReasoning(m); text != "" {
			sb.WriteString(text)
			sb.WriteString("\n")
		}
	}
	return strings.TrimSpace(sb.String())
}

// agentInputSummary 提取 Agent 输入中的用户消息摘要（AgentInput 为 TypedAgentInput[*schema.Message] 的类型别名）
func agentInputSummary(input *adk.AgentCallbackInput) string {
	if input == nil || input.Input == nil {
		return ""
	}
	return promptSummary(input.Input.Messages)
}
