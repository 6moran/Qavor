# Task 6: 短期记忆设计文档

## 文档信息
- **项目**：Qavor Agent 对话系统
- **模块**：会话短期记忆（Short-term Memory）
- **依赖模块**：上下文管理（Task 4）
- **关联模块**：Memory Extractor（Task 8）、长期记忆（Task 7）
- **目标**：实现会话级别的短期记忆，在单次会话中维持上下文连贯性

## 职责边界

| 组件 | 职责 |
|------|------|
| **Short Memory (Task 6)** | 管理当前会话的上下文 |
| **Memory Extractor (Task 8)** | 判断和提取关键信息 |
| **Long Memory (Task 7)** | 存储和检索长期记忆 |

## 数据流关系

```
┌──────────────────────────────────────────────────────────────┐
│                        数据流关系                             │
│                                                              │
│  ┌──────────────────────────────────────────────────────┐    │
│  │           Context Builder (Task 4)                   │    │
│  │           上下文构建（桥梁）                           │    │
│  └──────────────────────────────────────────────────────┘    │
│         ↑                          ↓                         │
│         │ 读取                      │ 读取                    │
│  ┌──────────────┐          ┌──────────────┐                  │
│  │ Short Memory │          │ Long Memory  │                  │
│  │ (Task 6)     │          │ (Task 7)     │                  │
│  │ 会话上下文    │          │ 长期记忆      │                  │
│  └──────────────┘          └──────────────┘                  │
│         │                          ↑                         │
│         │ 数据                      │ 存储                    │
│         ↓                          │                         │
│  ┌──────────────────────────────────────────────────────┐    │
│  │         Memory Extractor (Task 8)                    │    │
│  │         判断和提取关键信息                             │    │
│  └──────────────────────────────────────────────────────┘    │
└──────────────────────────────────────────────────────────────┘
```

---

# 1 概述

短期记忆是对话系统的"工作记忆"，负责在单次会话中维持对话的上下文连贯性：

1. **消息缓冲**：缓存当前会话的最近消息
2. **上下文摘要**：当消息过多时，生成摘要替代完整历史（异步）
3. **会话状态**：追踪当前会话的关键信息（如用户意图、讨论主题）

```
┌──────────────────────────────────────────────────────────────┐
│                       短期记忆模块                             │
│                                                              │
│  ┌──────────────────────────────────────────────────────┐    │
│  │                 SessionMemory                         │    │
│  │  ├── 消息缓冲区 (Message Buffer)                      │    │
│  │  ├── 上下文摘要 (Context Summary)                     │    │
│  │  └── 会话状态 (Session State)                         │    │
│  └──────────────────────────────────────────────────────┘    │
│         ↑                          ↓                         │
│  ┌──────────────┐          ┌──────────────────┐              │
│  │ Runtime      │          │  Context Builder │              │
│  │ (更新记忆)    │          │  (读取记忆)       │              │
│  └──────────────┘          └──────────────────┘              │
│                                                              │
│  ┌──────────────────────────────────────────────────────┐    │
│  │               存储层 (Storage)                         │    │
│  │  └── Redis: 热数据缓存（会话级生命周期）                │    │
│  └──────────────────────────────────────────────────────┘    │
└──────────────────────────────────────────────────────────────┘
```

## 1.1 SSE 与短期记忆的关系

| 关系 | 说明 |
|------|------|
| **SSE 不更新记忆** | SSE 只负责推送事件，不负责更新业务数据 |
| **Runtime 更新记忆** | Runtime（Agent 执行引擎）负责更新 Short Memory |
| **SSE 只推送事件** | SSE 接收 Runtime 产生的事件并推送给前端 |

---

# 2 生命周期

## 2.1 Short Memory 生命周期

```
会话开始
    │
    ├── 创建 Short Memory
    │   └── SessionMemory { ConversationID, Buffer, Summary, State }
    │
    ├── 每次消息交互
    │   ├── 用户消息 → 参与 Context 构建
    │   ├── AI 回复完成 → 异步保存 Assistant 消息
    │   └── 异步更新 Buffer 和 State
    │
    ├── 摘要压缩（异步）
    │   └── 当消息数/Token 超过阈值 → 异步生成摘要
    │
    └── 清理条件
        ├── 会话删除
        ├── 会话过期
        └── 达到清理策略
```

## 2.2 Redis TTL 机制

| 策略 | 说明 |
|------|------|
| **初始 TTL** | 会话创建时设置默认 TTL（如 24 小时） |
| **活跃刷新** | 每次消息交互时刷新 TTL |
| **过期清理** | TTL 到期后自动清理 |

