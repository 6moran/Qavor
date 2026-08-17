package service

import (
	"context"

	"Qavor/internal/model/dto/request"
)

// KnowledgeQueryService 知识库检索测试与示例问题服务。
type KnowledgeQueryService interface {
	// QueryTest 执行检索测试。meta 为可选的检索参数覆盖，不修改知识库配置。
	QueryTest(ctx context.Context, kbID, query string, meta map[string]any) (*RAGRetrieveResult, error)
	// GetQueryParams 返回检索参数选项定义，default 为知识库当前保存值或系统默认。
	GetQueryParams(kbID string) (*QueryParamsPayload, error)
	// UpdateQueryParams 更新知识库检索参数（白名单过滤后持久化）。
	UpdateQueryParams(kbID string, values map[string]any) error
	// GetSampleQuestions 获取已生成的示例问题；未生成时返回 404 业务错误。
	GetSampleQuestions(kbID string) (*SampleQuestionsPayload, error)
	// GenerateSampleQuestions 基于知识库名称/描述与文件标题生成示例问题并持久化。
	GenerateSampleQuestions(ctx context.Context, kbID string, count int) (*SampleQuestionsPayload, error)
	// GenerateDescription 生成或润色知识库描述（新建/编辑表单的 AI 润色）。
	// 请求必须携带 ChatModelID（由前端表单预填/选择传入）；缺失时返回明确提示。
	// current_description 为空时生成新描述，非空时在保留原意基础上润色；返回结果不持久化。
	GenerateDescription(ctx context.Context, req *request.GenerateDescriptionRequest) (string, error)
}

// QueryParamChoice select 型参数的可选值。
type QueryParamChoice struct {
	Value any    `json:"value"`
	Label string `json:"label"`
}

// QueryParamOption 检索参数选项定义，与前端 SearchConfigPanel 渲染契约一致。
type QueryParamOption struct {
	Key         string             `json:"key"`
	Label       string             `json:"label"`
	Type        string             `json:"type"`
	Default     any                `json:"default"`
	Min         *float64           `json:"min,omitempty"`
	Max         *float64           `json:"max,omitempty"`
	Step        *float64           `json:"step,omitempty"`
	Description string             `json:"description,omitempty"`
	Options     []QueryParamChoice `json:"options,omitempty"`
}

// QueryParamsView 检索参数视图。
type QueryParamsView struct {
	Options []QueryParamOption `json:"options"`
}

// QueryParamsPayload 检索参数响应。
type QueryParamsPayload struct {
	Params QueryParamsView `json:"params"`
}

// SampleQuestionsPayload 示例问题响应。
type SampleQuestionsPayload struct {
	Questions []string `json:"questions"`
}
