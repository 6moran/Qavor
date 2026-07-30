package embedding

import (
	"context"
	"os"
	"testing"
)

func TestNewOpenAIClient(t *testing.T) {
	tests := []struct {
		name    string
		apiKey  string
		model   string
		baseURL string
		timeout int
		wantErr bool
	}{
		{
			name:    "without api key",
			apiKey:  "",
			model:   "text-embedding-3-small",
			wantErr: true,
		},
		{
			name:    "with api key",
			apiKey:  "test-key",
			model:   "text-embedding-3-small",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewOpenAIClient(context.Background(), tt.apiKey, tt.model, tt.baseURL, tt.timeout)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewOpenAIClient() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && client == nil {
				t.Error("NewOpenAIClient() returned nil client without error")
			}
		})
	}
}

func TestNewOpenAIClientWithRealAPIKey(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set, skipping integration test")
	}

	ctx := context.Background()
	client, err := NewOpenAIClient(ctx, apiKey, "text-embedding-3-small", "", 60000)
	if err != nil {
		t.Fatalf("NewOpenAIClient() error = %v", err)
	}
	if client == nil {
		t.Fatal("NewOpenAIClient() returned nil client")
	}

	// 测试 EmbedStrings
	vectors, err := client.EmbedStrings(ctx, []string{"hello world"})
	if err != nil {
		t.Fatalf("EmbedStrings() error = %v", err)
	}
	if len(vectors) != 1 {
		t.Fatalf("EmbedStrings() returned %d vectors, expected 1", len(vectors))
	}
	if len(vectors[0]) == 0 {
		t.Fatal("EmbedStrings() returned empty vector")
	}

	t.Logf("Successfully embedded text, vector dimension: %d", len(vectors[0]))
}

func TestEmbedStrings(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set, skipping integration test")
	}

	ctx := context.Background()
	client, err := NewOpenAIClient(ctx, apiKey, "text-embedding-3-small", "", 60000)
	if err != nil {
		t.Fatalf("NewOpenAIClient() error = %v", err)
	}

	// 测试批量嵌入
	texts := []string{
		"Hello, world!",
		"This is a test.",
		"Embedding is useful for semantic search.",
	}

	vectors, err := client.EmbedStrings(ctx, texts)
	if err != nil {
		t.Fatalf("EmbedStrings() error = %v", err)
	}
	if len(vectors) != len(texts) {
		t.Fatalf("EmbedStrings() returned %d vectors, expected %d", len(vectors), len(texts))
	}

	for i, v := range vectors {
		if len(v) == 0 {
			t.Errorf("Vector %d is empty", i)
		}
	}

	t.Logf("Successfully embedded %d texts", len(texts))
}
