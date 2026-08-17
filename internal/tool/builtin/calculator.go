package builtin

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"

	"Qavor/internal/tool"
)

// CalculatorTool 计算器工具
type CalculatorTool struct{}

// Meta 返回工具元数据
func (t *CalculatorTool) Meta() tool.ToolMeta {
	return tool.ToolMeta{
		Name:        "calculator",
		Label:       "计算器",
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
func (t *CalculatorTool) Execute(_ context.Context, args map[string]any) (any, error) {
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

// evaluateExpression 解析并计算数学表达式。
// 支持 +、-、*、/、括号、一元负号、sqrt()、pow()、abs()。
func evaluateExpression(expr string) (float64, error) {
	p := &exprParser{s: []rune(expr)}
	val, err := p.parseExpr()
	if err != nil {
		return 0, err
	}
	p.skipSpaces()
	if p.pos < len(p.s) {
		return 0, fmt.Errorf("unexpected character: %q", string(p.s[p.pos]))
	}
	return val, nil
}

// exprParser 递归下降表达式解析器。
type exprParser struct {
	s   []rune
	pos int
}

func (p *exprParser) parseExpr() (float64, error) {
	left, err := p.parseTerm()
	if err != nil {
		return 0, err
	}
	for {
		p.skipSpaces()
		op := p.peek()
		if op != '+' && op != '-' {
			break
		}
		p.pos++
		right, err := p.parseTerm()
		if err != nil {
			return 0, err
		}
		if op == '+' {
			left += right
		} else {
			left -= right
		}
	}
	return left, nil
}

func (p *exprParser) parseTerm() (float64, error) {
	left, err := p.parseFactor()
	if err != nil {
		return 0, err
	}
	for {
		p.skipSpaces()
		op := p.peek()
		if op != '*' && op != '/' {
			break
		}
		p.pos++
		right, err := p.parseFactor()
		if err != nil {
			return 0, err
		}
		if op == '*' {
			left *= right
		} else {
			if right == 0 {
				return 0, errors.New("division by zero")
			}
			left /= right
		}
	}
	return left, nil
}

func (p *exprParser) parseFactor() (float64, error) {
	p.skipSpaces()
	ch := p.peek()

	// 一元负号
	if ch == '-' {
		p.pos++
		val, err := p.parseFactor()
		if err != nil {
			return 0, err
		}
		return -val, nil
	}

	// 括号表达式
	if ch == '(' {
		p.pos++
		val, err := p.parseExpr()
		if err != nil {
			return 0, err
		}
		p.skipSpaces()
		if p.peek() != ')' {
			return 0, errors.New("missing closing parenthesis")
		}
		p.pos++
		return val, nil
	}

	// 函数调用 (sqrt, pow, abs)
	if isAlpha(ch) {
		return p.parseFunction()
	}

	// 数字
	return p.parseNumber()
}

func (p *exprParser) parseFunction() (float64, error) {
	start := p.pos
	for p.pos < len(p.s) && isAlpha(p.s[p.pos]) {
		p.pos++
	}
	name := string(p.s[start:p.pos])
	p.skipSpaces()
	if p.peek() != '(' {
		return 0, fmt.Errorf("expected '(' after function %s", name)
	}
	p.pos++

	var args []float64
	p.skipSpaces()
	if p.peek() != ')' {
		for {
			arg, err := p.parseExpr()
			if err != nil {
				return 0, err
			}
			args = append(args, arg)
			p.skipSpaces()
			if p.peek() == ',' {
				p.pos++
				continue
			}
			break
		}
	}
	p.skipSpaces()
	if p.peek() != ')' {
		return 0, fmt.Errorf("missing ')' in function %s", name)
	}
	p.pos++

	switch name {
	case "sqrt":
		if len(args) != 1 {
			return 0, errors.New("sqrt requires 1 argument")
		}
		return math.Sqrt(args[0]), nil
	case "pow":
		if len(args) != 2 {
			return 0, errors.New("pow requires 2 arguments")
		}
		return math.Pow(args[0], args[1]), nil
	case "abs":
		if len(args) != 1 {
			return 0, errors.New("abs requires 1 argument")
		}
		return math.Abs(args[0]), nil
	default:
		return 0, fmt.Errorf("unknown function: %s", name)
	}
}

func (p *exprParser) parseNumber() (float64, error) {
	start := p.pos
	for p.pos < len(p.s) && (isDigit(p.s[p.pos]) || p.s[p.pos] == '.') {
		p.pos++
	}
	if start == p.pos {
		return 0, fmt.Errorf("unexpected character: %q", string(p.peek()))
	}
	return strconv.ParseFloat(string(p.s[start:p.pos]), 64)
}

func (p *exprParser) peek() rune {
	if p.pos >= len(p.s) {
		return 0
	}
	return p.s[p.pos]
}

func (p *exprParser) skipSpaces() {
	for p.pos < len(p.s) && (p.s[p.pos] == ' ' || p.s[p.pos] == '\t' || p.s[p.pos] == '\n' || p.s[p.pos] == '\r') {
		p.pos++
	}
}

func isDigit(c rune) bool {
	return c >= '0' && c <= '9'
}

func isAlpha(c rune) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}
