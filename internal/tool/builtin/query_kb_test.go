package builtin

import (
	"context"
	"errors"
	"testing"

	"Qavor/internal/service"
	"Qavor/internal/tool"
)

type fakeRAGService struct {
	lastKBIDs    []string
	lastQuery    string
	lastTopK     int
	result       *service.RAGRetrieveResult
	err          error
	answerCalled bool
}

func (f *fakeRAGService) Retrieve(_ context.Context, kbIDs []string, query string, topK int) (*service.RAGRetrieveResult, error) {
	f.lastKBIDs = append([]string(nil), kbIDs...)
	f.lastQuery = query
	f.lastTopK = topK
	if f.err != nil {
		return nil, f.err
	}
	return f.result, nil
}

func (f *fakeRAGService) Answer(context.Context, []string, string) (*service.RAGAnswerResult, error) {
	f.answerCalled = true
	return nil, errors.New("Answer must not be called by query_kb tool")
}

func (f *fakeRAGService) RetrieveTest(context.Context, []string, string, *service.RetrievalTestConfig) (*service.RAGRetrieveResult, error) {
	return nil, errors.New("RetrieveTest must not be called by query_kb tool")
}

func newQueryKBTool(fake *fakeRAGService) *QueryKBTool {
	return NewQueryKBTool(fake)
}

func TestQueryKBToolExecutesWithScopeFromContext(t *testing.T) {
	fake := &fakeRAGService{result: &service.RAGRetrieveResult{
		QueryText: "refund",
		Chunks: []service.RAGChunk{{
			KBID: "kb-1", ChunkID: "chunk-1", FileID: "file-1",
			Filename: "refund.md", Content: "refund policy", Score: 0.91,
		}},
	}}
	queryTool := newQueryKBTool(fake)

	ctx := tool.WithKnowledgeBaseIDs(context.Background(), []string{"kb-1"})
	out, err := queryTool.Execute(ctx, map[string]any{
		"query_text": "refund",
		"top_k":      float64(5),
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(fake.lastKBIDs) != 1 || fake.lastKBIDs[0] != "kb-1" {
		t.Fatalf("knowledge base ids = %v, want [kb-1]", fake.lastKBIDs)
	}
	if fake.lastQuery != "refund" {
		t.Fatalf("query = %q, want refund", fake.lastQuery)
	}
	if fake.lastTopK != 5 {
		t.Fatalf("topK = %d, want 5", fake.lastTopK)
	}
	if fake.answerCalled {
		t.Fatal("Answer() was called, want retrieval only")
	}
	chunks, ok := out.(map[string]any)["chunks"].([]service.RAGChunk)
	if !ok || len(chunks) != 1 {
		t.Fatalf("chunks = %#v, want one RAGChunk", out)
	}
	if chunks[0].KBID != "kb-1" {
		t.Fatalf("chunk kb id = %q, want kb-1", chunks[0].KBID)
	}
}

func TestQueryKBToolUsesDefaultTopKWhenAbsent(t *testing.T) {
	fake := &fakeRAGService{result: &service.RAGRetrieveResult{QueryText: "refund"}}
	queryTool := newQueryKBTool(fake)

	ctx := tool.WithKnowledgeBaseIDs(context.Background(), []string{"kb-1"})
	if _, err := queryTool.Execute(ctx, map[string]any{"query_text": "refund"}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if fake.lastTopK != QueryKBDefaultTopK {
		t.Fatalf("topK = %d, want tool default %d", fake.lastTopK, QueryKBDefaultTopK)
	}
}

func TestQueryKBToolRejectsMissingScope(t *testing.T) {
	fake := &fakeRAGService{result: &service.RAGRetrieveResult{QueryText: "refund"}}
	queryTool := newQueryKBTool(fake)

	if _, err := queryTool.Execute(context.Background(), map[string]any{"query_text": "refund"}); err == nil {
		t.Fatal("Execute() error = nil, want missing scope error")
	}
}

func TestQueryKBToolRejectsBlankQuery(t *testing.T) {
	fake := &fakeRAGService{result: &service.RAGRetrieveResult{QueryText: "refund"}}
	queryTool := newQueryKBTool(fake)

	ctx := tool.WithKnowledgeBaseIDs(context.Background(), []string{"kb-1"})
	for _, args := range []map[string]any{
		{},
		{"query_text": ""},
		{"query_text": "   "},
		{"query_text": 123},
	} {
		if _, err := queryTool.Execute(ctx, args); err == nil {
			t.Fatalf("Execute(%v) error = nil, want invalid query error", args)
		}
	}
}

func TestQueryKBToolFallsBackToDefaultOnInvalidTopK(t *testing.T) {
	fake := &fakeRAGService{result: &service.RAGRetrieveResult{QueryText: "refund"}}
	queryTool := newQueryKBTool(fake)

	ctx := tool.WithKnowledgeBaseIDs(context.Background(), []string{"kb-1"})
	for _, topK := range []any{float64(0), float64(-3), float64(21), "five", true} {
		fake.lastTopK = 0
		if _, err := queryTool.Execute(ctx, map[string]any{"query_text": "refund", "top_k": topK}); err != nil {
			t.Fatalf("Execute(top_k=%v) error = %v, want silent fallback to default", topK, err)
		}
		if fake.lastTopK != QueryKBDefaultTopK {
			t.Fatalf("Execute(top_k=%v) topK = %d, want default %d", topK, fake.lastTopK, QueryKBDefaultTopK)
		}
	}
}

func TestQueryKBToolReturnsNonNilEmptyChunks(t *testing.T) {
	fake := &fakeRAGService{result: &service.RAGRetrieveResult{QueryText: "refund"}}
	queryTool := newQueryKBTool(fake)

	ctx := tool.WithKnowledgeBaseIDs(context.Background(), []string{"kb-1"})
	out, err := queryTool.Execute(ctx, map[string]any{"query_text": "refund"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	chunks, ok := out.(map[string]any)["chunks"].([]service.RAGChunk)
	if !ok {
		t.Fatalf("chunks type = %T, want []service.RAGChunk", out.(map[string]any)["chunks"])
	}
	if chunks == nil {
		t.Fatal("chunks = nil, want non-nil empty slice")
	}
}

func TestQueryKBToolForwardsServiceError(t *testing.T) {
	fake := &fakeRAGService{err: errors.New("retrieval failed")}
	queryTool := newQueryKBTool(fake)

	ctx := tool.WithKnowledgeBaseIDs(context.Background(), []string{"kb-1"})
	if _, err := queryTool.Execute(ctx, map[string]any{"query_text": "refund"}); err == nil {
		t.Fatal("Execute() error = nil, want forwarded retrieval error")
	}
}

func TestQueryKBToolMetadata(t *testing.T) {
	meta := (&QueryKBTool{}).Meta()
	if meta.Name != tool.QueryKBToolName {
		t.Fatalf("name = %q, want %q", meta.Name, tool.QueryKBToolName)
	}
	if meta.Category != tool.CategoryKnowledge {
		t.Fatalf("category = %q, want %q", meta.Category, tool.CategoryKnowledge)
	}
}
