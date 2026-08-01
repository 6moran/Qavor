# Task 4 上下文管理 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 实现上下文管理模块，打通"历史读取 → Token裁剪 → Prompt组装 → 消息持久化"基础链路

**Architecture:** 6 个文件各司其职，通过 Manager 接口统一暴露。fetcher 从 DB 读历史，compressor 做 Token 硬切片，builder 组装 System+History+UserMessage，persist 同步/异步落库。

**Tech Stack:** Go, GORM, eino (schema.Message), zap logger

## Global Constraints

- Go 1.25+, 使用 `context.WithoutCancel` (Go 1.21+)
- 消息排序：DB 查询正序 (ASC)，无需内存反转
- Token 估算：中文 1 token ≈ 1.5 字符，英文 1 token ≈ 4 字符
- System Message 由 Builder 动态注入，DB 历史中不包含
- Tool Calling 消息对（assistant+tool）裁剪时必须保护不被拆开

## File Structure

| 文件 | 职责 | 依赖 |
|------|------|------|
| `internal/context/types.go` | 类型定义 (ContextConfig, ContextWindow, HistoryQuery) | schema |
| `internal/context/context.go` | Manager 接口定义 | types, schema |
| `internal/context/fetcher.go` | 历史消息读取 (historyReader) | repository |
| `internal/context/compressor.go` | Token 计数与硬切片 (tokenizer) | schema |
| `internal/context/builder.go` | Prompt 组装 (Builder) | types, schema |
| `internal/context/persist.go` | 消息持久化 (Persister) | repository, entity, schema |

---

### Task 1: Repository 扩展 — 添加 ListBeforeID 方法

**Files:**
- Modify: `internal/repository/message_repository.go:12-22` (接口)
- Modify: `internal/repository/message_repository.go` (新增实现)

**Interfaces:**
- Consumes: 无
- Produces: `MessageRepository.ListBeforeID(conversationID uint, beforeID uint, limit int) ([]entity.Message, error)`

- [ ] **Step 1: 在 MessageRepository 接口中添加 ListBeforeID 方法签名**

在 `internal/repository/message_repository.go` 第 21 行后添加：

```go
ListBeforeID(conversationID uint, beforeID uint, limit int) ([]entity.Message, error)
```

- [ ] **Step 2: 实现 ListBeforeID 方法**

在 `message_repository.go` 文件末尾添加：

```go
// ListBeforeID 根据会话 ID 获取指定 ID 之前的消息（正序）
func (r *messageRepository) ListBeforeID(conversationID uint, beforeID uint, limit int) ([]entity.Message, error) {
	var messages []entity.Message
	err := r.db.Where("conversation_id = ? AND id < ?", conversationID, beforeID).
		Order("created_at ASC").
		Limit(limit).
		Find(&messages).Error
	return messages, err
}
```

- [ ] **Step 3: 编译验证**

Run: `go build ./internal/repository/`
Expected: PASS (无编译错误)

- [ ] **Step 4: Commit**

```bash
git add internal/repository/message_repository.go
git commit -m "feat(repo): 添加 MessageRepository.ListBeforeID 游标分页方法"
```

---

### Task 2: 类型定义 — types.go

**Files:**
- Create: `internal/context/types.go`

**Interfaces:**
- Consumes: `github.com/cloudwego/eino/schema`
- Produces: `ContextConfig`, `ContextWindow`, `HistoryQuery` 结构体

- [ ] **Step 1: 创建 types.go**

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
	ConversationID uint   // 会话 ID
	Limit          int    // 最大消息数量
	BeforeID       uint   // 分页：此 ID 之前的消息（游标分页）
	Roles          []string // 过滤角色（可选）
}
```

- [ ] **Step 2: 编译验证**

Run: `go build ./internal/context/`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/context/types.go
git commit -m "feat(context): 添加上下文管理类型定义"
```

---

### Task 3: Manager 接口 — context.go

**Files:**
- Create: `internal/context/context.go`

