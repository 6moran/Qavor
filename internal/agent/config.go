package agent

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
)

// AgentConfig Agent 运行时配置
type AgentConfig struct {
	Name                   string            `json:"name"`
	Description            string            `json:"description"`
	Instruction            string            `json:"instruction"`
	ProviderID             string            `json:"provider_id,omitempty"`
	ModelName              string            `json:"model_name,omitempty"`
	Tools                  []string          `json:"tools,omitempty"`       // 内置工具名列表
	MCPServers             []string          `json:"mcp_servers,omitempty"` // 需要的 MCP 服务器名列表
	DisabledTools          []string          `json:"disabled_tools,omitempty"`
	MaxTokens              int               `json:"max_tokens,omitempty"`
	Temperature            float64           `json:"temperature,omitempty"`
	Metadata               map[string]string `json:"metadata,omitempty"`
	ToolRetrievalEnabled   bool              `json:"tool_retrieval_enabled,omitempty"`
	ToolRetrievalThreshold int               `json:"tool_retrieval_threshold,omitempty"`
	ToolRetrievalTopK      int               `json:"tool_retrieval_top_k,omitempty"`
	Skills                 []string          `json:"skills,omitempty"` // 选中的 Skill slug 列表
}

// Hash 计算配置哈希，用于缓存比较
func (c *AgentConfig) Hash() string {
	data, _ := json.Marshal(c)
	hash := sha256.Sum256(data)
	return fmt.Sprintf("%x", hash)
}
