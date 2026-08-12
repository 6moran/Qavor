package service

import (
	"context"
	"errors"
	"strings"

	"Qavor/internal/rag"
	"Qavor/internal/repository"
	"Qavor/pkg/config"
	apperrors "Qavor/pkg/errors"
	"Qavor/pkg/logger"

	einoretriever "github.com/cloudwego/eino/components/retriever"
	"go.uber.org/zap"
)

// 预定义 RAG 业务错误码（70xxx）。
const (
	CodeRAGNotConfigured   = 70001
	CodeRAGKBNotFound      = 70002
	CodeRAGEmbeddingFailed = 70003
	CodeRAGRetrievalFailed = 70004
	CodeRAGLLMFailed       = 70005
	CodeRAGInvalidRequest  = 70006
)

// RAGService RAG 问答服务实现。
type rAGService struct {
	cfg       config.RAGConfig
	kbRepo    repository.KnowledgeBaseRepository
	retriever einoretriever.Retriever
	answerer  rag.AnswerChain
}

// NewRAGService 创建 RAG 服务。retriever 或 answerer 缺失时对应能力返回 RAG_NOT_CONFIGURED。
func NewRAGService(cfg config.RAGConfig, kbRepo repository.KnowledgeBaseRepository, retriever einoretriever.Retriever, answerer rag.AnswerChain) RAGService {
	return &rAGService{cfg: cfg, kbRepo: kbRepo, retriever: retriever, answerer: answerer}
}

// validateRequest 校验请求参数，并返回实际可用的知识库 ID 列表及 kb_id→名称映射。
// 容错策略：只要至少有一个知识库存在就继续检索，仅当全部不存在时返回 CodeRAGKBNotFound；
// 不存在的 kb_id 会被静默剔除，调用方应使用返回的 validKBIDs 而非原始 kbIDs 继续检索。
func (s *rAGService) validateRequest(kbIDs []string, query string) (string, []string, map[string]string, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return "", nil, nil, apperrors.New(CodeRAGInvalidRequest, "query 不能为空")
	}
	if len(kbIDs) == 0 {
		return "", nil, nil, apperrors.New(CodeRAGInvalidRequest, "knowledge_base_ids 不能为空")
	}
	if len(kbIDs) > 10 {
		return "", nil, nil, apperrors.New(CodeRAGInvalidRequest, "knowledge_base_ids 最多 10 个")
	}
	if len(query) > 2000 {
		return "", nil, nil, apperrors.New(CodeRAGInvalidRequest, "query 过长")
	}

	// 校验每个知识库存在（一次批量查询，避免逐库往返）。
	for _, id := range kbIDs {
		if id == "" {
			return "", nil, nil, apperrors.New(CodeRAGInvalidRequest, "knowledge_base_ids 存在空值")
		}
	}
	bases, err := s.kbRepo.FindByKBIDs(kbIDs)
	if err != nil {
		return "", nil, nil, apperrors.New(CodeRAGRetrievalFailed, "校验知识库失败")
	}
	found := make(map[string]bool, len(bases))
	kbNames := make(map[string]string, len(bases))
	for _, base := range bases {
		found[base.KBID] = true
		kbNames[base.KBID] = base.Name
	}
	valid := make([]string, 0, len(kbIDs))
	for _, id := range kbIDs {
		if found[id] {
			valid = append(valid, id)
		}
	}
	// 至少一个知识库存在即可继续，全部不存在才报错。
	if len(valid) == 0 {
		return "", nil, nil, apperrors.New(CodeRAGKBNotFound, "知识库不存在")
	}
	return query, valid, kbNames, nil
}

