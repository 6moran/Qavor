# Task 7: 长期记忆简易框架设计文档

## 文档信息
- **项目**：Qavor Agent 对话系统
- **模块**：长期记忆简易框架（Long-term Memory）
- **依赖模块**：Memory Extractor（Task 8）
- **下游模块**：Context Builder（Task 4）
- **目标**：搭建长期记忆简易框架，支持用户级别的持久化记忆

## 实现阶段

| 阶段 | 内容 | 状态 |
|------|------|------|
| **V1.0** | 基础存储 + 文本匹配检索 | 当前阶段 |
| **V2.0** | 语义检索（RAG） | 后续迭代 |

## 职责边界

| 组件 | 职责 |
|------|------|
| **Short Memory (Task 6)** | 管理当前会话的上下文 |
| **Memory Extractor (Task 8)** | 判断和提取关键信息 |
| **Long Memory (Task 7)** | 存储用户记忆（本模块） |
| **Context Builder (Task 4)** | 检索用户记忆并注入上下文 |

---

# 1 概述

长期记忆负责跨会话的持久化记忆，是对话系统的"用户记忆存储"：

1. **用户画像**：存储用户的基本信息（姓名、职业、身份等）
2. **用户偏好**：存储用户的偏好设置（喜欢什么、不喜欢什么）
3. **历史摘要**：存储历史会话的关键摘要
4. **事实信息**：存储对话中提到的重要事实

## 1.1 说明

- **Memory Extractor 不属于本模块**：记忆提取由上游模块（Task 8）负责
- **本模块只负责存储**：接收上游归档的信息并持久化
- **检索由 Context Builder 负责**：通过检索接口获取相关记忆

## 1.2 用户记忆 vs 知识库

| 概念 | 说明 | 示例 |
|------|------|------|
| **用户记忆** | 关于用户自己的信息 | "我是程序员"、"喜欢Python" |
| **知识库** | 外部知识信息 | "Go语言语法"、"Docker教程" |

**本模块只负责用户记忆，不涉及知识库。**

```
┌──────────────────────────────────────────────────────────────┐
│                       长期记忆模块                             │
│                                                              │
│  ┌──────────────────────────────────────────────────────┐    │
│  │               LongTermMemory                          │    │
│  │  ├── 用户画像 (User Profile)                          │    │
│  │  ├── 用户偏好 (User Preference)                       │    │
│  │  ├── 历史摘要 (Conversation Summary)                  │    │
│  │  └── 事实信息 (Fact)                                  │    │
│  └──────────────────────────────────────────────────────┘    │
│         ↑                          ↓                         │
│  ┌──────────────┐          ┌──────────────────┐              │
│  │ Memory       │          │  存储层           │              │
│  │ Extractor    │          │  PostgreSQL       │              │
│  │ (Task 8)     │          │  Redis            │              │
│  └──────────────┘          └──────────────────┘              │
│                                                              │
│  ┌──────────────────────────────────────────────────────┐    │
│  │               检索接口 (Search)                        │    │
│  │  ├── V1.0: 文本匹配（简单关键词搜索）                  │    │
│  │  └── V2.0: 语义检索（基于向量的 RAG）                 │    │
│  └──────────────────────────────────────────────────────┘    │
└──────────────────────────────────────────────────────────────┘
```

---

# 2 目录结构

## 2.1 V1.0 目录结构（当前阶段）

```
internal/memory/
├── long_term/
│   ├── manager.go          // 长期记忆管理器（存储接口）
│   ├── searcher.go         // 记忆检索器（文本匹配）
│   ├── profile.go          // 用户画像管理
│   ├── preference.go       // 用户偏好管理
│   ├── summary.go          // 历史摘要管理
│   ├── entity.go           // 长期记忆实体定义
│   ├── repository.go       // 数据库仓储
│   └── store.go            // Redis 缓存
```

## 2.2 V2.0 目录结构（后续迭代）

```
internal/memory/
├── long_term/
│   ├── manager.go          // 长期记忆管理器
│   ├── searcher.go         // 记忆检索器（统一接口）
│   ├── profile.go          // 用户画像管理
│   ├── preference.go       // 用户偏好管理
│   ├── summary.go          // 历史摘要管理
│   ├── entity.go           // 长期记忆实体定义
│   ├── repository.go       // 数据库仓储
│   ├── store.go            // Redis 缓存
│   └── rag/                // 新增：RAG 语义检索
│       ├── interface.go    // RAG 接口定义
│       ├── vectorize.go    // 向量化接口
│       └── search.go       // 语义检索接口
```

