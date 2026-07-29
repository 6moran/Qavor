package service

import (
	"context"
	"os"
	"sync"

	"Qavor/internal/llm"
	"Qavor/internal/model/dto/request"
	dto "Qavor/internal/model/dto/response"
	"Qavor/internal/model/entity"
	"Qavor/internal/repository"
	"Qavor/pkg/errors"
)

// ModelProviderService 模型提供商服务接口
// 定义了模型提供商的 CRUD 操作以及 LLM 客户端管理功能
type ModelProviderService interface {
	// CreateProvider 创建模型提供商
	CreateProvider(req *request.CreateModelProviderRequest) (*dto.ModelProviderResponse, error)
	// GetProvider 根据 ID 获取模型提供商
	GetProvider(id uint) (*dto.ModelProviderResponse, error)
	// UpdateProvider 更新模型提供商
	UpdateProvider(id uint, req *request.UpdateModelProviderRequest) (*dto.ModelProviderResponse, error)
	// DeleteProvider 删除模型提供商
	DeleteProvider(id uint) error
	// ListProviders 获取模型提供商列表（分页）
	ListProviders(req *request.ModelProviderListRequest) (*dto.ModelProviderListResponse, error)
	// GetLLMClient 根据提供商 ID 和模型名称获取 LLM 客户端
	// 客户端会被缓存，使用 sync.Map 实现并发安全
	GetLLMClient(ctx context.Context, providerID string, model string) (*llm.RetryableClient, error)
	// GetLLMClientByCapability 根据能力类型获取 LLM 客户端
	// 会自动选择第一个支持该能力的提供商
	GetLLMClientByCapability(ctx context.Context, capability string, model string) (*llm.RetryableClient, error)
}

// modelProviderService 模型提供商服务实现
type modelProviderService struct {
	providerRepo repository.ModelProviderRepository
	// 使用 sync.Map 解决并发读写问题
	// sync.Map 是并发安全的，不需要额外加锁
	// 缓存 key 格式: "providerID:modelName"
	clientCache sync.Map
}

// NewModelProviderService 创建模型提供商服务
// 参数:
//   - providerRepo: 模型提供商仓储实例
//
// 返回: 实现 ModelProviderService 接口的服务实例
func NewModelProviderService(providerRepo repository.ModelProviderRepository) ModelProviderService {
	return &modelProviderService{
		providerRepo: providerRepo,
		// sync.Map 零值可用，不需要初始化
	}
}

// CreateProvider 创建模型提供商
// 验证 ProviderID 唯一性后创建新的模型提供商记录
// 参数:
//   - req: 创建模型提供商的请求参数
//
// 返回: 创建成功后的模型提供商响应，或错误信息
func (s *modelProviderService) CreateProvider(req *request.CreateModelProviderRequest) (*dto.ModelProviderResponse, error) {
	// 检查 provider_id 是否已存在，确保唯一性
	exists, err := s.providerRepo.FindByProviderID(req.ProviderID)
	if err != nil {
		return nil, err
	}
	if exists != nil {
		return nil, errors.New(errors.CodeModelProviderAlreadyExists, "Provider ID已存在")
	}

	// 构建模型提供商实体
	provider := &entity.ModelProvider{
		ProviderID:              req.ProviderID,
		DisplayName:             req.DisplayName,
		ProviderType:            req.ProviderType,
		DefaultProtocol:         req.DefaultProtocol,
		BaseURL:                 req.BaseURL,
		EmbeddingBaseURL:        req.EmbeddingBaseURL,
		RerankBaseURL:           req.RerankBaseURL,
		ModelsEndpoint:          req.ModelsEndpoint,
		EmbeddingModelsEndpoint: req.EmbeddingModelsEndpoint,
		RerankModelsEndpoint:    req.RerankModelsEndpoint,
		APIKeyEnv:               req.APIKeyEnv,
		APIKey:                  req.APIKey,
		Capabilities:            req.Capabilities,
		EnabledModels:           req.EnabledModels,
		HeadersJSON:             req.HeadersJSON,
		ExtraJSON:               req.ExtraJSON,
		IsEnabled:               true, // 新创建的提供商默认启用
	}

	// 保存到数据库
	if err := s.providerRepo.Create(provider); err != nil {
		return nil, err
	}

	return s.toResponse(provider), nil
}

// GetProvider 根据 ID 获取模型提供商
// 参数:
//   - id: 模型提供商的数据库 ID
//
// 返回: 模型提供商响应，或错误信息（包括不存在的情况）
func (s *modelProviderService) GetProvider(id uint) (*dto.ModelProviderResponse, error) {
	provider, err := s.providerRepo.FindByID(id)
	if err != nil {
		return nil, err
	}
	if provider == nil {
		return nil, errors.New(errors.CodeModelProviderNotFound, "模型提供商不存在")
	}
	return s.toResponse(provider), nil
}

