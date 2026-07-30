package request

// ModelProviderListRequest 模型供应商列表请求
type ModelProviderListRequest struct {
	Page     int    `form:"page" binding:"omitempty,min=1"`
	PageSize int    `form:"page_size" binding:"omitempty,min=1,max=100"`
	Keyword  string `form:"keyword" binding:"omitempty"`
}
