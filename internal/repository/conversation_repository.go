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
	Update(conversation *entity.Conversation) error
	Delete(id uint) error
	List(offset, limit int) ([]entity.Conversation, int64, error)
	ListWithStatus(status string, offset, limit int) ([]entity.Conversation, int64, error)
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

// List 分页获取会话列表
func (r *conversationRepository) List(offset, limit int) ([]entity.Conversation, int64, error) {
	var conversations []entity.Conversation
	var total int64

	query := r.db.Model(&entity.Conversation{})

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
func (r *conversationRepository) ListWithStatus(status string, offset, limit int) ([]entity.Conversation, int64, error) {
	var conversations []entity.Conversation
	var total int64

	query := r.db.Model(&entity.Conversation{}).Where("status = ?", status)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&conversations).Error
	if err != nil {
		return nil, 0, err
	}

	return conversations, total, nil
}
