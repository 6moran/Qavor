# Task 4: 上下文管理设计文档

## 文档信息
- **项目**：Qavor Agent 对话系统
- **模块**：上下文管理（Context Management）
- **依赖模块**：会话消息 CRUD（Task 1-3）、LLM 抽象层（设计文档）、Redis
- **下游模块**：SSE 流式服务（Task 5）、短期记忆（Task 6）、长期记忆（Task 7）
- **目标**：实现历史与记忆提取、上下文裁剪与压缩、Prompt 组装、消息持久化，为 LLM 调用提供完整上下文

---

# 1 概述

上下文管理是对话系统的核心枢纽，负责：
1. **历史与记忆提取**：从数据库和 Redis 加载历史消息和短期记忆
2. **上下文裁剪与压缩**：根据 Token 限制裁剪消息（硬切片），超出阈值的异步摘要生成将在 Task 6 接入
3. **Prompt 组装**：注入 System Prompt、RAG 检索结果、工具定义，生成 LLM 请求
4. **消息持久化**：将用户消息和助手回复写回数据库

它连接了数据层（Message Repository / Redis）和模型层（LLM Client），是 SSE 流式服务和短期记忆模块的基础。

```
┌─────────────────────────────────────────────────────────┐
│                    上下文管理模块                          │
│                                                         │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  │
│  │  历史与记忆   │  │ 上下文裁剪   │  │ Prompt 组装  │  │
│  │  提取        │→│  与压缩      │→│  (Builder)   │  │
│  │  (Fetcher)   │  │ (Compressor) │  │              │  │
│  └──────────────┘  └──────────────┘  └──────────────┘  │
│         ↑                                ↓              │
│  ┌──────────────┐              ┌──────────────────┐     │
│  │ MessageRepo  │              │  LLM Client      │     │
│  │ Redis        │              │  (模型层)         │     │
│  │ (数据层)      │              └──────────────────┘     │
│  └──────────────┘                    ↓                  │
│                               ┌──────────────┐          │
│                               │ 消息持久化    │          │
│                               │ (Persist)    │          │
│                               └──────────────┘          │
└─────────────────────────────────────────────────────────┘
```

---

# 2 目录结构

```
internal/context/
├── context.go          // 上下文管理器接口定义
├── fetcher.go          // 历史与记忆提取（从 DB/Redis 读取）
├── compressor.go       // 上下文裁剪与总结压缩
├── builder.go          // Prompt 组装逻辑
├── persist.go          // 消息持久化（写回数据库）
├── tokenizer.go        // Token 计数工具
├── types.go            // 上下文相关类型定义
└── config.go           // 上下文配置
```

---

# 3 类型定义 (types.go)

```go
package context

import (
    "github.com/cloudwego/eino/schema"
)

// ContextConfig 上下文配置
type ContextConfig struct {
    MaxTokens      int    // 模型最大 Token 限制（如 4096、8192、128000）
    ReserveTokens  int    // 预留给回复的 Token 数量
    SystemPrompt   string // 系统提示词
    TokenizerModel string // Token 计数使用的模型标识
}

// ContextWindow 上下文窗口
type ContextWindow struct {
    Messages       []*schema.Message // 裁剪后的消息列表
    TotalTokens    int               // 消息总 Token 数
    TrimmedCount   int               // 被裁剪的消息数量
    HasSystem      bool              // 是否包含系统消息
    MemoryContext  string            // 长期记忆上下文（可选）
    RAGContext     string            // RAG 检索结果（可选）
    ToolDefinitions []interface{}    // 工具定义（可选）
}

// HistoryQuery 历史查询参数
type HistoryQuery struct {
    ConversationID uint // 会话 ID
    Limit          int  // 最大消息数量
    BeforeID       uint // 分页：此 ID 之前的消息（游标分页）
}
```

---

# 4 上下文管理器接口 (context.go)

