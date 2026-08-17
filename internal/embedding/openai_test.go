package embedding

import (
	"errors"
	"testing"
)

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
