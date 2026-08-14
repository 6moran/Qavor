package rag

import (
	"context"
	"testing"

	"Qavor/internal/model/entity"
	"Qavor/internal/repository"

	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
	"github.com/pgvector/pgvector-go"
)

type dynamicRetrieverKBRepo struct {
	bases map[string]*entity.KnowledgeBase
}

func (r *dynamicRetrieverKBRepo) Create(*entity.KnowledgeBase) error { return nil }
func (r *dynamicRetrieverKBRepo) FindByKBID(kbID string) (*entity.KnowledgeBase, error) {
	return r.bases[kbID], nil
}
func (r *dynamicRetrieverKBRepo) FindByKBIDs(kbIDs []string) ([]*entity.KnowledgeBase, error) {
	bases := make([]*entity.KnowledgeBase, 0, len(kbIDs))
	for _, kbID := range kbIDs {
		if base := r.bases[kbID]; base != nil {
			bases = append(bases, base)
		}
	}
	return bases, nil
}
func (r *dynamicRetrieverKBRepo) List(int, int, string) ([]*entity.KnowledgeBase, int64, error) {
	return nil, 0, nil
}
func (r *dynamicRetrieverKBRepo) Update(*entity.KnowledgeBase) error { return nil }
func (r *dynamicRetrieverKBRepo) DeleteByKBID(string) error          { return nil }
func (r *dynamicRetrieverKBRepo) GetStatsByKBIDs([]string) (map[string]*repository.KnowledgeBaseStats, error) {
	return nil, nil
}

type dynamicRetrieverEmbedder struct{}

func (dynamicRetrieverEmbedder) EmbedStrings(context.Context, []string, ...embedding.Option) ([][]float64, error) {
	return [][]float64{{1, 0, 0}}, nil
}

type dynamicRetrieverResolver struct {
	lastEmbeddingModelID uint
	embedder             embedding.Embedder
	chatModel            model.ToolCallingChatModel
}

func (r *dynamicRetrieverResolver) ResolveEmbedding(_ context.Context, modelID uint) (embedding.Embedder, error) {
	r.lastEmbeddingModelID = modelID
	return r.embedder, nil
}

func (r *dynamicRetrieverResolver) ResolveChatModel(context.Context, uint) (model.ToolCallingChatModel, error) {
	return r.chatModel, nil
}

func (r *dynamicRetrieverResolver) ResolveReranker(context.Context, uint) (Reranker, error) {
	return nil, nil
}

type dynamicRetrieverChunkRepo struct {
	lastKBIDs []string
	lastLimit int
	rows      []repository.ChunkWithFile
	rowsByKB  map[string][]repository.ChunkWithFile
}

func (r *dynamicRetrieverChunkRepo) FindByFileID(context.Context, string, string) ([]*entity.KnowledgeChunk, error) {
	return nil, nil
}
func (r *dynamicRetrieverChunkRepo) ReplaceByFileID(context.Context, string, string, []*entity.KnowledgeChunk) error {
	return nil
}
func (r *dynamicRetrieverChunkRepo) FindNearestByKBIDs(_ context.Context, kbIDs []string, _ pgvector.Vector, limit int) ([]repository.ChunkWithFile, error) {
	r.lastKBIDs = append([]string(nil), kbIDs...)
	r.lastLimit = limit
	if len(kbIDs) > 0 && r.rowsByKB != nil {
		return r.rowsByKB[kbIDs[0]], nil
	}
	return r.rows, nil
}
func (r *dynamicRetrieverChunkRepo) FindKeywordByKBIDs(context.Context, []string, string, int) ([]repository.ChunkWithFile, error) {
	return nil, nil
}
func (r *dynamicRetrieverChunkRepo) FindRandomByKBIDs(context.Context, []string, int) ([]repository.ChunkWithFile, error) {
	return nil, nil
}

