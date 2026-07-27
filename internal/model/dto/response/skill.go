package response

import (
	"Qavor/internal/model/entity"
	"time"
)

// SkillResponse 技能响应
type SkillResponse struct {
	ID                uint             `json:"id"`
	Slug              string           `json:"slug"`
	Name              string           `json:"name"`
	Description       string           `json:"description"`
	SourceType        string           `json:"source_type"`
	ToolDependencies  entity.JSONArray `json:"tool_dependencies"`
	MCPDependencies   entity.JSONArray `json:"mcp_dependencies"`
	SkillDependencies entity.JSONArray `json:"skill_dependencies"`
	DirPath           string           `json:"dir_path"`
	Version           string           `json:"version,omitempty"`
	ContentHash       string           `json:"content_hash,omitempty"`
	ShareConfig       entity.JSON      `json:"share_config"`
	Enabled           bool             `json:"enabled"`
	CreatedBy         string           `json:"created_by,omitempty"`
	UpdatedBy         string           `json:"updated_by,omitempty"`
	CreatedAt         time.Time        `json:"created_at"`
	UpdatedAt         time.Time        `json:"updated_at"`
}

// SkillListResponse 技能列表响应
type SkillListResponse struct {
	Total int64           `json:"total"`
	Items []SkillResponse `json:"items"`
}
