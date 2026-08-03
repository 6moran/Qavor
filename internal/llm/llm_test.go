package llm

import (
	"context"
	"os"
	"testing"
)

func TestNewClient(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name     string
		provider string
		model    string
		apiKey   string
		baseURL  string
		timeout  int
		wantErr  bool
	}{
		{
			name:     "openai provider without api key",
			provider: "openai",
			model:    "gpt-4o",
			apiKey:   "",
			wantErr:  true,
		},
		{
			name:     "deepseek provider without api key",
			provider: "deepseek",
			model:    "deepseek-chat",
			apiKey:   "",
			wantErr:  true,
		},
		{
			name:     "moonshot provider without api key",
			provider: "moonshot",
			model:    "moonshot-v1-8k",
			apiKey:   "",
			wantErr:  true,
		},
		{
			name:     "unsupported provider",
			provider: "anthropic",
			model:    "claude-3",
			apiKey:   "test-key",
			wantErr:  true,
		},
		{
			name:     "ollama provider without api key should succeed",
			provider: "ollama",
			model:    "llama3",
			apiKey:   "",
			baseURL:  "http://localhost:11434",
			wantErr:  false, // Ollama 不需要 API key
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(ctx, tt.provider, tt.model, tt.apiKey, tt.baseURL, tt.timeout)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewClient() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && client == nil {
				t.Error("NewClient() returned nil client without error")
			}
		})
	}
}

func TestNewClientWithRealAPIKey(t *testing.T) {
	// 跳过没有 API key 的测试
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set, skipping integration test")
	}

	ctx := context.Background()
	client, err := NewClient(ctx, "openai", "gpt-4o", apiKey, "", 60000)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	if client == nil {
		t.Fatal("NewClient() returned nil client")
	}

	t.Logf("Successfully created OpenAI client")
}

func TestNewClientWithDeepSeek(t *testing.T) {
	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	if apiKey == "" {
		t.Skip("DEEPSEEK_API_KEY not set, skipping integration test")
	}

	ctx := context.Background()
	client, err := NewClient(ctx, "deepseek", "deepseek-chat", apiKey, "", 60000)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	if client == nil {
		t.Fatal("NewClient() returned nil client")
	}

	t.Logf("Successfully created DeepSeek client")
}

func TestNewClientWithOllama(t *testing.T) {
	// Ollama 测试需要本地运行 Ollama 服务
	t.Skip("Ollama integration test skipped - requires local Ollama service")

	ctx := context.Background()
	client, err := NewClient(ctx, "ollama", "llama3", "", "http://localhost:11434", 60000)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	if client == nil {
		t.Fatal("NewClient() returned nil client")
	}

	t.Logf("Successfully created Ollama client")
}