```go
package context

import (
    "context"
    "github.com/cloudwego/eino/schema"
)

// Manager 上下文管理器接口
type Manager interface {
    // FetchContext 提取历史与记忆（步骤1）
    // 从 MessageRepo 和 Redis 读取历史消息和短期记忆
    FetchContext(ctx context.Context, query *HistoryQuery) (*ContextWindow, error)

    // CompressContext 裁剪与压缩（步骤2）
    // Token 硬切片，保留最近的消息，丢弃超出窗口的旧消息
    // 注意：异步摘要生成（LLM Summary Worker）将在 Task 6 引入短期记忆组件时接入
    CompressContext(ctx context.Context, window *ContextWindow) (*ContextWindow, error)

    // BuildPrompt 组装 Prompt（步骤3）
    // 注入 System Prompt、RAG 检索结果、工具定义
    BuildPrompt(ctx context.Context, window *ContextWindow, userMessage *schema.Message) ([]*schema.Message, error)

    // PersistUserMessage 收到请求时同步先落库（防消息丢失）
    // 返回 UserMessageID 用于后续关联助手回复
    PersistUserMessage(ctx context.Context, conversationID uint, userMsg *schema.Message) (uint, error)

    // PersistAssistantMessage LLM 响应后保存回复
    PersistAssistantMessage(ctx context.Context, conversationID uint, assistantMsg *schema.Message) error

    // CountTokens 计算消息列表的 Token 数量
    CountMessagesTokens(messages []*schema.Message) (int, error)
}
```

---

# 5 历史与记忆提取 (fetcher.go)

## 5.1 职责
- 从 MessageRepository 读取会话历史消息
- 从 Redis 读取短期记忆（热数据）
- 支持游标分页（BeforeID）避免深分页
- 按时间正序返回（旧→新）
- 合并历史消息与记忆上下文

## 5.2 关键设计原则

> ⚠️ **消息排序与裁剪的注意事项**
> 
> 1. 数据库查询通常返回倒序（ID: 10, 9, 8），需要反转为正序（ID: 8, 9, 10）
> 2. 反转后，`messages[0]` 是最旧消息，`messages[len-1]` 是最新消息
> 3. **TrimMessages 裁剪时必须保护 System Message**（通常是 messages[0]）
> 4. Tool Calling 场景下，必须保护消息对（assistant+tool），避免配对断裂

## 5.3 实现

```go
package context

import (
    "context"

    "Qavor/internal/model/entity"
    "Qavor/internal/repository"

    "github.com/cloudwego/eino/schema"
)

// historyReader 历史消息读取器
type historyReader struct {
    messageRepo repository.MessageRepository
}

// NewHistoryReader 创建历史读取器
func NewHistoryReader(messageRepo repository.MessageRepository) *historyReader {
    return &historyReader{messageRepo: messageRepo}
}

// LoadHistory 加载历史消息
func (r *historyReader) LoadHistory(ctx context.Context, query *HistoryQuery) ([]*schema.Message, error) {
    limit := query.Limit
    if limit <= 0 {
        limit = 50 // 默认最近50条
    }

    // 从数据库查询消息（倒序：ID 从大到小）
    var messages []entity.Message
    var err error

    if query.BeforeID > 0 {
        // 游标分页：获取 BeforeID 之前的消息（ID < BeforeID）
        messages, err = r.messageRepo.ListBeforeID(query.ConversationID, query.BeforeID, limit)
    } else {
        // 获取最新的 limit 条消息
        messages, _, err = r.messageRepo.ListByConversationID(query.ConversationID, 0, limit)
    }

    if err != nil {
        return nil, err
    }

    // 转换为 eino schema.Message（正序：旧→新）
    // 注意：反转后 messages[0] 是最旧消息，messages[len-1] 是最新消息
    return r.toSchemaMessages(messages), nil
}

// toSchemaMessages 将实体消息转换为 schema.Message
func (r *historyReader) toSchemaMessages(messages []entity.Message) []*schema.Message {
    result := make([]*schema.Message, 0, len(messages))

    // 数据库查询是倒序，需要反转为正序
    for i := len(messages) - 1; i >= 0; i-- {
        msg := messages[i]
        result = append(result, &schema.Message{
            Role:    schema.ChatRole(msg.Role),
            Content: msg.Content,
        })
    }

    return result
}
```

## 5.3 Repository 扩展

