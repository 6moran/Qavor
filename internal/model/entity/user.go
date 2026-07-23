package entity

// User 用户实体
type User struct {
	BaseEntity
	Username string `gorm:"type:varchar(50);uniqueIndex;not null;comment:用户名" json:"username"`
	Password string `gorm:"type:varchar(255);not null;comment:密码" json:"-"`
	Email    string `gorm:"type:varchar(100);uniqueIndex;comment:邮箱" json:"email"`
	Nickname string `gorm:"type:varchar(50);comment:昵称" json:"nickname"`
	Avatar   string `gorm:"type:varchar(255);comment:头像" json:"avatar"`
	Status   int    `gorm:"type:tinyint;default:1;comment:状态:1正常,2禁用" json:"status"`
}

// TableName 指定表名
func (User) TableName() string {
	return "users"
}
