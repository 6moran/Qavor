package entity

// Agent 智能体实体
type Agent struct {
	BaseEntity
	Slug        string    `gorm:"type:varchar(80);uniqueIndex;not null;comment:智能体唯一标识" json:"slug"`
	BackendID   string    `gorm:"type:varchar(64);not null;index;comment:后端实现ID" json:"backend_id"`
	Name        string    `gorm:"type:varchar(100);not null;comment:智能体名称" json:"name"`
	Description string    `gorm:"type:text;comment:描述" json:"description,omitempty"`
	Icon        string    `gorm:"type:varchar(255);comment:图标URL" json:"icon,omitempty"`
	Pics        JSONArray `gorm:"type:json;not null;default:'[]';comment:展示图片列表" json:"pics"`
	ConfigJSON  JSON      `gorm:"type:json;not null;default:{};comment:配置信息" json:"config_json"`
	ShareConfig JSON      `gorm:"type:json;not null;default:{};comment:共享配置" json:"share_config"`
	IsDefault   bool      `gorm:"not null;default:false;index;comment:是否为默认智能体" json:"is_default"`
	IsSubagent  bool      `gorm:"not null;default:false;index;comment:是否为子智能体" json:"is_subagent"`
	CreatedBy   string    `gorm:"type:varchar(64);index;comment:创建人" json:"created_by,omitempty"`
	UpdatedBy   string    `gorm:"type:varchar(64);comment:更新人" json:"updated_by,omitempty"`
}

// TableName 指定表名
func (Agent) TableName() string {
	return "agents"
}
