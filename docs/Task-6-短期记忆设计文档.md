# Task 6: 短期记忆设计文档

## 文档信息
- **项目**：Qavor Agent 对话系统
- **模块**：会话短期记忆（Short-term Memory）
- **依赖模块**：上下文管理（Task 4）、SSE 流式服务（Task 5）
- **下游模块**：长期记忆（Task 7）
- **目标**：实现会话级别的短期记忆，在单次会话中维持上下文连贯性

---

# 1 概述

短期记忆是对话系统的"工作记忆"，负责在单次会话中维持对话的上下文连贯性：

1. **消息缓冲**：缓存当前会话的最近消息
2. **上下文摘要**：当消息过多时，生成摘要替代完整历史
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
│  │ SSE Service  │          │  Context Mgr     │              │
│  │ (更新记忆)    │          │  (读取记忆)       │              │
│  └──────────────┘          └──────────────────┘              │
│                                                              │
│  ┌──────────────────────────────────────────────────────┐    │
│  │               存储层 (Storage)                         │    │
│  │  ├── Redis: 热数据缓存（快速读写）                      │    │
│  │  └── PostgreSQL: 持久化存储（会话结束时）               │    │
│  └──────────────────────────────────────────────────────┘    │
└──────────────────────────────────────────────────────────────┘
```

---

# 2 目录结构

```
internal/memory/
├── short_term/
│   ├── memory.go           // 短期记忆接口
│   ├── buffer.go           // 消息缓冲区
│   ├── summary.go          // 上下文摘要生成
│   ├── state.go            // 会话状态管理
│   └── store.go            // Redis 存储
```

---

# 3 类型定义

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
    Role      string    `json:"role"`
    Content   string    `json:"content"`
    Timestamp time.Time `json:"timestamp"`
    Tokens    int       `json:"tokens"` // 估算 Token 数
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

# 4 短期记忆接口

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

    // UpdateMemory 更新短期记忆（每次消息交互后调用）
    UpdateMemory(ctx context.Context, conversationID uint, message *schema.Message) error

    // GetContext 获取用于 LLM 的上下文（包含摘要+最近消息）
    GetContext(ctx context.Context, conversationID uint, maxTokens int) ([]*schema.Message, error)

    // ClearMemory 清除会话的短期记忆
    ClearMemory(ctx context.Context, conversationID uint) error

    // ArchiveMemory 会话结束时归档记忆
    ArchiveMemory(ctx context.Context, conversationID uint) error
}
```

---

# 5 消息缓冲区 (buffer.go)

## 5.1 职责
- 维护最近 N 条消息的缓冲区
- 自动计算 Token 估算值
- 当缓冲区满时，触发摘要生成

