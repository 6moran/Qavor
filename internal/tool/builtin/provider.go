package builtin

import (
	"Qavor/internal/service"
	"Qavor/internal/tool"
)

// BuiltinToolProvider 内置工具提供者
type BuiltinToolProvider struct {
	ragService service.RAGService
}

// NewBuiltinToolProvider 创建内置工具提供者。ragService 非 nil 时注册知识库检索工具。
func NewBuiltinToolProvider(ragService service.RAGService) *BuiltinToolProvider {
	return &BuiltinToolProvider{ragService: ragService}
}

// GetTools 获取所有内置工具
func (p *BuiltinToolProvider) GetTools() []tool.BuiltinTool {
	tools := []tool.BuiltinTool{
		&CalculatorTool{},
		&CurrentTimeTool{},
	}
	if p.ragService != nil {
		tools = append(tools, NewQueryKBTool(p.ragService))
	}
	return tools
}
