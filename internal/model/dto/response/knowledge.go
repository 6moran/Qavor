package response

import (
	"Qavor/internal/model/entity"
	"time"
)

// KnowledgeBaseResponse 知识库响应
type KnowledgeBaseResponse struct {
	ID                 uint             `json:"id"`                             // 数据库自增主键
	KBID               string           `json:"kb_id"`                          // 对外使用的知识库唯一标识
	Name               string           `json:"name"`                           // 知识库名称
	Description        string           `json:"description,omitempty"`          // 知识库描述
	KBType             string           `json:"kb_type"`                        // 知识库类型
	EmbeddingModelSpec string           `json:"embedding_model_spec,omitempty"` // 向量化模型标识
	LLMModelSpec       string           `json:"llm_model_spec,omitempty"`       // 大模型表示
	QueryParams        entity.JSON      `json:"query_params,omitempty"`         // 检索参数
	AdditionalParams   entity.JSON      `json:"additional_params,omitempty"`    // 扩展参数
	SampleQuestions    entity.JSONArray `json:"sample_questions,omitempty"`     // 示例问题列表
	CreatedAt          time.Time        `json:"created_at"`                     // 创建时间
	UpdatedAt          time.Time        `json:"updated_at"`                     // 最后更新时间
}

// KnowledgeBaseListResponse 知识库列表响应
type KnowledgeBaseListResponse struct {
	Total int64                   `json:"total"` // 符合过滤条件的总数
	Items []KnowledgeBaseResponse `json:"items"` // 当前页知识库列表
}

// KnowledgeFileResponse 知识文件响应
type KnowledgeFileResponse struct {
	ID               uint      `json:"id"`                          // 数据库自增主键
	FileID           string    `json:"file_id"`                     // 对外使用的文件唯一标识
	KBID             string    `json:"kb_id"`                       // 所属知识库 ID；暂存文件可为空
	ParentID         string    `json:"parent_id,omitempty"`         // 父文件夹 ID
	Filename         string    `json:"filename"`                    // 清理后的安全文件名
	OriginalFilename string    `json:"original_filename,omitempty"` // 用户上传时的原始文件名
	FileType         string    `json:"file_type,omitempty"`         // 文件类型
	Path             string    `json:"path,omitempty"`              // MinIO 对象相对路径
	MinioURL         string    `json:"minio_url,omitempty"`         // 文件访问 URL
	MarkdownFile     string    `json:"markdown_file,omitempty"`     // 解析后 Markdown 文件路径
	Status           string    `json:"status"`                      // 文件处理状态
	ContentHash      string    `json:"content_hash"`                // 内容哈希
	FileSize         *int64    `json:"file_size,omitempty"`         // 文件大小，单位字节
	ChunkCount       int       `json:"chunk_count"`                 // 已生成的知识分块数量
	TokenCount       int64     `json:"token_count"`                 // 文档 Token 总数
	ContentType      string    `json:"content_type,omitempty"`      // 服务端检测到的 MIME 类型
	IsFolder         bool      `json:"is_folder"`                   // 是否为文件夹记录
	ErrorMessage     string    `json:"error_message,omitempty"`     // 处理失败时的错误信息
	CreatedAt        time.Time `json:"created_at"`                  // 创建时间
	UpdatedAt        time.Time `json:"updated_at"`                  // 最后更新时间
}

// KnowledgeFileListResponse 知识文件列表响应
type KnowledgeFileListResponse struct {
	Total int64                   `json:"total"` // 符合筛选条件的文件总数
	Items []KnowledgeFileResponse `json:"items"` // 当前页文件列表
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
