package response

import "time"

// SkillResponse 技能响应
type SkillResponse struct {
	ID          uint      `json:"id"`
	Slug        string    `json:"slug"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	SourceType  string    `json:"source_type"`
	DirPath     string    `json:"dir_path"`
	Version     string    `json:"version,omitempty"`
	ContentHash string    `json:"content_hash,omitempty"`
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// SkillListResponse 技能列表响应
type SkillListResponse struct {
	Total int64           `json:"total"`
	Items []SkillResponse `json:"items"`
}
