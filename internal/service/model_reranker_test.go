package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"Qavor/internal/model/entity"
	"Qavor/internal/rag"
	"Qavor/pkg/crypto"
	"Qavor/pkg/database/types"
)

type fakeResolveModelRepository struct {
	models map[uint]*entity.Model
}

func (r *fakeResolveModelRepository) FindByID(id uint) (*entity.Model, error) {
	return r.models[id], nil
}

func (r *fakeResolveModelRepository) Create(*entity.Model) error { return nil }
func (r *fakeResolveModelRepository) Update(*entity.Model) error { return nil }
func (r *fakeResolveModelRepository) Delete(uint) error          { return nil }
func (r *fakeResolveModelRepository) List(int, int, string, string) ([]*entity.Model, int64, error) {
	return nil, 0, nil
}

func TestModelService_ResolveRerankerUsesDecryptedConfiguration(t *testing.T) {
	var gotAuthorization string
	var gotTenant string
	var gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuthorization = r.Header.Get("Authorization")
		gotTenant = r.Header.Get("X-Tenant")
		buffer := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(buffer)
		gotBody = string(buffer)
		_, _ = w.Write([]byte(`{"results":[{"index":0,"relevance_score":0.9}]}`))
	}))
	defer server.Close()
	encrypted, err := crypto.Encrypt("decrypted-secret")
	if err != nil {
		t.Fatalf("加密测试密钥: %v", err)
	}
	repo := &fakeResolveModelRepository{models: map[uint]*entity.Model{
		7: {
			BaseEntity: entity.BaseEntity{ID: 7},
			Name:       "bge-reranker-v2-m3",
			BaseURL:    server.URL,
			APIKey:     encrypted,
			Headers:    types.StringMap{"X-Tenant": "qavor"},
			Timeout:    1000,
			Enabled:    true,
			ModelType:  "rerank",
		},
	}}
	svc := NewModelService(repo)
	reranker, err := svc.ResolveReranker(context.Background(), 7)
	if err != nil {
		t.Fatalf("解析重排模型: %v", err)
	}
	if _, err := reranker.Rerank(context.Background(), "退款流程", []rag.RerankDocument{{Content: "相关内容"}}, 1); err != nil {
		t.Fatalf("调用解析后的客户端: %v", err)
	}
	if gotAuthorization != "Bearer decrypted-secret" || gotTenant != "qavor" {
		t.Fatalf("authorization=%q tenant=%q", gotAuthorization, gotTenant)
	}
	if !strings.Contains(gotBody, `"model":"bge-reranker-v2-m3"`) {
		t.Fatalf("请求体未使用模型名称: %s", gotBody)
	}
}

func TestModelService_ResolveRerankerRejectsInvalidModels(t *testing.T) {
	tests := []struct {
		name  string
		id    uint
		model *entity.Model
	}{
		{name: "模型不存在", id: 1},
		{name: "模型已禁用", id: 2, model: &entity.Model{Enabled: false, ModelType: "rerank"}},
		{name: "模型类型错误", id: 3, model: &entity.Model{Enabled: true, ModelType: "chat"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeResolveModelRepository{models: map[uint]*entity.Model{}}
			if tt.model != nil {
				repo.models[tt.id] = tt.model
			}
			svc := NewModelService(repo)
			if _, err := svc.ResolveReranker(context.Background(), tt.id); err == nil {
				t.Fatal("期望解析失败")
			}
		})
	}
}

func TestModelService_ResolveRerankerUsesConfiguredTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(80 * time.Millisecond)
		_, _ = w.Write([]byte(`{"results":[{"index":0,"relevance_score":1}]}`))
	}))
	defer server.Close()
	repo := &fakeResolveModelRepository{models: map[uint]*entity.Model{
		4: {Name: "reranker", BaseURL: server.URL, Timeout: 10, Enabled: true, ModelType: "rerank"},
	}}
	svc := NewModelService(repo)
	reranker, err := svc.ResolveReranker(context.Background(), 4)
	if err != nil {
		t.Fatalf("解析重排模型: %v", err)
	}
	if _, err := reranker.Rerank(context.Background(), "query", []rag.RerankDocument{{Content: "doc"}}, 1); err == nil {
		t.Fatal("期望客户端按模型超时配置终止请求")
	}
}
