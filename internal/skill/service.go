package skill

import (
	"fmt"

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
	GetDependencyOptions(slug string) (*DependencyOptions, error)
	ListBuiltinSkills() ([]*entity.Skill, error)
	SyncBuiltinSkills() error
}

// DependencyOptions 依赖选项
type DependencyOptions struct {
	Tools []ToolOption `json:"tools"`
	MCPs  []MCPOption  `json:"mcps"`
}

// ToolOption 工具选项
type ToolOption struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// MCPOption MCP 选项
type MCPOption struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// SkillOption 前端选项
type SkillOption struct {
	Slug         string            `json:"slug"`
	Name         string            `json:"name"`
	Description  string            `json:"description"`
	Dependencies *DependencyBundle `json:"dependencies"`
	Enabled      bool              `json:"enabled"`
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
	return s.repo.Delete(slug)
}

func (s *skillService) GetOptions() ([]*SkillOption, error) {
	skills, err := s.repo.ListAll()
	if err != nil {
		return nil, err
	}

	var options []*SkillOption
	for _, sk := range skills {
		option := &SkillOption{
			Slug:        sk.Slug,
			Name:        sk.Name,
			Description: sk.Description,
			Enabled:     sk.Enabled,
		}

		// 尝试从文件加载依赖信息
		meta, err := s.loader.LoadMeta(sk.Slug)
		if err == nil && meta != nil {
			option.Dependencies = &DependencyBundle{
				ToolNames: meta.ToolDependencies,
				MCPNames:  meta.MCPDependencies,
			}
		} else {
			option.Dependencies = &DependencyBundle{
				ToolNames: toStringSlice(sk.ToolDependencies),
				MCPNames:  toStringSlice(sk.MCPDependencies),
			}
		}

		options = append(options, option)
	}
	return options, nil
}

// toStringSlice 将 JSONArray 转换为 []string
func toStringSlice(arr entity.JSONArray) []string {
	var result []string
	for _, v := range arr {
		if s, ok := v.(string); ok {
			result = append(result, s)
		}
	}
	return result
}

// GetDependencyOptions 获取依赖选项
func (s *skillService) GetDependencyOptions(slug string) (*DependencyOptions, error) {
	options := &DependencyOptions{
		Tools: []ToolOption{},
		MCPs:  []MCPOption{},
	}

	// 如果指定了 slug，获取该 skill 的依赖信息
	if slug != "" {
		meta, err := s.loader.LoadMeta(slug)
		if err == nil && meta != nil {
			// 这里可以根据实际需求填充工具和 MCP 选项
			// 目前返回空列表，后续可以集成工具注册表
		}
	}

	return options, nil
}

// ListBuiltinSkills 列出内置 Skills
func (s *skillService) ListBuiltinSkills() ([]*entity.Skill, error) {
	return s.repo.ListAll()
}

// SyncBuiltinSkills 同步内置 Skills
func (s *skillService) SyncBuiltinSkills() error {
	// TODO: 实现内置 Skills 同步逻辑
	return nil
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

// createSkillFromZip 从 zip 数据创建 skill 目录
func createSkillFromZip(skillDir string, data []byte) error {
	// TODO: 实现 zip 解压逻辑
	return nil
}

// createZipFromSkill 从 skill 目录创建 zip 数据
func createZipFromSkill(skillDir string) ([]byte, error) {
	// TODO: 实现 zip 压缩逻辑
	return nil, nil
}
