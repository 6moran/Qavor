package skill

import (
	"errors"
	"strings"

	"Qavor/internal/model/entity"

	"gorm.io/gorm"
)

// SkillRepository Skill 数据访问接口
type SkillRepository interface {
	FindByID(id uint) (*entity.Skill, error)
	FindBySlug(slug string) (*entity.Skill, error)
	List(offset, limit int, keyword string) ([]*entity.Skill, int64, error)
	ListAll() ([]*entity.Skill, error)
	Create(skill *entity.Skill) error
	Update(skill *entity.Skill) error
	Delete(slug string) error
}

type skillRepository struct {
	db *gorm.DB
}

// NewSkillRepository 创建 SkillRepository
func NewSkillRepository(db *gorm.DB) SkillRepository {
	return &skillRepository{db: db}
}

func (r *skillRepository) FindByID(id uint) (*entity.Skill, error) {
	var skill entity.Skill
	err := r.db.First(&skill, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &skill, nil
}

func (r *skillRepository) FindBySlug(slug string) (*entity.Skill, error) {
	var skill entity.Skill
	err := r.db.Where("slug = ?", slug).First(&skill).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &skill, nil
}

func (r *skillRepository) List(offset, limit int, keyword string) ([]*entity.Skill, int64, error) {
	query := r.db.Model(&entity.Skill{})

	if keyword = strings.TrimSpace(keyword); keyword != "" {
		query = query.Where("slug ILIKE ? OR name ILIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var skills []*entity.Skill
	if err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&skills).Error; err != nil {
		return nil, 0, err
	}
	return skills, total, nil
}

func (r *skillRepository) ListAll() ([]*entity.Skill, error) {
	var skills []*entity.Skill
	err := r.db.Where("enabled = ?", true).Order("name ASC").Find(&skills).Error
	if err != nil {
		return nil, err
	}
	return skills, nil
}

func (r *skillRepository) Create(skill *entity.Skill) error {
	return r.db.Create(skill).Error
}

func (r *skillRepository) Update(skill *entity.Skill) error {
	return r.db.Save(skill).Error
}

func (r *skillRepository) Delete(slug string) error {
	return r.db.Where("slug = ?", slug).Delete(&entity.Skill{}).Error
}
