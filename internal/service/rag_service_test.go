package service

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"Qavor/internal/model/entity"
	"Qavor/internal/repository"
	"Qavor/internal/trace"
	"Qavor/pkg/config"
	apperrors "Qavor/pkg/errors"

	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
)

type ragServiceKBRepo struct {
	bases map[string]*entity.KnowledgeBase
}

func (r *ragServiceKBRepo) Create(*entity.KnowledgeBase) error { return nil }
func (r *ragServiceKBRepo) FindByKBID(kbID string) (*entity.KnowledgeBase, error) {
	return r.bases[kbID], nil
}
func (r *ragServiceKBRepo) FindByKBIDs(kbIDs []string) ([]*entity.KnowledgeBase, error) {
	bases := make([]*entity.KnowledgeBase, 0, len(kbIDs))
	for _, kbID := range kbIDs {
		if base := r.bases[kbID]; base != nil {
			bases = append(bases, base)
		}
	}
	return bases, nil
}
func (r *ragServiceKBRepo) List(int, int, string) ([]*entity.KnowledgeBase, int64, error) {
	return nil, 0, nil
}
func (r *ragServiceKBRepo) Update(*entity.KnowledgeBase) error { return nil }
func (r *ragServiceKBRepo) DeleteByKBID(string) error          { return nil }
func (r *ragServiceKBRepo) GetStatsByKBIDs([]string) (map[string]*repository.KnowledgeBaseStats, error) {
	return nil, nil
}

type ragServiceRetriever struct {
	query string
	topK  int
	docs  []*schema.Document
}

func (r *ragServiceRetriever) Retrieve(_ context.Context, query string, opts ...retriever.Option) ([]*schema.Document, error) {
	r.query = query
	o := retriever.GetCommonOptions(&retriever.Options{}, opts...)
	if o.TopK != nil {
		r.topK = *o.TopK
	}
	return r.docs, nil
}

func TestRAGServiceRetrieveReturnsStructuredChunks(t *testing.T) {
	kbRepo := &ragServiceKBRepo{bases: map[string]*entity.KnowledgeBase{
		"kb-1": {KBID: "kb-1", EmbeddingModelID: 7, ChatModelID: 8},
	}}
	ret := &ragServiceRetriever{docs: []*schema.Document{{
		Content: "refund policy",
		MetaData: map[string]any{
			"kb_id": "kb-1", "chunk_id": "chunk-1", "file_id": "file-1", "filename": "refund.md", "score": 0.91,
		},
	}}}
	svc := NewRAGService(config.RAGConfig{VectorTopK: 5}, kbRepo, ret, nil, nil)

	result, err := svc.Retrieve(context.Background(), []string{"kb-1"}, "refund", 4)
	if err != nil {
		t.Fatalf("Retrieve() error = %v", err)
	}
	if ret.query != "refund" {
		t.Fatalf("retriever query = %q, want refund", ret.query)
	}
	if ret.topK != 4 {
		t.Fatalf("retriever topK = %d, want 4", ret.topK)
	}
	if result.QueryText != "refund" {
		t.Fatalf("result query = %q, want refund", result.QueryText)
	}
	if len(result.Chunks) != 1 {
		t.Fatalf("len(chunks) = %d, want 1", len(result.Chunks))
	}
	if result.Chunks[0].KBID != "kb-1" {
		t.Fatalf("chunk kb id = %q, want kb-1", result.Chunks[0].KBID)
	}
}

