package service

import "context"

// EvaluationService 评估基准与评估运行服务。
type EvaluationService interface {
	// Start 启动后台执行器，处理挂起的生成任务与评估运行（应用启动时调用）。
	Start(ctx context.Context)
	// Stop 停止后台执行器（应用关闭时调用）。
	Stop()

	// —— 数据集（基准） ——

	// UploadDataset 上传 JSONL 评测数据集文件并解析入库。
	UploadDataset(ctx context.Context, kbID, name, description string, file []byte, filename string) (*EvaluationDatasetDTO, error)
	// ListDatasets 获取知识库下的数据集列表。
	ListDatasets(kbID string) ([]*EvaluationDatasetDTO, error)
	// GetDataset 获取数据集详情（分页查看问答条目）。
	GetDataset(ctx context.Context, kbID, datasetID string, page, pageSize int) (*EvaluationDatasetDetailDTO, error)
	// DeleteDataset 删除数据集及其条目。
	DeleteDataset(datasetID string) error
	// DownloadDataset 导出数据集为 JSONL，返回文件名与内容。
	DownloadDataset(datasetID string) (filename string, data []byte, err error)
	// GenerateDataset 提交 AI 自动生成评测数据集任务（异步执行）。
	GenerateDataset(ctx context.Context, kbID string, req *GenerateDatasetRequest) (*EvaluationDatasetDTO, error)
	// ResumeDatasetGeneration 恢复失败的自动生成任务。
	ResumeDatasetGeneration(ctx context.Context, kbID, datasetID string) (*ResumeDatasetResponse, error)

	// —— 评估运行 ——

	// RunEvaluation 发起一次 RAG 评估运行（异步执行）。
	RunEvaluation(ctx context.Context, kbID string, req *RunEvaluationRequest) (*EvaluationRunDTO, error)
	// ListRuns 获取知识库下的评估运行列表。
	ListRuns(kbID string) ([]*EvaluationRunDTO, error)
	// GetRunResults 获取单次评估运行的结果（分页，支持 error_only 过滤）。
	GetRunResults(ctx context.Context, kbID, runID string, page, pageSize int, errorOnly bool) (*EvaluationRunDetailDTO, error)
	// DeleteRun 删除评估运行及其结果。
	DeleteRun(kbID, runID string) error
}

// EvaluationDatasetDTO 数据集响应。
type EvaluationDatasetDTO struct {
	DatasetID      string         `json:"dataset_id"`
	KBID           string         `json:"kb_id,omitempty"`
	Name           string         `json:"name"`
	Description    string         `json:"description,omitempty"`
	ItemCount      int            `json:"item_count"`
	HasGoldChunks  bool           `json:"has_gold_chunks"`
	HasGoldAnswers bool           `json:"has_gold_answers"`
	BuildMetadata  map[string]any `json:"build_metadata"`
	CreatedAt      string         `json:"created_at"`
	UpdatedAt      string         `json:"updated_at,omitempty"`
}

// EvaluationDatasetItemDTO 数据集问答条目响应。
type EvaluationDatasetItemDTO struct {
	Query        string   `json:"query"`
	GoldChunkIDs []string `json:"gold_chunk_ids,omitempty"`
	GoldAnswer   string   `json:"gold_answer,omitempty"`
}

// EvaluationDatasetDetailDTO 数据集详情响应。
type EvaluationDatasetDetailDTO struct {
	EvaluationDatasetDTO
	Items      []EvaluationDatasetItemDTO `json:"items"`
	Pagination *EvaluationPagination      `json:"pagination"`
}

// EvaluationPagination 分页信息。
type EvaluationPagination struct {
	Total      int `json:"total_items"`
	TotalPages int `json:"total_pages"`
	Page       int `json:"page"`
	PageSize   int `json:"page_size"`
}

// GenerateDatasetRequest 自动生成评测数据集的请求参数。
type GenerateDatasetRequest struct {
	Name             string `json:"name"`
	Description      string `json:"description"`
	Count            int    `json:"count"`
	NeighborsCount   int    `json:"neighbors_count"`
	ConcurrencyCount int    `json:"concurrency_count"`
	GenerationMode   string `json:"generation_mode"`
	GraphExpandTopK  int    `json:"graph_expand_top_k"`
	LLMModelSpec     string `json:"llm_model_spec"`
}

// ResumeDatasetResponse 恢复生成任务的响应。
type ResumeDatasetResponse struct {
	Message string `json:"message"`
}

// RunEvaluationRequest 发起评估运行的请求参数。
type RunEvaluationRequest struct {
	DatasetID   string         `json:"dataset_id"`
	Name        string         `json:"name"`
	ModelConfig RunModelConfig `json:"model_config"`
}

// RunModelConfig 评估运行的模型配置。
type RunModelConfig struct {
	AnswerLLM string `json:"answer_llm"`
	JudgeLLM  string `json:"judge_llm"`
}

// EvaluationRunDTO 评估运行响应。
type EvaluationRunDTO struct {
	RunID           string         `json:"run_id"`
	KBID            string         `json:"kb_id,omitempty"`
	DatasetID       string         `json:"dataset_id"`
	Name            string         `json:"name"`
	Status          string         `json:"status"`
	StartedAt       string         `json:"started_at,omitempty"`
	CompletedAt     string         `json:"completed_at,omitempty"`
	TotalItems      int            `json:"total_items"`
	CompletedItems  int            `json:"completed_items"`
	OverallScore    float64        `json:"overall_score,omitempty"`
	Metrics         map[string]any `json:"metrics,omitempty"`
	RetrievalConfig map[string]any `json:"retrieval_config,omitempty"`
	Progress        float64        `json:"progress,omitempty"`
	Message         string         `json:"message,omitempty"`
	CreatedAt       string         `json:"created_at"`
}

// EvaluationRunResultDTO 评估运行单项结果响应。
type EvaluationRunResultDTO struct {
	Query           string         `json:"query"`
	GeneratedAnswer string         `json:"generated_answer,omitempty"`
	Metrics         map[string]any `json:"metrics,omitempty"`
	AnswerScore     *float64       `json:"answer_score,omitempty"`
	Error           string         `json:"error,omitempty"`
	Status          string         `json:"status"`
}

// EvaluationRunDetailDTO 评估运行详情响应。
type EvaluationRunDetailDTO struct {
	RunID           string                   `json:"run_id"`
	Name            string                   `json:"name"`
	Status          string                   `json:"status"`
	StartedAt       string                   `json:"started_at,omitempty"`
	CompletedAt     string                   `json:"completed_at,omitempty"`
	TotalItems      int                      `json:"total_items"`
	CompletedItems  int                      `json:"completed_items"`
	OverallScore    float64                  `json:"overall_score,omitempty"`
	RetrievalConfig map[string]any           `json:"retrieval_config,omitempty"`
	Items           []EvaluationRunResultDTO `json:"items"`
	Pagination      *EvaluationRunPagination `json:"pagination"`
}

// EvaluationRunPagination 运行结果分页信息（与前端契约一致）。
type EvaluationRunPagination struct {
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
	Page       int `json:"page"`
	PageSize   int `json:"page_size"`
}
