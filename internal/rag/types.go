package rag

// IndexInput 文档索引入口。由 DocumentWorker 在解析完成后传递。
type IndexInput struct {
	KBID     string
	FileID   string
	Filename string
	Markdown string
}

// IndexedChunk 分块索引结果，用于后续 Embedding 与持久化。
type IndexedChunk struct {
	ChunkID    string
	ChunkIndex int
	Content    string
	TokenCount int
}

// IndexOutput 文档索引整体结果。
type IndexOutput struct {
	Chunks []IndexedChunk
}

// AnswerInput 问答入口。
type AnswerInput struct {
	KnowledgeBaseIDs []string
	Query            string
}

// Citation 单条引用。
type Citation struct {
	Index    int     `json:"index"`
	ChunkID  string  `json:"chunk_id"`
	FileID   string  `json:"file_id"`
	Filename string  `json:"filename"`
	Content  string  `json:"content"`
	Score    float64 `json:"score"`
}

// AnswerOutput 问答结果。
type AnswerOutput struct {
	Answer    string     `json:"answer"`
	Citations []Citation `json:"citations"`
}