func TestDynamicRetrieverResolvesEmbeddingAndForwardsTopK(t *testing.T) {
	kbRepo := &dynamicRetrieverKBRepo{bases: map[string]*entity.KnowledgeBase{
		"kb-1": {KBID: "kb-1", EmbeddingModelID: 7},
	}}
	resolver := &dynamicRetrieverResolver{embedder: dynamicRetrieverEmbedder{}}
	chunkRepo := &dynamicRetrieverChunkRepo{rows: []repository.ChunkWithFile{{
		KBID: "kb-1", ChunkID: "chunk-1", FileID: "file-1", Filename: "refund.md", Content: "refund policy", Score: 0.91,
	}}}

	ret := NewDynamicRetriever(kbRepo, resolver, chunkRepo, 5)
	docs, err := ret.Retrieve(
		context.Background(),
		"refund policy",
		WithKnowledgeBaseIDs([]string{"kb-1"}),
		retriever.WithTopK(3),
	)
	if err != nil {
		t.Fatalf("Retrieve() error = %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("len(docs) = %d, want 1", len(docs))
	}
	if resolver.lastEmbeddingModelID != 7 {
		t.Fatalf("embedding model id = %d, want 7", resolver.lastEmbeddingModelID)
	}
	if chunkRepo.lastLimit != 3 {
		t.Fatalf("limit = %d, want 3", chunkRepo.lastLimit)
	}
	if got := docs[0].MetaData[MetaKeyKBID]; got != "kb-1" {
		t.Fatalf("kb id = %v, want kb-1", got)
	}
}

func TestDynamicRetrieverRetrieveRankedKeepsEmbeddingGroupsIndependent(t *testing.T) {
	kbRepo := &dynamicRetrieverKBRepo{bases: map[string]*entity.KnowledgeBase{
		"kb-low":  {KBID: "kb-low", EmbeddingModelID: 7},
		"kb-high": {KBID: "kb-high", EmbeddingModelID: 8},
	}}
	resolver := &dynamicRetrieverResolver{embedder: dynamicRetrieverEmbedder{}}
	chunkRepo := &dynamicRetrieverChunkRepo{rowsByKB: map[string][]repository.ChunkWithFile{
		"kb-low": {
			{KBID: "kb-low", ChunkID: "low-first", Content: "低分第一", Score: 0.2},
			{KBID: "kb-low", ChunkID: "low-second", Content: "低分第二", Score: 0.1},
		},
		"kb-high": {
			{KBID: "kb-high", ChunkID: "high-first", Content: "高分第一", Score: 0.99},
		},
	}}
	dynamic := NewDynamicRetriever(kbRepo, resolver, chunkRepo, 20)

	lists, err := dynamic.RetrieveRanked(context.Background(), "查询",
		WithKnowledgeBaseIDs([]string{"kb-low", "kb-high"}), retriever.WithTopK(10))
	if err != nil {
		t.Fatalf("RetrieveRanked: %v", err)
	}
	if len(lists) != 2 {
		t.Fatalf("排名列表数量=%d，期望 2", len(lists))
	}
	if len(lists[0]) != 2 || lists[0][0].ID != "low-first" || lists[0][1].ID != "low-second" {
		t.Fatalf("第一模型组顺序=%v", rankedDocumentIDs(lists[0]))
	}
	if len(lists[1]) != 1 || lists[1][0].ID != "high-first" {
		t.Fatalf("第二模型组顺序=%v", rankedDocumentIDs(lists[1]))
	}
}

func rankedDocumentIDs(documents []*schema.Document) []string {
	ids := make([]string, 0, len(documents))
	for _, document := range documents {
		ids = append(ids, document.ID)
	}
	return ids
}

type sharedAnswerRetriever struct {
	called bool
	docs   []*schema.Document
}

func (r *sharedAnswerRetriever) Retrieve(context.Context, string, ...retriever.Option) ([]*schema.Document, error) {
	r.called = true
	return r.docs, nil
}

type sharedAnswerChatModel struct{}

func (sharedAnswerChatModel) Generate(context.Context, []*schema.Message, ...model.Option) (*schema.Message, error) {
	return &schema.Message{Role: schema.Assistant, Content: "generated answer"}, nil
}

func (sharedAnswerChatModel) Stream(context.Context, []*schema.Message, ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return schema.StreamReaderFromArray([]*schema.Message{{Role: schema.Assistant, Content: "generated answer"}}), nil
}

func (m sharedAnswerChatModel) WithTools([]*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}

func TestDynamicAnswerEngineUsesInjectedRetriever(t *testing.T) {
	kbRepo := &dynamicRetrieverKBRepo{bases: map[string]*entity.KnowledgeBase{
		"kb-1": {KBID: "kb-1", EmbeddingModelID: 7, ChatModelID: 8},
	}}
	ret := &sharedAnswerRetriever{docs: []*schema.Document{{
		Content:  "refund policy",
		MetaData: map[string]any{MetaKeyKBID: "kb-1", MetaKeyChunkID: "chunk-1"},
	}}}
	resolver := &dynamicRetrieverResolver{chatModel: sharedAnswerChatModel{}}
	engine := NewDynamicAnswerEngine(kbRepo, resolver, ret)

	out, err := engine.Answer(context.Background(), AnswerInput{
		KnowledgeBaseIDs: []string{"kb-1"},
		Query:            "refund",
	})
	if err != nil {
		t.Fatalf("Answer() error = %v", err)
	}
	if !ret.called {
		t.Fatal("injected retriever was not called")
	}
	if out.Answer != "generated answer" {
		t.Fatalf("answer = %q, want generated answer", out.Answer)
	}
}