func TestRAGServiceRetrievePreservesHybridScoresNamesAndOrder(t *testing.T) {
	kbRepo := &ragServiceKBRepo{bases: map[string]*entity.KnowledgeBase{
		"kb-1": {KBID: "kb-1", Name: "产品知识库", EmbeddingModelID: 7, ChatModelID: 8},
	}}
	ret := &ragServiceRetriever{docs: []*schema.Document{
		{ID: "second", Content: "模型排在第一的片段", MetaData: map[string]any{
			"kb_id": "kb-1", "chunk_id": "second", "file_id": "file-2", "filename": "second.md",
			"score": 0.96, "vector_score": 0.72, "keyword_score": 0.61, "rrf_score": 0.032,
			"rerank_score": 0.96, "matched_by": []string{"vector", "keyword"},
		}},
		{ID: "first", Content: "模型排在第二的片段", MetaData: map[string]any{
			"kb_id": "kb-1", "chunk_id": "first", "file_id": "file-1", "filename": "first.md",
			"score": 0.81, "vector_score": 0.8, "rrf_score": 0.016, "matched_by": []string{"vector"},
		}},
	}}
	svc := NewRAGService(config.RAGConfig{RerankTopK: 5}, kbRepo, ret, nil, nil)

	result, err := svc.Retrieve(context.Background(), []string{"kb-1"}, "退款", 2)
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}
	if len(result.Chunks) != 2 || result.Chunks[0].ChunkID != "second" || result.Chunks[1].ChunkID != "first" {
		t.Fatalf("结果顺序=%+v", result.Chunks)
	}
	first := result.Chunks[0]
	if first.KBName != "产品知识库" || first.VectorScore == nil || *first.VectorScore != 0.72 || first.KeywordScore == nil || *first.KeywordScore != 0.61 || first.RRFScore == nil || *first.RRFScore != 0.032 || first.RerankScore == nil || *first.RerankScore != 0.96 {
		t.Fatalf("阶段分数或知识库名称=%+v", first)
	}
	if !reflect.DeepEqual(first.MatchedBy, []string{"vector", "keyword"}) || first.Score != 0.96 {
		t.Fatalf("最终契约=%+v", first)
	}
	if result.Chunks[1].KeywordScore != nil || result.Chunks[1].RerankScore != nil {
		t.Fatalf("缺失的可选分数不应被填充=%+v", result.Chunks[1])
	}
}

func assertRAGInvalidRequest(t *testing.T, err error) {
	t.Helper()
	var biz *apperrors.BizError
	if !errors.As(err, &biz) {
		t.Fatalf("error = %v, want BizError", err)
	}
	if biz.Code != CodeRAGInvalidRequest {
		t.Fatalf("error code = %d, want %d (message: %s)", biz.Code, CodeRAGInvalidRequest, biz.Message)
	}
}

func TestRAGServiceRetrieveRejectsBlankQuery(t *testing.T) {
	kbRepo := &ragServiceKBRepo{bases: map[string]*entity.KnowledgeBase{
		"kb-1": {KBID: "kb-1", EmbeddingModelID: 7, ChatModelID: 8},
	}}
	svc := NewRAGService(config.RAGConfig{VectorTopK: 5}, kbRepo, &ragServiceRetriever{}, nil, nil)

	for _, query := range []string{"", "   ", "\n\t"} {
		if _, err := svc.Retrieve(context.Background(), []string{"kb-1"}, query, 4); err == nil {
			t.Fatalf("Retrieve(%q) error = nil, want invalid request", query)
		} else {
			assertRAGInvalidRequest(t, err)
		}
	}
}

func TestRAGServiceRetrieveRejectsEmptyKBIDs(t *testing.T) {
	kbRepo := &ragServiceKBRepo{bases: map[string]*entity.KnowledgeBase{
		"kb-1": {KBID: "kb-1", EmbeddingModelID: 7, ChatModelID: 8},
	}}
	svc := NewRAGService(config.RAGConfig{VectorTopK: 5}, kbRepo, &ragServiceRetriever{}, nil, nil)

	for _, kbIDs := range [][]string{nil, {}} {
		if _, err := svc.Retrieve(context.Background(), kbIDs, "refund", 4); err == nil {
			t.Fatalf("Retrieve(kbIDs=%v) error = nil, want invalid request", kbIDs)
		} else {
			assertRAGInvalidRequest(t, err)
		}
	}
}

