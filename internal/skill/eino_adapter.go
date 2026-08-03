package skill

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	einoskill "github.com/cloudwego/eino/adk/middlewares/skill"
	"go.uber.org/zap"

	"Qavor/pkg/logger"
)

// EinoSkillAdapter 封装 eino skill Backend，提供 SkillLoader 接口适配。
//
// eino 的 FrontMatter 仅包含 Name/Description/Context/Agent/Model，
// 而项目的 SkillMeta 还需要 ToolDependencies/MCPDependencies/SkillDependencies。
// 适配器会从 SKILL.md 的 frontmatter 中解析额外的依赖字段。
type EinoSkillAdapter struct {
	backend einoskill.Backend
	baseDir string
}

// NewEinoSkillAdapter 创建 EinoSkillAdapter。
//
// backend 是 eino 的 skill 后端实现；baseDir 是 skill 目录的根路径，
// 用于 GetSkillDir 和直接读取 SKILL.md 文件以解析依赖。
func NewEinoSkillAdapter(backend einoskill.Backend, baseDir string) *EinoSkillAdapter {
	return &EinoSkillAdapter{
		backend: backend,
		baseDir: baseDir,
	}
}

// ScanAll 扫描所有 skill，返回 SkillMeta 列表。
func (a *EinoSkillAdapter) ScanAll() ([]*SkillMeta, error) {
	ctx := context.Background()
	frontMatters, err := a.backend.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("eino backend List 失败: %w", err)
	}

	var skills []*SkillMeta
	for _, fm := range frontMatters {
		meta := &SkillMeta{
			Slug:              fm.Name,
			Name:              fm.Name,
			Description:       fm.Description,
			ToolDependencies:  []string{},
			MCPDependencies:   []string{},
			SkillDependencies: []string{},
		}

		// 尝试从文件解析依赖
		if deps, err := a.parseDependenciesFromDisk(fm.Name); err == nil {
			meta.ToolDependencies = deps.ToolNames
			meta.MCPDependencies = deps.MCPNames
			meta.SkillDependencies = deps.SkillDeps
		} else if logger.Initialized() {
			logger.Debug("解析 Skill 依赖失败，使用空依赖",
				zap.String("slug", fm.Name),
				zap.Error(err),
			)
		}

		skills = append(skills, meta)
	}
	return skills, nil
}

// LoadMeta 加载指定 slug 的 SkillMeta。
func (a *EinoSkillAdapter) LoadMeta(slug string) (*SkillMeta, error) {
	ctx := context.Background()
	skill, err := a.backend.Get(ctx, slug)
	if err != nil {
		return nil, fmt.Errorf("eino backend Get 失败: %w", err)
	}

	meta := &SkillMeta{
		Slug:              skill.Name,
		Name:              skill.Name,
		Description:       skill.Description,
		PromptContent:     skill.Content,
		ToolDependencies:  []string{},
		MCPDependencies:   []string{},
		SkillDependencies: []string{},
	}

	// 从文件解析依赖
	if deps, err := a.parseDependenciesFromDisk(slug); err == nil {
		meta.ToolDependencies = deps.ToolNames
		meta.MCPDependencies = deps.MCPNames
		meta.SkillDependencies = deps.SkillDeps
	}

	return meta, nil
}

// LoadPrompt 加载指定 slug 的提示词内容。
func (a *EinoSkillAdapter) LoadPrompt(slug string) (string, error) {
	ctx := context.Background()
	skill, err := a.backend.Get(ctx, slug)
	if err != nil {
		return "", fmt.Errorf("eino backend Get 失败: %w", err)
	}
	return skill.Content, nil
}

// GetSkillDir 获取指定 slug 的 skill 目录路径。
func (a *EinoSkillAdapter) GetSkillDir(slug string) string {
	return filepath.Join(a.baseDir, slug)
}

// skillDependencies 从文件解析的依赖信息
type skillDependencies struct {
	ToolNames []string
	MCPNames  []string
	SkillDeps []string
}

// parseDependenciesFromDisk 从 SKILL.md 文件解析依赖字段。
// eino 的 FrontMatter 不包含 tool_dependencies/mcp_dependencies/skill_dependencies，
// 因此需要直接读取文件 frontmatter 来提取这些字段。
func (a *EinoSkillAdapter) parseDependenciesFromDisk(slug string) (*skillDependencies, error) {
	skillFile := filepath.Join(a.baseDir, slug, "SKILL.md")

	data, err := os.ReadFile(skillFile)
	if err != nil {
		return nil, fmt.Errorf("读取 SKILL.md 失败: %w", err)
	}

	content := string(data)
	frontmatter, _, err := splitFrontmatter(content)
	if err != nil {
		return nil, fmt.Errorf("解析 frontmatter 失败: %w", err)
	}

	deps := &skillDependencies{
		ToolNames: []string{},
		MCPNames:  []string{},
		SkillDeps: []string{},
	}

	lines := strings.Split(frontmatter, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "tool_dependencies":
			deps.ToolNames = parseListValue(value)
		case "mcp_dependencies":
			deps.MCPNames = parseListValue(value)
		case "skill_dependencies":
			deps.SkillDeps = parseListValue(value)
		}
	}

	return deps, nil
}
