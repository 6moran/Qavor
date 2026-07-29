# 模型集成实现指南

> **重要提示**：本文档中的代码需要按顺序创建实际文件。请确保按照文档中的路径创建相应的目录和文件。

## 架构概述

基于现有代码结构，模型集成需要实现以下层次：

```
├── internal/
│   ├── llm/                    # LLM客户端层
│   │   ├── llm.go             # 当前OpenAI客户端
│   │   ├── retry.go           # 重试逻辑（新增）
│   │   ├── middleware.go      # 超时控制（新增）
│   │   ├── factory.go         # 模型工厂（新增）
│   │   └── provider/          # 各模型提供商适配器（新增）
│   ├── repository/            # 数据访问层
│   │   └── model_provider_repository.go  # 新增
│   ├── service/               # 业务逻辑层
│   │   └── model_provider_service.go     # 新增
│   └── api/v1/model_provider/ # API控制器层（需要手动创建目录）
│       ├── controller.go      # 新增
│       └── routes.go          # 新增
├── pkg/
│   └── utils/
│       └── token.go           # Token计算工具（新增）
```

## 注释规范

**所有代码必须包含详细的中文注释**，包括：
- 包级注释：说明包的功能和用途
- 类型注释：说明结构体/接口的作用
- 函数注释：说明函数的功能、参数、返回值
- 关键逻辑注释：说明算法思路和业务逻辑

## 实现步骤

> **实现顺序**：请按照以下顺序创建文件，确保依赖关系正确：
> 1. 先创建 `pkg/utils/token.go`（无依赖）
> 2. 再创建 `internal/llm/retry.go` 和 `internal/llm/middleware.go`（依赖 llm.go）
> 3. 然后创建 `internal/repository/model_provider_repository.go`（依赖 entity）
> 4. 接着创建 `internal/service/model_provider_service.go`（依赖 repository 和 llm）
> 5. 最后创建 `internal/api/v1/model_provider/controller.go`（依赖 service）

### 1. Token 计算与裁剪逻辑

在 `pkg/utils/token.go` 中实现：

> **推荐方案**：使用 `tiktoken-go` 库进行精确的 Token 计算（兼容 OpenAI 的 cl100k_base 编码）
> 需要先安装：`go get github.com/pkoukk/tiktoken-go`