func mapRAGRetrievalError(err error) error {
	switch {
	case errors.Is(err, rag.ErrEmbeddingNotConfigured):
		return apperrors.NewWithErr(CodeRAGNotConfigured, "RAG 未配置", err)
	case errors.Is(err, rag.ErrEmbeddingUnavailable):
		return apperrors.NewWithErr(CodeRAGEmbeddingFailed, "Embedding 服务不可用", err)
	case errors.Is(err, rag.ErrRetrievalUnavailable), errors.Is(err, rag.ErrEmbeddingModelMismatch):
		return apperrors.NewWithErr(CodeRAGRetrievalFailed, "向量检索失败", err)
	case errors.Is(err, rag.ErrChatModelMismatch):
		return apperrors.NewWithErr(CodeRAGLLMFailed, "知识库 Chat 模型不一致", err)
	default:
		return nil
	}
}

// Retrieve 执行检索但不调用 Chat Model
func (s *rAGService) Retrieve(ctx context.Context, kbIDs []string, query string, topK int) (*RAGRetrieveResult, error) {
	query, validKBIDs, kbNames, err := s.validateRequest(kbIDs, query)
	if err != nil {
		return nil, err
	}
	if s.retriever == nil {
		return nil, apperrors.New(CodeRAGNotConfigured, "RAG 未配置")
	}
	if topK < 0 || topK > 20 {
		return nil, apperrors.New(CodeRAGInvalidRequest, "top_k 必须在 1 到 20 之间")
	}
	if topK == 0 {
		topK = s.cfg.RerankTopK
		if topK <= 0 {
			topK = 5
		}
		if topK > 20 {
			topK = 20
		}
	}

	// 静默剔除不存在的知识库后，记录剩余不可用范围，便于排查配置漂移。
	if len(validKBIDs) < len(kbIDs) {
		dropped := make([]string, 0, len(kbIDs)-len(validKBIDs))
		validSet := make(map[string]bool, len(validKBIDs))
		for _, id := range validKBIDs {
			validSet[id] = true
		}
		for _, id := range kbIDs {
			if !validSet[id] {
				dropped = append(dropped, id)
			}
		}
		if logger.Initialized() {
			logger.Warn("部分知识库不存在，已跳过", zap.Strings("dropped_kb_ids", dropped))
		}
	}

	opts := []einoretriever.Option{
		rag.WithKnowledgeBaseIDs(validKBIDs),
		einoretriever.WithTopK(topK),
	}
	// 相似度阈值过滤：低于 ScoreThreshold 的噪声片段不进 prompt 上下文。
	if s.cfg.ScoreThreshold > 0 {
		opts = append(opts, einoretriever.WithScoreThreshold(s.cfg.ScoreThreshold))
	}
	return s.retrieveCore(ctx, query, validKBIDs, kbNames, opts...)
}

// RetrieveTest 执行检索测试：按单次请求的检索参数覆盖各阶段配置，不修改知识库配置。
// cfg 为 nil 时行为与 Retrieve 默认一致；cfg 中为 nil 的字段沿用系统默认。
func (s *rAGService) RetrieveTest(ctx context.Context, kbIDs []string, query string, cfg *RetrievalTestConfig) (*RAGRetrieveResult, error) {
	query, validKBIDs, kbNames, err := s.validateRequest(kbIDs, query)
	if err != nil {
		return nil, err
	}
	if s.retriever == nil {
		return nil, apperrors.New(CodeRAGNotConfigured, "RAG 未配置")
	}

	// 最终返回条数：显式 TopK > RerankTopK(测试覆盖/配置) > 5。
	topK := 0
	if cfg != nil && cfg.RerankTopK != nil && *cfg.RerankTopK > 0 {
		topK = *cfg.RerankTopK
	}
	if topK == 0 {
		topK = s.cfg.RerankTopK
	}
	if topK <= 0 {
		topK = 5
	}
	if cfg != nil && cfg.TopK != nil && *cfg.TopK > 0 {
		topK = *cfg.TopK
	}
	if topK < 1 || topK > 20 {
		return nil, apperrors.New(CodeRAGInvalidRequest, "top_k 必须在 1 到 20 之间")
	}

	opts := []einoretriever.Option{
		rag.WithKnowledgeBaseIDs(validKBIDs),
		einoretriever.WithTopK(topK),
	}
	if cfg != nil {
		// 覆盖各召回阶段候选窗口与 RRF 参数；nil 字段沿用默认配置。
		if cfg.VectorTopK != nil || cfg.KeywordTopK != nil || cfg.FusedTopK != nil || cfg.RRFK != nil {
			opts = append(opts, rag.WithHybridStageOptions(cfg.VectorTopK, cfg.KeywordTopK, cfg.FusedTopK, cfg.RRFK))
		}
		// 相似度阈值过滤：显式提供时覆盖系统默认。
		if cfg.ScoreThreshold != nil {
			opts = append(opts, einoretriever.WithScoreThreshold(*cfg.ScoreThreshold))
		}
	}
	return s.retrieveCore(ctx, query, validKBIDs, kbNames, opts...)
}

