package longterm

import (
	"context"
	"fmt"
	"strings"
	"time"

	shortterm "Qavor/internal/memory/short_term"
	memtypes "Qavor/internal/memory/types"
	"Qavor/internal/model/entity"
	"Qavor/internal/repository"

	"go.uber.org/zap"
)

// Manager 长期记忆管理器：协调抽取→入库→召回→渲染为 Prompt 文本
//
// 当前策略（P0 阶段，单用户/规模小）：全量拉取活跃记忆，按重要性排序，截断到 maxTokens
// P2 升级：pgvector 语义检索 top-K + 偏好类全量保底
type Manager struct {
	logger         *zap.Logger
	repo           repository.LongTermMemoryRepository
	extractor      *Extractor
	maxItems       int    // 召回最大条目
	maxTokens      int    // 注入 Prompt 最大 Token 数
	defaultUserID  uint   // JWT 尚未带 UserID 时使用的默认 user（0 表示全局匿名）
	onMemoryChange func() // 记忆变更回调（如清空 Agent 指令缓存，使全局记忆及时生效）
}

// SetMemoryChangeCallback 设置记忆变更回调（记忆写入/更新后触发）
func (m *Manager) SetMemoryChangeCallback(callback func()) {
	m.onMemoryChange = callback
}

// Config Manager 配置
type Config struct {
	MaxItems      int
	MaxTokens     int
	DefaultUserID uint
}

// NewManager 创建长期记忆管理器
func NewManager(
	logger *zap.Logger,
	repo repository.LongTermMemoryRepository,
	modelResolver shortterm.ModelResolver,
	cfg Config,
) *Manager {
	if cfg.MaxItems <= 0 {
		cfg.MaxItems = 200
	}
	if cfg.MaxTokens <= 0 {
		cfg.MaxTokens = 1500
	}
	return &Manager{
		logger:        logger,
		repo:          repo,
		extractor:     NewExtractor(logger, modelResolver),
		maxItems:      cfg.MaxItems,
		maxTokens:     cfg.MaxTokens,
		defaultUserID: cfg.DefaultUserID,
	}
}

// ResolveUserID 解析有效 userID（JWT 未携带则降级为 defaultUserID）
func (m *Manager) ResolveUserID(raw uint) uint {
	if raw > 0 {
		return raw
	}
	return m.defaultUserID
}

// ---------- 抽取 + 入库 ----------

// ExtractAfterTurn AI 回复完成后异步抽取记忆并入库
// userID: 当前用户（0 时走默认 user）
// conversationID / runID: 用于可追溯记录
// turn: 最近一轮 user + assistant 消息对
// modelID: 抽取使用的模型（0 则仅规则式）
func (m *Manager) ExtractAfterTurn(
	ctx context.Context,
	userID uint,
	conversationID uint,
	runID string,
	turn []TurnMessage,
	modelID uint,
) {
	if m.extractor == nil {
		return
	}
	uid := m.ResolveUserID(userID)

	m.extractor.ExtractAndStore(ctx, turn, modelID, func(items []ExtractedMemory) {
		if len(items) == 0 {
			return
		}
		entities := make([]*entity.LongTermMemory, 0, len(items))
		for _, it := range items {
			entities = append(entities, &entity.LongTermMemory{
				UserID:               uid,
				Category:             it.Category,
				Content:              it.Content,
				Importance:           it.Importance,
				Confidence:           it.Confidence,
				SourceConversationID: conversationID,
				SourceRunID:          runID,
			})
		}
		storeCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		if err := m.storeWithUpdate(storeCtx, entities); err != nil {
			m.logger.Warn("长期记忆入库失败", zap.Uint("user_id", uid), zap.Error(err))
			return
		}
		m.logger.Debug("长期记忆入库成功",
			zap.Uint("user_id", uid),
			zap.Int("count", len(entities)),
			zap.Uint("conv", conversationID),
		)
	})
}

