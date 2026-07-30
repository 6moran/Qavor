package response

// ModelProviderListResponse 模型供应商列表响应
type ModelProviderListResponse struct {
	Total int64                   `json:"total"`
	Items []ModelProviderResponse `json:"items"`
}

// ModelProviderResponse 模型供应商响应
type ModelProviderResponse struct {
	ID        uint   `json:"id"`
	Name      string `json:"name"`
	Protocol  string `json:"protocol"`
	BaseURL   string `json:"base_url"`
	Timeout   int    `json:"timeout"`
	Enabled   bool   `json:"enabled"`
}
