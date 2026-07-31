package repository

import (
	"errors"

	"Qavor/internal/model/entity"

	"gorm.io/gorm"
)

// ConversationRepository 会话仓储接口
type ConversationRepository interface {
	Create(conversation *entity.Conversation) error
	FindByID(id uint) (*entity.Conversation, error)
	FindByIDAndUserID(id, userID uint) (*entity.Conversation, error)
	Update(conversation *entity.Conversation) error
	Delete(id uint) error
	ListByUserID(userID uint, offset, limit int) ([]entity.Conversation, int64, error)
	ListByUserIDWithStatus(userID uint, status string, offset, limit int) ([]entity.Conversation, int64, error)
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
func (r *conversationRepository) Create(conversation *entity.Conversation) error {
	return r.db.Create(conversation).Error
}

// FindByID 根据 ID 查找会话
func (r *conversationRepository) FindByID(id uint) (*entity.Conversation, error) {
	var conversation entity.Conversation
	err := r.db.First(&conversation, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &conversation, nil
}

// FindByIDAndUserID 根据 ID 和用户 ID 查找会话（用户级权限校验）
func (r *conversationRepository) FindByIDAndUserID(id, userID uint) (*entity.Conversation, error) {
	var conversation entity.Conversation
	err := r.db.Where("id = ? AND user_id = ?", id, userID).First(&conversation).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &conversation, nil
}

// Update 更新会话（按需更新指定字段，避免零值覆盖）
func (r *conversationRepository) Update(conversation *entity.Conversation) error {
	updates := map[string]interface{}{
		"title":          conversation.Title,
		"status":         conversation.Status,
		"is_pinned":      conversation.IsPinned,
		"extra_metadata": conversation.ExtraMetadata,
	}
	return r.db.Model(conversation).Updates(updates).Error
}

// Delete 删除会话（软删除）
func (r *conversationRepository) Delete(id uint) error {
	return r.db.Delete(&entity.Conversation{}, id).Error
}

// ListByUserID 根据用户 ID 分页获取会话列表
func (r *conversationRepository) ListByUserID(userID uint, offset, limit int) ([]entity.Conversation, int64, error) {
	var conversations []entity.Conversation
	var total int64

	query := r.db.Model(&entity.Conversation{}).Where("user_id = ?", userID)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&conversations).Error
	if err != nil {
		return nil, 0, err
	}

	return conversations, total, nil
}

// ListByUserIDWithStatus 根据用户 ID 和状态分页获取会话列表
func (r *conversationRepository) ListByUserIDWithStatus(userID uint, status string, offset, limit int) ([]entity.Conversation, int64, error) {
	var conversations []entity.Conversation
	var total int64

	query := r.db.Model(&entity.Conversation{}).Where("user_id = ? AND status = ?", userID, status)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&conversations).Error
	if err != nil {
		return nil, 0, err
	}

	return conversations, total, nil
}
