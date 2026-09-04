package agent

import (
	"context"
	"testing"

	"Qavor/internal/tool"
)

type testBuiltinTool struct {
	name string
}

func (t *testBuiltinTool) Meta() tool.ToolMeta {
	return tool.ToolMeta{Name: t.name, Description: t.name, Category: tool.CategorySystem}
}

func (t *testBuiltinTool) Execute(context.Context, map[string]any) (any, error) {
	return nil, nil
}

func TestAvailableBuiltinToolNamesRequiresConfiguredKnowledgeAndRegisteredTool(t *testing.T) {
	registry := tool.NewRegistry()
	registry.Register(&testBuiltinTool{name: tool.QueryKBToolName})

	withoutKB := availableBuiltinToolNames(&AgentConfig{}, registry)
	if containsTool(withoutKB, tool.QueryKBToolName) {
		t.Fatalf("query_kb available without knowledge binding: %v", withoutKB)
	}
	withoutKBExplicit := availableBuiltinToolNames(&AgentConfig{Tools: []string{tool.QueryKBToolName}}, registry)
	if containsTool(withoutKBExplicit, tool.QueryKBToolName) {
		t.Fatalf("explicit query_kb available without knowledge binding: %v", withoutKBExplicit)
	}

	withKB := availableBuiltinToolNames(&AgentConfig{Knowledges: []string{"kb-1"}}, registry)
	if !containsTool(withKB, tool.QueryKBToolName) {
		t.Fatalf("query_kb missing with knowledge binding: %v", withKB)
	}
}

func TestAvailableBuiltinToolNamesOmitsMissingAndDuplicateTools(t *testing.T) {
	registry := tool.NewRegistry()
	registry.Register(&testBuiltinTool{name: tool.QueryKBToolName})
	cfg := &AgentConfig{
		Knowledges: []string{"kb-1"},
		Tools:      []string{tool.QueryKBToolName, "missing_tool"},
	}

	got := availableBuiltinToolNames(cfg, registry)
	if len(got) != 1 || got[0] != tool.QueryKBToolName {
		t.Fatalf("available names = %v, want [query_kb]", got)
	}
}
