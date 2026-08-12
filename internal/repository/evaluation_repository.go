package repository

import (
	"errors"

	"Qavor/internal/model/entity"

	"gorm.io/gorm"
)

// evaluationRepository 评估基准与评估运行的数据访问实现。
type evaluationRepository struct {
	db *gorm.DB
}

// dataset 构建状态常量（用于 pending 任务 SQL 过滤）。
const (
	datasetBuildPendingSQL = "pending"
	datasetBuildRunningSQL = "running"
)

// NewEvaluationRepository 创建评估数据访问实例。
func NewEvaluationRepository(db *gorm.DB) EvaluationRepository {
	return &evaluationRepository{db: db}
}

// CreateDataset 创建数据集记录。
func (r *evaluationRepository) CreateDataset(dataset *entity.EvaluationDataset) error {
	return r.db.Create(dataset).Error
}

// ListDatasetsByKBID 获取知识库下的数据集列表（创建时间倒序）。
func (r *evaluationRepository) ListDatasetsByKBID(kbID string) ([]*entity.EvaluationDataset, error) {
	var datasets []*entity.EvaluationDataset
	err := r.db.Where("kb_id = ?", kbID).Order("created_at DESC").Find(&datasets).Error
	return datasets, err
}

// ListPendingDatasets 获取所有构建中（pending/running）的数据集。
func (r *evaluationRepository) ListPendingDatasets() ([]*entity.EvaluationDataset, error) {
	var datasets []*entity.EvaluationDataset
	err := r.db.Where("build_metadata->>'status' IN ?", []string{datasetBuildPendingSQL, datasetBuildRunningSQL}).
		Order("created_at ASC").
		Find(&datasets).Error
	return datasets, err
}

// FindDatasetByID 根据数据集 ID 查询，不存在返回 nil。
func (r *evaluationRepository) FindDatasetByID(datasetID string) (*entity.EvaluationDataset, error) {
	var dataset entity.EvaluationDataset
	err := r.db.Where("dataset_id = ?", datasetID).First(&dataset).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &dataset, nil
}

// UpdateDataset 更新数据集记录。
func (r *evaluationRepository) UpdateDataset(dataset *entity.EvaluationDataset) error {
	return r.db.Save(dataset).Error
}

// DeleteDatasetByID 级联删除数据集及其问答条目。
func (r *evaluationRepository) DeleteDatasetByID(datasetID string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("dataset_id = ?", datasetID).Delete(&entity.EvaluationDatasetItem{}).Error; err != nil {
			return err
		}
		return tx.Where("dataset_id = ?", datasetID).Delete(&entity.EvaluationDataset{}).Error
	})
}

// ReplaceDatasetItems 事务内替换数据集条目：先删除旧条目再批量写入。
func (r *evaluationRepository) ReplaceDatasetItems(datasetID string, items []*entity.EvaluationDatasetItem) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("dataset_id = ?", datasetID).Delete(&entity.EvaluationDatasetItem{}).Error; err != nil {
			return err
		}
		if len(items) == 0 {
			return nil
		}
		return tx.Create(items).Error
	})
}

// CountDatasetItems 统计数据集条目总数。
func (r *evaluationRepository) CountDatasetItems(datasetID string) (int64, error) {
	var total int64
	err := r.db.Model(&entity.EvaluationDatasetItem{}).Where("dataset_id = ?", datasetID).Count(&total).Error
	return total, err
}

// ListDatasetItems 分页查询数据集条目。
func (r *evaluationRepository) ListDatasetItems(datasetID string, offset, limit int) ([]*entity.EvaluationDatasetItem, error) {
	var items []*entity.EvaluationDatasetItem
	err := r.db.Where("dataset_id = ?", datasetID).
		Order("sort_order ASC, id ASC").
		Offset(offset).Limit(limit).
		Find(&items).Error
	return items, err
}

// ListAllDatasetItems 查询数据集全部条目（按排序序号）。
func (r *evaluationRepository) ListAllDatasetItems(datasetID string) ([]*entity.EvaluationDatasetItem, error) {
	var items []*entity.EvaluationDatasetItem
	err := r.db.Where("dataset_id = ?", datasetID).
		Order("sort_order ASC, id ASC").
		Find(&items).Error
	return items, err
}

// CreateRun 创建评估运行记录。
func (r *evaluationRepository) CreateRun(run *entity.EvaluationRun) error {
	return r.db.Create(run).Error
}

// ListRunsByKBID 获取知识库下的评估运行列表（创建时间倒序）。
func (r *evaluationRepository) ListRunsByKBID(kbID string) ([]*entity.EvaluationRun, error) {
	var runs []*entity.EvaluationRun
	err := r.db.Where("kb_id = ?", kbID).Order("created_at DESC").Find(&runs).Error
	return runs, err
}

// ListPendingRuns 获取所有运行中（running）的评估任务。
func (r *evaluationRepository) ListPendingRuns() ([]*entity.EvaluationRun, error) {
	var runs []*entity.EvaluationRun
	err := r.db.Where("status = ?", "running").
		Order("created_at ASC").
		Find(&runs).Error
	return runs, err
}

// FindRunByID 根据运行 ID 查询，不存在返回 nil。
func (r *evaluationRepository) FindRunByID(runID string) (*entity.EvaluationRun, error) {
	var run entity.EvaluationRun
	err := r.db.Where("run_id = ?", runID).First(&run).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &run, nil
}

// UpdateRun 更新评估运行记录。
func (r *evaluationRepository) UpdateRun(run *entity.EvaluationRun) error {
	return r.db.Save(run).Error
}

// DeleteRunByID 级联删除评估运行及其单项结果。
func (r *evaluationRepository) DeleteRunByID(runID string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("run_id = ?", runID).Delete(&entity.EvaluationRunResult{}).Error; err != nil {
			return err
		}
		return tx.Where("run_id = ?", runID).Delete(&entity.EvaluationRun{}).Error
	})
}

// ReplaceRunResults 事务内替换运行单项结果。
func (r *evaluationRepository) ReplaceRunResults(runID string, results []*entity.EvaluationRunResult) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("run_id = ?", runID).Delete(&entity.EvaluationRunResult{}).Error; err != nil {
			return err
		}
		if len(results) == 0 {
			return nil
		}
		return tx.Create(results).Error
	})
}

// CountRunResults 统计运行单项结果总数（可按 errorOnly 过滤）。
func (r *evaluationRepository) CountRunResults(runID string, errorOnly bool) (int64, error) {
	query := r.db.Model(&entity.EvaluationRunResult{}).Where("run_id = ?", runID)
	if errorOnly {
		query = query.Where("status = ?", "error")
	}
	var total int64
	err := query.Count(&total).Error
	return total, err
}

// ListRunResults 分页查询运行单项结果（可按 errorOnly 过滤）。
func (r *evaluationRepository) ListRunResults(runID string, offset, limit int, errorOnly bool) ([]*entity.EvaluationRunResult, error) {
	query := r.db.Where("run_id = ?", runID)
	if errorOnly {
		query = query.Where("status = ?", "error")
	}
	var results []*entity.EvaluationRunResult
	err := query.Order("sort_order ASC, id ASC").Offset(offset).Limit(limit).Find(&results).Error
	return results, err
}

var _ EvaluationRepository = (*evaluationRepository)(nil)
