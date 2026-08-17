package rag

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestHTTPReranker_RequestContractAndResultOrder(t *testing.T) {
	var gotPath string
	var gotAuthorization string
	var gotCustom string
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuthorization = r.Header.Get("Authorization")
		gotCustom = r.Header.Get("X-Tenant")
		body, _ := io.ReadAll(r.Body)
		if r.Method != http.MethodPost {
			t.Errorf("请求方法=%s，期望 POST", r.Method)
		}
		if err := json.Unmarshal(body, &gotBody); err != nil {
			t.Errorf("解析请求体: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[{"index":1,"relevance_score":0.92},{"index":0,"relevance_score":0.75}]}`))
	}))
	defer server.Close()

	reranker, err := NewHTTPReranker(HTTPRerankerConfig{
		Model:   "bge-reranker-v2-m3",
		BaseURL: server.URL,
		APIKey:  "secret",
		Headers: map[string]string{"X-Tenant": "qavor"},
		Timeout: time.Second,
	})
	if err != nil {
		t.Fatalf("创建客户端: %v", err)
	}
	results, err := reranker.Rerank(context.Background(), "退款流程", []RerankDocument{
		{ID: "a", Content: "片段一"},
		{ID: "b", Content: "片段二"},
	}, 2)
	if err != nil {
		t.Fatalf("执行重排: %v", err)
	}
	if gotPath != "/v1/rerank" || gotAuthorization != "Bearer secret" || gotCustom != "qavor" {
		t.Fatalf("请求 path=%q authorization=%q custom=%q", gotPath, gotAuthorization, gotCustom)
	}
	wantBody := map[string]any{
		"model":            "bge-reranker-v2-m3",
		"query":            "退款流程",
		"documents":        []any{"片段一", "片段二"},
		"top_n":            float64(2),
		"return_documents": false,
	}
	if !reflect.DeepEqual(gotBody, wantBody) {
		t.Fatalf("请求体=%#v，期望 %#v", gotBody, wantBody)
	}
	wantResults := []RerankResult{{Index: 1, Score: 0.92}, {Index: 0, Score: 0.75}}
	if !reflect.DeepEqual(results, wantResults) {
		t.Fatalf("结果=%#v，期望 %#v", results, wantResults)
	}
}

func TestHTTPReranker_UsesCompleteEndpointAndCustomHeaderOverridesBearer(t *testing.T) {
	var gotPath string
	var gotAuthorization string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuthorization = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"results":[{"index":0,"relevance_score":1}]}`))
	}))
	defer server.Close()

	reranker, err := NewHTTPReranker(HTTPRerankerConfig{
		Model:   "reranker",
		BaseURL: server.URL + "/custom/rerank",
		APIKey:  "secret",
		Headers: map[string]string{"Authorization": "Custom token"},
	})
	if err != nil {
		t.Fatalf("创建客户端: %v", err)
	}
	if _, err := reranker.Rerank(context.Background(), "query", []RerankDocument{{Content: "doc"}}, 1); err != nil {
		t.Fatalf("执行重排: %v", err)
	}
	if gotPath != "/custom/rerank" || gotAuthorization != "Custom token" {
		t.Fatalf("请求 path=%q authorization=%q", gotPath, gotAuthorization)
	}
}

func TestHTTPReranker_PartialAndDuplicateResults(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []RerankResult
	}{
		{name: "部分结果", body: `{"results":[{"index":1,"relevance_score":0.8}]}`, want: []RerankResult{{Index: 1, Score: 0.8}}},
		{name: "重复索引保留首项", body: `{"results":[{"index":0,"relevance_score":0.9},{"index":0,"relevance_score":0.1},{"index":1,"relevance_score":0.7}]}`, want: []RerankResult{{Index: 0, Score: 0.9}, {Index: 1, Score: 0.7}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := rerankResponseServer(tt.body, http.StatusOK, 0)
			defer server.Close()
			reranker, _ := NewHTTPReranker(HTTPRerankerConfig{Model: "reranker", BaseURL: server.URL})
			got, err := reranker.Rerank(context.Background(), "query", []RerankDocument{{Content: "a"}, {Content: "b"}}, 2)
			if err != nil {
				t.Fatalf("执行重排: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("结果=%#v，期望 %#v", got, tt.want)
			}
		})
	}
}

func TestHTTPReranker_RejectsInvalidResponses(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		statusCode int
		wantError  string
	}{
		{name: "越界索引", body: `{"results":[{"index":2,"relevance_score":0.8}]}`, statusCode: http.StatusOK, wantError: "索引"},
		{name: "负索引", body: `{"results":[{"index":-1,"relevance_score":0.8}]}`, statusCode: http.StatusOK, wantError: "索引"},
		{name: "非有限分数", body: `{"results":[{"index":0,"relevance_score":1e309}]}`, statusCode: http.StatusOK, wantError: "分数"},
		{name: "空结果", body: `{"results":[]}`, statusCode: http.StatusOK, wantError: "空"},
		{name: "HTTP 错误", body: `service unavailable`, statusCode: http.StatusServiceUnavailable, wantError: "503"},
		{name: "非法 JSON", body: `{`, statusCode: http.StatusOK, wantError: "JSON"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := rerankResponseServer(tt.body, tt.statusCode, 0)
			defer server.Close()
			reranker, _ := NewHTTPReranker(HTTPRerankerConfig{Model: "reranker", BaseURL: server.URL})
			_, err := reranker.Rerank(context.Background(), "query", []RerankDocument{{Content: "a"}, {Content: "b"}}, 2)
			if err == nil || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("错误=%v，期望包含 %q", err, tt.wantError)
			}
		})
	}
}

func TestHTTPReranker_PropagatesContextTimeout(t *testing.T) {
	server := rerankResponseServer(`{"results":[{"index":0,"relevance_score":1}]}`, http.StatusOK, 100*time.Millisecond)
	defer server.Close()
	reranker, _ := NewHTTPReranker(HTTPRerankerConfig{Model: "reranker", BaseURL: server.URL, Timeout: time.Second})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := reranker.Rerank(ctx, "query", []RerankDocument{{Content: "a"}}, 1)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("错误=%v，期望 context deadline exceeded", err)
	}
}

func rerankResponseServer(body string, statusCode int, delay time.Duration) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if delay > 0 {
			time.Sleep(delay)
		}
		w.WriteHeader(statusCode)
		_, _ = w.Write([]byte(body))
	}))
}
