package rag

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"

	"Qavor/pkg/logger"

	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

// RankedVectorRetriever 返回一个或多个彼此独立的向量排名列表。
type RankedVectorRetriever interface {
	RetrieveRanked(ctx context.Context, query string, opts ...retriever.Option) ([][]*schema.Document, error)
}

// RerankSettingsReader 读取当前全局重排模型设置。
type RerankSettingsReader interface {
	RerankModelID(ctx context.Context) (uint, bool, error)
}

// RerankerResolver 按模型 ID 解析重排客户端。
type RerankerResolver interface {
	ResolveReranker(ctx context.Context, modelID uint) (Reranker, error)
}

// DynamicReranker 在每次请求时解析当前全局重排模型。
type DynamicReranker struct {
	settings RerankSettingsReader
	resolver RerankerResolver
}

// NewDynamicReranker 创建动态重排器。
func NewDynamicReranker(settings RerankSettingsReader, resolver RerankerResolver) *DynamicReranker {
	return &DynamicReranker{settings: settings, resolver: resolver}
}

// Rerank 返回重排后的候选、是否实际执行了重排及错误。
func (r *DynamicReranker) Rerank(ctx context.Context, query string, documents []*schema.Document, topN int) ([]*schema.Document, bool, error) {
	if r == nil || r.settings == nil || r.resolver == nil {
		return documents, false, nil
	}
	modelID, found, err := r.settings.RerankModelID(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("读取 Rerank 设置: %w", err)
	}
	if !found {
		return documents, false, nil
	}
	model, err := r.resolver.ResolveReranker(ctx, modelID)
	if err != nil {
		return nil, false, fmt.Errorf("解析 Rerank 模型 %d: %w", modelID, err)
	}
	candidates := make([]RerankDocument, len(documents))
	for index, document := range documents {
		candidates[index] = RerankDocument{ID: metaDataString(document, MetaKeyChunkID), Content: document.Content}
	}
	results, err := model.Rerank(ctx, query, candidates, topN)
	if err != nil {
		return nil, false, fmt.Errorf("调用 Rerank 模型 %d: %w", modelID, err)
	}
	seen := make(map[int]struct{}, len(results))
	ordered := make([]*schema.Document, 0, len(documents))
	for _, result := range results {
		if result.Index < 0 || result.Index >= len(documents) || math.IsNaN(result.Score) || math.IsInf(result.Score, 0) {
			return nil, false, errors.New("Rerank 返回无效结果")
		}
		if _, duplicate := seen[result.Index]; duplicate {
			continue
		}
		seen[result.Index] = struct{}{}
		document := cloneDocument(documents[result.Index])
		document.MetaData[MetaKeyRerankScore] = result.Score
		document.MetaData[MetaKeyScore] = result.Score
		ordered = append(ordered, document)
	}
	if len(ordered) == 0 {
		return nil, false, errors.New("Rerank 返回空结果")
	}
	for index, document := range documents {
		if _, ranked := seen[index]; !ranked {
			ordered = append(ordered, document)
		}
	}
	return ordered, true, nil
}

// HybridConfig 配置各召回阶段的候选窗口与 RRF 参数。
type HybridConfig struct {
	VectorTopK  int
	KeywordTopK int
	FusedTopK   int
	RerankTopK  int
	RRFK        int
}

// hybridStageOptions 是 HybridRetriever 的实现特定选项，允许单次请求覆盖各阶段参数。
// 字段为 nil 时沿用构造时的 HybridConfig；>0 时覆盖对应阶段。
type hybridStageOptions struct {
	VectorTopK  *int
	KeywordTopK *int
	FusedTopK   *int
	RRFK        *int
}

// WithHybridStageOptions 设置本次检索各阶段的候选窗口与 RRF 参数。
// 仅在对应值 > 0 时生效，其余阶段沿用构造时的默认配置。
func WithHybridStageOptions(vectorTopK, keywordTopK, fusedTopK, rrfK *int) retriever.Option {
	return retriever.WrapImplSpecificOptFn(func(o *hybridStageOptions) {
		o.VectorTopK = vectorTopK
		o.KeywordTopK = keywordTopK
		o.FusedTopK = fusedTopK
		o.RRFK = rrfK
	})
}

// HybridRetriever 并发执行向量与关键词召回，再进行 RRF 和可选重排。
type HybridRetriever struct {
	vector   RankedVectorRetriever
	keyword  retriever.Retriever
	reranker *DynamicReranker
	config   HybridConfig
}

// NewHybridRetriever 创建共享混合检索器。
func NewHybridRetriever(vector RankedVectorRetriever, keyword retriever.Retriever, reranker *DynamicReranker, config HybridConfig) *HybridRetriever {
	if config.VectorTopK <= 0 {
		config.VectorTopK = 20
	}
	if config.KeywordTopK <= 0 {
		config.KeywordTopK = 20
	}
	if config.FusedTopK <= 0 {
		config.FusedTopK = 20
	}
	if config.RerankTopK <= 0 {
		config.RerankTopK = 5
	}
	if config.RRFK <= 0 {
		config.RRFK = 60
	}
	return &HybridRetriever{vector: vector, keyword: keyword, reranker: reranker, config: config}
}

type vectorRetrievalResult struct {
	lists [][]*schema.Document
	err   error
}

