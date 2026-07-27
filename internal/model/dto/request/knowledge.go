package request

import "Qavor/internal/model/entity"

// CreateKnowledgeBaseRequest 创建知识库请求
type CreateKnowledgeBaseRequest struct {
	Name               string           `json:"name" binding:"required,max=255"`
	Description        string           `json:"description" binding:"omitempty"`
	KBType             string           `json:"kb_type" binding:"required,max=32"`
	EmbeddingModelSpec string           `json:"embedding_model_spec" binding:"omitempty,max=512"`
	LLMModelSpec       string           `json:"llm_model_spec" binding:"omitempty,max=512"`
	QueryParams        entity.JSON      `json:"query_params" binding:"omitempty"`
	AdditionalParams   entity.JSON      `json:"additional_params" binding:"omitempty"`
	ShareConfig        entity.JSON      `json:"share_config" binding:"omitempty"`
	SampleQuestions    entity.JSONArray `json:"sample_questions" binding:"omitempty"`
}

// UpdateKnowledgeBaseRequest 更新知识库请求
type UpdateKnowledgeBaseRequest struct {
	Name               string           `json:"name" binding:"omitempty,max=255"`
	Description        string           `json:"description" binding:"omitempty"`
	EmbeddingModelSpec string           `json:"embedding_model_spec" binding:"omitempty,max=512"`
	LLMModelSpec       string           `json:"llm_model_spec" binding:"omitempty,max=512"`
	QueryParams        entity.JSON      `json:"query_params" binding:"omitempty"`
	AdditionalParams   entity.JSON      `json:"additional_params" binding:"omitempty"`
	ShareConfig        entity.JSON      `json:"share_config" binding:"omitempty"`
	SampleQuestions    entity.JSONArray `json:"sample_questions" binding:"omitempty"`
}

// KnowledgeBaseListRequest 知识库列表请求
type KnowledgeBaseListRequest struct {
	Page     int    `form:"page" binding:"omitempty,min=1"`
	PageSize int    `form:"page_size" binding:"omitempty,min=1,max=100"`
	Keyword  string `form:"keyword" binding:"omitempty"`
	KBType   string `form:"kb_type" binding:"omitempty"`
}

// SearchKnowledgeRequest 知识库搜索请求
type SearchKnowledgeRequest struct {
	KBID      string `json:"kb_id" binding:"required"`
	QueryText string `json:"query_text" binding:"required"`
	FileName  string `json:"file_name" binding:"omitempty"`
}

// FindKnowledgeRequest 知识库查找请求
type FindKnowledgeRequest struct {
	KBID          string   `json:"kb_id" binding:"required"`
	FileID        string   `json:"file_id" binding:"required"`
	Patterns      []string `json:"patterns" binding:"required,min=1"`
	UseRegex      bool     `json:"use_regex" binding:"omitempty"`
	CaseSensitive bool     `json:"case_sensitive" binding:"omitempty"`
	MaxWindows    int      `json:"max_windows" binding:"omitempty,min=1,max=20"`
	WindowSize    int      `json:"window_size" binding:"omitempty,min=1,max=200"`
}

// OpenKnowledgeFileRequest 打开知识库文件请求
type OpenKnowledgeFileRequest struct {
	KBID       string `json:"kb_id" binding:"required"`
	FileID     string `json:"file_id" binding:"required"`
	Line       *int   `json:"line" binding:"omitempty,min=1"`
	Offset     *int   `json:"offset" binding:"omitempty,min=0"`
	WindowSize int    `json:"window_size" binding:"omitempty,min=1,max=2000"`
}
