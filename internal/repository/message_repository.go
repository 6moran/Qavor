package repository

import (
	"errors"

	"Qavor/internal/model/entity"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// MessageRepository 消息仓储接口
type MessageRepository interface {
	Create(message *entity.Message) error
	FindByID(id uint) (*entity.Message, error)
	FindByIDAndConversationID(id, conversationID uint) (*entity.Message, error)
	Update(message *entity.Message) error
	Delete(id uint) error
	ListByConversationID(conversationID uint, offset, limit int) ([]entity.Message, int64, error)
	ListByConversationIDWithRole(conversationID uint, role string, offset, limit int) ([]entity.Message, int64, error)
	CountByConversationID(conversationID uint) (int64, error)
	GetLatestByConversationID(conversationID uint) (*entity.Message, error)
	ListBeforeID(conversationID uint, beforeID uint, limit int) ([]entity.Message, error)
}

// messageRepository 消息仓储实现
type messageRepository struct {
	db *gorm.DB
}

// NewMessageRepository 创建消息仓储
func NewMessageRepository(db *gorm.DB) MessageRepository {
	return &messageRepository{db: db}
}

// Create 创建消息
func (r *messageRepository) Create(message *entity.Message) error {
	// 保存消息（不含关联，避免 GORM 自动处理 has-many 可能失败）
	if err := r.db.Omit(clause.Associations).Create(message).Error; err != nil {
		return err
	}
	// 显式保存 ToolCalls
	if len(message.ToolCalls) > 0 {
		for i := range message.ToolCalls {
			message.ToolCalls[i].MessageID = message.ID
		}
		if err := r.db.Create(&message.ToolCalls).Error; err != nil {
			return err
		}
	}
	return nil
}

// FindByID 根据 ID 查找消息
func (r *messageRepository) FindByID(id uint) (*entity.Message, error) {
	var message entity.Message
	err := r.db.First(&message, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &message, nil
}

// FindByIDAndConversationID 根据 ID 和会话 ID 查找消息
func (r *messageRepository) FindByIDAndConversationID(id, conversationID uint) (*entity.Message, error) {
	var message entity.Message
	err := r.db.Where("id = ? AND conversation_id = ?", id, conversationID).First(&message).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &message, nil
}

// Update 更新消息（按需更新指定字段，避免零值覆盖）
func (r *messageRepository) Update(message *entity.Message) error {
	updates := map[string]interface{}{
		"content":         message.Content,
		"extra_metadata":  message.ExtraMetadata,
		"delivery_status": message.DeliveryStatus,
	}
	return r.db.Model(message).Updates(updates).Error
}

// Delete 删除消息（软删除）
func (r *messageRepository) Delete(id uint) error {
	return r.db.Delete(&entity.Message{}, id).Error
}

// ListByConversationID 根据会话 ID 分页获取消息列表（正序：旧->新）
func (r *messageRepository) ListByConversationID(conversationID uint, offset, limit int) ([]entity.Message, int64, error) {
	var messages []entity.Message
	var total int64

	query := r.db.Model(&entity.Message{}).Where("conversation_id = ?", conversationID)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.Preload("ToolCalls").Order("created_at ASC").Offset(offset).Limit(limit).Find(&messages).Error
	if err != nil {
		return nil, 0, err
	}

	return messages, total, nil
}

// ListByConversationIDWithRole 根据会话 ID 和角色分页获取消息列表（正序：旧->新）
func (r *messageRepository) ListByConversationIDWithRole(conversationID uint, role string, offset, limit int) ([]entity.Message, int64, error) {
	var messages []entity.Message
	var total int64

	query := r.db.Model(&entity.Message{}).Where("conversation_id = ? AND role = ?", conversationID, role)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.Preload("ToolCalls").Order("created_at ASC").Offset(offset).Limit(limit).Find(&messages).Error
	if err != nil {
		return nil, 0, err
	}

	return messages, total, nil
}

// CountByConversationID 统计会话下的消息数量
func (r *messageRepository) CountByConversationID(conversationID uint) (int64, error) {
	var count int64
	err := r.db.Model(&entity.Message{}).Where("conversation_id = ?", conversationID).Count(&count).Error
	return count, err
}

// GetLatestByConversationID 获取会话下最新的一条消息
func (r *messageRepository) GetLatestByConversationID(conversationID uint) (*entity.Message, error) {
	var message entity.Message
	err := r.db.Where("conversation_id = ?", conversationID).Order("created_at DESC").First(&message).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &message, nil
}

// ListBeforeID 根据会话 ID 获取指定 ID 之前的消息（正序）
func (r *messageRepository) ListBeforeID(conversationID uint, beforeID uint, limit int) ([]entity.Message, error) {
	var messages []entity.Message
	err := r.db.Where("conversation_id = ? AND id < ?", conversationID, beforeID).
		Order("created_at ASC").
		Limit(limit).
		Find(&messages).Error
	return messages, err
}
