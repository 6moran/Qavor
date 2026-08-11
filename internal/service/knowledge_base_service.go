// Package service 提供知识库和知识文件的业务逻辑层
package service

import (
	"context"
	"errors"

	"Qavor/internal/model/dto/request"
	"Qavor/internal/model/dto/response"
	"Qavor/internal/model/entity"
	"Qavor/internal/repository"
	bizerrors "Qavor/pkg/errors"
	"Qavor/pkg/logger"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// knowledgeBaseService 知识库服务实现
type knowledgeBaseService struct {
	repo      repository.KnowledgeBaseRepository
	modelRepo repository.ModelRepository
	fileRepo  repository.KnowledgeFileRepository
	storage   ObjectStorage
	agentRepo repository.AgentRepository
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
	resp := knowledgeBaseResponse(base)

	// 获取统计信息
	statsMap, err := s.repo.GetStatsByKBIDs([]string{kbID})
	if err == nil {
		if stats, ok := statsMap[kbID]; ok {
			resp.Stats = &response.KnowledgeBaseStats{
				FileCount:         stats.FileCount,
				ChunkCount:        stats.ChunkCount,
				TokenCount:        stats.TokenCount,
				TotalSize:         stats.TotalSize,
				ProcessingCount:   stats.ProcessingCount,
				PendingParseCount: stats.PendingParseCount,
				PendingIndexCount: stats.PendingIndexCount,
			}
		}
	}

	return resp, nil
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

	// 批量获取统计信息
	kbIDs := make([]string, 0, len(bases))
	for _, base := range bases {
		kbIDs = append(kbIDs, base.KBID)
	}
	statsMap, _ := s.repo.GetStatsByKBIDs(kbIDs)

	// 转换为响应格式
	items := make([]response.KnowledgeBaseResponse, 0, len(bases))
	for _, base := range bases {
		resp := knowledgeBaseResponse(base)
		if stats, ok := statsMap[base.KBID]; ok {
			resp.Stats = &response.KnowledgeBaseStats{
				FileCount:         stats.FileCount,
				ChunkCount:        stats.ChunkCount,
				TokenCount:        stats.TokenCount,
				TotalSize:         stats.TotalSize,
				ProcessingCount:   stats.ProcessingCount,
				PendingParseCount: stats.PendingParseCount,
				PendingIndexCount: stats.PendingIndexCount,
			}
		}
		items = append(items, *resp)
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

// Delete 删除知识库：先清理对象存储中的原文件与解析结果，再在单事务中级联删除分块、文件、处理任务和知识库记录。
func (s *knowledgeBaseService) Delete(kbID string) error {
	// 查询要删除的知识库
	base, err := s.repo.FindByKBID(kbID)
	if err != nil {
		return err
	}
	if base == nil {
		return knowledgeBaseNotFoundError()
	}
	// 收集文件记录，用于清理对象存储；记录缺失时跳过对象清理。
	files, err := s.fileRepo.ListAllByKBID(context.Background(), kbID)
	if err != nil {
		return err
	}
	for _, file := range files {
		if file.Path != "" {
			if err := s.storage.Delete(file.Path); err != nil && !errors.Is(err, ErrObjectNotFound) {
				return err
			}
		}
		if file.MarkdownFile != "" {
			if err := s.storage.Delete(file.MarkdownFile); err != nil && !errors.Is(err, ErrObjectNotFound) {
				return err
			}
		}
	}
	// 级联删除数据库记录（同一事务）。
	if err := s.repo.DeleteByKBID(kbID); err != nil {
		return err
	}
	// 联动清理：解除所有智能体对该知识库的绑定，避免 Agent 侧残留失效引用。
	if s.agentRepo != nil {
		if affected, err := s.agentRepo.UnbindKnowledge(context.Background(), kbID); err != nil {
			// 绑定清理失败不应阻断删除主流程，仅记录告警。
			if logger.Initialized() {
				logger.Warn("删除知识库后清理 Agent 绑定失败", zap.String("kb_id", kbID), zap.Error(err))
			}
		} else if affected > 0 && logger.Initialized() {
			logger.Info("删除知识库后已自动解绑 Agent", zap.String("kb_id", kbID), zap.Int64("affected_agents", affected))
		}
	}
	return nil
}

// knowledgeBaseNotFoundError 返回知识库不存在的错误
func knowledgeBaseNotFoundError() error {
	return bizerrors.New(bizerrors.CodeResourceNotFound, "知识库不存在")
}

// NewKnowledgeBaseService 创建知识库服务实例。
// fileRepo 与 storage 用于删除知识库时清理文件记录与对象存储；
// agentRepo 用于删除知识库时联动解除 Agent 绑定（可为 nil，此时跳过联动清理）。
func NewKnowledgeBaseService(repo repository.KnowledgeBaseRepository, modelRepo repository.ModelRepository, fileRepo repository.KnowledgeFileRepository, storage ObjectStorage, agentRepo repository.AgentRepository) KnowledgeBaseService {
	return &knowledgeBaseService{repo: repo, modelRepo: modelRepo, fileRepo: fileRepo, storage: storage, agentRepo: agentRepo}
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
