package skill

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
)

// SkillsMiddleware Skill 中间件，扫描消息历史检测 SKILL.md 读取，触发激活
//
// 实现 eino ChatModelAgentMiddleware 接口，在 BeforeModelRewriteState 钩子中：
// 1. 扫描 Assistant 消息中的 read_file 工具调用
// 2. 检测 file_path 是否指向 skills/<slug>/SKILL.md
// 3. 激活对应 skill，写入 run-local，供 ToolFilterMiddleware 释放依赖工具
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

// maxSkillDescLen Skill 描述在提示词中的最大长度（避免长描述占用过多上下文）
const maxSkillDescLen = 200

// BuildPrompt 构建 Skill 提示词（L1 披露）。
// 披露 slug 与描述（截断），让模型理解每个 Skill 的用途，
// 并在需要时通过 read_file 读取对应 SKILL.md（L2 激活）。
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
