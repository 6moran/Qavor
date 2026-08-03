package service

import (
	"context"
	"encoding/json"
	"fmt"

	"Qavor/internal/model/dto/request"
	dto "Qavor/internal/model/dto/response"
	"Qavor/internal/model/entity"
	"Qavor/internal/repository"
	"Qavor/pkg/errors"

	"github.com/redis/go-redis/v9"
)

// MessageService 消息服务接口
type MessageService interface {
	CreateMessage(userID uint, req *request.CreateMessageRequest) (*dto.MessageResponse, error)
	GetMessage(id, conversationID uint) (*dto.MessageResponse, error)
	UpdateMessage(id, conversationID uint, req *request.UpdateMessageRequest) (*dto.MessageResponse, error)
	DeleteMessage(id, conversationID uint) error
	ListMessages(conversationID uint, req *request.MessageListRequest) (*dto.MessageListResponse, error)
	GetLatestMessage(conversationID uint) (*dto.MessageResponse, error)
}

// messageService 消息服务实现
type messageService struct {
	messageRepo      repository.MessageRepository
	conversationRepo repository.ConversationRepository
	redis            *redis.Client
}

// NewMessageService 创建消息服务
func NewMessageService(messageRepo repository.MessageRepository, conversationRepo repository.ConversationRepository, redis *redis.Client) MessageService {
	return &messageService{
		messageRepo:      messageRepo,
		conversationRepo: conversationRepo,
		redis:            redis,
	}
}

// CreateMessage 创建消息
func (s *messageService) CreateMessage(userID uint, req *request.CreateMessageRequest) (*dto.MessageResponse, error) {
	// 校验会话存在且属于当前用户
	conv, err := s.conversationRepo.FindByIDAndUserID(req.ConversationID, userID)
	if err != nil {
		return nil, err
	}
	if conv == nil {
		return nil, errors.New(errors.CodeConversationNotFound, "会话不存在")
	}

	// 校验会话状态
	if conv.Status != "active" {
		return nil, errors.New(errors.CodeConversationStatusInvalid, "会话已关闭或归档，无法发送消息")
	}

	// 设置默认消息类型
	messageType := req.MessageType
	if messageType == "" {
		messageType = "text"
	}

	message := &entity.Message{
		ConversationID: req.ConversationID,
		Role:           req.Role,
		Content:        req.Content,
		MessageType:    messageType,
		ImageContent:   req.ImageContent,
		ExtraMetadata:  req.ExtraMetadata,
	}

	if err := s.messageRepo.Create(message); err != nil {
		return nil, errors.New(errors.CodeInternalError, "创建消息失败")
	}

	// 发布到 Redis Stream，支持实时推送
	if err := s.publishToRedisStream(message); err != nil {
		// Redis 发布失败不影响主流程，仅记录日志
		fmt.Printf("Redis Stream 发布失败: %v\n", err)
	}

	return s.toResponse(message), nil
}

// publishToRedisStream 发布消息到 Redis Stream
func (s *messageService) publishToRedisStream(message *entity.Message) error {
	if s.redis == nil {
		return nil
	}

	streamKey := fmt.Sprintf("stream:conversation:%d:messages", message.ConversationID)

	// 转换为 DTO 再序列化，避免暴露 GORM 内部字段
	msgResp := s.toResponse(message)
	data, err := json.Marshal(msgResp)
	if err != nil {
		return err
	}

	// 发布到 Redis Stream
	ctx := context.Background()
	err = s.redis.XAdd(ctx, &redis.XAddArgs{
		Stream: streamKey,
		Values: map[string]interface{}{
			"message_id": message.ID,
			"role":       message.Role,
			"content":    message.Content,
			"data":       string(data),
		},
	}).Err()

	return err
}

// GetMessage 获取消息详情
func (s *messageService) GetMessage(id, conversationID uint) (*dto.MessageResponse, error) {
	message, err := s.messageRepo.FindByIDAndConversationID(id, conversationID)
	if err != nil {
		return nil, err
	}
	if message == nil {
		return nil, errors.New(errors.CodeMessageNotFound, "消息不存在")
	}

	return s.toResponse(message), nil
}

// UpdateMessage 更新消息
func (s *messageService) UpdateMessage(id, conversationID uint, req *request.UpdateMessageRequest) (*dto.MessageResponse, error) {
	message, err := s.messageRepo.FindByIDAndConversationID(id, conversationID)
	if err != nil {
		return nil, err
	}
	if message == nil {
		return nil, errors.New(errors.CodeMessageNotFound, "消息不存在")
	}

	if req.Content != "" {
		message.Content = req.Content
	}
	if req.ExtraMetadata != nil {
		message.ExtraMetadata = req.ExtraMetadata
	}

	if err := s.messageRepo.Update(message); err != nil {
		return nil, errors.New(errors.CodeInternalError, "更新消息失败")
	}

	return s.toResponse(message), nil
}

// DeleteMessage 删除消息
func (s *messageService) DeleteMessage(id, conversationID uint) error {
	message, err := s.messageRepo.FindByIDAndConversationID(id, conversationID)
	if err != nil {
		return err
	}
	if message == nil {
		return errors.New(errors.CodeMessageNotFound, "消息不存在")
	}

	return s.messageRepo.Delete(id)
}

// ListMessages 获取消息列表（正序：旧->新）
func (s *messageService) ListMessages(conversationID uint, req *request.MessageListRequest) (*dto.MessageListResponse, error) {
	page := req.Page
	if page < 1 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize < 1 {
		pageSize = 20
	}

	offset := (page - 1) * pageSize

	var messages []entity.Message
	var total int64
	var err error

	if req.Role != "" {
		messages, total, err = s.messageRepo.ListByConversationIDWithRole(conversationID, req.Role, offset, pageSize)
	} else {
		messages, total, err = s.messageRepo.ListByConversationID(conversationID, offset, pageSize)
	}

	if err != nil {
		return nil, err
	}

	items := make([]dto.MessageResponse, 0, len(messages))
	for _, msg := range messages {
		items = append(items, *s.toResponse(&msg))
	}

	return &dto.MessageListResponse{
		Total: total,
		Items: items,
	}, nil
}

// GetLatestMessage 获取最新消息
func (s *messageService) GetLatestMessage(conversationID uint) (*dto.MessageResponse, error) {
	message, err := s.messageRepo.GetLatestByConversationID(conversationID)
	if err != nil {
		return nil, err
	}
	if message == nil {
		return nil, errors.New(errors.CodeMessageNotFound, "会话暂无消息")
	}

	return s.toResponse(message), nil
}

// toResponse 将实体转换为响应 DTO
func (s *messageService) toResponse(msg *entity.Message) *dto.MessageResponse {
	return &dto.MessageResponse{
		ID:             msg.ID,
		ConversationID: msg.ConversationID,
		Role:           msg.Role,
		Content:        msg.Content,
		MessageType:    msg.MessageType,
		TokenCount:     msg.TokenCount,
		ImageContent:   msg.ImageContent,
		RunID:          msg.RunID,
		RequestID:      msg.RequestID,
		DeliveryStatus: msg.DeliveryStatus,
		CreatedAt:      msg.CreatedAt,
		UpdatedAt:      msg.UpdatedAt,
	}
}
