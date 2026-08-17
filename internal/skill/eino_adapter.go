package skill

import (
	"context"
	"fmt"
	"path/filepath"

	einoskill "github.com/cloudwego/eino/adk/middlewares/skill"
)

// EinoSkillAdapter 封装 eino skill Backend，提供 SkillLoader 接口适配。
type EinoSkillAdapter struct {
	backend einoskill.Backend
	baseDir string
}

// NewEinoSkillAdapter 创建 EinoSkillAdapter。
//
// backend 是 eino 的 skill 后端实现；baseDir 是 skill 目录的根路径，用于 GetSkillDir。
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
		skills = append(skills, &SkillMeta{
			Slug:        fm.Name,
			Name:        fm.Name,
			Description: fm.Description,
		})
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

	return &SkillMeta{
		Slug:          skill.Name,
		Name:          skill.Name,
		Description:   skill.Description,
		PromptContent: skill.Content,
	}, nil
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