```go
package utils

import (
    "github.com/cloudwego/eino/schema"
    "github.com/pkoukk/tiktoken-go"
)

// tokenizer 缓存 tokenizer 实例，避免重复创建
var tokenizer *tiktoken.Tiktoken

// init 初始化 tokenizer
func init() {
    // 使用 cl100k_base 编码（GPT-3.5/GPT-4 使用）
    var err error
    tokenizer, err = tiktoken.GetEncoding("cl100k_base")
    if err != nil {
        // 如果初始化失败，使用简单的估算方法作为备选
        tokenizer = nil
    }
}

// CountTokens 计算文本的 token 数量
// 优先使用 tiktoken 精确计算，如果不可用则使用简单估算
func CountTokens(text string) int {
    if text == "" {
        return 0
    }
    
    // 优先使用 tiktoken 精确计算
    if tokenizer != nil {
        return len(tokenizer.Encode(text, nil, nil))
    }
    
    // 备选方案：简单估算（英文按空格分词，中文按字分词）
    return estimateTokens(text)
}

// estimateTokens 简单估算 token 数量（备选方案）
func estimateTokens(text string) int {
    count := 0
    runes := []rune(text)
    i := 0
    
    for i < len(runes) {
        r := runes[i]
        
        // 中文字符
        if r >= 0x4E00 && r <= 0x9FFF {
            count++
            i++
            continue
        }
        
        // 英文单词
        if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
            // 读取完整单词
            for i < len(runes) && ((runes[i] >= 'a' && runes[i] <= 'z') || 
                (runes[i] >= 'A' && runes[i] <= 'Z') || (runes[i] >= '0' && runes[i] <= '9')) {
                i++
            }
            count++
            continue
        }
        
        // 其他字符（标点、空格等）
        count++
        i++
    }
    
    return count
}

// CountMessageTokens 计算消息列表的 token 总数
// 包含消息固定开销（角色、分隔符等）
func CountMessageTokens(messages []*schema.Message) int {
    total := 0
    for _, msg := range messages {
        // 每条消息有固定开销（角色、分隔符等）
        total += 4  // 消息固定开销
        total += CountTokens(msg.Content)
    }
    total += 2  // 对话结尾标记
    return total
}

// TrimMessages 根据 token 限制裁剪消息列表
// 保留系统消息和最近的消息，确保不超出 token 限制
func TrimMessages(messages []*schema.Message, maxTokens int) []*schema.Message {
    if len(messages) == 0 {
        return messages
    }
    
    // 分离系统消息和对话消息
    var systemMessages []*schema.Message
    var chatMessages []*schema.Message
    
    for _, msg := range messages {
        if msg.Role == schema.System {
            systemMessages = append(systemMessages, msg)
        } else {
            chatMessages = append(chatMessages, msg)
        }
    }
    
    // 计算系统消息的 token
    systemTokens := CountMessageTokens(systemMessages)
    
    // 如果系统消息已经超限，安全处理边界情况
    if systemTokens >= maxTokens {
        if len(systemMessages) > 0 {
            return []*schema.Message{systemMessages[0]}
        }
        return []*schema.Message{}
    }
    
    // 从最新消息开始保留，直到达到 token 限制
    remainingTokens := maxTokens - systemTokens
    var reversedChat []*schema.Message
    
    for i := len(chatMessages) - 1; i >= 0; i-- {
        msgTokens := CountMessageTokens([]*schema.Message{chatMessages[i]})
        if remainingTokens-msgTokens < 0 {
            break
        }
        remainingTokens -= msgTokens
        reversedChat = append(reversedChat, chatMessages[i])
    }
    
    // 翻转回正确的时间顺序
    trimmedChat := make([]*schema.Message, len(reversedChat))
    for i, msg := range reversedChat {
        trimmedChat[len(reversedChat)-1-i] = msg
    }
    
    // 合并系统消息和裁剪后的对话消息
    result := make([]*schema.Message, 0, len(systemMessages)+len(trimmedChat))
    result = append(result, systemMessages...)
    result = append(result, trimmedChat...)
    
    return result
}
```

### 2. 重试与超时控制

在 `internal/llm/retry.go` 中实现：

