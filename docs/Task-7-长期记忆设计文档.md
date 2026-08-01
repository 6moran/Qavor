# Task 7: 长期记忆简易框架设计文档

## 文档信息
- **项目**：Qavor Agent 对话系统
- **模块**：长期记忆简易框架（Long-term Memory）
- **依赖模块**：短期记忆（Task 6）、Embedding 模型层
- **下游模块**：RAG 检索（预留接口）
- **目标**：搭建长期记忆简易框架，支持用户级别的持久化记忆，预留接口后续对接 RAG 检索

---

# 1 概述

长期记忆负责跨会话的持久化记忆，是对话系统的"长期存储"：

1. **用户画像**：存储用户的基本信息、偏好、习惯
2. **历史摘要**：存储历史会话的关键摘要
3. **知识片段**：存储对话中提取的重要知识点
4. **RAG 预留**：为后续接入向量检索提供接口

```
┌──────────────────────────────────────────────────────────────┐
│                       长期记忆模块                             │
│                                                              │
│  ┌──────────────────────────────────────────────────────┐    │
│  │               LongTermMemory                          │    │
│  │  ├── 用户画像 (User Profile)                          │    │
│  │  ├── 历史摘要 (Conversation Summary)                  │    │
│  │  ├── 知识片段 (Knowledge Snippet)                     │    │
│  │  └── 记忆索引 (Memory Index)                          │    │
│  └──────────────────────────────────────────────────────┘    │
│         ↑                          ↓                         │
│  ┌──────────────┐          ┌──────────────────┐              │
│  │ Short-term   │          │  Embedding       │              │
│  │ Memory       │          │  Model Layer     │              │
│  │ (Task 6)     │          │  (向量化)         │              │
│  └──────────────┘          └──────────────────┘              │
│                                                              │
│  ┌──────────────────────────────────────────────────────┐    │
│  │               存储层 (Storage)                         │    │
│  │  ├── PostgreSQL: 结构化存储（用户画像、摘要）           │    │
│  │  ├── 向量数据库: 语义检索（预留接口）                   │    │
│  │  └── Redis: 热数据缓存（高频访问记忆）                 │    │
│  └──────────────────────────────────────────────────────┘    │
│                                                              │
│  ┌──────────────────────────────────────────────────────┐    │
│  │               RAG 预留接口                             │    │
│  │  ├── 向量化接口 (Vectorize)                           │    │
│  │  ├── 检索接口 (Search)                                │    │
│  │  └── 重排序接口 (Rerank)                              │    │
│  └──────────────────────────────────────────────────────┘    │
└──────────────────────────────────────────────────────────────┘
```

---

# 2 目录结构

```
internal/memory/
├── long_term/
│   ├── memory.go           // 长期记忆接口
│   ├── profile.go          // 用户画像管理
│   ├── summary.go          // 历史摘要管理
│   ├── knowledge.go        // 知识片段管理
│   ├── entity.go           // 长期记忆实体定义
│   ├── repository.go       // 数据库仓储
│   ├── store.go            // Redis 缓存
│   └── rag/
│       ├── interface.go    // RAG 接口定义（预留）
│       ├── vectorize.go    // 向量化接口
│       └── search.go       // 检索接口
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
    VectorID      string    `gorm:"type:varchar(128);index;comment:向量ID（用于RAG检索）" json:"vector_id,omitempty"`
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
    MemoryTypeProfile    = "profile"    // 用户画像
    MemoryTypeSummary    = "summary"    // 会话摘要
    MemoryTypeKnowledge  = "knowledge"  // 知识片段
    MemoryTypePreference = "preference" // 用户偏好
    MemoryTypeFact       = "fact"       // 事实信息
)

// UserProfile 用户画像（从记忆中提取）
type UserProfile struct {
    UserID        uint              `json:"user_id"`
    Name          string            `json:"name"`
    Preferences   map[string]string `json:"preferences"`   // 偏好设置
    Interests     []string          `json:"interests"`     // 兴趣领域
    CommunicationStyle string       `json:"communication_style"` // 沟通风格
    UpdatedAt     time.Time         `json:"updated_at"`
}
```

---

# 4 长期记忆接口 (memory.go)

