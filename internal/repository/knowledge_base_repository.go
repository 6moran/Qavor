package repository

import (
	"errors"
	"strings"

	"Qavor/internal/model/entity"

	"gorm.io/gorm"
)

// knowledgeBaseRepository 知识库数据访问实现
type knowledgeBaseRepository struct {
	db *gorm.DB
}

// NewKnowledgeBaseRepository 创建知识库数据访问实例
func NewKnowledgeBaseRepository(db *gorm.DB) KnowledgeBaseRepository {
	return &knowledgeBaseRepository{db: db}
}

// Create 创建知识库记录
func (r *knowledgeBaseRepository) Create(base *entity.KnowledgeBase) error {
	return r.db.Create(base).Error
}

// FindByKBID 根据知识库ID查询
func (r *knowledgeBaseRepository) FindByKBID(kbID string) (*entity.KnowledgeBase, error) {
	var base entity.KnowledgeBase
	// 查询单条记录，记录不存在时返回 nil 而非错误
	err := r.db.Where("kb_id = ?", kbID).First(&base).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &base, nil
}

// FindByKBIDs 批量根据知识库ID查询，单条 SQL 完成；空输入返回空切片。
func (r *knowledgeBaseRepository) FindByKBIDs(kbIDs []string) ([]*entity.KnowledgeBase, error) {
	if len(kbIDs) == 0 {
		return nil, nil
	}
	var bases []*entity.KnowledgeBase
	err := r.db.Where("kb_id IN ?", kbIDs).Find(&bases).Error
	if err != nil {
		return nil, err
	}
	return bases, nil
}

// List 分页查询知识库列表
func (r *knowledgeBaseRepository) List(offset, limit int, keyword string) ([]*entity.KnowledgeBase, int64, error) {
	// 构建查询条件
	query := r.db.Model(&entity.KnowledgeBase{})
	// 关键词搜索（模糊匹配名称和描述）
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		query = query.Where("name ILIKE ? OR description ILIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}
	// 查询总数
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	// 分页查询列表，按创建时间倒序
	var bases []*entity.KnowledgeBase
	if err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&bases).Error; err != nil {
		return nil, 0, err
	}
	return bases, total, nil
}

// Update 更新知识库记录
func (r *knowledgeBaseRepository) Update(base *entity.KnowledgeBase) error {
	return r.db.Save(base).Error
}

// DeleteByKBID 级联删除知识库：在同一数据库事务中删除分块、文件、处理任务和知识库记录。
// 调用方负责先清理对象存储中的文件对象。
func (r *knowledgeBaseRepository) DeleteByKBID(kbID string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("kb_id = ?", kbID).Delete(&entity.KnowledgeChunk{}).Error; err != nil {
			return err
		}
		if err := tx.Where("kb_id = ?", kbID).Delete(&entity.KnowledgeFile{}).Error; err != nil {
			return err
		}
		if err := tx.Where("kb_id = ?", kbID).Delete(&entity.DocumentProcessingJob{}).Error; err != nil {
			return err
		}
		return tx.Where("kb_id = ?", kbID).Delete(&entity.KnowledgeBase{}).Error
	})
}

// GetStatsByKBIDs 批量获取知识库统计信息
func (r *knowledgeBaseRepository) GetStatsByKBIDs(kbIDs []string) (map[string]*KnowledgeBaseStats, error) {
	if len(kbIDs) == 0 {
		return make(map[string]*KnowledgeBaseStats), nil
	}

	statsMap := make(map[string]*KnowledgeBaseStats, len(kbIDs))
	for _, kbID := range kbIDs {
		statsMap[kbID] = &KnowledgeBaseStats{}
	}

	// 统计文件数量和大小
	type fileStats struct {
		KBID          string
		FileCount     int64
		TotalSize     int64
		ChunkCount    int64
		TokenCount    int64
		ProcessingCnt int
		PendingParse  int
		PendingIndex  int
	}

	var fileResults []fileStats
	err := r.db.Model(&entity.KnowledgeFile{}).
		Select(`
			kb_id,
			COUNT(*) as file_count,
			COALESCE(SUM(file_size), 0) as total_size,
			COALESCE(SUM(chunk_count), 0) as chunk_count,
			COALESCE(SUM(token_count), 0) as token_count,
			SUM(CASE WHEN status IN ('parsing', 'indexing') THEN 1 ELSE 0 END) as processing_cnt,
			SUM(CASE WHEN status = 'parse_queued' THEN 1 ELSE 0 END) as pending_parse,
			SUM(CASE WHEN status = 'index_queued' THEN 1 ELSE 0 END) as pending_index
		`).
		Where("kb_id IN ?", kbIDs).
		Group("kb_id").
		Find(&fileResults).Error
	if err != nil {
		return nil, err
	}

	for _, result := range fileResults {
		if stats, ok := statsMap[result.KBID]; ok {
			stats.FileCount = result.FileCount
			stats.TotalSize = result.TotalSize
			stats.ChunkCount = result.ChunkCount
			stats.TokenCount = result.TokenCount
			stats.ProcessingCount = result.ProcessingCnt
			stats.PendingParseCount = result.PendingParse
			stats.PendingIndexCount = result.PendingIndex
		}
	}

	return statsMap, nil
}
