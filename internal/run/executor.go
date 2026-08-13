package run

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"Qavor/internal/agent"
	"Qavor/internal/agent/localfs/security"
	"Qavor/internal/eventbus"
	"Qavor/pkg/logger"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ModelResolver 根据模型 ID 解析出 Eino ChatModel（由 service.ModelService 实现）
type ModelResolver interface {
	ResolveChatModel(ctx context.Context, modelID uint) (model.ToolCallingChatModel, error)
}

// ApprovalRequest 待审批的工具调用（供前端展示 + SSE 发布）。
type ApprovalRequest struct {
	ToolName string `json:"name"`
	Args     string `json:"args"`
}

// InterruptedError 工具审批中断错误，携带恢复所需信息。
type InterruptedError struct {
	// CheckpointID 中断时保存的 checkpoint key（resume 用）。
	CheckpointID string
	// Requests 待审批的工具调用列表（action_requests）。
	Requests []ApprovalRequest
	// InterruptIDs 各中断上下文的 UUID（用于构建 resume targets key，与 eino id2Addr 的 key 匹配）。
	InterruptIDs []string
	// IsQuestion 是否为 ask_user 提问中断。
	IsQuestion bool
	// Questions 提问中断携带的问题列表（IsQuestion=true 时有效）。
	Questions []agent.AskUserQuestion
}

// Error 实现 error 接口。
func (e *InterruptedError) Error() string {
	return "agent interrupted for tool approval"
}

// Is 让 errors.Is(err, ErrInterrupted) 对 InterruptedError 成立（保留旧判定兼容）。
func (e *InterruptedError) Is(target error) bool {
	return target == ErrInterrupted
}

// agentExecutor Agent 执行器：桥接 eino adk 事件到 run.StreamEvent
type agentExecutor struct {
	agentMgr *agent.AgentManager
	resolver ModelResolver
}

// NewAgentExecutor 创建 Agent 执行适配器
func NewAgentExecutor(agentMgr *agent.AgentManager, resolver ModelResolver) AgentExecutor {
	return &agentExecutor{agentMgr: agentMgr, resolver: resolver}
}

// GetModelID 从 Agent 配置中解析模型 ID
func (e *agentExecutor) GetModelID(ctx context.Context, slug string) uint {
	cfg, err := e.agentMgr.GetConfig(ctx, slug)
	if err != nil {
		return 0
	}
	if modelIDStr, ok := cfg["model_id"].(string); ok && modelIDStr != "" {
		if modelID, err := strconv.ParseUint(modelIDStr, 10, 32); err == nil {
			return uint(modelID)
		}
	}
	return 0
}

// ExecuteOption 执行选项（向后兼容函数选项模式）。
type ExecuteOption func(*executeOptions)

type executeOptions struct {
	approvalMode string            // 审批模式（default/always_trust）
	resume       *agentResumeParam // 非 nil 时走 resume 执行
}

// agentResumeParam resume 执行参数。
type agentResumeParam struct {
	checkpointID string
	targets      map[string]any // 中断地址 → 审批决定
}

// WithApprovalMode 设置审批模式（default/always_trust）。
func WithApprovalMode(mode string) ExecuteOption {
	return func(o *executeOptions) { o.approvalMode = mode }
}

// WithResume 设置 resume 执行参数（审批恢复）。
func WithResume(checkpointID string, targets map[string]any) ExecuteOption {
	return func(o *executeOptions) {
		o.resume = &agentResumeParam{checkpointID: checkpointID, targets: targets}
	}
}

