package websearch

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// BraveProvider 调用 Brave Search API（备选 Provider）。
// 官方地址：https://api.search.brave.com，需通过环境变量 WEB_SEARCH_BASE_URL 显式配置。
type BraveProvider struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewBraveProvider 创建 Brave Provider。baseURL 末尾斜杠会被去掉。
func NewBraveProvider(baseURL, apiKey string) *BraveProvider {
	return &BraveProvider{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// Name 返回 Provider 名。
func (p *BraveProvider) Name() string { return "brave" }

// Search 调用 Brave /res/v1/web/search 端点。
// Brave 返回原始 SERP，无内置 answer 与内容清洗，Content 取 description 摘要。
func (p *BraveProvider) Search(ctx context.Context, req *SearchRequest) (*SearchResponse, error) {
	start := time.Now()

	maxResults := req.MaxResults
	if maxResults <= 0 || maxResults > 10 {
		maxResults = 5
	}

	u := p.baseURL + "/res/v1/web/search?" + url.Values{
		"q":           {req.Query},
		"count":       {strconv.Itoa(maxResults)},
		"country":     {"all"},
		"search_lang": {"zh-hans"},
	}.Encode()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("创建 Brave 请求失败: %w", err)
	}
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Accept-Encoding", "gzip")
	httpReq.Header.Set("X-Subscription-Token", p.apiKey)

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("调用 Brave API 失败: %w", err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			return
		}
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("brave API 返回非 200 状态码: %d", resp.StatusCode)
	}

	// Brave Web Search 响应结构（仅解析 web.results）
	var raw struct {
		Web struct {
			Results []struct {
				Title       string  `json:"title"`
				URL         string  `json:"url"`
				Description string  `json:"description"`
				Score       float64 `json:"score,omitempty"`
			} `json:"results"`
		} `json:"web"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("解析 Brave 响应失败: %w", err)
	}

	items := make([]SearchResultItem, 0, len(raw.Web.Results))
	for _, r := range raw.Web.Results {
		items = append(items, SearchResultItem{
			Title:   r.Title,
			URL:     r.URL,
			Content: r.Description,
			Score:   r.Score,
		})
	}

	return &SearchResponse{
		Query:        req.Query,
		Results:      items,
		ResponseTime: time.Since(start).Milliseconds(),
		Provider:     p.Name(),
	}, nil
}
