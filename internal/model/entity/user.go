package entity

import (
	"time"
)

// User 用户实体
type User struct {
	BaseEntity
	Nickname         string     `gorm:"type:varchar(100);not null;comment:昵称（用于显示）" json:"nickname"`
	UID              string     `gorm:"type:varchar(100);uniqueIndex;not null;comment:用户唯一标识（系统生成）" json:"uid"`
	Email            string     `gorm:"type:varchar(100);uniqueIndex;not null;comment:邮箱（用于登录）" json:"email"`
	PhoneNumber      string     `gorm:"type:varchar(20);uniqueIndex;comment:手机号" json:"phone_number,omitempty"`
	Avatar           string     `gorm:"type:varchar(255);comment:头像URL" json:"avatar,omitempty"`
	Password         string     `gorm:"type:varchar(255);not null;comment:密码哈希" json:"-"`
	Status           int        `gorm:"not null;default:1;comment:状态：1=正常，0=禁用" json:"status"`
	Role             string     `gorm:"type:varchar(20);not null;default:user;comment:角色：superadmin/admin/user" json:"role"`
	LastLogin        *time.Time `gorm:"comment:最后登录时间" json:"last_login,omitempty"`
	LoginFailedCount int        `gorm:"not null;default:0;comment:登录失败次数" json:"login_failed_count"`
	LastFailedLogin  *time.Time `gorm:"comment:最后一次登录失败时间" json:"last_failed_login,omitempty"`
	LoginLockedUntil *time.Time `gorm:"comment:账户锁定截止时间" json:"login_locked_until,omitempty"`
	IsDeleted        int        `gorm:"not null;default:0;index;comment:软删除标记：0=正常，1=已删除" json:"is_deleted"`
	DeletedAt        *time.Time `gorm:"comment:删除时间" json:"deleted_at,omitempty"`

	// 关联关系
	OperationLogs []OperationLog `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"operation_logs,omitempty"`
	APIKeys       []APIKey       `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"api_keys,omitempty"`
	AgentEnv      *AgentEnv      `gorm:"foreignKey:UID;references:UID;constraint:OnDelete:CASCADE" json:"agent_env,omitempty"`
}

// TableName 指定表名
func (User) TableName() string {
	return "users"
}

// IsLoginLocked 检查账户是否锁定
func (u *User) IsLoginLocked() bool {
	if u.LoginLockedUntil == nil {
		return false
	}
	return time.Now().Before(*u.LoginLockedUntil)
}

// GetRemainingLockTime 获取剩余锁定时间（秒）
func (u *User) GetRemainingLockTime() int {
	if u.LoginLockedUntil == nil {
		return 0
	}
	remaining := time.Until(*u.LoginLockedUntil).Seconds()
	if remaining < 0 {
		return 0
	}
	return int(remaining)
}

// IncrementFailedLogin 增加登录失败计数
func (u *User) IncrementFailedLogin() {
	u.LoginFailedCount++
	now := time.Now()
	u.LastFailedLogin = &now

	// 超过最大失败次数，锁定账户
	const maxLoginFailedAttempts = 5
	const loginLockDurationSeconds = 300
	if u.LoginFailedCount >= maxLoginFailedAttempts {
		lockUntil := now.Add(time.Duration(loginLockDurationSeconds) * time.Second)
		u.LoginLockedUntil = &lockUntil
	}
}

// ResetFailedLogin 重置登录失败状态
func (u *User) ResetFailedLogin() {
	u.LoginFailedCount = 0
	u.LastFailedLogin = nil
	u.LoginLockedUntil = nil
}