```go
package llm

import (
    "context"
    "math"
    "math/rand"
    "time"

    "github.com/cloudwego/eino/components/model"
    "github.com/cloudwego/eino/schema"
    pkgerrors "Qavor/pkg/errors"
)

// RetryConfig 重试配置
type RetryConfig struct {
    MaxRetries     int           // 最大重试次数
    InitialBackoff time.Duration // 初始退避时间
    MaxBackoff     time.Duration // 最大退避时间
    BackoffFactor  float64       // 退避因子
}

// DefaultRetryConfig 默认重试配置
func DefaultRetryConfig() *RetryConfig {
    return &RetryConfig{
        MaxRetries:     3,
        InitialBackoff: 1 * time.Second,
        MaxBackoff:     30 * time.Second,
        BackoffFactor:  2.0,
    }
}

// RetryableClient 支持重试的LLM客户端
type RetryableClient struct {
    *Client
    retryConfig *RetryConfig
}

// NewRetryableClient 创建支持重试的客户端
func NewRetryableClient(client *Client, retryConfig *RetryConfig) *RetryableClient {
    if retryConfig == nil {
        retryConfig = DefaultRetryConfig()
    }
    return &RetryableClient{
        Client:      client,
        retryConfig: retryConfig,
    }
}

// GenerateWithRetry 带重试的同步生成
func (c *RetryableClient) GenerateWithRetry(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
    var lastErr error
    
    for attempt := 0; attempt <= c.retryConfig.MaxRetries; attempt++ {
        // 如果不是第一次尝试，等待退避时间
        if attempt > 0 {
            backoff := c.calculateBackoff(attempt)
            select {
            case <-ctx.Done():
                return nil, ctx.Err()
            case <-time.After(backoff):
            }
        }
        
        result, err := c.Generate(ctx, input, opts...)
        if err == nil {
            return result, nil
        }
        
        lastErr = err
        
        // 检查是否应该重试
        if !c.isRetryableError(err) {
            return nil, err
        }
    }
    
    return nil, pkgerrors.NewWithErr(pkgerrors.CodeLLMRequestFailed, 
        "max retries exceeded", lastErr)
}

// StreamWithRetry 带重试的流式生成
func (c *RetryableClient) StreamWithRetry(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
    var lastErr error
    
    for attempt := 0; attempt <= c.retryConfig.MaxRetries; attempt++ {
        if attempt > 0 {
            backoff := c.calculateBackoff(attempt)
            select {
            case <-ctx.Done():
                return nil, ctx.Err()
            case <-time.After(backoff):
            }
        }
        
        result, err := c.Stream(ctx, input, opts...)
        if err == nil {
            return result, nil
        }
        
        lastErr = err
        
        if !c.isRetryableError(err) {
            return nil, err
        }
    }
    
    return nil, pkgerrors.NewWithErr(pkgerrors.CodeLLMRequestFailed,
        "max retries exceeded", lastErr)
}

// calculateBackoff 计算退避时间
func (c *RetryableClient) calculateBackoff(attempt int) time.Duration {
    backoff := float64(c.retryConfig.InitialBackoff) * math.Pow(c.retryConfig.BackoffFactor, float64(attempt-1))
    
    // 添加随机抖动
    jitter := rand.Float64() * 0.1 * backoff
    backoff += jitter
    
    // 限制最大退避时间
    if time.Duration(backoff) > c.retryConfig.MaxBackoff {
        backoff = float64(c.retryConfig.MaxBackoff)
    }
    
    return time.Duration(backoff)
}

// isRetryableError 判断错误是否可重试
func (c *RetryableClient) isRetryableError(err error) bool {
    // 网络错误可重试
    if _, ok := err.(interface{ Timeout() bool }); ok {
        return true
    }
    
    // HTTP 429 (Too Many Requests) 可重试
    if pkgerrors.Is(err, pkgerrors.CodeLLMRequestFailed) {
        // 检查是否是限流错误
        return true
    }
    
    // HTTP 5xx 错误可重试
    if pkgerrors.Is(err, pkgerrors.CodeLLMRequestFailed) {
        return true
    }
    
    return false
}
```

### 3. 超时控制中间件

在 `internal/llm/middleware.go` 中实现：

```go
package llm

import (
    "context"
    "time"

    "github.com/cloudwego/eino/components/model"
    "github.com/cloudwego/eino/schema"
)

// TimeoutConfig 超时配置
type TimeoutConfig struct {
    GenerateTimeout time.Duration // 同步生成超时时间
    StreamTimeout   time.Duration // 流式生成超时时间
}

// DefaultTimeoutConfig 默认超时配置
func DefaultTimeoutConfig() *TimeoutConfig {
    return &TimeoutConfig{
        GenerateTimeout: 60 * time.Second,
        StreamTimeout:   120 * time.Second,
    }
}

// TimeoutClient 支持超时控制的客户端
type TimeoutClient struct {
    *Client
    timeoutConfig *TimeoutConfig
}

// NewTimeoutClient 创建支持超时控制的客户端
func NewTimeoutClient(client *Client, timeoutConfig *TimeoutConfig) *TimeoutClient {
    if timeoutConfig == nil {
        timeoutConfig = DefaultTimeoutConfig()
    }
    return &TimeoutClient{
        Client:        client,
        timeoutConfig: timeoutConfig,
    }
}

// GenerateWithTimeout 带超时的同步生成
func (c *TimeoutClient) GenerateWithTimeout(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
    ctx, cancel := context.WithTimeout(ctx, c.timeoutConfig.GenerateTimeout)
    defer cancel()
    
    return c.Generate(ctx, input, opts...)
}

// StreamWithTimeout 带超时的流式生成
func (c *TimeoutClient) StreamWithTimeout(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
    ctx, cancel := context.WithTimeout(ctx, c.timeoutConfig.StreamTimeout)
    defer cancel()
    
    return c.Stream(ctx, input, opts...)
}
```