```go
package longterm

import (
    "context"
)

// Manager 长期记忆管理器接口
type Manager interface {
    // Save 保存长期记忆
    Save(ctx context.Context, memory *LongTermMemory) error

    // GetByID 根据 ID 获取记忆
    GetByID(ctx context.Context, id uint) (*LongTermMemory, error)

    // ListByUser 获取用户的所有记忆
    ListByUser(ctx context.Context, userID uint, memoryType string) ([]*LongTermMemory, error)

    // SearchByContent 按内容搜索记忆（简单文本匹配）
    SearchByContent(ctx context.Context, userID uint, keyword string) ([]*LongTermMemory, error)

    // Update 更新记忆
    Update(ctx context.Context, memory *LongTermMemory) error

    // Delete 删除记忆
    Delete(ctx context.Context, id uint) error

    // GetProfile 获取用户画像
    GetProfile(ctx context.Context, userID uint) (*UserProfile, error)

    // SaveProfile 保存用户画像
    SaveProfile(ctx context.Context, profile *UserProfile) error

    // ArchiveFromSession 从会话归档记忆
    ArchiveFromSession(ctx context.Context, userID, conversationID uint, summary string, knowledge []string) error
}
```

---

# 5 用户画像管理 (profile.go)

```go
package longterm

import (
    "context"
    "encoding/json"
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
    return "memory:profile:" + string(rune(userID))
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
        // 根据记忆类型更新画像
        switch mem.Metadata["category"] {
        case "name":
            profile.Name = mem.Content
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

# 6 历史摘要管理 (summary.go)

```go
package longterm

