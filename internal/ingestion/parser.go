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

// NewParser 创建解析器。images 可选，用于 .md 源文件内嵌 data URI 图片的上传回填，仅首个生效。
func NewParser(python DocumentParser, images ...ImageUploader) *Parser {
	var img ImageUploader
	if len(images) > 0 {
		img = images[0]
	}
	return &Parser{python: python, images: img}
}

func (p *Parser) Parse(ctx context.Context, input ParseInput) (ParseResult, error) {
	ext := strings.ToLower(filepath.Ext(input.Filename))
	var result ParseResult
	switch ext {
	case ".txt", ".md":
		markdown := string(input.Content)
		if ext == ".md" {
			markdown = ReplaceDataURILinks(markdown, DeriveImageFolder(input.Path), p.images)
		}
		result = ParseResult{Markdown: markdown}
	case ".docx", ".pptx", ".xlsx", ".pdf", ".jpg", ".jpeg", ".png", ".bmp", ".tiff", ".tif":
		if p.python == nil {
			return ParseResult{}, fmt.Errorf("未配置文档解析器")
		}
		// 图片回填与临时目录清理由 Python 解析器内部完成。
		var err error
		result, err = p.python.Parse(ctx, input)
		if err != nil {
			return ParseResult{}, err
		}
	default:
		return ParseResult{}, fmt.Errorf("不支持的文件格式: %s", filepath.Ext(input.Filename))
	}
	return finalizeParseResult(result)
}

func finalizeParseResult(result ParseResult) (ParseResult, error) {
	if !hasVisibleContent(result.Markdown) {
		return ParseResult{}, &ParserError{Code: "PARSER_EMPTY_CONTENT", Message: "文档解析结果为空"}
	}
	cleaned := (DocumentCleaner{}).Clean(result.Markdown)
	result.Markdown = cleaned.Markdown
	if result.Metadata == nil {
		result.Metadata = make(map[string]any)
	}
	if cleaned.Degraded {
		result.Metadata["cleaner_degraded"] = true
	}
	return result, nil
}

func hasVisibleContent(markdown string) bool {
	withoutArtifacts, err := removeEmptyArtifacts(markdown)
	if err != nil {
		withoutArtifacts = markdown
	}
	for _, r := range withoutArtifacts {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return true
		}
	}
	return false
}
