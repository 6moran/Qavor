package entity

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

// BaseEntity 基础实体
type BaseEntity struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// BeforeCreate GORM hook - 创建前
func (b *BaseEntity) BeforeCreate(tx *gorm.DB) error {
	return nil
}

// BeforeUpdate GORM hook - 更新前
func (b *BaseEntity) BeforeUpdate(tx *gorm.DB) error {
	return nil
}

// JSON 自定义JSON类型
type JSON map[string]interface{}

// Value 实现 driver.Valuer 接口
func (j JSON) Value() (driver.Value, error) {
	if j == nil {
		return "{}", nil
	}
	return json.Marshal(j)
}

// Scan 实现 sql.Scanner 接口
func (j *JSON) Scan(value interface{}) error {
	if value == nil {
		*j = make(JSON)
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		bytes = []byte(value.(string))
	}
	return json.Unmarshal(bytes, j)
}

// JSONArray 自定义JSON数组类型
type JSONArray []interface{}

// Value 实现 driver.Valuer 接口
func (a JSONArray) Value() (driver.Value, error) {
	if a == nil {
		return "[]", nil
	}
	return json.Marshal(a)
}

// Scan 实现 sql.Scanner 接口
func (a *JSONArray) Scan(value interface{}) error {
	if value == nil {
		*a = make(JSONArray, 0)
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		bytes = []byte(value.(string))
	}
	return json.Unmarshal(bytes, a)
}
