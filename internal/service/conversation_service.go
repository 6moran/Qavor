package service

import (
	"context"

	shortterm "Qavor/internal/memory/short_term"
	"Qavor/internal/model/dto/request"
	dto "Qavor/internal/model/dto/response"
	"Qavor/internal/model/entity"
	"Qavor/internal/repository"
	"Qavor/pkg/errors"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ConversationService 会话服务接口
type ConversationService interface {
	CreateConversation(ctx context.Context, req *request.CreateConversationRequest) (*dto.ConversationResponse, error)
	GetConversation(ctx context.Context, id uint) (*dto.ConversationResponse, error)
	UpdateConversation(ctx context.Context, id uint, req *request.UpdateConversationRequest) (*dto.ConversationResponse, error)
	DeleteConversation(ctx context.Context, id uint) error
	ListConversations(ctx context.Context, req *request.ConversationListRequest) (*dto.ConversationListResponse, error)
	SearchConversations(ctx context.Context, query string, page, pageSize int) (*dto.ConversationListResponse, error)
	CloseConversation(ctx context.Context, id uint) (*dto.ConversationResponse, error)
	ArchiveConversation(ctx context.Context, id uint) (*dto.ConversationResponse, error)
	ClearContext(ctx context.Context, id uint) error
}

// conversationService 会话服务实现
type conversationService struct {
	conversationRepo repository.ConversationRepository
	shortTermMgr     shortterm.Manager
	logger           *zap.Logger
}

// NewConversationService 创建会话服务
func NewConversationService(conversationRepo repository.ConversationRepository, shortTermMgr shortterm.Manager, logger *zap.Logger) ConversationService {
	return &conversationService{
		conversationRepo: conversationRepo,
		shortTermMgr:     shortTermMgr,
		logger:           logger,
	}
}

// CreateConversation 创建会话
func (s *conversationService) CreateConversation(ctx context.Context, req *request.CreateConversationRequest) (*dto.ConversationResponse, error) {
	conversation := &entity.Conversation{
		ThreadID: uuid.New().String(),
		Title:    req.Title,
		Status:   "active",
		AgentID:  req.AgentID,
	}

	if req.ToolApprovalMode != "" {
		md := conversation.ExtraMetadata
		if md == nil {
			md = entity.JSON{}
		}
		md["tool_approval_mode"] = req.ToolApprovalMode
		conversation.ExtraMetadata = md
	}

	if err := s.conversationRepo.Create(ctx, conversation); err != nil {
		return nil, errors.New(errors.CodeInternalError, "创建会话失败")
	}

	return s.toResponse(conversation), nil
}

// GetConversation 获取会话详情
func (s *conversationService) GetConversation(ctx context.Context, id uint) (*dto.ConversationResponse, error) {
	conversation, err := s.conversationRepo.FindByID(ctx, id)
	if err != nil {
		return nil, errors.New(errors.CodeInternalError, "查询会话失败")
	}
	if conversation == nil {
		return nil, errors.New(errors.CodeConversationNotFound, "会话不存在")
	}

	return s.toResponse(conversation), nil
}

// UpdateConversation 更新会话
func (s *conversationService) UpdateConversation(ctx context.Context, id uint, req *request.UpdateConversationRequest) (*dto.ConversationResponse, error) {
	conversation, err := s.conversationRepo.FindByID(ctx, id)
	if err != nil {
		return nil, errors.New(errors.CodeInternalError, "查询会话失败")
	}
	if conversation == nil {
		return nil, errors.New(errors.CodeConversationNotFound, "会话不存在")
	}

	if req.Title != "" {
		conversation.Title = req.Title
	}
	if req.IsPinned != nil {
		conversation.IsPinned = *req.IsPinned
	}
	if req.ToolApprovalMode != "" {
		md := conversation.ExtraMetadata
		if md == nil {
			md = entity.JSON{}
		}
		md["tool_approval_mode"] = req.ToolApprovalMode
		conversation.ExtraMetadata = md
	}

	if err := s.conversationRepo.Update(ctx, conversation); err != nil {
		return nil, errors.New(errors.CodeInternalError, "更新会话失败")
	}

	return s.toResponse(conversation), nil
}

// DeleteConversation 删除会话
func (s *conversationService) DeleteConversation(ctx context.Context, id uint) error {
	conversation, err := s.conversationRepo.FindByID(ctx, id)
	if err != nil {
		return errors.New(errors.CodeInternalError, "查询会话失败")
	}
	if conversation == nil {
		return errors.New(errors.CodeConversationNotFound, "会话不存在")
	}

	// 清除 Redis 中的短期记忆，避免孤儿数据
	if s.shortTermMgr != nil {
		if err := s.shortTermMgr.ClearMemory(ctx, id); err != nil {
			s.logger.Warn("清除短期记忆失败", zap.Uint("conversation_id", id), zap.Error(err))
		}
	}

	return s.conversationRepo.Delete(ctx, id)
}

// ListConversations 获取会话列表
func (s *conversationService) ListConversations(ctx context.Context, req *request.ConversationListRequest) (*dto.ConversationListResponse, error) {
	page := req.Page
	if page < 1 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize < 1 {
		pageSize = 10
	}

	offset := (page - 1) * pageSize

	var conversations []entity.Conversation
	var total int64
	var err error

	if req.Status != "" {
		conversations, total, err = s.conversationRepo.ListWithStatus(ctx, req.Status, offset, pageSize)
	} else {
		conversations, total, err = s.conversationRepo.List(ctx, offset, pageSize)
	}

	if err != nil {
		return nil, errors.New(errors.CodeInternalError, "查询会话列表失败")
	}

	return s.toListResponse(conversations, total), nil
}

// SearchConversations 搜索会话
func (s *conversationService) SearchConversations(ctx context.Context, query string, page, pageSize int) (*dto.ConversationListResponse, error) {
	if query == "" {
		return nil, errors.New(errors.CodeInvalidParam, "搜索关键词不能为空")
	}

	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}

	offset := (page - 1) * pageSize

	conversations, total, err := s.conversationRepo.Search(ctx, query, offset, pageSize)
	if err != nil {
		return nil, errors.New(errors.CodeInternalError, "搜索会话失败")
	}

	return s.toListResponse(conversations, total), nil
}