## 5.2 实现

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
func (m *MessageBufferManager) AddMessage(buffer *MessageBuffer, msg *schema.Message) {
    bufMsg := BufferMessage{
        Role:      string(msg.Role),
        Content:   msg.Content,
        Timestamp: time.Now(),
        Tokens:    estimateTokens(msg.Content),
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

// GetRecent 获取最近的 N 条消息
func (m *MessageBufferManager) GetRecent(buffer *MessageBuffer, n int) []BufferMessage {
    if n <= 0 || n > len(buffer.Messages) {
        n = len(buffer.Messages)
    }
    return buffer.Messages[len(buffer.Messages)-n:]
}

// estimateTokens 估算 Token 数
func estimateTokens(content string) int {
    chineseCount := 0
    otherCount := 0
    for _, r := range content {
        if r > 0x4E00 && r < 0x9FFF {
            chineseCount++
        } else {
            otherCount++
        }
    }
    return int(float64(chineseCount)/1.5+float64(otherCount)/4) + 4
}
```

---

# 6 上下文摘要 (summary.go)

## 6.1 职责
- 当消息过多时，调用 LLM 生成上下文摘要
- 摘要替代旧消息，减少 Token 消耗
- 支持增量更新摘要

## 6.2 摘要生成策略

```
触发条件：消息数量 > 阈值（如 20 条）或 Token 数 > 阈值（如 8000）

生成流程：
1. 取前 N 条消息作为输入
2. 调用 LLM 生成摘要
3. 用摘要 + 最近的消息 替换完整历史
```

## 6.3 实现

```go
package shortterm

import (
    "context"
    "fmt"

    "Qavor/internal/llm"
    "github.com/cloudwego/eino/schema"
    "go.uber.org/zap"
)

// SummaryGenerator 上下文摘要生成器
type SummaryGenerator struct {
    llmClient llm.Client
    logger    *zap.Logger
}

// NewSummaryGenerator 创建摘要生成器
func NewSummaryGenerator(llmClient llm.Client, logger *zap.Logger) *SummaryGenerator {
    return &SummaryGenerator{
        llmClient: llmClient,
        logger:    logger,
    }
}

// GenerateSummary 生成上下文摘要
func (g *SummaryGenerator) GenerateSummary(ctx context.Context, messages []BufferMessage) (string, error) {
    // 构建摘要请求
    var conversationText string
    for _, msg := range messages {
        conversationText += fmt.Sprintf("[%s]: %s\n", msg.Role, msg.Content)
    }

    prompt := []*schema.Message{
        {
            Role:    schema.System,
            Content: "请将以下对话总结为简洁的摘要，保留关键信息和用户意图。摘要应该在100字以内。",
        },
        {
            Role:    schema.User,
            Content: conversationText,
        },
    }

    // 调用 LLM 生成摘要
    response, err := g.llmClient.Generate(ctx, prompt)
    if err != nil {
        return "", fmt.Errorf("生成摘要失败: %w", err)
    }

    return response.Content, nil
}

// UpdateSummary 增量更新摘要
func (g *SummaryGenerator) UpdateSummary(ctx context.Context, oldSummary string, newMessages []BufferMessage) (string, error) {
    var newMessagesText string
    for _, msg := range newMessages {
        newMessagesText += fmt.Sprintf("[%s]: %s\n", msg.Role, msg.Content)
    }

    prompt := []*schema.Message{
        {
            Role:    schema.System,
            Content: "请将旧摘要和新对话内容合并，生成更新后的摘要。保留关键信息，摘要控制在150字以内。",
        },
        {
            Role:    schema.User,
            Content: fmt.Sprintf("旧摘要：\n%s\n\n新对话：\n%s", oldSummary, newMessagesText),
        },
    }

    response, err := g.llmClient.Generate(ctx, prompt)
    if err != nil {
        return "", fmt.Errorf("更新摘要失败: %w", err)
    }

    return response.Content, nil
}
```

---

# 7 会话状态 (state.go)

## 7.1 职责
- 追踪当前会话的关键信息
- 支持结构化状态查询
- 为长期记忆提供输入

## 7.2 实现

```go
package shortterm

// StateManager 会话状态管理器
type StateManager struct {
    // 可以注入 LLM 用于状态提取
}

// NewStateManager 创建状态管理器
func NewStateManager() *StateManager {
    return &StateManager{}
}

// ExtractState 从消息中提取会话状态
func (m *StateManager) ExtractState(messages []BufferMessage) *SessionState {
    state := &SessionState{
        Metadata: make(map[string]string),
    }

    // 简单的状态提取逻辑
    // 后续可以接入 LLM 进行更智能的状态提取
    if len(messages) > 0 {
        // 取最后一条消息作为当前用户意图
        state.UserIntent = messages[len(messages)-1].Content
    }

    return state
}

// UpdateState 更新会话状态
func (m *StateManager) UpdateState(state *SessionState, newMessage BufferMessage) {
    state.UserIntent = newMessage.Content
}
```

---

# 8 Redis 存储 (store.go)

## 8.1 职责
- 短期记忆的 Redis 缓存
- 支持快速读写
- 自动过期清理

## 8.2 实现

```go
package shortterm

import (
    "context"
    "encoding/json"
    "fmt"
    "time"

    "github.com/redis/go-redis/v9"
)

// RedisStore Redis 存储
type RedisStore struct {
    client *redis.Client
    ttl    time.Duration // 记忆过期时间
}

// NewRedisStore 创建 Redis 存储
func NewRedisStore(client *redis.Client, ttl time.Duration) *RedisStore {
    if ttl <= 0 {
        ttl = 2 * time.Hour // 默认2小时过期
    }
    return &RedisStore{
        client: client,
        ttl:    ttl,
    }
}

// key 生成 Redis key
func (s *RedisStore) key(conversationID uint) string {
    return fmt.Sprintf("memory:short_term:%d", conversationID)
}

// Save 保存短期记忆
func (s *RedisStore) Save(ctx context.Context, memory *SessionMemory) error {
    data, err := json.Marshal(memory)
    if err != nil {
        return err
    }

    return s.client.Set(ctx, s.key(memory.ConversationID), data, s.ttl).Err()
}

// Load 加载短期记忆
func (s *RedisStore) Load(ctx context.Context, conversationID uint) (*SessionMemory, error) {
    data, err := s.client.Get(ctx, s.key(conversationID)).Bytes()
    if err != nil {
        if err == redis.Nil {
            return nil, nil // 不存在
        }
        return nil, err
    }

    var memory SessionMemory
    if err := json.Unmarshal(data, &memory); err != nil {
        return nil, err
    }

    return &memory, nil
}

// Delete 删除短期记忆
func (s *RedisStore) Delete(ctx context.Context, conversationID uint) error {
    return s.client.Del(ctx, s.key(conversationID)).Err()
}

// Exists 检查短期记忆是否存在
func (s *RedisStore) Exists(ctx context.Context, conversationID uint) (bool, error) {
    count, err := s.client.Exists(ctx, s.key(conversationID)).Result()
    return count > 0, err
}
```

---

# 9 端到端流程

```
┌──────────────────────────────────────────────────────────────┐
│                     短期记忆工作流程                            │
│                                                              │
│  1. 用户发送消息                                              │
│     └── POST /api/v1/conversations/:id/messages              │
│                                                              │
│  2. 保存消息到数据库                                          │
│     └── messageRepo.Create(userMessage)                      │
│                                                              │
│  3. 加载短期记忆                                              │
│     └── memoryMgr.GetMemory(conversationID)                  │
│     └── 从 Redis 读取，不存在则创建空记忆                      │
│                                                              │
│  4. 更新短期记忆                                              │
│     └── memoryMgr.UpdateMemory(conversationID, userMessage)  │
│     ├── 添加消息到缓冲区                                      │
│     ├── 检查是否需要生成摘要                                   │
│     └── 更新会话状态                                          │
│                                                              │
│  5. 构建 LLM 上下文                                          │
│     └── memoryMgr.GetContext(conversationID, maxTokens)      │
│     └── 返回 摘要 + 最近消息（作为历史）                        │
│                                                              │
│  6. 调用 LLM（SSE 流式）                                     │
│     └── llmClient.Stream(ctx, messages)                      │
│                                                              │
│  7. 保存 Assistant 消息                                       │
│     └── messageRepo.Create(assistantMessage)                 │
│                                                              │
│  8. 更新短期记忆（Assistant 消息）                             │
│     └── memoryMgr.UpdateMemory(conversationID, assistantMsg) │
│                                                              │
│  9. 会话结束时归档（可选）                                     │
│     └── memoryMgr.ArchiveMemory(conversationID)              │
│     └── 提取关键信息 → 写入长期记忆（Task 7）                   │
└──────────────────────────────────────────────────────────────┘
```

---

# 10 配置示例

```yaml
short_term_memory:
  enabled: true
  buffer_size: 20              # 缓冲区最大消息数
  summary_threshold: 20        # 触发摘要生成的消息数阈值
  summary_max_tokens: 8000     # 触发摘要生成的 Token 阈值
  redis_ttl: 7200              # Redis 过期时间（秒），默认2小时
  summary_model: "gpt-4o-mini" # 摘要生成使用的模型
```

---

# 11 与其他模块的关系

| 模块 | 关系 | 说明 |
|------|------|------|
| **Task 4 上下文管理** | 输出 | 提供摘要+最近消息作为 LLM 上下文 |
| **Task 5 SSE** | 输入 | 每次消息交互后更新记忆 |
| **Task 7 长期记忆** | 下游 | 会话结束时归档关键信息 |
| **LLM 抽象层** | 调用 | 生成上下文摘要 |
| **Redis** | 存储 | 缓存热数据 |

---

# 12 后续扩展

1. **智能状态提取**：接入 LLM 进行更智能的会话状态分析
2. **多会话记忆共享**：同一用户的多个会话间共享关键信息
3. **记忆重要性评分**：为不同消息分配不同的重要性权重
4. **可视化记忆**：前端展示当前会话的记忆状态