需要在 `MessageRepository` 中新增方法：

```go
// MessageRepository 新增方法
ListBeforeID(conversationID uint, beforeID uint, limit int) ([]entity.Message, error)
```

---

# 6 上下文裁剪与总结压缩 (compressor.go)

## 6.1 职责
- 计算消息列表的 Token 数量
- 根据 Token 限制裁剪消息列表（硬切片：保留最近的消息，丢弃旧消息）
- 保留 System Prompt 的 Token 空间
- 保护 Tool Calling 消息对不被拆开

> 📌 **Task 4 阶段说明**：本阶段 Compressor 仅做 Token 计数与硬切片（Trimmer）。
> 超出阈值时的异步摘要生成（LLM Summary Worker）将在 Task 6 引入短期记忆组件时接入。

## 6.2 Token 计数策略

采用**估算策略**（无需调用 tokenizer API）：
- 英文：1 token ≈ 4 字符
- 中文：1 token ≈ 1.5 字符
- 代码：1 token ≈ 3 字符

> 后续可接入 tiktoken-go 实现精确计数

## 6.3 实现

```go
package context

import (
    "unicode/utf8"

    "github.com/cloudwego/eino/schema"
)

// tokenizer Token 计数与裁剪器
type tokenizer struct {
    maxTokens     int // 模型最大 Token
    reserveTokens int // 预留给回复的 Token
}

// NewTokenizer 创建裁剪器
func NewTokenizer(maxTokens, reserveTokens int) *tokenizer {
    return &tokenizer{
        maxTokens:     maxTokens,
        reserveTokens: reserveTokens,
    }
}

// EstimateTokens 估算单条消息的 Token 数
// 优化：使用 utf8.RuneCountInString 代替逐字符遍历，性能提升数倍
func (t *tokenizer) EstimateTokens(msg *schema.Message) int {
    content := msg.Content
    if content == "" {
        return 0
    }

    // 快速估算：根据字节数与字符数的比例判断中英文比例
    // 中文：UTF-8 编码通常 3 字节/字符，英文：1 字节/字符
    byteCount := len(content)
    charCount := utf8.RuneCountInString(content)

    // 计算中文字符比例（中文字符的 UTF-8 编码 >= 3 字节）
    // 粗估：如果 byteCount/charCount > 2，说明中文比例较高
    ratio := float64(byteCount) / float64(charCount)

    var tokens float64
    if ratio > 2.0 {
        // 中文为主：1 token ≈ 1.5 字符
        tokens = float64(charCount) / 1.5
    } else {
        // 英文为主：1 token ≈ 4 字符
        tokens = float64(charCount) / 4.0
    }

    return int(tokens) + 4 // 每条消息的固定开销（role + separators）
}

// TrimMessages 裁剪消息列表以适应 Token 窗口
// 策略：保留 System Prompt + 最近的消息，丢弃最旧的消息
// 重要：必须保护 System Message 和 Tool Calling 消息对
func (t *tokenizer) TrimMessages(messages []*schema.Message, systemTokens int) []*schema.Message {
    availableTokens := t.maxTokens - t.reserveTokens - systemTokens

    if availableTokens <= 0 {
        // Token 空间不足，只保留最后一条消息
        if len(messages) > 0 {
            return messages[len(messages)-1:]
        }
        return nil
    }

    // 从最新消息开始计算，保留尽可能多的消息
    totalTokens := 0
    keepStart := 0

    for i := len(messages) - 1; i >= 0; i-- {
        msgTokens := t.EstimateTokens(messages[i])
        if totalTokens+msgTokens > availableTokens {
            keepStart = i + 1
            break
        }
        totalTokens += msgTokens
        keepStart = i
    }

    // 关键：确保不会破坏 Tool Calling 消息对
    // 如果 keepStart 落在 tool_call 的 assistant 和 tool 结果之间，
    // 需要向前移动，保留完整的消息对
    // 注意：数据库查出的历史消息中不包含 System Message（由 Builder 动态注入）
    keepStart = t.adjustForToolCallingPairs(messages, keepStart)

    return messages[keepStart:]
}

// adjustForToolCallingPairs 调整裁剪起始位置，保护 Tool Calling 消息对
// 避免将 assistant (tool_call) 和 tool (结果) 拆开，否则会导致 400 Invalid Request
func (t *tokenizer) adjustForToolCallingPairs(messages []*schema.Message, keepStart int) int {
    if keepStart <= 0 {
        return keepStart
    }

    // 检查 keepStart 位置的消息是否是 tool 结果
    // 如果是，需要向前找到配对的 assistant (tool_call)
    for i := keepStart; i < len(messages); i++ {
        msg := messages[i]
        // 如果是 tool 消息，检查前一条是否是 assistant (带 tool_calls)
        if msg.Role == schema.Tool && i > 0 {
            prevMsg := messages[i-1]
            if prevMsg.Role == schema.Assistant && len(prevMsg.ToolCalls) > 0 {
                // 找到了 tool_call 和 tool 的配对
                // 如果 keepStart 在它们之间，需要向前调整
                if keepStart == i {
                    keepStart = i - 1
                }
            }
        }
    }

    return keepStart
}

// CountAllTokens 计算消息列表总 Token 数
func (t *tokenizer) CountAllTokens(messages []*schema.Message) int {
    total := 0
    for _, msg := range messages {
        total += t.EstimateTokens(msg)
    }
    return total
}
```