import (
    "context"
    "time"
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

# 7 知识片段管理 (knowledge.go)

```go
package longterm

import (
    "context"
)

// KnowledgeManager 知识片段管理器
type KnowledgeManager struct {
    repo LongTermRepository
}

// NewKnowledgeManager 创建知识管理器
func NewKnowledgeManager(repo LongTermRepository) *KnowledgeManager {
    return &KnowledgeManager{
        repo: repo,
    }
}

// SaveKnowledge 保存知识片段
func (m *KnowledgeManager) SaveKnowledge(ctx context.Context, userID uint, content string, metadata map[string]interface{}) error {
    memory := &LongTermMemory{
        UserID:     userID,
        MemoryType: MemoryTypeKnowledge,
        Content:    content,
        Importance: 0.7, // 默认重要性
        Metadata:   metadata,
    }

    return m.repo.Create(ctx, memory)
}

// SearchKnowledge 搜索知识片段
func (m *KnowledgeManager) SearchKnowledge(ctx context.Context, userID uint, keyword string) ([]*LongTermMemory, error) {
    return m.repo.SearchByContent(ctx, userID, keyword)
}
```

---

# 8 数据库仓储 (repository.go)

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

// SearchByContent 按内容搜索记忆
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

# 9 RAG 预留接口 (rag/)

## 9.1 接口定义 (interface.go)

```go
package rag

import (
    "context"
)

// RAGEngine RAG 引擎接口（预留）
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
    ID       string             `json:"id"`
    Score    float64            `json:"score"`    // 相似度分数
    Content  string             `json:"content"`
    Metadata map[string]interface{} `json:"metadata"`
}
```

## 9.2 向量化接口 (vectorize.go)

```go
package rag

import (
    "context"
    "fmt"

    "Qavor/internal/embedding"
)

// Vectorizer 向量化器
type Vectorizer struct {
    embedClient embedding.Client
}

// NewVectorizer 创建向量化器
func NewVectorizer(embedClient embedding.Client) *Vectorizer {
    return &Vectorizer{
        embedClient: embedClient,
    }
}

// Vectorize 向量化文本
func (v *Vectorizer) Vectorize(ctx context.Context, texts []string) ([][]float64, error) {
    resp, err := v.embedClient.CreateEmbedding(ctx, &embedding.EmbeddingRequest{
        Model: "text-embedding-3-small",
        Input: texts,
    })
    if err != nil {
        return nil, fmt.Errorf("向量化失败: %w", err)
    }

    vectors := make([][]float64, len(resp.Data))
    for i, data := range resp.Data {
        vectors[i] = data.Embedding
    }

    return vectors, nil
}
```

## 9.3 检索接口 (search.go)

```go
package rag

import (
    "context"
)

// Searcher 检索器
type Searcher struct {
    vectorizer *Vectorizer
    engine     RAGEngine
}

// NewSearcher 创建检索器
func NewSearcher(vectorizer *Vectorizer, engine RAGEngine) *Searcher {
    return &Searcher{
        vectorizer: vectorizer,
        engine:     engine,
    }
}

// Search 搜索相关记忆
func (s *Searcher) Search(ctx context.Context, query string, topK int) ([]SearchResult, error) {
    // 1. 向量化查询文本
    vectors, err := s.vectorizer.Vectorize(ctx, []string{query})
    if err != nil {
        return nil, err
    }

    // 2. 语义搜索
    return s.engine.Search(ctx, vectors[0], topK)
}
```

---

# 10 端到端流程

```
┌──────────────────────────────────────────────────────────────┐
│                     长期记忆工作流程                            │
│                                                              │
│  ┌──────────────────────────────────────────────────────┐    │
│  │  场景1：会话结束时归档                                  │    │
│  │                                                      │    │
│  │  1. SSE 流式结束                                      │    │
│  │     └── 流式对话完成                                   │    │
│  │                                                      │    │
│  │  2. 归档会话摘要                                       │    │
│  │     └── longTermMgr.ArchiveFromSession(              │    │
│  │           userID, convID, summary, knowledge)        │    │
│  │     ├── 保存会话摘要 → long_term_memories             │    │
│  │     ├── 保存知识片段 → long_term_memories             │    │
│  │     └── 更新用户画像 → Redis 缓存                     │    │
│  │                                                      │    │
│  │  3. 向量化（预留）                                     │    │
│  │     └── vectorizer.Vectorize(summary)                │    │
│  │     └── 存储向量 ID → vector_id 字段                  │    │
│  └──────────────────────────────────────────────────────┘    │
│                                                              │
│  ┌──────────────────────────────────────────────────────┐    │
│  │  场景2：对话时检索相关记忆                              │    │
│  │                                                      │    │
│  │  1. 用户发送消息                                       │    │
│  │     └── "帮我回顾一下之前讨论的技术方案"                │    │
│  │                                                      │    │
│  │  2. 语义检索（预留）                                   │    │
│  │     └── searcher.Search(query, topK=5)               │    │
│  │     └── 返回相关记忆片段                               │    │
│  │                                                      │    │
│  │  3. 注入上下文                                         │    │
│  │     └── 将检索结果注入 System Prompt                   │    │
│  │     └── "[用户记忆] 之前讨论了微服务架构..."            │    │
│  │                                                      │    │
│  │  4. 调用 LLM                                          │    │
│  │     └── 基于增强后的上下文生成回复                      │    │
│  └──────────────────────────────────────────────────────┘    │
└──────────────────────────────────────────────────────────────┘
```

---

# 11 配置示例

```yaml
long_term_memory:
  enabled: true
  redis_ttl: 86400              # Redis 缓存过期时间（秒），默认24小时
  
  # RAG 配置（预留）
  rag:
    enabled: false              # 是否启用 RAG
    embedding_model: "text-embedding-3-small"
    vector_db: "pgvector"       # 向量数据库类型
    vector_dimension: 1536      # 向量维度
    search_top_k: 5             # 检索返回数量
```

---

# 12 与其他模块的关系

| 模块 | 关系 | 说明 |
|------|------|------|
| **Task 6 短期记忆** | 上游 | 会话结束时归档关键信息 |
| **Embedding 模型层** | 调用 | 向量化记忆内容 |
| **Task 4 上下文管理** | 输出 | 注入检索到的相关记忆 |
| **会话 CRUD** | 依赖 | 获取会话信息用于归档 |
| **消息 CRUD** | 依赖 | 获取消息内容用于提取知识 |

---

# 13 后续扩展

1. **向量数据库集成**：接入 pgvector / Milvus / Qdrant
2. **记忆衰减**：根据时间和访问频率自动调整重要性
3. **记忆合并**：相似记忆自动合并
4. **多模态记忆**：支持图片、文件等多模态记忆
5. **记忆可视化**：前端展示用户记忆图谱

---

# 14 四个模块的连续性总结

```
┌──────────────────────────────────────────────────────────────┐
│                     模块连续性关系                              │
│                                                              │
│  Task 4: 上下文管理                                           │
│  ├── 输入: MessageRepository (数据库历史消息)                  │
│  ├── 处理: Token 裁剪 + Prompt 组装                          │
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
│  ├── 输入: 会话结束时的关键信息                               │
│  ├── 处理: 持久化存储 + 向量化（预留）                        │
│  └── 输出: 用户画像 + 历史摘要 + 知识片段                     │
│         ↓                                                    │
│  RAG 检索（预留）→ 注入 ContextManager                       │
└──────────────────────────────────────────────────────────────┘
```