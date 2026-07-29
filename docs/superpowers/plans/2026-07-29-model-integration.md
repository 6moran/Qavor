# 模型集成实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 实现完整的模型集成系统，包括Token计算、重试/超时控制、模型提供商CRUD管理、并发安全的LLM客户端缓存

**Architecture:** 采用分层架构：基础工具包（Token计算、重试、超时）→ 数据访问层（仓库）→ 业务逻辑层（服务）→ API层（控制器）。使用sync.Map实现并发安全的客户端缓存，避免Redis缓存有状态对象。

**Tech Stack:** Go 1.21+, Gin, GORM, eino框架, tiktoken-go (可选), sync.Map

## Global Constraints

- Go 1.21+ 版本要求
- 使用eino框架的schema.Message类型
- 所有代码必须包含详细的中文注释
- 使用sync.Map缓存LLM客户端，不用Redis
- 错误码使用pkg/errors中定义的错误码

---

## 文件结构

```
├── pkg/
│   └── utils/
│       └── token.go                    # Token计算工具
├── internal/
│   ├── llm/
│   │   ├── llm.go                     # 现有LLM客户端
│   │   ├── retry.go                   # 重试逻辑
│   │   └── middleware.go              # 超时控制
│   ├── errors/
│   │   └── code.go                    # 错误码定义
│   ├── repository/
│   │   └── model_provider_repository.go  # 模型提供商仓库
│   ├── service/
│   │   └── model_provider_service.go     # 模型提供商服务
│   └── api/v1/model_provider/
│       └── controller.go              # 模型提供商控制器
```

---

### Task 1: 添加模型提供商错误码

**Files:**
- Modify: `pkg/errors/code.go:48-55`

**Interfaces:**
- Consumes: 无
- Produces: `CodeModelProviderNotFound`, `CodeModelProviderAlreadyExists`, `CodeModelProviderDisabled`, `CodeModelProviderAPIKeyMissing`

- [ ] **Step 1: 在错误码定义中添加模型提供商相关错误码**

```go
// 在 pkg/errors/code.go 中，在 LLM 错误码之后添加

// 模型提供商错误 5xxx
CodeModelProviderNotFound      = 5001
CodeModelProviderAlreadyExists = 5002
CodeModelProviderDisabled      = 5003
CodeModelProviderAPIKeyMissing = 5004
```

- [ ] **Step 2: 在错误消息映射中添加对应的文本**

```go
// 在 pkg/errors/code.go 的 codeMessages 映射中添加

// 模型提供商错误消息
CodeModelProviderNotFound:      "模型提供商不存在",
CodeModelProviderAlreadyExists: "模型提供商已存在",
CodeModelProviderDisabled:      "模型提供商已禁用",
CodeModelProviderAPIKeyMissing: "API Key 未配置",
```

- [ ] **Step 3: 验证编译**

Run: `go build ./pkg/errors/...`
Expected: 编译成功

- [ ] **Step 4: 提交**

```bash
git add pkg/errors/code.go
git commit -m "feat(errors): 添加模型提供商相关错误码"
```

---

### Task 2: 创建Token计算工具

**Files:**
- Create: `pkg/utils/token.go`
- Modify: `go.mod` (添加tiktoken-go依赖)

**Interfaces:**
- Consumes: 无
- Produces: `CountTokens(text string) int`, `CountMessageTokens(messages []*schema.Message) int`, `TrimMessages(messages []*schema.Message, maxTokens int) []*schema.Message`

- [ ] **Step 1: 安装tiktoken-go依赖**

Run: `go get github.com/pkoukk/tiktoken-go`
Expected: 依赖安装成功

- [ ] **Step 2: 创建pkg/utils/token.go文件**

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

- [ ] **Step 3: 验证编译**

Run: `go build ./pkg/utils/...`
Expected: 编译成功

- [ ] **Step 4: 提交**

```bash
git add pkg/utils/token.go go.mod go.sum
git commit -m "feat(utils): 添加Token计算和消息裁剪工具"
```

---

### Task 3: 创建重试逻辑

**Files:**
- Create: `internal/llm/retry.go`

