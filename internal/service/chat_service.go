package service

import (
	"context"
	"fmt"

	"Qavor/internal/agent"
	"Qavor/internal/model/entity"
	"Qavor/internal/repository"

	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

// ChatServiceImpl 聊天服务实现
type ChatServiceImpl struct {
	agentMgr         *agent.AgentManager
	contextMgr       ContextManager
	messageRepo      repository.MessageRepository
	conversationRepo repository.ConversationRepository
	logger           *zap.Logger
}

// NewChatService 创建聊天服务
func NewChatService(
	agentMgr *agent.AgentManager,
	contextMgr ContextManager,
	messageRepo repository.MessageRepository,
	conversationRepo repository.ConversationRepository,
	logger *zap.Logger,
) *ChatServiceImpl {
	return &ChatServiceImpl{
		agentMgr:         agentMgr,
		contextMgr:       contextMgr,
		messageRepo:      messageRepo,
		conversationRepo: conversationRepo,
		logger:           logger,
	}
}

// Chat 发送消息并获取回复
func (s *ChatServiceImpl) Chat(ctx context.Context, conversationID uint, agentSlug string, message string) (*ChatResult, error) {
	// 1. 保存用户消息
	userMsg := &entity.Message{
		ConversationID: conversationID,
		Role:           "user",
		Content:        message,
	}
	if err := s.messageRepo.Create(userMsg); err != nil {
		return nil, fmt.Errorf("保存用户消息失败: %w", err)
	}

	// 2. 更新 Short Memory（用户消息）
	if s.contextMgr != nil {
		userSchemaMsg := &schema.Message{
			Role:    schema.User,
			Content: message,
		}
		if err := s.contextMgr.UpdateShortMemory(ctx, conversationID, userSchemaMsg); err != nil {
			s.logger.Warn("更新 Short Memory 失败", zap.Error(err))
		}
	}

	// 3. 获取 Agent
	a, err := s.agentMgr.GetOrCreate(ctx, agentSlug, nil)
	if err != nil {
		return nil, fmt.Errorf("获取 Agent 失败: %w", err)
	}

	// 4. 执行 Agent
	resp, err := a.Execute(ctx, message)
	if err != nil {
		return nil, fmt.Errorf("Agent 执行失败: %w", err)
	}

	// 5. 保存 Assistant 消息
	assistantMsg := &entity.Message{
		ConversationID: conversationID,
		Role:           "assistant",
		Content:        resp.Content,
	}
	if err := s.messageRepo.Create(assistantMsg); err != nil {
		s.logger.Error("保存 Assistant 消息失败", zap.Error(err))
	}

	// 6. 更新 Short Memory（Assistant 回复）
	if s.contextMgr != nil {
		assistantSchemaMsg := &schema.Message{
			Role:    schema.Assistant,
			Content: resp.Content,
		}
		if err := s.contextMgr.UpdateShortMemory(ctx, conversationID, assistantSchemaMsg); err != nil {
			s.logger.Warn("更新 Short Memory 失败", zap.Error(err))
		}
	}

	return &ChatResult{
		MessageID:      assistantMsg.ID,
		ConversationID: conversationID,
		Content:        resp.Content,
		DeliveryStatus: "complete",
	}, nil
}

// ChatStream 流式发送消息（预留）
func (s *ChatServiceImpl) ChatStream(_ context.Context, _ uint, _ string, _ string) (<-chan *schema.Message, error) {
	// TODO: 实现流式输出，接入 SSE
	return nil, fmt.Errorf("流式输出暂未实现")
}
