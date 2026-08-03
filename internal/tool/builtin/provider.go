package builtin

import (
	"Qavor/internal/tool"
)

// BuiltinToolProvider 内置工具提供者
type BuiltinToolProvider struct{}

// NewBuiltinToolProvider 创建内置工具提供者
func NewBuiltinToolProvider() *BuiltinToolProvider {
	return &BuiltinToolProvider{}
}

// GetTools 获取所有内置工具
func (p *BuiltinToolProvider) GetTools() []tool.BuiltinTool {
	return []tool.BuiltinTool{
		&CalculatorTool{},
		&CurrentTimeTool{},
	}
}
