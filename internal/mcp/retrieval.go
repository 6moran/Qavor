package mcp

import (
	"context"
	"crypto/sha256"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"

	"github.com/cloudwego/eino/components/tool"
	"go.uber.org/zap"

	"Qavor/pkg/logger"
)

// Embedder 嵌入接口（预留，后续接入 embedding 模型）
type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float64, error)
}

// ToolVectorizer 工具向量检索
type ToolVectorizer struct {
	mu          sync.RWMutex
	vectors     map[string][]float64
	embedder    Embedder
	mcpManager  *MCPManager
	fingerprint string
}

// NewToolVectorizer 创建工具向量检索器
func NewToolVectorizer(mcpManager *MCPManager, embedder Embedder) *ToolVectorizer {
	return &ToolVectorizer{
		vectors:    make(map[string][]float64),
		embedder:   embedder,
		mcpManager: mcpManager,
	}
}

// SelectTools 根据查询文本检索最相关的工具名
// 自动检测工具列表变化并重建向量，返回 nil 表示降级为全量注入
func (v *ToolVectorizer) SelectTools(ctx context.Context, query string, topK int) []string {
	if v.embedder == nil || v.mcpManager == nil {
		return nil
	}

	allTools := v.mcpManager.GetTools()
	fp := v.computeFingerprint(ctx, allTools)

	v.mu.RLock()
	changed := fp != v.fingerprint
	cached := len(v.vectors) > 0
	v.mu.RUnlock()

	if changed || !cached {
		if err := v.buildVectors(ctx, allTools); err != nil {
			return nil
		}
	}

	queryVectors, err := v.embedder.Embed(ctx, []string{query})
	if err != nil || len(queryVectors) == 0 {
		return nil
	}
	queryVec := queryVectors[0]

	type scored struct {
		name  string
		score float64
	}

	v.mu.RLock()
	results := make([]scored, 0, len(v.vectors))
	for name, vec := range v.vectors {
		results = append(results, scored{
			name:  name,
			score: cosineSimilarity(queryVec, vec),
		})
	}
	v.mu.RUnlock()

	sort.Slice(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})

	if topK > len(results) {
		topK = len(results)
	}

	names := make([]string, topK)
	for i := 0; i < topK; i++ {
		names[i] = results[i].name
	}
	return names
}

// buildVectors 从工具列表构建向量
func (v *ToolVectorizer) buildVectors(ctx context.Context, tools []tool.BaseTool) error {
	type toolInfo struct {
		name string
		text string
	}

	var infos []toolInfo
	for _, t := range tools {
		info, err := t.Info(ctx)
		if err != nil {
			continue
		}
		infos = append(infos, toolInfo{
			name: info.Name,
			text: info.Name + ": " + info.Desc,
		})
	}

	if len(infos) == 0 {
		return nil
	}

	texts := make([]string, len(infos))
	for i, info := range infos {
		texts[i] = info.text
	}

	vectors, err := v.embedder.Embed(ctx, texts)
	if err != nil {
		logger.Warn("构建工具向量失败，降级为全量注入", zap.Error(err))
		return err
	}

	v.mu.Lock()
	v.vectors = make(map[string][]float64, len(infos))
	for i, info := range infos {
		v.vectors[info.name] = vectors[i]
	}
	v.fingerprint = v.computeFingerprint(ctx, tools)
	v.mu.Unlock()

	logger.Info("工具向量构建完成", zap.Int("count", len(infos)))
	return nil
}

// computeFingerprint 计算工具列表指纹
func (v *ToolVectorizer) computeFingerprint(ctx context.Context, tools []tool.BaseTool) string {
	names := make([]string, 0, len(tools))
	for _, t := range tools {
		info, err := t.Info(ctx)
		if err != nil {
			continue
		}
		names = append(names, info.Name)
	}
	sort.Strings(names)
	hash := sha256.Sum256([]byte(strings.Join(names, ",")))
	return fmt.Sprintf("%x", hash)
}

// Clear 清空向量缓存
func (v *ToolVectorizer) Clear() {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.vectors = make(map[string][]float64)
	v.fingerprint = ""
}

// cosineSimilarity 计算余弦相似度
func cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}
	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}
