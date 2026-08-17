package rag

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"Qavor/internal/model/entity"
	"Qavor/internal/trace"

	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
)

type fakeRankedVectorRetriever struct {
	retrieve func(context.Context, string, ...retriever.Option) ([][]*schema.Document, error)
}

func (r *fakeRankedVectorRetriever) RetrieveRanked(ctx context.Context, query string, opts ...retriever.Option) ([][]*schema.Document, error) {
	return r.retrieve(ctx, query, opts...)
}

type fakeKeywordRetriever struct {
	retrieve func(context.Context, string, ...retriever.Option) ([]*schema.Document, error)
}

func (r *fakeKeywordRetriever) Retrieve(ctx context.Context, query string, opts ...retriever.Option) ([]*schema.Document, error) {
	return r.retrieve(ctx, query, opts...)
}

type fakeRerankSettingsReader struct {
	id    uint
	found bool
	err   error
}

func (r *fakeRerankSettingsReader) RerankModelID(context.Context) (uint, bool, error) {
	return r.id, r.found, r.err
}

type fakeRerankerResolver struct {
	reranker Reranker
	err      error
	modelID  uint
}

func (r *fakeRerankerResolver) ResolveReranker(_ context.Context, modelID uint) (Reranker, error) {
	r.modelID = modelID
	return r.reranker, r.err
}

type fakeReranker struct {
	results []RerankResult
	err     error
	query   string
	topN    int
	docs    []RerankDocument
}

func (r *fakeReranker) Rerank(_ context.Context, query string, docs []RerankDocument, topN int) ([]RerankResult, error) {
	r.query = query
	r.topN = topN
	r.docs = append([]RerankDocument(nil), docs...)
	return r.results, r.err
}

