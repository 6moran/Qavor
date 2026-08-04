package shortterm

import (
	"context"
	"fmt"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ManagerImpl 短期记忆管理器实现
type ManagerImpl struct {
	store         *RedisStore
	bufferManager *MessageBufferManager
	stateManager  *SessionStateManager
	summaryGen    *SummaryGenerator
	logger        *zap.Logger
}

// NewManager 创建短期记忆管理器
func NewManager(
	store *RedisStore,
	bufferManager *MessageBufferManager,
	stateManager *SessionStateManager,
	summaryGen *SummaryGenerator,
	logger *zap.Logger,
) *ManagerImpl {
	return &ManagerImpl{
		store:         store,
		bufferManager: bufferManager,
		stateManager:  stateManager,
		summaryGen:    summaryGen,
		logger:        logger,
	}
}

// GetMemory 获取会话的短期记忆
func (m *ManagerImpl) GetMemory(ctx context.Context, conversationID uint) (*SessionMemory, error) {
	// 从 Redis 加载
	memory, err := m.store.Load(ctx, conversationID)
	if err != nil {
		return nil, err
	}

	// 如果不存在，创建新的
	if memory == nil {
		memory = &SessionMemory{
			ConversationID: conversationID,
			Buffer: &MessageBuffer{
				Messages:    make([]BufferMessage, 0),
				MaxSize:     20,
				TotalTokens: 0,
			},
			Summary:   "",
			State:     NewSessionState(),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
	}

	return memory, nil
}

// UpdateMemory 更新短期记忆（AI回复完成后异步调用）
func (m *ManagerImpl) UpdateMemory(ctx context.Context, conversationID uint, message *schema.Message) error {
	// 1. 获取现有记忆
	memory, err := m.GetMemory(ctx, conversationID)
	if err != nil {
		return err
	}

	// 2. 生成消息ID
	messageID := fmt.Sprintf("msg_%s", uuid.New().String()[:8])

	// 3. 添加到缓冲区
	m.bufferManager.AddMessage(memory.Buffer, message, messageID)

	// 4. 更新会话状态
	m.stateManager.UpdateState(memory.State, message)

	// 5. 检查是否需要生成摘要
	if m.summaryGen.ShouldGenerate(memory.Buffer) {
		// 异步生成摘要
		m.summaryGen.GenerateSummaryAsync(ctx, memory.Buffer, func(summary string) {
			memory.Summary = summary
			memory.UpdatedAt = time.Now()
			if err := m.store.Save(ctx, memory); err != nil {
				m.logger.Error("保存摘要失败", zap.Error(err))
			}
		})
	}

	// 6. 保存到 Redis
	memory.UpdatedAt = time.Now()
	return m.store.Save(ctx, memory)
}

// GetContext 获取用于 LLM 的上下文（包含摘要+最近消息）
func (m *ManagerImpl) GetContext(ctx context.Context, conversationID uint, maxTokens int) ([]*schema.Message, error) {
	// 1. 获取记忆
	memory, err := m.GetMemory(ctx, conversationID)
	if err != nil {
		return nil, err
	}

	var messages []*schema.Message

	// 2. 添加摘要（如果有）
	if memory.Summary != "" {
		messages = append(messages, &schema.Message{
			Role:    schema.System,
			Content: "[会话摘要] " + memory.Summary,
		})
	}

	// 3. 获取最近消息（按 Token 数）
	recentMessages := m.bufferManager.GetMessagesByTokens(memory.Buffer, maxTokens)

	// 4. 转换为 schema.Message
	for _, msg := range recentMessages {
		messages = append(messages, &schema.Message{
			Role:    schema.User,
			Content: msg.Content,
		})
	}

	return messages, nil
}

// ClearMemory 清除会话的短期记忆
func (m *ManagerImpl) ClearMemory(ctx context.Context, conversationID uint) error {
	return m.store.Delete(ctx, conversationID)
}

// RefreshTTL 刷新会话的 Redis TTL
func (m *ManagerImpl) RefreshTTL(ctx context.Context, conversationID uint) error {
	return m.store.RefreshTTL(ctx, conversationID)
}