---

# 7 Prompt 组装 (builder.go)

## 7.1 职责
- 将 System Prompt + 裁剪后的消息组装为 LLM 请求格式
- 注入 System Prompt（基础指令）
- 注入 RAG 检索结果（相关知识片段，预留）
- 注入工具定义（Tool Calling，预留）
- 支持注入长期记忆上下文（对接 Task 7）
- 输出 `[]*schema.Message` 直接传给 LLM

## 7.2 组装流程

```
输入:
  ├── SystemPrompt: "你是一个有帮助的助手"
  ├── HistoryMessages: [msg1, msg2, ..., msgN]（已裁剪）
  ├── MemoryContext: "用户之前提到他喜欢Python"（来自长期记忆，可选）
  ├── RAGContext: "相关知识片段..."（来自 RAG 检索，可选）
  ├── ToolDefinitions: [...]（工具定义，可选）
  └── UserMessage: "帮我写一个排序算法"

输出:
  [
    { role: "system", content: "你是一个有帮助的助手\n\n[用户记忆] 用户之前提到他喜欢Python\n\n[相关知识] 相关知识片段..." },
    { role: "user", content: "你好" },
    { role: "assistant", content: "你好！有什么我可以帮助你的？" },
    ...
    { role: "user", content: "帮我写一个排序算法" }
  ]
```

## 7.3 实现

```go
package context

import (
    "fmt"
    "time"

    "github.com/cloudwego/eino/schema"
)

// Builder Prompt 组装器
type Builder struct {
    config *ContextConfig
}

// NewBuilder 创建组装器
func NewBuilder(config *ContextConfig) *Builder {
    return &Builder{config: config}
}

// BuildPrompt 组装最终的 Prompt 列表
// 签名与 Manager 接口保持一致
func (b *Builder) BuildPrompt(
    ctx context.Context,
    window *ContextWindow,
    userMessage *schema.Message,
) ([]*schema.Message, error) {

    result := make([]*schema.Message, 0, len(window.Messages)+2)

    // 1. 组装 System Prompt（包含记忆、RAG、工具定义）
    systemContent := b.buildSystemPrompt(window)
    result = append(result, &schema.Message{
        Role:    schema.System,
        Content: systemContent,
    })

    // 2. 追加历史消息（已裁剪）
    result = append(result, window.Messages...)

    // 3. 追加当前用户消息
    result = append(result, userMessage)

    return result, nil
}

// buildSystemPrompt 构建系统提示词
func (b *Builder) buildSystemPrompt(window *ContextWindow) string {
    content := b.config.SystemPrompt

    // 注入当前时间
    content += fmt.Sprintf("\n\n当前时间：%s", time.Now().Format("2006-01-02 15:04:05"))

    // 注入长期记忆上下文
    if window.MemoryContext != "" {
        content += fmt.Sprintf("\n\n[用户记忆]\n%s", window.MemoryContext)
    }

    // 注入 RAG 检索结果
    if window.RAGContext != "" {
        content += fmt.Sprintf("\n\n[相关知识]\n%s", window.RAGContext)
    }

    return content
}

// EstimateSystemTokens 估算 System Prompt 的 Token 数
// 与 buildSystemPrompt 入参类型一致，接收 ContextWindow
func (b *Builder) EstimateSystemTokens(window *ContextWindow) int {
    systemContent := b.buildSystemPrompt(window)
    tokenizer := NewTokenizer(0, 0)
    return tokenizer.EstimateTokens(&schema.Message{
        Role:    schema.System,
        Content: systemContent,
    })
}
```

