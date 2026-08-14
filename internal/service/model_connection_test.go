package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"Qavor/internal/model/dto/request"
	"Qavor/internal/model/entity"
	qerrors "Qavor/pkg/errors"
	pkgllm "Qavor/pkg/llm"
)

func TestModelService_TestRerankConnection(t *testing.T) {
	var gotQuery string
	var gotDocuments []string
	var gotHeader string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Get("X-Test-Header")
		var body struct {
			Query     string   `json:"query"`
			Documents []string `json:"documents"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("解析请求体: %v", err)
		}
		gotQuery = body.Query
		gotDocuments = body.Documents
		_, _ = w.Write([]byte(`{"results":[{"index":0,"relevance_score":0.9}]}`))
	}))
	defer server.Close()

	svc := NewModelService(&fakeResolveModelRepository{models: map[uint]*entity.Model{}})
	result, err := svc.TestConnection(context.Background(), &request.ModelConnectionTestRequest{
		Name: "bge-reranker-v2-m3", Protocol: "openai", BaseURL: server.URL,
		APIKey:  "secret",
		Headers: map[string]string{"X-Test-Header": "test-value"},
		Timeout: 1000, ModelType: "rerank",
	})
	if err != nil {
		t.Fatalf("测试 Rerank 连接: %v", err)
	}
	if !result.Connected || result.ModelType != "rerank" || result.LatencyMS < 1 {
		t.Fatalf("连接结果=%+v", result)
	}
	if gotQuery != "连接测试" || len(gotDocuments) != 2 || gotDocuments[0] != "相关内容" || gotDocuments[1] != "无关内容" {
		t.Fatalf("query=%q documents=%v", gotQuery, gotDocuments)
	}
	if gotHeader != "test-value" {
		t.Fatalf("自定义请求头未转发到 Rerank 服务: %q", gotHeader)
	}
}

func TestModelService_TestRerankConnectionRejectsInvalidOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"results":[{"index":8,"relevance_score":0.9}]}`))
	}))
	defer server.Close()
	svc := NewModelService(&fakeResolveModelRepository{models: map[uint]*entity.Model{}})
	_, err := svc.TestConnection(context.Background(), &request.ModelConnectionTestRequest{
		Name: "reranker", Protocol: "openai", BaseURL: server.URL,
		APIKey: "secret", Timeout: 1000, ModelType: "rerank",
	})
	bizErr, ok := err.(*qerrors.BizError)
	if !ok || bizErr.Code != qerrors.CodeLLMRequestFailed {
		t.Fatalf("错误=%v，期望连接请求失败业务错误", err)
	}
}

func TestClassifyConnectionTestError(t *testing.T) {
	cases := []struct {
		name     string
		err      error
		category string
	}{
		{"超时", context.DeadlineExceeded, "timeout"},
		{"deadline 文本", fmt.Errorf("post https://api.example.com: context deadline exceeded"), "timeout"},
		{"DNS 解析失败", fmt.Errorf(`Get "https://api.example.com/v1": dial tcp: lookup api.example.com: no such host`), "connection_failed"},
		{"连接拒绝", fmt.Errorf("dial tcp 127.0.0.1:11434: connect: connection refused"), "connection_failed"},
		{"端口含 404 不误判", fmt.Errorf("dial tcp localhost:4404: connect: connection refused"), "connection_failed"},
		{"https 不误判为连接失败", fmt.Errorf(`Post "https://api.openai.com/v1/chat/completions": openai: error, status code: 401, message: invalid_api_key`), "auth_failed"},
		{"余额不足状态码", fmt.Errorf("openai: error, status code: 402, message: insufficient_quota"), "insufficient_quota"},
		{"余额不足关键词", errors.New("You exceeded your current quota, please check your plan and billing details"), "insufficient_quota"},
		{"认证失败", fmt.Errorf("openai: error, status code: 401, message: invalid api key"), "auth_failed"},
		{"模型不存在", fmt.Errorf("openai: error, status code: 404, message: model not found"), "model_not_found"},
		{"限流", fmt.Errorf("openai: error, status code: 429, message: rate limit reached"), "rate_limited"},
		{"rerank 错误格式", errors.New("Rerank 服务返回 HTTP 402"), "insufficient_quota"},
		{"未知错误", errors.New("boom"), "unknown"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// 错误分类逻辑已抽到公共包 pkg/llm，与模型连接测试共用
			category := pkgllm.ClassifyError(tc.err).Category
			if category != tc.category {
				t.Fatalf("分类=%q，期望=%q（错误=%v）", category, tc.category, tc.err)
			}
		})
	}
}

func TestModelService_TestConnectionReturnsFriendlyError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusPaymentRequired)
		_, _ = w.Write([]byte(`{"error":{"message":"insufficient_quota"}}`))
	}))
	defer server.Close()
	svc := NewModelService(&fakeResolveModelRepository{models: map[uint]*entity.Model{}})
	_, err := svc.TestConnection(context.Background(), &request.ModelConnectionTestRequest{
		Name: "reranker", Protocol: "openai", BaseURL: server.URL,
		APIKey: "secret", Timeout: 1000, ModelType: "rerank",
	})
	bizErr, ok := err.(*qerrors.BizError)
	if !ok || bizErr.Code != qerrors.CodeLLMRequestFailed {
		t.Fatalf("错误=%v，期望连接请求失败业务错误", err)
	}
	if !strings.Contains(bizErr.Message, "余额") {
		t.Fatalf("message 应为友好提示，实际=%q", bizErr.Message)
	}
	if !strings.Contains(bizErr.Detail, "402") {
		t.Fatalf("detail 应为脱敏原始错误，实际=%q", bizErr.Detail)
	}
}
