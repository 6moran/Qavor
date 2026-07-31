package embedding

import (
	"context"
)

// Client Embedding 客户端接口
type Client interface {
	// EmbedStrings 将文本转换为向量
	// 返回 [][]float64，每个元素对应一个输入文本的向量
	EmbedStrings(ctx context.Context, input []string) ([][]float64, error)
}
