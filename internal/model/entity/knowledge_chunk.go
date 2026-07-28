package entity

// KnowledgeChunk 知识分块实体
type KnowledgeChunk struct {
	BaseEntity
	ChunkID          string    `gorm:"type:varchar(128);uniqueIndex;not null;comment:分块唯一标识" json:"chunk_id"`
	FileID           string    `gorm:"type:varchar(64);not null;comment:所属文件ID" json:"file_id"`
	KBID             string    `gorm:"type:varchar(80);not null;comment:所属知识库ID" json:"kb_id"`
	ChunkIndex       int       `gorm:"not null;comment:分块序号" json:"chunk_index"`
	Content          string    `gorm:"type:text;not null;comment:分块内容" json:"content"`
	StartCharPos     *int      `gorm:"comment:起始字符位置" json:"start_char_pos,omitempty"`
	EndCharPos       *int      `gorm:"comment:结束字符位置" json:"end_char_pos,omitempty"`
	StartTokenPos    *int      `gorm:"comment:起始Token位置" json:"start_token_pos,omitempty"`
	EndTokenPos      *int      `gorm:"comment:结束Token位置" json:"end_token_pos,omitempty"`
	GraphIndexed     bool      `gorm:"default:false;comment:是否已建立知识图谱索引" json:"graph_indexed"`
	EntIDs           JSONArray `gorm:"type:json;comment:关联的实体ID列表" json:"ent_ids,omitempty"`
	Tags             JSONArray `gorm:"type:json;comment:标签列表" json:"tags,omitempty"`
	ExtractionResult JSON      `gorm:"type:json;comment:抽取结果" json:"extraction_result,omitempty"`

	// 关联关系
	KnowledgeFile *KnowledgeFile `gorm:"foreignKey:FileID;references:FileID" json:"knowledge_file,omitempty"`
	KnowledgeBase *KnowledgeBase `gorm:"foreignKey:KBID;references:KBID" json:"knowledge_base,omitempty"`
}

// TableName 指定表名
func (KnowledgeChunk) TableName() string {
	return "knowledge_chunks"
}
