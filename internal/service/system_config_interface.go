package service

import "context"

// 系统配置键（与前端 configStore 使用的 key 保持一致）。
const (
	// SettingKeyDefaultModel 默认对话模型。
	SettingKeyDefaultModel = "default_model"
	// SettingKeyFastModel 快速对话模型。
	SettingKeyFastModel = "fast_model"
	// SettingKeyEmbedModel 嵌入模型。
	SettingKeyEmbedModel = "embed_model"
	// SettingKeyEnableContentGuard 是否启用内容审查。
	SettingKeyEnableContentGuard = "enable_content_guard"
	// SettingKeyEnableContentGuardLLM 是否启用 LLM 内容审查。
	SettingKeyEnableContentGuardLLM = "enable_content_guard_llm"
	// SettingKeyContentGuardLLMModel 内容审查使用的模型。
	SettingKeyContentGuardLLMModel = "content_guard_llm_model"
	// SettingKeyDefaultOCREngine 默认 OCR 引擎（rapid_ocr / api_ocr 等）。
	SettingKeyDefaultOCREngine = "default_ocr_engine"
	// SettingKeyMCPRetrievalEmbedModel MCP 工具向量检索使用的 embedding 模型（仅用于向量检索）。
	SettingKeyMCPRetrievalEmbedModel = "mcp_retrieval_embed_model"
)

// SystemConfigItem 描述一个配置项（前端设置页用于渲染表单说明）。
type SystemConfigItem struct {
	Des string `json:"des"`
}

// SystemConfig 表示可公开的系统级配置，结构即 /api/v1/system/config 响应体。
type SystemConfig struct {
	DefaultModel           string                      `json:"default_model"`
	FastModel              string                      `json:"fast_model"`
	EmbedModel             string                      `json:"embed_model"`
	MCPRetrievalEmbedModel string                      `json:"mcp_retrieval_embed_model"`
	EnableContentGuard     bool                        `json:"enable_content_guard"`
	EnableContentGuardLLM  bool                        `json:"enable_content_guard_llm"`
	ContentGuardLLMModel   string                      `json:"content_guard_llm_model"`
	DefaultOCREngine       string                      `json:"default_ocr_engine"`
	ConfigItems            map[string]SystemConfigItem `json:"_config_items,omitempty"`
}

// SystemConfigService 管理系统级配置（默认模型、内容审查等）。
type SystemConfigService interface {
	// Get 读取全部系统配置。
	Get(ctx context.Context) (*SystemConfig, error)
	// Update 更新单个配置项并返回最新配置。
	Update(ctx context.Context, key string, value any) (*SystemConfig, error)
	// UpdateBatch 批量更新配置项并返回最新配置。
	UpdateBatch(ctx context.Context, values map[string]any) (*SystemConfig, error)
	// GetConfigOptions 返回全部可配置项定义（前端配置表单渲染用）。
	GetConfigOptions(ctx context.Context) ([]ConfigOption, error)
	// UpdateConfigOption 更新单个配置项并返回更新后的定义。
	UpdateConfigOption(ctx context.Context, key string, value map[string]string) (*ConfigOption, error)
	// GetOCRAPIConfig 读取通用 OCR API 的有效配置（数据库优先，环境变量回退）。
	GetOCRAPIConfig(ctx context.Context) (OCRAPIConfig, error)
	// SetMCPRetrievalModelChangeCallback 设置 MCP 工具向量检索模型变更回调。
	// 更新 mcp_retrieval_embed_model 配置后调用（用于触发向量索引清空/重建）。
	SetMCPRetrievalModelChangeCallback(cb func())
}

// SensitiveState 描述敏感字段的值来源与脱敏预览。
type SensitiveState struct {
	// Source: "none" 未配置 / "database" 数据库 / "environment" 环境变量。
	Source  string `json:"source"`
	Preview string `json:"preview,omitempty"`
}

// ConfigOptionField 描述配置表单中的一个字段。
type ConfigOptionField struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Sensitive   bool   `json:"sensitive"`
	Environment string `json:"environment"`
	Placeholder string `json:"placeholder,omitempty"`
	Help        string `json:"help,omitempty"`
}

// ConfigOptionParams 描述配置表单的字段集合。
type ConfigOptionParams struct {
	Fields []ConfigOptionField `json:"fields"`
}

// ConfigOption 描述一个可配置项（结构对齐前端 OCRSettingsSection 渲染契约）。
type ConfigOption struct {
	Key            string                    `json:"key"`
	Name           string                    `json:"name"`
	Description    string                    `json:"description"`
	Value          map[string]string         `json:"value"`
	SensitiveState map[string]SensitiveState `json:"sensitive_state"`
	Params         ConfigOptionParams        `json:"params"`
}

// OCRAPIConfig 通用 OCR API 的有效配置。
type OCRAPIConfig struct {
	BaseURL string `json:"base_url"`
	APIKey  string `json:"api_key"`
	Model   string `json:"model"`
}