---

# 3 实体定义 (entity.go)

```go
package longterm

import (
    "time"
)

// LongTermMemory 长期记忆主表
type LongTermMemory struct {
    ID            uint      `gorm:"primarykey" json:"id"`
    UserID        uint      `gorm:"not null;index;comment:用户ID" json:"user_id"`
    MemoryType    string    `gorm:"type:varchar(30);not null;index;comment:记忆类型" json:"memory_type"`
    Content       string    `gorm:"type:text;not null;comment:记忆内容" json:"content"`
    Summary       string    `gorm:"type:text;comment:摘要" json:"summary"`
    Importance    float64   `gorm:"default:0.5;comment:重要性评分(0-1)" json:"importance"`
    SourceConvID  *uint     `gorm:"comment:来源会话ID" json:"source_conv_id,omitempty"`
    Metadata      JSON      `gorm:"type:json;comment:附加元数据" json:"metadata,omitempty"`
    VectorID      string    `gorm:"type:varchar(128);index;comment:向量ID（V2.0语义检索）" json:"vector_id,omitempty"`
    CreatedAt     time.Time `json:"created_at"`
    UpdatedAt     time.Time `json:"updated_at"`
    ExpiresAt     *time.Time `gorm:"comment:过期时间（可选）" json:"expires_at,omitempty"`
}

// TableName 指定表名
func (LongTermMemory) TableName() string {
    return "long_term_memories"
}

// 记忆类型常量
const (
    MemoryTypeProfile    = "profile"    // 用户画像（姓名、职业等）
    MemoryTypePreference = "preference" // 用户偏好（喜欢什么）
    MemoryTypeSummary    = "summary"    // 会话摘要
    MemoryTypeFact       = "fact"       // 事实信息
)

// UserProfile 用户画像
type UserProfile struct {
    UserID      uint              `json:"user_id"`
    Name        string            `json:"name"`
    Occupation  string            `json:"occupation"`      // 职业
    Identity    string            `json:"identity"`        // 身份标签
    Preferences map[string]string `json:"preferences"`     // 偏好设置
    Interests   []string          `json:"interests"`       // 兴趣领域
    UpdatedAt   time.Time         `json:"updated_at"`
}
```

---

# 4 长期记忆管理器 (manager.go)

## 4.1 存储接口

```go
package longterm

import (
    "context"
)

// Manager 长期记忆管理器接口（只负责存储）
type Manager interface {
    // Save 保存长期记忆
    Save(ctx context.Context, memory *LongTermMemory) error

    // GetByID 根据 ID 获取记忆
    GetByID(ctx context.Context, id uint) (*LongTermMemory, error)

    // ListByUser 获取用户的所有记忆
    ListByUser(ctx context.Context, userID uint, memoryType string) ([]*LongTermMemory, error)

    // Update 更新记忆
    Update(ctx context.Context, memory *LongTermMemory) error

    // Delete 删除记忆
    Delete(ctx context.Context, id uint) error

    // GetProfile 获取用户画像
    GetProfile(ctx context.Context, userID uint) (*UserProfile, error)

    // SaveProfile 保存用户画像
    SaveProfile(ctx context.Context, profile *UserProfile) error

    // ArchiveFromSession 从会话归档记忆
    ArchiveFromSession(ctx context.Context, userID, conversationID uint, summary string, preferences []string) error
}
```

## 4.2 检索接口（V1.0）

```go
package longterm

import (
    "context"
)

// SearchResult 检索结果
type SearchResult struct {
    Memory    *LongTermMemory `json:"memory"`
    Score     float64         `json:"score"`      // 相似度分数（V1.0 固定为 1.0）
    MatchType string          `json:"match_type"` // 匹配类型：text
}

// MemorySearcher 记忆检索器接口（V1.0 文本匹配）
type MemorySearcher interface {
    // Search 搜索相关记忆
    // keyword: 搜索关键词
    // topK: 返回数量限制
    Search(ctx context.Context, userID uint, keyword string, topK int) ([]*SearchResult, error)
}
```

---

# 5 用户画像管理 (profile.go)

