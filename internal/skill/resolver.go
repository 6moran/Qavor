package skill

import (
	"fmt"

	"go.uber.org/zap"

	"Qavor/pkg/logger"
)

// SkillResolver Skill 依赖解析器接口
type SkillResolver interface {
	DFSClosure(slugs []string) (map[string]*SkillMeta, map[string]string, error)
	ResolveDependencies(slug string) (*DependencyBundle, error)
	ValidateDependencies(slug string) error
}

type skillResolver struct {
	loader       SkillLoader
	toolRegistry ToolRegistry
	mcpManager   MCPManager
}

// ToolRegistry 工具注册表接口
type ToolRegistry interface {
	Has(name string) bool
}

// MCPManager MCP 管理器接口
type MCPManager interface {
	Has(name string) bool
}

// NewResolver 创建 SkillResolver
func NewResolver(loader SkillLoader, toolRegistry ToolRegistry, mcpManager MCPManager) SkillResolver {
	return &skillResolver{
		loader:       loader,
		toolRegistry: toolRegistry,
		mcpManager:   mcpManager,
	}
}

// DFSClosure DFS 展开依赖闭包
func (r *skillResolver) DFSClosure(slugs []string) (
	skillIndex map[string]*SkillMeta,
	toolOwnership map[string]string,
	err error,
) {
	skillIndex = make(map[string]*SkillMeta)
	toolOwnership = make(map[string]string)
	visited := make(map[string]bool)

	var dfs func(slug string) error
	dfs = func(slug string) error {
		if visited[slug] {
			return nil
		}
		visited[slug] = true

		meta, err := r.loader.LoadMeta(slug)
		if err != nil {
			return fmt.Errorf("加载 Skill '%s' 失败: %w", slug, err)
		}
		skillIndex[slug] = meta

		for _, toolName := range meta.ToolDependencies {
			if existingOwner, exists := toolOwnership[toolName]; exists {
				logger.Warn("工具被多个 Skill 依赖",
					zap.String("tool", toolName),
					zap.String("existing_owner", existingOwner),
					zap.String("new_owner", slug),
				)
			}
			toolOwnership[toolName] = slug
		}

		for _, depSlug := range meta.SkillDependencies {
			if err := dfs(depSlug); err != nil {
				return err
			}
		}

		return nil
	}

	for _, slug := range slugs {
		if err := dfs(slug); err != nil {
			return nil, nil, err
		}
	}

	return skillIndex, toolOwnership, nil
}

// ResolveDependencies 递归解析单个 Skill 的所有依赖
func (r *skillResolver) ResolveDependencies(slug string) (*DependencyBundle, error) {
	return r.resolveDeps(slug, make(map[string]bool))
}

func (r *skillResolver) resolveDeps(slug string, visited map[string]bool) (*DependencyBundle, error) {
	if visited[slug] {
		return nil, fmt.Errorf("检测到循环依赖: %s", slug)
	}
	visited[slug] = true

	meta, err := r.loader.LoadMeta(slug)
	if err != nil {
		return nil, err
	}

	bundle := &DependencyBundle{
		ToolNames: append([]string{}, meta.ToolDependencies...),
		MCPNames:  append([]string{}, meta.MCPDependencies...),
	}

	for _, depSlug := range meta.SkillDependencies {
		depBundle, err := r.resolveDeps(depSlug, visited)
		if err != nil {
			return nil, err
		}
		bundle.ToolNames = append(bundle.ToolNames, depBundle.ToolNames...)
		bundle.MCPNames = append(bundle.MCPNames, depBundle.MCPNames...)
	}

	bundle.ToolNames = dedup(bundle.ToolNames)
	bundle.MCPNames = dedup(bundle.MCPNames)

	return bundle, nil
}

// ValidateDependencies 校验依赖是否存在
func (r *skillResolver) ValidateDependencies(slug string) error {
	meta, err := r.loader.LoadMeta(slug)
	if err != nil {
		return err
	}

	// 校验工具依赖是否存在
	if r.toolRegistry != nil {
		for _, toolName := range meta.ToolDependencies {
			if !r.toolRegistry.Has(toolName) {
				return fmt.Errorf("tool dependency not found: %s", toolName)
			}
		}
	}

	// 校验MCP依赖是否存在
	if r.mcpManager != nil {
		for _, mcpName := range meta.MCPDependencies {
			if !r.mcpManager.Has(mcpName) {
				return fmt.Errorf("mcp dependency not found: %s", mcpName)
			}
		}
	}

	// 校验skill依赖是否存在（检测循环依赖）
	visited := make(map[string]bool)
	if err := r.checkCircularDependency(slug, visited); err != nil {
		return err
	}

	return nil
}

func (r *skillResolver) checkCircularDependency(slug string, visited map[string]bool) error {
	if visited[slug] {
		return fmt.Errorf("circular dependency detected: %s", slug)
	}
	visited[slug] = true

	meta, err := r.loader.LoadMeta(slug)
	if err != nil {
		return err
	}

	for _, depSlug := range meta.SkillDependencies {
		if err := r.checkCircularDependency(depSlug, visited); err != nil {
			return err
		}
	}

	delete(visited, slug)
	return nil
}
