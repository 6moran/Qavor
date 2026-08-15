package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"Qavor/internal/model/dto/request"
	"Qavor/pkg/errors"
)

// openAICompatServer 模拟 OpenAI 兼容 /models 接口；path 与返回体可配置。
func openAICompatServer(t *testing.T, data string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/models" {
			http.NotFound(w, r)
			return
		}
		if r.URL.Path != "/v1/models" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer sk-test" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(data))
	}))
}

func TestFetchRemoteModelsOpenAICompatible(t *testing.T) {
	srv := openAICompatServer(t, `{"data":[{"id":"gpt-4o"},{"id":"gpt-4o-mini"}]}`)
	defer srv.Close()

	svc := NewModelService(nil)
	names, err := svc.FetchRemoteModels(context.Background(), &request.FetchRemoteModelsRequest{
		BaseURL: srv.URL + "/v1",
		APIKey:  "sk-test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(names) != 2 || names[0] != "gpt-4o" || names[1] != "gpt-4o-mini" {
		t.Fatalf("unexpected names: %v", names)
	}
}

func TestFetchRemoteModelsOpenAIFallbackV1(t *testing.T) {
	// base_url 不带 /v1：先试 {base}/models（404），再回退 {base}/v1/models
	srv := openAICompatServer(t, `{"data":[{"id":"deepseek-chat"},{"id":"deepseek-reasoner"}]}`)
	defer srv.Close()

	svc := NewModelService(nil)
	names, err := svc.FetchRemoteModels(context.Background(), &request.FetchRemoteModelsRequest{
		BaseURL: srv.URL,
		APIKey:  "sk-test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(names) != 2 || names[0] != "deepseek-chat" {
		t.Fatalf("unexpected names: %v", names)
	}
}

func TestFetchRemoteModelsOllama(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tags" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"models":[{"name":"llama3.1:8b"},{"name":"qwen2.5:7b"}]}`))
	}))
	defer srv.Close()

	svc := NewModelService(nil)
	names, err := svc.FetchRemoteModels(context.Background(), &request.FetchRemoteModelsRequest{
		BaseURL:  srv.URL,
		Protocol: "ollama",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(names) != 2 || names[0] != "llama3.1:8b" || names[1] != "qwen2.5:7b" {
		t.Fatalf("unexpected names: %v", names)
	}
}

func TestFetchRemoteModelsServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	svc := NewModelService(nil)
	_, err := svc.FetchRemoteModels(context.Background(), &request.FetchRemoteModelsRequest{
		BaseURL: srv.URL + "/v1",
		APIKey:  "sk-test",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.IsBizError(err) {
		t.Fatalf("expected biz error, got: %v", err)
	}
	if err.Error() == "" {
		t.Fatal("expected friendly message")
	}
}

func TestFetchRemoteModelsEmptyList(t *testing.T) {
	srv := openAICompatServer(t, `{"data":[]}`)
	defer srv.Close()

	svc := NewModelService(nil)
	_, err := svc.FetchRemoteModels(context.Background(), &request.FetchRemoteModelsRequest{
		BaseURL: srv.URL + "/v1",
		APIKey:  "sk-test",
	})
	if err == nil || !errors.IsBizError(err) {
		t.Fatalf("expected biz error for empty list, got: %v", err)
	}
}

func TestFetchRemoteModelsMissingBaseURL(t *testing.T) {
	svc := NewModelService(nil)
	_, err := svc.FetchRemoteModels(context.Background(), &request.FetchRemoteModelsRequest{})
	if err == nil || !errors.IsBizError(err) {
		t.Fatalf("expected biz error for missing base_url, got: %v", err)
	}
}
