package shortterm

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Store 短期记忆存储接口
type Store interface {
	Save(ctx context.Context, memory *SessionMemory) error
	Load(ctx context.Context, conversationID uint) (*SessionMemory, error)
	Delete(ctx context.Context, conversationID uint) error
}

// ManagerImpl 短期记忆管理器实现
type ManagerImpl struct {
	store         Store
	bufferManager *MessageBufferManager
	stateManager  *SessionStateManager
	summaryGen    *SummaryGenerator
	logger        *zap.Logger
	metrics       *MemoryMetrics
}

// MemoryMetrics 短期记忆监控指标（并发安全，使用 atomic 操作）
type MemoryMetrics struct {
	UpdateCount   atomic.Int64 // 更新次数
	SummaryCount  atomic.Int64 // 摘要生成次数
	TotalDuration atomic.Int64 // 总耗时（nanoseconds）
	BufferSize    atomic.Int32 // 当前缓冲区大小
	SummaryLength atomic.Int32 // 摘要长度
}

// AvgUpdateTime 计算平均更新耗时
func (m *MemoryMetrics) AvgUpdateTime() time.Duration {
	count := m.UpdateCount.Load()
	if count == 0 {
		return 0
	}
	return time.Duration(m.TotalDuration.Load() / count)
}

// Snapshot 返回 metrics 的快照（用于日志/展示）
type MemoryMetricsSnapshot struct {
	UpdateCount   int64
	SummaryCount  int64
	AvgUpdateTime time.Duration
	TotalDuration time.Duration
	BufferSize    int32
	SummaryLength int32
}

// Snapshot 获取当前指标的快照
func (m *MemoryMetrics) Snapshot() *MemoryMetricsSnapshot {
	return &MemoryMetricsSnapshot{
		UpdateCount:   m.UpdateCount.Load(),
		SummaryCount:  m.SummaryCount.Load(),
		AvgUpdateTime: m.AvgUpdateTime(),
		TotalDuration: time.Duration(m.TotalDuration.Load()),
		BufferSize:    m.BufferSize.Load(),
		SummaryLength: m.SummaryLength.Load(),
	}
}

// NewManager 创建短期记忆管理器
func NewManager(
	store Store,
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
		metrics:       &MemoryMetrics{},
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

	// 防御性修复（根因）：Redis 中可能残留旧 schema / 损坏记录，其 Buffer 或 State
	// 为 nil。若直接传入 AddMessage / UpdateState / GetMessagesByTokens 会触发 nil
	// 解引用 panic（worker.go:385 → UpdateMemory → AddMessage）。此处统一保证
	// SessionMemory 的不变式：Buffer 与 State 永远非空，读到残缺数据也自愈。
	if memory.Buffer == nil {
		memory.Buffer = &MessageBuffer{
			Messages:    make([]BufferMessage, 0),
			MaxSize:     20,
			TotalTokens: 0,
		}
	}
	if memory.State == nil {
		memory.State = NewSessionState()
	}

	return memory, nil
}

// UpdateMemory 更新短期记忆（AI回复完成后异步调用）
// modelID 指定摘要/状态抽取使用的模型，0 时降级为规则式
func (m *ManagerImpl) UpdateMemory(ctx context.Context, conversationID uint, message *schema.Message, modelID uint) error {
	start := time.Now()

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
	// 有模型时跳过规则式，由阈值触发的 LLM 抽取统一更新；无模型时降级为规则式
	if !m.stateManager.HasModel(modelID) {
		m.stateManager.UpdateState(memory.State, message)
	}

	// 5. 检查是否需要生成摘要（缓冲区达到阈值）
	if m.summaryGen.ShouldGenerate(memory.Buffer) {
		// 滑动窗口：取前半部分消息，保留后半部分
		oldSummary := memory.Summary
		summarizedMessages := m.bufferManager.SlideWindow(memory.Buffer)

		// 构建临时 buffer 传入摘要生成器
		tempBuffer := &MessageBuffer{
			Messages:    summarizedMessages,
			MaxSize:     memory.Buffer.MaxSize,
			TotalTokens: 0,
		}
		for _, msg := range summarizedMessages {
			tempBuffer.TotalTokens += msg.Tokens
		}

		// 先保存（滑动窗口后的状态立即生效）
		memory.UpdatedAt = time.Now()
		if err := m.store.Save(ctx, memory); err != nil {
			m.logger.Error("保存短期记忆失败", zap.Error(err))
		}

		// 异步生成摘要 + 状态抽取，两者完成后一次性写回（消除 RMW 竞态）
		recentMsgs := append([]BufferMessage(nil), memory.Buffer.Messages...)
		m.runAsyncSummaryAndState(ctx, conversationID, tempBuffer, oldSummary, recentMsgs, modelID)

		// 更新监控指标
		m.metrics.SummaryCount.Add(1)
	} else {
		// 6. 保存到 Redis
		memory.UpdatedAt = time.Now()
		if err := m.store.Save(ctx, memory); err != nil {
			return err
		}
	}

	// 更新监控指标
	m.metrics.UpdateCount.Add(1)
	m.metrics.TotalDuration.Add(int64(time.Since(start)))
	m.metrics.BufferSize.Store(int32(len(memory.Buffer.Messages)))
	if memory.Summary != "" {
		m.metrics.SummaryLength.Store(int32(len(memory.Summary)))
	}

	// 调试日志
	m.logger.Debug("UpdateMemory 完成",
		zap.Uint("conversation_id", conversationID),
		zap.Int("buffer_size", len(memory.Buffer.Messages)),
		zap.Int("summary_length", len(memory.Summary)),
		zap.Duration("duration", time.Since(start)),
	)

	return nil
}

// runAsyncSummaryAndState 并行触发摘要生成和状态抽取，两者都完成后一次性写回 Redis
// 消除两个独立回调各自 Read-Modify-Write 导致的丢失更新竞态
func (m *ManagerImpl) runAsyncSummaryAndState(
	ctx context.Context,
	conversationID uint,
	buffer *MessageBuffer,
	oldSummary string,
	recentMsgs []BufferMessage,
	modelID uint,
) {
	var (
		newSummary string
		newState   *SessionState
		wg         sync.WaitGroup
		needState  = m.stateManager.HasModel(modelID)
	)

	wg.Add(1)
	m.summaryGen.GenerateSummaryAsync(ctx, buffer, oldSummary, modelID, func(summary string) {
		newSummary = summary
		wg.Done()
	})

	if needState {
		wg.Add(1)
		m.stateManager.ExtractStateAsync(ctx, recentMsgs, modelID, func(state *SessionState) {
			newState = state
			wg.Done()
		})
	}

	// 独立 goroutine 等待两者完成，一次性写回
	go func() {
		wg.Wait()
		asyncCtx := context.WithoutCancel(ctx)
		latest, err := m.GetMemory(asyncCtx, conversationID)
		if err != nil || latest == nil {
			m.logger.Error("异步摘要/状态回调：读取最新记忆失败", zap.Error(err))
			return
		}
		if newSummary != "" {
			latest.Summary = newSummary
		}
		if newState != nil {
			latest.State = newState
		}
		latest.UpdatedAt = time.Now()
		if err := m.store.Save(asyncCtx, latest); err != nil {
			m.logger.Error("保存摘要/状态失败", zap.Error(err))
		}
	}()
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

// GetMetrics 获取监控指标快照
func (m *ManagerImpl) GetMetrics() *MemoryMetricsSnapshot {
	return m.metrics.Snapshot()
}
