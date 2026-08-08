package service

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"Qavor/internal/agent"
	"Qavor/internal/model/entity"
	"Qavor/internal/repository"
	"Qavor/internal/sse"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

// ChatServiceImpl 聊天服务实现
type ChatServiceImpl struct {
	agentMgr         *agent.AgentManager
	contextMgr       ContextManager
	modelSvc         ModelService
	sseMgr           *sse.Manager
	messageRepo      repository.MessageRepository
	conversationRepo repository.ConversationRepository
	logger           *zap.Logger
}

// NewChatService 创建聊天服务
func NewChatService(
	agentMgr *agent.AgentManager,
	contextMgr ContextManager,
	modelSvc ModelService,
	sseMgr *sse.Manager,
	messageRepo repository.MessageRepository,
	conversationRepo repository.ConversationRepository,
	logger *zap.Logger,
) *ChatServiceImpl {
	return &ChatServiceImpl{
		agentMgr:         agentMgr,
		contextMgr:       contextMgr,
		modelSvc:         modelSvc,
		sseMgr:           sseMgr,
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

	// 获取 Agent（传入 LLM 客户端）
	a, err := s.agentMgr.GetOrCreate(ctx, agentSlug, llmClient)
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

// ChatStream 流式发送消息，通过 SSE 推送事件
func (s *ChatServiceImpl) ChatStream(ctx context.Context, conversationID uint, agentSlug string, message string) error {
	// 1. 保存用户消息
	userMsg := &entity.Message{
		ConversationID: conversationID,
		Role:           "user",
		Content:        message,
	}
	if err := s.messageRepo.Create(userMsg); err != nil {
		return fmt.Errorf("保存用户消息失败: %w", err)
	}

	// 2. 更新短期记忆（用户消息）
	if s.contextMgr != nil {
		userSchemaMsg := &schema.Message{
			Role:    schema.User,
			Content: message,
		}
		if err := s.contextMgr.UpdateShortMemory(ctx, conversationID, userSchemaMsg); err != nil {
			s.logger.Warn("更新 Short Memory 失败", zap.Error(err))
		}
	}

	// 3. 获取 Agent 配置，创建 LLM 客户端
	agentCfg, err := s.agentMgr.GetConfig(ctx, agentSlug)
	if err != nil {
		return fmt.Errorf("获取 Agent 配置失败: %w", err)
	}

	// 从配置中获取模型 ID 并创建 ToolCallingChatModel
	var llmClient model.ToolCallingChatModel
	if modelIDStr, ok := agentCfg["model_id"].(string); ok && modelIDStr != "" {
		modelID, parseErr := strconv.ParseUint(modelIDStr, 10, 32)
		if parseErr == nil && s.modelSvc != nil {
			llmClient, _ = s.modelSvc.ResolveChatModel(ctx, uint(modelID))
		}
	}

	// 获取 Agent（传入 LLM 客户端）
	a, err := s.agentMgr.GetOrCreate(ctx, agentSlug, llmClient)
	if err != nil {
		return fmt.Errorf("获取 Agent 失败: %w", err)
	}

	// 4. 发送 message.start 事件
	msgID := fmt.Sprintf("msg_%d_%d", conversationID, time.Now().UnixNano())
	s.sendSSEEvent("admin", sse.NewSSEEvent(sse.EventMessageStart, sse.MessageStartData{
		MessageID:      msgID,
		ConversationID: conversationID,
	}))

	// 5. 执行 Agent 并流式推送事件
	var fullContent string
	iter := a.ExecuteIter(ctx, message)
	for {
		event, ok := iter.Next()
		if !ok {
			break
		}
		if event.Err != nil {
			s.sendSSEEvent("admin", sse.NewSSEEvent(sse.EventMessageError, sse.ErrorData{
				Code:    "AGENT_ERROR",
				Message: event.Err.Error(),
			}))
			return event.Err
		}
		if event.Output != nil && event.Output.MessageOutput != nil {
			mv := event.Output.MessageOutput
			switch mv.Role {
			case schema.Assistant:
				// 模型输出
				if mv.IsStreaming && mv.MessageStream != nil {
					// 流式：从 StreamReader 逐块读取
					s.handleStreamingOutput("admin", msgID, mv.MessageStream)
				} else if mv.Message != nil {
					// 非流式：直接发送完整内容
					fullContent = mv.Message.Content
					s.sendSSEEvent("admin", sse.NewSSEEvent(sse.EventMessageDelta, sse.MessageDeltaData{
						MessageID: msgID,
						Content:   mv.Message.Content,
						Index:     0,
					}))
				}
			case schema.Tool:
				// 工具调用结果
				s.sendSSEEvent("admin", sse.NewSSEEvent(sse.EventToolCall, sse.ToolCallData{
					MessageID: msgID,
					ToolName:  mv.ToolName,
					Result:    mv.Message.Content,
				}))
			}
		}
	}

	// 6. 保存 Assistant 消息
	assistantMsg := &entity.Message{
		ConversationID: conversationID,
		Role:           "assistant",
		Content:        fullContent,
	}
	if err := s.messageRepo.Create(assistantMsg); err != nil {
		s.logger.Error("保存 Assistant 消息失败", zap.Error(err))
	}

	// 7. 更新短期记忆（Assistant 回复）
	if s.contextMgr != nil {
		assistantSchemaMsg := &schema.Message{
			Role:    schema.Assistant,
			Content: fullContent,
		}
		if err := s.contextMgr.UpdateShortMemory(ctx, conversationID, assistantSchemaMsg); err != nil {
			s.logger.Warn("更新 Short Memory 失败", zap.Error(err))
		}
	}

	// 8. 发送 message.complete 事件
	s.sendSSEEvent("admin", sse.NewSSEEvent(sse.EventMessageComplete, sse.MessageCompleteData{
		MessageID:    msgID,
		Content:      fullContent,
		FinishReason: "stop",
	}))

	return nil
}

// sendSSEEvent 发送 SSE 事件到用户
func (s *ChatServiceImpl) sendSSEEvent(username string, event sse.SSEEvent) {
	if s.sseMgr == nil {
		s.logger.Warn("SSE Manager 为空，无法发送事件")
		return
	}
	s.logger.Info("发送 SSE 事件",
		zap.String("username", username),
		zap.String("event_type", string(event.Type)),
		zap.Any("data", event.Data),
	)
	if err := s.sseMgr.SendToUser(username, event); err != nil {
		s.logger.Warn("发送 SSE 事件失败", zap.Error(err))
	}
}

// handleStreamingOutput 处理流式输出
func (s *ChatServiceImpl) handleStreamingOutput(username string, msgID string, stream *schema.StreamReader[*schema.Message]) {
	defer stream.Close()
	index := 0
	for {
		chunk, err := stream.Recv()
		if err != nil {
			break
		}
		if chunk != nil && chunk.Content != "" {
			s.sendSSEEvent(username, sse.NewSSEEvent(sse.EventMessageDelta, sse.MessageDeltaData{
				MessageID: msgID,
				Content:   chunk.Content,
				Index:     index,
			}))
			index++
		}
		if reasoning := extractReasoning(chunk); reasoning != "" {
			s.sendSSEEvent(username, sse.NewSSEEvent(sse.EventMessageDelta, sse.MessageDeltaData{
				MessageID: msgID,
				Reasoning: reasoning,
				Index:     index,
			}))
		}
	}
}

// extractReasoning 从流式消息中提取推理内容增量（reasoning part）
func extractReasoning(m *schema.Message) string {
	if m == nil {
		return ""
	}
	for _, part := range m.AssistantGenMultiContent {
		if part.Type == schema.ChatMessagePartTypeReasoning && part.Reasoning != nil {
			return part.Reasoning.Text
		}
	}
	return ""
}
