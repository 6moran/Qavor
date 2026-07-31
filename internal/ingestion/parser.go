package ingestion

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"unicode"
)

// Parser handles Go-native text formats and delegates binary Office formats.
type Parser struct {
	python DocumentParser
}

func NewParser(python DocumentParser) *Parser {
	return &Parser{python: python}
}

func (p *Parser) Parse(ctx context.Context, input ParseInput) (ParseResult, error) {
	switch strings.ToLower(filepath.Ext(input.Filename)) {
	case ".txt", ".md":
		return ParseResult{Markdown: normalizeText(string(input.Content))}, nil
	case ".docx", ".pdf", ".pptx":
		if p.python == nil {
			return ParseResult{}, fmt.Errorf("document parser is not configured")
		}
		return p.python.Parse(ctx, input)
	default:
		return ParseResult{}, fmt.Errorf("unsupported document type: %s", filepath.Ext(input.Filename))
	}
}

func normalizeText(input string) string {
	input = strings.ReplaceAll(input, "\r\n", "\n")
	input = strings.ReplaceAll(input, "\r", "\n")
	lines := strings.Split(input, "\n")
	output := make([]string, 0, len(lines))
	empty := false
	for _, line := range lines {
		line = strings.Map(func(r rune) rune {
			if unicode.IsControl(r) && r != '\t' {
				return -1
			}
			return r
		}, line)
		line = strings.TrimSpace(line)
		if line == "" {
			if !empty && len(output) > 0 {
				output = append(output, "")
			}
			empty = true
			continue
		}
		output = append(output, line)
		empty = false
	}
	return strings.TrimSpace(strings.Join(output, "\n"))
}
