package repository

import (
	"errors"

	"Qavor/internal/model/entity"

	"gorm.io/gorm"
)

// knowledgeFileRepository 知识文件数据访问实现
type knowledgeFileRepository struct {
	db *gorm.DB
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
