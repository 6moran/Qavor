package skill

import (
	"context"
	"testing"
)

func TestExtractSkillSlug(t *testing.T) {
	m := &SkillsMiddleware{}

	tests := []struct {
		path string
		slug string
	}{
		{"skills/kb/SKILL.md", "kb"},
		{"./skills/kb/SKILL.md", "kb"},
		{"skills/knowledge-base/SKILL.md", "knowledge-base"},
		{"some/other/path/SKILL.md", ""},
		{"skills//SKILL.md", ""},
		{"SKILL.md", ""},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := m.extractSkillSlug(tt.path)
			if got != tt.slug {
				t.Errorf("extractSkillSlug(%q) = %q, want %q", tt.path, got, tt.slug)
			}
		})
	}
}

func TestBuildPrompt(t *testing.T) {
	loader := &mockLoader{
		skills: map[string]*SkillMeta{
			"kb": {Slug: "kb", Name: "知识库检索"},
		},
	}
	resolver := NewResolver(loader, nil, nil)
	activation := NewActivationState()
	m := NewSkillsMiddleware(loader, resolver, activation)

	skills := []*SkillMeta{
		{Slug: "kb", Name: "知识库检索"},
		{Slug: "code", Name: "代码分析"},
	}

	prompt, err := BuildPrompt(m, context.Background(), "Base prompt", skills)
	if err != nil {
		t.Fatalf("BuildPrompt() error = %v", err)
	}

	if len(prompt) <= len("Base prompt") {
		t.Error("BuildPrompt() should append skill section to base prompt")
	}
}

func BuildPrompt(m *SkillsMiddleware, ctx context.Context, base string, skills []*SkillMeta) (string, error) {
	return m.BuildPrompt(ctx, base, skills)
}

func TestProcessToolCall_NonReadFile(t *testing.T) {
	loader := &mockLoader{skills: map[string]*SkillMeta{}}
	resolver := NewResolver(loader, nil, nil)
	activation := NewActivationState()
	m := NewSkillsMiddleware(loader, resolver, activation)

	result, err := m.ProcessToolCall(context.Background(), "other_tool", map[string]any{}, "ok")
	if err != nil {
		t.Fatalf("ProcessToolCall() error = %v", err)
	}
	if result != "ok" {
		t.Errorf("ProcessToolCall() result = %v, want ok", result)
	}
}

func TestProcessToolCall_SkillMdDetection(t *testing.T) {
	loader := &mockLoader{
		skills: map[string]*SkillMeta{
			"kb": {Slug: "kb", Name: "知识库检索"},
		},
	}
	resolver := NewResolver(loader, nil, nil)
	activation := NewActivationState()
	m := NewSkillsMiddleware(loader, resolver, activation)

	// read_file 调用但不是 SKILL.md
	_, err := m.ProcessToolCall(context.Background(), "read_file", map[string]any{
		"file_path": "skills/kb/README.md",
	}, "content")
	if err != nil {
		t.Fatalf("ProcessToolCall() error = %v", err)
	}
	if activation.IsActivated("kb") {
		t.Error("Should not activate for non-SKILL.md file")
	}

	// read_file 调用 SKILL.md
	_, err = m.ProcessToolCall(context.Background(), "read_file", map[string]any{
		"file_path": "skills/kb/SKILL.md",
	}, "skill content")
	if err != nil {
		t.Fatalf("ProcessToolCall() error = %v", err)
	}
	if !activation.IsActivated("kb") {
		t.Error("Should activate after reading SKILL.md")
	}
}