```go
// Redis TTL 策略
func (s *RedisStore) Save(ctx context.Context, memory *SessionMemory) error {
    key := s.key(memory.ConversationID)
    data, _ := json.Marshal(memory)

    // 保存并刷新 TTL（24小时）
    return s.client.Set(ctx, key, data, 24*time.Hour).Err()
}

// 每次消息交互时刷新 TTL
func (s *RedisStore) RefreshTTL(ctx context.Context, conversationID uint) error {
    key := s.key(conversationID)
    return s.client.Expire(ctx, key, 24*time.Hour).Err()
}
```

---

# 3 目录结构

```
internal/memory/
├── short_term/
│   ├── memory.go           // 短期记忆接口
│   ├── buffer.go           // 消息缓冲区
│   ├── summary.go          // 上下文摘要生成（异步）
│   ├── state.go            // 会话状态管理
│   └── store.go            // Redis 存储
```

---

# 4 类型定义

```go
package shortterm

import (
    "time"
)

// SessionMemory 会话短期记忆
type SessionMemory struct {
    ConversationID uint            `json:"conversation_id"`
    UserID         uint            `json:"user_id"`
    Buffer         *MessageBuffer  `json:"buffer"`           // 消息缓冲区
    Summary        string          `json:"summary"`          // 上下文摘要
    State          *SessionState   `json:"state"`            // 会话状态
    CreatedAt      time.Time       `json:"created_at"`
    UpdatedAt      time.Time       `json:"updated_at"`
}

// MessageBuffer 消息缓冲区
type MessageBuffer struct {
    Messages    []BufferMessage `json:"messages"`     // 缓存的消息
    MaxSize     int             `json:"max_size"`     // 最大消息数
    TotalTokens int             `json:"total_tokens"` // 估算总 Token 数
}

// BufferMessage 缓冲消息
type BufferMessage struct {
    MessageID     string    `json:"message_id"`      // 消息唯一标识
    Role          string    `json:"role"`             // 消息角色
    Content       string    `json:"content"`          // 消息内容
    Timestamp     time.Time `json:"timestamp"`        // 时间戳
    Tokens        int       `json:"tokens"`           // 估算 Token 数
    ConversationID uint     `json:"conversation_id"`  // 会话ID
    Metadata      map[string]string `json:"metadata,omitempty"` // 元数据
}

// SessionState 会话状态
type SessionState struct {
    Topic       string            `json:"topic"`        // 当前讨论主题
    UserIntent  string            `json:"user_intent"`  // 用户意图
    KeyEntities []string          `json:"key_entities"` // 关键实体
    Metadata    map[string]string `json:"metadata"`     // 其他元数据
}
```

---

# 5 短期记忆接口

```go
package shortterm

import (
    "context"
    "github.com/cloudwego/eino/schema"
)

// Manager 短期记忆管理器接口
type Manager interface {
    // GetMemory 获取会话的短期记忆
    GetMemory(ctx context.Context, conversationID uint) (*SessionMemory, error)

    // UpdateMemory 更新短期记忆（AI回复完成后异步调用）
    UpdateMemory(ctx context.Context, conversationID uint, message *schema.Message) error

    // GetContext 获取用于 LLM 的上下文（包含摘要+最近消息）
    GetContext(ctx context.Context, conversationID uint, maxTokens int) ([]*schema.Message, error)

    // ClearMemory 清除会话的短期记忆
    ClearMemory(ctx context.Context, conversationID uint) error

    // RefreshTTL 刷新会话的 Redis TTL
    RefreshTTL(ctx context.Context, conversationID uint) error
}
```

---

# 6 更新时机

## 6.1 正确的更新流程

```
用户发送消息
    │
    ├── 1. 用户消息参与 Context 构建（同步）
    │   └── Context Builder 读取 Short Memory
    │
    ├── 2. 调用 LLM（同步）
    │
    ├── 3. AI 回复完成（同步）
    │
    └── 4. 异步更新 Short Memory
        ├── 保存 Assistant 消息到 Buffer
        ├── 更新 SessionState
        └── 刷新 Redis TTL
```

## 6.2 错误的更新流程（不要这样做）

```
用户发送消息
    │
    ├── ❌ 更新 Short Memory（不要在这里更新）
    │
    ├── 调用 LLM
    │
    └── AI 回复完成
```

---

# 7 摘要生成策略

## 7.1 异步摘要生成

摘要生成是**后台异步任务**，不应该阻塞用户聊天流程。

