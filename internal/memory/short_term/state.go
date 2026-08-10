package shortterm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

// SessionStateManager 会话状态管理器
// 优先用 LLM 抽取意图/实体/主题；未配置模型或抽取失败时降级为规则式
type SessionStateManager struct {
	logger        *zap.Logger
	modelResolver ModelResolver
}

// NewSessionStateManager 创建会话状态管理器
// modelResolver 为可选参数，nil 时降级为规则式抽取
func NewSessionStateManager(logger *zap.Logger, modelResolver ModelResolver) *SessionStateManager {
	return &SessionStateManager{
		logger:        logger,
		modelResolver: modelResolver,
	}
}

// HasModel 是否配置了可用的抽取模型
func (m *SessionStateManager) HasModel(modelID uint) bool {
	return m.modelResolver != nil && modelID > 0
}

// UpdateState 更新会话状态（规则式，作为降级或无模型时的主路径）
func (m *SessionStateManager) UpdateState(state *SessionState, message *schema.Message) {
	if state == nil {
		return
	}

	content := message.Content
	now := time.Now()

	// 1. 更新用户意图（简单规则：识别问句）
	if message.Role == schema.User {
		intent := m.detectIntent(content)
		state.UserIntent = intent.Primary

		// 记录意图历史
		state.IntentHistory = append(state.IntentHistory, Intent{
			Primary:    intent.Primary,
			Confidence: intent.Confidence,
			Timestamp:  now,
		})

		// 限制意图历史数量
		if len(state.IntentHistory) > 20 {
			state.IntentHistory = state.IntentHistory[len(state.IntentHistory)-20:]
		}
	}

	// 2. 更新关键实体（简单规则：提取引号内容 + 常见实体）
	entities := m.extractEntities(content)
	for _, entity := range entities {
		// 检查是否已存在
		exists := false
		for _, e := range state.ExtractedEntities {
			if e.Name == entity.Name {
				e.LastSeen = now
				exists = true
				break
			}
		}
		if !exists {
			state.ExtractedEntities = append(state.ExtractedEntities, Entity{
				Name:      entity.Name,
				Type:      entity.Type,
				FirstSeen: now,
				LastSeen:  now,
			})
		}
	}

	// 限制实体数量
	if len(state.ExtractedEntities) > 20 {
		state.ExtractedEntities = state.ExtractedEntities[len(state.ExtractedEntities)-20:]
	}

	// 同步更新 KeyEntities（兼容旧逻辑）
	state.KeyEntities = make([]string, 0)
	for _, e := range state.ExtractedEntities {
		state.KeyEntities = append(state.KeyEntities, e.Name)
	}

	// 3. 更新主题（简单规则：使用最新的关键词）
	if message.Role == schema.User && len(content) > 0 {
		// 简单提取前10个字符作为主题
		if len(content) > 10 {
			state.Topic = content[:10] + "..."
		} else {
			state.Topic = content
		}
	}
}

// IntentDetection 意图检测结果
type IntentDetection struct {
	Primary    string
	Confidence float64
}

// detectIntent 检测用户意图
func (m *SessionStateManager) detectIntent(content string) IntentDetection {
	content = strings.ToLower(content)

	// 问题意图
	if strings.Contains(content, "？") || strings.Contains(content, "?") ||
		strings.Contains(content, "什么") || strings.Contains(content, "怎么") ||
		strings.Contains(content, "为什么") || strings.Contains(content, "哪里") {
		return IntentDetection{Primary: "question", Confidence: 0.8}
	}

	// 请求意图
	if strings.Contains(content, "帮我") || strings.Contains(content, "请") ||
		strings.Contains(content, "能不能") || strings.Contains(content, "可以") {
		return IntentDetection{Primary: "request", Confidence: 0.7}
	}

	// 创建意图
	if strings.Contains(content, "创建") || strings.Contains(content, "新建") ||
		strings.Contains(content, "添加") || strings.Contains(content, "写一个") {
		return IntentDetection{Primary: "create", Confidence: 0.7}
	}

	// 修改意图
	if strings.Contains(content, "修改") || strings.Contains(content, "更新") ||
		strings.Contains(content, "改变") || strings.Contains(content, "调整") {
		return IntentDetection{Primary: "modify", Confidence: 0.7}
	}

	// 删除意图
	if strings.Contains(content, "删除") || strings.Contains(content, "移除") ||
		strings.Contains(content, "去掉") {
		return IntentDetection{Primary: "delete", Confidence: 0.7}
	}

	// 查询意图
	if strings.Contains(content, "查看") || strings.Contains(content, "显示") ||
		strings.Contains(content, "列出") || strings.Contains(content, "查询") {
		return IntentDetection{Primary: "query", Confidence: 0.7}
	}

	// 默认：陈述
	return IntentDetection{Primary: "statement", Confidence: 0.5}
}

// EntityInfo 提取的实体信息
type EntityInfo struct {
	Name string
	Type string
}