func TestHybridRetriever_StartsBothBranchesAndFusesResults(t *testing.T) {
	vectorStarted := make(chan struct{})
	keywordStarted := make(chan struct{})
	vector := &fakeRankedVectorRetriever{retrieve: func(ctx context.Context, _ string, opts ...retriever.Option) ([][]*schema.Document, error) {
		close(vectorStarted)
		select {
		case <-keywordStarted:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		common := retriever.GetCommonOptions(&retriever.Options{}, opts...)
		if common.TopK == nil || *common.TopK != 20 || common.ScoreThreshold == nil || *common.ScoreThreshold != 0.4 {
			t.Errorf("向量选项=%+v", common)
		}
		return [][]*schema.Document{{
			rrfDocument("vector-only", "向量", "vector", 0.9),
			rrfDocument("dual", "双路", "vector", 0.7),
		}}, nil
	}}
	keyword := &fakeKeywordRetriever{retrieve: func(ctx context.Context, _ string, opts ...retriever.Option) ([]*schema.Document, error) {
		close(keywordStarted)
		select {
		case <-vectorStarted:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		common := retriever.GetCommonOptions(&retriever.Options{}, opts...)
		if common.TopK == nil || *common.TopK != 20 || common.ScoreThreshold != nil {
			t.Errorf("关键词选项=%+v", common)
		}
		return []*schema.Document{
			rrfDocument("keyword-only", "关键词", "keyword", 0.8),
			rrfDocument("dual", "双路", "keyword", 0.6),
		}, nil
	}}
	hybrid := NewHybridRetriever(vector, keyword, nil, HybridConfig{
		VectorTopK: 20, KeywordTopK: 20, FusedTopK: 20, RerankTopK: 5, RRFK: 60,
	})
	threshold := 0.4
	docs, err := hybrid.Retrieve(context.Background(), "退款流程",
		WithKnowledgeBaseIDs([]string{"kb-1"}), retriever.WithScoreThreshold(threshold))
	if err != nil {
		t.Fatalf("混合检索: %v", err)
	}
	if len(docs) != 3 || docs[0].ID != "dual" {
		t.Fatalf("融合结果=%v", rankedDocumentIDs(docs))
	}
}

func TestHybridRetriever_DegradesSingleBranchAndFailsWhenBothFail(t *testing.T) {
	branchError := errors.New("分支失败")
	successVector := func(context.Context, string, ...retriever.Option) ([][]*schema.Document, error) {
		return [][]*schema.Document{{rrfDocument("vector", "向量", "vector", 0.8)}}, nil
	}
	successKeyword := func(context.Context, string, ...retriever.Option) ([]*schema.Document, error) {
		return []*schema.Document{rrfDocument("keyword", "关键词", "keyword", 0.7)}, nil
	}
	tests := []struct {
		name        string
		vector      func(context.Context, string, ...retriever.Option) ([][]*schema.Document, error)
		keyword     func(context.Context, string, ...retriever.Option) ([]*schema.Document, error)
		wantID      string
		wantFailure bool
	}{
		{name: "向量失败", vector: func(context.Context, string, ...retriever.Option) ([][]*schema.Document, error) {
			return nil, branchError
		}, keyword: successKeyword, wantID: "keyword"},
		{name: "关键词失败", vector: successVector, keyword: func(context.Context, string, ...retriever.Option) ([]*schema.Document, error) {
			return nil, branchError
		}, wantID: "vector"},
		{name: "两路失败", vector: func(context.Context, string, ...retriever.Option) ([][]*schema.Document, error) {
			return nil, branchError
		}, keyword: func(context.Context, string, ...retriever.Option) ([]*schema.Document, error) {
			return nil, branchError
		}, wantFailure: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hybrid := NewHybridRetriever(&fakeRankedVectorRetriever{retrieve: tt.vector}, &fakeKeywordRetriever{retrieve: tt.keyword}, nil, HybridConfig{FusedTopK: 20, RerankTopK: 5, RRFK: 60})
			docs, err := hybrid.Retrieve(context.Background(), "query", WithKnowledgeBaseIDs([]string{"kb-1"}))
			if tt.wantFailure {
				if !errors.Is(err, ErrRetrievalUnavailable) {
					t.Fatalf("错误=%v", err)
				}
				return
			}
			if err != nil || len(docs) != 1 || docs[0].ID != tt.wantID {
				t.Fatalf("docs=%v err=%v", rankedDocumentIDs(docs), err)
			}
		})
	}
}

func TestHybridRetriever_PropagatesCancellation(t *testing.T) {
	blockedVector := &fakeRankedVectorRetriever{retrieve: func(ctx context.Context, _ string, _ ...retriever.Option) ([][]*schema.Document, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}}
	blockedKeyword := &fakeKeywordRetriever{retrieve: func(ctx context.Context, _ string, _ ...retriever.Option) ([]*schema.Document, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	_, err := NewHybridRetriever(blockedVector, blockedKeyword, nil, HybridConfig{}).Retrieve(ctx, "query", WithKnowledgeBaseIDs([]string{"kb-1"}))
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("错误=%v", err)
	}
}

func TestHybridRetriever_RerankAbsenceFailureAndSuccess(t *testing.T) {
	vector := &fakeRankedVectorRetriever{retrieve: func(context.Context, string, ...retriever.Option) ([][]*schema.Document, error) {
		return [][]*schema.Document{{
			rrfDocument("first", "第一", "vector", 0.9),
			rrfDocument("second", "第二", "vector", 0.8),
			rrfDocument("third", "第三", "vector", 0.7),
		}}, nil
	}}
	keyword := &fakeKeywordRetriever{retrieve: func(context.Context, string, ...retriever.Option) ([]*schema.Document, error) { return nil, nil }}
	cfg := HybridConfig{VectorTopK: 20, KeywordTopK: 20, FusedTopK: 20, RerankTopK: 2, RRFK: 60}

	without, err := NewHybridRetriever(vector, keyword, nil, cfg).Retrieve(context.Background(), "query", WithKnowledgeBaseIDs([]string{"kb-1"}))
	if err != nil || !reflect.DeepEqual(rankedDocumentIDs(without), []string{"first", "second"}) {
		t.Fatalf("未配置重排 docs=%v err=%v", rankedDocumentIDs(without), err)
	}

	failingModel := &fakeReranker{err: errors.New("服务不可用")}
	failingDynamic := NewDynamicReranker(&fakeRerankSettingsReader{id: 7, found: true}, &fakeRerankerResolver{reranker: failingModel}, nil)
	degraded, err := NewHybridRetriever(vector, keyword, failingDynamic, cfg).Retrieve(context.Background(), "query", WithKnowledgeBaseIDs([]string{"kb-1"}))
	if err != nil || !reflect.DeepEqual(rankedDocumentIDs(degraded), []string{"first", "second"}) {
		t.Fatalf("重排失败降级 docs=%v err=%v", rankedDocumentIDs(degraded), err)
	}

	model := &fakeReranker{results: []RerankResult{{Index: 1, Score: 0.95}}}
	resolver := &fakeRerankerResolver{reranker: model}
	dynamic := NewDynamicReranker(&fakeRerankSettingsReader{id: 7, found: true}, resolver, nil)
	reranked, err := NewHybridRetriever(vector, keyword, dynamic, cfg).Retrieve(context.Background(), "query", WithKnowledgeBaseIDs([]string{"kb-1"}))
	if err != nil {
		t.Fatalf("重排成功: %v", err)
	}
	if !reflect.DeepEqual(rankedDocumentIDs(reranked), []string{"second", "first"}) {
		t.Fatalf("重排顺序=%v", rankedDocumentIDs(reranked))
	}
	if metaDataFloat64(reranked[0], MetaKeyRerankScore, 0) != 0.95 || metaDataFloat64(reranked[0], MetaKeyScore, 0) != 0.95 {
		t.Fatalf("重排分数=%v", reranked[0].MetaData)
	}
	if resolver.modelID != 7 || model.query != "query" || model.topN != 2 || len(model.docs) != 3 {
		t.Fatalf("resolverID=%d query=%q topN=%d docs=%d", resolver.modelID, model.query, model.topN, len(model.docs))
	}
}

func TestHybridRetriever_StageOptionsOverridePerRequest(t *testing.T) {
	vector := &fakeRankedVectorRetriever{retrieve: func(_ context.Context, _ string, opts ...retriever.Option) ([][]*schema.Document, error) {
		common := retriever.GetCommonOptions(&retriever.Options{}, opts...)
		if common.TopK == nil || *common.TopK != 12 {
			t.Errorf("向量 TopK=%v, want 12", common.TopK)
		}
		return [][]*schema.Document{{rrfDocument("vector", "向量", "vector", 0.8)}}, nil
	}}
	keyword := &fakeKeywordRetriever{retrieve: func(_ context.Context, _ string, opts ...retriever.Option) ([]*schema.Document, error) {
		common := retriever.GetCommonOptions(&retriever.Options{}, opts...)
		if common.TopK == nil || *common.TopK != 7 {
			t.Errorf("关键词 TopK=%v, want 7", common.TopK)
		}
		return []*schema.Document{rrfDocument("keyword", "关键词", "keyword", 0.7)}, nil
	}}
	hybrid := NewHybridRetriever(vector, keyword, nil, HybridConfig{
		VectorTopK: 20, KeywordTopK: 20, FusedTopK: 20, RerankTopK: 5, RRFK: 60,
	})
	vectorTopK, keywordTopK := 12, 7
	docs, err := hybrid.Retrieve(context.Background(), "query",
		WithKnowledgeBaseIDs([]string{"kb-1"}),
		WithHybridStageOptions(&vectorTopK, &keywordTopK, nil, nil))
	if err != nil {
		t.Fatalf("混合检索: %v", err)
	}
	if len(docs) != 2 {
		t.Fatalf("结果数=%d, want 2 (%v)", len(docs), rankedDocumentIDs(docs))
	}
}

func TestHybridRetriever_StageOptionsIgnoreInvalidValues(t *testing.T) {
	vector := &fakeRankedVectorRetriever{retrieve: func(_ context.Context, _ string, opts ...retriever.Option) ([][]*schema.Document, error) {
		common := retriever.GetCommonOptions(&retriever.Options{}, opts...)
		if common.TopK == nil || *common.TopK != 20 {
			t.Errorf("向量 TopK=%v, want 默认 20", common.TopK)
		}
		return [][]*schema.Document{{rrfDocument("vector", "向量", "vector", 0.8)}}, nil
	}}
	keyword := &fakeKeywordRetriever{retrieve: func(_ context.Context, _ string, opts ...retriever.Option) ([]*schema.Document, error) {
		common := retriever.GetCommonOptions(&retriever.Options{}, opts...)
		if common.TopK == nil || *common.TopK != 20 {
			t.Errorf("关键词 TopK=%v, want 默认 20", common.TopK)
		}
		return []*schema.Document{rrfDocument("keyword", "关键词", "keyword", 0.7)}, nil
	}}
	hybrid := NewHybridRetriever(vector, keyword, nil, HybridConfig{
		VectorTopK: 20, KeywordTopK: 20, FusedTopK: 20, RerankTopK: 5, RRFK: 60,
	})
	zero, negative := 0, -1
	if _, err := hybrid.Retrieve(context.Background(), "query",
		WithKnowledgeBaseIDs([]string{"kb-1"}),
		WithHybridStageOptions(&zero, &negative, nil, nil)); err != nil {
		t.Fatalf("混合检索: %v", err)
	}
}

func TestHybridRetriever_StageOptionsCapFusedResults(t *testing.T) {
	vector := &fakeRankedVectorRetriever{retrieve: func(_ context.Context, _ string, _ ...retriever.Option) ([][]*schema.Document, error) {
		return [][]*schema.Document{{
			rrfDocument("a", "a", "vector", 0.9),
			rrfDocument("b", "b", "vector", 0.8),
			rrfDocument("c", "c", "vector", 0.7),
		}}, nil
	}}
	keyword := &fakeKeywordRetriever{retrieve: func(_ context.Context, _ string, _ ...retriever.Option) ([]*schema.Document, error) {
		return nil, nil
	}}
	hybrid := NewHybridRetriever(vector, keyword, nil, HybridConfig{
		VectorTopK: 20, KeywordTopK: 20, FusedTopK: 20, RerankTopK: 5, RRFK: 60,
	})
	fusedTopK := 2
	docs, err := hybrid.Retrieve(context.Background(), "query",
		WithKnowledgeBaseIDs([]string{"kb-1"}),
		WithHybridStageOptions(nil, nil, &fusedTopK, nil))
	if err != nil {
		t.Fatalf("混合检索: %v", err)
	}
	if len(docs) != 2 {
		t.Fatalf("融合窗口=%d, want 2", len(docs))
	}
}

// rerankTraceWriter 捕获手动埋点写入的 Span，用于验证重排追踪。
type rerankTraceWriter struct {
	started []*entity.TraceSpan
	ends    []trace.SpanEnd
}

func (w *rerankTraceWriter) CreateTrace(context.Context, *entity.TraceRecord) error { return nil }
func (w *rerankTraceWriter) UpdateTraceMetadata(context.Context, string, trace.TraceMetadata) error {
	return nil
}
func (w *rerankTraceWriter) StartSpan(_ context.Context, span *entity.TraceSpan) error {
	w.started = append(w.started, span)
	return nil
}
func (w *rerankTraceWriter) EndSpan(_ context.Context, _ string, end trace.SpanEnd) error {
	w.ends = append(w.ends, end)
	return nil
}

func TestDynamicReranker_TraceSpan(t *testing.T) {
	docs := []*schema.Document{{ID: "d0", Content: "c0"}, {ID: "d1", Content: "c1"}, {ID: "d2", Content: "c2"}}
	model := &fakeReranker{results: []RerankResult{{Index: 1, Score: 0.95}}}
	writer := &rerankTraceWriter{}
	tracer := trace.NewTracer(writer, trace.Config{Enabled: true})
	dynamic := NewDynamicReranker(&fakeRerankSettingsReader{id: 7, found: true}, &fakeRerankerResolver{reranker: model}, tracer)
	ctx := trace.WithSpanContext(context.Background(), trace.SpanContext{TraceID: "t-rerank", SpanID: "retriever-span", Sampled: true})

	_, applied, err := dynamic.Rerank(ctx, "query", docs, 2)
	if err != nil || !applied {
		t.Fatalf("rerank err=%v applied=%v", err, applied)
	}
	if len(writer.started) != 1 {
		t.Fatalf("started spans = %d, want 1", len(writer.started))
	}
	sp := writer.started[0]
	if sp.Kind != "rerank" || sp.Operation != "reranker.rerank" {
		t.Fatalf("kind/operation = %s/%s", sp.Kind, sp.Operation)
	}
	if sp.ParentSpanID != "retriever-span" {
		t.Fatalf("parent = %q", sp.ParentSpanID)
	}
	if topN, _ := sp.Attributes["rerank.top_n"].(int); topN != 2 {
		t.Fatalf("top_n = %v", sp.Attributes["rerank.top_n"])
	}
	if len(writer.ends) != 1 || writer.ends[0].Status != trace.SpanStatusOK {
		t.Fatalf("ends = %+v", writer.ends)
	}
}

func TestDynamicReranker_TraceSpanError(t *testing.T) {
	docs := []*schema.Document{{ID: "d0", Content: "c0"}}
	writer := &rerankTraceWriter{}
	tracer := trace.NewTracer(writer, trace.Config{Enabled: true})
	dynamic := NewDynamicReranker(
		&fakeRerankSettingsReader{id: 7, found: true},
		&fakeRerankerResolver{reranker: &fakeReranker{err: errors.New("服务不可用")}},
		tracer,
	)
	ctx := trace.WithSpanContext(context.Background(), trace.SpanContext{TraceID: "t-rerank-err", SpanID: "retriever-span", Sampled: true})
	if _, _, err := dynamic.Rerank(ctx, "query", docs, 1); err == nil {
		t.Fatal("expected rerank error")
	}
	if len(writer.ends) != 1 || writer.ends[0].Status != trace.SpanStatusError {
		t.Fatalf("ends = %+v", writer.ends)
	}
}

func TestDynamicReranker_NoTraceWithoutSpanContext(t *testing.T) {
	docs := []*schema.Document{{ID: "d0", Content: "c0"}}
	writer := &rerankTraceWriter{}
	tracer := trace.NewTracer(writer, trace.Config{Enabled: true})
	dynamic := NewDynamicReranker(
		&fakeRerankSettingsReader{id: 7, found: true},
		&fakeRerankerResolver{reranker: &fakeReranker{results: []RerankResult{{Index: 0, Score: 0.5}}}},
		tracer,
	)
	if _, _, err := dynamic.Rerank(context.Background(), "query", docs, 1); err != nil {
		t.Fatalf("rerank err=%v", err)
	}
	if len(writer.started) != 0 {
		t.Fatalf("started spans = %d, want 0（无 SpanContext 不追踪）", len(writer.started))
	}
}