// Execute 执行 Agent，通过 emit 回调发出流式事件，返回完整的 Assistant 消息列表（用于持久化）。
// 支持两种模式：
//   - 首次执行：正常 Run，敏感工具触发审批时返回 *InterruptedError。
//   - resume 恢复：WithResume 提供 checkpointID + 审批决定，从断点继续执行。
func (e *agentExecutor) Execute(ctx context.Context, slug, query string, history []*schema.Message, emit func(StreamEvent), opts ...ExecuteOption) ([]*schema.Message, error) {
	opt := &executeOptions{}
	for _, o := range opts {
		o(opt)
	}

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

	// 3. 审批模式写入 ctx（ApprovalMiddleware 从 ctx 读取）
	ctx = agent.WithApprovalMode(ctx, opt.approvalMode)

	// 4. 执行（首次 Run 或 resume）并遍历事件
	var assistantMsgs []*schema.Message
	var iter *agent.AgentEventIterator
	if opt.resume != nil {
		iter, err = a.Resume(ctx, opt.resume.checkpointID, opt.resume.targets)
		if err != nil {
			return nil, fmt.Errorf("恢复 Agent 执行失败: %w", err)
		}
	} else {
		iter = a.ExecuteIter(ctx, query, history...)
	}
	for {
		if ctx.Err() != nil {
			return assistantMsgs, nil
		}
		event, ok := iter.Next()
		if !ok {
			break
		}
		if event.Err != nil {
			return assistantMsgs, fmt.Errorf("agent 事件错误: %w", event.Err)
		}
		// 工具审批中断
		if event.Action != nil && event.Action.Interrupted != nil {
			return assistantMsgs, e.interruptedError(event.Action.Interrupted)
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
			// 安全策略黑名单拦截：工具结果文本携带 SecurityBlockMarker。
			// 一旦命中即终止本轮 run，向用户说明后停止，从结构上杜绝模型尝试绕过黑名单。
			// 注意：流式工具结果（IsStreaming=true）的 GetMessage 会消费底层流，
			// 因此本分支内只获取一次消息，并复用给 toolResultEvent，避免二次消费导致内容丢失。
			msg, _ := mv.GetMessage()
			if msg != nil && strings.Contains(msg.Content, security.SecurityBlockMarker) {
				const finalMsg = "该命令被安全策略阻止执行（不在允许的执行范围内）。已停止本次操作。"
				finalID := uuid.New().String()
				emit(StreamEvent{Type: "text_delta", MessageID: finalID, Role: "assistant", Content: finalMsg})
				emit(StreamEvent{Type: "message_end", MessageID: finalID, Role: "assistant"})
				assistantMsgs = append(assistantMsgs, &schema.Message{Role: schema.Assistant, Content: finalMsg})
				return assistantMsgs, nil
			}
			// 工具结果可能是流式（Message 为 nil，内容在 MessageStream 中）；
			// 已在上方通过 GetMessage() 一次性合并完整消息，复用以避免重复消费流。
			emit(toolResultEvent(mv, msg))
		}
	}
	return assistantMsgs, nil
}

// interruptedError 从中断事件提取审批信息。
// CheckPointID 在顶层 InterruptInfo；审批的工具名/参数在根因 InterruptCtx 的 Info 中
// （由 ApprovalMiddleware 以 *agent.ApprovalRequest 或 *agent.AskUserRequest 塞入）。
func (e *agentExecutor) interruptedError(in *adk.InterruptInfo) *InterruptedError {
	ie := &InterruptedError{CheckpointID: in.CheckPointID}

	if logger.Initialized() {
		logger.GetLogger().Info("interruptedError 收到中断信号",
			zap.String("checkpoint_id", in.CheckPointID),
			zap.Int("context_count", len(in.InterruptContexts)),
		)
	}

	// 遍历中断上下文链，收集根因及所有层级的工具审批请求
	var walk func(ictx *adk.InterruptCtx)
	walk = func(ictx *adk.InterruptCtx) {
		if ictx == nil {
			return
		}

		infoType := "nil"
		if ictx.Info != nil {
			infoType = fmt.Sprintf("%T", ictx.Info)
		}
		if logger.Initialized() {
			logger.GetLogger().Info("interruptedError 遍历 InterruptCtx",
				zap.String("id", ictx.ID),
				zap.Bool("is_root_cause", ictx.IsRootCause),
				zap.String("info_type", infoType),
				zap.Any("info_value", ictx.Info),
			)
		}

		// 只收集根因（工具级）的中断 UUID，避免 agent 层级的节点收到
		// string resume data 导致 ChatModelAgent panic。
		if ictx.ID != "" && ictx.IsRootCause {
			ie.InterruptIDs = append(ie.InterruptIDs, ictx.ID)
		}
		switch info := ictx.Info.(type) {
		case *agent.ApprovalRequest:
			if logger.Initialized() {
				logger.GetLogger().Info("interruptedError 匹配到 ApprovalRequest",
					zap.String("tool_name", info.ToolName),
					zap.String("args", info.Args),
				)
			}
			ie.Requests = append(ie.Requests, ApprovalRequest{ToolName: info.ToolName, Args: info.Args})
		case *agent.AskUserRequest:
			if logger.Initialized() {
				logger.GetLogger().Info("interruptedError 匹配到 AskUserRequest",
					zap.Int("question_count", len(info.Questions)),
				)
			}
			ie.IsQuestion = true
			ie.Questions = info.Questions
		default:
			if logger.Initialized() {
				logger.GetLogger().Info("interruptedError 未匹配的类型",
					zap.String("info_type", infoType),
				)
			}
		}
		walk(ictx.Parent)
	}
	for _, c := range in.InterruptContexts {
		walk(c)
	}

	if logger.Initialized() {
		logger.GetLogger().Info("interruptedError 结果",
			zap.String("checkpoint_id", ie.CheckpointID),
			zap.Bool("is_question", ie.IsQuestion),
			zap.Int("question_count", len(ie.Questions)),
			zap.Int("request_count", len(ie.Requests)),
			zap.Int("interrupt_ids_count", len(ie.InterruptIDs)),
		)
	}

	return ie
}

