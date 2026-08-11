package service

import "context"

// SettingKeyRAGRerankModelID 是全局重排模型设置键。
const SettingKeyRAGRerankModelID = "rag.rerank_model_id"

// RAGSettings 表示可公开的全局 RAG 设置。
type RAGSettings struct {
	RerankModelID   *uint
	RerankModelName string
}

// RAGSettingsService 管理全局 RAG 设置。
type RAGSettingsService interface {
	Get(ctx context.Context) (*RAGSettings, error)
	UpdateRerankModel(ctx context.Context, modelID *uint) (*RAGSettings, error)
	RerankModelID(ctx context.Context) (uint, bool, error)
}
