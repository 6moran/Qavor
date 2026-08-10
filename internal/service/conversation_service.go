package service

import (
	shortterm "Qavor/internal/memory/short_term"
	"Qavor/internal/model/dto/request"
	dto "Qavor/internal/model/dto/response"
	"Qavor/internal/model/entity"
	"Qavor/internal/repository"
	"Qavor/pkg/errors"
	"context"

	"github.com/google/uuid"
)

// ConversationService 会话服务接口
type ConversationService interface {
	CreateConversation(req *request.CreateConversationRequest) (*dto.ConversationResponse, error)
	GetConversation(id uint) (*dto.ConversationResponse, error)
	UpdateConversation(id uint, req *request.UpdateConversationRequest) (*dto.ConversationResponse, error)
	DeleteConversation(id uint) error
	ListConversations(req *request.ConversationListRequest) (*dto.ConversationListResponse, error)
	SearchConversations(query string, page, pageSize int) (*dto.ConversationListResponse, error)
	CloseConversation(id uint) (*dto.ConversationResponse, error)
	ArchiveConversation(id uint) (*dto.ConversationResponse, error)
}

// conversationService 会话服务实现
type conversationService struct {
	conversationRepo repository.ConversationRepository
	shortTermMgr     shortterm.Manager
}

// NewConversationService 创建会话服务
func NewConversationService(conversationRepo repository.ConversationRepository, shortTermMgr shortterm.Manager) ConversationService {
	return &conversationService{
		conversationRepo: conversationRepo,
		shortTermMgr:     shortTermMgr,
	}
}

// CreateConversation 创建会话
func (s *conversationService) CreateConversation(req *request.CreateConversationRequest) (*dto.ConversationResponse, error) {
	conversation := &entity.Conversation{
		ThreadID: uuid.New().String(),
		Title:    req.Title,
		Status:   "active",
		AgentID:  req.AgentID,
	}

	if err := s.conversationRepo.Create(conversation); err != nil {
		return nil, errors.New(errors.CodeInternalError, "创建会话失败")
	}

	return s.toResponse(conversation), nil
}

// GetConversation 获取会话详情
func (s *conversationService) GetConversation(id uint) (*dto.ConversationResponse, error) {
	conversation, err := s.conversationRepo.FindByID(id)
	if err != nil {
		return nil, err
	}
	if conversation == nil {
		return nil, errors.New(errors.CodeConversationNotFound, "会话不存在")
	}

	return s.toResponse(conversation), nil
}

// UpdateConversation 更新会话
func (s *conversationService) UpdateConversation(id uint, req *request.UpdateConversationRequest) (*dto.ConversationResponse, error) {
	conversation, err := s.conversationRepo.FindByID(id)
	if err != nil {
		return nil, err
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

	if err := s.conversationRepo.Update(conversation); err != nil {
		return nil, errors.New(errors.CodeInternalError, "更新会话失败")
	}

	return s.toResponse(conversation), nil
}

// DeleteConversation 删除会话
func (s *conversationService) DeleteConversation(id uint) error {
	conversation, err := s.conversationRepo.FindByID(id)
	if err != nil {
		return err
	}
	if conversation == nil {
		return errors.New(errors.CodeConversationNotFound, "会话不存在")
	}

	// 清除 Redis 中的短期记忆，避免孤儿数据
	if s.shortTermMgr != nil {
		ctx := context.Background()
		if err := s.shortTermMgr.ClearMemory(ctx, id); err != nil {
			// 记忆清除失败不阻断删除流程，仅记录
			_ = err
		}
	}

	return s.conversationRepo.Delete(id)
}

// ListConversations 获取会话列表
func (s *conversationService) ListConversations(req *request.ConversationListRequest) (*dto.ConversationListResponse, error) {
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
		conversations, total, err = s.conversationRepo.ListWithStatus(req.Status, offset, pageSize)
	} else {
		conversations, total, err = s.conversationRepo.List(offset, pageSize)
	}

	if err != nil {
		return nil, err
	}

	items := make([]dto.ConversationResponse, 0, len(conversations))
	for _, conv := range conversations {
		items = append(items, *s.toResponse(&conv))
	}

	return &dto.ConversationListResponse{
		Total: total,
		Items: items,
	}, nil
}

// SearchConversations 搜索会话
func (s *conversationService) SearchConversations(query string, page, pageSize int) (*dto.ConversationListResponse, error) {
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

	conversations, total, err := s.conversationRepo.Search(query, offset, pageSize)
	if err != nil {
		return nil, err
	}

	items := make([]dto.ConversationResponse, 0, len(conversations))
	for _, conv := range conversations {
		items = append(items, *s.toResponse(&conv))
	}

	return &dto.ConversationListResponse{
		Total: total,
		Items: items,
	}, nil
}

// CloseConversation 关闭会话
func (s *conversationService) CloseConversation(id uint) (*dto.ConversationResponse, error) {
	conversation, err := s.conversationRepo.FindByID(id)
	if err != nil {
		return nil, err
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
	if err := s.conversationRepo.Update(conversation); err != nil {
		return nil, errors.New(errors.CodeInternalError, "关闭会话失败")
	}

	return s.toResponse(conversation), nil
}

// ArchiveConversation 归档会话
func (s *conversationService) ArchiveConversation(id uint) (*dto.ConversationResponse, error) {
	conversation, err := s.conversationRepo.FindByID(id)
	if err != nil {
		return nil, err
	}
	if conversation == nil {
		return nil, errors.New(errors.CodeConversationNotFound, "会话不存在")
	}

	if conversation.Status == "archived" {
		return nil, errors.New(errors.CodeConversationStatusInvalid, "会话已归档")
	}

	conversation.Status = "archived"
	if err := s.conversationRepo.Update(conversation); err != nil {
		return nil, errors.New(errors.CodeInternalError, "归档会话失败")
	}

	return s.toResponse(conversation), nil
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
	}
}