```go
package longterm

import (
    "context"
    "encoding/json"
    "fmt"
    "time"

    "github.com/redis/go-redis/v9"
    "go.uber.org/zap"
)

// ProfileManager 用户画像管理器
type ProfileManager struct {
    repo   LongTermRepository
    redis  *redis.Client
    logger *zap.Logger
}

// NewProfileManager 创建画像管理器
func NewProfileManager(repo LongTermRepository, redis *redis.Client, logger *zap.Logger) *ProfileManager {
    return &ProfileManager{
        repo:   repo,
        redis:  redis,
        logger: logger,
    }
}

// profileKey 生成 Redis key
func (m *ProfileManager) profileKey(userID uint) string {
    return fmt.Sprintf("memory:profile:%d", userID)
}

// GetProfile 获取用户画像
func (m *ProfileManager) GetProfile(ctx context.Context, userID uint) (*UserProfile, error) {
    // 1. 先从 Redis 缓存读取
    if m.redis != nil {
        data, err := m.redis.Get(ctx, m.profileKey(userID)).Bytes()
        if err == nil {
            var profile UserProfile
            if json.Unmarshal(data, &profile) == nil {
                return &profile, nil
            }
        }
    }

    // 2. 从数据库读取
    memories, err := m.repo.ListByUser(ctx, userID, MemoryTypeProfile)
    if err != nil {
        return nil, err
    }

    // 3. 从记忆中构建画像
    profile := m.buildProfile(userID, memories)

    // 4. 缓存到 Redis
    if m.redis != nil {
        data, _ := json.Marshal(profile)
        m.redis.Set(ctx, m.profileKey(userID), data, time.Hour)
    }

    return profile, nil
}

// buildProfile 从记忆构建用户画像
func (m *ProfileManager) buildProfile(userID uint, memories []*LongTermMemory) *UserProfile {
    profile := &UserProfile{
        UserID:      userID,
        Preferences: make(map[string]string),
        Interests:   make([]string, 0),
    }

    for _, mem := range memories {
        switch mem.Metadata["category"] {
        case "name":
            profile.Name = mem.Content
        case "occupation":
            profile.Occupation = mem.Content
        case "identity":
            profile.Identity = mem.Content
        case "preference":
            key := mem.Metadata["key"].(string)
            profile.Preferences[key] = mem.Content
        case "interest":
            profile.Interests = append(profile.Interests, mem.Content)
        }
    }

    profile.UpdatedAt = time.Now()
    return profile
}

// SaveProfile 保存用户画像
func (m *ProfileManager) SaveProfile(ctx context.Context, profile *UserProfile) error {
    // 缓存到 Redis
    if m.redis != nil {
        data, _ := json.Marshal(profile)
        m.redis.Set(ctx, m.profileKey(profile.UserID), data, time.Hour)
    }

    return nil
}
```

---

# 6 用户偏好管理 (preference.go)

```go
package longterm

import (
    "context"
)

// PreferenceManager 用户偏好管理器
type PreferenceManager struct {
    repo LongTermRepository
}

// NewPreferenceManager 创建偏好管理器
func NewPreferenceManager(repo LongTermRepository) *PreferenceManager {
    return &PreferenceManager{
        repo: repo,
    }
}

// SavePreference 保存用户偏好
func (m *PreferenceManager) SavePreference(ctx context.Context, userID uint, key, value string) error {
    memory := &LongTermMemory{
        UserID:     userID,
        MemoryType: MemoryTypePreference,
        Content:    value,
        Importance: 0.6,
        Metadata: JSON{
            "category": "preference",
            "key":      key,
        },
    }

    return m.repo.Create(ctx, memory)
}

// GetPreferences 获取用户所有偏好
func (m *PreferenceManager) GetPreferences(ctx context.Context, userID uint) (map[string]string, error) {
    memories, err := m.repo.ListByUser(ctx, userID, MemoryTypePreference)
    if err != nil {
        return nil, err
    }

    preferences := make(map[string]string)
    for _, mem := range memories {
        if key, ok := mem.Metadata["key"].(string); ok {
            preferences[key] = mem.Content
        }
    }

    return preferences, nil
}
```

---

# 7 历史摘要管理 (summary.go)

```go
package longterm

import (
    "context"
)

// SummaryManager 历史摘要管理器
type SummaryManager struct {
    repo LongTermRepository
}

// NewSummaryManager 创建摘要管理器
func NewSummaryManager(repo LongTermRepository) *SummaryManager {
    return &SummaryManager{
        repo: repo,
    }
}

// SaveSummary 保存会话摘要
func (m *SummaryManager) SaveSummary(ctx context.Context, userID, conversationID uint, summary string, importance float64) error {
    memory := &LongTermMemory{
        UserID:       userID,
        MemoryType:   MemoryTypeSummary,
        Content:      summary,
        Summary:      summary,
        Importance:   importance,
        SourceConvID: &conversationID,
        Metadata: JSON{
            "conversation_id": conversationID,
        },
    }

    return m.repo.Create(ctx, memory)
}

// GetRecentSummaries 获取用户最近的会话摘要
func (m *SummaryManager) GetRecentSummaries(ctx context.Context, userID uint, limit int) ([]*LongTermMemory, error) {
    return m.repo.ListByUserWithType(ctx, userID, MemoryTypeSummary, limit)
}
```

