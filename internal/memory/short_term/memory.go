package shortterm

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

// Store 短期记忆存储接口
type Store interface {
	Save(ctx context.Context, memory *SessionContext) error
	Load(ctx context.Context, conversationID uint) (*SessionContext, error)
	Delete(ctx context.Context, conversationID uint) error
}

// ManagerImpl 短期记忆管理器实现
type ManagerImpl struct {
	store      Store
	recentMsgs *RecentMessagesManager
	taskState  *TaskStateManager
	summaryGen *SummaryGenerator
	logger     *zap.Logger
	metrics    *MemoryMetrics
}

// MemoryMetrics 短期记忆监控指标（并发安全，使用 atomic 操作）
type MemoryMetrics struct {
	UpdateCount   atomic.Int64 // 更新次数
	SummaryCount  atomic.Int64 // 摘要生成次数
	TotalDuration atomic.Int64 // 总耗时（nanoseconds）
	MessageCount  atomic.Int32 // 当前消息数
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

// Snapshot 返回 metrics 的快照
type MemoryMetricsSnapshot struct {
	UpdateCount   int64
	SummaryCount  int64
	AvgUpdateTime time.Duration
	TotalDuration time.Duration
	MessageCount  int32
	SummaryLength int32
}

// Snapshot 获取当前指标的快照
func (m *MemoryMetrics) Snapshot() *MemoryMetricsSnapshot {
	return &MemoryMetricsSnapshot{
		UpdateCount:   m.UpdateCount.Load(),
		SummaryCount:  m.SummaryCount.Load(),
		AvgUpdateTime: m.AvgUpdateTime(),
		TotalDuration: time.Duration(m.TotalDuration.Load()),
		MessageCount:  m.MessageCount.Load(),
		SummaryLength: m.SummaryLength.Load(),
	}
}

// NewManager 创建短期记忆管理器
func NewManager(
	store Store,
	recentMsgs *RecentMessagesManager,
	taskState *TaskStateManager,
	summaryGen *SummaryGenerator,
	logger *zap.Logger,
) *ManagerImpl {
	return &ManagerImpl{
		store:      store,
		recentMsgs: recentMsgs,
		taskState:  taskState,
		summaryGen: summaryGen,
		logger:     logger,
		metrics:    &MemoryMetrics{},
	}
}

// GetMemory 获取会话的短期上下文
func (m *ManagerImpl) GetMemory(ctx context.Context, conversationID uint) (*SessionContext, error) {
	memory, err := m.store.Load(ctx, conversationID)
	if err != nil {
		return nil, err
	}

	if memory == nil {
		memory = &SessionContext{
			ConversationID: conversationID,
			Messages:       make([]Message, 0),
			Summary:        "",
			TaskState:      NewTaskState(),
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}
	}

	return memory, nil
}

// UpdateMemory 更新短期上下文
func (m *ManagerImpl) UpdateMemory(ctx context.Context, conversationID uint, message *schema.Message, modelID uint) error {
	start := time.Now()

	// 1. 获取现有上下文
	memory, err := m.GetMemory(ctx, conversationID)
	if err != nil {
		return err
	}

	// 2. 添加到最近消息
	m.recentMsgs.AddMessage(&memory.Messages, message)

	// 3. 更新任务状态（无模型时用规则式）
	if !m.taskState.HasModel(modelID) {
		m.taskState.UpdateState(memory.TaskState, message)
	}

	// 4. 检查是否需要生成摘要
	if m.summaryGen.ShouldGenerate(memory.Messages) {
		oldSummary := memory.Summary
		summarizedMessages := m.recentMsgs.SlideWindow(&memory.Messages)

		// 先保存（滑动窗口后的状态立即生效）
		memory.UpdatedAt = time.Now()
		if err := m.store.Save(ctx, memory); err != nil {
			m.logger.Error("保存短期上下文失败", zap.Error(err))
		}

		// 异步生成摘要 + 任务状态抽取
		recentCopy := append([]Message(nil), memory.Messages...)
		m.runAsyncSummaryAndState(ctx, conversationID, summarizedMessages, oldSummary, recentCopy, modelID)

		m.metrics.SummaryCount.Add(1)
	} else {
		// 5. 直接保存
		memory.UpdatedAt = time.Now()
		if err := m.store.Save(ctx, memory); err != nil {
			return err
		}
	}

	// 更新监控指标
	m.metrics.UpdateCount.Add(1)
	m.metrics.TotalDuration.Add(int64(time.Since(start)))
	m.metrics.MessageCount.Store(int32(len(memory.Messages)))
	if memory.Summary != "" {
		m.metrics.SummaryLength.Store(int32(len(memory.Summary)))
	}

	m.logger.Debug("UpdateMemory 完成",
		zap.Uint("conversation_id", conversationID),
		zap.Int("message_count", len(memory.Messages)),
		zap.Int("summary_length", len(memory.Summary)),
		zap.Duration("duration", time.Since(start)),
	)

	return nil
}

// runAsyncSummaryAndState 并行触发摘要生成和任务状态抽取，两者都完成后一次性写回
func (m *ManagerImpl) runAsyncSummaryAndState(
	ctx context.Context,
	conversationID uint,
	summarizedMessages []Message,
	oldSummary string,
	recentMessages []Message,
	modelID uint,
) {
	var (
		newSummary string
		newState   *TaskState
		wg         sync.WaitGroup
		needState  = m.taskState.HasModel(modelID)
	)

	wg.Add(1)
	m.summaryGen.GenerateSummaryAsync(ctx, summarizedMessages, oldSummary, modelID, func(summary string) {
		newSummary = summary
		wg.Done()
	})

	if needState {
		wg.Add(1)
		m.taskState.ExtractTaskStateAsync(ctx, recentMessages, modelID, func(state *TaskState) {
			newState = state
			wg.Done()
		})
	}

	go func() {
		wg.Wait()
		asyncCtx := context.WithoutCancel(ctx)
		latest, err := m.GetMemory(asyncCtx, conversationID)
		if err != nil || latest == nil {
			m.logger.Error("异步摘要/状态回调：读取最新上下文失败", zap.Error(err))
			return
		}
		if newSummary != "" {
			latest.Summary = newSummary
		}
		if newState != nil {
			latest.TaskState = newState
		}
		latest.UpdatedAt = time.Now()
		if err := m.store.Save(asyncCtx, latest); err != nil {
			m.logger.Error("保存摘要/任务状态失败", zap.Error(err))
		}
	}()
}

// GetContext 获取用于 LLM 的上下文
func (m *ManagerImpl) GetContext(ctx context.Context, conversationID uint, maxTokens int) ([]*schema.Message, error) {
	memory, err := m.GetMemory(ctx, conversationID)
	if err != nil {
		return nil, err
	}

	var messages []*schema.Message

	// 添加摘要（如果有）
	if memory.Summary != "" {
		messages = append(messages, &schema.Message{
			Role:    schema.System,
			Content: "[任务恢复摘要] " + memory.Summary,
		})
	}

	// 获取最近消息
	recentMessages := m.recentMsgs.GetMessagesByTokens(memory.Messages, maxTokens)
	for _, msg := range recentMessages {
		messages = append(messages, &schema.Message{
			Role:    schema.User, // 统一用 User，保持消息流的连贯性
			Content: msg.Content,
		})
	}

	return messages, nil
}

// ClearMemory 清除会话的短期上下文
func (m *ManagerImpl) ClearMemory(ctx context.Context, conversationID uint) error {
	return m.store.Delete(ctx, conversationID)
}

// GetMetrics 获取监控指标快照
func (m *ManagerImpl) GetMetrics() *MemoryMetricsSnapshot {
	return m.metrics.Snapshot()
}

// GetTaskState 获取任务状态（供外部读取）
func (m *ManagerImpl) GetTaskState(ctx context.Context, conversationID uint) (*TaskState, error) {
	memory, err := m.GetMemory(ctx, conversationID)
	if err != nil {
		return nil, err
	}
	return memory.TaskState, nil
}
