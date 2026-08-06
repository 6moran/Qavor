package run

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"Qavor/internal/agent"
	"Qavor/internal/eventbus"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
)

// ModelResolver 根据模型 ID 解析出 Eino ChatModel（由 service.ModelService 实现）
type ModelResolver interface {
	ResolveChatModel(ctx context.Context, modelID uint) (model.ToolCallingChatModel, error)
}

// agentExecutor AgentExecutor 的实现：桥接 eino adk 事件到 run.StreamEvent
type agentExecutor struct {
	agentMgr *agent.AgentManager
	resolver ModelResolver
}

// NewAgentExecutor 创建 Agent 执行适配器
func NewAgentExecutor(agentMgr *agent.AgentManager, resolver ModelResolver) AgentExecutor {
	return &agentExecutor{agentMgr: agentMgr, resolver: resolver}
}

// Execute 执行 Agent，通过 emit 回调发出流式事件，返回完整的 Assistant 消息列表（用于持久化）
func (e *agentExecutor) Execute(ctx context.Context, slug, query string, emit func(StreamEvent)) ([]*schema.Message, error) {
	// 1. 获取 Agent 配置，解析模型 ID
	cfg, err := e.agentMgr.GetConfig(ctx, slug)
	if err != nil {
		return nil, fmt.Errorf("获取 Agent 配置失败: %w", err)
	}

	var llm model.ToolCallingChatModel
	if modelIDStr, ok := cfg["model_id"].(string); ok && modelIDStr != "" {
		modelID, parseErr := strconv.ParseUint(modelIDStr, 10, 32)
		if parseErr == nil && e.resolver != nil {
			llm, _ = e.resolver.ResolveChatModel(ctx, uint(modelID))
		}
	}
	if llm == nil {
		return nil, fmt.Errorf("Agent 的 LLM 配置为空，请检查 agent_slug: %s 对应的模型配置", slug)
	}

	// 2. 获取/创建 Agent
	a, err := e.agentMgr.GetOrCreate(ctx, slug, llm)
	if err != nil {
		return nil, fmt.Errorf("获取 Agent 失败: %w", err)
	}

	// 3. 执行并遍历事件
	var assistantMsgs []*schema.Message
	iter := a.ExecuteIter(ctx, query)
	for {
		event, ok := iter.Next()
		if !ok {
			break
		}
		if event.Err != nil {
			return assistantMsgs, fmt.Errorf("agent 事件错误: %w", event.Err)
		}
		// 工具审批中断
		if event.Action != nil && event.Action.Interrupted != nil {
			return assistantMsgs, ErrInterrupted
		}
		if event.Output == nil || event.Output.MessageOutput == nil {
			continue
		}
		mv := event.Output.MessageOutput
		switch mv.Role {
		case schema.Assistant:
			if msg := e.emitAssistant(ctx, mv, emit); msg != nil {
				assistantMsgs = append(assistantMsgs, msg)
			}
		case schema.Tool:
			emit(StreamEvent{
				Type:      "tool_result",
				MessageID: uuid.New().String(),
				Role:      "tool",
				Content:   mv.Message.Content,
				ToolCall:  &eventbus.ToolCallInfo{Name: mv.ToolName},
			})
		}
	}
	return assistantMsgs, nil
}

// emitAssistant 处理 Assistant 输出：流式 token / 非流式完整消息 / 工具调用
// 返回累积完整的 Assistant 消息（用于持久化）
func (e *agentExecutor) emitAssistant(ctx context.Context, mv *adk.TypedMessageVariant[*schema.Message], emit func(StreamEvent)) *schema.Message {
	if mv == nil {
		return nil
	}
	msgID := uuid.New().String()

	// 流式输出：逐 chunk 发 text_delta，共享 msgID
	if mv.IsStreaming && mv.MessageStream != nil {
		content := e.emitStream(ctx, mv.MessageStream, msgID, emit)
		emit(StreamEvent{Type: "message_end", MessageID: msgID, Role: "assistant"})
		return &schema.Message{Role: schema.Assistant, Content: content}
	}

	// 非流式完整消息
	if mv.Message != nil {
		if mv.Message.Content != "" {
			emit(StreamEvent{
				Type:      "text_delta",
				MessageID: msgID,
				Role:      "assistant",
				Content:   mv.Message.Content,
			})
		}
		// 工具调用
		for _, tc := range mv.Message.ToolCalls {
			idx := 0
			if tc.Index != nil {
				idx = *tc.Index
			}
			emit(StreamEvent{
				Type:      "tool_call",
				MessageID: msgID,
				Role:      "assistant",
				ToolCall: &eventbus.ToolCallInfo{
					ID:    tc.ID,
					Name:  tc.Function.Name,
					Args:  tc.Function.Arguments,
					Index: idx,
				},
			})
		}
		emit(StreamEvent{Type: "message_end", MessageID: msgID, Role: "assistant"})
		return mv.Message
	}
	return nil
}

// emitStream 读取流式输出，逐 chunk 发出 text_delta 事件，返回累积的完整内容
func (e *agentExecutor) emitStream(ctx context.Context, stream *schema.StreamReader[*schema.Message], msgID string, emit func(StreamEvent)) string {
	defer stream.Close()
	var sb strings.Builder
	for {
		if ctx.Err() != nil {
			return sb.String()
		}
		chunk, err := stream.Recv()
		if err != nil {
			return sb.String()
		}
		if chunk != nil && chunk.Content != "" {
			sb.WriteString(chunk.Content)
			emit(StreamEvent{
				Type:      "text_delta",
				MessageID: msgID,
				Role:      "assistant",
				Content:   chunk.Content,
			})
		}
	}
}
