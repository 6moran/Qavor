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
	logger        *zap.Logger
	repo          repository.LongTermMemoryRepository
	extractor     *Extractor
	maxItems      int  // 召回最大条目
	maxTokens     int  // 注入 Prompt 最大 Token 数
	defaultUserID uint // JWT 尚未带 UserID 时使用的默认 user（0 表示全局匿名）
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
		if err := m.repo.StoreBatch(storeCtx, entities); err != nil {
			m.logger.Warn("长期记忆批量入库失败", zap.Uint("user_id", uid), zap.Error(err))
			return
		}
		m.logger.Debug("长期记忆入库成功",
			zap.Uint("user_id", uid),
			zap.Int("count", len(entities)),
			zap.Uint("conv", conversationID),
		)
	})
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
