package entity

import "time"

// EvaluationDataset 评估基准（评测数据集）实体。
// 一个数据集属于一个知识库，包含若干问答条目（EvaluationDatasetItem）。
// BuildMetadata 记录自动生成任务的运行状态（status/source/progress/message/error_message/params）。
type EvaluationDataset struct {
	BaseEntity
	DatasetID     string `gorm:"type:varchar(64);uniqueIndex;not null;comment:数据集唯一标识" json:"dataset_id"`
	KBID          string `gorm:"type:varchar(80);index;not null;comment:所属知识库ID" json:"kb_id"`
	Name          string `gorm:"type:varchar(100);not null;comment:数据集名称" json:"name"`
	Description   string `gorm:"type:text;comment:数据集描述" json:"description,omitempty"`
	ItemCount     int    `gorm:"not null;default:0;comment:问答条目总数" json:"item_count"`
	HasGoldChunks bool   `gorm:"not null;default:false;comment:是否包含 Gold Chunks" json:"has_gold_chunks"`
	HasGoldAnswer bool   `gorm:"column:has_gold_answers;not null;default:false;comment:是否包含 Gold Answer" json:"has_gold_answers"`
	BuildMetadata JSON   `gorm:"type:json;default:'{}';comment:构建元数据（状态/进度/来源/参数）" json:"build_metadata"`
}

// TableName 指定表名
func (EvaluationDataset) TableName() string {
	return "evaluation_datasets"
}

// EvaluationDatasetItem 评估基准问答条目。
// 支持两种评估形态：仅检索评估（Query + GoldChunkIDs）、问答评估（Query + GoldAnswer）。
type EvaluationDatasetItem struct {
	BaseEntity
	DatasetID    string    `gorm:"type:varchar(64);index;not null;comment:所属数据集ID" json:"dataset_id"`
	Query        string    `gorm:"type:text;not null;comment:问题" json:"query"`
	GoldChunkIDs JSONArray `gorm:"type:json;comment:期望命中的分块ID列表" json:"gold_chunk_ids,omitempty"`
	GoldAnswer   string    `gorm:"type:text;comment:参考答案" json:"gold_answer,omitempty"`
	SortOrder    int       `gorm:"not null;default:0;comment:排序序号" json:"sort_order"`
}

// TableName 指定表名
func (EvaluationDatasetItem) TableName() string {
	return "evaluation_dataset_items"
}

// EvaluationRun 一次 RAG 评估运行。
// 对一个数据集逐条执行检索（可选答案生成与评判），汇总指标后写入 Metrics。
type EvaluationRun struct {
	BaseEntity
	RunID           string     `gorm:"type:varchar(64);uniqueIndex;not null;comment:运行唯一标识" json:"run_id"`
	KBID            string     `gorm:"type:varchar(80);index;not null;comment:所属知识库ID" json:"kb_id"`
	DatasetID       string     `gorm:"type:varchar(64);not null;comment:使用的数据集ID" json:"dataset_id"`
	Name            string     `gorm:"type:varchar(100);not null;comment:评估名称" json:"name"`
	Status          string     `gorm:"type:varchar(20);not null;default:running;comment:运行状态" json:"status"`
	StartedAt       *time.Time `gorm:"comment:开始时间" json:"started_at,omitempty"`
	CompletedAt     *time.Time `gorm:"comment:完成时间" json:"completed_at,omitempty"`
	TotalItems      int        `gorm:"not null;default:0;comment:条目总数" json:"total_items"`
	CompletedItems  int        `gorm:"not null;default:0;comment:已完成条目数" json:"completed_items"`
	OverallScore    float64    `gorm:"not null;default:0;comment:综合评分(0-1)" json:"overall_score,omitempty"`
	Metrics         JSON       `gorm:"type:json;default:'{}';comment:汇总指标(recall@10等)" json:"metrics,omitempty"`
	RetrievalConfig JSON       `gorm:"type:json;default:'{}';comment:本次使用的检索配置" json:"retrieval_config,omitempty"`
	Progress        float64    `gorm:"not null;default:0;comment:进度百分比(0-100)" json:"progress,omitempty"`
	Message         string     `gorm:"type:text;comment:运行消息" json:"message,omitempty"`
}

// TableName 指定表名
func (EvaluationRun) TableName() string {
	return "evaluation_runs"
}

// EvaluationRunResult 评估运行的单项结果。
type EvaluationRunResult struct {
	BaseEntity
	RunID           string   `gorm:"type:varchar(64);index;not null;comment:所属运行ID" json:"run_id"`
	Query           string   `gorm:"type:text;not null;comment:问题" json:"query"`
	GeneratedAnswer string   `gorm:"type:text;comment:生成答案" json:"generated_answer,omitempty"`
	Metrics         JSON     `gorm:"type:json;default:'{}';comment:单项指标" json:"metrics,omitempty"`
	AnswerScore     *float64 `gorm:"comment:答案评判得分(0-1)" json:"answer_score,omitempty"`
	ErrorMessage    string   `gorm:"type:text;comment:错误信息" json:"error,omitempty"`
	Status          string   `gorm:"type:varchar(20);not null;default:completed;comment:单项状态(completed/error)" json:"status"`
	SortOrder       int      `gorm:"not null;default:0;comment:排序序号" json:"sort_order"`
}

// TableName 指定表名
func (EvaluationRunResult) TableName() string {
	return "evaluation_run_results"
}
