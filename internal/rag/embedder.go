package rag

import (
	"context"

	"Qavor/internal/embedding"
	einoEmbedding "github.com/cloudwego/eino/components/embedding"
)

// NewEmbedderFromClient 将模型管理模块创建的客户端接入 RAG 的 Eino 组件。
func NewEmbedderFromClient(ctx context.Context, client embedding.Client) einoEmbedding.Embedder {
	// 暂时只包装了 /internal/embedding
	_ = ctx
	return embedding.AsEinoEmbedder(client)
}
