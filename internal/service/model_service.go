package service

import (
	"context"
	"fmt"
	"time"

	"Qavor/internal/embedding"
	"Qavor/internal/llm"
	"Qavor/internal/model/dto/request"
	dto "Qavor/internal/model/dto/response"
	"Qavor/internal/model/entity"
	"Qavor/internal/rag"
	"Qavor/internal/repository"
	"Qavor/pkg/crypto"
	"Qavor/pkg/database/types"
	"Qavor/pkg/errors"

	einoEmbedding "github.com/cloudwego/eino/components/embedding"
	einoModel "github.com/cloudwego/eino/components/model"
)

// ModelService 模型服务接口
type ModelService interface {
	// CreateModel 创建模型
	CreateModel(req *request.CreateModelRequest) (*dto.ModelResponse, error)
	// GetModel 根据 ID 获取模型
	GetModel(id uint) (*dto.ModelResponse, error)
	// UpdateModel 更新模型
	UpdateModel(id uint, req *request.UpdateModelRequest) (*dto.ModelResponse, error)
	// DeleteModel 删除模型
	DeleteModel(id uint) error
	// ListModels 获取模型列表（分页）
	ListModels(req *request.ModelListRequest) (*dto.ModelListResponse, error)
	// GetModelWithDecryptedKey 获取模型（解密 API Key，用于内部使用）
	GetModelWithDecryptedKey(id uint) (*entity.Model, error)
	// CreateLLMClient 根据模型 ID 创建 LLM 客户端。
	CreateLLMClient(ctx context.Context, modelID uint) (llm.Client, error)
	// CreateEmbeddingClient 根据模型 ID 创建 Embedding 客户端。
	CreateEmbeddingClient(ctx context.Context, modelID uint) (embedding.Client, error)
	// ResolveEmbedding 根据模型管理中的 ID 创建原生 Eino Embedder。
	ResolveEmbedding(ctx context.Context, modelID uint) (einoEmbedding.Embedder, error)
	// ResolveChatModel 根据模型管理中的 ID 创建原生 Eino ChatModel。
	ResolveChatModel(ctx context.Context, modelID uint) (einoModel.ToolCallingChatModel, error)
	// ResolveChatModelWithTimeout 创建带超时下限的 Chat 模型客户端，用于生成类批处理场景。
	// 客户端超时取模型配置与 minTimeout 中的较大值；minTimeout 非正时与 ResolveChatModel 一致。
	ResolveChatModelWithTimeout(ctx context.Context, modelID uint, minTimeout time.Duration) (einoModel.ToolCallingChatModel, error)
	// ResolveReranker 根据模型管理中的 ID 创建重排客户端。
	ResolveReranker(ctx context.Context, modelID uint) (rag.Reranker, error)
	// TestConnection 测试未保存的模型配置是否能正常连接。
	TestConnection(ctx context.Context, req *request.ModelConnectionTestRequest) (*dto.ModelConnectionTestResponse, error)
	// FetchRemoteModels 远程拉取供应商的模型列表（OpenAI 兼容 /models 或 Ollama /api/tags）。
	FetchRemoteModels(ctx context.Context, req *request.FetchRemoteModelsRequest) ([]string, error)
	// SetModelConfigChangeCallback 设置模型配置变更回调
	SetModelConfigChangeCallback(callback func(modelID string))
	// GetModelInfo 获取模型基本信息，用于动态调整上下文窗口
	GetModelInfo(modelID uint) (provider, name string, contextWindow int, ok bool)
}

// modelService 模型服务实现
type modelService struct {
	modelRepo           repository.ModelRepository
	onModelConfigChange func(modelID string) // 模型配置变更回调
}

// NewModelService 创建模型服务
func NewModelService(modelRepo repository.ModelRepository) ModelService {
	return &modelService{
		modelRepo: modelRepo,
	}
}

// SetModelConfigChangeCallback 设置模型配置变更回调
func (s *modelService) SetModelConfigChangeCallback(callback func(modelID string)) {
	s.onModelConfigChange = callback
}