---

# 8 记忆检索器 (searcher.go) - V1.0

```go
package longterm

import (
    "context"

    "go.uber.org/zap"
)

// SearchConfig 检索配置
type SearchConfig struct {
    DefaultTopK int // 默认返回数量
}

// MemorySearcherImpl 记忆检索器实现（V1.0 文本匹配）
type MemorySearcherImpl struct {
    repo   LongTermRepository
    config *SearchConfig
    logger *zap.Logger
}

// NewMemorySearcher 创建记忆检索器
func NewMemorySearcher(
    repo LongTermRepository,
    config *SearchConfig,
    logger *zap.Logger,
) *MemorySearcherImpl {
    if config == nil {
        config = &SearchConfig{
            DefaultTopK: 5,
        }
    }

    return &MemorySearcherImpl{
        repo:   repo,
        config: config,
        logger: logger,
    }
}

// Search 搜索相关记忆（文本匹配）
func (s *MemorySearcherImpl) Search(ctx context.Context, userID uint, keyword string, topK int) ([]*SearchResult, error) {
    if topK <= 0 {
        topK = s.config.DefaultTopK
    }

    // 文本匹配搜索
    memories, err := s.repo.SearchByContent(ctx, userID, keyword)
    if err != nil {
        return nil, err
    }

    // 转换为搜索结果
    results := make([]*SearchResult, 0, len(memories))
    for _, mem := range memories {
        results = append(results, &SearchResult{
            Memory:    mem,
            Score:     1.0, // 文本匹配默认分数
            MatchType: "text",
        })
    }

    // 限制返回数量
    if len(results) > topK {
        results = results[:topK]
    }

    return results, nil
}
```

---

# 9 数据库仓储 (repository.go)

```go
package longterm

import (
    "context"

    "gorm.io/gorm"
)

// LongTermRepository 长期记忆仓储接口
type LongTermRepository interface {
    Create(ctx context.Context, memory *LongTermMemory) error
    FindByID(ctx context.Context, id uint) (*LongTermMemory, error)
    ListByUser(ctx context.Context, userID uint, memoryType string) ([]*LongTermMemory, error)
    ListByUserWithType(ctx context.Context, userID uint, memoryType string, limit int) ([]*LongTermMemory, error)
    SearchByContent(ctx context.Context, userID uint, keyword string) ([]*LongTermMemory, error)
    Update(ctx context.Context, memory *LongTermMemory) error
    Delete(ctx context.Context, id uint) error
}

// longTermRepository 长期记忆仓储实现
type longTermRepository struct {
    db *gorm.DB
}

// NewLongTermRepository 创建长期记忆仓储
func NewLongTermRepository(db *gorm.DB) LongTermRepository {
    return &longTermRepository{db: db}
}

// Create 创建长期记忆
func (r *longTermRepository) Create(ctx context.Context, memory *LongTermMemory) error {
    return r.db.WithContext(ctx).Create(memory).Error
}

// FindByID 根据 ID 查找记忆
func (r *longTermRepository) FindByID(ctx context.Context, id uint) (*LongTermMemory, error) {
    var memory LongTermMemory
    err := r.db.WithContext(ctx).First(&memory, id).Error
    if err != nil {
        return nil, err
    }
    return &memory, nil
}

// ListByUser 获取用户的所有记忆
func (r *longTermRepository) ListByUser(ctx context.Context, userID uint, memoryType string) ([]*LongTermMemory, error) {
    var memories []*LongTermMemory
    query := r.db.WithContext(ctx).Where("user_id = ?", userID)
    if memoryType != "" {
        query = query.Where("memory_type = ?", memoryType)
    }
    err := query.Order("importance DESC, created_at DESC").Find(&memories).Error
    return memories, err
}

// ListByUserWithType 获取用户指定类型的记忆（限制数量）
func (r *longTermRepository) ListByUserWithType(ctx context.Context, userID uint, memoryType string, limit int) ([]*LongTermMemory, error) {
    var memories []*LongTermMemory
    err := r.db.WithContext(ctx).
        Where("user_id = ? AND memory_type = ?", userID, memoryType).
        Order("importance DESC, created_at DESC").
        Limit(limit).
        Find(&memories).Error
    return memories, err
}

// SearchByContent 按内容搜索记忆（简单文本匹配）
func (r *longTermRepository) SearchByContent(ctx context.Context, userID uint, keyword string) ([]*LongTermMemory, error) {
    var memories []*LongTermMemory
    err := r.db.WithContext(ctx).
        Where("user_id = ? AND content LIKE ?", userID, "%"+keyword+"%").
        Order("importance DESC").
        Find(&memories).Error
    return memories, err
}

// Update 更新记忆
func (r *longTermRepository) Update(ctx context.Context, memory *LongTermMemory) error {
    return r.db.WithContext(ctx).Save(memory).Error
}

// Delete 删除记忆
func (r *longTermRepository) Delete(ctx context.Context, id uint) error {
    return r.db.WithContext(ctx).Delete(&LongTermMemory{}, id).Error
}
```

