package config

import "testing"

func TestDocumentParserConfigApplyDefaults(t *testing.T) {
	cfg := DocumentParserConfig{}
	cfg.ApplyDefaults()
	if cfg.PythonPath != "python" || cfg.PoolSize != 2 || cfg.MaxTasksPerWorker != 100 {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
}

func TestDocumentParserConfigPreservesExplicitPoolValues(t *testing.T) {
	cfg := DocumentParserConfig{PythonPath: "py", PoolSize: 4, MaxTasksPerWorker: 20}
	cfg.ApplyDefaults()
	if cfg.PoolSize != 4 || cfg.MaxTasksPerWorker != 20 {
		t.Fatalf("explicit values changed: %+v", cfg)
	}
}

func TestRAGApplyDefaultsTopKNormalization(t *testing.T) {
	cfg := &RAGConfig{TopK: 5} // 模拟 yaml 仅配置 top_k: 5
	cfg.ApplyDefaults()
	if cfg.VectorTopK != 5 {
		t.Errorf("VectorTopK = %d, want 5 (top_k 归一化)", cfg.VectorTopK)
	}
	if cfg.ScoreThreshold != 0.3 {
		t.Errorf("ScoreThreshold = %.2f, want 0.3", cfg.ScoreThreshold)
	}
}

func TestRAGApplyDefaultsExplicitVectorTopKWins(t *testing.T) {
	cfg := &RAGConfig{TopK: 5, VectorTopK: 8} // 显式配置优先
	cfg.ApplyDefaults()
	if cfg.VectorTopK != 8 {
		t.Errorf("VectorTopK = %d, want 8 (显式配置优先)", cfg.VectorTopK)
	}
}

func TestRAGApplyDefaultsExplicitHybridValuesWin(t *testing.T) {
	cfg := &RAGConfig{
		TopK: 5, VectorTopK: 11, KeywordTopK: 12, FusedTopK: 13, RerankTopK: 4, RRFK: 70,
	}
	cfg.ApplyDefaults()
	if cfg.VectorTopK != 11 || cfg.KeywordTopK != 12 || cfg.FusedTopK != 13 || cfg.RerankTopK != 4 || cfg.RRFK != 70 {
		t.Fatalf("显式混合检索配置被覆盖: %+v", cfg)
	}
}