---

# 8 消息持久化 (persist.go)

## 8.1 职责
- **用户消息**：收到请求时立即同步保存，获取 UserMessageID
- **助手回复**：LLM 返回结果后单独保存，关联 UserMessageID
- 支持同步/异步写入
- 处理写入失败的重试逻辑

## 8.2 关键设计原则

> ⚠️ **用户消息必须先于 LLM 调用保存**
> 
> 如果 LLM 调用失败（超时/429/500），用户的提问仍然保留在数据库中，不会丢失。

## 8.3 实现

```go
package context

import (
    "context"

    "Qavor/internal/model/entity"
    "Qavor/internal/repository"

    "github.com/cloudwego/eino/schema"
    "go.uber.org/zap"
)

// Persister 消息持久化器
type Persister struct {
    messageRepo repository.MessageRepository
    logger      *zap.Logger
}

// NewPersister 创建持久化器
func NewPersister(messageRepo repository.MessageRepository, logger *zap.Logger) *Persister {
    return &Persister{
        messageRepo: messageRepo,
        logger:      logger,
    }
}

// PersistUserMessage 立即保存用户消息（同步）
// 在收到请求时立即调用，确保用户消息不丢失
// 返回 UserMessageID 用于后续关联助手回复
func (p *Persister) PersistUserMessage(ctx context.Context, conversationID uint, userMsg *schema.Message) (uint, error) {
    userEntity := &entity.Message{
        ConversationID: conversationID,
        Role:           string(userMsg.Role),
        Content:        userMsg.Content,
    }
    if err := p.messageRepo.Create(userEntity); err != nil {
        p.logger.Error("保存用户消息失败", zap.Error(err))
        return 0, err
    }

    return userEntity.ID, nil
}

// PersistAssistantMessage 保存助手回复（同步）
// 在 LLM 返回结果后调用
func (p *Persister) PersistAssistantMessage(ctx context.Context, conversationID uint, assistantMsg *schema.Message) error {
    assistantEntity := &entity.Message{
        ConversationID: conversationID,
        Role:           string(assistantMsg.Role),
        Content:        assistantMsg.Content,
    }
    if err := p.messageRepo.Create(assistantEntity); err != nil {
        p.logger.Error("保存助手回复失败", zap.Error(err))
        return err
    }

    return nil
}

// PersistAssistantMessageAsync 异步保存助手回复
// 在 SSE 流结束后调用
func (p *Persister) PersistAssistantMessageAsync(ctx context.Context, conversationID uint, assistantMsg *schema.Message) {
    go func() {
        asyncCtx := p.createAsyncContext(ctx)
        if err := p.PersistAssistantMessage(asyncCtx, conversationID, assistantMsg); err != nil {
            p.logger.Error("异步保存助手回复失败", zap.Error(err))
        }
    }()
}

// createAsyncContext 创建异步上下文，保留链路追踪信息但不受父 context 取消影响
// Go 1.21+ 使用 context.WithoutCancel 保留值但取消传播
func (p *Persister) createAsyncContext(ctx context.Context) context.Context {
    // context.WithoutCancel 返回一个新 context：
    // 1. 继承父 context 的所有值（TraceID、SpanID 等链路追踪信息）
    // 2. 但不受父 context 的取消影响（HTTP 请求结束后仍可使用）
    return context.WithoutCancel(ctx)
}
```

---

# 9 端到端流程

