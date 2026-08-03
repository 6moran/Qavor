package skill

import (
	"context"
	"fmt"

	einotool "github.com/cloudwego/eino/components/tool"

	"Qavor/internal/tool"
)

// ToolRouter 动态工具路由，根据激活状态过滤工具
type ToolRouter struct {
	allTools   map[string]einotool.BaseTool
	toolOwner  map[string]string // toolName → ownerSlug
	activation *ActivationState
}

// NewToolRouter 创建 ToolRouter
func NewToolRouter(activation *ActivationState) *ToolRouter {
	return &ToolRouter{
		allTools:   make(map[string]einotool.BaseTool),
		toolOwner:  make(map[string]string),
		activation: activation,
	}
}

// Register 注册工具，指定所有者（空字符串表示非 Skill 工具）
func (r *ToolRouter) Register(tool einotool.BaseTool, ownerSlug string) {
	info, _ := tool.Info(context.Background())
	r.allTools[info.Name] = tool
	if ownerSlug != "" {
		r.toolOwner[info.Name] = ownerSlug
	}
}

// RegisterBuiltin 注册内置工具（非 Skill 工具）
func (r *ToolRouter) RegisterBuiltin(t tool.BuiltinTool) {
	einoTool := tool.NewBuiltinToolAdapter(t)
	r.Register(einoTool, "")
}

// GetVisibleTools 获取当前可见的工具列表
func (r *ToolRouter) GetVisibleTools() []einotool.BaseTool {
	var result []einotool.BaseTool
	for name, t := range r.allTools {
		ownerSlug, isSkillTool := r.toolOwner[name]
		if isSkillTool {
			if r.activation.IsActivated(ownerSlug) {
				result = append(result, t)
			}
		} else {
			result = append(result, t)
		}
	}
	return result
}

// Run 执行工具（带门控检查）
func (r *ToolRouter) Run(ctx context.Context, toolName string, args string) (string, error) {
	tool, ok := r.allTools[toolName]
	if !ok {
		return "", fmt.Errorf("tool not found: %s", toolName)
	}

	if ownerSlug, isSkillTool := r.toolOwner[toolName]; isSkillTool {
		if !r.activation.IsActivated(ownerSlug) {
			return "", fmt.Errorf(
				"tool '%s' requires activation. Please read its SKILL.md first by calling read_file(path='skills/%s/SKILL.md')",
				toolName, ownerSlug,
			)
		}
	}

	invokable, ok := tool.(einotool.InvokableTool)
	if !ok {
		return "", fmt.Errorf("tool '%s' does not implement InvokableTool", toolName)
	}

	return invokable.InvokableRun(ctx, args)
}

// HasTool 检查工具是否存在
func (r *ToolRouter) HasTool(toolName string) bool {
	_, ok := r.allTools[toolName]
	return ok
}

// GetToolOwner 获取工具的所有者 slug（非 Skill 工具返回空字符串）
func (r *ToolRouter) GetToolOwner(toolName string) string {
	return r.toolOwner[toolName]
}
