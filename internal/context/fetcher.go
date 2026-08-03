package context

import (
	"context"

	"Qavor/internal/model/entity"
	"Qavor/internal/repository"

	"github.com/cloudwego/eino/schema"
)

// historyReader 历史消息读取器
type historyReader struct {
	messageRepo repository.MessageRepository
}

// NewHistoryReader 创建历史读取器
func NewHistoryReader(messageRepo repository.MessageRepository) *historyReader {
	return &historyReader{messageRepo: messageRepo}
}

// LoadHistory 加载历史消息
func (r *historyReader) LoadHistory(ctx context.Context, query *ContextHistoryQuery) ([]*schema.Message, error) {
	limit := query.Limit
	if limit <= 0 {
		limit = 50
	}

	var messages []entity.Message
	var err error

	if query.BeforeID > 0 {
		messages, err = r.messageRepo.ListBeforeID(query.ConversationID, query.BeforeID, limit)
	} else {
		messages, _, err = r.messageRepo.ListByConversationID(query.ConversationID, 0, limit)
	}

	if err != nil {
		return nil, err
	}

	if len(query.Roles) > 0 {
		messages = r.filterByRoles(messages, query.Roles)
	}

	return r.toSchemaMessages(messages), nil
}

// filterByRoles 按角色过滤消息
func (r *historyReader) filterByRoles(messages []entity.Message, roles []string) []entity.Message {
	roleSet := make(map[string]bool, len(roles))
	for _, role := range roles {
		roleSet[role] = true
	}

	filtered := make([]entity.Message, 0, len(messages))
	for _, msg := range messages {
		if roleSet[msg.Role] {
			filtered = append(filtered, msg)
		}
	}
	return filtered
}

// toSchemaMessages 将实体消息转换为 schema.Message
func (r *historyReader) toSchemaMessages(messages []entity.Message) []*schema.Message {
	result := make([]*schema.Message, 0, len(messages))
	for _, msg := range messages {
		result = append(result, &schema.Message{
			Role:    schema.RoleType(msg.Role),
			Content: msg.Content,
		})
	}
	return result
}
