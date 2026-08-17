package security

import (
	"strings"

	"Qavor/pkg/config"
)

// ExitCode 退出码语义解释：解决"非零退出码 ≠ 失败"。
type ExitCode struct {
	enabled bool
}

func newExitCode(cfg config.ExitCodeConfig, base bool) *ExitCode {
	return &ExitCode{enabled: base && (cfg.Enabled == nil || *cfg.Enabled)}
}

// softExitCode1 退出码 1 视为信息性的命令。
var softExitCode1 = map[string]bool{
	"grep": true, // 无匹配
	"find": true, // 部分目录不可访问或无匹配
	"diff": true, // 内容不同
	"cmp":  true, // 内容不同
	"test": true, // 条件为假
	"[":    true, // 条件为假
}

// Interpret 判断退出码是否为失败，返回是否失败和说明。
func (e *ExitCode) Interpret(command string, exitCode int) (isFailure bool, note string) {
	if e == nil || !e.enabled {
		return exitCode != 0, ""
	}
	if exitCode == 0 {
		return false, "命令成功执行"
	}
	base := baseCommand(lastSegment(command))
	if exitCode == 1 && softExitCode1[base] {
		return false, "命令退出码为 1（信息性，非失败）"
	}
	return true, ""
}

// lastSegment 返回命令链中最后一个非空命令分段（按 ; && || | 换行切分，引号感知）。
func lastSegment(command string) string {
	parts := splitBySeparators(command)
	for i := len(parts) - 1; i >= 0; i-- {
		if s := strings.TrimSpace(parts[i]); s != "" {
			return s
		}
	}
	return command
}

// baseCommand 取段内第一个命令词，忽略 VAR=x 前缀与引号。
func baseCommand(segment string) string {
	tokens := splitCommandTokens(segment)
	for _, tok := range tokens {
		if strings.Contains(tok.value, "=") && !strings.ContainsAny(tok.value, "[]()*?") {
			continue // 形如 VAR=x 的环境变量赋值
		}
		return strings.Trim(tok.value, `"'`)
	}
	return ""
}

// splitBySeparators 按命令链分隔符切分，引号内的分隔符不生效。
func splitBySeparators(command string) []string {
	var segments []string
	var cur strings.Builder
	var quote rune
	escaped := false
	for _, r := range command {
		if escaped {
			cur.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' && quote != '\'' {
			escaped = true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
			continue
		}
		switch r {
		case '\'', '"':
			quote = r
			cur.WriteRune(r)
		case ';', '|', '&', '\n':
			segments = append(segments, cur.String())
			cur.Reset()
		default:
			cur.WriteRune(r)
		}
	}
	segments = append(segments, cur.String())
	return segments
}
