package agent

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
)

// AgentConfig Agent 运行时配置
type AgentConfig struct {
	Name          string            `json:"name"`
	Description   string            `json:"description"`
	Instruction   string            `json:"instruction"`
	ProviderID    string            `json:"provider_id,omitempty"`
	ModelName     string            `json:"model_name,omitempty"`
	Tools         []string          `json:"tools,omitempty"`
	DisabledTools []string          `json:"disabled_tools,omitempty"`
	MaxTokens     int               `json:"max_tokens,omitempty"`
	Temperature   float64           `json:"temperature,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

// Hash 计算配置哈希，用于缓存比较
func (c *AgentConfig) Hash() string {
	data, _ := json.Marshal(c)
	hash := sha256.Sum256(data)
	return fmt.Sprintf("%x", hash)
}
