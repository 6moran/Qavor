package security

import (
	"encoding/json"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"

	"Qavor/pkg/config"

	"gopkg.in/yaml.v3"
)

// Syntax 写前语法预检。
// 结构数据（JSON/YAML）是原子整体，解析失败阻断写入；
// 源码语言仅当本次编辑引入了新语法错误才警告，不阻断。
type Syntax struct {
	enabled bool
}

func newSyntax(cfg config.SyntaxConfig, base bool) *Syntax {
	return &Syntax{enabled: base && (cfg.Enabled == nil || *cfg.Enabled)}
}

// SyntaxResult 语法预检结果。
type SyntaxResult struct {
	Block   bool   // 是否阻断写入
	Message string // 说明（空表示通过）
}

// Validate 对写入内容做语法预检。
// oldContent 为空时表示新建文件（新建文件解析失败直接阻断）。
// 校验器自身故障一律返回通过（不因校验器 panic 阻断写文件）。
func (s *Syntax) Validate(filePath, oldContent, newContent string) (result SyntaxResult) {
	if s == nil || !s.enabled {
		return SyntaxResult{}
	}
	defer func() {
		if recover() != nil {
			result = SyntaxResult{} // 校验器故障视为不检查
		}
	}()

	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".json":
		newValid := json.Valid([]byte(newContent))
		if newValid {
			return SyntaxResult{}
		}
		if oldContent == "" || !json.Valid([]byte(oldContent)) {
			return SyntaxResult{Message: "JSON 语法错误（原本已损坏或新建，仅提示）"}
		}
		return SyntaxResult{Block: true, Message: "JSON 语法错误，本次编辑将写入非法 JSON"}
	case ".yaml", ".yml":
		newValid := parseYAML(newContent)
		if newValid == nil {
			return SyntaxResult{}
		}
		if oldContent == "" || parseYAML(oldContent) != nil {
			return SyntaxResult{Message: "YAML 语法错误（原本已损坏或新建，仅提示）"}
		}
		return SyntaxResult{Block: true, Message: "YAML 语法错误，本次编辑将写入非法 YAML"}
	case ".go":
		newValid := parseGo(newContent)
		if newValid == nil {
			return SyntaxResult{}
		}
		if oldContent == "" || parseGo(oldContent) != nil {
			return SyntaxResult{Message: "Go 语法错误（原本已损坏或新建，仅提示）"}
		}
		return SyntaxResult{Message: "Go 语法错误，本次编辑引入了新的语法错误"}
	}
	return SyntaxResult{}
}

func parseYAML(content string) error {
	var v any
	return yaml.Unmarshal([]byte(content), &v)
}

func parseGo(content string) error {
	fset := token.NewFileSet()
	_, err := parser.ParseFile(fset, "probe.go", content, 0)
	return err
}