```go
// 摘要生成配置
type SummaryConfig struct {
    MessageThreshold int    // 消息数量阈值（如 20 条）
    TokenThreshold   int    // Token 阈值（如 8000）
    EnableAsync      bool   // 是否启用异步生成
}

// 检查是否需要生成摘要
func (m *Manager) shouldGenerateSummary(memory *SessionMemory) bool {
    return len(memory.Buffer.Messages) > m.config.MessageThreshold ||
           memory.Buffer.TotalTokens > m.config.TokenThreshold
}

// 异步生成摘要
func (m *Manager) generateSummaryAsync(ctx context.Context, conversationID uint) {
    go func() {
        // 1. 获取当前消息
        memory, _ := m.GetMemory(ctx, conversationID)
        
        // 2. 调用 LLM 生成摘要
        summary, _ := m.llmClient.Complete(ctx, buildSummaryPrompt(memory.Buffer.Messages))
        
        // 3. 更新内存
        memory.Summary = summary
        m.store.Save(ctx, memory)
    }()
}
```

---

# 8 消息缓冲区 (buffer.go)

## 8.1 职责
- 维护最近 N 条消息的缓冲区
- 自动计算 Token 估算值
- 当缓冲区满时，触发摘要生成（异步）

## 8.2 实现

```go
package shortterm

import (
    "context"
    "time"

    "github.com/cloudwego/eino/schema"
    "go.uber.org/zap"
)

// MessageBufferManager 消息缓冲区管理器
type MessageBufferManager struct {
    logger    *zap.Logger
    maxSize   int // 缓冲区最大消息数
}

// NewMessageBufferManager 创建缓冲区管理器
func NewMessageBufferManager(logger *zap.Logger, maxSize int) *MessageBufferManager {
    if maxSize <= 0 {
        maxSize = 20 // 默认缓冲20条消息
    }
    return &MessageBufferManager{
        logger:  logger,
        maxSize: maxSize,
    }
}

// AddMessage 添加消息到缓冲区
func (m *MessageBufferManager) AddMessage(buffer *MessageBuffer, msg *schema.Message, messageID string) {
    bufMsg := BufferMessage{
        MessageID:      messageID,
        Role:           string(msg.Role),
        Content:        msg.Content,
        Timestamp:      time.Now(),
        Tokens:         estimateTokens(msg.Content),
        ConversationID: 0, // 由调用方设置
    }

    buffer.Messages = append(buffer.Messages, bufMsg)
    buffer.TotalTokens += bufMsg.Tokens

    // 如果缓冲区满，移除最旧的消息
    for len(buffer.Messages) > m.maxSize {
        removed := buffer.Messages[0]
        buffer.Messages = buffer.Messages[1:]
        buffer.TotalTokens -= removed.Tokens
    }
}
```

---

# 9 会话状态 (state.go)

## 9.1 更新策略

当前 SessionState 使用**简单规则更新**，后续可接入 LLM 提取。

```go
// 简单规则更新
func (m *Manager) updateStateSimple(state *SessionState, message *schema.Message) {
    // 1. 更新主题（简单规则：提取名词）
    // 2. 更新用户意图（简单规则：识别问句）
    // 3. 更新关键实体（简单规则：提取专有名词）
    
    // 后续可接入 LLM 进行智能提取
}
```

## 9.2 后续扩展

- 接入 LLM 进行智能意图识别
- 接入 NER 进行实体提取
- 接入主题模型进行主题追踪

---

# 10 短期记忆六大维度

## 10.1 维度总览

| 维度 | 关注点 | 当前状态 | 优先级 |
|------|--------|----------|--------|
| **缓冲和存储** | 消息缓存、Redis 持久化、TTL 管理 | ✅ 已实现 | - |
| **压缩和摘要** | 滑动窗口、LLM 摘要、规则降级 | ✅ 已实现 | - |
| **状态追踪** | 主题、意图、实体提取 | ⚠️ 简单规则 | P2 |
| **上下文组装** | 摘要+最近消息、Token 限制 | ✅ 已实现 | - |
| **智能增强** | 实体链接、重要性排序、指代消解 | ❌ 未实现 | P2 |
| **监控运维** | 质量评估、调试日志、性能指标 | ❌ 未实现 | P1 |

---

## 10.2 缓冲和存储（✅ 已实现）

### 已完成
- 消息缓冲区（FIFO）
- Redis 持久化
- TTL 自动过期（24h）
- 活跃刷新 TTL

### 架构图

