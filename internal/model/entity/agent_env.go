package entity

import "time"

// AgentEnv Agent环境变量实体
type AgentEnv struct {
	BaseEntity
	Env       JSON      `gorm:"type:json;not null;default:{};comment:环境变量配置" json:"env"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName 指定表名
func (AgentEnv) TableName() string {
	return "agent_envs"
}
