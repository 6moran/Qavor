package websearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// TavilyProvider 调用 Tavily Search API。
// 官方地址：https://api.tavily.com，需通过环境变量 WEB_SEARCH_BASE_URL 显式配置。
type TavilyProvider struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewTavilyProvider 创建 Tavily Provider。baseURL 末尾斜杠会被去掉。
func NewTavilyProvider(baseURL, apiKey string) *TavilyProvider {
	return &TavilyProvider{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// Name 返回 Provider 名。
func (p *TavilyProvider) Name() string { return "tavily" }

// Search 调用 Tavily /search 端点。
func (p *TavilyProvider) Search(ctx context.Context, req *SearchRequest) (*SearchResponse, error) {
	start := time.Now()

	maxResults := req.MaxResults
	if maxResults <= 0 || maxResults > 10 {
		maxResults = 5
	}

	payload := map[string]any{
		"query":          req.Query,
		"max_results":    maxResults,
		"search_depth":   "basic",
		"include_answer": true,
	}
	if req.Topic != "" {
		payload["topic"] = req.Topic
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("构建 Tavily 请求失败: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		p.baseURL+"/search", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("创建 Tavily 请求失败: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("调用 Tavily API 失败: %w", err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			fmt.Println("关闭响应体失败:", err)
		}
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tavily API 返回非 200 状态码: %d", resp.StatusCode)
	}

	// Tavily 响应结构（仅解析需要的字段）
	var raw struct {
		Results []struct {
			Title   string  `json:"title"`
			URL     string  `json:"url"`
			Content string  `json:"content"`
			Score   float64 `json:"score"`
		} `json:"results"`
		Answer string `json:"answer"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("解析 Tavily 响应失败: %w", err)
	}

	items := make([]SearchResultItem, 0, len(raw.Results))
	for _, r := range raw.Results {
		items = append(items, SearchResultItem{
			Title:   r.Title,
			URL:     r.URL,
			Content: r.Content,
			Score:   r.Score,
		})
	}

	return &SearchResponse{
		Query:        req.Query,
		Results:      items,
		Answer:       raw.Answer,
		ResponseTime: time.Since(start).Milliseconds(),
		Provider:     p.Name(),
	}, nil
}
