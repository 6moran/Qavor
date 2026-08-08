package rag

import (
	"errors"
	"strings"

	"Qavor/pkg/utils"
)

// Chunker 通用文档结构感知分块器（general 预设）。
// 分块策略：代码块围栏与表格行整体保留不切碎；以 Markdown 标题为语义边界；
// 超长文本段按 Token 窗口切片并允许重叠；超长代码/表格按行切片保持行完整。
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

	units := mergeHeadingBlocks(parseMDBlocks(text))
	var chunks []string
	for _, u := range units {
		content := strings.TrimSpace(strings.Join(u.lines, "\n"))
		if content == "" {
			continue
		}
		if utils.CountTokens(content) <= c.maxTokens {
			chunks = append(chunks, content)
			continue
		}
		// 代码块/表格按行切片保持行完整；标题与文本段按 Token 窗口切片。
		switch u.kind {
		case blockCode, blockTable:
			chunks = append(chunks, splitByLines(content, c.maxTokens)...)
		default:
			chunks = append(chunks, c.sliceByTokens(content)...)
		}
	}
	if len(chunks) == 0 {
		return nil, errors.New("no valid chunks produced")
	}
	return chunks, nil
}

// mdBlockKind 行块类型。
type mdBlockKind int

const (
	blockText mdBlockKind = iota
	blockHeading
	blockCode
	blockTable
)

// mdBlock 一次扫描中的连续同类行块。
type mdBlock struct {
	kind  mdBlockKind
	lines []string
}

// parseMDBlocks 单遍扫描 Markdown 行，识别代码块、表格、标题与普通文本。
func parseMDBlocks(markdown string) []mdBlock {
	var (
		blocks []mdBlock
		inCode bool
		fence  string
	)
	appendLine := func(kind mdBlockKind, line string) {
		if len(blocks) == 0 || blocks[len(blocks)-1].kind != kind {
			blocks = append(blocks, mdBlock{kind: kind})
		}
		blocks[len(blocks)-1].lines = append(blocks[len(blocks)-1].lines, line)
	}

	for _, raw := range strings.Split(markdown, "\n") {
		line := strings.TrimRight(raw, " \t")
		trimmed := strings.TrimSpace(line)
		if inCode {
			if isCodeFence(trimmed, fence) {
				inCode = false
			}
			appendLine(blockCode, line)
			continue
		}
		if isCodeFence(trimmed, "") {
			inCode = true
			fence = fenceMarker(trimmed)
			appendLine(blockCode, line)
			continue
		}
		if trimmed == "" {
			// 空行作为文本段的段落分隔保留，不单独成块。
			if len(blocks) > 0 && blocks[len(blocks)-1].kind == blockText {
				blocks[len(blocks)-1].lines = append(blocks[len(blocks)-1].lines, "")
			}
			continue
		}
		switch {
		case isHeadingLine(trimmed):
			appendLine(blockHeading, line)
		case isTableLine(trimmed):
			appendLine(blockTable, line)
		default:
			appendLine(blockText, line)
		}
	}
	return blocks
}

// mergeHeadingBlocks 将标题行块与其后紧邻的文本块合并为同一语义单元。
func mergeHeadingBlocks(blocks []mdBlock) []mdBlock {
	units := make([]mdBlock, 0, len(blocks))
	for _, b := range blocks {
		if b.kind == blockText && len(units) > 0 && units[len(units)-1].kind == blockHeading {
			units[len(units)-1].lines = append(units[len(units)-1].lines, b.lines...)
			continue
		}
		units = append(units, b)
	}
	return units
}

// isCodeFence 判断行是否为代码围栏：fence 为空时识别开围栏，否则匹配闭合围栏。
func isCodeFence(trimmed, fence string) bool {
	if fence == "" {
		return strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~")
	}
	if strings.HasPrefix(trimmed, fence) {
		rest := strings.TrimSpace(strings.TrimPrefix(trimmed, fence))
		return rest == ""
	}
	return false
}

// fenceMarker 提取开围栏标记（连续的 ``` 或 ~~~）。
func fenceMarker(trimmed string) string {
	if strings.HasPrefix(trimmed, "```") {
		return "```"
	}
	return "~~~"
}

// isHeadingLine 判断行是否为 Markdown 标题（# 后必须有空格）。
func isHeadingLine(trimmed string) bool {
	if !strings.HasPrefix(trimmed, "#") {
		return false
	}
	i := 0
	for i < len(trimmed) && trimmed[i] == '#' && i < 6 {
		i++
	}
	return i < len(trimmed) && trimmed[i] == ' '
}

// isTableLine 判断行是否为表格行（行首为 |）。
func isTableLine(trimmed string) bool {
	return strings.HasPrefix(trimmed, "|")
}

// sliceByLines 按行累积分块，每块 Token 数不超过 maxTokens，保持行完整。
func splitByLines(content string, maxTokens int) []string {
	var (
		result []string
		buf    []string
		tokens int
	)
	for _, line := range strings.Split(content, "\n") {
		lineTokens := utils.CountTokens(line)
		if tokens+lineTokens > maxTokens && len(buf) > 0 {
			result = append(result, strings.Join(buf, "\n"))
			buf = nil
			tokens = 0
		}
		buf = append(buf, line)
		tokens += lineTokens
	}
	if len(buf) > 0 {
		result = append(result, strings.Join(buf, "\n"))
	}
	return result
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
