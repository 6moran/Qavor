package types

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// ModelParams 模型默认推理参数
type ModelParams struct {
	MaxTokens        int      `json:"max_tokens"`
	Temperature      float64  `json:"temperature"`
	TopP             float64  `json:"top_p"`
	PresencePenalty  float64  `json:"presence_penalty"`
	FrequencyPenalty float64  `json:"frequency_penalty"`
	Stop             []string `json:"stop,omitempty"`
}

// Value 实现 driver.Valuer 接口
func (p ModelParams) Value() (driver.Value, error) {
	return json.Marshal(p)
}

// Scan 实现 sql.Scanner 接口
func (p *ModelParams) Scan(src interface{}) error {
	if src == nil {
		return nil
	}
	bytes, ok := src.([]byte)
	if !ok {
		return fmt.Errorf("failed to scan ModelParams: expected []byte, got %T", src)
	}
	return json.Unmarshal(bytes, p)
}

// DefaultModelParams 返回默认的模型参数
func DefaultModelParams() ModelParams {
	return ModelParams{
		MaxTokens:        4096,
		Temperature:      0.7,
		TopP:             1.0,
		PresencePenalty:  0,
		FrequencyPenalty: 0,
	}
}