---

# 10 端到端流程

## 10.1 长期记忆触发方式

**不要使用"会话结束时归档"**，因为实际产品中会话可能长期存在。

改为根据**触发条件**执行 Memory Extractor：

| 触发条件 | 说明 |
|---------|------|
| **消息数量阈值** | 当消息数量达到阈值时触发 |
| **Token 阈值** | 当 Token 数达到阈值时触发 |
| **用户主动归档** | 用户主动点击"归档"按钮 |
| **后台定时任务** | 定期扫描并提取 |

## 10.2 Memory Extractor 触发流程

```
触发条件满足
    │
    ├── 1. 调用 Memory Extractor
    │   └── extractor.ExtractFromConversation(ctx, userID, convID, messages)
    │
    ├── 2. 提取关键信息
    │   └── 返回 []ExtractedMemory（用户画像、偏好、摘要）
    │
    └── 3. 保存到 Long Memory
        └── longTermMgr.Save(ctx, memory)
```

## 10.3 场景1：消息数量阈值触发

```
每次消息交互
    │
    ├── 检查消息数量
    │   if messageCount >= threshold {
    │       triggerMemoryExtractor()
    │   }
    │
    └── Memory Extractor 提取 → Long Memory 存储
```

## 10.4 场景2：对话时检索相关记忆

```
用户发送消息："我之前说过什么关于Python的事情？"
    │
    ├── 1. 调用记忆检索器（V1.0 文本匹配）
    │   └── searcher.Search(userID, "Python", topK=5)
    │   └── 返回相关记忆片段
    │
    ├── 2. 注入上下文
    │   └── 将检索结果注入 System Prompt
    │   └── "[用户记忆] 用户是程序员，喜欢Python，之前讨论过Python异步编程..."
    │
    └── 3. 调用 LLM
        └── 基于增强后的上下文生成回复
```

---

# 11 配置示例

## 11.1 V1.0 配置

```yaml
long_term_memory:
  enabled: true
  redis_ttl: 86400              # Redis 缓存过期时间（秒），默认24小时
  
  # Memory Extractor 触发配置
  extractor_trigger:
    message_threshold: 20       # 消息数量阈值
    token_threshold: 8000       # Token 阈值
    enable_timer: true          # 是否启用定时任务
    timer_interval: "0 */6 * * *"  # 每6小时执行一次
  
  # 检索配置（V1.0 文本匹配）
  search:
    default_top_k: 5            # 默认返回数量
```

## 11.2 V2.0 配置（后续迭代）

```yaml
long_term_memory:
  enabled: true
  redis_ttl: 86400
  
  extractor_trigger:
    message_threshold: 20
    token_threshold: 8000
    enable_timer: true
    timer_interval: "0 */6 * * *"
  
  # 检索配置（V2.0 语义检索）
  search:
    default_top_k: 5
    enable_semantic_search: true  # 启用语义检索
  
  # RAG 配置
  rag:
    enabled: true
    embedding_model: "text-embedding-3-small"
    vector_db: "pgvector"
    vector_dimension: 1536
```

---

# 12 与其他模块的关系

| 模块 | 关系 | 说明 |
|------|------|------|
| **Memory Extractor (Task 8)** | 数据源 | 接收提取的记忆并存储 |
| **Context Builder (Task 4)** | 读取 | 调用检索接口获取相关记忆 |
| **会话 CRUD** | 依赖 | 获取会话信息 |
| **消息 CRUD** | 依赖 | 获取消息内容 |

