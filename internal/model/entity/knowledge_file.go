package entity

// KnowledgeFile 知识文件实体
type KnowledgeFile struct {
	BaseEntity
	FileID           string `gorm:"type:varchar(64);uniqueIndex;not null;comment:文件唯一标识" json:"file_id"`
	KBID             string `gorm:"type:varchar(80);not null;index;comment:所属知识库ID" json:"kb_id"`
	ParentID         string `gorm:"type:varchar(64);index;comment:父文件ID（文件夹结构）" json:"parent_id,omitempty"`
	Filename         string `gorm:"type:varchar(512);not null;comment:文件名" json:"filename"`
	OriginalFilename string `gorm:"type:varchar(512);comment:原始文件名" json:"original_filename,omitempty"`
	FileType         string `gorm:"type:varchar(64);comment:文件类型" json:"file_type,omitempty"`
	Path             string `gorm:"type:varchar(1024);comment:文件路径" json:"path,omitempty"`
	MinioURL         string `gorm:"type:varchar(1024);comment:MinIO存储URL" json:"minio_url,omitempty"`
	MarkdownFile     string `gorm:"type:varchar(1024);comment:Markdown文件路径" json:"markdown_file,omitempty"`
	Status           string `gorm:"type:varchar(32);index;default:uploaded;comment:处理状态" json:"status"`
	ContentHash      string `gorm:"type:varchar(128);index;comment:内容哈希（去重）" json:"content_hash"`
	FileSize         *int64 `gorm:"comment:文件大小（字节）" json:"file_size,omitempty"`
	ChunkCount       int    `gorm:"default:0;comment:分块数量" json:"chunk_count"`
	TokenCount       int64  `gorm:"default:0;comment:Token数量" json:"token_count"`
	ContentType      string `gorm:"type:varchar(64);comment:内容类型" json:"content_type,omitempty"`
	ProcessingParams JSON   `gorm:"type:json;comment:处理参数" json:"processing_params,omitempty"`
	IsFolder         bool   `gorm:"default:false;comment:是否为文件夹" json:"is_folder"`
	ErrorMessage     string `gorm:"type:text;comment:错误信息" json:"error_message,omitempty"`
	// 关联关系
	KnowledgeBase *KnowledgeBase   `gorm:"foreignKey:KBID;references:KBID" json:"knowledge_base,omitempty"`
	Parent        *KnowledgeFile   `gorm:"foreignKey:ParentID;references:FileID" json:"parent,omitempty"`
	Children      []KnowledgeFile  `gorm:"foreignKey:ParentID;references:FileID" json:"children,omitempty"`
	Chunks        []KnowledgeChunk `gorm:"foreignKey:FileID;references:FileID" json:"chunks,omitempty"`
}

// TableName 指定表名
func (KnowledgeFile) TableName() string {
	return "knowledge_files"
}
