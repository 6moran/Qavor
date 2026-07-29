package request

import "Qavor/internal/model/entity"

// CreateKnowledgeBaseRequest 创建知识库请求
type CreateKnowledgeBaseRequest struct {
	DatabaseName       string           `json:"database_name" binding:"required,max=255"`         // 知识库名称
	Description        string           `json:"description" binding:"required"`                   // 知识库用途和内容说明
	KBType             string           `json:"kb_type" binding:"omitempty,max=32"`               // 向量存储后端类型；未传时默认使用 pgvector
	EmbeddingModelSpec string           `json:"embedding_model_spec" binding:"omitempty,max=512"` // 向量化模型
	LLMModelSpec       string           `json:"llm_model_spec" binding:"omitempty,max=512"`       // 知识库关联的大模型
	QueryParams        entity.JSON      `json:"query_params" binding:"omitempty"`                 // 检索参数配置
	AdditionalParams   entity.JSON      `json:"additional_params" binding:"omitempty"`            // 知识库类型相关的扩展参数
	SampleQuestions    entity.JSONArray `json:"sample_questions" binding:"omitempty"`             // 示例问题列表
}

// UpdateKnowledgeBaseRequest 更新知识库请求
type UpdateKnowledgeBaseRequest struct {
	Name             string      `json:"name" binding:"required,max=255"`            // 更新后的知识库名称
	Description      string      `json:"description" binding:"required"`             // 更新后的知识库描述
	LLMModelSpec     string      `json:"llm_model_spec" binding:"omitempty,max=512"` // 更新后的大模型规格
	AdditionalParams entity.JSON `json:"additional_params" binding:"omitempty"`      // 更新扩展参数
}

// KnowledgeBaseListRequest 知识库列表请求
type KnowledgeBaseListRequest struct {
	Page     int    `form:"page" binding:"omitempty,min=1"`              // 页码，从 1 开始
	PageSize int    `form:"page_size" binding:"omitempty,min=1,max=100"` // 每页记录数
	Keyword  string `form:"keyword" binding:"omitempty"`                 // 名称或描述的模糊搜索关键词
	KBType   string `form:"kb_type" binding:"omitempty"`                 // 知识库类型过滤条件
}

// KnowledgeFileListRequest 知识库文件列表请求
type KnowledgeFileListRequest struct {
	ParentID   string `form:"parent_id" binding:"omitempty,max=64"`        // 父文件夹 ID；空值表示根目录
	PathPrefix string `form:"path_prefix" binding:"omitempty,max=1024"`    // 路径型虚拟目录前缀
	Status     string `form:"status" binding:"omitempty,max=32"`           // 文件处理状态；all 表示不过滤
	Page       int    `form:"page" binding:"omitempty,min=1"`              // 页码，从 1 开始
	PageSize   int    `form:"page_size" binding:"omitempty,min=1,max=500"` // 每页数量，业务层默认 100
	Recursive  bool   `form:"recursive"`                                   // 是否跨子目录递归筛选
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