### 4. 模型提供商仓库层

首先实现模型提供商的CRUD操作：

```go
// internal/repository/model_provider_repository.go
package repository

import (
    "errors"
    "Qavor/internal/model/entity"
    "gorm.io/gorm"
)

type ModelProviderRepository interface {
    FindByID(id uint) (*entity.ModelProvider, error)
    FindByProviderID(providerID string) (*entity.ModelProvider, error)
    Create(provider *entity.ModelProvider) error
    Update(provider *entity.ModelProvider) error
    Delete(id uint) error
    List(offset, limit int, keyword string) ([]*entity.ModelProvider, int64, error)
    FindEnabledByCapability(capability string) ([]*entity.ModelProvider, error)
}

type modelProviderRepository struct {
    db *gorm.DB
}

func NewModelProviderRepository(db *gorm.DB) ModelProviderRepository {
    return &modelProviderRepository{db: db}
}

func (r *modelProviderRepository) FindByID(id uint) (*entity.ModelProvider, error) {
    var provider entity.ModelProvider
    err := r.db.First(&provider, id).Error
    if err != nil {
        if errors.Is(err, gorm.ErrRecordNotFound) {
            return nil, nil
        }
        return nil, err
    }
    return &provider, nil
}

func (r *modelProviderRepository) FindByProviderID(providerID string) (*entity.ModelProvider, error) {
    var provider entity.ModelProvider
    err := r.db.Where("provider_id = ?", providerID).First(&provider).Error
    if err !=nil {
        if errors.Is(err, gorm.ErrRecordNotFound) {
            return nil, nil
        }
        return nil, err
    }
    return &provider, nil
}

func (r *modelProviderRepository) Create(provider *entity.ModelProvider) error {
    return r.db.Create(provider).Error
}

func (r *modelProviderRepository) Update(provider *entity.ModelProvider) error {
    return r.db.Save(provider).Error
}

func (r *modelProviderRepository) Delete(id uint) error {
    return r.db.Delete(&entity.ModelProvider{}, id).Error
}

func (r *modelProviderRepository) List(offset, limit int, keyword string) ([]*entity.ModelProvider, int64, error) {
    var providers []*entity.ModelProvider
    var total int64

    query := r.db.Model(&entity.ModelProvider{})
    if keyword != "" {
        query = query.Where("display_name LIKE ? OR provider_id LIKE ?", 
            "%"+keyword+"%", "%"+keyword+"%")
    }

    if err := query.Count(&total).Error; err != nil {
        return nil, 0, err
    }

    err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&providers).Error
    if err != nil {
        return nil, 0, err
    }

    return providers, total, nil
}

func (r *modelProviderRepository) FindEnabledByCapability(capability string) ([]*entity.ModelProvider, error) {
    var providers []*entity.ModelProvider
    err := r.db.Where("is_enabled = ? AND JSON_CONTAINS(capabilities, ?)", 
        true, `"`+capability+`"`).Find(&providers).Error
    return providers, err
}
```

### 5. 模型提供商服务层

实现业务逻辑和模型工厂：