// CreateModel 创建模型
func (s *modelService) CreateModel(req *request.CreateModelRequest) (*dto.ModelResponse, error) {
	// 加密 API Key
	var encryptedAPIKey string
	if req.APIKey != "" {
		encrypted, err := crypto.Encrypt(req.APIKey)
		if err != nil {
			return nil, errors.New(errors.CodeInternalError, "API Key加密失败")
		}
		encryptedAPIKey = encrypted
	}

	// 设置默认值
	timeout := req.Timeout
	if timeout == 0 {
		timeout = 60000
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	modelType := req.ModelType
	if modelType == "" {
		modelType = "chat"
	}

	// 构建模型参数
	params := toModelParams(req.Params)

	// 构建实体
	model := &entity.Model{
		Name:            req.Name,
		Remark:          req.Remark,
		Protocol:        req.Protocol,
		BaseURL:         req.BaseURL,
		APIKey:          encryptedAPIKey,
		Headers:         types.StringMap(req.Headers),
		Timeout:         timeout,
		Enabled:         enabled,
		ModelType:       modelType,
		Params:          params,
		ContextWindow:   req.ContextWindow,
		MaxOutputTokens: req.MaxOutputTokens,
	}

	// 保存到数据库
	if err := s.modelRepo.Create(model); err != nil {
		return nil, err
	}

	return s.toResponse(model), nil
}

// GetModel 根据 ID 获取模型
func (s *modelService) GetModel(id uint) (*dto.ModelResponse, error) {
	model, err := s.modelRepo.FindByID(id)
	if err != nil {
		return nil, err
	}
	if model == nil {
		return nil, errors.New(errors.CodeInvalidParam, "模型不存在")
	}

	return s.toResponse(model), nil
}

// UpdateModel 更新模型
func (s *modelService) UpdateModel(id uint, req *request.UpdateModelRequest) (*dto.ModelResponse, error) {
	model, err := s.modelRepo.FindByID(id)
	if err != nil {
		return nil, err
	}
	if model == nil {
		return nil, errors.New(errors.CodeInvalidParam, "模型不存在")
	}

	// 保存旧的配置，用于清除缓存
	oldModel := &entity.Model{
		Name:     model.Name,
		Protocol: model.Protocol,
		BaseURL:  model.BaseURL,
		APIKey:   model.APIKey,
	}

	// 更新字段
	if req.Name != "" {
		model.Name = req.Name
	}
	model.Remark = req.Remark
	if req.Protocol != "" {
		model.Protocol = req.Protocol
	}
	if req.BaseURL != "" {
		model.BaseURL = req.BaseURL
	}
	if req.APIKey != "" {
		encrypted, err := crypto.Encrypt(req.APIKey)
		if err != nil {
			return nil, errors.New(errors.CodeInternalError, "API Key加密失败")
		}
		model.APIKey = encrypted
	}
	if req.Headers != nil {
		model.Headers = types.StringMap(req.Headers)
	}
	if req.Timeout > 0 {
		model.Timeout = req.Timeout
	}
	if req.Enabled != nil {
		model.Enabled = *req.Enabled
	}
	if req.ModelType != "" {
		model.ModelType = req.ModelType
	}
	if req.Params != nil {
		model.Params = toModelParams(req.Params)
	}
	if req.ContextWindow > 0 {
		model.ContextWindow = req.ContextWindow
	}
	if req.MaxOutputTokens > 0 {
		model.MaxOutputTokens = req.MaxOutputTokens
	}

	// 保存更新
	if err := s.modelRepo.Update(model); err != nil {
		return nil, err
	}

	// 清除 LLM 客户端缓存
	// 解密旧的 API Key 用于清除缓存
	if oldModel.APIKey != "" {
		decryptedOldKey, err := crypto.Decrypt(oldModel.APIKey)
		if err == nil {
			llm.ClearCache(oldModel.Protocol, oldModel.Name, decryptedOldKey, oldModel.BaseURL)
		}
	}

	// 调用回调函数，清除使用该模型的 Agent 缓存
	if s.onModelConfigChange != nil {
		modelIDStr := fmt.Sprintf("%d", id)
		s.onModelConfigChange(modelIDStr)
	}

	return s.toResponse(model), nil
}

// DeleteModel 删除模型
func (s *modelService) DeleteModel(id uint) error {
	model, err := s.modelRepo.FindByID(id)
	if err != nil {
		return err
	}
	if model == nil {
		return errors.New(errors.CodeInvalidParam, "模型不存在")
	}

	// 清除 LLM 客户端缓存
	if model.APIKey != "" {
		decryptedKey, err := crypto.Decrypt(model.APIKey)
		if err == nil {
			llm.ClearCache(model.Protocol, model.Name, decryptedKey, model.BaseURL)
		}
	}

	return s.modelRepo.Delete(id)
}

// ListModels 获取模型列表（分页）
func (s *modelService) ListModels(req *request.ModelListRequest) (*dto.ModelListResponse, error) {
	// 设置默认分页参数
	page := req.Page
	if page < 1 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize < 1 {
		pageSize = 10
	}

	// 计算偏移量
	offset := (page - 1) * pageSize

	// 查询数据库
	models, total, err := s.modelRepo.List(offset, pageSize, req.Keyword, req.ModelType)
	if err != nil {
		return nil, err
	}

	// 转换为响应 DTO
	items := make([]dto.ModelResponse, 0, len(models))
	for _, model := range models {
		items = append(items, *s.toResponse(model))
	}

	return &dto.ModelListResponse{
		Total: total,
		Items: items,
	}, nil
}

// GetModelWithDecryptedKey 获取模型（解密 API Key，用于内部使用）
func (s *modelService) GetModelWithDecryptedKey(id uint) (*entity.Model, error) {
	model, err := s.modelRepo.FindByID(id)
	if err != nil {
		return nil, err
	}
	if model == nil {
		return nil, errors.New(errors.CodeInvalidParam, "模型不存在")
	}

	// 解密 API Key
	if model.APIKey != "" {
		decrypted, err := crypto.Decrypt(model.APIKey)
		if err != nil {
			return nil, errors.New(errors.CodeInternalError, "API Key解密失败")
		}
		model.APIKey = decrypted
	}

	return model, nil
}

// CreateLLMClient 创建 LLM Client
func (s *modelService) CreateLLMClient(ctx context.Context, modelID uint) (llm.Client, error) {
	model, err := s.GetModelWithDecryptedKey(modelID)
	if err != nil {
		return nil, err
	}

	if model.ModelType != "chat" {
		return nil, errors.New(errors.CodeInvalidParam, "模型类型不是 chat")
	}

	return llm.NewClient(ctx, model.Protocol, model.Name, model.APIKey, model.BaseURL, model.Timeout)
}

// CreateEmbeddingClient 创建 Embedding Client
func (s *modelService) CreateEmbeddingClient(ctx context.Context, modelID uint) (embedding.Client, error) {
	model, err := s.GetModelWithDecryptedKey(modelID)
	if err != nil {
		return nil, err
	}

	if model.ModelType != "embedding" {
		return nil, errors.New(errors.CodeInvalidParam, "模型类型不是 embedding")
	}
	if !model.Enabled {
		return nil, errors.New(errors.CodeInvalidParam, "模型未启用")
	}

	return embedding.NewOpenAIClient(ctx, model.APIKey, model.Name, model.BaseURL, model.Timeout)
}

// ResolveEmbedding 根据模型管理中的配置创建 Eino Embedding 组件。
func (s *modelService) ResolveEmbedding(ctx context.Context, modelID uint) (einoEmbedding.Embedder, error) {
	client, err := s.CreateEmbeddingClient(ctx, modelID)
	if err != nil {
		return nil, err
	}
	return embedding.AsEinoEmbedder(client), nil
}

// ResolveChatModel 根据模型管理中的配置创建原生 Eino ChatModel。
func (s *modelService) ResolveChatModel(ctx context.Context, modelID uint) (einoModel.ToolCallingChatModel, error) {
	return s.ResolveChatModelWithTimeout(ctx, modelID, 0)
}

// ResolveChatModelWithTimeout 创建带超时下限的 Chat 模型客户端。
// 批处理场景（如生成示例问题）可传入较长 minTimeout，避免模型配置的短超时导致生成失败。
func (s *modelService) ResolveChatModelWithTimeout(ctx context.Context, modelID uint, minTimeout time.Duration) (einoModel.ToolCallingChatModel, error) {
	model, err := s.GetModelWithDecryptedKey(modelID)
	if err != nil {
		return nil, err
	}
	if !model.Enabled {
		return nil, errors.New(errors.CodeInvalidParam, "模型未启用")
	}
	if model.ModelType != "chat" {
		return nil, errors.New(errors.CodeInvalidParam, "模型类型不是 chat")
	}
	timeout := model.Timeout
	if minTimeout > 0 && time.Duration(timeout)*time.Millisecond < minTimeout {
		timeout = int(minTimeout / time.Millisecond)
	}
	client, err := llm.NewClient(ctx, model.Protocol, model.Name, model.APIKey, model.BaseURL, timeout)
	if err != nil {
		return nil, err
	}
	chatModel := client.GetToolCallingModel()
	if chatModel == nil {
		return nil, errors.New(errors.CodeInternalError, "LLM client 不支持 ToolCallingChatModel")
	}
	return chatModel, nil
}

// ResolveReranker 根据模型管理中的配置创建重排客户端。
func (s *modelService) ResolveReranker(_ context.Context, modelID uint) (rag.Reranker, error) {
	model, err := s.GetModelWithDecryptedKey(modelID)
	if err != nil {
		return nil, err
	}
	if !model.Enabled {
		return nil, errors.New(errors.CodeInvalidParam, "模型未启用")
	}
	if model.ModelType != "rerank" {
		return nil, errors.New(errors.CodeInvalidParam, "模型类型不是 rerank")
	}
	headers := make(map[string]string, len(model.Headers))
	for key, value := range model.Headers {
		headers[key] = value
	}
	return rag.NewHTTPReranker(rag.HTTPRerankerConfig{
		Model:   model.Name,
		BaseURL: model.BaseURL,
		APIKey:  model.APIKey,
		Headers: headers,
		Timeout: time.Duration(model.Timeout) * time.Millisecond,
	})
}

// toResponse 将实体转换为响应 DTO
func (s *modelService) toResponse(model *entity.Model) *dto.ModelResponse {
	return &dto.ModelResponse{
		ID:        model.ID,
		Name:      model.Name,
		Remark:    model.Remark,
		Protocol:  model.Protocol,
		BaseURL:   model.BaseURL,
		Headers:   map[string]string(model.Headers),
		Timeout:   model.Timeout,
		Enabled:   model.Enabled,
		ModelType: model.ModelType,
		Params: dto.ModelParams{
			MaxTokens:        model.Params.MaxTokens,
			Temperature:      model.Params.Temperature,
			TopP:             model.Params.TopP,
			PresencePenalty:  model.Params.PresencePenalty,
			FrequencyPenalty: model.Params.FrequencyPenalty,
			Stop:             model.Params.Stop,
		},
		ContextWindow:   model.ContextWindow,
		MaxOutputTokens: model.MaxOutputTokens,
		CreatedAt:       model.CreatedAt,
		UpdatedAt:       model.UpdatedAt,
	}
}

// toModelParams 将请求参数转换为实体参数
func toModelParams(req *request.ModelParams) types.ModelParams {
	params := types.DefaultModelParams()
	if req != nil {
		if req.MaxTokens > 0 {
			params.MaxTokens = req.MaxTokens
		}
		if req.Temperature > 0 {
			params.Temperature = req.Temperature
		}
		if req.TopP > 0 {
			params.TopP = req.TopP
		}
		params.PresencePenalty = req.PresencePenalty
		params.FrequencyPenalty = req.FrequencyPenalty
		params.Stop = req.Stop
	}
	return params
}

// GetModelInfo 获取模型基本信息，用于动态调整上下文窗口
// 返回 provider、name、context_window 和是否成功
func (s *modelService) GetModelInfo(modelID uint) (provider, name string, contextWindow int, ok bool) {
	model, err := s.modelRepo.FindByID(modelID)
	if err != nil || model == nil {
		return "", "", 0, false
	}
	return model.Protocol, model.Name, model.ContextWindow, true
}
