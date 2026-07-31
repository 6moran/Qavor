package repository

import "Qavor/internal/model/entity"

// AgentRepository 智能体数据访问接口
type AgentRepository interface {
	Create(agent *entity.Agent) error
	GetBySlug(slug string) (*entity.Agent, error)
	GetDefault() (*entity.Agent, error)
	Update(agent *entity.Agent) error
	Delete(slug string) error
	List(offset, limit int, keyword string) ([]*entity.Agent, int64, error)
	SetDefault(slug string) error
	ClearDefault() error
}