```
┌────────────────────────────────────────────────────────────────────────┐
│                        上下文管理模块 (Context Manager)                │
│                                                                        │
│  ┌────────────────┐                                                    │
│  │ 1. 历史与记忆提取│ ◄─── 从数据库读取 (MessageRepo / Redis)          │
│  │    (Fetcher)   │                                                    │
│  └───────┬────────┘                                                    │
│          │                                                             │
│          ▼                                                             │
│  ┌────────────────┐      ┌─────────────────────────────┐               │
│  │ 2. 上下文裁剪与│ ───► │ Token 硬切片                │               │
│  │    总结压缩    │      │ (异步摘要 Task 6 接入)       │               │
│  │   (Compressor) │      └─────────────────────────────┘               │
│  └───────┬────────┘                                                    │
│          │                                                             │
│          ▼                                                             │
│  ┌────────────────┐      ┌─────────────────────────────┐               │
│  │ 3. Prompt 组装 │ ◄─── │ 注入 System / RAG / 工具定义 │               │
│  │    (Builder)   │      └─────────────────────────────┘               │
│  └───────┬────────┘                                                    │
│          │                                                             │
│          ▼                                                             │
│  ┌────────────────┐                                                    │
│  │ 4. LLM Client  │ ───► 发送请求给大模型                                │
│  └───────┬────────┘                                                    │
│          │                                                             │
│          ▼ (拿到模型返回的 Response)                                     │
│  ┌────────────────┐                                                    │
│  │ 5. 消息持久化   │ ───► 将 [User消息] 和 [Assistant回复]              │
│  │    (Persist)   │      异步/同步写回 MessageRepo 数据库              │
│  └────────────────┘                                                    │
└────────────────────────────────────────────────────────────────────────┘
```

---

# 10 配置示例

```yaml
context:
  max_tokens: 128000        # 模型最大 Token（gpt-4o: 128k, gpt-4o-mini: 128k）
  reserve_tokens: 4096      # 预留给回复的 Token
  system_prompt: |
    你是 Qavor 智能助手，请根据用户的问题提供准确、有帮助的回答。
    如果不确定答案，请诚实说明。
  history_limit: 50         # 默认加载历史消息数量
  summary_threshold: 20     # 触发摘要生成的消息数阈值
  summary_max_tokens: 8000  # 触发摘要生成的 Token 阈值
  persist_mode: "async"     # 消息持久化模式：sync/async
```

---

# 10 与其他模块的关系

| 模块 | 关系 | 说明 |
|------|------|------|
| **MessageRepo** | 依赖 | 读取历史消息，持久化用户消息和助手回复 |
| **Redis** | 依赖 | 读取短期记忆缓存（热数据） |
| **LLM 抽象层** | 调用 | 发送组装后的 Prompt 给大模型 |
| **Task 5 SSE** | 上游 | SSE Controller 调用 ContextManager 构建上下文 |
| **Task 6 短期记忆** | 集成 | 读取短期记忆，接入异步摘要生成（LLM Summary Worker） |
| **Task 7 长期记忆** | 集成 | 从长期记忆获取用户偏好、历史摘要 |
| **RAG 模块** | 集成 | Prompt 组装时注入 RAG 检索结果（预留） |
| **Tool 模块** | 集成 | Prompt 组装时注入工具定义（预留） |

---

# 12 上下文管理六大维度

## 12.1 维度总览

| 维度 | 关注点 | 当前状态 | 优先级 |
|------|--------|----------|--------|
| **存储和传递** | 消息持久化、历史加载、数据流转 | ✅ 已实现 | - |
| **裁剪和压缩** | Token 限制、硬切片、摘要生成 | ⚠️ 部分实现 | P1 |
| **组装和注入** | System Prompt、RAG、工具定义 | ✅ 已实现 | - |
| **理解和增强** | 代词消解、省略恢复、实体链接 | ❌ 未实现 | P2 |
| **优化和策略** | 滑动窗口、重要性排序、动态调整 | ❌ 未实现 | P2 |
| **监控和调试** | Token 用量追踪、上下文质量评估 | ❌ 未实现 | P1 |

---

## 12.2 存储和传递（✅ 已实现）

