package repository

import (
	"context"
	"errors"
	"strings"

	"Qavor/internal/model/entity"

	"gorm.io/gorm"
)

// knowledgeFileRepository 知识文件数据访问实现
type knowledgeFileRepository struct {
	db *gorm.DB
}

// SearchByKBID 按文件名、原始文件名或路径进行不区分大小写的模糊检索。
func (r *knowledgeFileRepository) SearchByKBID(kbID, keyword string, offset, limit int) ([]*entity.KnowledgeFile, int64, error) {
	pattern := "%" + strings.ReplaceAll(strings.ReplaceAll(keyword, "%", "\\%"), "_", "\\_") + "%"
	query := r.db.Model(&entity.KnowledgeFile{}).
		Where("kb_id = ?", kbID).
		Where("filename ILIKE ? ESCAPE '\\' OR original_filename ILIKE ? ESCAPE '\\' OR path ILIKE ? ESCAPE '\\'", pattern, pattern, pattern)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var files []*entity.KnowledgeFile
	if err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&files).Error; err != nil {
		return nil, 0, err
	}
	return files, total, nil
}

// NewKnowledgeFileRepository 创建知识文件数据访问实例
func NewKnowledgeFileRepository(db *gorm.DB) KnowledgeFileRepository {
	return &knowledgeFileRepository{db: db}
}

// Create 创建文件记录
func (r *knowledgeFileRepository) Create(file *entity.KnowledgeFile) error {
	return r.db.Create(file).Error
}

// FindByKBIDAndFileID 根据知识库ID和文件ID查询
func (r *knowledgeFileRepository) FindByKBIDAndFileID(kbID, fileID string) (*entity.KnowledgeFile, error) {
	var file entity.KnowledgeFile
	// 联合查询知识库ID和文件ID，记录不存在时返回 nil
	err := r.db.Where("kb_id = ? AND file_id = ?", kbID, fileID).First(&file).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &file, nil
}

// ListByKBID 分页查询知识库下的文件列表
func (r *knowledgeFileRepository) ListByKBID(kbID string, offset, limit int, parentID, pathPrefix string, recursive bool, status string) ([]*entity.KnowledgeFile, int64, error) {
	// 按知识库ID过滤
	query := r.db.Model(&entity.KnowledgeFile{}).Where("kb_id = ?", kbID)
	// path_prefix 用于路径型虚拟目录；未传路径前缀且非递归查询时，只返回指定父目录的直接子项。
	if pathPrefix != "" {
		query = query.Where("path LIKE ?", pathPrefix+"%")
	} else if !recursive {
		query = query.Where("parent_id = ?", parentID)
	}
	// 按状态过滤（空字符串表示不过滤）
	if status != "" {
		query = query.Where("status = ?", status)
	}
	// 查询总数
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	// 分页查询列表，按创建时间倒序
	var files []*entity.KnowledgeFile
	if err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&files).Error; err != nil {
		return nil, 0, err
	}
	return files, total, nil
}

// DeleteByKBIDAndFileID 根据知识库ID和文件ID删除
func (r *knowledgeFileRepository) DeleteByKBIDAndFileID(kbID, fileID string) error {
	return r.db.Where("kb_id = ? AND file_id = ?", kbID, fileID).Delete(&entity.KnowledgeFile{}).Error
}

// DeleteWithChunks 在一个数据库事务中删除文件的分块和文件记录。
func (r *knowledgeFileRepository) DeleteWithChunks(ctx context.Context, kbID, fileID string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("kb_id = ? AND file_id = ?", kbID, fileID).Delete(&entity.KnowledgeChunk{}).Error; err != nil {
			return err
		}
		return tx.Where("kb_id = ? AND file_id = ?", kbID, fileID).Delete(&entity.KnowledgeFile{}).Error
	})
}

func (r *knowledgeFileRepository) UpdateProcessingResult(kbID, fileID, status, markdownFile, errorMessage string) error {
	return r.db.Model(&entity.KnowledgeFile{}).Where("kb_id = ? AND file_id = ?", kbID, fileID).Updates(map[string]any{"status": status, "markdown_file": markdownFile, "error_message": errorMessage}).Error
}

// TransitionStatus 比较并设置文件状态，从允许的状态之一转换到目标状态。
func (r *knowledgeFileRepository) TransitionStatus(ctx context.Context, kbID, fileID string, from []string, to string, updates map[string]any) (bool, error) {
	sets := map[string]any{"status": to}
	for k, v := range updates {
		sets[k] = v
	}
	result := r.db.WithContext(ctx).
		Model(&entity.KnowledgeFile{}).
		Where("kb_id = ? AND file_id = ? AND status IN ?", kbID, fileID, from).
		Updates(sets)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
}

// ListByKBIDAndStatuses 返回指定知识库中匹配任一状态的文件，最多返回 limit 条。
func (r *knowledgeFileRepository) ListByKBIDAndStatuses(ctx context.Context, kbID string, statuses []string, limit int) ([]*entity.KnowledgeFile, error) {
	var files []*entity.KnowledgeFile
	if err := r.db.WithContext(ctx).
		Where("kb_id = ? AND status IN ?", kbID, statuses).
		Order("created_at ASC").
		Limit(limit).
		Find(&files).Error; err != nil {
		return nil, err
	}
	return files, nil
}
