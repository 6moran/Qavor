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

func (s *rAGService) validateRequest(kbIDs []string, query string) (string, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return "", apperrors.New(CodeRAGInvalidRequest, "query 不能为空")
	}
	if len(kbIDs) == 0 {
		return "", apperrors.New(CodeRAGInvalidRequest, "knowledge_base_ids 不能为空")
	}
	if len(kbIDs) > 10 {
		return "", apperrors.New(CodeRAGInvalidRequest, "knowledge_base_ids 最多 10 个")
	}
	if len(query) > 2000 {
		return "", apperrors.New(CodeRAGInvalidRequest, "query 过长")
	}

	// 校验每个知识库存在。
	for _, id := range kbIDs {
		if id == "" {
			return "", apperrors.New(CodeRAGInvalidRequest, "knowledge_base_ids 存在空值")
		}
		kb, err := s.kbRepo.FindByKBID(id)
		if err != nil {
			return "", apperrors.New(CodeRAGRetrievalFailed, "校验知识库失败")
		}
		if kb == nil {
			return "", apperrors.New(CodeRAGKBNotFound, "知识库不存在")
		}
	}
	return query, nil
}

func mapRAGRetrievalError(err error) error {
	switch {
	case errors.Is(err, rag.ErrEmbeddingNotConfigured):
		return apperrors.New(CodeRAGNotConfigured, "RAG 未配置")
	case errors.Is(err, rag.ErrEmbeddingUnavailable):
		return apperrors.New(CodeRAGEmbeddingFailed, "Embedding 服务不可用")
	case errors.Is(err, rag.ErrRetrievalUnavailable), errors.Is(err, rag.ErrEmbeddingModelMismatch):
		return apperrors.New(CodeRAGRetrievalFailed, "向量检索失败")
	default:
		return nil
	}
}

// Retrieve 执行检索但不调用 Chat Model
func (s *rAGService) Retrieve(ctx context.Context, kbIDs []string, query string, topK int) (*RAGRetrieveResult, error) {
	query, err := s.validateRequest(kbIDs, query)
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
		topK = s.cfg.VectorTopK
		if topK <= 0 {
			topK = 5
		}
		if topK > 20 {
			topK = 20
		}
	}

	docs, err := s.retriever.Retrieve(ctx, query, rag.WithKnowledgeBaseIDs(kbIDs), einoretriever.WithTopK(topK))
	if err != nil {
		if mapped := mapRAGRetrievalError(err); mapped != nil {
			return nil, mapped
		}
		if logger.Initialized() {
			logger.Warn("RAG 检索失败", zap.Error(err))
		}
		return nil, apperrors.New(CodeRAGRetrievalFailed, "向量检索失败")
	}

	chunks := rag.BuildRetrievedChunks(docs)
	result := &RAGRetrieveResult{QueryText: query, Chunks: make([]RAGChunk, 0, len(chunks))}
	for _, chunk := range chunks {
		result.Chunks = append(result.Chunks, RAGChunk{
			KBID: chunk.KBID, ChunkID: chunk.ChunkID, FileID: chunk.FileID,
			Filename: chunk.Filename, Content: chunk.Content, Score: chunk.Score,
		})
	}
	return result, nil
}

// Answer 执行问答
func (s *rAGService) Answer(ctx context.Context, kbIDs []string, query string) (*RAGAnswerResult, error) {
	query, err := s.validateRequest(kbIDs, query)
	if err != nil {
		return nil, err
	}
	if s.answerer == nil {
		return nil, apperrors.New(CodeRAGNotConfigured, "RAG 未配置")
	}

	out, err := s.answerer.Answer(ctx, rag.AnswerInput{KnowledgeBaseIDs: kbIDs, Query: query})
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
			Index:    c.Index,
			ChunkID:  c.ChunkID,
			FileID:   c.FileID,
			Filename: c.Filename,
			Content:  c.Content,
			Score:    c.Score,
		})
	}
	return result, nil
}