```go
// internal/service/model_provider_service.go
package service

import (
    "context"
    "os"
    "strings"
    "sync"
    "Qavor/internal/llm"
    "Qavor/internal/model/dto/request"
    dto "Qavor/internal/model/dto/response"
    "Qavor/internal/model/entity"
    "Qavor/internal/repository"
    "Qavor/pkg/errors"
)

type ModelProviderService interface {
    CreateProvider(req *request.CreateModelProviderRequest) (*dto.ModelProviderResponse, error)
    GetProvider(id uint) (*dto.ModelProviderResponse, error)
    UpdateProvider(id uint, req *request.UpdateModelProviderRequest) (*dto.ModelProviderResponse, error)
    DeleteProvider(id uint) error
    ListProviders(req *request.ModelProviderListRequest) (*dto.ModelProviderListResponse, error)
    GetLLMClient(ctx context.Context, providerID string, model string) (*llm.RetryableClient, error)
    GetLLMClientByCapability(ctx context.Context, capability string, model string) (*llm.RetryableClient, error)
}

type modelProviderService struct {
    providerRepo repository.ModelProviderRepository
    // 使用 sync.Map 解决并发读写问题
    // sync.Map 是并发安全的，不需要额外加锁
    clientCache sync.Map
}

func NewModelProviderService(providerRepo repository.ModelProviderRepository) ModelProviderService {
    return &modelProviderService{
        providerRepo: providerRepo,
        // sync.Map 零值可用，不需要初始化
    }
}

func (s *modelProviderService) CreateProvider(req *request.CreateModelProviderRequest) (*dto.ModelProviderResponse, error) {
    // 检查provider_id是否已存在
    exists, err := s.providerRepo.FindByProviderID(req.ProviderID)
    if err != nil {
        return nil, err
    }
    if exists != nil {
        return nil, errors.New(errors.CodeModelProviderAlreadyExists, "Provider ID已存在")
    }

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
        IsEnabled:               true,
    }

    if err := s.providerRepo.Create(provider); err != nil {
        return nil, err
    }

    return s.toResponse(provider), nil
}

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

    // 更新字段
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

    if err := s.providerRepo.Update(provider); err != nil {
        return nil, err
    }

    // 清除缓存：使用精确 Key 删除
    // 缓存key格式是 "providerID:model"
    // 遍历所有可能的 model 名称，精确删除对应的缓存
    s.invalidateProviderCache(oldProviderID)

    return s.toResponse(provider), nil
}

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
// 使用精确 Key 删除，避免 Range 遍历的性能问题
func (s *modelProviderService) invalidateProviderCache(providerID string) {
    // 方案1：如果已知所有启用的模型，可以直接删除精确的 key
    // 这里我们从数据库查询该提供商启用的模型列表
    provider, err := s.providerRepo.FindByProviderID(providerID)
    if err != nil || provider == nil {
        return
    }

    // 从 EnabledModels 中提取所有模型名称，精确删除缓存
    if models, ok := provider.EnabledModels.([]interface{}); ok {
        for _, model := range models {
            if modelName, ok := model.(string); ok {
                cacheKey := providerID + ":" + modelName
                s.clientCache.Delete(cacheKey)
            }
        }
    }
    
    // 方案2：如果无法获取模型列表，使用通配符删除（仅在必要时）
    // 注意：sync.Map 不支持通配符删除，这里保留 Range 作为备选
    // 实际生产中建议维护一个索引来跟踪缓存的 key
}

func (s *modelProviderService) ListProviders(req *request.ModelProviderListRequest) (*dto.ModelProviderListResponse, error) {
    page := req.Page
    if page < 1 {
        page = 1
    }
    pageSize := req.PageSize
    if pageSize < 1 {
        pageSize = 10
    }

    offset := (page - 1) * pageSize
    providers, total, err := s.providerRepo.List(offset, pageSize, req.Keyword)
    if err != nil {
        return nil, err
    }

    items := make([]dto.ModelProviderResponse, 0, len(providers))
    for _, provider := range providers {
        items = append(items, *s.toResponse(provider))
    }

    return &dto.ModelProviderListResponse{
        Total: total,
        Items: items,
    }, nil
}

// GetLLMClient 获取LLM客户端（并发安全）
// 使用 sync.Map + LoadOrStore 防止并发击穿和重复创建
func (s *modelProviderService) GetLLMClient(ctx context.Context, providerID string, model string) (*llm.RetryableClient, error) {
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
    
    if !provider.IsEnabled {
        return nil, errors.New(errors.CodeModelProviderDisabled, "模型提供商已禁用")
    }
    
    // 3. 解析API Key
    apiKey := provider.APIKey
    if provider.APIKeyEnv != "" && apiKey == "" {
        apiKey = os.Getenv(provider.APIKeyEnv)
    }
    if apiKey == "" {
        return nil, errors.New(errors.CodeModelProviderAPIKeyMissing, "API Key未配置")
    }
    
    // 4. 创建基础LLM客户端
    config := &llm.Config{
        Model:   model,
        APIKey:  apiKey,
        BaseURL: provider.BaseURL,
    }
    
    client, err := llm.NewClient(ctx, config)
    if err != nil {
        return nil, err
    }
    
    // 5. 包装为带重试的客户端
    retryableClient := llm.NewRetryableClient(client, nil)
    
    // 6. 使用 LoadOrStore 原子操作写入缓存
    // 如果多个goroutine同时创建，只有第一个会成功存储，其他的会使用已存储的
    actual, _ := s.clientCache.LoadOrStore(cacheKey, retryableClient)
    return actual.(*llm.RetryableClient), nil
}

func (s *modelProviderService) GetLLMClientByCapability(ctx context.Context, capability string, model string) (*llm.RetryableClient, error) {
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
        CreatedBy:               provider.CreatedBy,
        UpdatedBy:               provider.UpdatedBy,
        CreatedAt:               provider.CreatedAt,
        UpdatedAt:               provider.UpdatedAt,
    }
}

// getEnvVariable 获取环境变量
func getEnvVariable(key string) string {
    return os.Getenv(key)
}
```

