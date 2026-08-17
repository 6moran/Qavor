package skill

// SkillMeta 从 SKILL.md 解析的元数据
type SkillMeta struct {
	Slug          string `json:"slug"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	Version       string `json:"version,omitempty"`
	Author        string `json:"author,omitempty"`
	PromptContent string `json:"-"` // 系统提示词内容
}