// UpdateProvider 更新模型提供商
// 只更新请求中非空的字段，同时清除相关的 LLM 客户端缓存
// 参数:
//   - id: 模型提供商的数据库 ID
//   - req: 更新模型提供商的请求参数
//
// 返回: 更新后的模型提供商响应，或错误信息
func (s *modelProviderService) UpdateProvider(id uint, req *request.UpdateModelProviderRequest) (*dto.ModelProviderResponse, error) {
	provider, err := s.providerRepo.FindByID(id)
	if err != nil {
		return nil, err
	}
	if provider == nil {
		return nil, errors.New(errors.CodeModelProviderNotFound, "模型提供商不存在")
	}

	// 保存旧的 providerID 用于缓存清除
	oldProviderID := provider.ProviderID

	// 只更新请求中非空的字段
	if req.DisplayName != "" {
		provider.DisplayName = req.DisplayName
	}
	if req.BaseURL != "" {
		provider.BaseURL = req.BaseURL
	}
	if req.DefaultProtocol != "" {
		provider.DefaultProtocol = req.DefaultProtocol
	}
	if req.EmbeddingBaseURL != "" {
		provider.EmbeddingBaseURL = req.EmbeddingBaseURL
	}
	if req.RerankBaseURL != "" {
		provider.RerankBaseURL = req.RerankBaseURL
	}
	if req.ModelsEndpoint != "" {
		provider.ModelsEndpoint = req.ModelsEndpoint
	}
	if req.EmbeddingModelsEndpoint != "" {
		provider.EmbeddingModelsEndpoint = req.EmbeddingModelsEndpoint
	}
	if req.RerankModelsEndpoint != "" {
		provider.RerankModelsEndpoint = req.RerankModelsEndpoint
	}
	if req.APIKeyEnv != "" {
		provider.APIKeyEnv = req.APIKeyEnv
	}
	if req.APIKey != "" {
		provider.APIKey = req.APIKey
	}
	if req.Capabilities != nil {
		provider.Capabilities = req.Capabilities
	}
	if req.EnabledModels != nil {
		provider.EnabledModels = req.EnabledModels
	}
	if req.HeadersJSON != nil {
		provider.HeadersJSON = req.HeadersJSON
	}
	if req.ExtraJSON != nil {
		provider.ExtraJSON = req.ExtraJSON
	}
	if req.IsEnabled != nil {
		provider.IsEnabled = *req.IsEnabled
	}

	// 保存更新到数据库
	if err := s.providerRepo.Update(provider); err != nil {
		return nil, err
	}

	// 清除缓存：使用精确 Key 删除
	s.invalidateProviderCache(oldProviderID)

	return s.toResponse(provider), nil
}

// DeleteProvider 删除模型提供商
// 删除前会清除相关的 LLM 客户端缓存
// 参数:
//   - id: 模型提供商的数据库 ID
//
// 返回: 错误信息（如果存在）
func (s *modelProviderService) DeleteProvider(id uint) error {
	provider, err := s.providerRepo.FindByID(id)
	if err != nil {
		return err
	}
	if provider == nil {
		return errors.New(errors.CodeModelProviderNotFound, "模型提供商不存在")
	}

	// 清除缓存：使用精确 Key 删除
	s.invalidateProviderCache(provider.ProviderID)

	return s.providerRepo.Delete(id)
}

// invalidateProviderCache 清除指定提供商的所有缓存
// 从数据库查询该提供商启用的模型列表，精确删除每个模型的缓存
// 使用精确 Key 删除，避免 Range 遍历的性能问题
func (s *modelProviderService) invalidateProviderCache(providerID string) {
	// 从数据库查询该提供商的信息
	provider, err := s.providerRepo.FindByProviderID(providerID)
	if err != nil || provider == nil {
		return
	}

	// 从 EnabledModels 中提取所有模型名称，精确删除缓存
	// JSONArray 已经是 []interface{} 类型，直接遍历即可
	for _, model := range provider.EnabledModels {
		if modelName, ok := model.(string); ok {
			// 构建缓存 key 并删除
			cacheKey := providerID + ":" + modelName
			s.clientCache.Delete(cacheKey)
		}
	}
}

// ListProviders 获取模型提供商列表（分页）
// 支持关键词搜索和分页功能
// 参数:
//   - req: 列表查询请求参数，包含分页和搜索条件
//
// 返回: 分页的模型提供商列表响应，或错误信息
func (s *modelProviderService) ListProviders(req *request.ModelProviderListRequest) (*dto.ModelProviderListResponse, error) {
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
	providers, total, err := s.providerRepo.List(offset, pageSize, req.Keyword)
	if err != nil {
		return nil, err
	}

	// 转换为响应 DTO
	items := make([]dto.ModelProviderResponse, 0, len(providers))
	for _, provider := range providers {
		items = append(items, *s.toResponse(provider))
	}

	return &dto.ModelProviderListResponse{
		Total: total,
		Items: items,
	}, nil
}

