package agent

import (
	"strings"
	"testing"
	"time"

	"Qavor/internal/tool"
)

func TestBuildMainAgentInstructionPreservesConfiguredContent(t *testing.T) {
	base := "自定义提示\n\nSkill: search\n\n长期记忆: Go 工程师"
	got := buildMainAgentInstruction(base, []string{tool.QueryKBToolName})
	for _, want := range []string{base, "你可以使用 query_kb", "内部资料"} {
		if !strings.Contains(got, want) {
			t.Fatalf("instruction missing %q: %s", want, got)
		}
	}
	for _, forbidden := range []string{"必须先调用 query_kb", "对于每个用户问题"} {
		if strings.Contains(got, forbidden) {
			t.Fatalf("instruction contains forced routing %q: %s", forbidden, got)
		}
	}
}

func TestBuildMainAgentInstructionOmitsRAGWithoutAvailableTool(t *testing.T) {
	got := buildMainAgentInstruction("自定义提示", []string{tool.AskUserToolName})
	if strings.Contains(got, "query_kb") {
		t.Fatalf("instruction unexpectedly mentions query_kb: %s", got)
	}
}

func TestBuildAgentInstructionUsesDefaultAndDoesNotDuplicateRAGGuidance(t *testing.T) {
	once := buildMainAgentInstruction("", []string{tool.QueryKBToolName})
	twice := appendKnowledgeToolGuidance(once, []string{tool.QueryKBToolName})
	if !strings.Contains(once, defaultAgentInstruction) {
		t.Fatalf("default instruction missing: %s", once)
	}
	if strings.Count(twice, knowledgeToolGuidanceMarker) != 1 {
		t.Fatalf("RAG guidance duplicated: %s", twice)
	}
}

func TestBuildSubagentInstructionPreservesContentAndAddsOwnGuidance(t *testing.T) {
	now := time.Date(2026, 8, 31, 10, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	got := buildSubagentInstruction("子智能体提示", []string{tool.QueryKBToolName}, now)
	for _, want := range []string{"子智能体提示", "2026-08-31 10:00:00", "report_need_input", "你可以使用 query_kb"} {
		if !strings.Contains(got, want) {
			t.Fatalf("subagent instruction missing %q: %s", want, got)
		}
	}
	if strings.Contains(got, "子智能体具备向用户提问的能力") {
		t.Fatalf("subagent received main-agent-only guidance: %s", got)
	}
}