**Interfaces:**
- Consumes: `ContextConfig`, `ContextWindow`, `HistoryQuery` (from types.go), `schema.Message`
- Produces: `Manager` 接口

- [ ] **Step 1: 创建 context.go**

```go
package context

import (
	"context"

	"github.com/cloudwego/eino/schema"
)

// Manager 上下文管理器接口
type Manager interface {
	// FetchContext 提取历史与记忆（步骤1）
	// 从 MessageRepo 读取历史消息
	FetchContext(ctx context.Context, query *HistoryQuery) (*ContextWindow, error)

	// CompressContext 裁剪与压缩（步骤2）
	// Token 硬切片，保留最近的消息，丢弃超出窗口的旧消息
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
	CountTokens(messages []*schema.Message) int
}
```

- [ ] **Step 2: 编译验证**

Run: `go build ./internal/context/`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/context/context.go
git commit -m "feat(context): 添加 Manager 接口定义"
```

---

### Task 4: 历史消息读取 — fetcher.go

**Files:**
- Create: `internal/context/fetcher.go`

**Interfaces:**
- Consumes: `repository.MessageRepository`, `HistoryQuery`
- Produces: `historyReader` struct, `LoadHistory(ctx, query) ([]*schema.Message, error)`

- [ ] **Step 1: 创建 fetcher.go**

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
// 数据库已按 created_at ASC 排序（正序：旧→新），直接返回
func (r *historyReader) LoadHistory(ctx context.Context, query *HistoryQuery) ([]*schema.Message, error) {
	limit := query.Limit
	if limit <= 0 {
		limit = 50 // 默认最近50条
	}

	var messages []entity.Message
	var err error

	if query.BeforeID > 0 {
		// 游标分页：获取 BeforeID 之前的消息
		messages, err = r.messageRepo.ListBeforeID(query.ConversationID, query.BeforeID, limit)
	} else {
		// 获取最新的 limit 条消息（正序）
		messages, _, err = r.messageRepo.ListByConversationID(query.ConversationID, 0, limit)
	}

	if err != nil {
		return nil, err
	}

	// 过滤角色
	if len(query.Roles) > 0 {
		messages = r.filterByRoles(messages, query.Roles)
	}

	return r.toSchemaMessages(messages), nil
}

// filterByRoles 按角色过滤消息
func (r *historyReader) filterByRoles(messages []entity.Message, roles []string) []entity.Message {
	roleSet := make(map[string]bool, len(roles))
	for _, role := range roles {
		roleSet[role] = true
	}

	filtered := make([]entity.Message, 0, len(messages))
	for _, msg := range messages {
		if roleSet[msg.Role] {
			filtered = append(filtered, msg)
		}
	}
	return filtered
}

// toSchemaMessages 将实体消息转换为 schema.Message
// DB 已按正序排列，直接按顺序转换即可
func (r *historyReader) toSchemaMessages(messages []entity.Message) []*schema.Message {
	result := make([]*schema.Message, 0, len(messages))
	for _, msg := range messages {
		result = append(result, &schema.Message{
			Role:    schema.ChatRole(msg.Role),
			Content: msg.Content,
		})
	}
	return result
}
```

- [ ] **Step 2: 编译验证**

