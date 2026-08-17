package repository

import (
	"context"

	"Qavor/internal/model/entity"
)

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
	// UnbindKnowledge 移除所有智能体配置中对指定知识库的绑定；
	// 返回受影响（被修改并保存）的智能体数量，未绑定的智能体不计入。
	UnbindKnowledge(ctx context.Context, kbID string) (int64, error)
}