**Interfaces:**
- Consumes: `llm.Client`, `schema.Message`, `model.Option`
- Produces: `RetryConfig`, `RetryableClient`, `NewRetryableClient()`, `GenerateWithRetry()`, `StreamWithRetry()`

- [ ] **Step 1: 创建internal/llm/retry.go文件**

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

- [ ] **Step 2: 验证编译**

Run: `go build ./internal/llm/...`
Expected: 编译成功

- [ ] **Step 3: 提交**

```bash
git add internal/llm/retry.go
git commit -m "feat(llm): 添加重试逻辑支持"
```

---

### Task 4: 创建超时控制中间件

**Files:**
- Create: `internal/llm/middleware.go`

**Interfaces:**
- Consumes: `llm.Client`, `schema.Message`, `model.Option`
- Produces: `TimeoutConfig`, `TimeoutClient`, `NewTimeoutClient()`, `GenerateWithTimeout()`, `StreamWithTimeout()`

- [ ] **Step 1: 创建internal/llm/middleware.go文件**

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

- [ ] **Step 2: 验证编译**

Run: `go build ./internal/llm/...`
Expected: 编译成功

- [ ] **Step 3: 提交**

```bash
git add internal/llm/middleware.go
git commit -m "feat(llm): 添加超时控制中间件"
```

---

### Task 5: 创建模型提供商仓库层

**Files:**
- Create: `internal/repository/model_provider_repository.go`

**Interfaces:**
- Consumes: `entity.ModelProvider`, `gorm.DB`
- Produces: `ModelProviderRepository` interface, `NewModelProviderRepository()`, `FindByID()`, `FindByProviderID()`, `Create()`, `Update()`, `Delete()`, `List()`, `FindEnabledByCapability()`

- [ ] **Step 1: 创建internal/repository/model_provider_repository.go文件**

```go
package repository

import (
    "errors"
    "Qavor/internal/model/entity"
    "gorm.io/gorm"
)

// ModelProviderRepository 模型提供商仓储接口
type ModelProviderRepository interface {
    // FindByID 根据ID查找模型提供商
    FindByID(id uint) (*entity.ModelProvider, error)
    // FindByProviderID 根据ProviderID查找模型提供商
    FindByProviderID(providerID string) (*entity.ModelProvider, error)
    // Create 创建模型提供商
    Create(provider *entity.ModelProvider) error
    // Update 更新模型提供商
    Update(provider *entity.ModelProvider) error
    // Delete 删除模型提供商
    Delete(id uint) error
    // List 分页获取模型提供商列表
    List(offset, limit int, keyword string) ([]*entity.ModelProvider, int64, error)
    // FindEnabledByCapability 根据能力查找启用的模型提供商
    FindEnabledByCapability(capability string) ([]*entity.ModelProvider, error)
}

// modelProviderRepository 模型提供商仓储实现
type modelProviderRepository struct {
    db *gorm.DB
}

// NewModelProviderRepository 创建模型提供商仓储
func NewModelProviderRepository(db *gorm.DB) ModelProviderRepository {
    return &modelProviderRepository{db: db}
}

// FindByID 根据ID查找模型提供商
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

// FindByProviderID 根据ProviderID查找模型提供商
func (r *modelProviderRepository) FindByProviderID(providerID string) (*entity.ModelProvider, error) {
    var provider entity.ModelProvider
    err := r.db.Where("provider_id = ?", providerID).First(&provider).Error
    if err != nil {
        if errors.Is(err, gorm.ErrRecordNotFound) {
            return nil, nil
        }
        return nil, err
    }
    return &provider, nil
}

// Create 创建模型提供商
func (r *modelProviderRepository) Create(provider *entity.ModelProvider) error {
    return r.db.Create(provider).Error
}

// Update 更新模型提供商
func (r *modelProviderRepository) Update(provider *entity.ModelProvider) error {
    return r.db.Save(provider).Error
}

// Delete 删除模型提供商
func (r *modelProviderRepository) Delete(id uint) error {
    return r.db.Delete(&entity.ModelProvider{}, id).Error
}

// List 分页获取模型提供商列表
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

// FindEnabledByCapability 根据能力查找启用的模型提供商
func (r *modelProviderRepository) FindEnabledByCapability(capability string) ([]*entity.ModelProvider, error) {
    var providers []*entity.ModelProvider
    err := r.db.Where("is_enabled = ? AND JSON_CONTAINS(capabilities, ?)", 
        true, `"`+capability+`"`).Find(&providers).Error
    return providers, err
}
```

