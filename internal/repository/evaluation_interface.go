package repository

import (
	"Qavor/internal/model/entity"
)

// EvaluationRepository 评估基准与评估运行的数据访问接口。
type EvaluationRepository interface {
	// —— 数据集（基准） ——

	// CreateDataset 创建数据集记录。
	CreateDataset(dataset *entity.EvaluationDataset) error
	// ListDatasetsByKBID 获取知识库下的数据集列表（创建时间倒序）。
	ListDatasetsByKBID(kbID string) ([]*entity.EvaluationDataset, error)
	// ListPendingDatasets 获取所有构建中（pending/running）的数据集，用于后台执行器轮询。
	ListPendingDatasets() ([]*entity.EvaluationDataset, error)
	// FindDatasetByID 根据数据集 ID 查询，不存在返回 nil。
	FindDatasetByID(datasetID string) (*entity.EvaluationDataset, error)
	// UpdateDataset 更新数据集记录。
	UpdateDataset(dataset *entity.EvaluationDataset) error
	// DeleteDatasetByID 级联删除数据集及其问答条目。
	DeleteDatasetByID(datasetID string) error
	// ReplaceDatasetItems 事务内替换数据集条目：先删除旧条目再批量写入。
	ReplaceDatasetItems(datasetID string, items []*entity.EvaluationDatasetItem) error
	// CountDatasetItems 统计数据集条目总数。
	CountDatasetItems(datasetID string) (int64, error)
	// ListDatasetItems 分页查询数据集条目。
	ListDatasetItems(datasetID string, offset, limit int) ([]*entity.EvaluationDatasetItem, error)
	// ListAllDatasetItems 查询数据集全部条目（按排序序号）。
	ListAllDatasetItems(datasetID string) ([]*entity.EvaluationDatasetItem, error)

	// —— 评估运行 ——

	// CreateRun 创建评估运行记录。
	CreateRun(run *entity.EvaluationRun) error
	// ListRunsByKBID 获取知识库下的评估运行列表（创建时间倒序）。
	ListRunsByKBID(kbID string) ([]*entity.EvaluationRun, error)
	// ListPendingRuns 获取所有运行中（running）的评估任务，用于后台执行器轮询。
	ListPendingRuns() ([]*entity.EvaluationRun, error)
	// FindRunByID 根据运行 ID 查询，不存在返回 nil。
	FindRunByID(runID string) (*entity.EvaluationRun, error)
	// UpdateRun 更新评估运行记录。
	UpdateRun(run *entity.EvaluationRun) error
	// DeleteRunByID 级联删除评估运行及其单项结果。
	DeleteRunByID(runID string) error
	// ReplaceRunResults 事务内替换运行单项结果。
	ReplaceRunResults(runID string, results []*entity.EvaluationRunResult) error
	// CountRunResults 统计运行单项结果总数（可按 errorOnly 过滤）。
	CountRunResults(runID string, errorOnly bool) (int64, error)
	// ListRunResults 分页查询运行单项结果（可按 errorOnly 过滤）。
	ListRunResults(runID string, offset, limit int, errorOnly bool) ([]*entity.EvaluationRunResult, error)
}
