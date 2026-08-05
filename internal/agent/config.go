package agent

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
)

// AgentConfig Agent 运行时配置（只包含智能体相关配置）
type AgentConfig struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Instruction string `json:"instruction"`
	ModelID     string `json:"model_id,omitempty"` // 关联的模型 ID

	// 模型参数
	Temperature *float64 `json:"temperature,omitempty"` // 温度参数（0-2）
	MaxTokens   *int     `json:"max_tokens,omitempty"`  // 最大输出 token 数

	// 工具相关
	Tools                  []string `json:"tools,omitempty"`       // 关联的工具列表
	MCPServers             []string `json:"mcp_servers,omitempty"` // 关联的 MCP 服务器列表
	ToolRetrievalEnabled   bool     `json:"tool_retrieval_enabled"`
	ToolRetrievalThreshold int      `json:"tool_retrieval_threshold,omitempty"`
	ToolRetrievalTopK      int      `json:"tool_retrieval_top_k,omitempty"`

	// 扩展相关
	Skills     []string `json:"skills,omitempty"`     // 关联的 Skill 列表
	Knowledges []string `json:"knowledges,omitempty"` // 关联的知识库列表
	Subagents  []string `json:"subagents,omitempty"`  // 关联的子智能体列表

	// 智能体配置
	MaxIteration int `json:"max_iteration,omitempty"` // Deep Agent 最大迭代次数

	// EnableGeneralSubAgent 是否允许主智能体自动创建通用子智能体（默认 true，用户可关闭）
	// nil 表示未配置，默认启用
	EnableGeneralSubAgent *bool `json:"enable_general_subagent"`

	// IsSubagent 是否为子智能体（运行时从 entity 注入，不持久化、不暴露给前端）
	IsSubagent bool `json:"-"`

	// Slug 智能体唯一标识（运行时从 entity 注入，用于派生沙箱目录）
	Slug string `json:"-"`
}

// Hash 计算配置哈希，用于缓存比较
func (c *AgentConfig) Hash() string {
	data, _ := json.Marshal(c)
	hash := sha256.Sum256(data)
	return fmt.Sprintf("%x", hash)
}
