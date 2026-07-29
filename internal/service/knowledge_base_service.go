// Package service 提供知识库和知识文件的业务逻辑层
package service

import (
	"Qavor/internal/model/dto/request"
	"Qavor/internal/model/dto/response"
	"Qavor/internal/model/entity"
	"Qavor/internal/repository"
	bizerrors "Qavor/pkg/errors"

	"github.com/google/uuid"
)

// defaultKnowledgeBaseType 是未显式指定 kb_type 时使用的向量存储后端。
const defaultKnowledgeBaseType = "pgvector"

// knowledgeBaseService 知识库服务实现
type knowledgeBaseService struct {
	repo repository.KnowledgeBaseRepository
}

// Get 根据知识库ID获取知识库详情
func (s *knowledgeBaseService) Get(kbID string) (*response.KnowledgeBaseResponse, error) {
	// 根据KBID查询知识库
	base, err := s.repo.FindByKBID(kbID)
	if err != nil {
		return nil, err
	}
	// 知识库不存在则返回错误
	if base == nil {
		return nil, knowledgeBaseNotFoundError()
	}
	return knowledgeBaseResponse(base), nil
}

// List 分页获取知识库列表
func (s *knowledgeBaseService) List(req *request.KnowledgeBaseListRequest) (*response.KnowledgeBaseListResponse, error) {
	// 参数校验和默认值设置
	page := req.Page
	if page < 1 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize < 1 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}

	// 查询知识库列表
	bases, total, err := s.repo.List((page-1)*pageSize, pageSize, req.Keyword, req.KBType)
	if err != nil {
		return nil, err
	}
	// 转换为响应格式
	items := make([]response.KnowledgeBaseResponse, 0, len(bases))
	for _, base := range bases {
		items = append(items, *knowledgeBaseResponse(base))
	}
	return &response.KnowledgeBaseListResponse{Total: total, Items: items}, nil
}

// Update 更新知识库信息
func (s *knowledgeBaseService) Update(kbID string, req *request.UpdateKnowledgeBaseRequest) (*response.KnowledgeBaseResponse, error) {
	// 查询要更新的知识库
	base, err := s.repo.FindByKBID(kbID)
	if err != nil {
		return nil, err
	}
	if base == nil {
		return nil, knowledgeBaseNotFoundError()
	}
	// 更新非空字段
	if req.Name != "" {
		base.Name = req.Name
	}
	if req.Description != "" {
		base.Description = req.Description
	}
	if req.LLMModelSpec != "" {
		base.LLMModelSpec = req.LLMModelSpec
	}
	if req.AdditionalParams != nil {
		base.AdditionalParams = req.AdditionalParams
	}
	// 保存更新
	if err := s.repo.Update(base); err != nil {
		return nil, err
	}
	return knowledgeBaseResponse(base), nil
}

// Delete 删除知识库
func (s *knowledgeBaseService) Delete(kbID string) error {
	// 查询要删除的知识库
	base, err := s.repo.FindByKBID(kbID)
	if err != nil {
		return err
	}
	if base == nil {
		return knowledgeBaseNotFoundError()
	}
	// 执行删除
	return s.repo.DeleteByKBID(kbID)
}

// knowledgeBaseNotFoundError 返回知识库不存在的错误
func knowledgeBaseNotFoundError() error {
	return bizerrors.New(bizerrors.CodeResourceNotFound, "知识库不存在")
}

// NewKnowledgeBaseService 创建知识库服务实例
func NewKnowledgeBaseService(repo repository.KnowledgeBaseRepository) KnowledgeBaseService {
	return &knowledgeBaseService{repo: repo}
}

// Create 创建知识库
func (s *knowledgeBaseService) Create(req *request.CreateKnowledgeBaseRequest) (*response.KnowledgeBaseResponse, error) {
	kbType := req.KBType
	if kbType == "" {
		kbType = defaultKnowledgeBaseType
	}
	// 构建知识库实体
	base := &entity.KnowledgeBase{
		KBID:               uuid.NewString(), // 生成唯一标识
		Name:               req.DatabaseName,
		Description:        req.Description,
		KBType:             kbType,
		EmbeddingModelSpec: req.EmbeddingModelSpec,
		LLMModelSpec:       req.LLMModelSpec,
		QueryParams:        req.QueryParams,
		AdditionalParams:   req.AdditionalParams,
		SampleQuestions:    req.SampleQuestions,
	}
	// 保存到数据库
	if err := s.repo.Create(base); err != nil {
		return nil, err
	}
	return knowledgeBaseResponse(base), nil
}

// knowledgeBaseResponse 将知识库实体转换为响应格式
func knowledgeBaseResponse(base *entity.KnowledgeBase) *response.KnowledgeBaseResponse {
	return &response.KnowledgeBaseResponse{
		ID:                 base.ID,
		KBID:               base.KBID,
		Name:               base.Name,
		Description:        base.Description,
		KBType:             base.KBType,
		EmbeddingModelSpec: base.EmbeddingModelSpec,
		LLMModelSpec:       base.LLMModelSpec,
		QueryParams:        base.QueryParams,
		AdditionalParams:   base.AdditionalParams,
		SampleQuestions:    base.SampleQuestions,
		CreatedAt:          base.CreatedAt,
		UpdatedAt:          base.UpdatedAt,
	}
}
