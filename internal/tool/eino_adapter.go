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

	// 构建参数 schema（支持嵌套结构：array 元素的类型、object 的子参数）
	params := make(map[string]*schema.ParameterInfo)
	for _, arg := range meta.Args {
		params[arg.Name] = buildParameterInfo(&arg)
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

// buildParameterInfo 递归构建嵌套参数 schema（数组元素类型、对象子参数）。
func buildParameterInfo(arg *ArgDef) *schema.ParameterInfo {
	pi := &schema.ParameterInfo{
		Type:     schema.DataType(arg.Type),
		Desc:     arg.Description,
		Required: arg.Required,
		Enum:     arg.Enum,
	}
	if arg.Type == "array" && arg.ElemInfo != nil {
		pi.ElemInfo = buildParameterInfo(arg.ElemInfo)
	}
	if arg.Type == "object" && len(arg.SubParams) > 0 {
		pi.SubParams = make(map[string]*schema.ParameterInfo, len(arg.SubParams))
		for name, sub := range arg.SubParams {
			pi.SubParams[name] = buildParameterInfo(sub)
		}
	}
	return pi
}

// InvokableRun 执行工具，实现 eino InvokableTool 接口
// 工具执行出错时（如文件不存在、参数无效），把错误信息作为 tool result 返回给 LLM，
// 而不是返回 error 中断整个对话流——让 LLM 有机会自行处理错误（如告知用户、换一种方式）
func (a *builtinToolAdapter) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...einotool.Option) (string, error) {
	var argsMap map[string]any
	if err := json.Unmarshal([]byte(argumentsInJSON), &argsMap); err != nil {
		// 参数解析失败也返回给 LLM，让它知道参数格式不对
		errResult, _ := json.Marshal(map[string]any{
			"error":   true,
			"message": "参数 JSON 解析失败: " + err.Error(),
		})
		return string(errResult), nil
	}

	result, err := a.builtinTool.Execute(ctx, argsMap)
	if err != nil {
		// 工具执行错误作为结果返回给 LLM，不中断流
		errResult, _ := json.Marshal(map[string]any{
			"error":   true,
			"message": err.Error(),
		})
		return string(errResult), nil
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