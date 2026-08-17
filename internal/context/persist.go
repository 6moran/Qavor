package context

import (
	"context"

	"Qavor/internal/model/entity"
	"Qavor/internal/repository"

	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

// ContextPersister 消息持久化器
type ContextPersister struct {
	messageRepo repository.MessageRepository
	logger      *zap.Logger
}

// NewContextPersister 创建持久化器
func NewContextPersister(messageRepo repository.MessageRepository, logger *zap.Logger) *ContextPersister {
	return &ContextPersister{
		messageRepo: messageRepo,
		logger:      logger,
	}
}

// PersistUserMessage 立即保存用户消息（同步）
func (p *ContextPersister) PersistUserMessage(conversationID uint, userMsg *schema.Message) (uint, error) {
	userEntity := &entity.Message{
		ConversationID: conversationID,
		Role:           string(userMsg.Role),
		Content:        userMsg.Content,
	}
	if err := p.messageRepo.Create(userEntity); err != nil {
		p.logger.Error("保存用户消息失败", zap.Error(err))
		return 0, err
	}

	return userEntity.ID, nil
}

// PersistAssistantMessage 保存助手回复（同步）
func (p *ContextPersister) PersistAssistantMessage(conversationID uint, assistantMsg *schema.Message) error {
	assistantEntity := &entity.Message{
		ConversationID: conversationID,
		Role:           string(assistantMsg.Role),
		Content:        assistantMsg.Content,
	}
	if err := p.messageRepo.Create(assistantEntity); err != nil {
		p.logger.Error("保存助手回复失败", zap.Error(err))
		return err
	}

	return nil
}

// PersistAssistantMessageAsync 异步保存助手回复
func (p *ContextPersister) PersistAssistantMessageAsync(ctx context.Context, conversationID uint, assistantMsg *schema.Message) {
	go func() {
		asyncCtx := context.WithoutCancel(ctx)
		if err := p.PersistAssistantMessage(conversationID, assistantMsg); err != nil {
			p.logger.Error("异步保存助手回复失败", zap.Error(err))
		}
		_ = asyncCtx
	}()
}
