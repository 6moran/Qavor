package security

import (
	"regexp"
	"strings"

	"Qavor/pkg/config"
)

// LinePrefix read 行号前缀检测。
// eino read_file 工具自带行号前缀（格式 `     12\tcontent`），CowAgent 用 `12|content`；
// 写入时若把展示性前缀混进 content 会写坏文件，命中即拒绝写入。
type LinePrefix struct {
	enabled bool
}

func newLinePrefix(cfg config.LinePrefixConfig, base bool) *LinePrefix {
	return &LinePrefix{enabled: base && (cfg.Enabled == nil || *cfg.Enabled)}
}

// linePrefixRe 同时兼容 eino 的 `    12\tcontent` 与 CowAgent 的 `12|content`。
var linePrefixRe = regexp.MustCompile(`^\s*\d+\s*[|\t]`)

// LooksLikeLineNumberedBlock 判断内容是否像行号前缀块。
// 非空行中大多数行（≥50%）以行号 + 分隔符开头即判定命中。
func (lp *LinePrefix) LooksLikeLineNumberedBlock(content string) bool {
	if lp == nil || !lp.enabled || content == "" {
		return false
	}
	lines := strings.Split(content, "\n")
	if len(lines) < 2 {
		return false
	}
	nonEmpty, matched := 0, 0
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		nonEmpty++
		if linePrefixRe.MatchString(line) {
			matched++
		}
	}
	return nonEmpty >= 2 && matched*2 >= nonEmpty
}
