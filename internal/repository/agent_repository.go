package repository

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"Qavor/internal/model/entity"
	"Qavor/pkg/database"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"Qavor/pkg/logger"
)

const agentCachePrefix = "agent:"
const agentCacheTTL = 5 * time.Minute

type agentRepository struct {
	db *gorm.DB
}

func NewAgentRepository(db *gorm.DB) AgentRepository {
	return &agentRepository{db: db}
}

func (r *agentRepository) Create(agent *entity.Agent) error {
	return r.db.Create(agent).Error
}

func (r *agentRepository) GetBySlug(slug string) (*entity.Agent, error) {
	// 尝试从 Redis 读取
	if database.RedisAvailable() {
		ctx := context.Background()
		data, err := database.GetRedis().Get(ctx, agentCachePrefix+slug).Bytes()
		if err == nil && len(data) > 0 {
			var agent entity.Agent
			if json.Unmarshal(data, &agent) == nil {
				return &agent, nil
			}
		}
	}

	// 读数据库
	var agent entity.Agent
	err := r.db.Where("slug = ?", slug).First(&agent).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	// 写入 Redis 缓存
	r.setCache(slug, &agent)

	return &agent, nil
}

func (r *agentRepository) GetDefault() (*entity.Agent, error) {
	var agent entity.Agent
	err := r.db.Where("is_default = ?", true).First(&agent).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &agent, nil
}

func (r *agentRepository) Update(agent *entity.Agent) error {
	if err := r.db.Save(agent).Error; err != nil {
		return err
	}
	r.deleteCache(agent.Slug)
	return nil
}

func (r *agentRepository) Delete(slug string) error {
	if err := r.db.Where("slug = ?", slug).Delete(&entity.Agent{}).Error; err != nil {
		return err
	}
	r.deleteCache(slug)
	return nil
}

func (r *agentRepository) List(offset, limit int, keyword string) ([]*entity.Agent, int64, error) {
	query := r.db.Model(&entity.Agent{})

	if keyword = strings.TrimSpace(keyword); keyword != "" {
		query = query.Where("slug ILIKE ? OR name ILIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var agents []*entity.Agent
	if err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&agents).Error; err != nil {
		return nil, 0, err
	}
	return agents, total, nil
}

func (r *agentRepository) SetDefault(slug string) error {
	return r.db.Model(&entity.Agent{}).Where("slug = ?", slug).Update("is_default", true).Error
}

func (r *agentRepository) ClearDefault() error {
	return r.db.Model(&entity.Agent{}).Where("is_default = ?", true).Update("is_default", false).Error
}

func (r *agentRepository) setCache(slug string, agent *entity.Agent) {
	if !database.RedisAvailable() {
		return
	}
	data, err := json.Marshal(agent)
	if err != nil {
		return
	}
	ctx := context.Background()
	if err := database.GetRedis().Set(ctx, agentCachePrefix+slug, data, agentCacheTTL).Err(); err != nil {
		logger.Warn("Redis 写入缓存失败", zap.String("slug", slug), zap.Error(err))
	}
}

func (r *agentRepository) deleteCache(slug string) {
	if !database.RedisAvailable() {
		return
	}
	ctx := context.Background()
	if err := database.GetRedis().Del(ctx, agentCachePrefix+slug).Err(); err != nil {
		logger.Warn("Redis 删除缓存失败", zap.String("slug", slug), zap.Error(err))
	}
}