// storeWithUpdate 智能存储：检查是否存在相似记忆，决定更新/合并/新建
func (m *Manager) storeWithUpdate(ctx context.Context, items []*entity.LongTermMemory) error {
	for _, newItem := range items {
		// 1. 精确匹配：完全相同的内容
		existing, err := m.repo.FindByContent(ctx, newItem.UserID, newItem.Category, newItem.Content)
		if err != nil {
			m.logger.Warn("查询精确匹配失败", zap.Error(err))
			// 降级：直接创建
			if err := m.repo.Store(ctx, newItem); err != nil {
				return err
			}
			continue
		}

		if existing != nil {
			// 精确匹配存在：更新（取更高的 importance/confidence）
			if newItem.Importance > existing.Importance {
				existing.Importance = newItem.Importance
			}
			if newItem.Confidence > existing.Confidence {
				existing.Confidence = newItem.Confidence
			}
			existing.SourceConversationID = newItem.SourceConversationID
			existing.SourceRunID = newItem.SourceRunID
			existing.IsSuppressed = false // 取消压制（如果之前被压制）
			if err := m.repo.Update(ctx, existing); err != nil {
				return err
			}
			m.logger.Debug("长期记忆精确匹配更新",
				zap.Uint("id", existing.ID),
				zap.String("content", existing.Content),
			)
			continue
		}

		// 2. 相似匹配：检查是否存在相似记忆（同类别 + 内容包含关系）
		similar, err := m.repo.FindSimilar(ctx, newItem.UserID, newItem.Category, newItem.Content)
		if err != nil {
			m.logger.Warn("查询相似记忆失败", zap.Error(err))
			// 降级：直接创建
			if err := m.repo.Store(ctx, newItem); err != nil {
				return err
			}
			continue
		}

		if len(similar) > 0 {
			// 找到最相似的记忆
			bestMatch := &similar[0]

			// 判断是否需要 Supersede（新记忆取代旧记忆）
			if m.shouldSupersede(bestMatch, newItem) {
				// 旧记忆被新记忆取代
				if err := m.repo.Supersede(ctx, bestMatch.ID, newItem); err != nil {
					return err
				}
				m.logger.Debug("长期记忆 Supersede",
					zap.Uint("old_id", bestMatch.ID),
					zap.String("old_content", bestMatch.Content),
					zap.String("new_content", newItem.Content),
				)
				continue
			}

			// 否则：合并（更新旧记忆的属性）
			if newItem.Importance > bestMatch.Importance {
				bestMatch.Importance = newItem.Importance
			}
			if newItem.Confidence > bestMatch.Confidence {
				bestMatch.Confidence = newItem.Confidence
			}
			bestMatch.SourceConversationID = newItem.SourceConversationID
			bestMatch.SourceRunID = newItem.SourceRunID
			bestMatch.IsSuppressed = false
			if err := m.repo.Update(ctx, bestMatch); err != nil {
				return err
			}
			m.logger.Debug("长期记忆合并更新",
				zap.Uint("id", bestMatch.ID),
				zap.String("content", bestMatch.Content),
			)
			continue
		}

		// 3. 无匹配：创建新记忆
		if err := m.repo.Store(ctx, newItem); err != nil {
			return err
		}
		m.logger.Debug("长期记忆新建",
			zap.String("category", newItem.Category),
			zap.String("content", newItem.Content),
		)
	}

	// 记忆发生实际变更，触发回调（如清空 Agent 指令缓存，使全局长期记忆及时生效）
	if m.onMemoryChange != nil {
		m.onMemoryChange()
	}

	return nil
}

// shouldSupersede 判断是否应该用新记忆取代旧记忆
// 规则非常保守：只有当新旧内容完全相同时才 Supersede
// 避免误合并不同语义的记忆
func (m *Manager) shouldSupersede(old *entity.LongTermMemory, new *entity.LongTermMemory) bool {
	// 必须完全相同的内容才考虑 Supersede
	if old.Content != new.Content {
		return false
	}

	// 新记忆重要性必须 >= 旧记忆
	if new.Importance < old.Importance {
		return false
	}

	return true
}