### 已完成
- 消息持久化（同步/异步）
- 历史消息加载（游标分页）
- Redis 短期记忆缓存

### 架构图

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│  用户消息   │ ──► │ MessageRepo │ ──► │  历史加载   │
│  (Persist)  │     │  (MySQL)    │     │  (Fetcher)  │
└─────────────┘     └─────────────┘     └─────────────┘
                           │
                           ▼
                    ┌─────────────┐
                    │   Redis     │
                    │ (短期记忆)   │
                    └─────────────┘
```

---

## 12.3 裁剪和压缩（⚠️ 部分实现）

### 已完成
- Token 硬切片（保留最近消息）
- Tool Calling 消息对保护

### 待实现

#### 12.3.1 LLM 摘要压缩
```go
// Compressor 新增方法
type Compressor interface {
    // SummarizeMessages 使用 LLM 生成消息摘要
    SummarizeMessages(ctx context.Context, messages []*schema.Message) (string, error)
    
    // CompressWithSummary 使用摘要替代旧消息
    CompressWithSummary(ctx context.Context, window *ContextWindow) (*ContextWindow, error)
}
```

**触发条件**：
- 消息数量 > `SummaryThreshold`（默认 20 条）
- Token 数量 > `SummaryMaxTokens`（默认 8000）

**压缩策略**：
```
原始消息: [msg1, msg2, msg3, ..., msg20, msg21, msg22]
                    │
                    ▼ LLM 摘要
压缩后:   [摘要: "用户讨论了...", msg21, msg22]
```

#### 12.3.2 重要性排序裁剪
```go
// 重要性评分
type MessageImportance struct {
    Message  *schema.Message
    Score    float64  // 0.0 - 1.0
    Reason   string   // 评分原因
}

// 评分规则
// - 包含实体/数字的消息：+0.3
// - 包含决策/结论的消息：+0.4
// - 用户明确要求记住的消息：+0.5
// - 纯寒暄/确认消息：-0.2
```

#### 12.3.3 时间范围裁剪
```go
// 按时间裁剪
type TimeRangeTrimmer struct {
    MaxAge      time.Duration  // 最大保留时间（如 24h）
    MinMessages int            // 最少保留消息数
}
```

---

## 12.4 组装和注入（✅ 已实现）

### 已完成
- System Prompt 注入
- 长期记忆上下文注入
- RAG 检索结果注入
- 工具定义注入

### 增强：上下文指代注入

```go
// ContextWindow 新增字段
type ContextWindow struct {
    // ... 现有字段
    EntityContext string  // 实体上下文：代词→实体映射
}

// 注入格式
// [实体上下文]
// 张三 → 用户提到的人物
// 北京 → 用户提到的地点
// Python → 用户偏好的编程语言
```

---

## 12.5 理解和增强（❌ 未实现）

### 12.5.1 代词消解（Coreference Resolution）

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
    Resolve(ctx context.Context, messages []*schema.Message) ([]*EntityLink, error)
}

type EntityLink struct {
    Pronoun   string  // 代词：他、她、它、这个
    Entity    string  // 实体：张三、北京
    Position  int     // 在消息中的位置
    Confidence float64 // 置信度
}
```

**实现策略**：
1. **规则匹配**：基于性别、单复数等语法规则
2. **LLM 辅助**：复杂场景调用轻量级 LLM
3. **上下文缓存**：实体信息缓存到 `ContextWindow`

### 12.5.2 省略恢复

**问题场景**：
```
用户：今天天气怎么样？
助手：北京今天晴天，25度
用户：明天呢？  // 省略了"天气"
```

**解决方案**：
```go
// EllipsisRecovery 省略恢复器
type EllipsisRecovery interface {
    // Recover 恢复省略的信息
    Recover(ctx context.Context, currentMsg *schema.Message, history []*schema.Message) (*schema.Message, error)
}
```

**实现策略**：
1. **模式识别**：识别常见省略模式（"呢"、"那"、"这个"）
2. **上下文补全**：从历史消息中提取被省略的信息
3. **Prompt 增强**：在用户消息前追加补全信息

### 12.5.3 实体链接

