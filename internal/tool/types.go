package tool

import (
	"context"
	"errors"
)

// Category 工具分类
type Category string

const (
	CategorySystem    Category = "builtin"
	CategoryKnowledge Category = "knowledge"
	CategoryPlatform  Category = "platform"
	CategoryDebug     Category = "debug"
)

// QueryKBToolName 知识库检索工具名。
const QueryKBToolName = "query_kb"

// ToolMeta 工具元数据
type ToolMeta struct {
	Name        string   `json:"name"`
	Label       string   `json:"label,omitempty"`
	Description string   `json:"description"`
	Category    Category `json:"category"`
	Tags        []string `json:"tags,omitempty"`
	Args        []ArgDef `json:"args,omitempty"`
	ConfigGuide string   `json:"config_guide,omitempty"`
	Sensitive   bool     `json:"sensitive,omitempty"`
}

// ArgDef 参数定义
type ArgDef struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
}

// BuiltinTool 内置工具接口
type BuiltinTool interface {
	Meta() ToolMeta
	Execute(ctx context.Context, args map[string]any) (any, error)
}

// SensitiveTools 敏感工具列表，执行前需要用户审批
var SensitiveTools = map[string]bool{
	"execute": true,
}

// 错误定义
var (
	ErrToolNotFound         = errors.New("tool not found")
	ErrToolExecutionTimeout = errors.New("tool execution timeout")
	ErrToolExecutionFailed  = errors.New("tool execution failed")
)

// ToolProvider 工具提供者接口，用于避免导入循环
type ToolProvider interface {
	GetTools() []BuiltinTool
}
