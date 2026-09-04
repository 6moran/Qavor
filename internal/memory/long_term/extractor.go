package longterm

import (
	memtypes "Qavor/internal/memory/types"
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	shortterm "Qavor/internal/memory/short_term"

	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

// ============================================================
// 长期记忆筛选器（Long-term Memory Filter）
//
// 架构流程：
//   User Message
//        │
//        ▼
//   ┌────────────────┐
//   │ Eligibility    │  ← 是否值得长期记忆？
//   │ 跨会话价值判断  │
//   └───────┬────────┘
//           │
//     NO ───┴─── YES
//     │           │
//   DISCARD       ▼
//            Extract Memory
//                  │
//                  ▼
//              Category
//                  │
//                  ▼
//                Scope  ← 记忆的粒度/范围
//                  │
//                  ▼
//        Importance / Confidence
//                  │
//                  ▼
//           Dedup / Conflict  ← 去重/冲突处理
//                  │
//                  ▼
//            Long-term DB
//
// 核心判断：如果用户明天新建一个完全不同的会话，
// 这条信息是否仍然值得 AI 知道？
// ============================================================

// Extractor 长期记忆筛选器
type Extractor struct {
	logger        *zap.Logger
	modelResolver shortterm.ModelResolver
}

// NewExtractor 创建长期记忆筛选器
func NewExtractor(logger *zap.Logger, modelResolver shortterm.ModelResolver) *Extractor {
	return &Extractor{logger: logger, modelResolver: modelResolver}
}

// HasModel 是否有可用模型
func (e *Extractor) HasModel(modelID uint) bool {
	return e.modelResolver != nil && modelID > 0
}

// ============================================================
// 数据结构
// ============================================================

// ExtractedMemory 抽取结果单条
type ExtractedMemory struct {
	Category   string  `json:"category"`   // preference/identity/environment/knowledge/decision
	Content    string  `json:"content"`    // 一句话总结的记忆正文
	Scope      string  `json:"scope"`      // 记忆粒度：user/project/global
	Importance float64 `json:"importance"` // 0.0~1.0，越重要越容易被保留
	Confidence float64 `json:"confidence"` // 0.0~1.0，抽取置信度
	Reason     string  `json:"reason"`     // 抽取原因（用于审计）
}

// extractionResult LLM 返回结构
type extractionResult struct {
	Memories []ExtractedMemory `json:"memories"`
}

// TurnMessage 对话轮消息
type TurnMessage struct {
	Role    string
	Content string
}

// ============================================================
// 常量配置
// ============================================================

const (
	minConfidenceToStore = 0.6 // confidence 最低门槛
	minContentLength     = 8   // 内容最短字符数
	maxContentLength     = 100 // 内容最长字符数
	minImportanceToStore = 0.5 // importance 最低门槛
)

// 分类 importance 最低门槛
var categoryMinImportance = map[string]float64{
	memtypes.CategoryPreference:  0.6,
	memtypes.CategoryIdentity:    0.6,
	memtypes.CategoryEnvironment: 0.6,
	memtypes.CategoryDecision:    0.7,
	memtypes.CategoryKnowledge:   0.5,
}

// ============================================================
// 核心入口
// ============================================================

// ExtractAndStore 执行抽取并通过 callback 返回结果
func (e *Extractor) ExtractAndStore(
	_ context.Context,
	messages []TurnMessage,
	modelID uint,
	callback func([]ExtractedMemory),
) {
	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		var result []ExtractedMemory

		// 优先 LLM 筛选
		if e.HasModel(modelID) {
			var err error
			result, err = e.extractWithLLM(bgCtx, messages, modelID)
			if err != nil {
				e.logger.Debug("长期记忆 LLM 筛选失败", zap.Error(err))
				result = nil
			}
		}

		// LLM 无结果或失败时，不进行规则式兜底
		// 宁可不记，也不要记垃圾

		// 质量过滤
		result = filterByThreshold(result)

		if len(result) > 0 && callback != nil {
			callback(result)
		}
	}()
}

// ============================================================
// LLM 筛选（核心逻辑）
// ============================================================

// extractWithLLM 用 LLM 进行跨会话价值判断
func (e *Extractor) extractWithLLM(ctx context.Context, messages []TurnMessage, modelID uint) ([]ExtractedMemory, error) {
	if !e.HasModel(modelID) {
		return nil, fmt.Errorf("no model configured")
	}

	client, err := e.modelResolver.CreateLLMClient(ctx, modelID)
	if err != nil {
		return nil, fmt.Errorf("创建 LLM 客户端失败: %w", err)
	}

	prompt := e.buildFilterPrompt(messages)
	resp, err := client.Generate(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("LLM 生成失败: %w", err)
	}

	content := stripJSONCodeFence(resp.Content)
	if content == "" {
		return nil, fmt.Errorf("LLM 返回空内容")
	}

	var r extractionResult
	if err := json.Unmarshal([]byte(content), &r); err != nil {
		return nil, fmt.Errorf("解析筛选结果失败: %w, raw=%s", err, resp.Content)
	}

	// 清洗与校验
	out := make([]ExtractedMemory, 0, len(r.Memories))
	for _, m := range r.Memories {
		m.Category = strings.TrimSpace(strings.ToLower(m.Category))
		m.Scope = strings.TrimSpace(strings.ToLower(m.Scope))
		m.Content = strings.TrimSpace(m.Content)
		m.Reason = strings.TrimSpace(m.Reason)

		// 基础校验
		if !memtypes.IsValidCategory(m.Category) || m.Content == "" {
			continue
		}
		if len(m.Content) < minContentLength || len(m.Content) > maxContentLength {
			continue
		}
		// Scope 校验（默认 user 级别）
		if m.Scope == "" {
			m.Scope = "user"
		}
		if m.Scope != "user" && m.Scope != "project" && m.Scope != "global" {
			m.Scope = "user"
		}
		// 默认值填充
		if m.Importance <= 0 {
			m.Importance = 0.5
		}
		if m.Confidence <= 0 {
			m.Confidence = 0.7
		}

		out = append(out, m)
	}

	return out, nil
}

