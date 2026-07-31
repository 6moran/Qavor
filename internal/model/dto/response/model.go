package response

import "time"

// ModelParams 模型默认推理参数
type ModelParams struct {
	MaxTokens        int      `json:"max_tokens"`
	Temperature      float64  `json:"temperature"`
	TopP             float64  `json:"top_p"`
	PresencePenalty  float64  `json:"presence_penalty"`
	FrequencyPenalty float64  `json:"frequency_penalty"`
	Stop             []string `json:"stop,omitempty"`
}

// ModelResponse 模型响应
type ModelResponse struct {
	ID             uint              `json:"id"`
	Name           string            `json:"name"`
	Protocol       string            `json:"protocol"`
	BaseURL        string            `json:"base_url"`
	OrganizationID string            `json:"org_id,omitempty"`
	Headers        map[string]string `json:"headers,omitempty"`
	Timeout        int               `json:"timeout"`
	Enabled        bool              `json:"enabled"`
	ModelType      string            `json:"model_type"`
	Params         ModelParams       `json:"params"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
}

// ModelListResponse 模型列表响应
type ModelListResponse struct {
	Total int64           `json:"total"`
	Items []ModelResponse `json:"items"`
}
