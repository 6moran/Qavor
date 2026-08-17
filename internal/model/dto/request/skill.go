package request

// CreateSkillRequest 创建技能请求
type CreateSkillRequest struct {
	Slug        string `json:"slug" binding:"required,max=128"`
	Name        string `json:"name" binding:"required,max=128"`
	Description string `json:"description" binding:"required"`
	SourceType  string `json:"source_type" binding:"required,oneof=builtin upload remote"`
	DirPath     string `json:"dir_path" binding:"required,max=512"`
	Version     string `json:"version" binding:"omitempty,max=64"`
}

// UpdateSkillRequest 更新技能请求
type UpdateSkillRequest struct {
	Name        string `json:"name" binding:"omitempty,max=128"`
	Description string `json:"description" binding:"omitempty"`
	Version     string `json:"version" binding:"omitempty,max=64"`
	Enabled     *bool  `json:"enabled" binding:"omitempty"`
}

// SkillListRequest 技能列表请求
type SkillListRequest struct {
	Page       int    `form:"page" binding:"omitempty,min=1"`
	PageSize   int    `form:"page_size" binding:"omitempty,min=1,max=100"`
	Keyword    string `form:"keyword" binding:"omitempty"`
	SourceType string `form:"source_type" binding:"omitempty,oneof=builtin upload remote"`
}
