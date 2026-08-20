package repository

import (
	"context"
	"strings"
	"time"

	"Qavor/internal/model/entity"

	"gorm.io/gorm"
)

// LongTermMemoryRepository 长期记忆仓储接口
type LongTermMemoryRepository interface {
	Store(ctx context.Context, m *entity.LongTermMemory) error
	StoreBatch(ctx context.Context, items []*entity.LongTermMemory) error
	ListByUser(ctx context.Context, userID uint) ([]entity.LongTermMemory, error)
	ListByUserAndCategory(ctx context.Context, userID uint, category string) ([]entity.LongTermMemory, error)
	ListActiveByUser(ctx context.Context, userID uint, limit int) ([]entity.LongTermMemory, error)
	FindByContent(ctx context.Context, userID uint, category string, content string) (*entity.LongTermMemory, error)
	FindSimilar(ctx context.Context, userID uint, category string, content string) ([]entity.LongTermMemory, error)
	Supersede(ctx context.Context, oldID uint, newMemory *entity.LongTermMemory) error
	Update(ctx context.Context, m *entity.LongTermMemory) error
	MarkRecalled(ctx context.Context, id uint) error
	Suppress(ctx context.Context, id uint) error
}

type longTermMemoryRepository struct {
	db *gorm.DB
}

// NewLongTermMemoryRepository 创建长期记忆仓储
func NewLongTermMemoryRepository(db *gorm.DB) LongTermMemoryRepository {
	return &longTermMemoryRepository{db: db}
}

// Store 保存单条记忆（若 userID+category+content 完全相同则 Upsert：更新重要性/置信度/召回来源）
func (r *longTermMemoryRepository) Store(ctx context.Context, m *entity.LongTermMemory) error {
	existing, err := r.FindByContent(ctx, m.UserID, m.Category, m.Content)
	if err != nil {
		return err
	}
	if existing != nil {
		// 合并：取更高的 importance/confidence，刷新来源信息
		if m.Importance > existing.Importance {
			existing.Importance = m.Importance
		}
		if m.Confidence > existing.Confidence {
			existing.Confidence = m.Confidence
		}
		if m.SourceConversationID > 0 {
			existing.SourceConversationID = m.SourceConversationID
		}
		if m.SourceRunID != "" {
			existing.SourceRunID = m.SourceRunID
		}
		existing.IsSuppressed = false
		return r.db.WithContext(ctx).Save(existing).Error
	}
	return r.db.WithContext(ctx).Create(m).Error
}

// StoreBatch 批量保存（逐条 Upsert，保证去重语义）
func (r *longTermMemoryRepository) StoreBatch(ctx context.Context, items []*entity.LongTermMemory) error {
	for _, m := range items {
		if err := r.Store(ctx, m); err != nil {
			return err
		}
	}
	return nil
}

// ListByUser 查询某用户的全部记忆（含被压制的，供 debug/管理用）
func (r *longTermMemoryRepository) ListByUser(ctx context.Context, userID uint) ([]entity.LongTermMemory, error) {
	var items []entity.LongTermMemory
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).
		Order("importance DESC, confidence DESC, created_at DESC").
		Find(&items).Error
	return items, err
}

// ListByUserAndCategory 按类别查询
func (r *longTermMemoryRepository) ListByUserAndCategory(ctx context.Context, userID uint, category string) ([]entity.LongTermMemory, error) {
	var items []entity.LongTermMemory
	err := r.db.WithContext(ctx).Where("user_id = ? AND category = ?", userID, category).
		Order("importance DESC, confidence DESC, created_at DESC").
		Find(&items).Error
	return items, err
}

// ListActiveByUser 查询活跃记忆（排除被压制的），按重要性排序，可选 limit
func (r *longTermMemoryRepository) ListActiveByUser(ctx context.Context, userID uint, limit int) ([]entity.LongTermMemory, error) {
	var items []entity.LongTermMemory
	q := r.db.WithContext(ctx).
		Where("user_id = ? AND is_suppressed = false", userID).
		Order("importance DESC, recall_count DESC, confidence DESC, created_at DESC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	err := q.Find(&items).Error
	return items, err
}

// FindByContent 按 (user, category, content) 精确匹配记忆
func (r *longTermMemoryRepository) FindByContent(ctx context.Context, userID uint, category string, content string) (*entity.LongTermMemory, error) {
	var m entity.LongTermMemory
	err := r.db.WithContext(ctx).Where("user_id = ? AND category = ? AND content = ?",
		userID, category, content).First(&m).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// FindSimilar 查找相似记忆（同类别 + 精确包含关系）
// 只有当新内容完全包含旧内容，或旧内容完全包含新内容时才认为相似
// 避免误匹配（如"用户是学生"和"用户的学生叫魏正想"不应该被关联）
func (r *longTermMemoryRepository) FindSimilar(ctx context.Context, userID uint, category string, content string) ([]entity.LongTermMemory, error) {
	var items []entity.LongTermMemory
	// 只查询同类别、未压制的记忆
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND category = ? AND is_suppressed = false",
			userID, category).
		Order("importance DESC, created_at DESC").
		Find(&items).Error
	if err != nil {
		return nil, err
	}

	// 在应用层进行精确的包含关系检查
	// 只有当两个内容有实质性的包含关系时才返回
	var similar []entity.LongTermMemory
	for _, item := range items {
		if isSubstantialOverlap(item.Content, content) {
			similar = append(similar, item)
		}
	}
	return similar, nil
}

// isSubstantialOverlap 检查两个内容是否有实质性重叠
// 规则：
// 1. 完全相同 → 是
// 2. 一个完全包含另一个 → 是
// 3. 只是部分词汇重叠 → 否
func isSubstantialOverlap(a, b string) bool {
	if a == b {
		return true
	}
	// 检查完全包含关系
	if strings.Contains(a, b) || strings.Contains(b, a) {
		return true
	}
	return false
}

// Update 更新记忆
func (r *longTermMemoryRepository) Update(ctx context.Context, m *entity.LongTermMemory) error {
	return r.db.WithContext(ctx).Save(m).Error
}

// Supersede 取代旧记忆：将旧记忆标记为已压制，创建新记忆
// 用于记忆更新场景（如用户说"项目已经从 Go 改成 Java 了"）
func (r *longTermMemoryRepository) Supersede(ctx context.Context, oldID uint, newMemory *entity.LongTermMemory) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. 压制旧记忆
		if err := tx.Model(&entity.LongTermMemory{}).
			Where("id = ?", oldID).
			Update("is_suppressed", true).Error; err != nil {
			return err
		}
		// 2. 创建新记忆
		return tx.Create(newMemory).Error
	})
}

// MarkRecalled 标记被召回：更新时间 +1、last_recalled_at = now
func (r *longTermMemoryRepository) MarkRecalled(ctx context.Context, id uint) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&entity.LongTermMemory{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"recall_count":     gorm.Expr("recall_count + 1"),
			"last_recalled_at": now,
		}).Error
}

// Suppress 压制某条记忆（判定过时/错误）
func (r *longTermMemoryRepository) Suppress(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Model(&entity.LongTermMemory{}).
		Where("id = ?", id).
		Update("is_suppressed", true).Error
}