func TestRAGServiceRetrieveRejectsTooManyKBIDs(t *testing.T) {
	kbRepo := &ragServiceKBRepo{bases: map[string]*entity.KnowledgeBase{
		"kb-1": {KBID: "kb-1", EmbeddingModelID: 7, ChatModelID: 8},
	}}
	svc := NewRAGService(config.RAGConfig{VectorTopK: 5}, kbRepo, &ragServiceRetriever{}, nil, nil)

	kbIDs := make([]string, 11)
	for i := range kbIDs {
		kbIDs[i] = "kb-1"
	}
	if _, err := svc.Retrieve(context.Background(), kbIDs, "refund", 4); err == nil {
		t.Fatal("Retrieve(11 kbIDs) error = nil, want invalid request")
	} else {
		assertRAGInvalidRequest(t, err)
	}
}

func TestRAGServiceRetrieveRejectsExcessiveTopK(t *testing.T) {
	kbRepo := &ragServiceKBRepo{bases: map[string]*entity.KnowledgeBase{
		"kb-1": {KBID: "kb-1", EmbeddingModelID: 7, ChatModelID: 8},
	}}
	svc := NewRAGService(config.RAGConfig{VectorTopK: 5}, kbRepo, &ragServiceRetriever{}, nil, nil)

	for _, topK := range []int{-1, 21, 100} {
		if _, err := svc.Retrieve(context.Background(), []string{"kb-1"}, "refund", topK); err == nil {
			t.Fatalf("Retrieve(topK=%d) error = nil, want invalid request", topK)
		} else {
			assertRAGInvalidRequest(t, err)
		}
	}
}

func TestRAGServiceRetrieveUsesConfigDefaultTopK(t *testing.T) {
	kbRepo := &ragServiceKBRepo{bases: map[string]*entity.KnowledgeBase{
		"kb-1": {KBID: "kb-1", EmbeddingModelID: 7, ChatModelID: 8},
	}}
	ret := &ragServiceRetriever{}
	svc := NewRAGService(config.RAGConfig{VectorTopK: 9, RerankTopK: 3}, kbRepo, ret, nil, nil)

	result, err := svc.Retrieve(context.Background(), []string{"kb-1"}, "refund", 0)
	if err != nil {
		t.Fatalf("Retrieve() error = %v", err)
	}
	if ret.topK != 3 {
		t.Fatalf("retriever topK = %d, want rerank default 3", ret.topK)
	}
	if len(result.Chunks) != 0 {
		t.Fatalf("len(chunks) = %d, want 0", len(result.Chunks))
	}
	if result.Chunks == nil {
		t.Fatal("chunks = nil, want non-nil empty slice")
	}
}

func TestRAGServiceRetrieveReturnsNonNilEmptyChunks(t *testing.T) {
	kbRepo := &ragServiceKBRepo{bases: map[string]*entity.KnowledgeBase{
		"kb-1": {KBID: "kb-1", EmbeddingModelID: 7, ChatModelID: 8},
	}}
	svc := NewRAGService(config.RAGConfig{VectorTopK: 5}, kbRepo, &ragServiceRetriever{}, nil, nil)

	result, err := svc.Retrieve(context.Background(), []string{"kb-1"}, "refund", 4)
	if err != nil {
		t.Fatalf("Retrieve() error = %v", err)
	}
	if result.Chunks == nil {
		t.Fatal("chunks = nil, want non-nil empty slice")
	}
	if len(result.Chunks) != 0 {
		t.Fatalf("len(chunks) = %d, want 0", len(result.Chunks))
	}
}

