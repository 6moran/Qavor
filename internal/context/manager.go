package context

import (
	"context"
	"fmt"
	"strings"
	"time"

	longterm "Qavor/internal/memory/long_term"
	shortterm "Qavor/internal/memory/short_term"
	"Qavor/internal/model/entity"
	"Qavor/internal/repository"
	"Qavor/internal/trace"
	"Qavor/pkg/utils"

	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

// contextManager 上下文管理器实现
type contextManager struct {
	config       *ContextConfig
	fetcher      *historyReader
	tokenizer    *ContextTokenizer
	builder      *ContextBuilder
	persister    *ContextPersister
	shortTermMgr shortterm.Manager
	longTermMgr  *longterm.Manager
	summarizer   Summarizer
	logger       *zap.Logger
}

// NewContextManager 创建上下文管理器
func NewContextManager(
	config *ContextConfig,
	messageRepo repository.MessageRepository,
	shortTermMgr shortterm.Manager,
	longTermMgr *longterm.Manager,
	summarizer Summarizer,
	logger *zap.Logger,
) ContextManager {
	return &contextManager{
		config:       config,
		fetcher:      NewHistoryReader(messageRepo),
		tokenizer:    NewContextTokenizer(config.MaxTokens, config.ReserveTokens),
		builder:      NewContextBuilder(config),
		persister:    NewContextPersister(messageRepo, logger),
		shortTermMgr: shortTermMgr,
		longTermMgr:  longTermMgr,
		summarizer:   summarizer,
		logger:       logger,
	}
}

// LoadHistory 加载对话历史（含短期记忆摘要），返回裁剪后的消息列表
// 实现 run.ContextProvider 接口
func (m *contextManager) LoadHistory(ctx context.Context, conversationID uint) ([]*schema.Message, error) {
	window, err := m.FetchContext(ctx, &ContextHistoryQuery{
		ConversationID: conversationID,
		Limit:          50,
	})
	if err != nil {
		return nil, err
	}
	window, err = m.CompressContext(ctx, window)
	if err != nil {
		return nil, err
	}
	return window.Messages, nil
}

// FetchContext 提取历史消息与短期记忆
func (m *contextManager) FetchContext(ctx context.Context, query *ContextHistoryQuery) (*ContextWindow, error) {
	start := time.Now()

	spanCtx, span := trace.StartSpan(ctx, entity.SpanKindContext, "FetchContext",
		fmt.Sprintf("conv=%d limit=%d", query.ConversationID, query.Limit))

	messages, err := m.fetcher.LoadHistory(spanCtx, query)
	if err != nil {
		span.End(spanCtx, entity.SpanStatusError, "", err.Error(), 0, 0)
		return nil, err
	}

	window := &ContextWindow{
		Messages:    messages,
		TotalTokens: m.tokenizer.CountAllTokens(messages),
	}

	// 加载短期记忆摘要与状态
	if m.shortTermMgr != nil {
		memory, err := m.shortTermMgr.GetMemory(spanCtx, query.ConversationID)
		if err != nil {
			m.logger.Warn("加载短期记忆失败", zap.Error(err))
		} else if memory != nil {
			if memory.Summary != "" {
				window.ShortTermSummary = memory.Summary
			}
			window.ShortTermState = renderShortTermState(memory.State)
		}
	}

	// 召回长期记忆（用户画像/偏好/决策），注入到 window.MemoryContext
	// 注：目前 JWT 未携带 UserID，使用 defaultUserID（0 → 全局匿名用户）；
	// 待 User 表完成后，此处改为从 ctx 中读取 userID
	if m.longTermMgr != nil {
		memText, ltmErr := m.longTermMgr.RetrieveForPrompt(spanCtx, 0)
		if ltmErr != nil {
			m.logger.Warn("召回长期记忆失败", zap.Error(ltmErr))
		} else if memText != "" {
			window.MemoryContext = memText
		}
	}

	// 初始化 Token 用量统计
	window.TokenUsage = &TokenUsage{
		ConversationID: query.ConversationID,
		InputTokens:    window.TotalTokens,
		ProcessingTime: time.Since(start),
		Timestamp:      time.Now(),
	}

	// 调试日志
	m.logger.Debug("FetchContext 完成",
		zap.Uint("conversation_id", query.ConversationID),
		zap.Int("message_count", len(messages)),
		zap.Int("total_tokens", window.TotalTokens),
		zap.Duration("duration", time.Since(start)),
	)

	span.End(spanCtx, entity.SpanStatusSuccess,
		fmt.Sprintf("%d msgs, %d tokens, summary=%v", len(messages), window.TotalTokens, window.ShortTermSummary != ""),
		"", window.TotalTokens, 0)

	return window, nil
}

// CompressContext Token 硬切片裁剪
func (m *contextManager) CompressContext(ctx context.Context, window *ContextWindow) (*ContextWindow, error) {
	start := time.Now()

	spanCtx, span := trace.StartSpan(ctx, entity.SpanKindContext, "CompressContext",
		fmt.Sprintf("orig=%d msgs, %d tokens", len(window.Messages), window.TotalTokens))

	systemTokens := m.builder.EstimateSystemTokens(window)

	originalCount := len(window.Messages)
	window.Messages = m.tokenizer.TrimMessages(window.Messages, systemTokens)
	window.TrimmedCount = originalCount - len(window.Messages)
	window.TotalTokens = m.tokenizer.CountAllTokens(window.Messages)

	// 更新 Token 用量统计
	if window.TokenUsage != nil {
		window.TokenUsage.TrimmedCount = window.TrimmedCount
		window.TokenUsage.ProcessingTime += time.Since(start)
	}

	// 调试日志
	m.logger.Debug("CompressContext 完成",
		zap.Int("original_count", originalCount),
		zap.Int("trimmed_count", window.TrimmedCount),
		zap.Int("final_count", len(window.Messages)),
		zap.Int("total_tokens", window.TotalTokens),
		zap.Duration("duration", time.Since(start)),
	)

	span.End(spanCtx, entity.SpanStatusSuccess,
		fmt.Sprintf("trimmed=%d, final=%d msgs, %d tokens", window.TrimmedCount, len(window.Messages), window.TotalTokens),
		"", window.TotalTokens, 0)

	return window, nil
}

// CompressWithSummary 使用 LLM 摘要压缩上下文
func (m *contextManager) CompressWithSummary(ctx context.Context, window *ContextWindow) (*ContextWindow, error) {
	start := time.Now()

	spanCtx, span := trace.StartSpan(ctx, entity.SpanKindContext, "CompressWithSummary",
		fmt.Sprintf("msg_count=%d", len(window.Messages)))

	// 检查是否需要摘要（消息数 > 阈值）
	summaryThreshold := 20 // 默认阈值
	summaryGenerated := false
	if len(window.Messages) > summaryThreshold && m.summarizer != nil {
		// 调用 LLM 生成摘要
		summary, err := m.summarizer.Generate(spanCtx, window.Messages)
		if err != nil {
			m.logger.Warn("LLM 摘要生成失败，降级为硬切片", zap.Error(err))
			span.End(spanCtx, entity.SpanStatusError, "", err.Error(), 0, 0)
		} else {
			// 用摘要替代旧消息
			window.Messages = []*schema.Message{
				{Role: schema.System, Content: "[会话摘要] " + summary},
			}
			window.ShortTermSummary = summary
			summaryGenerated = true

			m.logger.Debug("LLM 摘要生成成功",
				zap.Int("original_count", len(window.Messages)),
				zap.Int("summary_length", len(summary)),
				zap.Duration("duration", time.Since(start)),
			)
			span.End(spanCtx, entity.SpanStatusSuccess,
				fmt.Sprintf("summary_generated=%v, length=%d", summaryGenerated, len(summary)),
				"", 0, 0)
		}
	} else {
		span.End(spanCtx, entity.SpanStatusSuccess,
			fmt.Sprintf("skipped (msg_count=%d <= threshold=%d)", len(window.Messages), summaryThreshold),
			"", 0, 0)
	}

	// 硬切片
	return m.CompressContext(spanCtx, window)
}

// BuildPrompt 组装 Prompt
func (m *contextManager) BuildPrompt(ctx context.Context, window *ContextWindow, userMessage *schema.Message) []*schema.Message {
	start := time.Now()

	spanCtx, span := trace.StartSpan(ctx, entity.SpanKindContext, "BuildPrompt",
		fmt.Sprintf("input=%d msgs, %d tokens", len(window.Messages), window.TotalTokens))

	result := m.builder.BuildPrompt(window, userMessage)

	// 调试日志
	m.logger.Debug("BuildPrompt 完成",
		zap.Int("input_count", len(window.Messages)),
		zap.Int("output_count", len(result)),
		zap.Int("total_tokens", window.TotalTokens),
		zap.Duration("duration", time.Since(start)),
	)

	span.End(spanCtx, entity.SpanStatusSuccess,
		fmt.Sprintf("output=%d msgs", len(result)),
		"", window.TotalTokens, 0)

	return result
}

// PersistUserMessage 同步保存用户消息
func (m *contextManager) PersistUserMessage(_ context.Context, conversationID uint, userMsg *schema.Message) (uint, error) {
	return m.persister.PersistUserMessage(conversationID, userMsg)
}

// PersistAssistantMessage 保存助手回复
func (m *contextManager) PersistAssistantMessage(_ context.Context, conversationID uint, assistantMsg *schema.Message) error {
	return m.persister.PersistAssistantMessage(conversationID, assistantMsg)
}

// CountTokens 计算消息列表的 Token 数量
func (m *contextManager) CountTokens(messages []*schema.Message) int {
	return m.tokenizer.CountAllTokens(messages)
}

// UpdateShortMemory 更新短期记忆（AI回复完成后调用）
// modelID 指定摘要/状态抽取使用的模型，0 时降级为规则式
func (m *contextManager) UpdateShortMemory(ctx context.Context, conversationID uint, message *schema.Message, modelID uint) error {
	if m.shortTermMgr == nil {
		return nil
	}

	spanCtx, span := trace.StartSpan(ctx, entity.SpanKindContext, "UpdateShortMemory",
		fmt.Sprintf("conv=%d role=%s", conversationID, message.Role))

	err := m.shortTermMgr.UpdateMemory(spanCtx, conversationID, message, modelID)

	if err != nil {
		span.End(spanCtx, entity.SpanStatusError, "", err.Error(), 0, 0)
	} else {
		span.End(spanCtx, entity.SpanStatusSuccess, "", "", 0, 0)
	}
	return err
}

// GetShortMemoryContext 获取短期记忆上下文
func (m *contextManager) GetShortMemoryContext(ctx context.Context, conversationID uint, maxTokens int) ([]*schema.Message, error) {
	if m.shortTermMgr == nil {
		return nil, nil
	}
	return m.shortTermMgr.GetContext(ctx, conversationID, maxTokens)
}

// renderShortTermState 将会话状态渲染为可注入 Prompt 的文本
func renderShortTermState(state *shortterm.SessionState) string {
	if state == nil {
		return ""
	}
	var parts []string
	if state.Topic != "" {
		parts = append(parts, "主题: "+state.Topic)
	}
	if state.UserIntent != "" {
		parts = append(parts, "用户意图: "+state.UserIntent)
	}
	if len(state.KeyEntities) > 0 {
		parts = append(parts, "关键实体: "+strings.Join(state.KeyEntities, ", "))
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "\n")
}

// GetAgentState 获取 Agent 状态面板数据
// 聚合 token_usage（上下文使用）、todos（待办）、files（文件）、subagent_runs（子Agent运行）
func (m *contextManager) GetAgentState(ctx context.Context, conversationID uint) (*AgentState, error) {
	state := &AgentState{
		Todos:        []AgentTodo{},
		Files:        map[string]AgentFile{},
		SubagentRuns: []AgentSubagentRun{},
		Artifacts:    []string{},
	}

	// —— 计算 token_usage ——
	// 1. FetchContext: 加载历史消息 + 短期记忆（裁剪前）
	window, err := m.FetchContext(ctx, &ContextHistoryQuery{
		ConversationID: conversationID,
		Limit:          50,
	})
	if err != nil {
		m.logger.Warn("GetAgentState: FetchContext 失败", zap.Uint("conv", conversationID), zap.Error(err))
		return state, nil // 降级返回空状态，不阻断面板
	}

	// 记录裁剪前的消息数与 Token 数
	stateMessagesTokensBeforeCall := window.TotalTokens
	stateMessageCountBeforeCall := len(window.Messages)

	// 2. CompressContext: 硬切片裁剪
	window, err = m.CompressContext(ctx, window)
	if err != nil {
		m.logger.Warn("GetAgentState: CompressContext 失败", zap.Error(err))
	}

	// 3. 按角色拆分消息 Token
	contentTokens, contentCount := 0, 0
	toolTokens, toolCount := 0, 0
	for _, msg := range window.Messages {
		tokens := m.tokenizer.EstimateTokens(msg)
		if msg.Role == schema.Tool {
			toolTokens += tokens
			toolCount++
		} else {
			contentTokens += tokens
			contentCount++
		}
	}

	// 4. 摘要 Token
	summaryActive := window.ShortTermSummary != ""
	summaryTokens := 0
	if summaryActive {
		summaryTokens = utils.CountTokens(window.ShortTermSummary)
	}

	// 5. System Prompt Token（含摘要/状态注入）
	systemTokens := m.builder.EstimateSystemTokens(window)

	llmMessagesTokens := contentTokens + toolTokens + summaryTokens
	llmInputTokens := systemTokens + llmMessagesTokens // tools_tokens 暂未追踪

	// 摘要触发阈值（与 short_term DefaultSummaryConfig.TokenThreshold 一致）
	summaryTriggerTokens := 8000
	contextWindow := m.config.MaxTokens
	remainingContextTokens := contextWindow - llmInputTokens
	if remainingContextTokens < 0 {
		remainingContextTokens = 0
	}

	state.TokenUsage = &AgentTokenUsage{
		SummaryActive:                 summaryActive,
		SummaryMessageTokens:          summaryTokens,
		LlmMessagesTokens:             llmMessagesTokens,
		LlmContentMessageTokens:       contentTokens,
		LlmToolMessageTokens:          toolTokens,
		StateMessagesTokensBeforeCall: stateMessagesTokensBeforeCall,
		StateMessageCountBeforeCall:   stateMessageCountBeforeCall,
		LlmMessageCount:               len(window.Messages),
		LlmContentMessageCount:        contentCount,
		LlmToolMessageCount:           toolCount,
		SystemTokens:                  systemTokens,
		ToolsTokens:                   0,
		ToolCount:                     0,
		LlmInputTokens:                llmInputTokens,
		SummaryTriggerTokens:          summaryTriggerTokens,
		ContextWindow:                 contextWindow,
		RemainingContextTokens:        remainingContextTokens,
	}

	// 填充短期记忆监控指标
	if m.shortTermMgr != nil {
		state.MemoryMetrics = m.shortTermMgr.GetMetrics()
	}

	return state, nil
}
