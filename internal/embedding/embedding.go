package embedding

import (
	"context"

	einoEmbedding "github.com/cloudwego/eino/components/embedding"
)

// Client Embedding 客户端接口
type Client interface {
	// EmbedStrings 将文本转换为向量
	// 返回 [][]float64，每个元素对应一个输入文本的向量
	EmbedStrings(ctx context.Context, input []string) ([][]float64, error)
}

// AsEinoEmbedder 将已有模型管理/CRUD 使用的 Embedding Client 适配为 Eino Embedder。
// Client 与 Eino Embedder 共享 EmbedStrings 方法，因此不创建第二个模型客户端。
func AsEinoEmbedder(client Client) einoEmbedding.Embedder {
	if client == nil {
		return nil
	}
	return &einoClientAdapter{client: client}
}

type einoClientAdapter struct {
	client Client
}

func (a *einoClientAdapter) EmbedStrings(ctx context.Context, input []string, _ ...einoEmbedding.Option) ([][]float64, error) {
	return a.client.EmbedStrings(ctx, input)
}