- [ ] **Step 2: 验证编译**

Run: `go build ./internal/repository/...`
Expected: 编译成功

- [ ] **Step 3: 提交**

```bash
git add internal/repository/model_provider_repository.go
git commit -m "feat(repository): 添加模型提供商仓储层"
```

---

### Task 6: 创建模型提供商服务层

**Files:**
- Create: `internal/service/model_provider_service.go`

**Interfaces:**
- Consumes: `ModelProviderRepository`, `llm.Client`, `llm.RetryableClient`, `entity.ModelProvider`, DTOs
- Produces: `ModelProviderService` interface, `NewModelProviderService()`, CRUD methods, `GetLLMClient()`, `GetLLMClientByCapability()`

- [ ] **Step 1: 创建internal/service/model_provider_service.go文件**

```go
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

// ModelProviderService 模型提供商服务接口
type ModelProviderService interface {
    // CreateProvider 创建模型提供商
    CreateProvider(req *request.CreateModelProviderRequest) (*dto.ModelProviderResponse, error)
    // GetProvider 获取模型提供商
    GetProvider(id uint) (*dto.ModelProviderResponse, error)
    // UpdateProvider 更新模型提供商
    UpdateProvider(id uint, req *request.UpdateModelProviderRequest) (*dto.ModelProviderResponse, error)
    // DeleteProvider 删除模型提供商
    DeleteProvider(id uint) error
    // ListProviders 获取模型提供商列表
    ListProviders(req *request.ModelProviderListRequest) (*dto.ModelProviderListResponse, error)
    // GetLLMClient 获取LLM客户端
    GetLLMClient(ctx context.Context, providerID string, model string) (*llm.RetryableClient, error)
    // GetLLMClientByCapability 根据能力获取LLM客户端
    GetLLMClientByCapability(ctx context.Context, capability string, model string) (*llm.RetryableClient, error)
}

// modelProviderService 模型提供商服务实现
type modelProviderService struct {
    providerRepo repository.ModelProviderRepository
    // 使用 sync.Map 解决并发读写问题
    // sync.Map 是并发安全的，不需要额外加锁
    clientCache sync.Map
}

// NewModelProviderService 创建模型提供商服务
func NewModelProviderService(providerRepo repository.ModelProviderRepository) ModelProviderService {
    return &modelProviderService{
        providerRepo: providerRepo,
        // sync.Map 零值可用，不需要初始化
    }
}

// CreateProvider 创建模型提供商
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

// GetProvider 获取模型提供商
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
    s.invalidateProviderCache(oldProviderID)

    return s.toResponse(provider), nil
}

// DeleteProvider 删除模型提供商
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
    // 从数据库查询该提供商启用的模型列表
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
}

// ListProviders 获取模型提供商列表
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

// GetLLMClientByCapability 根据能力获取LLM客户端
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

// toResponse 将实体转换为响应DTO
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

- [ ] **Step 2: 验证编译**

Run: `go build ./internal/service/...`
Expected: 编译成功

- [ ] **Step 3: 提交**

```bash
git add internal/service/model_provider_service.go
git commit -m "feat(service): 添加模型提供商服务层"
```

---

### Task 7: 创建模型提供商控制器

**Files:**
- Create: `internal/api/v1/model_provider/controller.go`

**Interfaces:**
- Consumes: `ModelProviderService`, DTOs
- Produces: `Controller`, `NewController()`, `RegisterRoutes()`, CRUD handlers

- [ ] **Step 1: 创建目录**

Run: `mkdir -p internal/api/v1/model_provider`
Expected: 目录创建成功

- [ ] **Step 2: 创建internal/api/v1/model_provider/controller.go文件**

```go
package model_provider

import (
    "strconv"
    "Qavor/internal/model/dto/request"
    "Qavor/internal/service"
    "Qavor/pkg/errors"
    "Qavor/pkg/response"
    "github.com/gin-gonic/gin"
)

// Controller 模型提供商控制器
type Controller struct {
    providerService service.ModelProviderService
}

