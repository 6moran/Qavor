package ingestion

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"unicode"
)

// Parser 处理 Go 原生文本格式，并将二进制 Office/图片/PDF 格式委托给 Python 解析器处理。
type Parser struct {
	python DocumentParser
	images ImageUploader
}

// NewParser 创建解析器。images 可选，用于 .md 源文件内嵌 data URI 图片的上传回填。
func NewParser(python DocumentParser, images ...ImageUploader) *Parser {
	var img ImageUploader
	if len(images) > 0 {
		img = images[0]
	}
	return &Parser{python: python, images: img}
}

func (p *Parser) Parse(ctx context.Context, input ParseInput) (ParseResult, error) {
	ext := strings.ToLower(filepath.Ext(input.Filename))
	switch ext {
	case ".txt", ".md":
		markdown := normalizeText(string(input.Content))
		if ext == ".md" {
			markdown = ReplaceDataURILinks(markdown, DeriveImageFolder(input.Path), p.images)
		}
		return ParseResult{Markdown: markdown}, nil
	case ".docx", ".pptx", ".xlsx", ".pdf", ".jpg", ".jpeg", ".png", ".bmp", ".tiff", ".tif":
		if p.python == nil {
			return ParseResult{}, fmt.Errorf("未配置文档解析器")
		}
		// 图片回填与临时目录清理由 Python 解析器内部完成。
		return p.python.Parse(ctx, input)
	default:
		return ParseResult{}, fmt.Errorf("不支持的文件格式: %s", filepath.Ext(input.Filename))
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
