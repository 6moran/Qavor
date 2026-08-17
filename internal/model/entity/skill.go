package entity

// Skill 技能实体
type Skill struct {
	BaseEntity
	Slug        string `gorm:"type:varchar(128);uniqueIndex;not null;comment:技能唯一标识（目录名）" json:"slug"`
	Name        string `gorm:"type:varchar(128);not null;comment:技能名称" json:"name"`
	Description string `gorm:"type:text;not null;comment:技能描述" json:"description"`
	SourceType  string `gorm:"type:varchar(32);not null;index;default:upload;comment:来源：builtin/upload/remote" json:"source_type"`
	DirPath     string `gorm:"type:varchar(512);not null;comment:技能目录路径（相对save_dir）" json:"dir_path"`
	Version     string `gorm:"type:varchar(64);comment:版本号（语义化版本）" json:"version,omitempty"`
	ContentHash string `gorm:"type:varchar(128);comment:技能目录内容哈希" json:"content_hash,omitempty"`
	Enabled     bool   `gorm:"not null;default:true;comment:是否启用" json:"enabled"`
}

// TableName 指定表名
func (Skill) TableName() string {
	return "skills"
}
