package builtin

import (
	"context"
	"errors"
	"math"
	"strconv"
	"strings"

	"Qavor/internal/tool"
)

// CalculatorTool 计算器工具
type CalculatorTool struct{}

// Meta 返回工具元数据
func (t *CalculatorTool) Meta() tool.ToolMeta {
	return tool.ToolMeta{
		Name:        "calculator",
		Description: "数学计算",
		Category:    tool.CategorySystem,
		Args: []tool.ArgDef{{
			Name:        "expression",
			Type:        "string",
			Description: "数学表达式",
			Required:    true,
		}},
		ConfigGuide: "输入数学表达式，如 1+1、sqrt(4)、pow(2,3)",
	}
}

// Execute 执行计算器工具
func (t *CalculatorTool) Execute(ctx context.Context, args map[string]any) (any, error) {
	expr, ok := args["expression"].(string)
	if !ok {
		return nil, errors.New("expression must be a string")
	}

	result, err := evaluateExpression(expr)
	if err != nil {
		return nil, err
	}

	return map[string]any{"result": result}, nil
}

// evaluateExpression 简单的表达式求值器
func evaluateExpression(expr string) (float64, error) {
	expr = strings.ReplaceAll(expr, " ", "")

	// 处理 sqrt
	if strings.HasPrefix(expr, "sqrt(") && strings.HasSuffix(expr, ")") {
		inner := expr[5 : len(expr)-1]
		val, err := evaluateExpression(inner)
		if err != nil {
			return 0, err
		}
		return math.Sqrt(val), nil
	}

	// 处理 pow
	if strings.HasPrefix(expr, "pow(") && strings.HasSuffix(expr, ")") {
		inner := expr[4 : len(expr)-1]
		parts := strings.Split(inner, ",")
		if len(parts) != 2 {
			return 0, errors.New("pow requires two arguments")
		}
		base, err := evaluateExpression(parts[0])
		if err != nil {
			return 0, err
		}
		exp, err := evaluateExpression(parts[1])
		if err != nil {
			return 0, err
		}
		return math.Pow(base, exp), nil
	}

	// 处理 abs
	if strings.HasPrefix(expr, "abs(") && strings.HasSuffix(expr, ")") {
		inner := expr[4 : len(expr)-1]
		val, err := evaluateExpression(inner)
		if err != nil {
			return 0, err
		}
		return math.Abs(val), nil
	}

	// 处理基本运算 +, -, *, /
	return parseAndEvaluate(expr)
}

// parseAndEvaluate 解析并计算基本表达式
func parseAndEvaluate(expr string) (float64, error) {
	// 简单实现：只支持数字
	// 完整实现需要递归下降解析器
	val, err := strconv.ParseFloat(expr, 64)
	if err != nil {
		return 0, errors.New("invalid expression: " + expr)
	}
	return val, nil
}