Run: `go build ./internal/context/`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/context/fetcher.go
git commit -m "feat(context): 实现历史消息读取器 fetcher"
```

---

### Task 5: Token 计数与裁剪 — compressor.go

**Files:**
- Create: `internal/context/compressor.go`

**Interfaces:**
- Consumes: `ContextConfig`, `schema.Message`
- Produces: `tokenizer` struct, `EstimateTokens(msg)`, `TrimMessages(messages, systemTokens)`, `CountAllTokens(messages)`

- [ ] **Step 1: 创建 compressor.go**

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
func (t *tokenizer) EstimateTokens(msg *schema.Message) int {
	content := msg.Content
	if content == "" {
		return 0
	}

	// 快速估算：根据字节数与字符数的比例判断中英文比例
	byteCount := len(content)
	charCount := utf8.RuneCountInString(content)

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
// 策略：保留最近的消息，丢弃最旧的消息
// 注意：System Message 由 Builder 动态注入，DB 历史中不包含
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

	// 保护 Tool Calling 消息对不被拆开
	keepStart = t.adjustForToolCallingPairs(messages, keepStart)

	return messages[keepStart:]
}

// adjustForToolCallingPairs 调整裁剪起始位置，保护 Tool Calling 消息对
// 避免将 assistant (tool_call) 和 tool (结果) 拆开
func (t *tokenizer) adjustForToolCallingPairs(messages []*schema.Message, keepStart int) int {
	if keepStart <= 0 {
		return keepStart
	}

	for i := keepStart; i < len(messages); i++ {
		msg := messages[i]
		if msg.Role == schema.Tool && i > 0 {
			prevMsg := messages[i-1]
			if prevMsg.Role == schema.Assistant && len(prevMsg.ToolCalls) > 0 {
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

- [ ] **Step 2: 编译验证**

Run: `go build ./internal/context/`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/context/compressor.go
git commit -m "feat(context): 实现 Token 计数与裁剪器 compressor"
```

---

### Task 6: Prompt 组装 — builder.go

**Files:**
- Create: `internal/context/builder.go`

**Interfaces:**
- Consumes: `ContextConfig`, `ContextWindow`, `schema.Message`
- Produces: `Builder` struct, `BuildPrompt(ctx, window, userMsg)`, `EstimateSystemTokens(window)`

- [ ] **Step 1: 创建 builder.go**

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
func (b *Builder) EstimateSystemTokens(window *ContextWindow) int {
	systemContent := b.buildSystemPrompt(window)
	tokenizer := NewTokenizer(0, 0)
	return tokenizer.EstimateTokens(&schema.Message{
		Role:    schema.System,
		Content: systemContent,
	})
}
```

注意：需要在文件头添加 `import "context"`。

- [ ] **Step 2: 编译验证**

Run: `go build ./internal/context/`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/context/builder.go
git commit -m "feat(context): 实现 Prompt 组装器 builder"
```

---

### Task 7: 消息持久化 — persist.go

**Files:**
- Create: `internal/context/persist.go`

**Interfaces:**
- Consumes: `repository.MessageRepository`, `entity.Message`, `schema.Message`
- Produces: `Persister` struct, `PersistUserMessage(ctx, convID, userMsg) (uint, error)`, `PersistAssistantMessage(ctx, convID, assistantMsg)`, `PersistAssistantMessageAsync(ctx, convID, assistantMsg)`

- [ ] **Step 1: 创建 persist.go**

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
// 在 SSE 流结束后调用，使用 context.WithoutCancel 保留链路追踪
func (p *Persister) PersistAssistantMessageAsync(ctx context.Context, conversationID uint, assistantMsg *schema.Message) {
	go func() {
		asyncCtx := context.WithoutCancel(ctx)
		if err := p.PersistAssistantMessage(asyncCtx, conversationID, assistantMsg); err != nil {
			p.logger.Error("异步保存助手回复失败", zap.Error(err))
		}
	}()
}
```

- [ ] **Step 2: 编译验证**

Run: `go build ./internal/context/`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/context/persist.go
git commit -m "feat(context): 实现消息持久化器 persist"
```

---

### Task 8: 全模块编译验证 + 整理

**Files:**
- 无新增，验证所有文件编译通过

- [ ] **Step 1: 全量编译**

Run: `go build ./...`
Expected: PASS

- [ ] **Step 2: 检查导入循环**

Run: `go vet ./internal/context/`
Expected: PASS

- [ ] **Step 3: 最终 Commit**

```bash
git add -A
git commit -m "feat(context): 完成上下文管理模块，打通历史读取→裁剪→组装→持久化链路"
```

---

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-07-30-context-management.md`. Two execution options:

**1. Subagent-Driven (recommended)** - I dispatch a fresh subagent per task, review between tasks, fast iteration

**2. Inline Execution** - Execute tasks in this session using executing-plans, batch execution with checkpoints

Which approach?
