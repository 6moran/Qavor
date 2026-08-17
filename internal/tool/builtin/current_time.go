package builtin

import (
	"context"
	"time"

	"Qavor/internal/tool"
)

// CurrentTimeTool 当前时间工具
type CurrentTimeTool struct{}

// Meta 返回工具元数据
func (t *CurrentTimeTool) Meta() tool.ToolMeta {
	return tool.ToolMeta{
		Name:        "current_time",
		Label:       "当前时间",
		Description: "获取当前时间",
		Category:    tool.CategorySystem,
		Args:        []tool.ArgDef{},
		ConfigGuide: "获取当前系统时间",
	}
}

// Execute 执行当前时间工具
func (t *CurrentTimeTool) Execute(ctx context.Context, args map[string]any) (any, error) {
	now := time.Now()
	return map[string]any{
		"timestamp": now.Unix(),
		"datetime":  now.Format(time.RFC3339),
		"timezone":  now.Location().String(),
	}, nil
}
