package context

import (
	"context"

	shortterm "Qavor/internal/memory/short_term"
	"Qavor/internal/repository"

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
	logger       *zap.Logger
}

// NewContextManager 创建上下文管理器
func NewContextManager(
	config *ContextConfig,
	messageRepo repository.MessageRepository,
	shortTermMgr shortterm.Manager,
	logger *zap.Logger,
) ContextManager {
	return &contextManager{
		config:       config,
		fetcher:      NewHistoryReader(messageRepo),
		tokenizer:    NewContextTokenizer(config.MaxTokens, config.ReserveTokens),
		builder:      NewContextBuilder(config),
		persister:    NewContextPersister(messageRepo, logger),
		shortTermMgr: shortTermMgr,
		logger:       logger,
	}
}

// FetchContext 提取历史消息
func (m *contextManager) FetchContext(ctx context.Context, query *ContextHistoryQuery) (*ContextWindow, error) {
	messages, err := m.fetcher.LoadHistory(ctx, query)
	if err != nil {
		return nil, err
	}

	return &ContextWindow{
		Messages:    messages,
		TotalTokens: m.tokenizer.CountAllTokens(messages),
	}, nil
}

// CompressContext Token 硬切片裁剪
func (m *contextManager) CompressContext(_ context.Context, window *ContextWindow) (*ContextWindow, error) {
	systemTokens := m.builder.EstimateSystemTokens(window)

	originalCount := len(window.Messages)
	window.Messages = m.tokenizer.TrimMessages(window.Messages, systemTokens)
	window.TrimmedCount = originalCount - len(window.Messages)
	window.TotalTokens = m.tokenizer.CountAllTokens(window.Messages)

	return window, nil
}

// BuildPrompt 组装 Prompt
func (m *contextManager) BuildPrompt(_ context.Context, window *ContextWindow, userMessage *schema.Message) []*schema.Message {
	return m.builder.BuildPrompt(window, userMessage)
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
func (m *contextManager) UpdateShortMemory(ctx context.Context, conversationID uint, message *schema.Message) error {
	if m.shortTermMgr == nil {
		return nil
	}
	return m.shortTermMgr.UpdateMemory(ctx, conversationID, message)
}

// GetShortMemoryContext 获取短期记忆上下文
func (m *contextManager) GetShortMemoryContext(ctx context.Context, conversationID uint, maxTokens int) ([]*schema.Message, error) {
	if m.shortTermMgr == nil {
		return nil, nil
	}
	return m.shortTermMgr.GetContext(ctx, conversationID, maxTokens)
}