func TestRAGServiceRetrieveRejectsBlankKBID(t *testing.T) {
	kbRepo := &ragServiceKBRepo{bases: map[string]*entity.KnowledgeBase{
		"kb-1": {KBID: "kb-1", EmbeddingModelID: 7, ChatModelID: 8},
	}}
	svc := NewRAGService(config.RAGConfig{VectorTopK: 5}, kbRepo, &ragServiceRetriever{}, nil, nil)

	if _, err := svc.Retrieve(context.Background(), []string{"", "kb-1"}, "refund", 4); err == nil {
		t.Fatal("Retrieve(blank kb id) error = nil, want invalid request")
	} else {
		assertRAGInvalidRequest(t, err)
	}
}

func TestRAGServiceRetrieveTrimsQuery(t *testing.T) {
	kbRepo := &ragServiceKBRepo{bases: map[string]*entity.KnowledgeBase{
		"kb-1": {KBID: "kb-1", EmbeddingModelID: 7, ChatModelID: 8},
	}}
	ret := &ragServiceRetriever{}
	svc := NewRAGService(config.RAGConfig{VectorTopK: 5}, kbRepo, ret, nil, nil)

	result, err := svc.Retrieve(context.Background(), []string{"kb-1"}, "  refund  ", 4)
	if err != nil {
		t.Fatalf("Retrieve() error = %v", err)
	}
	if result.QueryText != strings.TrimSpace("  refund  ") {
		t.Fatalf("query = %q, want trimmed refund", result.QueryText)
	}
}

// TestRAGServiceRetrieveToleratesPartialMissingKBs 验证：只要至少有一个知识库存在，
// 即使用户传入的 kb_id 列表中包含不存在的项，也不应报错（不存在项被静默跳过）。
func TestRAGServiceRetrieveToleratesPartialMissingKBs(t *testing.T) {
	kbRepo := &ragServiceKBRepo{bases: map[string]*entity.KnowledgeBase{
		"kb-1": {KBID: "kb-1", EmbeddingModelID: 7, ChatModelID: 8},
	}}
	svc := NewRAGService(config.RAGConfig{VectorTopK: 5}, kbRepo, &ragServiceRetriever{}, nil, nil)

	// 仅 "kb-1" 存在，"missing" 不存在，整体应成功而非报 70002。
	if _, err := svc.Retrieve(context.Background(), []string{"kb-1", "missing"}, "refund", 4); err != nil {
		t.Fatalf("Retrieve() with partial missing kb_ids error = %v, want nil", err)
	}
}

// TestRAGServiceRetrieveRejectsAllMissingKBs 验证：全部知识库都不存在时仍报 70002。
func TestRAGServiceRetrieveRejectsAllMissingKBs(t *testing.T) {
	kbRepo := &ragServiceKBRepo{bases: map[string]*entity.KnowledgeBase{
		"kb-1": {KBID: "kb-1", EmbeddingModelID: 7, ChatModelID: 8},
	}}
	svc := NewRAGService(config.RAGConfig{VectorTopK: 5}, kbRepo, &ragServiceRetriever{}, nil, nil)

	_, err := svc.Retrieve(context.Background(), []string{"ghost-1", "ghost-2"}, "refund", 4)
	if err == nil {
		t.Fatal("Retrieve() error = nil, want CodeRAGKBNotFound")
	}
	var biz *apperrors.BizError
	if !errors.As(err, &biz) {
		t.Fatalf("error = %v, want BizError", err)
	}
	if biz.Code != CodeRAGKBNotFound {
		t.Fatalf("error code = %d, want %d (message: %s)", biz.Code, CodeRAGKBNotFound, biz.Message)
	}
}

func TestRAGServiceRetrieveTestDefaultsToRerankTopK(t *testing.T) {
	kbRepo := &ragServiceKBRepo{bases: map[string]*entity.KnowledgeBase{
		"kb-1": {KBID: "kb-1", EmbeddingModelID: 7, ChatModelID: 8},
	}}
	ret := &ragServiceRetriever{}
	svc := NewRAGService(config.RAGConfig{RerankTopK: 3}, kbRepo, ret, nil, nil)

	result, err := svc.RetrieveTest(context.Background(), []string{"kb-1"}, "refund", nil)
	if err != nil {
		t.Fatalf("RetrieveTest() error = %v", err)
	}
	if ret.topK != 3 {
		t.Fatalf("retriever topK = %d, want rerank default 3", ret.topK)
	}
	if result.Chunks == nil {
		t.Fatal("chunks = nil, want non-nil empty slice")
	}
}

