package skill

import (
	"context"
	"fmt"
	"strings"

	"Qavor/pkg/logger"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

// SkillsMiddleware Skill 中间件。
//
// 职责：
//   - BuildPrompt：L1 渐进式披露，将可用 skill 列表注入系统提示词
//   - BeforeModelRewriteState：扫描消息历史中的 read_file 工具调用，
//     检测到 skills/<slug>/SKILL.md 读取时打印日志"skill [slug] 已激活"
//
// 不涉及工具门控或激活释放——模型读取 SKILL.md 后自然知道如何使用 skill。
type SkillsMiddleware struct {
	*adk.TypedBaseChatModelAgentMiddleware[*schema.Message]
	loader SkillLoader
}

// NewSkillsMiddleware 创建 SkillsMiddleware
func NewSkillsMiddleware(loader SkillLoader) *SkillsMiddleware {
	return &SkillsMiddleware{
		loader: loader,
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

// BeforeModelRewriteState 在每次模型调用前扫描消息历史，
// 检测 read_file 工具调用是否指向 skills/<slug>/SKILL.md，
// 命中时打印"skill [slug] 已激活"日志。
func (m *SkillsMiddleware) BeforeModelRewriteState(ctx context.Context, state *adk.TypedChatModelAgentState[*schema.Message], _ *adk.TypedModelContext[*schema.Message]) (context.Context, *adk.TypedChatModelAgentState[*schema.Message], error) {
	for _, msg := range state.Messages {
		if msg.Role != schema.Assistant {
			continue
		}
		for _, tc := range msg.ToolCalls {
			if tc.Function.Name != "read_file" {
				continue
			}
			// read_file 只有一个参数 file_path
			slug := extractSkillSlug(tc.Function.Arguments)
			if slug != "" {
				logger.Info("Skill 已激活", zap.String("slug", slug))
			}
		}
	}
	return ctx, state, nil
}

// extractSkillSlug 从 read_file 的参数 JSON 中提取 skill slug。
// 参数格式：{"file_path": "skills/kb/SKILL.md"} 或 {"file_path":"skills/kb/SKILL.md"}
// 返回 slug（如 "kb"），不匹配时返回空串。
func extractSkillSlug(argsJSON string) string {
	// 简单字符串匹配而非 JSON 解析——参数通常为内联 JSON 且仅含 file_path 字段
	// 匹配 "file_path": "skills/<slug>/SKILL.md"
	const prefix = `"file_path":`
	idx := strings.Index(argsJSON, prefix)
	if idx < 0 {
		return ""
	}
	rest := argsJSON[idx+len(prefix):]
	// 跳过空白和冒号后的引号
	rest = strings.TrimSpace(rest)
	if !strings.HasPrefix(rest, `"`) {
		return ""
	}
	rest = rest[1:] // 跳过开头的引号

	// 找 skills/ 前缀
	const skillsPrefix = "skills/"
	spIdx := strings.Index(rest, skillsPrefix)
	if spIdx < 0 {
		return ""
	}
	afterSkills := rest[spIdx+len(skillsPrefix):]
	// 提取 slug（到下一个 /）
	slashIdx := strings.Index(afterSkills, "/")
	if slashIdx < 0 {
		return ""
	}
	slug := afterSkills[:slashIdx]
	if slug == "" {
		return ""
	}
	// 验证后缀是 SKILL.md
	if !strings.HasPrefix(afterSkills[slashIdx:], "/SKILL.md") && !strings.HasPrefix(afterSkills[slashIdx:], "/SKILL.md\"") {
		return ""
	}
	return slug
}

// maxSkillDescLen Skill 描述在提示词中的最大长度（避免长描述占用过多上下文）
const maxSkillDescLen = 200

// BuildPrompt 构建 Skill 提示词（L1 披露）。
// 披露 slug 与描述（截断），让模型理解每个 Skill 的用途，
// 并在需要时通过 read_file 读取对应 SKILL.md。
func (m *SkillsMiddleware) BuildPrompt(
	ctx context.Context,
	basePrompt string,
	skills []*SkillMeta,
) (string, error) {
	if len(skills) == 0 {
		return basePrompt, nil
	}

	var skillLines []string
	for _, s := range skills {
		desc := s.Description
		if r := []rune(desc); len(r) > maxSkillDescLen {
			desc = string(r[:maxSkillDescLen]) + "…"
		}
		if desc == "" {
			skillLines = append(skillLines, fmt.Sprintf("- %s", s.Slug))
		} else {
			skillLines = append(skillLines, fmt.Sprintf("- %s: %s", s.Slug, desc))
		}
	}

	skillSection := fmt.Sprintf(`
## 可用 Skills

你可以按需使用以下 Skill。注意：Skill 不是工具，不能直接调用 Skill 的名字。必须先读取对应 SKILL.md，了解用法后，再按其指导使用已有的工具。

%s

使用步骤：
1. 调用 read_file(path="skills/<slug>/SKILL.md") 读取 Skill 说明
2. 按说明使用已有的工具（如 execute、read_file 等）完成任务
3. 不要调用 Skill 名字本身——它不是工具
`, strings.Join(skillLines, "\n"))

	return basePrompt + "\n" + skillSection, nil
}