// ---------- 召回 + 渲染为 Prompt 文本 ----------

// categoryDisplayTitle 类别 → 显示标题
var categoryDisplayTitle = map[string]string{
	memtypes.CategoryPreference:  "用户偏好",
	memtypes.CategoryIdentity:    "用户画像",
	memtypes.CategoryEnvironment: "工作环境",
	memtypes.CategoryKnowledge:   "项目事实",
	memtypes.CategoryTask:        "持续性任务",
	memtypes.CategoryDecision:    "决策约定",
}

// RetrieveForPrompt 召回该用户活跃长期记忆并渲染为可注入 Prompt 的文本
// 先按 (importance * (1 + recall_count/max(1, 全局最大召回次数))) 估算动态权重排序，
// 截断到 maxTokens 内，按类别分组输出；顺便 MarkRecalled 更新召回时间+次数
func (m *Manager) RetrieveForPrompt(ctx context.Context, userID uint) (string, error) {
	if m.repo == nil {
		return "", nil
	}
	uid := m.ResolveUserID(userID)
	items, err := m.repo.ListActiveByUser(ctx, uid, m.maxItems)
	if err != nil {
		return "", fmt.Errorf("查询长期记忆失败: %w", err)
	}
	if len(items) == 0 {
		return "", nil
	}

	// 按类别分组渲染，并逐步累计 token 预算
	// P0 阶段用字符数近似 token：1 token ≈ 4 chars（中英文混合保守估算 = 3.5 chars）
	const charsPerToken = 3
	charsBudget := m.maxTokens * charsPerToken

	groups := map[string][]entity.LongTermMemory{}
	for _, it := range items {
		groups[it.Category] = append(groups[it.Category], it)
	}

	// 固定顺序输出：identity → preference → environment → knowledge → decision → sustainable_task
	order := []string{
		memtypes.CategoryIdentity,
		memtypes.CategoryPreference,
		memtypes.CategoryEnvironment,
		memtypes.CategoryKnowledge,
		memtypes.CategoryDecision,
		memtypes.CategoryTask,
	}

	var recalledIDs []uint
	var sections []string
	totalChars := 0

	for _, cat := range order {
		list, ok := groups[cat]
		if !ok || len(list) == 0 {
			continue
		}
		title := categoryDisplayTitle[cat]
		if title == "" {
			title = cat
		}
		sectionHeader := fmt.Sprintf("■ %s：\n", title)
		sectionHeaderLen := len(sectionHeader)
		sectionContent := ""
		sectionContentLen := 0
		sectionIDs := make([]uint, 0, len(list))
		sectionAny := false

		for i, it := range list {
			line := fmt.Sprintf("  %d. %s\n", i+1, it.Content)
			lineLen := len(line)
			if totalChars+sectionHeaderLen+sectionContentLen+lineLen > charsBudget {
				break
			}
			sectionContent += line
			sectionContentLen += lineLen
			sectionIDs = append(sectionIDs, it.ID)
			sectionAny = true
		}
		if sectionAny {
			sections = append(sections, sectionHeader+sectionContent)
			totalChars += sectionHeaderLen + sectionContentLen
			recalledIDs = append(recalledIDs, sectionIDs...)
		}
	}

	var result string
	if len(sections) > 0 {
		result = strings.TrimRight(strings.Join(sections, ""), "\n")
	}

	// 异步 MarkRecalled（不阻塞主链路）
	if len(recalledIDs) > 0 {
		go func() {
			markCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			for _, id := range recalledIDs {
				_ = m.repo.MarkRecalled(markCtx, id)
			}
		}()
	}

	if result == "" {
		return "", nil
	}
	m.logger.Debug("长期记忆召回",
		zap.Uint("user_id", uid),
		zap.Int("items", len(recalledIDs)),
		zap.Int("chars", len(result)),
	)
	return result, nil
}