type keywordRetrievalResult struct {
	documents []*schema.Document
	err       error
}

// Retrieve 执行混合召回、融合、可选重排和最终截断。
func (r *HybridRetriever) Retrieve(ctx context.Context, query string, opts ...retriever.Option) ([]*schema.Document, error) {
	if r == nil || r.vector == nil || r.keyword == nil {
		return nil, ErrRetrievalUnavailable
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(query) == "" {
		return nil, errors.New("查询不能为空")
	}
	implementationOptions := retriever.GetImplSpecificOptions(&pgRetrieverOptions{}, opts...)
	if len(implementationOptions.KnowledgeBaseIDs) == 0 {
		return nil, errors.New("知识库 ID 不能为空")
	}
	// 单次请求可覆盖各阶段参数；nil/非正值忽略，沿用构造时的默认配置。
	config := r.config
	stageOptions := retriever.GetImplSpecificOptions(&hybridStageOptions{}, opts...)
	if stageOptions.VectorTopK != nil && *stageOptions.VectorTopK > 0 {
		config.VectorTopK = *stageOptions.VectorTopK
	}
	if stageOptions.KeywordTopK != nil && *stageOptions.KeywordTopK > 0 {
		config.KeywordTopK = *stageOptions.KeywordTopK
	}
	if stageOptions.FusedTopK != nil && *stageOptions.FusedTopK > 0 {
		config.FusedTopK = *stageOptions.FusedTopK
	}
	if stageOptions.RRFK != nil && *stageOptions.RRFK > 0 {
		config.RRFK = *stageOptions.RRFK
	}

	vectorOptions := append([]retriever.Option{}, opts...)
	vectorOptions = append(vectorOptions, retriever.WithTopK(config.VectorTopK))
	keywordOptions := []retriever.Option{
		WithKnowledgeBaseIDs(implementationOptions.KnowledgeBaseIDs),
		retriever.WithTopK(config.KeywordTopK),
	}
	vectorChannel := make(chan vectorRetrievalResult, 1)
	keywordChannel := make(chan keywordRetrievalResult, 1)
	var waitGroup sync.WaitGroup
	waitGroup.Add(2)
	go func() {
		defer waitGroup.Done()
		lists, err := r.vector.RetrieveRanked(ctx, query, vectorOptions...)
		vectorChannel <- vectorRetrievalResult{lists: lists, err: err}
	}()
	go func() {
		defer waitGroup.Done()
		documents, err := r.keyword.Retrieve(ctx, query, keywordOptions...)
		keywordChannel <- keywordRetrievalResult{documents: documents, err: err}
	}()
	waitGroup.Wait()
	vectorResult := <-vectorChannel
	keywordResult := <-keywordChannel
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if vectorResult.err != nil && keywordResult.err != nil {
		return nil, fmt.Errorf("%w: 向量分支: %v; 关键词分支: %v", ErrRetrievalUnavailable, vectorResult.err, keywordResult.err)
	}
	if logger.Initialized() {
		if vectorResult.err != nil {
			logger.Warn("向量检索失败，降级到关键词结果", zap.Error(vectorResult.err))
		}
		if keywordResult.err != nil {
			logger.Warn("关键词检索失败，降级到向量结果", zap.Error(keywordResult.err))
		}
	}

	lists := make([][]*schema.Document, 0, len(vectorResult.lists)+1)
	if vectorResult.err == nil {
		for _, list := range vectorResult.lists {
			lists = append(lists, markRetrievalBranch(list, "vector"))
		}
	}
	if keywordResult.err == nil && len(keywordResult.documents) > 0 {
		lists = append(lists, markRetrievalBranch(keywordResult.documents, "keyword"))
	}
	fused := FuseRRF(lists, config.RRFK, config.FusedTopK)
	finalTopK := config.RerankTopK
	commonOptions := retriever.GetCommonOptions(&retriever.Options{}, opts...)
	if commonOptions.TopK != nil && *commonOptions.TopK > 0 {
		finalTopK = *commonOptions.TopK
	}
	if finalTopK > config.FusedTopK {
		finalTopK = config.FusedTopK
	}
	if finalTopK > 20 {
		finalTopK = 20
	}
	if finalTopK <= 0 {
		finalTopK = config.RerankTopK
	}
	if len(fused) > 0 && r.reranker != nil {
		reranked, applied, err := r.reranker.Rerank(ctx, query, fused, finalTopK)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || ctx.Err() != nil {
				return nil, ctx.Err()
			}
			if logger.Initialized() {
				logger.Warn("Rerank 失败，降级到 RRF 结果", zap.Error(err))
			}
		} else if applied {
			fused = reranked
		}
	}
	if len(fused) > finalTopK {
		fused = fused[:finalTopK]
	}
	return fused, nil
}

func markRetrievalBranch(documents []*schema.Document, branch string) []*schema.Document {
	marked := make([]*schema.Document, 0, len(documents))
	for _, document := range documents {
		if document == nil {
			marked = append(marked, nil)
			continue
		}
		cloned := cloneDocument(document)
		cloned.MetaData[MetaKeyMatchedBy] = []string{branch}
		marked = append(marked, cloned)
	}
	return marked
}

var _ retriever.Retriever = (*HybridRetriever)(nil)
