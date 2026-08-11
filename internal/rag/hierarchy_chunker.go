package rag

import (
	"errors"
	"strings"

	"Qavor/pkg/utils"
)

// HierarchyChunker 标题父子块分块器（hierarchy 预设）。
// 以 Markdown 标题为边界构建标题树，输出两类块：
//   - 父块：章节标题路径 + 子标题列表 + 正文首句摘要，用于章节级粗检索；
//   - 子块：以完整标题路径为前缀的正文窗口切片，用于内容级精检索，
//     检索命中时天然携带章节上下文（路径直接内嵌在内容中）。
//
// 文档中识别不到任何标题时回退通用分块。
type HierarchyChunker struct {
	maxTokens     int
	overlapTokens int
	fallback      *Chunker
}

// NewHierarchyChunker 创建标题父子块分块器。
func NewHierarchyChunker(maxTokens, overlapTokens int) *HierarchyChunker {
	return &HierarchyChunker{
		maxTokens:     maxTokens,
		overlapTokens: overlapTokens,
		fallback:      NewChunker(maxTokens, overlapTokens),
	}
}

// headingNode 标题树节点。根节点 level=0、title=""，仅承载文档前言正文。
type headingNode struct {
	level    int
	title    string
	body     []string
	children []*headingNode
}

// Split 将 Markdown 切分为「父块（章节摘要）+ 子块（带标题路径的正文）」序列。
// 块按文档顺序输出：章节父块在前，其子块随后，再递归处理更深层章节。
func (c *HierarchyChunker) Split(markdown string) ([]string, error) {
	text := strings.TrimSpace(markdown)
	if text == "" {
		return nil, errors.New("empty markdown content")
	}

	root := buildHeadingTree(text)
	if len(root.children) == 0 {
		// 无标题文档：不属于层级结构，回退通用分块。
		return c.fallback.Split(markdown)
	}

	var chunks []string
	var walk func(node *headingNode, path []*headingNode)
	walk = func(node *headingNode, path []*headingNode) {
		newPath := path
		if node.level > 0 {
			newPath = append(newPath, node)
			if len(node.children) > 0 {
				if parent := c.buildParentBlock(newPath); parent != "" {
					chunks = append(chunks, parent)
				}
			}
			chunks = append(chunks, c.splitBodyBlocks(newPath, node.body)...)
		} else if len(node.body) > 0 {
			// 根节点：文档开头的无标题前言。
			chunks = append(chunks, c.splitBodyBlocks(nil, node.body)...)
		}
		for _, child := range node.children {
			walk(child, newPath)
		}
	}
	walk(root, nil)

	if len(chunks) == 0 {
		return nil, errors.New("no valid chunks produced")
	}
	return chunks, nil
}

// buildHeadingTree 单遍扫描 Markdown，构建标题树；代码围栏内的行一律视为正文。
func buildHeadingTree(markdown string) *headingNode {
	root := &headingNode{}
	stack := []*headingNode{root}
	appendBody := func(line string) {
		stack[len(stack)-1].body = append(stack[len(stack)-1].body, line)
	}

	inCode := false
	fence := ""
	for _, raw := range strings.Split(markdown, "\n") {
		trimmed := strings.TrimSpace(raw)
		if inCode {
			if isCodeFence(trimmed, fence) {
				inCode = false
			}
			appendBody(raw)
			continue
		}
		if isCodeFence(trimmed, "") {
			inCode = true
			fence = fenceMarker(trimmed)
			appendBody(raw)
			continue
		}
		if level, title := headingInfo(trimmed); level > 0 {
			// 弹出层级 >= 当前标题的节点，使当前标题成为最近祖先的子节点。
			for len(stack) > 1 && stack[len(stack)-1].level >= level {
				stack = stack[:len(stack)-1]
			}
			node := &headingNode{level: level, title: title}
			stack[len(stack)-1].children = append(stack[len(stack)-1].children, node)
			stack = append(stack, node)
			continue
		}
		appendBody(raw)
	}
	return root
}

// headingInfo 解析 Markdown 标题行，返回层级（1-6）与标题文本；非标题行返回 (0, "")。
func headingInfo(trimmed string) (int, string) {
	if !strings.HasPrefix(trimmed, "#") {
		return 0, ""
	}
	i := 0
	for i < len(trimmed) && trimmed[i] == '#' && i < 6 {
		i++
	}
	if i >= len(trimmed) || trimmed[i] != ' ' {
		return 0, ""
	}
	return i, strings.TrimSpace(trimmed[i+1:])
}

// formatPath 将标题路径格式化为 Markdown 标题行序列，如 "# 部署\n## 环境要求"。
func formatPath(path []*headingNode) string {
	lines := make([]string, 0, len(path))
	for _, n := range path {
		lines = append(lines, strings.Repeat("#", n.level)+" "+n.title)
	}
	return strings.Join(lines, "\n")
}

// buildParentBlock 构建章节父块：标题路径 + 子标题列表 + 正文首句摘要。
// 整体受 maxTokens 约束，超长时按 Token 窗口截断。
func (c *HierarchyChunker) buildParentBlock(path []*headingNode) string {
	node := path[len(path)-1]
	var sb strings.Builder
	sb.WriteString(formatPath(path))

	titles := make([]string, 0, len(node.children))
	for _, ch := range node.children {
		titles = append(titles, ch.title)
	}
	if len(titles) > 0 {
		sb.WriteString("\n\n本章包含：")
		sb.WriteString(strings.Join(titles, "、"))
	}
	if head := summarizeHead(node.body, c.maxTokens); head != "" {
		sb.WriteString("\n\n")
		sb.WriteString(head)
	}
	content := strings.TrimSpace(sb.String())
	if content == "" {
		return ""
	}
	return fitToTokenWindow(content, c.maxTokens)
}

// summarizeHead 取正文前若干非空行作为章节摘要，token 数不超过 maxTokens/2。
func summarizeHead(body []string, maxTokens int) string {
	var lines []string
	var tokens int
	limit := maxTokens / 2
	for _, ln := range body {
		trimmed := strings.TrimSpace(ln)
		if trimmed == "" {
			continue
		}
		lineTokens := utils.CountTokens(trimmed)
		if tokens+lineTokens > limit {
			break
		}
		lines = append(lines, trimmed)
		tokens += lineTokens
	}
	return strings.Join(lines, "\n")
}

// splitBodyBlocks 将章节正文切分为子块：每个子块以完整标题路径为前缀。
// 超长正文按 Token 窗口切片（复用自然边界回溯），并为路径前缀预留 Token 预算。
func (c *HierarchyChunker) splitBodyBlocks(path []*headingNode, body []string) []string {
	text := strings.TrimSpace(strings.Join(body, "\n"))
	if text == "" {
		return nil
	}

	prefix := ""
	if len(path) > 0 {
		prefix = formatPath(path) + "\n\n"
	}
	// 为路径前缀预留预算，保证「前缀 + 正文」整块不超 maxTokens。
	budget := c.maxTokens
	if prefix != "" {
		budget = c.maxTokens - utils.CountTokens(prefix)
		if budget < c.maxTokens/4 {
			budget = c.maxTokens / 4
		}
	}

	if utils.CountTokens(text) <= budget {
		return []string{prefix + text}
	}

	chunker := NewChunker(budget, c.overlapTokens)
	var out []string
	for _, p := range chunker.sliceByTokens(text) {
		if strings.TrimSpace(p) == "" {
			continue
		}
		out = append(out, prefix+p)
	}
	return out
}
