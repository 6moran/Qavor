package service

import (
	"math"
	"testing"
)

func TestComputeRetrievalMetrics(t *testing.T) {
	tests := []struct {
		name       string
		retrieved  []string
		gold       []string
		wantRecall float64
		wantPrec   float64
		wantMAP    float64
		wantNDCG   float64
	}{
		{
			name:       "全部命中",
			retrieved:  []string{"a", "b", "c"},
			gold:       []string{"a", "b"},
			wantRecall: 1.0,
			wantPrec:   2.0 / 3.0,
			wantMAP:    1.0,
			wantNDCG:   1.0,
		},
		{
			name:       "部分命中",
			retrieved:  []string{"a", "x", "b", "y"},
			gold:       []string{"a", "b", "z"},
			wantRecall: 2.0 / 3.0,
			wantPrec:   2.0 / 4.0,
			wantMAP:    (1.0/1.0 + 2.0/3.0) / 3.0,
			wantNDCG:   (1.0/math.Log2(2) + 1.0/math.Log2(4)) / (1.0/math.Log2(2) + 1.0/math.Log2(3)),
		},
		{
			name:       "无命中",
			retrieved:  []string{"x", "y"},
			gold:       []string{"a", "b"},
			wantRecall: 0,
			wantPrec:   0,
			wantMAP:    0,
			wantNDCG:   0,
		},
		{
			name:       "gold 为空返回零值",
			retrieved:  []string{"a"},
			gold:       []string{},
			wantRecall: 0,
			wantPrec:   0,
			wantMAP:    0,
			wantNDCG:   0,
		},
		{
			name:       "超过 TopK 截断",
			retrieved:  []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11"},
			gold:       []string{"11"},
			wantRecall: 0, // 第 11 位被截断
			wantPrec:   0,
			wantMAP:    0,
			wantNDCG:   0,
		},
		{
			name:       "命中靠后位置影响排序指标",
			retrieved:  []string{"x", "x", "x", "a"},
			gold:       []string{"a"},
			wantRecall: 1.0,
			wantPrec:   1.0 / 4.0,
			wantMAP:    (1.0 / 4.0) / 1.0,
			wantNDCG:   1.0 / math.Log2(5),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeRetrievalMetrics(tt.retrieved, tt.gold)
			assertClose(t, "recall@10", got.RecallAt10, tt.wantRecall)
			assertClose(t, "precision@10", got.PrecisionAt10, tt.wantPrec)
			assertClose(t, "map@10", got.MapAt10, tt.wantMAP)
			assertClose(t, "ndcg@10", got.NDCGAt10, tt.wantNDCG)
		})
	}
}

func assertClose(t *testing.T, name string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("%s = %v, want %v", name, got, want)
	}
}
