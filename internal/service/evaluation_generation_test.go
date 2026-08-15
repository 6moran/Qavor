package service

import "testing"

func TestValidateGenerateRequestAllowsGraphEnhanced(t *testing.T) {
	req := &GenerateDatasetRequest{
		Name:             "图增强基准",
		Count:            2,
		NeighborsCount:   2,
		ConcurrencyCount: 1,
		GenerationMode:   "graph_enhanced",
		GraphExpandTopK:  3,
	}
	if err := validateGenerateRequest(req); err != nil {
		t.Fatalf("validateGenerateRequest() error = %v", err)
	}
}
