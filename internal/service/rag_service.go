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
	cfg      config.RAGConfig
	kbRepo   repository.KnowledgeBaseRepository
	answerer rag.AnswerChain
}

// NewRAGService 创建 RAG 服务。answerer 为 nil 时 Answer 返回 RAG_NOT_CONFIGURED。
func NewRAGService(cfg config.RAGConfig, kbRepo repository.KnowledgeBaseRepository, answerer rag.AnswerChain) RAGService {
	return &rAGService{cfg: cfg, kbRepo: kbRepo, answerer: answerer}
}

// Answer 执行问答。
func (s *rAGService) Answer(ctx context.Context, kbIDs []string, query string) (*RAGAnswerResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, apperrors.New(CodeRAGInvalidRequest, "query 不能为空")
	}
	if len(kbIDs) == 0 {
		return nil, apperrors.New(CodeRAGInvalidRequest, "knowledge_base_ids 不能为空")
	}
	if len(kbIDs) > 10 {
		return nil, apperrors.New(CodeRAGInvalidRequest, "knowledge_base_ids 最多 10 个")
	}
	if len(query) > 2000 {
		return nil, apperrors.New(CodeRAGInvalidRequest, "query 过长")
	}

	if s.answerer == nil {
		return nil, apperrors.New(CodeRAGNotConfigured, "RAG 未配置")
	}

	// 校验每个知识库存在。
	for _, id := range kbIDs {
		if id == "" {
			return nil, apperrors.New(CodeRAGInvalidRequest, "knowledge_base_ids 存在空值")
		}
		kb, err := s.kbRepo.FindByKBID(id)
		if err != nil {
			return nil, apperrors.New(CodeRAGRetrievalFailed, "校验知识库失败")
		}
		if kb == nil {
			return nil, apperrors.New(CodeRAGKBNotFound, "知识库不存在")
		}
	}

	out, err := s.answerer.Answer(ctx, rag.AnswerInput{KnowledgeBaseIDs: kbIDs, Query: query})
	if err != nil {
		switch {
		case errors.Is(err, rag.ErrEmbeddingNotConfigured):
			return nil, apperrors.New(CodeRAGNotConfigured, "RAG 未配置")
		case errors.Is(err, rag.ErrEmbeddingUnavailable):
			return nil, apperrors.New(CodeRAGEmbeddingFailed, "Embedding 服务不可用")
		case errors.Is(err, rag.ErrRetrievalUnavailable):
			return nil, apperrors.New(CodeRAGRetrievalFailed, "向量检索失败")
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
