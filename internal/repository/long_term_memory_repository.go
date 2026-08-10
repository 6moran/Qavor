package repository

import (
	"context"
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

// FindByContent 按 (user, category, content) 定位记忆（用于 Upsert 去重）
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
