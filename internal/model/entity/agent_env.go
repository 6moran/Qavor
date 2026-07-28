package entity

import "time"

// AgentEnv Agent环境变量实体
type AgentEnv struct {
	BaseEntity
	UID       string    `gorm:"type:varchar(100);uniqueIndex;not null;comment:用户UID" json:"uid"`
	Env       JSON      `gorm:"type:json;not null;default:{};comment:环境变量配置" json:"env"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// 关联关系
	User *User `gorm:"foreignKey:UID;references:UID" json:"user,omitempty"`
}

// TableName 指定表名
func (AgentEnv) TableName() string {
	return "agent_envs"
}