// extractEntities 提取关键实体
func (m *SessionStateManager) extractEntities(content string) []EntityInfo {
	var entities []EntityInfo

	// 1. 提取引号内容
	start := 0
	for i, ch := range content {
		if isQuote(ch) {
			if start == 0 {
				start = i + 1
			} else {
				entity := content[start:i]
				if entity != "" {
					entities = append(entities, EntityInfo{
						Name: entity,
						Type: "quoted",
					})
				}
				start = 0
			}
		}
	}

	// 2. 提取常见实体模式
	// 人名模式：张三、李四等
	namePatterns := []string{"张三", "李四", "王五", "赵六"}
	for _, name := range namePatterns {
		if strings.Contains(content, name) {
			entities = append(entities, EntityInfo{
				Name: name,
				Type: "person",
			})
		}
	}

	// 地点模式：北京、上海等
	locationPatterns := []string{"北京", "上海", "广州", "深圳", "杭州"}
	for _, location := range locationPatterns {
		if strings.Contains(content, location) {
			entities = append(entities, EntityInfo{
				Name: location,
				Type: "location",
			})
		}
	}

	// 技术模式：Python、Go、Java等
	techPatterns := []string{"Python", "Go", "Java", "JavaScript", "TypeScript", "React", "Vue"}
	for _, tech := range techPatterns {
		if strings.Contains(content, tech) {
			entities = append(entities, EntityInfo{
				Name: tech,
				Type: "technology",
			})
		}
	}

	return entities
}

// isQuote 判断字符是否为引号（支持 ASCII、中文左/右双引号）
func isQuote(ch rune) bool {
	return ch == '"' || ch == '\u201C' || ch == '\u201D'
}

// NewSessionState 创建新的会话状态
func NewSessionState() *SessionState {
	return &SessionState{
		Topic:             "",
		UserIntent:        "",
		KeyEntities:       make([]string, 0),
		ExtractedEntities: make([]Entity, 0),
		IntentHistory:     make([]Intent, 0),
		Metadata:          make(map[string]string),
	}
}

// stateExtractionResult LLM 抽取结果
type stateExtractionResult struct {
	Topic    string       `json:"topic"`
	Intent   string       `json:"intent"`
	Entities []EntityInfo `json:"entities"`
}

// ExtractStateAsync 异步用 LLM 抽取会话状态，抽取成功后通过 callback 返回新的状态
// modelID 指定使用的模型，0 时不抽取；未配置 modelResolver 时也不抽取
func (m *SessionStateManager) ExtractStateAsync(ctx context.Context, messages []BufferMessage, modelID uint, callback func(*SessionState)) {
	go func() {
		if !m.HasModel(modelID) {
			return
		}
		extractCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		result, err := m.extractWithLLM(extractCtx, messages, modelID)
		if err != nil {
			m.logger.Warn("LLM 状态抽取失败，保留现有状态", zap.Error(err))
			return
		}
		if callback != nil {
			callback(m.buildStateFromResult(result))
		}
	}()
}

// extractWithLLM 用 LLM 抽取会话状态
func (m *SessionStateManager) extractWithLLM(ctx context.Context, messages []BufferMessage, modelID uint) (*stateExtractionResult, error) {
	client, err := m.modelResolver.CreateLLMClient(ctx, modelID)
	if err != nil {
		return nil, fmt.Errorf("创建 LLM 客户端失败: %w", err)
	}

	promptMessages := m.buildStateExtractionPrompt(messages)
	resp, err := client.Generate(ctx, promptMessages)
	if err != nil {
		return nil, fmt.Errorf("LLM 状态抽取失败: %w", err)
	}

	content := stripJSONCodeFence(resp.Content)
	if content == "" {
		return nil, fmt.Errorf("LLM 返回空内容")
	}

	var result stateExtractionResult
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, fmt.Errorf("解析 LLM 状态抽取结果失败: %w, raw=%s", err, content)
	}
	return &result, nil
}

// buildStateExtractionPrompt 构建状态抽取的 prompt
func (m *SessionStateManager) buildStateExtractionPrompt(messages []BufferMessage) []*schema.Message {
	systemPrompt := `你是一个对话状态分析助手。请分析给定的对话，提取当前会话状态，并以 JSON 格式返回。
要求：
1. topic: 当前讨论主题，10-20字概括
2. intent: 用户主要意图，从以下选择：question/request/create/modify/delete/query/statement
3. entities: 关键实体列表，每个实体含 name 和 type（type 可为 person/location/technology/quoted/other）

只返回 JSON，不要任何解释。格式：
{"topic":"...","intent":"...","entities":[{"name":"...","type":"..."}]}

如果对话中无明显实体，entities 返回空数组。`

	var parts []string
	for _, msg := range messages {
		role := "用户"
		if msg.Role == "assistant" {
			role = "助手"
		}
		parts = append(parts, fmt.Sprintf("[%s] %s", role, msg.Content))
	}
	conversation := strings.Join(parts, "\n")

	return []*schema.Message{
		{Role: schema.System, Content: systemPrompt},
		{Role: schema.User, Content: "对话内容：\n" + conversation + "\n\n请提取会话状态(JSON)："},
	}
}

// buildStateFromResult 将抽取结果构建为完整的 SessionState
func (m *SessionStateManager) buildStateFromResult(result *stateExtractionResult) *SessionState {
	now := time.Now()
	state := NewSessionState()
	state.Topic = result.Topic
	state.UserIntent = result.Intent
	state.IntentHistory = []Intent{
		{Primary: result.Intent, Confidence: 0.9, Timestamp: now},
	}
	for _, e := range result.Entities {
		state.ExtractedEntities = append(state.ExtractedEntities, Entity{
			Name:      e.Name,
			Type:      e.Type,
			FirstSeen: now,
			LastSeen:  now,
		})
		state.KeyEntities = append(state.KeyEntities, e.Name)
	}
	return state
}

// stripJSONCodeFence 去除 LLM 可能加的 ```json ... ``` 包裹
func stripJSONCodeFence(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		if idx := strings.IndexByte(s, '\n'); idx >= 0 {
			s = s[idx+1:]
		}
		s = strings.TrimSpace(s)
		s = strings.TrimSuffix(s, "```")
		s = strings.TrimSpace(s)
	}
	return s
}
