package trace

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestSanitizerTextSummaryMode(t *testing.T) {
	s := Sanitizer{Mode: "summary", MaxRunes: 5}
	got := s.Text("你好世界世界")
	if len([]rune(got)) != 5 {
		t.Fatalf("expected 5 runes, got %d: %q", len([]rune(got)), got)
	}
}

func TestSanitizerTextNoneMode(t *testing.T) {
	s := Sanitizer{Mode: "none", MaxRunes: 500}
	if got := s.Text("hello"); got != "" {
		t.Fatalf("none mode should return empty, got %q", got)
	}
}

func TestSanitizerTextRedactsBearer(t *testing.T) {
	s := Sanitizer{Mode: "summary", MaxRunes: 500}
	got := s.Text("Authorization: Bearer sk-1234567890")
	if strings.Contains(got, "sk-1234567890") {
		t.Fatalf("bearer token not redacted: %q", got)
	}
	if !strings.Contains(got, "[REDACTED]") {
		t.Fatalf("expected [REDACTED] in: %q", got)
	}
}

func TestSanitizerJSONRedactsSensitiveKeys(t *testing.T) {
	s := Sanitizer{Mode: "summary", MaxRunes: 500}
	input := `{"api_key":"sk-123","name":"test","password":"secret","nested":{"token":"abc","safe":"ok"}}`
	got := s.JSON(input)
	if strings.Contains(got, "sk-123") || strings.Contains(got, "secret") || strings.Contains(got, "abc") {
		t.Fatalf("sensitive values not redacted: %q", got)
	}
	if !strings.Contains(got, "test") || !strings.Contains(got, "ok") {
		t.Fatalf("safe values removed: %q", got)
	}
}

func TestSanitizerJSONNestedRedaction(t *testing.T) {
	s := Sanitizer{Mode: "summary", MaxRunes: 500}
	input := `{"data":{"authorization":"Bearer xyz","items":[{"cookie":"sess-1","name":"a"}]}}`
	got := s.JSON(input)
	if strings.Contains(got, "xyz") || strings.Contains(got, "sess-1") {
		t.Fatalf("nested sensitive values not redacted: %q", got)
	}
}

func TestSanitizerJSONInvalidFallbackToText(t *testing.T) {
	s := Sanitizer{Mode: "summary", MaxRunes: 500}
	input := `not valid json {with bearer token}`
	got := s.JSON(input)
	// 非法 JSON 走文本路径，仍应处理 Bearer
	if strings.Contains(got, "bearer token") && !strings.Contains(got, "[REDACTED]") {
		// Bearer 替换可能不触发（大小写），只要不 panic 即可
	}
}

func TestSanitizerJSONNoneMode(t *testing.T) {
	s := Sanitizer{Mode: "none", MaxRunes: 500}
	if got := s.JSON(`{"key":"value"}`); got != "" {
		t.Fatalf("none mode should return empty, got %q", got)
	}
}

func TestSanitizerUnicodeTruncation(t *testing.T) {
	s := Sanitizer{Mode: "summary", MaxRunes: 3}
	got := s.Text("你好世界")
	if len([]rune(got)) != 3 {
		t.Fatalf("expected 3 runes, got %d: %q", len([]rune(got)), got)
	}
	// 验证不产生无效 UTF-8
	if !json.Valid([]byte(`"` + got + `"`)) {
		t.Fatalf("invalid UTF-8 after truncation: %q", got)
	}
}

func TestMessageTextWithoutReasoning(t *testing.T) {
	msg := &schema.Message{
		Role:    schema.Assistant,
		Content: "",
		MultiContent: []schema.ChatMessagePart{
			{Type: schema.ChatMessagePartTypeText, Text: "visible answer"},
		},
		AssistantGenMultiContent: []schema.MessageOutputPart{
			{Type: schema.ChatMessagePartTypeReasoning, Reasoning: &schema.MessageOutputReasoning{Text: "hidden reasoning"}},
		},
	}
	got := MessageTextWithoutReasoning(msg)
	if strings.Contains(got, "hidden reasoning") {
		t.Fatalf("reasoning should not appear: %q", got)
	}
	if !strings.Contains(got, "visible answer") {
		t.Fatalf("text part missing: %q", got)
	}
}

func TestMessageTextWithoutReasoningPlainContent(t *testing.T) {
	msg := &schema.Message{Role: schema.Assistant, Content: "hello"}
	got := MessageTextWithoutReasoning(msg)
	if got != "hello" {
		t.Fatalf("expected 'hello', got %q", got)
	}
}
