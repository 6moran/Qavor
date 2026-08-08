package skill

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.uber.org/zap"

	"Qavor/pkg/logger"
)

// SkillLoader Skill 文件加载器接口
type SkillLoader interface {
	ScanAll() ([]*SkillMeta, error)
	LoadMeta(slug string) (*SkillMeta, error)
	LoadPrompt(slug string) (string, error)
	GetSkillDir(slug string) string
}

// FileLoader 基于文件系统的 SkillLoader 实现
type FileLoader struct {
	skillsDir string
}

// NewFileLoader 创建 FileLoader
func NewFileLoader(skillsDir string) *FileLoader {
	return &FileLoader{skillsDir: skillsDir}
}

// ScanAll 扫描 skills_dir 下所有子目录，返回所有 SkillMeta
func (l *FileLoader) ScanAll() ([]*SkillMeta, error) {
	entries, err := os.ReadDir(l.skillsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("读取 skills 目录失败: %w", err)
	}

	var skills []*SkillMeta
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		meta, err := l.LoadMeta(entry.Name())
		if err != nil {
			logger.Warn("加载 Skill 失败，跳过",
				zap.String("slug", entry.Name()),
				zap.Error(err),
			)
			continue
		}
		if meta != nil {
			skills = append(skills, meta)
		}
	}
	return skills, nil
}

// LoadMeta 读取指定 slug 的 SKILL.md，解析 frontmatter
func (l *FileLoader) LoadMeta(slug string) (*SkillMeta, error) {
	skillDir := filepath.Join(l.skillsDir, slug)
	skillFile := filepath.Join(skillDir, "SKILL.md")

	data, err := os.ReadFile(skillFile)
	if err != nil {
		return nil, fmt.Errorf("读取 SKILL.md 失败: %w", err)
	}

	content := string(data)
	meta, err := parseFrontmatter(content)
	if err != nil {
		return nil, fmt.Errorf("解析 frontmatter 失败: %w", err)
	}

	meta.Slug = slug
	return meta, nil
}

// LoadPrompt 读取指定 slug 的 SKILL.md 正文部分
func (l *FileLoader) LoadPrompt(slug string) (string, error) {
	skillDir := filepath.Join(l.skillsDir, slug)
	skillFile := filepath.Join(skillDir, "SKILL.md")

	data, err := os.ReadFile(skillFile)
	if err != nil {
		return "", fmt.Errorf("读取 SKILL.md 失败: %w", err)
	}

	content := string(data)
	_, body, err := splitFrontmatter(content)
	if err != nil {
		return "", fmt.Errorf("解析 frontmatter 失败: %w", err)
	}

	return strings.TrimSpace(body), nil
}

// GetSkillDir 获取 skill 目录路径
func (l *FileLoader) GetSkillDir(slug string) string {
	return filepath.Join(l.skillsDir, slug)
}

// parseFrontmatter 解析 frontmatter 和正文
func parseFrontmatter(content string) (*SkillMeta, error) {
	frontmatter, _, err := splitFrontmatter(content)
	if err != nil {
		return nil, err
	}

	meta := &SkillMeta{}

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
		case "name":
			meta.Name = value
		case "description":
			meta.Description = value
		case "version":
			meta.Version = value
		case "author":
			meta.Author = value
		}
	}

	return meta, nil
}

// splitFrontmatter 分离 frontmatter 和正文
func splitFrontmatter(content string) (string, string, error) {
	content = strings.TrimPrefix(content, "\xef\xbb\xbf") // 移除 BOM

	if !strings.HasPrefix(content, "---") {
		return "", content, nil
	}

	endIdx := strings.Index(content[3:], "---")
	if endIdx == -1 {
		return "", "", fmt.Errorf("未找到 frontmatter 结束标记")
	}

	frontmatter := content[3 : endIdx+3]
	body := content[endIdx+6:]

	return frontmatter, body, nil
}

// parseListValue 解析 YAML 列表值（支持单行和多行）
func parseListValue(value string) []string {
	value = strings.TrimSpace(value)
	if value == "[]" || value == "" {
		return []string{}
	}

	// 单行格式：[item1, item2]
	if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
		value = strings.Trim(value, "[]")
		items := strings.Split(value, ",")
		var result []string
		for _, item := range items {
			item = strings.TrimSpace(item)
			item = strings.Trim(item, "\"'")
			if item != "" {
				result = append(result, item)
			}
		}
		return result
	}

	// 单个值
	value = strings.Trim(value, "\"'")
	if value != "" {
		return []string{value}
	}

	return []string{}
}

// NewLoader 创建 SkillLoader，优先使用 FileLoader
func NewLoader(skillsDir string) SkillLoader {
	return NewFileLoader(skillsDir)
}
