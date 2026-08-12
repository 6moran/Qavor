package service

import "context"

// MindmapService 提供知识导图的查询、生成和增量更新能力。
type MindmapService interface {
	ListDatabases(ctx context.Context) ([]MindmapDatabaseDTO, error)
	ListFiles(ctx context.Context, kbID string) ([]MindmapFileDTO, error)
	Get(ctx context.Context, kbID string) (*MindmapDTO, error)
	GetDiff(ctx context.Context, kbID string) (*MindmapDiffDTO, error)
	Generate(ctx context.Context, kbID string, req *GenerateMindmapRequest) (*GenerateMindmapResponse, error)
}

// MindmapNode 是前端 Markmap 渲染所需的树节点。
type MindmapNode struct {
	Content  string         `json:"content"`
	Children []*MindmapNode `json:"children,omitempty"`
}

// MindmapDatabaseDTO 是可生成导图的知识库摘要。
type MindmapDatabaseDTO struct {
	KBID string `json:"kb_id"`
	Name string `json:"name"`
}

// MindmapFileDTO 是导图生成可选的文件摘要。
type MindmapFileDTO struct {
	FileID   string `json:"file_id"`
	Filename string `json:"filename"`
	Status   string `json:"status"`
}

// MindmapDTO 是知识库当前保存的导图及其来源信息。
type MindmapDTO struct {
	Mindmap       *MindmapNode `json:"mindmap,omitempty"`
	FileIDs       []string     `json:"file_ids"`
	GeneratedAt   string       `json:"generated_at,omitempty"`
	FileCount     int          `json:"file_count"`
	HasMindmap    bool         `json:"has_mindmap"`
	Generating    bool         `json:"generating,omitempty"`     // 是否正在后台生成
	GenerateError string       `json:"generate_error,omitempty"` // 最近一次生成失败原因
}

// MindmapDiffDTO 描述当前文件集合相对导图来源的变化。
type MindmapDiffDTO struct {
	AddedFiles   []string `json:"added_files"`
	RemovedFiles []string `json:"removed_files"`
	NeedsUpdate  bool     `json:"needs_update"`
}

// GenerateMindmapRequest 是生成或增量更新导图的请求参数。
type GenerateMindmapRequest struct {
	FileIDs     []string `json:"file_ids"`
	UserPrompt  string   `json:"user_prompt"`
	Incremental bool     `json:"incremental"`
}

// GenerateMindmapResponse 是导图生成结果。
type GenerateMindmapResponse struct {
	Mindmap       *MindmapNode `json:"mindmap"`
	FileIDs       []string     `json:"file_ids"`
	Incremental   bool         `json:"incremental"`
	NoAINeeded    bool         `json:"no_ai_needed"`
	Generating    bool         `json:"generating,omitempty"`     // true 表示已在后台开始生成
	GenerateError string       `json:"generate_error,omitempty"` // 后台生成失败时的原因
}
