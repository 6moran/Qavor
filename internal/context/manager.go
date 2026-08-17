package context

import (
	"context"
	"fmt"
	"strings"
	"time"

	"Qavor/internal/llm"
	longterm "Qavor/internal/memory/long_term"
	shortterm "Qavor/internal/memory/short_term"
	"Qavor/internal/model/entity"
	"Qavor/internal/repository"
	"Qavor/internal/trace"
	"Qavor/pkg/utils"

	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

// ModelResolver 用于根据模型 ID 解析 LLM 客户端
type ModelResolver interface {
	// CreateLLMClient 创建 LLM 客户端
	CreateLLMClient(ctx context.Context, modelID uint) (llm.Client, error)
	// GetContextWindow 获取模型的上下文窗口大小，0 表示使用默认值
	GetContextWindow(modelID uint) int
}

// llmClientAdapter 将 llm.Client 适配为 LLMClient 接口
type llmClientAdapter struct {
	client llm.Client
}

func (a *llmClientAdapter) Generate(ctx context.Context, input []*schema.Message) (*schema.Message, error) {
	return a.client.Generate(ctx, input)
}

// contextManager 上下文管理器实现
type contextManager struct {
	config        *ContextConfig
	fetcher       *historyReader
	tokenizer     *ContextTokenizer
	builder       *ContextBuilder
	persister     *ContextPersister
	shortTermMgr  shortterm.Manager
	longTermMgr   *longterm.Manager
	summarizer    Summarizer    // 默认摘要器（可为 nil）
	modelResolver ModelResolver // 用于动态创建摘要器
	logger        *zap.Logger
	tracer        *trace.Tracer
}

// NewContextManager 创建上下文管理器
// tracer 为链路追踪实例（可为 nil，nil 时手动 span 为 no-op）
func NewContextManager(
	config *ContextConfig,
	messageRepo repository.MessageRepository,
	shortTermMgr shortterm.Manager,
	longTermMgr *longterm.Manager,
	summarizer Summarizer,
	modelResolver ModelResolver,
	logger *zap.Logger,
	tracer *trace.Tracer,
) ContextManager {
	return &contextManager{
		config:        config,
		fetcher:       NewHistoryReader(messageRepo),
		tokenizer:     NewContextTokenizer(config.MaxTokens, config.ReserveTokens),
		builder:       NewContextBuilder(config),
		persister:     NewContextPersister(messageRepo, logger),
		shortTermMgr:  shortTermMgr,
		longTermMgr:   longTermMgr,
		summarizer:    summarizer,
		modelResolver: modelResolver,
		logger:        logger,
		tracer:        tracer,
	}
}

// LoadHistory 加载对话历史（含短期记忆摘要），返回裁剪后的消息列表
// maxTokens 为该请求对应的模型上下文窗口大小，0 时使用默认值
// modelID 为当前使用的模型 ID，用于摘要生成，0 时不生成摘要
// 实现 run.ContextProvider 接口
func (m *contextManager) LoadHistory(ctx context.Context, conversationID uint, maxTokens int, modelID uint) ([]*schema.Message, error) {
	// 如果指定了 maxTokens，使用该值裁剪；否则使用默认值
	var tokenizer *ContextTokenizer
	if maxTokens > 0 {
		tokenizer = NewContextTokenizer(maxTokens, m.config.ReserveTokens)
	} else {
		tokenizer = m.tokenizer
	}

	window, err := m.FetchContext(ctx, &ContextHistoryQuery{
		ConversationID: conversationID,
		Limit:          50,
	})
	if err != nil {
		return nil, err
	}

	// 计算总 token 数
	totalTokens := tokenizer.CountAllTokens(window.Messages)

	// 判断是否需要摘要：总 token 超过上下文窗口的 80%
	summaryThreshold := int(float64(tokenizer.maxTokens) * 0.8)
	needSummary := totalTokens > summaryThreshold && modelID > 0 && m.modelResolver != nil

	if needSummary {
		// 根据 modelID 动态创建 LLM 客户端和摘要器
		llmClient, err := m.modelResolver.CreateLLMClient(ctx, modelID)
		if err == nil {
			// 使用适配器将 llm.Client 转换为 LLMClient 接口
			summarySummarizer := NewLLMSummarizer(m.logger, &llmClientAdapter{client: llmClient})
			window, err = m.CompressWithSummary(ctx, window, summarySummarizer)
			if err != nil {
				m.logger.Warn("LLM 摘要压缩失败，降级为硬切片", zap.Error(err))
			}
		} else {
			m.logger.Warn("创建 LLM 客户端失败，跳过摘要", zap.Error(err))
		}
	} else {
		// token 未超限或无可用模型，直接硬切片
		systemTokens := m.builder.EstimateSystemTokens(window)
		window.Messages = tokenizer.TrimMessages(window.Messages, systemTokens)
		window.TrimmedCount = len(window.Messages)
		window.TotalTokens = tokenizer.CountAllTokens(window.Messages)
	}

	return window.Messages, nil
}

// FetchContext 提取历史消息与短期记忆
func (m *contextManager) FetchContext(ctx context.Context, query *ContextHistoryQuery) (*ContextWindow, error) {
	start := time.Now()

	spanCtx, span := trace.StartSpan(m.tracer, ctx, entity.SpanKindContext, "FetchContext",
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
	// SkipLongTermMemory=true 时跳过召回，避免 UI 状态查询等高频路径重复 DB 查询
	if m.longTermMgr != nil && !query.SkipLongTermMemory {
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

	spanCtx, span := trace.StartSpan(m.tracer, ctx, entity.SpanKindContext, "CompressContext",
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
// summarizer 为当前请求使用的摘要器
func (m *contextManager) CompressWithSummary(ctx context.Context, window *ContextWindow, summarizer Summarizer) (*ContextWindow, error) {
	start := time.Now()

	spanCtx, span := trace.StartSpan(m.tracer, ctx, entity.SpanKindContext, "CompressWithSummary",
		fmt.Sprintf("msg_count=%d", len(window.Messages)))

	// 使用传入的 summarizer 生成摘要
	summaryGenerated := false
	if summarizer != nil {
		// 调用 LLM 生成摘要
		summary, err := summarizer.Generate(spanCtx, window.Messages)
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
			fmt.Sprintf("skipped (summarizer=nil)"),
			"", 0, 0)
	}

	// 硬切片
	return m.CompressContext(spanCtx, window)
}

// BuildPrompt 组装 Prompt
func (m *contextManager) BuildPrompt(ctx context.Context, window *ContextWindow, userMessage *schema.Message) []*schema.Message {
	start := time.Now()

	spanCtx, span := trace.StartSpan(m.tracer, ctx, entity.SpanKindContext, "BuildPrompt",
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

	spanCtx, span := trace.StartSpan(m.tracer, ctx, entity.SpanKindContext, "UpdateShortMemory",
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
// modelID 用于获取模型的上下文窗口大小，0 时使用默认值
func (m *contextManager) GetAgentState(ctx context.Context, conversationID uint, modelID uint) (*AgentState, error) {
	// 根据 modelID 获取上下文窗口大小
	contextWindow := m.config.MaxTokens // 默认值
	if modelID > 0 && m.modelResolver != nil {
		if cw := m.modelResolver.GetContextWindow(modelID); cw > 0 {
			contextWindow = cw
		}
	}
	state := &AgentState{
		Todos:        []AgentTodo{},
		Files:        map[string]AgentFile{},
		SubagentRuns: []AgentSubagentRun{},
		Artifacts:    []string{},
	}

	// —— 计算 token_usage ——
	// 1. FetchContext: 加载历史消息 + 短期记忆（裁剪前）
	// 跳过长期记忆召回：UI 状态查询每 5 秒轮询一次，长期记忆变化频率极低，
	// 召回会带来重复 DB 查询开销；系统 Prompt token 估算会少算长期记忆部分（约几百 tokens），可接受
	window, err := m.FetchContext(ctx, &ContextHistoryQuery{
		ConversationID:     conversationID,
		Limit:              50,
		SkipLongTermMemory: true,
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

	// 摘要触发阈值：在上下文窗口 80% 时触发，确保摘要先于裁剪生成
	summaryTriggerTokens := int(float64(contextWindow) * 0.8)
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
