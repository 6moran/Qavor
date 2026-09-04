package skill

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"Qavor/internal/model/entity"
)

// SkillService Skill 业务服务接口
type SkillService interface {
	List(offset, limit int, keyword string) ([]*entity.Skill, int64, error)
	GetBySlug(slug string) (*entity.Skill, error)
	Create(skill *entity.Skill) error
	Update(slug string, skill *entity.Skill) error
	Delete(slug string) error
	GetOptions() ([]*SkillOption, error)
	Import(slug string, data []byte) error
	Export(slug string) ([]byte, error)
	ListBuiltinSkills() ([]*entity.Skill, error)
}

// SkillOption 前端选项
type SkillOption struct {
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
}

type skillService struct {
	repo   SkillRepository
	loader SkillLoader
}

// NewSkillService 创建 SkillService
func NewSkillService(repo SkillRepository, loader SkillLoader) SkillService {
	return &skillService{repo: repo, loader: loader}
}

func (s *skillService) List(offset, limit int, keyword string) ([]*entity.Skill, int64, error) {
	return s.repo.List(offset, limit, keyword)
}

func (s *skillService) GetBySlug(slug string) (*entity.Skill, error) {
	return s.repo.FindBySlug(slug)
}

func (s *skillService) Create(skill *entity.Skill) error {
	existing, err := s.repo.FindBySlug(skill.Slug)
	if err != nil {
		return err
	}
	if existing != nil {
		return fmt.Errorf("Skill '%s' 已存在", skill.Slug)
	}
	return s.repo.Create(skill)
}

func (s *skillService) Update(slug string, skill *entity.Skill) error {
	existing, err := s.repo.FindBySlug(slug)
	if err != nil {
		return err
	}
	if existing == nil {
		return fmt.Errorf("Skill '%s' 不存在", slug)
	}
	skill.ID = existing.ID
	return s.repo.Update(skill)
}

func (s *skillService) Delete(slug string) error {
	existing, err := s.repo.FindBySlug(slug)
	if err != nil {
		return err
	}
	if existing == nil {
		return fmt.Errorf("Skill '%s' 不存在", slug)
	}

	// 删除文件
	skillDir := s.loader.GetSkillDir(slug)
	if _, statErr := os.Stat(skillDir); statErr == nil {
		if err := os.RemoveAll(skillDir); err != nil {
			return fmt.Errorf("删除 Skill 目录失败: %w", err)
		}
	}

	return s.repo.Delete(slug)
}

func (s *skillService) GetOptions() ([]*SkillOption, error) {
	skills, err := s.repo.ListAll()
	if err != nil {
		return nil, err
	}

	var options []*SkillOption
	for _, sk := range skills {
		options = append(options, &SkillOption{
			Slug:        sk.Slug,
			Name:        sk.Name,
			Description: sk.Description,
			Enabled:     sk.Enabled,
		})
	}
	return options, nil
}

// ListBuiltinSkills 列出内置 Skills
func (s *skillService) ListBuiltinSkills() ([]*entity.Skill, error) {
	return s.repo.ListAll()
}

// Import 导入 skill（从 zip 文件）
func (s *skillService) Import(slug string, data []byte) error {
	// 检查 skill 是否已存在
	existing, err := s.repo.FindBySlug(slug)
	if err != nil {
		return err
	}
	if existing != nil {
		return fmt.Errorf("Skill '%s' 已存在", slug)
	}

	// 创建 skill 目录
	skillDir := s.loader.GetSkillDir(slug)
	if err := createSkillFromZip(skillDir, data); err != nil {
		return fmt.Errorf("导入 skill 失败: %w", err)
	}

	// 创建数据库记录
	skill := &entity.Skill{
		Slug: slug,
		Name: slug,
	}
	return s.repo.Create(skill)
}

// Export 导出 skill（为 zip 文件）
func (s *skillService) Export(slug string) ([]byte, error) {
	skillDir := s.loader.GetSkillDir(slug)
	return createZipFromSkill(skillDir)
}

// createSkillFromZip 从 zip 数据解压到 skill 目录
// 支持 zip 内带单层公共顶层目录（如 my-skill/SKILL.md）或不带顶层目录（如 SKILL.md）。
func createSkillFromZip(skillDir string, data []byte) error {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return fmt.Errorf("解析 zip 失败: %w", err)
	}

	var names []string
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		names = append(names, filepath.ToSlash(f.Name))
	}
	top := commonTopDir(names)

	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}

		rel := filepath.ToSlash(f.Name)
		// 拒绝含 .. 路径段的条目，防止 zip-slip 路径穿越
		for _, seg := range strings.Split(rel, "/") {
			if seg == ".." {
				return fmt.Errorf("非法路径: %s", f.Name)
			}
		}
		if top != "" {
			rel = strings.TrimPrefix(rel, top)
		}
		rel = filepath.FromSlash(rel)
		if rel == "" || rel == "." {
			continue
		}

		dest := filepath.Join(skillDir, rel)
		// 防止 zip-slip 路径穿越
		if relTo, relErr := filepath.Rel(skillDir, dest); relErr != nil ||
			relTo == ".." || strings.HasPrefix(relTo, ".."+string(filepath.Separator)) {
			return fmt.Errorf("非法路径: %s", f.Name)
		}

		if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
			return fmt.Errorf("创建目录失败: %w", err)
		}

		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("读取 zip 条目 %s 失败: %w", f.Name, err)
		}
		content, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return fmt.Errorf("读取 zip 条目 %s 失败: %w", f.Name, err)
		}

		if err := os.WriteFile(dest, content, 0644); err != nil {
			return fmt.Errorf("写入文件 %s 失败: %w", rel, err)
		}
	}
	return nil
}

// createZipFromSkill 从 skill 目录打包 zip 数据
// zip 内条目带 skill 目录名作为顶层目录，保证可被 InstallFromZip 重新导入。
func createZipFromSkill(skillDir string) ([]byte, error) {
	entries, err := readSkillDir(skillDir)
	if err != nil {
		return nil, fmt.Errorf("读取 skill 目录失败: %w", err)
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("skill 目录为空")
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	prefix := filepath.Base(skillDir)
	for rel, content := range entries {
		name := filepath.ToSlash(filepath.Join(prefix, rel))
		w, err := zw.Create(name)
		if err != nil {
			return nil, fmt.Errorf("创建 zip 条目 %s 失败: %w", name, err)
		}
		if _, err := w.Write(content); err != nil {
			return nil, fmt.Errorf("写入 zip 条目 %s 失败: %w", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("关闭 zip 失败: %w", err)
	}
	return buf.Bytes(), nil
}