// GetLLMClient 根据提供商 ID 和模型名称获取 LLM 客户端（并发安全）
// 使用 sync.Map + LoadOrStore 防止并发击穿和重复创建
// 流程：
// 1. 尝试从缓存读取
// 2. 缓存未命中则查询数据库
// 3. 创建新的 LLM 客户端
// 4. 使用原子操作写入缓存
func (s *modelProviderService) GetLLMClient(ctx context.Context, providerID string, model string) (*llm.RetryableClient, error) {
	// 构建缓存 key
	cacheKey := providerID + ":" + model

	// 1. 尝试从缓存读取（sync.Map 支持并发读）
	if client, ok := s.clientCache.Load(cacheKey); ok {
		return client.(*llm.RetryableClient), nil
	}

	// 2. 缓存未命中，查询数据库
	provider, err := s.providerRepo.FindByProviderID(providerID)
	if err != nil {
		return nil, err
	}
	if provider == nil {
		return nil, errors.New(errors.CodeModelProviderNotFound, "模型提供商不存在")
	}

	// 检查提供商是否启用
	if !provider.IsEnabled {
		return nil, errors.New(errors.CodeModelProviderDisabled, "模型提供商已禁用")
	}

	// 3. 解析 API Key（优先使用直接配置的，其次从环境变量读取）
	apiKey := provider.APIKey
	if provider.APIKeyEnv != "" && apiKey == "" {
		apiKey = os.Getenv(provider.APIKeyEnv)
	}
	if apiKey == "" {
		return nil, errors.New(errors.CodeModelProviderAPIKeyMissing, "API Key未配置")
	}

	// 4. 创建基础 LLM 客户端
	config := &llm.Config{
		Model:   model,
		APIKey:  apiKey,
		BaseURL: provider.BaseURL,
	}

	client, err := llm.NewClient(ctx, config)
	if err != nil {
		return nil, err
	}

	// 5. 包装为带超时控制的客户端
	timeoutClient := llm.NewTimeoutClient(client, nil)

	// 6. 包装为带重试的客户端（支持超时控制）
	retryableClient := llm.NewRetryableClient(timeoutClient, nil)

	// 7. 使用 LoadOrStore 原子操作写入缓存
	// 如果多个 goroutine 同时创建，只有第一个会成功存储，其他的会使用已存储的
	actual, _ := s.clientCache.LoadOrStore(cacheKey, retryableClient)
	return actual.(*llm.RetryableClient), nil
}

// GetLLMClientByCapability 根据能力类型获取 LLM 客户端
// 会自动选择第一个支持该能力的启用状态的提供商
// 参数:
//   - ctx: 上下文，用于控制超时和取消
//   - capability: 能力类型（如 "chat"、"embedding"、"rerank"）
//   - model: 模型名称
//
// 返回: LLM 客户端实例，或错误信息
func (s *modelProviderService) GetLLMClientByCapability(ctx context.Context, capability string, model string) (*llm.RetryableClient, error) {
	// 查找所有支持该能力的启用状态提供商
	providers, err := s.providerRepo.FindEnabledByCapability(capability)
	if err != nil {
		return nil, err
	}
	if len(providers) == 0 {
		return nil, errors.New(errors.CodeInvalidParam, "未找到支持该能力的模型提供商")
	}

	// 使用第一个可用的提供商
	return s.GetLLMClient(ctx, providers[0].ProviderID, model)
}

// toResponse 将实体转换为响应 DTO
// 负责 ModelProvider 实体到 ModelProviderResponse 的字段映射
func (s *modelProviderService) toResponse(provider *entity.ModelProvider) *dto.ModelProviderResponse {
	return &dto.ModelProviderResponse{
		ID:                      provider.ID,
		ProviderID:              provider.ProviderID,
		DisplayName:             provider.DisplayName,
		ProviderType:            provider.ProviderType,
		DefaultProtocol:         provider.DefaultProtocol,
		BaseURL:                 provider.BaseURL,
		EmbeddingBaseURL:        provider.EmbeddingBaseURL,
		RerankBaseURL:           provider.RerankBaseURL,
		ModelsEndpoint:          provider.ModelsEndpoint,
		EmbeddingModelsEndpoint: provider.EmbeddingModelsEndpoint,
		RerankModelsEndpoint:    provider.RerankModelsEndpoint,
		APIKeyEnv:               provider.APIKeyEnv,
		Capabilities:            provider.Capabilities,
		EnabledModels:           provider.EnabledModels,
		HeadersJSON:             provider.HeadersJSON,
		ExtraJSON:               provider.ExtraJSON,
		IsEnabled:               provider.IsEnabled,
		IsBuiltin:               provider.IsBuiltin,
		CreatedAt:               provider.CreatedAt,
		UpdatedAt:               provider.UpdatedAt,
	}
}
