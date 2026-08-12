package request

import "Qavor/internal/model/entity"

// CreateKnowledgeBaseRequest 创建知识库请求
type CreateKnowledgeBaseRequest struct {
	DatabaseName       string           `json:"database_name" binding:"required,max=255"`         // 知识库名称
	Description        string           `json:"description" binding:"required"`                   // 知识库用途和内容说明
	EmbeddingModelID   uint             `json:"embedding_model_id" binding:"required,min=1"`      // 必须绑定 Embedding 模型
	ChatModelID        uint             `json:"chat_model_id" binding:"required,min=1"`           // 必须绑定 Chat 模型
	EmbeddingModelSpec string           `json:"embedding_model_spec" binding:"omitempty,max=512"` // 向量化模型
	LLMModelSpec       string           `json:"llm_model_spec" binding:"omitempty,max=512"`       // 知识库关联的大模型
	QueryParams        entity.JSON      `json:"query_params" binding:"omitempty"`                 // 检索参数配置
	AdditionalParams   entity.JSON      `json:"additional_params" binding:"omitempty"`            // 知识库类型相关的扩展参数
	SampleQuestions    entity.JSONArray `json:"sample_questions" binding:"omitempty"`             // 示例问题列表
}

// UpdateKnowledgeBaseRequest 更新知识库请求
type UpdateKnowledgeBaseRequest struct {
	Name             string      `json:"name" binding:"required,max=255"`              // 更新后的知识库名称
	Description      string      `json:"description" binding:"required"`               // 更新后的知识库描述
	EmbeddingModelID uint        `json:"embedding_model_id" binding:"omitempty,min=1"` // 更新后的 Embedding 模型
	ChatModelID      uint        `json:"chat_model_id" binding:"omitempty,min=1"`      // 更新后的 Chat 模型
	LLMModelSpec     string      `json:"llm_model_spec" binding:"omitempty,max=512"`   // 更新后的大模型规格
	AdditionalParams entity.JSON `json:"additional_params" binding:"omitempty"`        // 更新扩展参数
}

// KnowledgeBaseListRequest 知识库列表请求
type KnowledgeBaseListRequest struct {
	Page     int    `form:"page" binding:"omitempty,min=1"`              // 页码，从 1 开始
	PageSize int    `form:"page_size" binding:"omitempty,min=1,max=100"` // 每页记录数
	Keyword  string `form:"keyword" binding:"omitempty"`                 // 名称或描述的模糊搜索关键词
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

// CreateKnowledgeFolderRequest 创建知识库文件夹请求
type CreateKnowledgeFolderRequest struct {
	FolderName string `json:"folder_name" binding:"required,max=255"`
	ParentID   string `json:"parent_id" binding:"omitempty,max=64"`
}

// SearchKnowledgeFileRequest 文件管理按名称搜索请求
type SearchKnowledgeFileRequest struct {
	Query  string `form:"query" binding:"required,max=255"`
	Offset int    `form:"offset" binding:"omitempty,min=0"`
	Limit  int    `form:"limit" binding:"omitempty,min=1,max=100"`
}

// BatchDeleteKnowledgeFileRequest 批量删除知识库文件请求。
type BatchDeleteKnowledgeFileRequest struct {
	FileIDs []string `json:"file_ids" binding:"required,min=1,max=50,dive,required,max=64"`
}

// ChunkParserConfig 分块解析配置。
type ChunkParserConfig struct {
	ChunkTokenNum     int `json:"chunk_token_num" binding:"required,min=50,max=4000"`
	OverlappedPercent int `json:"overlapped_percent" binding:"min=0,max=50"`
}

// ChunkParams 分块参数。
type ChunkParams struct {
	ChunkPresetID     string            `json:"chunk_preset_id" binding:"required,max=64"`
	ChunkParserConfig ChunkParserConfig `json:"chunk_parser_config" binding:"required"`
}

// IndexKnowledgeFilesRequest 手动入库请求。
type IndexKnowledgeFilesRequest struct {
	FileIDs []string    `json:"file_ids" binding:"required,min=1,max=50,dive,required,max=64"`
	Params  ChunkParams `json:"params" binding:"required"`
}

// IndexOneKnowledgeFileRequest 单文件手动入库请求，文件 ID 来自路径参数。
type IndexOneKnowledgeFileRequest struct {
	Params ChunkParams `json:"params" binding:"required"`
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

// QueryTestRequest 检索测试请求
// Meta 为可选的检索参数覆盖（白名单键由服务层解析），不修改知识库配置。
type QueryTestRequest struct {
	Query string         `json:"query" binding:"required,max=2000"` // 查询文本
	Meta  map[string]any `json:"meta"`                              // 检索参数覆盖
}

// UpdateQueryParamsRequest 更新知识库检索参数请求（整包 meta，服务层白名单过滤）。
type UpdateQueryParamsRequest map[string]any

// GenerateSampleQuestionsRequest 生成示例问题请求
type GenerateSampleQuestionsRequest struct {
	Count int `json:"count" binding:"omitempty,min=1,max=50"` // 生成数量，默认 10
}