**问题场景**：
```
用户：我喜欢 Python
用户：帮我写个脚本  // 需要知道"脚本"是 Python 脚本
```

**解决方案**：
```go
// EntityLinker 实体链接器
type EntityLinker interface {
    // Link 链接实体到具体指代
    Link(ctx context.Context, messages []*schema.Message) ([]*EntityInfo, error)
}

type EntityInfo struct {
    Name       string            // 实体名
    Type       string            // 实体类型：person/location/language
    Attributes map[string]string // 实体属性
    FirstMention int             // 首次提及的消息索引
}
```

---

## 12.6 优化和策略（❌ 未实现）

### 12.6.1 滑动窗口

```go
// SlidingWindow 滑动窗口
type SlidingWindow struct {
    MaxAge      time.Duration  // 最大保留时间
    MinMessages int            // 最少保留消息数
    MaxMessages int            // 最多保留消息数
}

// 策略
// 1. 优先保留最近时间的消息
// 2. 保证至少 MinMessages 条消息
// 3. 不超过 MaxMessages 条消息
```

### 12.6.2 动态窗口调整

```go
// DynamicWindow 动态窗口
type DynamicWindow struct {
    BaseSize        int     // 基础窗口大小
    ComplexityScale float64 // 复杂度缩放因子
}

// 根据问题复杂度调整窗口
// - 简单问题（天气、时间）：小窗口
// - 复杂问题（代码调试、多轮推理）：大窗口
```

### 12.6.3 重要性排序

```go
// ImportanceRanker 重要性排序器
type ImportanceRanker interface {
    // Rank 对消息进行重要性排序
    Rank(messages []*schema.Message) []*MessageImportance
}

// 评分维度
// - 实体密度：包含多少命名实体
// - 信息增益：是否提供了新信息
// - 决策相关：是否包含决策/结论
// - 用户强调：用户是否明确要求记住
```

---

## 12.7 监控和调试（❌ 未实现）

### 12.7.1 Token 用量追踪

```go
// TokenUsage Token 用量
type TokenUsage struct {
    ConversationID uint
    RequestID      string
    InputTokens    int
    OutputTokens   int
    TotalTokens    int
    TruncatedCount int     // 被裁剪的消息数
    Timestamp      time.Time
}

// 存储到数据库，用于成本核算和优化
```

### 12.7.2 上下文质量评估

```go
// ContextQuality 上下文质量
type ContextQuality struct {
    Completeness   float64  // 完整性：保留了多少关键信息
    Relevance      float64  // 相关性：上下文与问题的相关度
    Coherence      float64  // 连贯性：上下文是否连贯
    InformationLoss float64 // 信息损失：裁剪丢失了多少信息
}

// 评估方法
// - 完整性：实体覆盖率
// - 相关性：语义相似度
// - 连贯性：对话流畅度
// - 信息损失：摘要前后对比
```

### 12.7.3 调试日志

```go
// ContextDebug 上下文调试信息
type ContextDebug struct {
    OriginalMessages  []*schema.Message  // 原始消息
    TrimmedMessages   []*schema.Message  // 裁剪后消息
    SystemPrompt      string             // 最终 System Prompt
    TokenBreakdown    map[string]int     // Token 分布
    CompressionRatio  float64            // 压缩率
    ProcessingTime    time.Duration      // 处理耗时
}

// 输出格式
// [DEBUG] 原始消息: 15 条, 4500 tokens
// [DEBUG] 裁剪后: 8 条, 2800 tokens
// [DEBUG] 压缩率: 37.8%
// [DEBUG] 处理耗时: 23ms
```

---

# 13 后续扩展优先级

## P0（当前迭代）
1. ✅ 消息持久化
2. ✅ 历史加载
3. ✅ Token 裁剪
4. ✅ Prompt 组装

## P1（下一迭代）
1. LLM 摘要压缩
2. Token 用量追踪
3. 调试日志

## P2（后续迭代）
1. 代词消解
2. 省略恢复
3. 滑动窗口
4. 动态窗口调整

## P3（远期规划）
1. 重要性排序
2. 上下文质量评估
3. 多模态支持