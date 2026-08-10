package security

import (
	"fmt"
	"strings"

	"Qavor/pkg/config"
)

// commandToken 命令切分后的 token，quoted 标记是否来自引号包裹。
type commandToken struct {
	value  string
	quoted bool
}

// Command 高危命令黑名单，token 级解析避免误报。
type Command struct {
	enabled bool
	bans    []string // 追加的高危命令词（token 级精确匹配）
}

func newCommand(cfg config.CommandConfig, base bool) *Command {
	return &Command{
		enabled: base && (cfg.Enabled == nil || *cfg.Enabled),
		bans:    cfg.ExtraBans,
	}
}

// Check 检查命令是否命中高危规则，命中返回统一拒绝错误。
// 只拦截毁灭性命令：rm -rf /、dd 擦盘、独立出现的 shutdown/reboot/halt/poweroff。
func (c *Command) Check(command string) error {
	if c == nil || !c.enabled || command == "" {
		return nil
	}
	if c.blocks(splitCommandTokens(command)) {
		return fmt.Errorf("%w: 该命令不在允许的执行范围内", ErrDenied)
	}
	return nil
}

func (c *Command) blocks(tokens []commandToken) bool {
	values := make([]string, len(tokens))
	for i, t := range tokens {
		values[i] = t.value
	}
	for i, t := range tokens {
		// 系统词只在非引号（裸命令）时拦截，避免误伤 echo "shutdown" 等输出文本
		if !t.quoted {
			switch t.value {
			case "shutdown", "reboot", "halt", "poweroff":
				if i == 0 || values[i-1] != "man" {
					return true
				}
			}
		}
		// rm -rf / 或 rm -rf /*
		if t.value == "rm" && hasAnyFlag(values[i+1:]) {
			for _, arg := range values[i+1:] {
				if arg == "/" || arg == "/*" {
					return true
				}
			}
		}
		// dd 擦盘：dd if=/dev/zero
		if t.value == "dd" {
			for _, arg := range values[i+1:] {
				if strings.HasPrefix(arg, "if=/dev/zero") {
					return true
				}
			}
		}
	}
	// 追加的黑名单词（token 级精确匹配）
	if len(c.bans) > 0 {
		for _, tok := range values {
			for _, ban := range c.bans {
				if strings.EqualFold(tok, ban) {
					return true
				}
			}
		}
	}
	return false
}

func hasAnyFlag(tokens []string) bool {
	for _, t := range tokens {
		if strings.HasPrefix(t, "-") {
			return true
		}
	}
	return false
}

// splitCommandTokens 引号感知的命令切分，支持单双引号与反斜杠转义。
// 与 shlex 语义接近：引号内的空白不切分，引号本身不保留，并标记是否来自引号。
func splitCommandTokens(s string) []commandToken {
	var tokens []commandToken
	var cur strings.Builder
	quoted := false
	var quote rune
	escaped := false
	flush := func() {
		if cur.Len() > 0 {
			tokens = append(tokens, commandToken{value: cur.String(), quoted: quoted})
			cur.Reset()
			quoted = false
		}
	}
	for _, r := range s {
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
			quoted = true
		case ' ', '\t', '\n', '\r':
			flush()
		default:
			cur.WriteRune(r)
		}
	}
	flush()
	return tokens
}