// retrieveCore 执行检索并转换为结构化分块结果。
func (s *rAGService) retrieveCore(ctx context.Context, query string, validKBIDs []string, kbNames map[string]string, opts ...einoretriever.Option) (*RAGRetrieveResult, error) {
	docs, err := s.retriever.Retrieve(ctx, query, opts...)
	if err != nil {
		if mapped := mapRAGRetrievalError(err); mapped != nil {
			return nil, mapped
		}
		if logger.Initialized() {
			logger.Warn("RAG 检索失败", zap.Error(err))
		}
		return nil, apperrors.NewWithErr(CodeRAGRetrievalFailed, "向量检索失败", err)
	}

	chunks := rag.BuildRetrievedChunks(docs)
	result := &RAGRetrieveResult{QueryText: query, Chunks: make([]RAGChunk, 0, len(chunks))}
	for _, chunk := range chunks {
		result.Chunks = append(result.Chunks, RAGChunk{
			KBID: chunk.KBID, KBName: kbNames[chunk.KBID], ChunkID: chunk.ChunkID,
			FileID: chunk.FileID, Filename: chunk.Filename, Content: chunk.Content,
			Score: chunk.Score, VectorScore: chunk.VectorScore, KeywordScore: chunk.KeywordScore,
			RRFScore: chunk.RRFScore, RerankScore: chunk.RerankScore, MatchedBy: chunk.MatchedBy,
		})
	}
	return result, nil
}

// Answer 执行问答
func (s *rAGService) Answer(ctx context.Context, kbIDs []string, query string) (*RAGAnswerResult, error) {
	query, validKBIDs, _, err := s.validateRequest(kbIDs, query)
	if err != nil {
		return nil, err
	}
	if s.answerer == nil {
		return nil, apperrors.New(CodeRAGNotConfigured, "RAG 未配置")
	}

	out, err := s.answerer.Answer(ctx, rag.AnswerInput{KnowledgeBaseIDs: validKBIDs, Query: query})
	if err != nil {
		if mapped := mapRAGRetrievalError(err); mapped != nil {
			return nil, mapped
		}
		switch {
		case errors.Is(err, rag.ErrLLMUnavailable), errors.Is(err, rag.ErrLLMNotConfigured):
			return nil, apperrors.New(CodeRAGLLMFailed, "LLM 调用失败")
		default:
			if logger.Initialized() {
				logger.Warn("RAG 问答失败", zap.Error(err))
			}
			return nil, apperrors.New(apperrors.CodeInternalError, "RAG 问答失败")
		}
	}

	result := &RAGAnswerResult{Answer: out.Answer}
	for _, c := range out.Citations {
		result.Citations = append(result.Citations, RAGCitation{
			Index:        c.Index,
			ChunkID:      c.ChunkID,
			FileID:       c.FileID,
			Filename:     c.Filename,
			Content:      c.Content,
			Score:        c.Score,
			VectorScore:  c.VectorScore,
			KeywordScore: c.KeywordScore,
			RRFScore:     c.RRFScore,
			RerankScore:  c.RerankScore,
			MatchedBy:    c.MatchedBy,
		})
	}
	return result, nil
}
