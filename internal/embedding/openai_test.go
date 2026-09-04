package embedding

import (
	"context"
	"errors"
	"testing"
)

type fallbackProbeClient struct {
	err      error
	probeSet bool
}

func (c *fallbackProbeClient) EmbedStrings(ctx context.Context, _ []string) ([][]float64, error) {
	c.probeSet = IsArkFallbackProbe(ctx)
	if c.err != nil {
		return nil, c.err
	}
	return [][]float64{{1}}, nil
}

func TestShouldTryArkMultimodal(t *testing.T) {
	for _, message := range []string{
		"The requested model does not support this api",
		"model not support this API",
		"500 Internal Server Error",
	} {
		if !shouldTryArkMultimodal(errors.New(message)) {
			t.Fatalf("expected multimodal fallback for %q", message)
		}
	}
}

func TestNormalizeArkMultimodalBaseURLPreservesCodingPlanPath(t *testing.T) {
	got := normalizeArkMultimodalBaseURL("https://ark.cn-beijing.volces.com/api/coding/v3")
	if got != "https://ark.cn-beijing.volces.com/api/coding/v3" {
		t.Fatalf("base URL was rewritten: %q", got)
	}
}

func TestArkEndpointClientMarksOnlyStandardAttemptAsFallbackProbe(t *testing.T) {
	standard := &fallbackProbeClient{err: errors.New("The requested model does not support this api")}
	multimodal := &fallbackProbeClient{}
	client := newArkEndpointClient(standard, multimodal)

	if _, err := client.EmbedStrings(context.Background(), []string{"query"}); err != nil {
		t.Fatalf("EmbedStrings() error = %v", err)
	}
	if !standard.probeSet {
		t.Fatal("standard attempt was not marked as fallback probe")
	}
	if multimodal.probeSet {
		t.Fatal("multimodal fallback must retain its real trace status")
	}
}