```
┌─────────────────────────────────────────────────────────────┐
│                    缓冲和存储                                 │
│                                                             │
│  ┌──────────────┐     ┌──────────────┐     ┌──────────────┐ │
│  │  消息到来    │ ──► │  缓冲区      │ ──► │  Redis      │ │
│  │  (AddMessage)│     │  (FIFO)      │     │  (持久化)    │ │
│  └──────────────┘     └──────────────┘     └──────────────┘ │
│                             │                      │        │
│                             │ 满                   │ TTL    │
│                             ▼                      ▼        │
│                      ┌──────────────┐     ┌──────────────┐  │
│                      │  触发摘要    │     │  自动过期    │  │
│                      │  (异步)      │     │  (24h)       │  │
│                      └──────────────┘     └──────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

---

## 10.3 压缩和摘要（✅ 已实现）

### 已完成
- 滑动窗口（前半摘要，后半保留）
- LLM 摘要生成
- 规则式降级摘要
- 异步生成（不阻塞主流程）

### 摘要生成流程

```
缓冲区达到阈值
    │
    ├── 1. 滑动窗口
    │   ├── 前半部分 → 待摘要消息
    │   └── 后半部分 → 保留
    │
    ├── 2. 生成摘要（异步）
    │   ├── 尝试 LLM 摘要
    │   │   └── 失败 → 降级为规则式
    │   └── 规则式摘要：截取最近5条
    │
    └── 3. 更新存储
        ├── 旧摘要 + 新消息 → 新摘要
        └── 保存到 Redis
```

### 待优化
- 摘要质量评估
- 增量摘要（避免全量重新生成）
- 多粒度摘要（详细/简洁）

---

## 10.4 状态追踪（⚠️ 简单规则）

### 已完成
- 基础 SessionState 结构
- 简单规则更新

### 当前实现

```go
type SessionState struct {
    Topic       string            // 当前讨论主题
    UserIntent  string            // 用户意图
    KeyEntities []string          // 关键实体
    Metadata    map[string]string // 其他元数据
}
```

### 待实现

#### 10.4.1 智意图识别
```go
// IntentRecognizer 意图识别器
type IntentRecognizer interface {
    // Recognize 识别用户意图
    Recognize(ctx context.Context, messages []*schema.Message) (*Intent, error)
}

type Intent struct {
    Primary   string   // 主要意图：查询/创建/修改/删除
    Secondary string   // 次要意图
    Confidence float64 // 置信度
    Entities  []string // 涉及的实体
}
```

#### 10.4.2 主题追踪
```go
// TopicTracker 主题追踪器
type TopicTracker interface {
    // Track 追踪主题变化
    Track(ctx context.Context, messages []*schema.Message) (*Topic, error)
}

type Topic struct {
    Current   string    // 当前主题
    History   []string  // 主题历史
    Keywords  []string  // 关键词
    StartTime time.Time // 主题开始时间
}
```

#### 10.4.3 实体提取
```go
// EntityExtractor 实体提取器
type EntityExtractor interface {
    // Extract 提取实体
    Extract(ctx context.Context, text string) ([]*Entity, error)
}

type Entity struct {
    Name      string  // 实体名
    Type      string  // 实体类型：person/location/organization
    StartPos  int     // 起始位置
    EndPos    int     // 结束位置
    Confidence float64 // 置信度
}
```

---

## 10.5 上下文组装（✅ 已实现）

### 已完成
- 摘要 + 最近消息组合
- Token 限制控制
- 按 Token 数获取消息

### 组装流程

```
GetContext(conversationID, maxTokens)
    │
    ├── 1. 获取 SessionMemory
    │
    ├── 2. 添加摘要（如果有）
    │   └── [会话摘要] xxx
    │
    ├── 3. 获取最近消息（按 Token 数）
    │   └── GetMessagesByTokens(buffer, maxTokens)
    │
    └── 4. 返回 []*schema.Message
```

### 待优化
- 摘要与消息的 Token 分配策略
- 重要消息优先保留
- 动态调整摘要/消息比例

---

## 10.6 智能增强（❌ 未实现）

### 10.6.1 实体链接

**问题场景**：
```
用户：我喜欢 Python
用户：帮我写个脚本  // 需要知道是 Python 脚本
```

**解决方案**：
```go
// EntityLinker 实体链接器
type EntityLinker interface {
    // Link 链接实体到具体指代
    Link(ctx context.Context, state *SessionState, currentMsg string) ([]*EntityLink, error)
}

type EntityLink struct {
    Entity    string  // 实体名
    Type      string  // 实体类型
    Context   string  // 上下文信息
    Confidence float64 // 置信度
}
```

### 10.6.2 重要性排序

```go
// ImportanceRanker 重要性排序器
type ImportanceRanker interface {
    // Rank 对消息进行重要性排序
    Rank(messages []BufferMessage) []*MessageImportance
}

type MessageImportance struct {
    Message  BufferMessage
    Score    float64  // 0.0 - 1.0
    Reason   string   // 评分原因
}

