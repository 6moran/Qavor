package tool

import (
	"context"
	"encoding/json"

	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// builtinToolAdapter 将 BuiltinTool 包装为 eino tool.BaseTool
type builtinToolAdapter struct {
	builtinTool BuiltinTool
}

// NewBuiltinToolAdapter 创建内置工具适配器
func NewBuiltinToolAdapter(t BuiltinTool) einotool.BaseTool {
	return &builtinToolAdapter{builtinTool: t}
}

// Info 获取工具元数据
func (a *builtinToolAdapter) Info(_ context.Context) (*schema.ToolInfo, error) {
	meta := a.builtinTool.Meta()

	// 构建参数 schema
	params := make(map[string]*schema.ParameterInfo)
	for _, arg := range meta.Args {
		params[arg.Name] = &schema.ParameterInfo{
			Type:     schema.DataType(arg.Type),
			Desc:     arg.Description,
			Required: arg.Required,
		}
	}

	var paramsOneOf *schema.ParamsOneOf
	if len(params) > 0 {
		paramsOneOf = schema.NewParamsOneOfByParams(params)
	}

	return &schema.ToolInfo{
		Name:        meta.Name,
		Desc:        meta.Description,
		ParamsOneOf: paramsOneOf,
	}, nil
}

// InvokableRun 执行工具，实现 eino InvokableTool 接口
func (a *builtinToolAdapter) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...einotool.Option) (string, error) {
	var argsMap map[string]any
	if err := json.Unmarshal([]byte(argumentsInJSON), &argsMap); err != nil {
		return "", err
	}

	result, err := a.builtinTool.Execute(ctx, argsMap)
	if err != nil {
		return "", err
	}

	resultBytes, err := json.Marshal(result)
	if err != nil {
		return "", err
	}

	return string(resultBytes), nil
}

// ToEinoTools 将所有内置工具转换为 eino tool.BaseTool
func (r *Registry) ToEinoTools() []einotool.BaseTool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]einotool.BaseTool, 0, len(r.tools))
	for _, t := range r.tools {
		result = append(result, NewBuiltinToolAdapter(t))
	}
	return result
}

// ToEinoToolsByNames 根据工具名列表转换为 eino tool.BaseTool
func (r *Registry) ToEinoToolsByNames(names []string) []einotool.BaseTool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]einotool.BaseTool, 0, len(names))
	for _, name := range names {
		if t, ok := r.tools[name]; ok {
			result = append(result, NewBuiltinToolAdapter(t))
		}
	}
	return result
}