// CloseConversation 关闭会话
func (s *conversationService) CloseConversation(ctx context.Context, id uint) (*dto.ConversationResponse, error) {
	conversation, err := s.conversationRepo.FindByID(ctx, id)
	if err != nil {
		return nil, errors.New(errors.CodeInternalError, "查询会话失败")
	}
	if conversation == nil {
		return nil, errors.New(errors.CodeConversationNotFound, "会话不存在")
	}

	if conversation.Status == "closed" {
		return nil, errors.New(errors.CodeConversationStatusInvalid, "会话已关闭")
	}
	if conversation.Status == "archived" {
		return nil, errors.New(errors.CodeConversationStatusInvalid, "会话已归档，无法关闭")
	}

	conversation.Status = "closed"
	if err := s.conversationRepo.Update(ctx, conversation); err != nil {
		return nil, errors.New(errors.CodeInternalError, "关闭会话失败")
	}

	return s.toResponse(conversation), nil
}

// ArchiveConversation 归档会话
func (s *conversationService) ArchiveConversation(ctx context.Context, id uint) (*dto.ConversationResponse, error) {
	conversation, err := s.conversationRepo.FindByID(ctx, id)
	if err != nil {
		return nil, errors.New(errors.CodeInternalError, "查询会话失败")
	}
	if conversation == nil {
		return nil, errors.New(errors.CodeConversationNotFound, "会话不存在")
	}

	if conversation.Status == "archived" {
		return nil, errors.New(errors.CodeConversationStatusInvalid, "会话已归档")
	}

	conversation.Status = "archived"
	if err := s.conversationRepo.Update(ctx, conversation); err != nil {
		return nil, errors.New(errors.CodeInternalError, "归档会话失败")
	}

	return s.toResponse(conversation), nil
}

// ClearContext 清空会话上下文（轻量重置：清除短期记忆，保留会话本身）
func (s *conversationService) ClearContext(ctx context.Context, id uint) error {
	// 清除 Redis 中的短期记忆
	if s.shortTermMgr != nil {
		if err := s.shortTermMgr.ClearMemory(ctx, id); err != nil {
			s.logger.Warn("清除短期记忆失败", zap.Uint("conversation_id", id), zap.Error(err))
			return errors.New(errors.CodeInternalError, "清除上下文失败")
		}
	}

	return nil
}

// toResponse 将实体转换为响应 DTO
func (s *conversationService) toResponse(conv *entity.Conversation) *dto.ConversationResponse {
	return &dto.ConversationResponse{
		ID:        conv.ID,
		ThreadID:  conv.ThreadID,
		AgentID:   conv.AgentID,
		Title:     conv.Title,
		Status:    conv.Status,
		IsPinned:  conv.IsPinned,
		CreatedAt: conv.CreatedAt,
		UpdatedAt: conv.UpdatedAt,
		Metadata:  conv.ExtraMetadata,
	}
}

// toListResponse 将会话列表转换为列表响应
func (s *conversationService) toListResponse(conversations []entity.Conversation, total int64) *dto.ConversationListResponse {
	items := make([]dto.ConversationResponse, 0, len(conversations))
	for _, conv := range conversations {
		items = append(items, *s.toResponse(&conv))
	}
	return &dto.ConversationListResponse{
		Total: total,
		Items: items,
	}
}
