package entity

import (
	"Qavor/pkg/database/types"
)

// Model 模型实体（单一实体，包含所有连接信息）
type Model struct {
	BaseEntity
	Name            string            `gorm:"type:varchar(100);not null;comment:服务商展示名" json:"name"`
	Remark          string            `gorm:"type:varchar(255);comment:模型配置备注" json:"remark,omitempty"`
	Protocol        string            `gorm:"type:varchar(32);not null;default:openai;comment:协议类型" json:"protocol"`
	BaseURL         string            `gorm:"type:varchar(500);not null;comment:API基础URL" json:"base_url"`
	APIKey          string            `gorm:"type:varchar(500);comment:密钥（加密存储）" json:"api_key,omitempty"`
	Headers         types.StringMap   `gorm:"type:json;default:'{}';comment:自定义请求头" json:"headers,omitempty"`
	Timeout         int               `gorm:"not null;default:60000;comment:超时时间(ms)" json:"timeout"`
	Enabled         bool              `gorm:"not null;default:true;index;comment:是否启用" json:"enabled"`
	ModelType       string            `gorm:"type:varchar(32);not null;default:chat;comment:模型类型(chat/embedding/rerank)" json:"model_type"`
	Params          types.ModelParams `gorm:"type:json;default:'{}';comment:模型推理参数" json:"params"`
	ContextWindow   int               `gorm:"not null;default:0;comment:上下文窗口大小(token数，0表示使用默认值)" json:"context_window"`
	MaxOutputTokens int               `gorm:"not null;default:0;comment:最大输出token数(0表示使用默认值)" json:"max_output_tokens"`
}

// TableName 指定表名
func (Model) TableName() string {
	return "models"
}
