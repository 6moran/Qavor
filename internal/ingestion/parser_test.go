package ingestion

import (
	"context"
	"strings"
	"testing"
)

// fakePythonParser 记录调用,可返回预置结果。
type fakePythonParser struct {
	callCount int
	result    ParseResult
	err       error
}

func (f *fakePythonParser) Parse(_ context.Context, _ ParseInput) (ParseResult, error) {
	f.callCount++
	if f.err != nil {
		return ParseResult{}, f.err
	}
	return f.result, nil
}

func TestParseTxtDirectly(t *testing.T) {
	p := NewParser(&fakePythonParser{})
	got, err := p.Parse(context.Background(), ParseInput{Filename: "a.txt", Content: []byte("  hello  \r\n\r\n world \r\n")})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got.Markdown, "hello") || !strings.Contains(got.Markdown, "world") {
		t.Fatalf("markdown mismatch: %q", got.Markdown)
	}
}

func TestParseBinaryDelegatesToPython(t *testing.T) {
	for _, ext := range []string{".docx", ".pptx", ".pdf"} {
		python := &fakePythonParser{}
		p := NewParser(python)
		if _, err := p.Parse(context.Background(), ParseInput{Filename: "a" + ext, Content: []byte("data")}); err != nil {
			t.Fatalf("%s: unexpected error: %v", ext, err)
		}
		if python.callCount != 1 {
			t.Fatalf("%s: python callCount = %d, want 1", ext, python.callCount)
		}
	}
}

func TestParseBinaryWithoutPython(t *testing.T) {
	p := NewParser(nil)
	if _, err := p.Parse(context.Background(), ParseInput{Filename: "a.docx", Content: []byte("data")}); err == nil {
		t.Fatal("expected error without python parser")
	}
}

func TestParseForwardsInputToPython(t *testing.T) {
	python := &fakePythonParser{}
	p := NewParser(python)
	_, err := p.Parse(context.Background(), ParseInput{Filename: "a.pdf", Content: []byte("pdf-data"), Path: "knowledge/kb-1/f-1.pdf"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if python.callCount != 1 {
		t.Fatalf("python callCount = %d, want 1", python.callCount)
	}
}

func TestParseUnsupportedExtension(t *testing.T) {
	p := NewParser(nil)
	if _, err := p.Parse(context.Background(), ParseInput{Filename: "a.zip", Content: []byte("data")}); err == nil {
		t.Fatal("expected error for unsupported extension")
	}
}
