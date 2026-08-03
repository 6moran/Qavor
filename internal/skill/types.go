package skill

// SkillMeta 从 SKILL.md 解析的元数据
type SkillMeta struct {
	Slug              string   `json:"slug"`
	Name              string   `json:"name"`
	Description       string   `json:"description"`
	ToolDependencies  []string `json:"tool_dependencies"`
	MCPDependencies   []string `json:"mcp_dependencies"`
	SkillDependencies []string `json:"skill_dependencies"`
	Version           string   `json:"version,omitempty"`
	Author            string   `json:"author,omitempty"`
	PromptContent     string   `json:"-"` // 系统提示词内容
}

// DependencyBundle 运行时依赖包
type DependencyBundle struct {
	ToolNames []string
	MCPNames  []string
}

// dedup 去重字符串切片
func dedup(items []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, item := range items {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}
	return result
}
