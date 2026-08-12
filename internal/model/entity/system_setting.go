package entity

// SystemSetting 系统键值设置实体。
type SystemSetting struct {
	BaseEntity
	Key   string `gorm:"type:varchar(128);uniqueIndex;not null" json:"key"`
	Value string `gorm:"type:text;not null" json:"value"`
}

// TableName 指定系统设置表名。
func (SystemSetting) TableName() string {
	return "system_settings"
}
