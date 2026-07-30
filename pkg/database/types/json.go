package types

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// JSON 自定义 JSON 类型，用于 GORM 的 JSON 序列化
type JSON json.RawMessage

// Value 实现 driver.Valuer 接口
func (j JSON) Value() (driver.Value, error) {
	if j == nil {
		return "{}", nil
	}
	return json.Marshal(j)
}

// Scan 实现 sql.Scanner 接口
func (j *JSON) Scan(src interface{}) error {
	if src == nil {
		*j = make(JSON, 0)
		return nil
	}
	bytes, ok := src.([]byte)
	if !ok {
		return fmt.Errorf("failed to scan JSON: expected []byte, got %T", src)
	}
	return json.Unmarshal(bytes, j)
}

// StringMap 自定义 map[string]string 类型，用于 GORM 的 JSON 序列化
type StringMap map[string]string

// Value 实现 driver.Valuer 接口
func (s StringMap) Value() (driver.Value, error) {
	if s == nil {
		return "{}", nil
	}
	return json.Marshal(s)
}

// Scan 实现 sql.Scanner 接口
func (s *StringMap) Scan(src interface{}) error {
	if src == nil {
		*s = make(StringMap)
		return nil
	}
	bytes, ok := src.([]byte)
	if !ok {
		return fmt.Errorf("failed to scan StringMap: expected []byte, got %T", src)
	}
	return json.Unmarshal(bytes, s)
}

// JSONArray 自定义 []interface{} 类型，用于 GORM 的 JSON 序列化
type JSONArray []interface{}

// Value 实现 driver.Valuer 接口
func (a JSONArray) Value() (driver.Value, error) {
	if a == nil {
		return "[]", nil
	}
	return json.Marshal(a)
}

// Scan 实现 sql.Scanner 接口
func (a *JSONArray) Scan(src interface{}) error {
	if src == nil {
		*a = make(JSONArray, 0)
		return nil
	}
	bytes, ok := src.([]byte)
	if !ok {
		return fmt.Errorf("failed to scan JSONArray: expected []byte, got %T", src)
	}
	return json.Unmarshal(bytes, a)
}
