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

// Extractor 长期记忆抽取器
// 优先用 LLM JSON 抽取；抽取失败或无模型时降级为规则式关键词匹配
type Extractor struct {
	logger        *zap.Logger
	modelResolver shortterm.ModelResolver
}

// NewExtractor 创建长期记忆抽取器
// modelResolver 可选（nil 时仅规则式）
func NewExtractor(logger *zap.Logger, modelResolver shortterm.ModelResolver) *Extractor {
	return &Extractor{logger: logger, modelResolver: modelResolver}
}

// HasModel 是否有可用模型
func (e *Extractor) HasModel(modelID uint) bool {
	return e.modelResolver != nil && modelID > 0
}

// ExtractedMemory 抽取结果单条（未落库的原始结果）
type ExtractedMemory struct {
	Category   string  `json:"category"`   // preference/identity/environment/knowledge/sustainable_task/decision
	Content    string  `json:"content"`    // 一句话总结的记忆正文
	Importance float64 `json:"importance"` // 0.0~1.0，越重要越容易被保留
	Confidence float64 `json:"confidence"` // 0.0~1.0，抽取置信度
}

// extractionResult LLM 返回结构
type extractionResult struct {
	Memories []ExtractedMemory `json:"memories"`
}

// ExtractAndStore 执行抽取并通过 callback 返回结果（调用方负责入库，便于在 goroutine 中异步）
// 有模型 → LLM；无模型或失败 → 规则式；完全没抽取到也不报错
// 注：抽取 goroutine 使用 context.Background()，避免主请求结束后 ctx 被取消导致 LLM 调用失败
func (e *Extractor) ExtractAndStore(
	_ context.Context,
	messages []TurnMessage,
	modelID uint,
	callback func([]ExtractedMemory),
) {
	go func() {
		// 用独立 context，不依赖主请求 ctx（主请求可能已结束/取消）
		bgCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		result, err := e.extractWithLLM(bgCtx, messages, modelID)
		if err != nil || len(result) == 0 {
			if err != nil {
				e.logger.Debug("长期记忆 LLM 抽取失败或无结果，降级规则式", zap.Error(err))
			}
			result = e.extractRuleBased(messages)
		}
		if len(result) > 0 && callback != nil {
			callback(result)
		}
	}()
}

// extractWithLLM 用 LLM 抽取
func (e *Extractor) extractWithLLM(ctx context.Context, messages []TurnMessage, modelID uint) ([]ExtractedMemory, error) {
	if !e.HasModel(modelID) {
		return nil, fmt.Errorf("no model configured")
	}
	client, err := e.modelResolver.CreateLLMClient(ctx, modelID)
	if err != nil {
		return nil, fmt.Errorf("创建 LLM 客户端失败: %w", err)
	}

	prompt := e.buildPrompt(messages)
	// 复用外层传入的 ctx（已在 ExtractAndStore 中设置 60s 超时）
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
		return nil, fmt.Errorf("解析抽取结果失败: %w, raw=%s", err, resp.Content)
	}

	// 清洗与校验
	out := make([]ExtractedMemory, 0, len(r.Memories))
	for _, m := range r.Memories {
		m.Category = strings.TrimSpace(strings.ToLower(m.Category))
		m.Content = strings.TrimSpace(m.Content)
		if !memtypes.IsValidCategory(m.Category) || m.Content == "" {
			continue
		}
		if m.Importance <= 0 {
			m.Importance = 0.5
		}
		if m.Confidence <= 0 {
			m.Confidence = 0.7
		}
		if len(m.Content) > 500 {
			m.Content = m.Content[:500]
		}
		out = append(out, m)
	}
	return out, nil
}

// TurnMessage 对话轮消息（轻量结构，与 state.go 的 BufferMessage 对齐）
type TurnMessage struct {
	Role    string
	Content string
}