// emitAssistant 处理 Assistant 输出：流式 token / 非流式完整消息 / 工具调用
// 返回累积完整的 Assistant 消息（用于持久化）
func (e *agentExecutor) emitAssistant(ctx context.Context, mv *adk.TypedMessageVariant[*schema.Message], emit func(StreamEvent)) *schema.Message {
	if mv == nil {
		return nil
	}
	msgID := uuid.New().String()

	// 流式输出：逐 chunk 发 text_delta，共享 msgID
	// 流式工具调用（chunk.ToolCalls）在流结束后统一补发，避免事件丢失
	if mv.IsStreaming && mv.MessageStream != nil {
		content, reasoningContent, toolCalls := e.emitStream(ctx, mv.MessageStream, msgID, emit)
		// ctx 取消时跳过后续 emit，但仍返回已累积的内容用于持久化
		if ctx.Err() == nil {
			for _, tc := range toolCalls {
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
		}
		return &schema.Message{
			Role:             schema.Assistant,
			Content:          content,
			ReasoningContent: reasoningContent,
			ToolCalls:        toolCalls,
		}
	}

	// 非流式完整消息
	if mv.Message != nil {
		// ctx 取消时跳过所有 emit，避免继续推送事件
		if ctx.Err() == nil {
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
				// 拦截 write_todos 工具调用，提取 TODO 列表
				if tc.Function.Name == "write_todos" {
					if todos := parseWriteTodosArgs(tc.Function.Arguments); todos != nil {
						emit(StreamEvent{
							Type:  "todo_update",
							Role:  "assistant",
							Todos: todos,
						})
					}
				}
			}
			emit(StreamEvent{Type: "message_end", MessageID: msgID, Role: "assistant"})
		}
		return mv.Message
	}
	return nil
}

// toolResultEvent 提取工具结果消息，并保留调用 ID 供下游关联。
// 非流式 Tool 事件内容在 mv.Message 中；流式 Tool 事件（可流式工具）
// Message 为 nil，需消费 MessageStream 合并。msg 参数为调用方已合并的完整消息
// （经一次 GetMessage() 取得并复用），避免在流式场景下二次消费流导致内容丢失。
func toolResultEvent(mv *adk.MessageVariant, msg *schema.Message) StreamEvent {
	event := StreamEvent{
		Type:      "tool_result",
		MessageID: uuid.New().String(),
		Role:      "tool",
		ToolCall:  &eventbus.ToolCallInfo{},
	}
	if mv == nil {
		return event
	}
	event.ToolCall.Name = mv.ToolName
	if msg != nil {
		event.Content = msg.Content
		event.ToolCall.ID = msg.ToolCallID
		if event.ToolCall.Name == "" {
			event.ToolCall.Name = msg.ToolName
		}
	}
	return event
}

