package websearch

import "context"

// SearchRequest 搜索请求（统一抽象，与 Provider 无关）。
type SearchRequest struct {
	Query      string `json:"query"`
	MaxResults int    `json:"max_results,omitempty"` // 默认 5，范围 1-10
	Topic      string `json:"topic,omitempty"`       // general | news
}

// SearchResultItem 搜索结果项（统一格式，前端 WebSearchTool.vue 直接消费）。
type SearchResultItem struct {
	Title   string  `json:"title"`           // 网页标题
	URL     string  `json:"url"`             // 网页地址
	Content string  `json:"content"`         // 内容摘要
	Score   float64 `json:"score,omitempty"` // 相关度评分 (0-1)
}

// SearchResponse 搜索响应（统一格式）。
type SearchResponse struct {
	Query        string             `json:"query"`
	Results      []SearchResultItem `json:"results"`
	Answer       string             `json:"answer,omitempty"`   // LLM 综合答案（如 Provider 提供）
	ResponseTime int64              `json:"response_time"`      // 耗时(ms)
	Provider     string             `json:"provider,omitempty"` // 实际使用的 Provider 名
}

// SearchProvider 搜索引擎 Provider 接口，屏蔽不同厂商 API 差异。
type SearchProvider interface {
	Search(ctx context.Context, req *SearchRequest) (*SearchResponse, error)
	Name() string
}
