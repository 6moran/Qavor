package rag

import (
	"strings"
	"testing"
)

func TestChunkerKeepsTableIntact(t *testing.T) {
	md := "| 列A | 列B |\n| --- | --- |\n| 1 | 2 |\n| 3 | 4 |"
	chunks, err := NewChunker(800, 0).Split(md)
	if err != nil {
		t.Fatalf("Split() error = %v", err)
	}
	if len(chunks) != 1 {
		t.Fatalf("len(chunks) = %d, want 1（表格整体保留）", len(chunks))
	}
	if !strings.Contains(chunks[0], "| 1 | 2 |") {
		t.Fatalf("chunk 缺少表格行: %q", chunks[0])
	}
}

func TestChunkerSplitsLongTextByTokens(t *testing.T) {
	text := strings.Repeat("这是一个用于触发超长切分的中文测试句子。", 60)
	chunks, err := NewChunker(20, 0).Split(text)
	if err != nil {
		t.Fatalf("Split() error = %v", err)
	}
	if len(chunks) <= 1 {
		t.Fatalf("len(chunks) = %d, want > 1（超长文本按 Token 窗口切分）", len(chunks))
	}
}

func TestChunkerRejectsEmptyInput(t *testing.T) {
	if _, err := NewChunker(800, 0).Split("   \n\n "); err == nil {
		t.Fatal("Split(空内容) error = nil, want error")
	}
}

func TestChunkerKeepsImageOCRTextAndURL(t *testing.T) {
	markdown := "# 销售报告\n\n![一季度 120 万 二季度 165 万](https://objects.test/chart.png)"
	chunks, err := NewChunker(800, 100).Split(markdown)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(chunks, "\n")
	for _, expected := range []string{"一季度 120 万", "二季度 165 万", "https://objects.test/chart.png"} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("chunk output %q does not contain %q", joined, expected)
		}
	}
}

func TestChunkerBacktracksToNaturalBoundary(t *testing.T) {
	// 每个句子以句号结尾：切分后每块应尽量结束在句号之后，而不是句子中间。
	text := strings.Repeat("这是一个用于验证自然边界回溯的中文测试句子。", 60)
	chunks, err := NewChunker(40, 0).Split(text)
	if err != nil {
		t.Fatalf("Split() error = %v", err)
	}
	if len(chunks) <= 1 {
		t.Fatalf("len(chunks) = %d, want > 1（超长文本需要切分）", len(chunks))
	}
	for i, ch := range chunks {
		trimmed := strings.TrimSpace(ch)
		if !strings.HasSuffix(trimmed, "。") {
			t.Fatalf("chunk %d 未在自然边界结束: %q", i, trimmed)
		}
	}
}

func TestChunkerBacktracksToParagraphBoundary(t *testing.T) {
	// 长文本包含多个段落：切点应优先落在段落分隔（\n\n）之后。
	paragraph := strings.Repeat("第一段的内容用于填充窗口空间。", 40)
	md := strings.Repeat(paragraph+"\n\n", 4)
	chunks, err := NewChunker(60, 0).Split(md)
	if err != nil {
		t.Fatalf("Split() error = %v", err)
	}
	for i, ch := range chunks {
		trimmed := strings.TrimSpace(ch)
		if trimmed == "" {
			continue
		}
		// 每个块应以完整段落或句号结尾（禁止以残缺句子收尾）。
		if !strings.HasSuffix(trimmed, "。") {
			t.Fatalf("chunk %d 未以自然边界结束: %q", i, trimmed)
		}
	}
}