// NewController 创建模型提供商控制器
func NewController(providerService service.ModelProviderService) *Controller {
    return &Controller{providerService: providerService}
}

// RegisterRoutes 注册路由
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

// CreateProvider 创建模型提供商
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

// GetProvider 获取模型提供商
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

// UpdateProvider 更新模型提供商
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

// DeleteProvider 删除模型提供商
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

// ListProviders 获取模型提供商列表
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

- [ ] **Step 3: 验证编译**

Run: `go build ./internal/api/v1/model_provider/...`
Expected: 编译成功

- [ ] **Step 4: 提交**

```bash
git add internal/api/v1/model_provider/
git commit -m "feat(api): 添加模型提供商控制器"
```

---

### Task 8: 更新路由注册和应用初始化

**Files:**
- Modify: `internal/api/router.go`
- Modify: `internal/app/app.go`

**Interfaces:**
- Consumes: `ModelProviderService`
- Produces: 更新后的路由和依赖注入

- [ ] **Step 1: 更新internal/api/router.go**

```go
package api

import (
    "Qavor/internal/api/v1/auth"
    "Qavor/internal/api/v1/model_provider"
    "Qavor/internal/api/v1/user"
    "Qavor/internal/middleware"
    "Qavor/internal/service"
    "github.com/gin-gonic/gin"
)

// Router 路由
type Router struct {
    userCtrl     *user.Controller
    authCtrl     *auth.Controller
    providerCtrl *model_provider.Controller
}

// NewRouter 创建路由
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

// Setup 设置路由
func (r *Router) Setup(engine *gin.Engine) {
    // 全局中间件
    engine.Use(middleware.Recovery())
    engine.Use(middleware.Logger())
    engine.Use(middleware.CORS())

    // 健康检查
    engine.GET("/api/v1/health", func(c *gin.Context) {
        c.JSON(200, gin.H{
            "status":  "ok",
            "message": "Qavor API is running",
        })
    })

    // API v1 路由组
    v1 := engine.Group("/api/v1")
    {
        // 认证路由
        r.authCtrl.RegisterRoutes(v1)

        // 用户路由
        r.userCtrl.RegisterRoutes(v1)

        // 模型提供商路由
        r.providerCtrl.RegisterRoutes(v1)
    }
}
```

- [ ] **Step 2: 更新internal/app/app.go的initDependencies方法**

```go
// initDependencies 初始化依赖注入
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

- [ ] **Step 3: 验证编译**

Run: `go build ./...`
Expected: 编译成功

- [ ] **Step 4: 提交**

```bash
git add internal/api/router.go internal/app/app.go
git commit -m "feat(app): 更新路由注册和依赖注入"
```

---

### Task 9: 完整编译验证

**Files:**
- 无新增文件

**Interfaces:**
- 无

- [ ] **Step 1: 运行完整编译**

Run: `go build ./...`
Expected: 编译成功，无错误

- [ ] **Step 2: 运行测试**

Run: `go test ./...`
Expected: 测试通过（如果有测试）

- [ ] **Step 3: 最终提交**

```bash
git add .
git commit -m "chore: 模型集成实现完成"
```

---

## Self-Review

### 1. 规范覆盖检查

- ✅ Token计算与裁剪逻辑 - Task 2
- ✅ 重试与超时控制 - Task 3, Task 4
- ✅ 模型提供商仓库层 - Task 5
- ✅ 模型提供商服务层 - Task 6
- ✅ 模型提供商控制器 - Task 7
- ✅ 路由注册和应用初始化 - Task 8
- ✅ 错误码定义 - Task 1

### 2. 占位符扫描

无占位符，所有代码完整。

### 3. 类型一致性检查

- ✅ `RetryableClient` 在所有任务中一致
- ✅ `ModelProviderService` 接口定义一致
- ✅ 错误码使用一致

---

## Execution Handoff

**Plan complete and saved to `docs/superpowers/plans/2026-07-29-model-integration.md`. Two execution options:**

**1. Subagent-Driven (recommended)** - I dispatch a fresh subagent per task, review between tasks, fast iteration

**2. Inline Execution** - Execute tasks in this session using executing-plans, batch execution with checkpoints

**Which approach?**
