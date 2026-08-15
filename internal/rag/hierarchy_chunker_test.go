package rag

import (
	"strings"
	"testing"
)

func TestHierarchyChunkerProducesParentAndChildBlocks(t *testing.T) {
	md := `# 部署
系统部署说明正文。
## 环境要求
需要 8G 内存。
### 硬件
推荐 16G 内存。
### 软件
需要 Go 1.25。
## 网络
需要开放 8080 端口。
`
	chunks, err := NewHierarchyChunker(200, 0).Split(md)
	if err != nil {
		t.Fatalf("Split() error = %v", err)
	}
	joined := strings.Join(chunks, "\n")

	// 1. 父块：章节标题 + 子标题列表 + 正文摘要。
	if !strings.Contains(joined, "本章包含：环境要求、网络") {
		t.Fatalf("缺少「部署」章节父块（子标题列表）: %q", joined)
	}
	if !strings.Contains(joined, "本章包含：硬件、软件") {
		t.Fatalf("缺少「环境要求」章节父块（子标题列表）: %q", joined)
	}

	// 2. 子块：携带完整标题路径前缀。
	if !strings.Contains(joined, "# 部署\n## 环境要求\n### 硬件\n\n推荐 16G 内存。") {
		t.Fatalf("硬件子块缺少完整标题路径前缀: %q", joined)
	}
	if !strings.Contains(joined, "# 部署\n## 网络\n\n需要开放 8080 端口。") {
		t.Fatalf("网络子块缺少标题路径前缀: %q", joined)
	}

	// 3. 所有原始内容均未丢失。
	for _, want := range []string{"需要 8G 内存。", "需要 Go 1.25。", "系统部署说明正文。"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("内容缺失 %q: %q", want, joined)
		}
	}

	// 4. 父块 + 至少 4 个子块（前言、环境要求、硬件、软件、网络）。
	if len(chunks) < 5 {
		t.Fatalf("len(chunks) = %d, want >= 5（父块 + 子块）", len(chunks))
	}
}

func TestHierarchyChunkerOrdersParentBeforeChildren(t *testing.T) {
	md := "# 章节A\n正文A。\n## 小节A1\n正文A1。\n# 章节B\n正文B。\n"
	chunks, err := NewHierarchyChunker(200, 0).Split(md)
	if err != nil {
		t.Fatalf("Split() error = %v", err)
	}
	parentIdx := -1
	a1Idx := -1
	for i, ch := range chunks {
		switch {
		case strings.HasPrefix(ch, "# 章节A\n## 小节A1\n"):
			a1Idx = i
		case strings.HasPrefix(ch, "# 章节A\n"):
			if strings.Contains(ch, "本章包含：小节A1") {
				parentIdx = i
			}
		}
	}
	if parentIdx == -1 {
		t.Fatalf("未找到「章节A」父块: %q", chunks)
	}
	if a1Idx == -1 {
		t.Fatalf("未找到「小节A1」子块: %q", chunks)
	}
	if parentIdx > a1Idx {
		t.Fatalf("父块(index=%d)应排在子块(index=%d)之前", parentIdx, a1Idx)
	}
}

func TestHierarchyChunkerFallsBackWithoutHeadings(t *testing.T) {
	md := "只有正文没有标题。\n第二行内容。"
	chunks, err := NewHierarchyChunker(800, 0).Split(md)
	if err != nil {
		t.Fatalf("Split() error = %v", err)
	}
	if len(chunks) != 1 {
		t.Fatalf("len(chunks) = %d, want 1（回退通用分块）", len(chunks))
	}
	if !strings.Contains(chunks[0], "只有正文没有标题。") {
		t.Fatalf("chunk 内容异常: %q", chunks[0])
	}
}

func TestHierarchyChunkerSkipsCodeFenceHeadings(t *testing.T) {
	md := "# 用法\n下面是代码：\n```go\n// 这不是标题\n# 注释行\n```\n## 参数\n参数说明。\n"
	chunks, err := NewHierarchyChunker(200, 0).Split(md)
	if err != nil {
		t.Fatalf("Split() error = %v", err)
	}
	joined := strings.Join(chunks, "\n")
	if strings.Contains(joined, "## 注释行") {
		t.Fatalf("代码围栏内的 # 行被误判为标题: %q", joined)
	}
	if !strings.Contains(joined, "## 参数") {
		t.Fatalf("缺少正常标题子块: %q", joined)
	}
}

func TestHierarchyChunkerSplitsLongBodyWithPathPrefix(t *testing.T) {
	// 正文远超 maxTokens：应切分为多个带标题路径前缀的子块。
	longBody := strings.Repeat("这是很长的一段章节正文内容。", 60)
	md := "# 长章节\n" + longBody
	chunks, err := NewHierarchyChunker(40, 0).Split(md)
	if err != nil {
		t.Fatalf("Split() error = %v", err)
	}
	if len(chunks) <= 1 {
		t.Fatalf("len(chunks) = %d, want > 1（长正文需要多子块）", len(chunks))
	}
	for i, ch := range chunks {
		if !strings.HasPrefix(ch, "# 长章节\n") {
			t.Fatalf("chunk %d 缺少标题路径前缀: %q", i, ch)
		}
	}
}

func TestNewSplitterHierarchy(t *testing.T) {
	s := NewSplitter(PresetHierarchy, 800, 0)
	if _, ok := s.(*HierarchyChunker); !ok {
		t.Fatalf("NewSplitter(%q) type = %T, want *HierarchyChunker", PresetHierarchy, s)
	}
	if !IsValidChunkPreset(PresetHierarchy) {
		t.Fatalf("IsValidChunkPreset(%q) = false, want true", PresetHierarchy)
	}
}

func TestHierarchyChunkerRejectsEmptyInput(t *testing.T) {
	if _, err := NewHierarchyChunker(800, 0).Split("   \n\n "); err == nil {
		t.Fatal("Split(空内容) error = nil, want error")
	}
}
