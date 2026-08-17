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

// AskUserToolName 向用户提问工具名（仅主智能体使用）。
const AskUserToolName = "ask_user"

// ReportNeedInputToolName 子智能体上报需要用户输入的工具名（仅子智能体使用）。
// 子智能体通过此工具触发中断，中断通过 eino 中断树传播到主智能体，
// 主智能体的 ApprovalMiddleware 识别后发起 ask_user 中断。
const ReportNeedInputToolName = "report_need_input"

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

// ArgDef 参数定义，支持嵌套结构（数组元素类型、对象子参数）。
type ArgDef struct {
	Name        string              `json:"name"`
	Type        string              `json:"type"`
	Description string              `json:"description"`
	Required    bool                `json:"required"`
	Enum        []string            `json:"enum,omitempty"`       // 枚举值（仅 string 类型有效）
	ElemInfo    *ArgDef             `json:"elem_info,omitempty"`  // 数组元素类型（仅 array 类型有效）
	SubParams   map[string]*ArgDef  `json:"sub_params,omitempty"` // 对象子参数（仅 object 类型有效）
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