func TestRAGServiceRetrieveTestUsesExplicitTopK(t *testing.T) {
	kbRepo := &ragServiceKBRepo{bases: map[string]*entity.KnowledgeBase{
		"kb-1": {KBID: "kb-1", EmbeddingModelID: 7, ChatModelID: 8},
	}}
	ret := &ragServiceRetriever{}
	svc := NewRAGService(config.RAGConfig{RerankTopK: 3}, kbRepo, ret, nil, nil)

	topK := 8
	result, err := svc.RetrieveTest(context.Background(), []string{"kb-1"}, "refund", &RetrievalTestConfig{TopK: &topK})
	if err != nil {
		t.Fatalf("RetrieveTest() error = %v", err)
	}
	if ret.topK != 8 {
		t.Fatalf("retriever topK = %d, want 8", ret.topK)
	}
	if result.QueryText != "refund" {
		t.Fatalf("query = %q, want refund", result.QueryText)
	}
}

func TestRAGServiceRetrieveTestRejectsOutOfRangeTopK(t *testing.T) {
	kbRepo := &ragServiceKBRepo{bases: map[string]*entity.KnowledgeBase{
		"kb-1": {KBID: "kb-1", EmbeddingModelID: 7, ChatModelID: 8},
	}}
	svc := NewRAGService(config.RAGConfig{RerankTopK: 3}, kbRepo, &ragServiceRetriever{}, nil, nil)

	topK := 100
	_, err := svc.RetrieveTest(context.Background(), []string{"kb-1"}, "refund", &RetrievalTestConfig{TopK: &topK})
	if err == nil {
		t.Fatal("RetrieveTest(topK=100) error = nil, want invalid request")
	} else {
		assertRAGInvalidRequest(t, err)
	}
}

func TestRAGServiceRetrieveTestNotConfigured(t *testing.T) {
	kbRepo := &ragServiceKBRepo{bases: map[string]*entity.KnowledgeBase{
		"kb-1": {KBID: "kb-1", EmbeddingModelID: 7, ChatModelID: 8},
	}}
	svc := NewRAGService(config.RAGConfig{}, kbRepo, nil, nil, nil)

	_, err := svc.RetrieveTest(context.Background(), []string{"kb-1"}, "refund", nil)
	if err == nil {
		t.Fatal("RetrieveTest() error = nil, want CodeRAGNotConfigured")
	}
	var biz *apperrors.BizError
	if !errors.As(err, &biz) || biz.Code != CodeRAGNotConfigured {
		t.Fatalf("error = %v, want CodeRAGNotConfigured", err)
	}
}

func TestRAGServiceRetrieveTestRejectsAllMissingKBs(t *testing.T) {
	kbRepo := &ragServiceKBRepo{bases: map[string]*entity.KnowledgeBase{
		"kb-1": {KBID: "kb-1", EmbeddingModelID: 7, ChatModelID: 8},
	}}
	svc := NewRAGService(config.RAGConfig{RerankTopK: 3}, kbRepo, &ragServiceRetriever{}, nil, nil)

	_, err := svc.RetrieveTest(context.Background(), []string{"ghost"}, "refund", nil)
	if err == nil {
		t.Fatal("RetrieveTest() error = nil, want CodeRAGKBNotFound")
	}
	var biz *apperrors.BizError
	if !errors.As(err, &biz) || biz.Code != CodeRAGKBNotFound {
		t.Fatalf("error = %v, want CodeRAGKBNotFound", err)
	}
}

