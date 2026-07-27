package entity

import "time"

// APIKey API密钥实体
type APIKey struct {
	ID         uint       `gorm:"primarykey" json:"id"`
	KeyHash    string     `gorm:"type:varchar(64);uniqueIndex;not null;comment:密钥哈希" json:"key_hash"`
	KeyPrefix  string     `gorm:"type:varchar(16);not null;comment:密钥前缀（用于展示）" json:"key_prefix"`
	Name       string     `gorm:"type:varchar(100);not null;comment:密钥名称" json:"name"`
	UserID     *uint      `gorm:"index;comment:所属用户ID" json:"user_id,omitempty"`
	ExpiresAt  *time.Time `gorm:"comment:过期时间" json:"expires_at,omitempty"`
	IsEnabled  bool       `gorm:"not null;default:true;comment:是否启用" json:"is_enabled"`
	LastUsedAt *time.Time `gorm:"comment:最后使用时间" json:"last_used_at,omitempty"`
	CreatedBy  string     `gorm:"type:varchar(64);not null;comment:创建人" json:"created_at_by"`
	CreatedAt  time.Time  `json:"created_at"`

	// 关联关系
	User *User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

// TableName 指定表名
func (APIKey) TableName() string {
	return "api_keys"
}

// IsValid 检查密钥是否有效
func (k *APIKey) IsValid() bool {
	if !k.IsEnabled {
		return false
	}
	if k.ExpiresAt != nil && time.Now().After(*k.ExpiresAt) {
		return false
	}
	return true
}
