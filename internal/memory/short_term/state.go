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

// TaskStateManager 任务状态管理器
// 优先用 LLM 抽取任务状态；未配置模型或抽取失败时降级为规则式
type TaskStateManager struct {
	logger        *zap.Logger
	modelResolver ModelResolver
}

// NewTaskStateManager 创建任务状态管理器
func NewTaskStateManager(logger *zap.Logger, modelResolver ModelResolver) *TaskStateManager {
	return &TaskStateManager{
		logger:        logger,
		modelResolver: modelResolver,
	}
}

// HasModel 是否配置了可用的抽取模型
func (m *TaskStateManager) HasModel(modelID uint) bool {
	return m.modelResolver != nil && modelID > 0
}

// UpdateState 规则式更新任务状态（降级或无模型时的主路径）
func (m *TaskStateManager) UpdateState(state *TaskState, message *schema.Message) {
	if state == nil {
		return
	}

	content := message.Content
	state.LastActivityAt = time.Now()

	if message.Role != schema.User {
		return
	}

	// 提取目标：如果用户消息包含目标性关键词，更新 goal
	if state.Goal == "" || containsGoalKeyword(content) {
		if goal := extractGoal(content); goal != "" {
			state.Goal = goal
		}
	}

	// 提取技术上下文：文件名、框架、工具
	techItems := extractTechContext(content)
	for _, item := range techItems {
		if !containsString(state.TechContext, item) {
			state.TechContext = append(state.TechContext, item)
		}
	}

	// 限制列表长度
	if len(state.TechContext) > 10 {
		state.TechContext = state.TechContext[len(state.TechContext)-10:]
	}
}

// ExtractTaskStateAsync 异步用 LLM 抽取任务状态
func (m *TaskStateManager) ExtractTaskStateAsync(ctx context.Context, messages []Message, modelID uint, callback func(*TaskState)) {
	go func() {
		if !m.HasModel(modelID) {
			return
		}
		extractCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		result, err := m.extractWithLLM(extractCtx, messages, modelID)
		if err != nil {
			m.logger.Warn("LLM 任务状态抽取失败，保留现有状态", zap.Error(err))
			return
		}
		if callback != nil {
			callback(m.buildTaskStateFromResult(result))
		}
	}()
}

// taskStateExtractionResult LLM 抽取结果
type taskStateExtractionResult struct {
	Goal           string   `json:"goal"`
	CompletedSteps []string `json:"completed_steps"`
	PendingSteps   []string `json:"pending_steps"`
	TechContext    []string `json:"tech_context"`
}

// extractWithLLM 用 LLM 抽取任务状态
func (m *TaskStateManager) extractWithLLM(ctx context.Context, messages []Message, modelID uint) (*taskStateExtractionResult, error) {
	client, err := m.modelResolver.CreateLLMClient(ctx, modelID)
	if err != nil {
		return nil, fmt.Errorf("创建 LLM 客户端失败: %w", err)
	}

	prompt := m.buildExtractionPrompt(messages)
	resp, err := client.Generate(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("LLM 任务状态抽取失败: %w", err)
	}

	content := stripJSONCodeFence(resp.Content)
	if content == "" {
		return nil, fmt.Errorf("LLM 返回空内容")
	}

	var result taskStateExtractionResult
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, fmt.Errorf("解析 LLM 任务状态结果失败: %w, raw=%s", err, content)
	}
	return &result, nil
}

// buildExtractionPrompt 构建任务状态抽取的 prompt
func (m *TaskStateManager) buildExtractionPrompt(messages []Message) []*schema.Message {
	systemPrompt := `你是一个任务状态分析助手。请分析对话，提取当前任务状态，以 JSON 格式返回。

要求：
1. goal: 当前任务目标（用户要完成什么），10-30字
2. completed_steps: 已完成的步骤列表
3. pending_steps: 待完成的步骤列表
4. tech_context: 相关技术上下文（涉及的文件、框架、工具、代码位置）

只返回 JSON，不要任何解释。格式：
{"goal":"...","completed_steps":["..."],"pending_steps":["..."],"tech_context":["..."]}

如果信息不足，对应字段返回空数组。`

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
		{Role: schema.User, Content: "对话内容：\n" + conversation + "\n\n请提取任务状态(JSON)："},
	}
}

// buildTaskStateFromResult 将 LLM 抽取结果构建为 TaskState
func (m *TaskStateManager) buildTaskStateFromResult(result *taskStateExtractionResult) *TaskState {
	state := NewTaskState()
	state.Goal = result.Goal
	state.CompletedSteps = result.CompletedSteps
	state.PendingSteps = result.PendingSteps
	state.TechContext = result.TechContext
	return state
}

// ---------- 规则式辅助函数 ----------

// containsGoalKeyword 检查是否包含目标性关键词
func containsGoalKeyword(content string) bool {
	keywords := []string{"我要", "我想", "帮我", "请", "需要", "做", "写", "实现", "完成", "修改", "添加", "删除"}
	for _, kw := range keywords {
		if strings.Contains(content, kw) {
			return true
		}
	}
	return false
}

// extractGoal 从用户消息中提取任务目标
func extractGoal(content string) string {
	// 取前30字作为目标概括
	if len(content) > 30 {
		content = content[:30]
	}
	return content
}

// extractTechContext 提取技术上下文（文件名、框架、工具）
func extractTechContext(content string) []string {
	var items []string

	// 文件名模式：xxx.go, xxx.py, xxx.js 等
	fileExts := []string{".go", ".py", ".js", ".ts", ".tsx", ".jsx", ".java", ".rs", ".sql", ".yaml", ".json", ".md"}
	for _, ext := range fileExts {
		idx := strings.Index(content, ext)
		if idx >= 0 {
			// 向前找到文件名开头
			start := idx
			for start > 0 && content[start-1] != ' ' && content[start-1] != '/' && content[start-1] != '\\' && content[start-1] != '`' {
				start--
			}
			fileName := content[start : idx+len(ext)]
			if fileName != "" && !containsString(items, fileName) {
				items = append(items, fileName)
			}
		}
	}

	// 技术术语
	techTerms := []string{
		"Redis", "PostgreSQL", "MySQL", "MongoDB",
		"Docker", "Kubernetes", "K8s",
		"Gin", "GORM", "Echo", "Fiber",
		"React", "Vue", "Angular", "Next.js",
		"AWS", "GCP", "Azure",
		"GPT", "Claude", "LLM", "OpenAI",
	}
	for _, term := range techTerms {
		if strings.Contains(content, term) && !containsString(items, term) {
			items = append(items, term)
		}
	}

	return items
}

// containsString 检查字符串切片是否包含指定字符串
func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
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
