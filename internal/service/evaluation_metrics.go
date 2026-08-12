package service

import "math"

// RetrievalMetrics 单项检索指标（与前端展示契约一致）。
type RetrievalMetrics struct {
	RecallAt10    float64 `json:"recall@10"`
	PrecisionAt10 float64 `json:"precision@10"`
	MapAt10       float64 `json:"map@10"`
	NDCGAt10      float64 `json:"ndcg@10"`
}

// topK 检索指标统一使用的截断数量。
const evalTopK = 10

// computeRetrievalMetrics 根据检索返回的分块 ID 列表与期望命中的 Gold Chunk ID 列表计算检索指标。
// 命中判断基于分块 ID 的精确匹配；gold 为空时指标全部为 0（该条目不参与召回类指标统计）。
func computeRetrievalMetrics(retrievedIDs, goldIDs []string) RetrievalMetrics {
	goldSet := make(map[string]bool, len(goldIDs))
	for _, id := range goldIDs {
		if id != "" {
			goldSet[id] = true
		}
	}
	if len(goldSet) == 0 {
		return RetrievalMetrics{}
	}

	// 截断到 TopK
	k := evalTopK
	if len(retrievedIDs) < k {
		k = len(retrievedIDs)
	}
	top := retrievedIDs[:k]

	// 命中序列（保持检索顺序，用于 AP 与 NDCG）
	hits := make([]bool, len(top))
	hitCount := 0
	for i, id := range top {
		if goldSet[id] {
			hits[i] = true
			hitCount++
		}
	}

	metrics := RetrievalMetrics{
		RecallAt10:    float64(hitCount) / float64(len(goldSet)),
		PrecisionAt10: float64(hitCount) / float64(k),
	}

	// MAP@10：对每个相关文档计算其位置的精确率并取平均
	var apSum float64
	seenHits := 0
	for i, hit := range hits {
		if hit {
			seenHits++
			apSum += float64(seenHits) / float64(i+1)
		}
	}
	if seenHits > 0 {
		metrics.MapAt10 = apSum / float64(len(goldSet))
	}

	// NDCG@10：按命中位置计算 DCG，除以理想 DCG
	dcg := 0.0
	for i, hit := range hits {
		if hit {
			dcg += 1.0 / math.Log2(float64(i+2))
		}
	}
	idcg := 0.0
	for i := 0; i < hitCount; i++ {
		idcg += 1.0 / math.Log2(float64(i+2))
	}
	if idcg > 0 {
		metrics.NDCGAt10 = dcg / idcg
	}

	return metrics
}

// metricsToMap 将检索指标转换为 JSON 友好的 map。
func (m RetrievalMetrics) toMap() map[string]any {
	return map[string]any{
		"recall@10":    round4(m.RecallAt10),
		"precision@10": round4(m.PrecisionAt10),
		"map@10":       round4(m.MapAt10),
		"ndcg@10":      round4(m.NDCGAt10),
	}
}

// round4 保留 4 位小数。
func round4(v float64) float64 {
	return math.Round(v*10000) / 10000
}
