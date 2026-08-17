package repository

import (
	"context"
	"errors"

	"Qavor/internal/model/entity"

	"gorm.io/gorm"
)

// ConversationRepository 会话仓储接口
type ConversationRepository interface {
	Create(ctx context.Context, conversation *entity.Conversation) error
	FindByID(ctx context.Context, id uint) (*entity.Conversation, error)
	FindByThreadID(ctx context.Context, threadID string) (*entity.Conversation, error)
	Update(ctx context.Context, conversation *entity.Conversation) error
	Delete(ctx context.Context, id uint) error
	List(ctx context.Context, offset, limit int) ([]entity.Conversation, int64, error)
	ListWithStatus(ctx context.Context, status string, offset, limit int) ([]entity.Conversation, int64, error)
	Search(ctx context.Context, query string, offset, limit int) ([]entity.Conversation, int64, error)
}

// conversationRepository 会话仓储实现
type conversationRepository struct {
	db *gorm.DB
}

// NewConversationRepository 创建会话仓储
func NewConversationRepository(db *gorm.DB) ConversationRepository {
	return &conversationRepository{db: db}
}

// Create 创建会话
func (r *conversationRepository) Create(ctx context.Context, conversation *entity.Conversation) error {
	return r.db.WithContext(ctx).Create(conversation).Error
}

// FindByID 根据 ID 查找会话
func (r *conversationRepository) FindByID(ctx context.Context, id uint) (*entity.Conversation, error) {
	var conversation entity.Conversation
	err := r.db.WithContext(ctx).First(&conversation, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &conversation, nil
}

// FindByThreadID 根据 ThreadID 查找会话
func (r *conversationRepository) FindByThreadID(ctx context.Context, threadID string) (*entity.Conversation, error) {
	var conversation entity.Conversation
	err := r.db.WithContext(ctx).Where("thread_id = ?", threadID).First(&conversation).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &conversation, nil
}

// Update 更新会话（按需更新指定字段，避免零值覆盖）
func (r *conversationRepository) Update(ctx context.Context, conversation *entity.Conversation) error {
	updates := map[string]interface{}{
		"title":          conversation.Title,
		"status":         conversation.Status,
		"is_pinned":      conversation.IsPinned,
		"extra_metadata": conversation.ExtraMetadata,
	}
	return r.db.WithContext(ctx).Model(conversation).Updates(updates).Error
}

// Delete 删除会话（软删除）
func (r *conversationRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&entity.Conversation{}, id).Error
}

// List 分页获取会话列表
func (r *conversationRepository) List(ctx context.Context, offset, limit int) ([]entity.Conversation, int64, error) {
	var conversations []entity.Conversation
	var total int64

	query := r.db.WithContext(ctx).Model(&entity.Conversation{})

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&conversations).Error
	if err != nil {
		return nil, 0, err
	}

	return conversations, total, nil
}

// ListWithStatus 根据状态分页获取会话列表
func (r *conversationRepository) ListWithStatus(ctx context.Context, status string, offset, limit int) ([]entity.Conversation, int64, error) {
	var conversations []entity.Conversation
	var total int64

	query := r.db.WithContext(ctx).Model(&entity.Conversation{}).Where("status = ?", status)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&conversations).Error
	if err != nil {
		return nil, 0, err
	}

	return conversations, total, nil
}

// Search 搜索会话（按标题模糊匹配）
func (r *conversationRepository) Search(ctx context.Context, query string, offset, limit int) ([]entity.Conversation, int64, error) {
	var conversations []entity.Conversation
	var total int64

	dbQuery := r.db.WithContext(ctx).Model(&entity.Conversation{}).Where("title LIKE ?", "%"+query+"%")

	if err := dbQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := dbQuery.Order("is_pinned DESC, updated_at DESC").Offset(offset).Limit(limit).Find(&conversations).Error
	if err != nil {
		return nil, 0, err
	}

	return conversations, total, nil
}
