package ingestion

import (
	"context"
	"errors"
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
		python := &fakePythonParser{result: ParseResult{Markdown: "parsed binary content"}}
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
	python := &fakePythonParser{result: ParseResult{Markdown: "parsed binary content"}}
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

func TestParseFinalizesAllSuccessfulResultsWithCleaner(t *testing.T) {
	python := &fakePythonParser{result: ParseResult{Markdown: "#   \r\n\r\n\r\n正文  \r\n"}}
	p := NewParser(python)

	got, err := p.Parse(context.Background(), ParseInput{Filename: "a.pdf", Content: []byte("pdf")})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := "正文"; got.Markdown != want {
		t.Fatalf("Markdown = %q, want %q", got.Markdown, want)
	}
	if got.Metadata == nil {
		t.Fatal("Metadata was not initialized")
	}
	if _, degraded := got.Metadata["cleaner_degraded"]; degraded {
		t.Fatalf("unexpected cleaner_degraded metadata: %#v", got.Metadata)
	}
}

func TestParseRejectsResultWithoutVisibleContentBeforeCleaning(t *testing.T) {
	p := NewParser(&fakePythonParser{result: ParseResult{Markdown: " \r\n<!-- image -->\r\n![]()\r\n# \r\n"}})
	_, err := p.Parse(context.Background(), ParseInput{Filename: "a.pdf", Content: []byte("pdf")})
	var parseErr *ParserError
	if !errors.As(err, &parseErr) {
		t.Fatalf("error = %v, want ParserError", err)
	}
	if parseErr.Code != "PARSER_EMPTY_CONTENT" || parseErr.Message != "文档解析结果为空" {
		t.Fatalf("ParserError = %#v", parseErr)
	}
}