## 12.1 职责边界

| 职责 | 负责模块 |
|------|---------|
| **记忆提取** | Task 8 Memory Extractor |
| **记忆存储** | Task 7 长期记忆（本模块） |
| **记忆检索** | Task 7 MemorySearcher（本模块提供接口） |
| **记忆注入上下文** | Task 4 Context Builder |

## 12.2 数据流关系

```
Short Memory (Task 6) ──── 数据 ────▶ Memory Extractor (Task 8)
                                            │
                                            │ 提取
                                            ▼
                                      Long Memory (Task 7)
                                            │
                                            │ 存储
                                            ▼
                                      PostgreSQL + Redis
                                            │
                                            │ 检索
                                            ▼
                                      MemorySearcher
                                            │
                                            │ 注入
                                            ▼
                                      Context Builder (Task 4)
```

---

# 13 后续迭代（V2.0）

## 13.1 V2.0 新增内容

1. **RAG 语义检索模块**：接入向量化和语义搜索
2. **向量数据库集成**：接入 pgvector / Milvus / Qdrant
3. **统一检索接口**：支持文本匹配和语义检索切换

## 13.2 V2.0 目录结构

```
internal/memory/long_term/rag/
├── interface.go    // RAG 接口定义
├── vectorize.go    // 向量化接口
└── search.go       // 语义检索接口
```

## 13.3 V2.0 接口定义

```go
package rag

import (
    "context"
)

// RAGEngine RAG 引擎接口
type RAGEngine interface {
    // Vectorize 向量化文本
    Vectorize(ctx context.Context, texts []string) ([][]float64, error)

    // Search 语义搜索
    Search(ctx context.Context, query []float64, topK int) ([]SearchResult, error)

    // Index 建立索引
    Index(ctx context.Context, id string, vector []float64, metadata map[string]interface{}) error

    // Delete 删除索引
    Delete(ctx context.Context, id string) error
}

// SearchResult 搜索结果
type SearchResult struct {
    ID       string                 `json:"id"`
    Score    float64                `json:"score"`
    Content  string                 `json:"content"`
    Metadata map[string]interface{} `json:"metadata"`
}
```

## 13.4 V2.0 检索器升级

```go
// MemorySearcherImpl V2.0 记忆检索器
type MemorySearcherImpl struct {
    repo        LongTermRepository
    ragSearcher RAGSearcher    // V2.0 新增
    config      *SearchConfig
    logger      *zap.Logger
}

// V2.0 新增：语义检索
func (s *MemorySearcherImpl) SemanticSearch(ctx context.Context, userID uint, query string, topK int) ([]*SearchResult, error) {
    if s.ragSearcher == nil {
        return s.TextSearch(ctx, userID, query, topK)
    }
    return s.ragSearcher.Search(ctx, userID, query, topK)
}
```

---

# 14 四个模块的连续性总结

```
┌──────────────────────────────────────────────────────────────┐
│                     模块连续性关系                              │
│                                                              │
│  Task 4: 上下文管理                                           │
│  ├── 输入: MessageRepository (数据库历史消息)                  │
│  ├── 处理: Token 裁剪 + Prompt 组装                          │
│  ├── 读取: MemorySearcher (用户记忆检索)                      │
│  └── 输出: []*schema.Message → LLM                          │
│                     ↓                                        │
│  Task 5: SSE 流式服务                                         │
│  ├── 输入: ContextManager 构建的上下文                        │
│  ├── 处理: 调用 LLM.Stream()，推送事件                       │
│  └── 输出: SSE 事件流 → 前端                                 │
│                     ↓                                        │
│  Task 6: 短期记忆                                             │
│  ├── 输入: 每次消息交互                                      │
│  ├── 处理: 消息缓冲 + 摘要生成 + 状态追踪                     │
│  └── 输出: 摘要 + 最近消息 → ContextManager                  │
│                     ↓                                        │
│  Task 7: 长期记忆                                             │
│  ├── 输入: Memory Extractor 提取的关键信息                    │
│  ├── 处理: 持久化存储用户画像、偏好、摘要                      │
│  ├── 存储: PostgreSQL + Redis                                │
│  └── 输出: 用户画像 + 偏好 + 历史摘要                         │
│         ↓                                                    │
│  MemorySearcher → Context Builder → LLM                     │
│                                                              │
│  V1.0: 文本匹配检索                                          │
│  V2.0: 语义检索（RAG）                                       │
└──────────────────────────────────────────────────────────────┘
```