// 评分规则
// - 包含实体/数字：+0.3
// - 包含决策/结论：+0.4
// - 用户明确要求记住：+0.5
// - 纯寒暄/确认：-0.2
```

### 10.6.3 指代消解

**问题场景**：
```
用户：张三今天去北京了
用户：他住在哪里？  // "他" 指向 "张三"
```

**解决方案**：
```go
// CoreferenceResolver 代词消解器
type CoreferenceResolver interface {
    // Resolve 识别代词指向的实体
    Resolve(ctx context.Context, messages []BufferMessage) ([]*EntityLink, error)
}
```

---

## 10.7 监控运维（❌ 未实现）

### 10.7.1 质量评估

```go
// MemoryQuality 记忆质量
type MemoryQuality struct {
    ConversationID uint
    Completeness   float64  // 完整性：保留了多少关键信息
    Coherence      float64  // 连贯性：上下文是否连贯
    Relevance      float64  // 相关性：记忆与问题的相关度
    CompressionRatio float64 // 压缩率：摘要/原始
    Timestamp      time.Time
}

// 评估方法
// - 完整性：实体覆盖率
// - 连贯性：对话流畅度
// - 相关性：语义相似度
// - 压缩率：摘要长度/原始消息长度
```

### 10.7.2 调试日志

```go
// MemoryDebug 记忆调试信息
type MemoryDebug struct {
    ConversationID uint
    BufferSize     int              // 缓冲区大小
    SummaryLength  int              // 摘要长度
    TokenUsage     int              // Token 使用量
    CompressionEvents []CompressionEvent  // 压缩事件
    ProcessingTime time.Duration    // 处理耗时
}

type CompressionEvent struct {
    Type      string    // 滑动窗口/LLM摘要/规则降级
    Timestamp time.Time
    InputCount int      // 输入消息数
    OutputCount int     // 输出消息数
    Success   bool
    Error     string
}

// 输出格式
// [DEBUG] 会话 123: 缓冲区 15 条, 摘要 200 字
// [DEBUG] 压缩事件: 滑动窗口, 15→8 条
// [DEBUG] 处理耗时: 12ms
```

### 10.7.3 性能指标

```go
// MemoryMetrics 记忆性能指标
type MemoryMetrics struct {
    ConversationID   uint
    UpdateCount      int64         // 更新次数
    SummaryCount     int64         // 摘要生成次数
    AvgUpdateTime    time.Duration // 平均更新耗时
    AvgSummaryTime   time.Duration // 平均摘要耗时
    CacheHitRate     float64       // 缓存命中率
    MemorySize       int64         // 内存占用（字节）
}

// 监控告警
// - 摘要生成失败率 > 5% → 告警
// - 平均更新耗时 > 100ms → 告警
// - 缓存命中率 < 80% → 告警
```

---

# 11 与其他模块的关系

| 模块 | 关系 | 说明 |
|------|------|------|
| **Context Builder (Task 4)** | 读取 | 读取 Short Memory 构建上下文 |
| **Runtime** | 写入 | AI 回复完成后更新 Short Memory |
| **Memory Extractor (Task 8)** | 数据源 | 提供会话数据用于提取 |
| **SSE Service (Task 5)** | 无关 | SSE 只推送事件，不更新记忆 |

## 11.1 模块关系图

```
┌─────────────────────────────────────────────────────────────┐
│                      模块关系                                │
│                                                             │
│  Runtime (Agent 执行引擎)                                    │
│      │                                                      │
│      │ 更新                                                 │
│      ▼                                                      │
│  Short Memory (Task 6) ──── 数据 ────▶ Memory Extractor     │
│      ↑                                      │               │
│      │ 读取                                 │ 提取          │
│      │                                      ▼               │
│  Context Builder (Task 4)            Long Memory (Task 7)   │
│      │                                                      │
│      │ 构建上下文                                             │
│      ▼                                                      │
│  ChatModel (LLM)                                            │
│      │                                                      │
│      │ 流式输出                                              │
│      ▼                                                      │
│  SSE Service (Task 5) ──── 推送 ────▶ Browser               │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

---

# 12 后续扩展优先级

## P0（当前迭代）
1. ✅ 消息缓冲
2. ✅ 滑动窗口
3. ✅ LLM 摘要
4. ✅ 上下文组装

## P1（下一迭代）
1. 调试日志
2. 性能指标
3. 质量评估

## P2（后续迭代）
1. 智意图识别
2. 主题追踪
3. 实体提取
4. 实体链接
5. 重要性排序

## P3（远期规划）
1. 指代消解
2. 多粒度摘要
3. 增量摘要