### 6. 模型提供商控制器

> **注意**：此目录需要手动创建 `internal/api/v1/model_provider/`

实现REST API接口：

```go
// internal/api/v1/model_provider/controller.go
package model_provider

import (
    "net/http"
    "strconv"
    "Qavor/internal/model/dto/request"
    "Qavor/internal/service"
    "Qavor/pkg/errors"
    "Qavor/pkg/response"
    "github.com/gin-gonic/gin"
)

type Controller struct {
    providerService service.ModelProviderService
}

func NewController(providerService service.ModelProviderService) *Controller {
    return &Controller{providerService: providerService}
}

func (ctrl *Controller) RegisterRoutes(router *gin.RouterGroup) {
    providers := router.Group("/model-providers")
    {
        providers.POST("", ctrl.CreateProvider)
        providers.GET("", ctrl.ListProviders)
        providers.GET("/:id", ctrl.GetProvider)
        providers.PUT("/:id", ctrl.UpdateProvider)
        providers.DELETE("/:id", ctrl.DeleteProvider)
    }
}

func (ctrl *Controller) CreateProvider(c *gin.Context) {
    var req request.CreateModelProviderRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        response.Error(c, errors.New(errors.CodeInvalidParam, err.Error()))
        return
    }

    resp, err := ctrl.providerService.CreateProvider(&req)
    if err != nil {
        response.Error(c, err)
        return
    }

    response.Success(c, resp)
}

func (ctrl *Controller) GetProvider(c *gin.Context) {
    idStr := c.Param("id")
    id, err := strconv.ParseUint(idStr, 10, 32)
    if err != nil {
        response.Error(c, errors.New(errors.CodeInvalidParam, "无效的ID"))
        return
    }

    resp, err := ctrl.providerService.GetProvider(uint(id))
    if err != nil {
        response.Error(c, err)
        return
    }

    response.Success(c, resp)
}

func (ctrl *Controller) UpdateProvider(c *gin.Context) {
    idStr := c.Param("id")
    id, err := strconv.ParseUint(idStr, 10, 32)
    if err != nil {
        response.Error(c, errors.New(errors.CodeInvalidParam, "无效的ID"))
        return
    }

    var req request.UpdateModelProviderRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        response.Error(c, errors.New(errors.CodeInvalidParam, err.Error()))
        return
    }

    resp, err := ctrl.providerService.UpdateProvider(uint(id), &req)
    if err != nil {
        response.Error(c, err)
        return
    }

    response.Success(c, resp)
}

func (ctrl *Controller) DeleteProvider(c *gin.Context) {
    idStr := c.Param("id")
    id, err := strconv.ParseUint(idStr, 10, 32)
    if err != nil {
        response.Error(c, errors.New(errors.CodeInvalidParam, "无效的ID"))
        return
    }

    if err := ctrl.providerService.DeleteProvider(uint(id)); err != nil {
        response.Error(c, err)
        return
    }

    response.Success(c, nil)
}

func (ctrl *Controller) ListProviders(c *gin.Context) {
    var req request.ModelProviderListRequest
    if err := c.ShouldBindQuery(&req); err != nil {
        response.Error(c, errors.New(errors.CodeInvalidParam, err.Error()))
        return
    }

    resp, err := ctrl.providerService.ListProviders(&req)
    if err != nil {
        response.Error(c, err)
        return
    }

    response.Success(c, resp)
}
```

