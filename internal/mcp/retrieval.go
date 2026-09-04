package mcp

import (
	"context"
	"math"
	"sort"
	"sync"

	"github.com/cloudwego/eino/components/tool"
	"go.uber.org/zap"

	"Qavor/internal/embedding"
	"Qavor/pkg/logger"
)

// Embedder 嵌入接口。复用项目标准 embedding.Client（EmbedStrings），
// 通过 clientEmbedder 适配为统一调用签名。
type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float64, error)
}

// clientEmbedder 将 embedding.Client（EmbedStrings）适配为 Embedder（Embed）。
// 两个接口方法签名一致，仅方法名不同。
type clientEmbedder struct {
	client embedding.Client
}

// Embed 实现 Embedder 接口
func (e *clientEmbedder) Embed(ctx context.Context, texts []string) ([][]float64, error) {
	return e.client.EmbedStrings(ctx, texts)
}

// ProviderFactory 懒创建 embedding provider 的工厂函数。
// 由 app.go 注入闭包（捕获 modelSvc + systemConfigSvc），每次调用重新解析
// 当前配置的向量模型。返回 nil client 表示未配置，检索不激活。
// key 用于标识当前 provider 对应的模型（如 "db:5" 或 "cfg:openai/text-embedding"），
// 供 ensureProvider 检测模型切换。
type ProviderFactory func(ctx context.Context) (client embedding.Client, key string, err error)

// ToolVectorizer 工具向量检索。
//
// 生命周期（状态机）：
//   - 未激活（默认）：vectors 为空，provider 为 nil，不创建任何向量。
//   - 激活（可检索）：首次 SelectTools 时 provider 懒创建（仅一次），
//     向量增量嵌入（只增不改），每轮检索：读取 → 维度过滤 → 余弦排序 → topK。
//   - 换向量模型：Clear() 清空索引并释放 provider，下次 SelectTools 懒重建。
type ToolVectorizer struct {
	mu          sync.RWMutex
	vectors     map[string][]float64 // 工具名 → 向量，只增不改
	mcpManager  *MCPManager
	factory     ProviderFactory // provider 懒创建工厂（nil = 不启用检索）
	provider    Embedder        // 当前 provider（nil = 未初始化）
	providerKey string          // 当前 provider 的模型标识，检测换模型
}

// NewToolVectorizer 创建工具向量检索器。
// factory 为 nil 时检索不生效（全量注入）。
func NewToolVectorizer(mcpManager *MCPManager, factory ProviderFactory) *ToolVectorizer {
	return &ToolVectorizer{
		vectors:    make(map[string][]float64),
		mcpManager: mcpManager,
		factory:    factory,
	}
}

// ensureProvider 懒创建 provider，并检测模型切换。
// 每次检索时调用：通过 factory 解析当前模型标识，若与已缓存的 providerKey 不同，
// 说明模型已切换 → 清空索引 + 释放旧 provider；provider 为 nil 时懒创建。
func (v *ToolVectorizer) ensureProvider(ctx context.Context) bool {
	if v.factory == nil || v.mcpManager == nil {
		return false
	}

	client, key, err := v.factory(ctx)
	if err != nil {
		logger.Warn("MCP 工具向量检索：解析 embedding provider 失败，降级为全量注入", zap.Error(err))
		return false
	}

	v.mu.Lock()
	defer v.mu.Unlock()

	// 模型未变且 provider 已就绪 → 复用
	if v.provider != nil && v.providerKey == key {
		return true
	}

	// 模型切换：清空索引 + 释放旧 provider（下次检索懒重建）
	v.provider = nil
	v.vectors = make(map[string][]float64)

	if client == nil {
		v.providerKey = key
		return false
	}

	v.provider = &clientEmbedder{client: client}
	v.providerKey = key
	logger.Info("MCP 工具向量检索模型已就绪", zap.String("model_key", key))
	return true
}

// SelectTools 根据查询文本检索最相关的工具名。
// 返回 nil 表示降级为全量注入（provider 不可用 / 检索失败 / 无结果）。
// 内部流程：懒创建/检测 provider → 增量嵌入 → 维度过滤 → 余弦排序 → topK。
func (v *ToolVectorizer) SelectTools(ctx context.Context, query string, topK int) []string {
	if !v.ensureProvider(ctx) {
		return nil
	}

	// 增量嵌入：只嵌入工具缓存里有、向量缓存里没有的工具（只增不改）
	v.ensureVectors(ctx)

	// 查询向量化
	queryVectors, err := v.provider.Embed(ctx, []string{query})
	if err != nil || len(queryVectors) == 0 {
		logger.Warn("MCP 工具向量检索：查询向量化失败，降级为全量注入", zap.Error(err))
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
		// 维度过滤：维度不匹配的向量丢弃（换模型后旧向量自然失效）
		if len(vec) != len(queryVec) {
			continue
		}
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

// ensureVectors 增量嵌入：只嵌入工具缓存里有、向量缓存里没有的工具。
// 已有向量一律跳过（只增不改），与 provider 变化无关。
func (v *ToolVectorizer) ensureVectors(ctx context.Context) {
	allTools := v.mcpManager.GetTools()
	if len(allTools) == 0 {
		return
	}

	// 找出缺失的工具
	v.mu.RLock()
	var missing []tool.BaseTool
	for _, t := range allTools {
		info, err := t.Info(ctx)
		if err != nil {
			continue
		}
		if _, ok := v.vectors[info.Name]; !ok {
			missing = append(missing, t)
		}
	}
	provider := v.provider
	v.mu.RUnlock()

	if len(missing) == 0 {
		return
	}

	// 构造嵌入文本（与 provider 无关，仅依赖工具自身）
	texts := make([]string, len(missing))
	names := make([]string, len(missing))
	for i, t := range missing {
		info, err := t.Info(ctx)
		if err != nil {
			continue
		}
		names[i] = info.Name
		texts[i] = info.Name + ": " + info.Desc
	}

	vectors, err := provider.Embed(ctx, texts)
	if err != nil {
		logger.Warn("MCP 工具向量检索：增量嵌入失败，跳过本轮", zap.Error(err))
		return
	}

	// 写入（只增不改：不覆盖已有项）
	v.mu.Lock()
	for i, name := range names {
		if _, ok := v.vectors[name]; !ok {
			v.vectors[name] = vectors[i]
		}
	}
	v.mu.Unlock()

	logger.Info("MCP 工具向量增量嵌入完成", zap.Int("added", len(names)))
}

// Clear 清空向量索引并释放 provider。
// 换向量模型 / 工具热重载时由外部调用，下次 SelectTools 懒重建。
func (v *ToolVectorizer) Clear() {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.provider = nil
	v.providerKey = ""
	v.vectors = make(map[string][]float64)
	logger.Info("MCP 工具向量检索缓存已清空")
}

// HasProvider 是否已激活检索（provider 已就绪）。
func (v *ToolVectorizer) HasProvider() bool {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.provider != nil
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