// buildFilterPrompt 构建筛选 Prompt
func (e *Extractor) buildFilterPrompt(messages []TurnMessage) []*schema.Message {
	system := `你是一个长期记忆筛选器。你的任务不是记录用户说过的话，而是识别值得跨会话长期保存的信息。

## 核心判断标准

只有同时满足以下所有条件的信息才允许保存：

1. 信息明确来自用户（不是 AI 的回复或指令）
2. 不是当前一次性任务（"帮我分析""生成 PPT"等）
3. 不是当前问题或临时指令（"怎么实现""为什么报错"等）
4. 未来新建会话时仍然可能有用
5. 信息具有相对稳定性（不会频繁变化）

## 保存什么（5 个类别）

- preference: 稳定偏好（"以后回答尽量简洁""不要用 Markdown 表格"）
- identity: 稳定身份信息（"我是 Go 后端开发""我们团队 5 个人"）
- environment: 稳定环境（"项目主要使用 Go + MySQL""部署在阿里云"）
- knowledge: 用户/项目长期事实（"这个项目采用 Redis Stream""数据库用 PostgreSQL"）
- decision: 已确定的长期决策（"第一版不做工作流编排""决定用 gRPC"）

## Scope（记忆粒度）

- user: 仅当前用户适用（偏好、个人身份）
- project: 项目级别（项目技术栈、架构决策）
- global: 全局通用（通用最佳实践）

## 明确排除（以下内容直接返回空）

- "帮我..."、"请你..."、"分析..."、"生成..."、"总结..."、"告诉我..."
- "基于这些内容..."、"根据这个..."
- 当前对话中的临时要求
- 当前任务的中间信息
- AI 提出的问题
- 用户对当前问题的回答（一次性）
- 问候/确认/感谢（"好的""谢谢""收到"）

## 重要性判断

importance 评分标准（0.0~1.0）：
- 0.9~1.0: 用户明确的长期偏好/核心身份/关键决策
- 0.7~0.8: 项目级事实/技术约定
- 0.5~0.6: 一般性环境信息/辅助知识
- < 0.5: 不值得保存

## 要求

1. 只提取对话中明确出现的信息，不要脑补或推测
2. 每条 content 是一句完整、简洁的陈述（15~80 字）
3. confidence 是你对这条信息"跨会话价值"的信心（0.0~1.0）
4. 没有值得长期记住的信息，memories 就返回 []
5. 只输出 JSON，不要任何解释

## 返回格式

{
  "memories": [
    {
      "category": "preference",
      "content": "用户希望回答简洁，不要使用 Markdown 表格",
      "scope": "user",
      "importance": 0.85,
      "confidence": 0.95,
      "reason": "用户明确表达的格式偏好，未来会话仍然适用"
    }
  ]
}

如果没有任何值得保存的信息，返回：
{"memories": []}`

	// 构建对话内容（传入用户消息和 AI 回复作为上下文）
	var parts []string
	total := 0
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		var line string
		if msg.Role == "assistant" {
			content := msg.Content
			if len(content) > 200 {
				content = content[:200] + "..."
			}
			line = fmt.Sprintf("[助手] %s", content)
		} else {
			line = fmt.Sprintf("[用户] %s", msg.Content)
		}
		total += len(line)
		if total > 3000 {
			break
		}
		parts = append([]string{line}, parts...)
	}
	conversation := strings.Join(parts, "\n")

	return []*schema.Message{
		{Role: schema.System, Content: system},
		{Role: schema.User, Content: "对话内容：\n" + conversation + "\n\n请判断是否有值得跨会话长期保存的信息(JSON)："},
	}
}

// ============================================================
// 质量过滤
// ============================================================

// filterByThreshold 过滤不满足最低质量门槛的记忆
func filterByThreshold(items []ExtractedMemory) []ExtractedMemory {
	out := make([]ExtractedMemory, 0, len(items))
	for _, m := range items {
		// 置信度检查
		if m.Confidence < minConfidenceToStore {
			continue
		}
		// 内容长度检查
		trimmed := strings.TrimSpace(m.Content)
		if len(trimmed) < minContentLength || len(trimmed) > maxContentLength {
			continue
		}
		// 分类阈值检查
		minImp := categoryMinImportance[m.Category]
		if minImp <= 0 {
			minImp = minImportanceToStore
		}
		if m.Importance < minImp {
			continue
		}
		out = append(out, m)
	}
	return out
}

// ============================================================
// 工具函数
// ============================================================

// stripJSONCodeFence 去除 LLM 可能加的 ```json ... ``` 包裹
var codeFenceRE = regexp.MustCompile("(?s)```(?:json)?\\s*(.*?)\\s*```")

func stripJSONCodeFence(s string) string {
	s = strings.TrimSpace(s)
	if m := codeFenceRE.FindStringSubmatch(s); m != nil {
		return strings.TrimSpace(m[1])
	}
	return s
}