### 7. 路由注册

更新路由配置：

```go
// internal/api/router.go
package api

import (
    "Qavor/internal/api/v1/auth"
    "Qavor/internal/api/v1/model_provider"
    "Qavor/internal/api/v1/user"
    "Qavor/internal/middleware"
    "Qavor/internal/service"
    "github.com/gin-gonic/gin"
)

type Router struct {
    userCtrl     *user.Controller
    authCtrl     *auth.Controller
    providerCtrl *model_provider.Controller
}

func NewRouter(
    userService service.UserService,
    authService service.AuthService,
    providerService service.ModelProviderService,
) *Router {
    return &Router{
        userCtrl:     user.NewController(userService),
        authCtrl:     auth.NewController(authService, userService),
        providerCtrl: model_provider.NewController(providerService),
    }
}

func (r *Router) Setup(engine *gin.Engine) {
    engine.Use(middleware.Recovery())
    engine.Use(middleware.Logger())
    engine.Use(middleware.CORS())

    engine.GET("/api/v1/health", func(c *gin.Context) {
        c.JSON(200, gin.H{
            "status":  "ok",
            "message": "Qavor API is running",
        })
    })

    v1 := engine.Group("/api/v1")
    {
        r.authCtrl.RegisterRoutes(v1)
        r.userCtrl.RegisterRoutes(v1)
        r.providerCtrl.RegisterRoutes(v1) // 注册模型提供商路由
    }
}
```

### 8. 更新应用初始化

更新app.go以初始化模型提供商服务：

```go
// internal/app/app.go
func (a *App) initDependencies() {
    // 创建 Repository
    userRepo := repository.NewUserRepository(a.postgresDB)
    providerRepo := repository.NewModelProviderRepository(a.postgresDB)

    // 创建邮件客户端
    emailClient := email.NewSMTPClient(&a.cfg.Email)

    // 创建 Service
    userSvc := service.NewUserService(userRepo)
    authSvc := service.NewAuthService(userRepo, userSvc, emailClient)
    providerSvc := service.NewModelProviderService(providerRepo)

    // 创建 Router
    a.router = api.NewRouter(userSvc, authSvc, providerSvc)
}
```

## 使用示例