// emitStream 读取流式输出，逐 chunk 发出 text_delta 事件，返回累积的完整内容与推理内容。
// 同时收集流式工具调用：chunk.ToolCalls 中每个 index 的 ID/name 相同、args 为增量片段，
// 按 index 拼接为完整的 ToolCall，供调用方在流结束后补发 tool_call 事件。
//
// 使用 goroutine + select 模式读取 stream.Recv()，确保 ctx 取消时能立即返回，
// 不会阻塞在 LLM 流式 API 的 HTTP 响应体读取上。
// ctx 取消时 stream.Close() 在独立 goroutine 中执行，避免同步关闭阻塞。
func (e *agentExecutor) emitStream(ctx context.Context, stream *schema.StreamReader[*schema.Message], msgID string, emit func(StreamEvent)) (string, string, []schema.ToolCall) {
	var sb strings.Builder
	var reasoningSb strings.Builder
	startTime := time.Now()
	chunkCount := 0
	firstChunkAt := time.Time{}
	// ctx 取消时 stream.Close() 在 goroutine 中执行，避免同步关闭阻塞
	defer func() {
		if ctx.Err() != nil {
			go stream.Close()
		} else {
			stream.Close()
		}
	}()
	defer func() {
		if chunkCount > 0 {
			logger.GetLogger().Debug("流式输出统计",
				zap.Int("chunk_count", chunkCount),
				zap.Int("total_chars", sb.Len()),
				zap.Int("reasoning_chars", reasoningSb.Len()),
				zap.Duration("first_token_latency", firstChunkAt.Sub(startTime)),
				zap.Duration("total_duration", time.Since(startTime)),
			)
		}
	}()
	// index → toolCalls 切片位置，避免 map 与切片双份拷贝不同步
	toolCallIdx := make(map[int]int)
	var toolCalls []schema.ToolCall
	mergeToolCall := func(tc *schema.ToolCall) {
		idx := 0
		if tc.Index != nil {
			idx = *tc.Index
		}
		if pos, ok := toolCallIdx[idx]; ok {
			existing := &toolCalls[pos]
			if tc.ID != "" {
				existing.ID = tc.ID
			}
			if tc.Function.Name != "" {
				existing.Function.Name = tc.Function.Name
			}
			existing.Function.Arguments += tc.Function.Arguments
			return
		}
		clone := *tc
		toolCallIdx[idx] = len(toolCalls)
		toolCalls = append(toolCalls, clone)
	}

	// recvResult 封装 stream.Recv() 的返回值，配合 select 响应 ctx 取消
	type recvResult struct {
		chunk *schema.Message
		err   error
	}
	recvCh := make(chan recvResult, 1)
	// 启动首次 recv
	go func() {
		chunk, err := stream.Recv()
		recvCh <- recvResult{chunk, err}
	}()

	for {
		select {
		case <-ctx.Done():
			return sb.String(), reasoningSb.String(), toolCalls
		case res := <-recvCh:
			if res.err != nil {
				return sb.String(), reasoningSb.String(), toolCalls
			}
			chunk := res.chunk
			// 启动下一次 recv（无论 chunk 是否为 nil 都继续）
			go func() {
				c, err := stream.Recv()
				recvCh <- recvResult{c, err}
			}()
			if chunk == nil {
				continue
			}
			if chunk.Content != "" {
				if chunkCount == 0 {
					firstChunkAt = time.Now()
				}
				chunkCount++
				sb.WriteString(chunk.Content)
				emit(StreamEvent{
					Type:      "text_delta",
					MessageID: msgID,
					Role:      "assistant",
					Content:   chunk.Content,
				})
			}
			// 提取推理内容：
			// 1. 优先读 Message.ReasoningContent 字段（eino v0.10.0+ 原生支持，stream chunk 直接携带）
			// 2. 兜底从 AssistantGenMultiContent 的 reasoning part 提取
			reasoningContent := chunk.ReasoningContent
			if reasoningContent == "" {
				reasoningContent = extractReasoning(chunk)
			}
			if reasoningContent != "" {
				reasoningSb.WriteString(reasoningContent)
				if logger.Initialized() {
					logger.GetLogger().Debug("emitStream 提取到推理内容",
						zap.Int("reasoning_chars", len(reasoningContent)),
						zap.String("reasoning_preview", truncateStr(reasoningContent, 120)))
				}
				emit(StreamEvent{
					Type:      "text_delta",
					MessageID: msgID,
					Role:      "assistant",
					Reasoning: reasoningContent,
				})
			}
			for i := range chunk.ToolCalls {
				mergeToolCall(&chunk.ToolCalls[i])
			}
		}
	}
}

// truncateStr 截断字符串用于日志预览
func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// extractReasoning 从流式消息中提取推理内容增量（reasoning part）
func extractReasoning(m *schema.Message) string {
	if m == nil {
		return ""
	}
	for _, part := range m.AssistantGenMultiContent {
		if part.Type == schema.ChatMessagePartTypeReasoning && part.Reasoning != nil {
			return part.Reasoning.Text
		}
	}
	return ""
}

// writeTodosArgs write_todos 工具的参数结构（对应 eino deep.writeTodosArguments）
type writeTodosArgs struct {
	Todos []struct {
		Content    string `json:"content"`
		ActiveForm string `json:"activeForm"`
		Status     string `json:"status"`
	} `json:"todos"`
}

// parseWriteTodosArgs 解析 write_todos 工具调用参数，返回前端可用的 TodoItem 列表
func parseWriteTodosArgs(argsJSON string) []eventbus.TodoItem {
	if argsJSON == "" {
		return nil
	}
	var args writeTodosArgs
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return nil
	}
	if len(args.Todos) == 0 {
		return nil
	}
	items := make([]eventbus.TodoItem, 0, len(args.Todos))
	for i, t := range args.Todos {
		content := t.Content
		if content == "" {
			content = t.ActiveForm
		}
		items = append(items, eventbus.TodoItem{
			ID:      fmt.Sprintf("todo-%d", i),
			Content: content,
			Status:  t.Status,
		})
	}
	return items
}