// ragServiceTraceWriter 捕获手动埋点写入的 Span，用于验证检索追踪。
type ragServiceTraceWriter struct {
	started []*entity.TraceSpan
	ends    []trace.SpanEnd
}

func (w *ragServiceTraceWriter) CreateTrace(context.Context, *entity.TraceRecord) error { return nil }
func (w *ragServiceTraceWriter) UpdateTraceMetadata(context.Context, string, trace.TraceMetadata) error {
	return nil
}
func (w *ragServiceTraceWriter) StartSpan(_ context.Context, span *entity.TraceSpan) error {
	w.started = append(w.started, span)
	return nil
}
func (w *ragServiceTraceWriter) EndSpan(_ context.Context, _ string, end trace.SpanEnd) error {
	w.ends = append(w.ends, end)
	return nil
}

func TestRAGServiceRetrieveTraceSpan(t *testing.T) {
	kbRepo := &ragServiceKBRepo{bases: map[string]*entity.KnowledgeBase{
		"kb-1": {KBID: "kb-1", EmbeddingModelID: 7, ChatModelID: 8},
	}}
	ret := &ragServiceRetriever{docs: []*schema.Document{{
		Content: "doc",
		MetaData: map[string]any{
			"kb_id": "kb-1", "chunk_id": "c1", "file_id": "f1", "filename": "a.md", "score": 0.9,
		},
	}}}
	writer := &ragServiceTraceWriter{}
	tracer := trace.NewTracer(writer, trace.Config{Enabled: true})
	svc := NewRAGService(config.RAGConfig{VectorTopK: 5}, kbRepo, ret, nil, tracer)
	ctx := trace.WithSpanContext(context.Background(), trace.SpanContext{TraceID: "t-ret", SpanID: "tool-span", Sampled: true})

	if _, err := svc.Retrieve(ctx, []string{"kb-1"}, "query", 3); err != nil {
		t.Fatalf("Retrieve() error = %v", err)
	}
	if len(writer.started) != 1 {
		t.Fatalf("started spans = %d, want 1", len(writer.started))
	}
	sp := writer.started[0]
	if sp.Kind != "retriever" || sp.Operation != "retriever.search" {
		t.Fatalf("kind/operation = %s/%s", sp.Kind, sp.Operation)
	}
	if sp.ParentSpanID != "tool-span" {
		t.Fatalf("parent = %q, want tool-span", sp.ParentSpanID)
	}
	if topK, _ := sp.Attributes["retriever.top_k"].(int); topK != 3 {
		t.Fatalf("top_k = %v, want 3", sp.Attributes["retriever.top_k"])
	}
	if len(writer.ends) != 1 || writer.ends[0].Status != trace.SpanStatusOK {
		t.Fatalf("ends = %+v", writer.ends)
	}
	hitCount, _ := writer.ends[0].Attributes["retriever.hit_count"].(int)
	if hitCount != 1 {
		t.Fatalf("hit_count = %v, want 1", writer.ends[0].Attributes["retriever.hit_count"])
	}
}

func TestRAGServiceRetrieveNoTraceWithoutSpanContext(t *testing.T) {
	kbRepo := &ragServiceKBRepo{bases: map[string]*entity.KnowledgeBase{
		"kb-1": {KBID: "kb-1", EmbeddingModelID: 7, ChatModelID: 8},
	}}
	writer := &ragServiceTraceWriter{}
	tracer := trace.NewTracer(writer, trace.Config{Enabled: true})
	svc := NewRAGService(config.RAGConfig{VectorTopK: 5}, kbRepo, &ragServiceRetriever{}, nil, tracer)
	if _, err := svc.Retrieve(context.Background(), []string{"kb-1"}, "query", 3); err != nil {
		t.Fatalf("Retrieve() error = %v", err)
	}
	if len(writer.started) != 0 {
		t.Fatalf("started spans = %d, want 0（无 SpanContext 不追踪）", len(writer.started))
	}
}
