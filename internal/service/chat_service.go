package service

import (
	"context"
	"fmt"
	"strconv"

	"Qavor/internal/agent"
	"Qavor/internal/model/entity"
	"Qavor/internal/repository"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

// ChatServiceImpl 聊天服务实现
type ChatServiceImpl struct {
	agentMgr         *agent.AgentManager
	contextMgr       ContextManager
	modelSvc         ModelService
	messageRepo      repository.MessageRepository
	conversationRepo repository.ConversationRepository
	logger           *zap.Logger
}

// NewChatService 创建聊天服务
func NewChatService(
	agentMgr *agent.AgentManager,
	contextMgr ContextManager,
	modelSvc ModelService,
	messageRepo repository.MessageRepository,
	conversationRepo repository.ConversationRepository,
	logger *zap.Logger,
) *ChatServiceImpl {
	return &ChatServiceImpl{
		agentMgr:         agentMgr,
		contextMgr:       contextMgr,
		modelSvc:         modelSvc,
		messageRepo:      messageRepo,
		conversationRepo: conversationRepo,
		logger:           logger,
	}
}

// Chat 发送消息并获取回复
func (s *ChatServiceImpl) Chat(ctx context.Context, conversationID uint, agentSlug string, message string) (*ChatResult, error) {
	// conversationID == 0 表示不需要持久化（如标题生成场景）
	persist := conversationID > 0

	// 0. 解析模型 ID（用于短期记忆的摘要/状态抽取）
	modelID := s.resolveModelID(ctx, agentSlug)

	// 1. 保存用户消息
	if persist {
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
			if err := s.contextMgr.UpdateShortMemory(ctx, conversationID, userSchemaMsg, modelID); err != nil {
				s.logger.Warn("更新 Short Memory 失败", zap.Error(err))
			}
		}
	}

	// 3. 获取 Agent 配置，创建 LLM 客户端
	agentCfg, err := s.agentMgr.GetConfig(ctx, agentSlug)
	if err != nil {
		return nil, fmt.Errorf("获取 Agent 配置失败: %w", err)
	}

	// 从配置中获取模型 ID 并创建 ToolCallingChatModel
	var llmClient model.ToolCallingChatModel
	if modelIDStr, ok := agentCfg["model_id"].(string); ok && modelIDStr != "" {
		modelID, parseErr := strconv.ParseUint(modelIDStr, 10, 32)
		if parseErr == nil && s.modelSvc != nil {
			llmClient, _ = s.modelSvc.ResolveChatModel(ctx, uint(modelID))
		}
	}

	if llmClient == nil {
		return nil, fmt.Errorf("Agent 的 LLM 配置为空，请检查 agent_slug: %s 对应的模型配置", agentSlug)
	}

	var respContent string
	if persist {
		// 正常聊天：通过 Agent 执行（含工具调用）
		a, err := s.agentMgr.GetOrCreate(ctx, agentSlug, llmClient)
		if err != nil {
			return nil, fmt.Errorf("获取 Agent 失败: %w", err)
		}
		resp, err := a.Execute(ctx, message)
		if err != nil {
			return nil, fmt.Errorf("Agent 执行失败: %w", err)
		}
		respContent = resp.Content
	} else {
		// 标题生成：直接调用 LLM，不走 Agent 工具链，避免工具调用拖慢标题生成
		// 该路径不经 Agent.Execute，trace 由 Middleware 管理的 http.server Span 覆盖
		out, err := llmClient.Generate(ctx, []*schema.Message{
			{Role: schema.User, Content: message},
		})
		if err != nil {
			return nil, fmt.Errorf("LLM 调用失败: %w", err)
		}
		respContent = out.Content
	}

	// 5. 保存 Assistant 消息
	var messageID uint
	if persist {
		assistantMsg := &entity.Message{
			ConversationID: conversationID,
			Role:           "assistant",
			Content:        respContent,
		}
		if err := s.messageRepo.Create(assistantMsg); err != nil {
			s.logger.Error("保存 Assistant 消息失败", zap.Error(err))
		} else {
			messageID = assistantMsg.ID
		}

		// 6. 更新 Short Memory（Assistant 回复）
		if s.contextMgr != nil {
			assistantSchemaMsg := &schema.Message{
				Role:    schema.Assistant,
				Content: respContent,
			}
			if err := s.contextMgr.UpdateShortMemory(ctx, conversationID, assistantSchemaMsg, modelID); err != nil {
				s.logger.Warn("更新 Short Memory 失败", zap.Error(err))
			}
		}
	}

	return &ChatResult{
		MessageID:      messageID,
		ConversationID: conversationID,
		Content:        respContent,
		DeliveryStatus: "complete",
	}, nil
}

// resolveModelID 从 Agent 配置中解析模型 ID（用于短期记忆的摘要/状态抽取）
func (s *ChatServiceImpl) resolveModelID(ctx context.Context, agentSlug string) uint {
	agentCfg, err := s.agentMgr.GetConfig(ctx, agentSlug)
	if err != nil {
		return 0
	}
	if modelIDStr, ok := agentCfg["model_id"].(string); ok && modelIDStr != "" {
		if modelID, err := strconv.ParseUint(modelIDStr, 10, 32); err == nil {
			return uint(modelID)
		}
	}
	return 0
}
