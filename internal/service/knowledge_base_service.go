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

// knowledgeBaseService 知识库服务实现
type knowledgeBaseService struct {
	repo      repository.KnowledgeBaseRepository
	modelRepo repository.ModelRepository
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
	bases, total, err := s.repo.List((page-1)*pageSize, pageSize, req.Keyword)
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
	if req.EmbeddingModelID > 0 {
		if req.EmbeddingModelID != base.EmbeddingModelID {
			return nil, bizerrors.New(bizerrors.CodeConflict,
				"知识库的 Embedding 模型创建后不可修改；请新建知识库并重新入库")
		}
	}
	if req.ChatModelID > 0 {
		if err := validateKnowledgeBaseModel(s.modelRepo, req.ChatModelID, "chat"); err != nil {
			return nil, err
		}
		base.ChatModelID = req.ChatModelID
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
func NewKnowledgeBaseService(repo repository.KnowledgeBaseRepository, modelRepo repository.ModelRepository) KnowledgeBaseService {
	return &knowledgeBaseService{repo: repo, modelRepo: modelRepo}
}

// Create 创建知识库
func (s *knowledgeBaseService) Create(req *request.CreateKnowledgeBaseRequest) (*response.KnowledgeBaseResponse, error) {
	if err := validateKnowledgeBaseModel(s.modelRepo, req.EmbeddingModelID, "embedding"); err != nil {
		return nil, err
	}
	if err := validateKnowledgeBaseModel(s.modelRepo, req.ChatModelID, "chat"); err != nil {
		return nil, err
	}
	// 构建知识库实体
	base := &entity.KnowledgeBase{
		KBID:               uuid.NewString(), // 生成唯一标识
		Name:               req.DatabaseName,
		Description:        req.Description,
		EmbeddingModelID:   req.EmbeddingModelID,
		ChatModelID:        req.ChatModelID,
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
		EmbeddingModelID:   base.EmbeddingModelID,
		ChatModelID:        base.ChatModelID,
		EmbeddingModelSpec: base.EmbeddingModelSpec,
		LLMModelSpec:       base.LLMModelSpec,
		QueryParams:        base.QueryParams,
		AdditionalParams:   base.AdditionalParams,
		SampleQuestions:    base.SampleQuestions,
		CreatedAt:          base.CreatedAt,
		UpdatedAt:          base.UpdatedAt,
	}
}

func validateKnowledgeBaseModel(modelRepo repository.ModelRepository, modelID uint, expectedType string) error {
	if modelID == 0 {
		return bizerrors.New(bizerrors.CodeInvalidParam, expectedType+" 模型不能为空")
	}
	if modelRepo == nil {
		return bizerrors.New(bizerrors.CodeInternalError, "模型服务未配置")
	}
	model, err := modelRepo.FindByID(modelID)
	if err != nil {
		return err
	}
	if model == nil {
		return bizerrors.New(bizerrors.CodeResourceNotFound, "模型不存在")
	}
	if !model.Enabled {
		return bizerrors.New(bizerrors.CodeInvalidParam, "模型未启用")
	}
	if model.ModelType != expectedType {
		return bizerrors.New(bizerrors.CodeInvalidParam, "模型类型不匹配")
	}
	return nil
}