### 1. 创建模型提供商

```bash
curl -X POST http://localhost:8080/api/v1/model-providers \
  -H "Content-Type: application/json" \
  -d '{
    "provider_id": "openai",
    "display_name": "OpenAI",
    "provider_type": "openai",
    "base_url": "https://api.openai.com/v1",
    "api_key": "your-api-key",
    "capabilities": ["chat", "embedding"],
    "enabled_models": ["gpt-4", "gpt-3.5-turbo"]
  }'
```

### 2. 获取LLM客户端

```go
// 在业务代码中使用
providerSvc := service.NewModelProviderService(providerRepo)
client, err := providerSvc.GetLLMClient(ctx, "openai", "gpt-4")
if err != nil {
    log.Fatal(err)
}

// 使用客户端进行对话
messages := []*schema.Message{
    {
        Role: schema.System,
        Content: "你是一个有帮助的助手",
    },
    {
        Role: schema.User,
        Content: "你好，请介绍一下你自己",
    },
}

response, err := client.Generate(ctx, messages)
if err != nil {
    log.Fatal(err)
}

fmt.Println(response.Content)
```

### 3. 按能力获取客户端

```go
// 自动选择支持chat能力的提供商
client, err := providerSvc.GetLLMClientByCapability(ctx, "chat", "gpt-4")
```

## 扩展支持其他模型提供商

要支持其他模型提供商（如Claude、文心一言等），需要：

1. **扩展eino框架支持**：eino框架已经支持多种模型，可以通过配置不同的模型类型来支持。

2. **添加提供商类型映射**：

```go
// internal/llm/factory.go
package llm

import (
    "context"
    "Qavor/internal/model/entity"
)

type ModelFactory struct{}

func NewModelFactory() *ModelFactory {
    return &ModelFactory{}
}

func (f *ModelFactory) CreateClient(ctx context.Context, provider *entity.ModelProvider, model string) (*Client, error) {
    switch provider.ProviderType {
    case "openai":
        return f.createOpenAIClient(ctx, provider, model)
    case "claude":
        return f.createClaudeClient(ctx, provider, model)
    case "wenxin":
        return f.createWenxinClient(ctx, provider, model)
    default:
        return nil, errors.New(errors.CodeInvalidParam, "不支持的模型提供商类型")
    }
}

func (f *ModelFactory) createOpenAIClient(ctx context.Context, provider *entity.ModelProvider, model string) (*Client, error) {
    // OpenAI客户端创建逻辑
    config := &Config{
        Model:   model,
        APIKey:  provider.APIKey,
        BaseURL: provider.BaseURL,
    }
    return NewClient(ctx, config)
}
```

## 最佳实践

### 缓存策略（重要区分）

| 缓存类型 | 存储位置 | 用途 | 说明 |
|---------|---------|------|------|
| **Session 会话** | Redis | 存储用户会话、Token、临时数据 | 支持分布式，有TTL过期 |
| **LLM Client 对象** | 本地内存 (sync.Map) | 维护模型提供商客户端单例 | 避免重复创建，进程级缓存 |

> **为什么不用 Redis 缓存 Client 对象？**
> - Client 对象包含 HTTP 连接池、TCP 连接等有状态资源
> - 序列化/反序列化会破坏对象内部状态
> - 每个服务实例应维护自己的 Client 连接池
> - 使用 `sync.Map` 本地缓存 + `LoadOrStore` 原子操作防止并发击穿

### 其他最佳实践

1. **错误处理**：统一的错误码和错误处理机制
2. **日志记录**：记录模型调用的详细信息用于调试
3. **限流控制**：对模型API调用进行限流
4. **监控告警**：监控模型调用成功率和响应时间

## 下一步

1. 实现完整的模型提供商CRUD功能
2. 添加模型调用日志记录
3. 实现模型调用统计
4. 添加模型健康检查
5. 实现模型负载均衡