// buildPrompt 构建抽取 Prompt
func (e *Extractor) buildPrompt(messages []TurnMessage) []*schema.Message {
	system := `你是一个长期记忆抽取助手。阅读下面的对话，提取值得跨会话记住的信息，以 JSON 格式返回。

有效类别 category（必须严格使用，不能造新值）：
  - preference: 用户明确表达的偏好（喜欢/讨厌/希望/不要/习惯/格式要求/风格要求）
  - identity: 用户身份/职业/角色/技能栈/公司/项目背景/社交关系（朋友/同事/家人）等画像信息
  - environment: 用户工作环境（操作系统/IDE/部署环境/网络/使用语言/工具链）
  - knowledge: 项目事实/技术约定/代码架构信息等长期有效的知识
  - sustainable_task: 持续性任务（长期项目/跟踪中但需要下次继续的）
  - decision: 明确达成的决策、约定、采用的方案（避免未来重复讨论）

判断标准（重要）：
  - identity 类：用户提到的人物关系（"X是我的朋友/同事/家人"）、自我介绍（"我是做XX的"）、所属组织（"我们公司/团队/项目"）都应抽取，importance ≥ 0.6
  - preference 类：任何关于喜欢/讨厌/希望/要求的表达都应抽取，importance ≥ 0.6
  - 一次性闲话/纯问候（"你好""谢谢""好的"）= importance 0，跳过
  - 模糊两可时宁可抽取（importance 0.5），不要漏掉

要求：
1. 只提取对话中明确出现的信息，不要脑补或推测
2. 每条 content 是一句完整的陈述，不超过 50 字，用"用户..."开头
3. importance 是对未来对话的重要性（0.0~1.0）：
   - 通用偏好/身份关系 = 0.6+
   - 项目级决策/技术约定 = 0.8+
   - 一次性闲话/问候 = 0（跳过）
4. confidence 是你对抽取准确性的信心（0.0~1.0）
5. 没有值得长期记住的信息，memories 就返回 []
6. 只输出 JSON，不要任何解释

返回格式：
{"memories":[{"category":"identity","content":"用户提到付涛和吴涵是他的朋友","importance":0.6,"confidence":0.9},{"category":"preference","content":"用户希望回答简洁不用Markdown表格","importance":0.8,"confidence":0.95}]}`

	// 拼接对话（截断到最近 ~2000 字，避免超长）
	var parts []string
	total := 0
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		role := "用户"
		if msg.Role == "assistant" {
			role = "助手"
		}
		line := fmt.Sprintf("[%s] %s", role, msg.Content)
		total += len(line)
		if total > 2500 {
			break
		}
		parts = append([]string{line}, parts...)
	}
	conversation := strings.Join(parts, "\n")

	return []*schema.Message{
		{Role: schema.System, Content: system},
		{Role: schema.User, Content: "对话内容：\n" + conversation + "\n\n请抽取长期记忆(JSON)："},
	}
}

// ---------------- 规则式降级 ----------------

// 规则式：基于关键词和正则做保底抽取（无模型或 LLM 失败时）
// 覆盖 6 个类别，每个用户消息依次过 6 个 matcher，命中即收集
func (e *Extractor) extractRuleBased(messages []TurnMessage) []ExtractedMemory {
	var out []ExtractedMemory
	for _, msg := range messages {
		if msg.Role != "user" {
			continue
		}
		content := strings.TrimSpace(msg.Content)
		if content == "" {
			continue
		}
		// 过滤纯问候/闲聊（不值得记忆）
		if isSmallTalk(content) {
			continue
		}
		if m := e.matchPreference(content); m != nil {
			out = append(out, *m)
		}
		if m := e.matchIdentity(content); m != nil {
			out = append(out, *m)
		}
		if m := e.matchEnvironment(content); m != nil {
			out = append(out, *m)
		}
		if m := e.matchKnowledge(content); m != nil {
			out = append(out, *m)
		}
		if m := e.matchDecision(content); m != nil {
			out = append(out, *m)
		}
		if m := e.matchSustainableTask(content); m != nil {
			out = append(out, *m)
		}
	}
	return out
}

// isSmallTalk 判断是否为纯问候/闲聊，这类不值得记忆
var smallTalkRE = regexp.MustCompile(`^(你好|您好|hi|hello|hey|嗨|哈喽|早上好|下午好|晚上好|谢谢|感谢|辛苦了|ok|好的|收到|嗯|哦|哈哈|嘿嘿|再见|拜拜|bye)[\s!！.。~]*$`)

