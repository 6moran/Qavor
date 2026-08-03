package rag

import (
	"errors"
	"strings"

	"Qavor/pkg/utils"
)

// Chunker 固定规则分块器。
// 第一版策略：优先按空行切分；对超长段再按 Token 窗口切片并允许重叠。
type Chunker struct {
	maxTokens     int
	overlapTokens int
}

// NewChunker 创建 Chunker。
func NewChunker(maxTokens, overlapTokens int) *Chunker {
	if maxTokens <= 0 {
		maxTokens = 800
	}
	if overlapTokens < 0 {
		overlapTokens = 0
	}
	if overlapTokens >= maxTokens {
		overlapTokens = maxTokens / 4
	}
	return &Chunker{maxTokens: maxTokens, overlapTokens: overlapTokens}
}

// Split 将 Markdown 切分为稳定顺序的文本块。
func (c *Chunker) Split(markdown string) ([]string, error) {
	text := strings.TrimSpace(markdown)
	if text == "" {
		return nil, errors.New("empty markdown content")
	}

	// 1. 优先按空行切分。
	paragraphs := splitByBlankLine(text)

	// 2. 对每个段落按 Token 窗口切片，必要时重叠。
	var chunks []string
	for _, p := range paragraphs {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		sliced := c.sliceByTokens(p)
		chunks = append(chunks, sliced...)
	}

	if len(chunks) == 0 {
		return nil, errors.New("no valid chunks produced")
	}
	return chunks, nil
}

// splitByBlankLine 按连续空行切分。
func splitByBlankLine(text string) []string {
	raw := strings.Split(text, "\n\n")
	parts := make([]string, 0, len(raw))
	for _, p := range raw {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		parts = append(parts, p)
	}
	return parts
}

// sliceByTokens 按 Token 窗口切片，并在切片之间保留 overlapTokens 重叠。
func (c *Chunker) sliceByTokens(paragraph string) []string {
	total := utils.CountTokens(paragraph)
	if total <= c.maxTokens {
		return []string{paragraph}
	}

	// 粗略按字符切分，再用 Token 度量裁剪；保证每块不超过 maxTokens。
	runes := []rune(paragraph)
	var result []string
	start := 0
	for start < len(runes) {
		end := minRune(start+guessCharWindow(total, len(runes), c.maxTokens), len(runes))
		piece := string(runes[start:end])
		// 裁剪回退：按 Token 精确控制。
		piece = fitToTokenWindow(piece, c.maxTokens)
		if strings.TrimSpace(piece) != "" {
			result = append(result, piece)
		}
		if end >= len(runes) {
			break
		}
		// 推进：跳过重叠区域，避免重复过多。
		advance := len([]rune(piece)) - estimateOverlapRunes(c.overlapTokens, piece)
		if advance <= 0 {
			advance = 1
		}
		start += advance
	}
	return result
}

// guessCharWindow 基于 Token/字符比估算字符窗口长度。
func guessCharWindow(totalTokens, totalRunes, maxTokens int) int {
	if totalTokens <= 0 || totalRunes <= 0 {
		return maxTokens * 3
	}
	ratio := float64(totalRunes) / float64(totalTokens)
	window := int(float64(maxTokens) * ratio)
	if window <= 0 {
		window = maxTokens * 3
	}
	return window
}

// fitToTokenWindow 逐字符裁剪直到 Token 数不超过 maxTokens。
func fitToTokenWindow(s string, maxTokens int) string {
	if utils.CountTokens(s) <= maxTokens {
		return s
	}
	runes := []rune(s)
	// 二分查找可容纳的最大前缀。
	lo, hi := 0, len(runes)
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if utils.CountTokens(string(runes[:mid])) <= maxTokens {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return string(runes[:lo])
}

// estimateOverlapRunes 根据重叠 Token 数估算 rune 数量。
func estimateOverlapRunes(overlapTokens int, sample string) int {
	if overlapTokens <= 0 || sample == "" {
		return 0
	}
	tokens := utils.CountTokens(sample)
	if tokens <= 0 {
		return 0
	}
	runes := len([]rune(sample))
	ratio := float64(runes) / float64(tokens)
	n := int(float64(overlapTokens) * ratio)
	if n >= runes {
		return runes / 4
	}
	return n
}

func minRune(a, b int) int {
	if a < b {
		return a
	}
	return b
}
