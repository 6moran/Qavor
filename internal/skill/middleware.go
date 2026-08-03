package skill

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/adk"
	"go.uber.org/zap"

	"Qavor/pkg/logger"
)

// SkillsMiddleware Skill 中间件，拦截 read_file 调用，检测 SKILL.md 读取，触发激活
type SkillsMiddleware struct {
	loader     SkillLoader
	resolver   SkillResolver
	activation *ActivationState
}

// NewSkillsMiddleware 创建 SkillsMiddleware
func NewSkillsMiddleware(loader SkillLoader, resolver SkillResolver, activation *ActivationState) *SkillsMiddleware {
	return &SkillsMiddleware{
		loader:     loader,
		resolver:   resolver,
		activation: activation,
	}
}

// ProcessToolCall 处理工具调用，检测 SKILL.md 读取
func (m *SkillsMiddleware) ProcessToolCall(
	ctx context.Context,
	toolName string,
	args map[string]any,
	result string,
) (string, error) {
	if toolName != "read_file" {
		return result, nil
	}

	filePath, ok := args["file_path"].(string)
	if !ok || !strings.HasSuffix(filePath, "/SKILL.md") {
		return result, nil
	}

	slug := m.extractSkillSlug(filePath)
	if slug == "" {
		return result, nil
	}

	if !m.isVisibleSlug(slug) {
		return result, nil
	}

	m.activation.Activate(slug)

	// 将已激活的技能 slug 注入 agent 运行时上下文
	// ToolFilterMiddleware.BeforeModelRewriteState 会读取并释放对应工具
	activated := m.activation.GetActivated()
	_ = adk.SetRunLocalValue(ctx, "activated_skills", activated)

	if logger.Initialized() {
		logger.Info("Skill 已激活", zap.String("slug", slug))
	}

	return result, nil
}

// BuildPrompt 构建 Skill 提示词
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
	segments := strings.Split(parts, "/")

	for i, segment := range segments {
		if segment == "skills" && i+1 < len(segments) {
			slug := segments[i+1]
			if strings.HasSuffix(slug, "SKILL.md") {
				slug = strings.TrimSuffix(slug, "/SKILL.md")
				slug = strings.TrimSuffix(slug, "SKILL.md")
			}
			if slug != "" {
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
