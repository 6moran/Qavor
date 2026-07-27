package response

import (
	"Qavor/internal/model/entity"
	"time"
)

// KnowledgeBaseResponse 知识库响应
type KnowledgeBaseResponse struct {
	ID                 uint             `json:"id"`
	KBID               string           `json:"kb_id"`
	Name               string           `json:"name"`
	Description        string           `json:"description,omitempty"`
	KBType             string           `json:"kb_type"`
	EmbeddingModelSpec string           `json:"embedding_model_spec,omitempty"`
	LLMModelSpec       string           `json:"llm_model_spec,omitempty"`
	QueryParams        entity.JSON      `json:"query_params,omitempty"`
	AdditionalParams   entity.JSON      `json:"additional_params,omitempty"`
	ShareConfig        entity.JSON      `json:"share_config,omitempty"`
	SampleQuestions    entity.JSONArray `json:"sample_questions,omitempty"`
	CreatedBy          string           `json:"created_by,omitempty"`
	CreatedAt          time.Time        `json:"created_at"`
	UpdatedAt          time.Time        `json:"updated_at"`
}

// KnowledgeBaseListResponse 知识库列表响应
type KnowledgeBaseListResponse struct {
	Total int64                   `json:"total"`
	Items []KnowledgeBaseResponse `json:"items"`
}

// KnowledgeFileResponse 知识文件响应
type KnowledgeFileResponse struct {
	ID               uint      `json:"id"`
	FileID           string    `json:"file_id"`
	KBID             string    `json:"kb_id"`
	ParentID         string    `json:"parent_id,omitempty"`
	Filename         string    `json:"filename"`
	OriginalFilename string    `json:"original_filename,omitempty"`
	FileType         string    `json:"file_type,omitempty"`
	Path             string    `json:"path,omitempty"`
	MinioURL         string    `json:"minio_url,omitempty"`
	MarkdownFile     string    `json:"markdown_file,omitempty"`
	Status           string    `json:"status"`
	ContentHash      string    `json:"content_hash"`
	FileSize         *int64    `json:"file_size,omitempty"`
	ChunkCount       int       `json:"chunk_count"`
	TokenCount       int64     `json:"token_count"`
	ContentType      string    `json:"content_type,omitempty"`
	IsFolder         bool      `json:"is_folder"`
	ErrorMessage     string    `json:"error_message,omitempty"`
	CreatedBy        string    `json:"created_by,omitempty"`
	UpdatedBy        string    `json:"updated_by,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// KnowledgeFileListResponse 知识文件列表响应
type KnowledgeFileListResponse struct {
	Total int64                   `json:"total"`
	Items []KnowledgeFileResponse `json:"items"`
}

// KnowledgeChunkResponse 知识分块响应
type KnowledgeChunkResponse struct {
	ID            uint             `json:"id"`
	ChunkID       string           `json:"chunk_id"`
	FileID        string           `json:"file_id"`
	KBID          string           `json:"kb_id"`
	ChunkIndex    int              `json:"chunk_index"`
	Content       string           `json:"content"`
	StartCharPos  *int             `json:"start_char_pos,omitempty"`
	EndCharPos    *int             `json:"end_char_pos,omitempty"`
	StartTokenPos *int             `json:"start_token_pos,omitempty"`
	EndTokenPos   *int             `json:"end_token_pos,omitempty"`
	GraphIndexed  bool             `json:"graph_indexed"`
	Tags          entity.JSONArray `json:"tags,omitempty"`
	CreatedAt     time.Time        `json:"created_at"`
	UpdatedAt     time.Time        `json:"updated_at"`
}

// SearchResultResponse 搜索结果响应
type SearchResultResponse struct {
	ID       string      `json:"id"`
	KBID     string      `json:"kb_id"`
	FileID   string      `json:"file_id,omitempty"`
	Content  string      `json:"content"`
	Metadata interface{} `json:"metadata,omitempty"`
}

// SearchOutputResponse 搜索输出响应
type SearchOutputResponse struct {
	KBID    string                 `json:"kb_id"`
	Results []SearchResultResponse `json:"results,omitempty"`
}

// FindWindowResponse 查找窗口响应
type FindWindowResponse struct {
	StartLine    int    `json:"start_line"`
	EndLine      int    `json:"end_line"`
	MatchedLines []int  `json:"matched_lines"`
	Content      string `json:"content"`
}

// FindOutputResponse 查找输出响应
type FindOutputResponse struct {
	KBID         string               `json:"kb_id"`
	FileID       string               `json:"file_id"`
	Semantic     bool                 `json:"semantic"`
	MatchMode    string               `json:"match_mode"`
	TotalMatches int                  `json:"total_matches"`
	Windows      []FindWindowResponse `json:"windows"`
}

// OpenFileOutputResponse 打开文件输出响应
type OpenFileOutputResponse struct {
	KBID          string `json:"kb_id"`
	FileID        string `json:"file_id"`
	StartLine     int    `json:"start_line"`
	EndLine       int    `json:"end_line"`
	TotalLines    int    `json:"total_lines"`
	Offset        int    `json:"offset"`
	WindowSize    int    `json:"window_size"`
	HasMoreBefore bool   `json:"has_more_before"`
	HasMoreAfter  bool   `json:"has_more_after"`
	NextOffset    *int   `json:"next_offset,omitempty"`
	Content       string `json:"content"`
}
