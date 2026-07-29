package entity

import "time"

// OperationLog 操作日志实体
type OperationLog struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	Operation string    `gorm:"type:varchar(100);not null;comment:操作类型" json:"operation"`
	Details   string    `gorm:"type:text;comment:操作详情" json:"details,omitempty"`
	IPAddress string    `gorm:"type:varchar(50);comment:操作IP地址" json:"ip_address,omitempty"`
	Timestamp time.Time `gorm:"comment:操作时间" json:"timestamp"`
}

// TableName 指定表名
func (OperationLog) TableName() string {
	return "operation_logs"
}
