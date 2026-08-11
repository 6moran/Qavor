package builtin

import (
	"Qavor/internal/service"
	"Qavor/internal/tool"
)

// BuiltinToolProvider 内置工具提供者
type BuiltinToolProvider struct {
	ragService    service.RAGService
	webSearchTool tool.BuiltinTool
}

// NewBuiltinToolProvider 创建内置工具提供者。
//   - ragService 非 nil 时注册知识库检索工具
//   - webSearchTool 非 nil 时注册联网搜索工具（由 app.go 按 config.App.Mode 与 API Key 决定）
func NewBuiltinToolProvider(ragService service.RAGService, webSearchTool tool.BuiltinTool) *BuiltinToolProvider {
	return &BuiltinToolProvider{
		ragService:    ragService,
		webSearchTool: webSearchTool,
	}
}

// GetTools 获取所有内置工具
func (p *BuiltinToolProvider) GetTools() []tool.BuiltinTool {
	tools := []tool.BuiltinTool{
		&CalculatorTool{},
		&CurrentTimeTool{},
	}
	if p.webSearchTool != nil {
		tools = append(tools, p.webSearchTool)
	}
	if p.ragService != nil {
		tools = append(tools, NewQueryKBTool(p.ragService))
	}
	return tools
}