func isSmallTalk(content string) bool {
	lower := strings.ToLower(content)
	return smallTalkRE.MatchString(lower)
}

// matchPreference 偏好类：喜欢/讨厌/希望/要求/习惯/格式
func (e *Extractor) matchPreference(content string) *ExtractedMemory {
	patterns := []string{
		"我喜欢", "我不喜欢", "我讨厌", "我希望", "我期望",
		"我不要", "不要用", "不用", "请用", "习惯", "要求",
		"喜欢用", "不喜欢用", "偏好", "格式",
	}
	for _, kw := range patterns {
		if strings.Contains(content, kw) {
			return &ExtractedMemory{
				Category:   memtypes.CategoryPreference,
				Content:    content,
				Importance: 0.6,
				Confidence: 0.5,
			}
		}
	}
	return nil
}

// matchIdentity 身份/关系类：我是/我的/职业/角色/朋友/同事/家人
var identityRE = regexp.MustCompile(`(我是|我叫|我做|我的职业|我的工作|我的角色|我是做|的朋友|的同事|的家人|的同学|的老板|的领导|我们公司|我们团队|我们项目)`)

func (e *Extractor) matchIdentity(content string) *ExtractedMemory {
	if identityRE.MatchString(content) {
		return &ExtractedMemory{
			Category:   memtypes.CategoryIdentity,
			Content:    content,
			Importance: 0.6,
			Confidence: 0.5,
		}
	}
	return nil
}

// matchEnvironment 环境类：操作系统/IDE/部署/语言/工具
var environmentRE = regexp.MustCompile(`(我用|我装|我部署|系统是|操作系统|Windows|Linux|macOS|IDE|VSCode|Goland|GoLand|部署在|部署到|服务器是|语言是|用Go|用Python|用Java|用Node|开发语言)`)

func (e *Extractor) matchEnvironment(content string) *ExtractedMemory {
	if environmentRE.MatchString(content) {
		return &ExtractedMemory{
			Category:   memtypes.CategoryEnvironment,
			Content:    content,
			Importance: 0.6,
			Confidence: 0.5,
		}
	}
	return nil
}

// matchKnowledge 知识/事实类：项目事实/架构/约定/规范
var knowledgeRE = regexp.MustCompile(`(项目用|项目是|架构是|架构用|规范是|约定是|技术栈|代码结构|目录结构|模块是|框架是|基于|使用的)`)

func (e *Extractor) matchKnowledge(content string) *ExtractedMemory {
	if knowledgeRE.MatchString(content) {
		return &ExtractedMemory{
			Category:   memtypes.CategoryKnowledge,
			Content:    content,
			Importance: 0.6,
			Confidence: 0.5,
		}
	}
	return nil
}

var decisionRE = regexp.MustCompile(`(决定|约定|采用|就用|就按|敲定|就这么定|结论|确定了|定下来|商定)`)

func (e *Extractor) matchDecision(content string) *ExtractedMemory {
	if decisionRE.MatchString(content) {
		return &ExtractedMemory{
			Category:   memtypes.CategoryDecision,
			Content:    content,
			Importance: 0.7,
			Confidence: 0.5,
		}
	}
	return nil
}

// matchSustainableTask 持续性任务：下次继续/待办/还要/跟踪
var sustainableTaskRE = regexp.MustCompile(`(下次继续|下次再|待办|还要|继续做|跟踪中|跟进中|正在进行|计划做|准备做|需要继续)`)

func (e *Extractor) matchSustainableTask(content string) *ExtractedMemory {
	if sustainableTaskRE.MatchString(content) {
		return &ExtractedMemory{
			Category:   memtypes.CategoryTask,
			Content:    content,
			Importance: 0.7,
			Confidence: 0.5,
		}
	}
	return nil
}

// -------------- utils --------------

var codeFenceRE = regexp.MustCompile("(?s)```(?:json)?\\s*(.*?)\\s*```")

// stripJSONCodeFence 去除 LLM 可能加的 ```json ... ``` 包裹
func stripJSONCodeFence(s string) string {
	s = strings.TrimSpace(s)
	if m := codeFenceRE.FindStringSubmatch(s); m != nil {
		return strings.TrimSpace(m[1])
	}
	return s
}
