package rag

import (
	"errors"
	"regexp"
	"strings"

	"Qavor/pkg/utils"
)

// faqQuestionPattern 匹配问题行：Q1:、Q：、问：、**Q: 等（行首，允许 markdown 加粗符号）。
var faqQuestionPattern = regexp.MustCompile(`(?i)^\*{0,2}(?:q\d*|问)\s*[:：.、]`)

// FAQChunker 按问答对切分 FAQ 文档。
// 每个「问题行 + 其后的回答内容」作为一个独立块；文档中识别不到问答对时回退通用分块。
type FAQChunker struct {
	maxTokens     int
	overlapTokens int
	fallback      *Chunker
}

// NewFAQChunker 创建 FAQ 分块器。
func NewFAQChunker(maxTokens, overlapTokens int) *FAQChunker {
	return &FAQChunker{
		maxTokens:     maxTokens,
		overlapTokens: overlapTokens,
		fallback:      NewChunker(maxTokens, overlapTokens),
	}
}

// Split 将 FAQ Markdown 切分为问答对块。
func (c *FAQChunker) Split(markdown string) ([]string, error) {
	text := strings.TrimSpace(markdown)
	if text == "" {
		return nil, errors.New("empty markdown content")
	}

	// 逐行扫描：问题行开启新问答对，其余行归属当前问答对。
	var (
		pairs   []string
		current []string
		started bool
	)
	for _, line := range strings.Split(text, "\n") {
		if faqQuestionPattern.MatchString(strings.TrimSpace(line)) {
			if started {
				pairs = append(pairs, strings.Join(current, "\n"))
			}
			current = []string{line}
			started = true
			continue
		}
		if started {
			current = append(current, line)
		}
	}
	if started {
		pairs = append(pairs, strings.Join(current, "\n"))
	}

	// 未识别到任何问答对：该文档不是 FAQ 格式，回退通用分块。
	if len(pairs) == 0 {
		return c.fallback.Split(markdown)
	}

	var chunks []string
	for _, pair := range pairs {
		trimmed := strings.TrimSpace(pair)
		if trimmed == "" {
			continue
		}
		if utils.CountTokens(trimmed) <= c.maxTokens {
			chunks = append(chunks, trimmed)
			continue
		}
		// 超长问答对按行切分，保持每行完整。
		chunks = append(chunks, splitByLines(pair, c.maxTokens)...)
	}
	if len(chunks) == 0 {
		return nil, errors.New("no valid chunks produced")
	}
	return chunks, nil
}
