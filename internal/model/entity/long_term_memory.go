package entity

import (
	"time"

	"gorm.io/gorm"
)

// LongTermMemory 长期记忆实体（用户级，跨会话持久化）
// Category 见 internal/memory/types/memory.go 的常量：
// preference（偏好）/ identity（画像/身份）/ environment（环境）/ knowledge（项目知识）/ sustainable_task（持续性任务）/ decision（决策）
type LongTermMemory struct {
	BaseEntity

	UserID               uint           `gorm:"index;not null;comment:用户ID，0 表示全局/匿名（JWT未携带用户ID时的降级）" json:"user_id"`
	Category             string         `gorm:"type:varchar(32);not null;index;comment:记忆类别 preference/identity/environment/knowledge/sustainable_task/decision" json:"category"`
	Content              string         `gorm:"type:text;not null;comment:记忆正文" json:"content"`
	Importance           float64        `gorm:"not null;default:0;comment:重要性 0.0~1.0" json:"importance"`
	Confidence           float64        `gorm:"not null;default:0;comment:LLM 抽取置信度 0.0~1.0" json:"confidence"`
	SourceConversationID uint           `gorm:"index;comment:来源会话ID（可追溯）" json:"source_conversation_id,omitempty"`
	SourceRunID          string         `gorm:"type:varchar(64);comment:来源 Run ID" json:"source_run_id,omitempty"`
	LastRecalledAt       *time.Time     `gorm:"comment:上次召回时间" json:"last_recalled_at,omitempty"`
	RecallCount          int            `gorm:"not null;default:0;comment:召回次数，用于动态权重" json:"recall_count"`
	IsSuppressed         bool           `gorm:"not null;default:false;index;comment:用户/LLM判定过时后压制" json:"is_suppressed"`
	DeletedAt            gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName 指定表名
func (LongTermMemory) TableName() string {
	return "long_term_memories"
}
