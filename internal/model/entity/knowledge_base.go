package entity

// KnowledgeBase 知识库实体
type KnowledgeBase struct {
	BaseEntity
	KBID               string    `gorm:"type:varchar(80);uniqueIndex;not null;comment:知识库唯一标识" json:"kb_id"`
	Name               string    `gorm:"type:varchar(255);not null;index;comment:知识库名称" json:"name"`
	Description        string    `gorm:"type:text;comment:描述" json:"description,omitempty"`
	KBType             string    `gorm:"type:varchar(32);not null;default:'rag';index;comment:知识库类型" json:"kb_type"`
	EmbeddingModelID   uint      `gorm:"not null;index;comment:Embedding模型ID" json:"embedding_model_id"`
	ChatModelID        uint      `gorm:"not null;index;comment:Chat模型ID" json:"chat_model_id"`
	EmbeddingModelSpec string    `gorm:"type:varchar(512);comment:Embedding模型规格" json:"embedding_model_spec,omitempty"`
	LLMModelSpec       string    `gorm:"type:varchar(512);comment:LLM模型规格" json:"llm_model_spec,omitempty"`
	QueryParams        JSON      `gorm:"type:json;comment:查询参数" json:"query_params,omitempty"`
	AdditionalParams   JSON      `gorm:"type:json;comment:附加参数" json:"additional_params,omitempty"`
	Mindmap            JSON      `gorm:"type:json;comment:思维导图数据" json:"mindmap,omitempty"`
	MindmapFileIDs     JSONArray `gorm:"type:json;comment:思维导图关联文件ID列表" json:"mindmap_file_ids,omitempty"`
	MindmapMetadata    JSON      `gorm:"type:json;comment:思维导图元数据" json:"mindmap_metadata,omitempty"`
	SampleQuestions    JSONArray `gorm:"type:json;comment:示例问题列表" json:"sample_questions,omitempty"`

	// 关联关系
	Files []KnowledgeFile `gorm:"foreignKey:KBID;references:KBID" json:"files,omitempty"`
}

// TableName 指定表名
func (KnowledgeBase) TableName() string {
	return "knowledge_bases"
}
