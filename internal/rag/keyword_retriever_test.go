package rag

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"Qavor/internal/model/entity"
	"Qavor/internal/repository"

	"github.com/cloudwego/eino/components/retriever"
	"github.com/pgvector/pgvector-go"
)

type fakeKeywordChunkRepository struct {
	query  string
	kbIDs  []string
	limit  int
	rows   []repository.ChunkWithFile
	err    error
	called bool
}

func (r *fakeKeywordChunkRepository) FindByFileID(context.Context, string, string) ([]*entity.KnowledgeChunk, error) {
	return nil, nil
}
func (r *fakeKeywordChunkRepository) ReplaceByFileID(context.Context, string, string, []*entity.KnowledgeChunk) error {
	return nil
}
func (r *fakeKeywordChunkRepository) FindNearestByKBIDs(context.Context, []string, pgvector.Vector, int) ([]repository.ChunkWithFile, error) {
	return nil, nil
}
func (r *fakeKeywordChunkRepository) FindKeywordByKBIDs(_ context.Context, kbIDs []string, query string, limit int) ([]repository.ChunkWithFile, error) {
	r.called = true
	r.query = query
	r.kbIDs = append([]string(nil), kbIDs...)
	r.limit = limit
	return r.rows, r.err
}
func (r *fakeKeywordChunkRepository) FindRandomByKBIDs(context.Context, []string, int) ([]repository.ChunkWithFile, error) {
	return nil, nil
}

func TestKeywordRetriever_NormalizesAndForwardsOptions(t *testing.T) {
	repo := &fakeKeywordChunkRepository{rows: []repository.ChunkWithFile{{
		ChunkID: "chunk-1", KBID: "kb-1", FileID: "file-1", Filename: "退款.md",
		Content: "退款流程", Score: 0.83,
	}}}
	keyword := NewKeywordRetriever(repo, 20)
	docs, err := keyword.Retrieve(context.Background(), "  API-2026\t 退款\n流程  ",
		WithKnowledgeBaseIDs([]string{"kb-1", "kb-2"}), retriever.WithTopK(7))
	if err != nil {
		t.Fatalf("关键词检索: %v", err)
	}
	if repo.query != "api-2026 退款 流程" || repo.limit != 7 || !reflect.DeepEqual(repo.kbIDs, []string{"kb-1", "kb-2"}) {
		t.Fatalf("query=%q kbIDs=%v limit=%d", repo.query, repo.kbIDs, repo.limit)
	}
	if len(docs) != 1 || docs[0].ID != "chunk-1" || docs[0].Content != "退款流程" {
		t.Fatalf("文档=%+v", docs)
	}
	wantMeta := map[string]any{
		MetaKeyChunkID: "chunk-1", MetaKeyKBID: "kb-1", MetaKeyFileID: "file-1",
		MetaKeyFilename: "退款.md", MetaKeyScore: 0.83,
	}
	if !reflect.DeepEqual(docs[0].MetaData, wantMeta) {
		t.Fatalf("元数据=%#v，期望 %#v", docs[0].MetaData, wantMeta)
	}
}

func TestKeywordRetriever_RejectsEmptyInputs(t *testing.T) {
	tests := []struct {
		name  string
		query string
		opts  []retriever.Option
	}{
		{name: "空查询", query: " \t\n", opts: []retriever.Option{WithKnowledgeBaseIDs([]string{"kb-1"})}},
		{name: "空知识库", query: "退款流程"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeKeywordChunkRepository{}
			_, err := NewKeywordRetriever(repo, 20).Retrieve(context.Background(), tt.query, tt.opts...)
			if err == nil || repo.called {
				t.Fatalf("错误=%v called=%v", err, repo.called)
			}
		})
	}
}

func TestKeywordRetriever_PropagatesCancellationAndWrapsRepositoryError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	canceledRepo := &fakeKeywordChunkRepository{}
	_, err := NewKeywordRetriever(canceledRepo, 20).Retrieve(ctx, "退款流程", WithKnowledgeBaseIDs([]string{"kb-1"}))
	if !errors.Is(err, context.Canceled) || canceledRepo.called {
		t.Fatalf("取消错误=%v called=%v", err, canceledRepo.called)
	}

	repoErr := errors.New("数据库不可用")
	failingRepo := &fakeKeywordChunkRepository{err: repoErr}
	_, err = NewKeywordRetriever(failingRepo, 20).Retrieve(context.Background(), "退款流程", WithKnowledgeBaseIDs([]string{"kb-1"}))
	if !errors.Is(err, ErrRetrievalUnavailable) || !errors.Is(err, repoErr) {
		t.Fatalf("仓储错误包装=%v", err)
	}
}
