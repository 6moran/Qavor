package websearch

import (
	"context"
	"time"
)

// MockProvider 本地开发用 Mock，无需 API Key。
// debug 模式且未配置 API Key 时启用，返回固定结果方便前端联调。
type MockProvider struct{}

// NewMockProvider 创建 Mock Provider。
func NewMockProvider() *MockProvider { return &MockProvider{} }

// Name 返回 Provider 名。
func (p *MockProvider) Name() string { return "mock" }

// Search 返回固定的模拟搜索结果。
func (p *MockProvider) Search(_ context.Context, req *SearchRequest) (*SearchResponse, error) {
	start := time.Now()
	maxResults := req.MaxResults
	if maxResults <= 0 || maxResults > 10 {
		maxResults = 5
	}

	// 固定 mock 数据池，按 max_results 截取
	pool := []SearchResultItem{
		{
			Title:   "Mock 搜索结果 1 - " + req.Query,
			URL:     "https://example.com/result-1",
			Content: "这是 Mock Provider 返回的模拟搜索结果，用于本地开发联调前端展示。真实部署请配置 WEB_SEARCH_API_KEY。",
			Score:   0.95,
		},
		{
			Title:   "Mock 搜索结果 2 - " + req.Query,
			URL:     "https://example.com/result-2",
			Content: "第二个模拟结果。当前为 debug 模式且未配置 API Key，自动启用 Mock Provider。",
			Score:   0.88,
		},
		{
			Title:   "Mock 搜索结果 3 - " + req.Query,
			URL:     "https://example.com/result-3",
			Content: "第三个模拟结果。配置 WEB_SEARCH_API_KEY 与 WEB_SEARCH_BASE_URL 后即可切换到真实搜索。",
			Score:   0.72,
		},
	}

	results := make([]SearchResultItem, 0, maxResults)
	for i := 0; i < maxResults && i < len(pool); i++ {
		results = append(results, pool[i])
	}

	return &SearchResponse{
		Query:        req.Query,
		Results:      results,
		Answer:       "Mock Provider 生成的模拟答案（开发模式）。",
		ResponseTime: time.Since(start).Milliseconds(),
		Provider:     p.Name(),
	}, nil
}
