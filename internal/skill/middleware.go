package skill

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"

	"Qavor/pkg/logger"
)

// SkillsMiddleware Skill 中间件，扫描消息历史检测 SKILL.md 读取，触发激活
//
// 实现 eino ChatModelAgentMiddleware 接口，在 BeforeModelRewriteState 钩子中：
// 1. 扫描 Assistant 消息中的 read_file 工具调用
// 2. 检测 file_path 是否指向 skills/<slug>/SKILL.md
// 3. 激活对应 skill，写入 run-local，供 ToolFilterMiddleware 释放依赖工具
type SkillsMiddleware struct {
	*adk.TypedBaseChatModelAgentMiddleware[*schema.Message]
	loader     SkillLoader
	activation *ActivationState
}

// NewSkillsMiddleware 创建 SkillsMiddleware
func NewSkillsMiddleware(loader SkillLoader, activation *ActivationState) *SkillsMiddleware {
	return &SkillsMiddleware{
		loader:     loader,
		activation: activation,
	}
}

// GetLoader 获取 SkillLoader
func (m *SkillsMiddleware) GetLoader() SkillLoader {
	return m.loader
}

// BeforeAgent 初始化基础中间件
func (m *SkillsMiddleware) BeforeAgent(ctx context.Context, runCtx *adk.ChatModelAgentContext[*schema.Message]) (context.Context, *adk.ChatModelAgentContext[*schema.Message], error) {
	return ctx, runCtx, nil
}

// AfterAgent 无需处理
func (m *SkillsMiddleware) AfterAgent(ctx context.Context, state *adk.TypedChatModelAgentState[*schema.Message]) (context.Context, error) {
	return ctx, nil
}

// BeforeModelRewriteState 在每次模型调用前扫描消息历史，检测 SKILL.md 读取
func (m *SkillsMiddleware) BeforeModelRewriteState(
	ctx context.Context,
	state *adk.TypedChatModelAgentState[*schema.Message],
	_ *adk.TypedModelContext[*schema.Message],
) (context.Context, *adk.TypedChatModelAgentState[*schema.Message], error) {
	// 从消息历史中检测 read_file 读取 SKILL.md 的工具调用
	activated := m.detectSkillReads(state.Messages)
	if len(activated) == 0 {
		return ctx, state, nil
	}

	// 合并已激活的 skill
	existing := m.activation.GetActivated()
	merged := dedup(append(existing, activated...))
	for _, slug := range activated {
		m.activation.Activate(slug)
	}

	// 写入 agent 运行时上下文，ToolFilterMiddleware 读取并释放对应工具
	_ = adk.SetRunLocalValue(ctx, "activated_skills", merged)

	if logger.Initialized() {
		logger.Info("Skill 已激活", zap.Strings("slugs", activated))
	}

	return ctx, state, nil
}

// detectSkillReads 扫描消息历史，找出读取了 SKILL.md 的 skill
func (m *SkillsMiddleware) detectSkillReads(messages []*schema.Message) []string {
	var activated []string
	seen := make(map[string]bool)

	for _, msg := range messages {
		if msg.Role != schema.Assistant {
			continue
		}
		for _, tc := range msg.ToolCalls {
			if tc.Function.Name != "read_file" {
				continue
			}
			slug := m.parseSkillSlugFromArgs(tc.Function.Arguments)
			if slug == "" || seen[slug] || !m.isVisibleSlug(slug) {
				continue
			}
			seen[slug] = true
			activated = append(activated, slug)
		}
	}
	return activated
}

// parseSkillSlugFromArgs 从 read_file 参数中解析 skill slug
func (m *SkillsMiddleware) parseSkillSlugFromArgs(argsJSON string) string {
	if argsJSON == "" {
		return ""
	}

	var args map[string]any
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return ""
	}

	filePath, _ := args["file_path"].(string)
	if filePath == "" {
		// 兼容其他参数名
		filePath, _ = args["path"].(string)
	}
	if filePath == "" {
		return ""
	}

	return m.extractSkillSlug(filePath)
}

// BuildPrompt 构建 Skill 提示词（L1 披露）
func (m *SkillsMiddleware) BuildPrompt(
	ctx context.Context,
	basePrompt string,
	skills []*SkillMeta,
) (string, error) {
	if len(skills) == 0 {
		return basePrompt, nil
	}

	var skillNames []string
	for _, s := range skills {
		skillNames = append(skillNames, s.Name)
	}

	skillSection := fmt.Sprintf(`
## 可用 Skills

你有以下 Skill 可用：%s

使用方法：
1. 先调用 read_file(path="skills/<slug>/SKILL.md") 读取 Skill 说明
2. 读取后即可使用该 Skill 提供的工具
3. 每个 Skill 的工具只有在读取其 SKILL.md 后才能使用
`, strings.Join(skillNames, "、"))

	return basePrompt + "\n" + skillSection, nil
}

// extractSkillSlug 从文件路径提取 slug
func (m *SkillsMiddleware) extractSkillSlug(filePath string) string {
	// 支持多种路径格式：skills/kb/SKILL.md, ./skills/kb/SKILL.md
	parts := strings.ReplaceAll(filePath, "\\", "/")
	parts = strings.ReplaceAll(parts, "./", "")

	// 必须是读取 SKILL.md 文件
	if !strings.HasSuffix(parts, "SKILL.md") {
		return ""
	}

	segments := strings.Split(parts, "/")

	for i, segment := range segments {
		if segment == "skills" && i+1 < len(segments) {
			slug := segments[i+1]
			if slug != "SKILL.md" {
				return slug
			}
		}
	}
	return ""
}

// isVisibleSlug 检查 slug 是否在可见列表中
func (m *SkillsMiddleware) isVisibleSlug(slug string) bool {
	meta, err := m.loader.LoadMeta(slug)
	if err != nil {
		return false
	}
	return meta != nil
}
