package tool

import (
	"context"
	"testing"
)

func TestKnowledgeBaseIDsContextUsesDefensiveCopies(t *testing.T) {
	ids := []string{"kb-1"}
	ctx := WithKnowledgeBaseIDs(context.Background(), ids)
	ids[0] = "mutated-input"

	got := KnowledgeBaseIDsFromContext(ctx)
	if len(got) != 1 || got[0] != "kb-1" {
		t.Fatalf("KnowledgeBaseIDsFromContext() = %v, want [kb-1]", got)
	}

	got[0] = "mutated-output"
	again := KnowledgeBaseIDsFromContext(ctx)
	if len(again) != 1 || again[0] != "kb-1" {
		t.Fatalf("second KnowledgeBaseIDsFromContext() = %v, want [kb-1]", again)
	}
}
