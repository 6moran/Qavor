package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"Qavor/internal/model/entity"
	"Qavor/internal/repository"
	qerrors "Qavor/pkg/errors"
)

// systemConfigModelRepository 是 SystemConfigService 依赖的模型查询能力。
type systemConfigModelRepository interface {
	FindByID(id uint) (*entity.Model, error)
}

// modelTypeByConfigKey 模型类配置项要求的模型类型，空串表示不限制。
var modelTypeByConfigKey = map[string]string{
	SettingKeyDefaultModel:         "chat",
	SettingKeyFastModel:            "chat",
	SettingKeyContentGuardLLMModel: "chat",
	SettingKeyEmbedModel:           "embedding",
}

// boolConfigKeys 布尔类配置项白名单。
var boolConfigKeys = map[string]bool{
	SettingKeyEnableContentGuard:    true,
	SettingKeyEnableContentGuardLLM: true,
}

// modelConfigKeys 模型类配置项白名单。
var modelConfigKeys = map[string]bool{
	SettingKeyDefaultModel:         true,
	SettingKeyFastModel:            true,
	SettingKeyEmbedModel:           true,
	SettingKeyContentGuardLLMModel: true,
}

// plainStringConfigKeys 普通字符串配置项白名单（不校验模型/布尔，直接存储）。
var plainStringConfigKeys = map[string]bool{
	SettingKeyDefaultOCREngine: true,
}

// OCR 配置项键与配置项定义。
const (
	// SettingKeyOCRAPIOpts 通用 OCR API 配置项（JSON：{"base_url": "...", "api_key": "...", "model": "..."}）。
	// 与其他配置项统一使用下划线命名（如 default_ocr_engine），避免前端键名不一致导致更新被拒。
	SettingKeyOCRAPIOpts = "ocr_api_opts"
	// EnvOCRAPIBaseURL / EnvOCRAPIKey / EnvOCRAPIModel 通用 OCR API 的环境变量回退。
	EnvOCRAPIBaseURL = "QAVOR_OCR_API_BASE_URL"
	EnvOCRAPIKey     = "QAVOR_OCR_API_KEY"
	EnvOCRAPIModel   = "QAVOR_OCR_MODEL"
)

// buildConfigOptions 返回全部可配置项定义（注册表，新增引擎在此扩展）。
func buildConfigOptions() []ConfigOption {
	return []ConfigOption{
		{
			Key:         SettingKeyOCRAPIOpts,
			Name:        "通用 OCR API",
			Description: "通过 HTTP 接口调用外部 OCR 服务（如硅基流动等平台）。PDF 由系统逐页渲染后上传识别，图片直接上传。",
			Value:       map[string]string{},
			SensitiveState: map[string]SensitiveState{
				"api_key": {Source: "none"},
			},
			Params: ConfigOptionParams{
				Fields: []ConfigOptionField{
					{
						Key:         "base_url",
						Label:       "服务地址",
						Environment: EnvOCRAPIBaseURL,
						Placeholder: "https://api.example.com/v1/ocr",
						Help:        "接收图片上传的 OCR 接口地址，图片以 multipart 文件字段 image 提交。",
					},
					{
						Key:         "api_key",
						Label:       "API Key",
						Sensitive:   true,
						Environment: EnvOCRAPIKey,
						Help:        "接口访问凭证，随请求头 Authorization: Bearer <key> 发送。",
					},
					{
						Key:         "model",
						Label:       "模型名称",
						Environment: EnvOCRAPIModel,
						Placeholder: "Qwen2.5-VL-72B-Instruct",
						Help:        "使用哪个 OCR/视觉模型，随请求以表单字段 model 提交；部分服务可不填。",
					},
				},
			},
		},
	}
}

type systemConfigService struct {
	settingsRepo repository.SystemSettingRepository
	modelRepo    systemConfigModelRepository
}

// NewSystemConfigService 创建系统配置服务。
func NewSystemConfigService(settingsRepo repository.SystemSettingRepository, modelRepo systemConfigModelRepository) SystemConfigService {
	return &systemConfigService{settingsRepo: settingsRepo, modelRepo: modelRepo}
}

// Get 读取全部系统配置。
func (s *systemConfigService) Get(ctx context.Context) (*SystemConfig, error) {
	cfg := &SystemConfig{ConfigItems: buildSystemConfigItems()}

	var err error
	if cfg.DefaultModel, _, err = s.settingsRepo.Get(ctx, SettingKeyDefaultModel); err != nil {
		return nil, wrapSettingReadError(err)
	}
	if cfg.FastModel, _, err = s.settingsRepo.Get(ctx, SettingKeyFastModel); err != nil {
		return nil, wrapSettingReadError(err)
	}
	if cfg.EmbedModel, _, err = s.settingsRepo.Get(ctx, SettingKeyEmbedModel); err != nil {
		return nil, wrapSettingReadError(err)
	}
	if cfg.ContentGuardLLMModel, _, err = s.settingsRepo.Get(ctx, SettingKeyContentGuardLLMModel); err != nil {
		return nil, wrapSettingReadError(err)
	}

	for _, key := range []string{SettingKeyEnableContentGuard, SettingKeyEnableContentGuardLLM} {
		value, _, readErr := s.settingsRepo.Get(ctx, key)
		if readErr != nil {
			return nil, wrapSettingReadError(readErr)
		}
		parsed, parseErr := strconv.ParseBool(strings.TrimSpace(value))
		if parseErr != nil {
			parsed = false
		}
		if key == SettingKeyEnableContentGuard {
			cfg.EnableContentGuard = parsed
		} else {
			cfg.EnableContentGuardLLM = parsed
		}
	}

	if cfg.DefaultOCREngine, _, err = s.settingsRepo.Get(ctx, SettingKeyDefaultOCREngine); err != nil {
		return nil, wrapSettingReadError(err)
	}

	return cfg, nil
}

// Update 更新单个配置项并返回最新配置。
func (s *systemConfigService) Update(ctx context.Context, key string, value any) (*SystemConfig, error) {
	return s.UpdateBatch(ctx, map[string]any{key: value})
}

// UpdateBatch 批量更新配置项并返回最新配置。
func (s *systemConfigService) UpdateBatch(ctx context.Context, values map[string]any) (*SystemConfig, error) {
	for key, value := range values {
		normalized, err := s.normalizeValue(key, value)
		if err != nil {
			return nil, err
		}
		if err := s.settingsRepo.Upsert(ctx, key, normalized); err != nil {
			return nil, fmt.Errorf("更新系统配置 %s: %w", key, err)
		}
	}
	return s.Get(ctx)
}

// GetConfigOptions 返回全部可配置项定义，并从数据库/环境变量回填当前值。
func (s *systemConfigService) GetConfigOptions(ctx context.Context) ([]ConfigOption, error) {
	options := buildConfigOptions()
	for i := range options {
		if err := s.fillOption(ctx, &options[i]); err != nil {
			return nil, err
		}
	}
	return options, nil
}

// UpdateConfigOption 更新单个配置项。空值字段会从存储中清除（回退环境变量）。
func (s *systemConfigService) UpdateConfigOption(ctx context.Context, key string, value map[string]string) (*ConfigOption, error) {
	option, err := s.findConfigOption(key)
	if err != nil {
		return nil, err
	}
	stored := make(map[string]string)
	for _, field := range option.Params.Fields {
		raw := strings.TrimSpace(value[field.Key])
		if raw != "" {
			stored[field.Key] = raw
		}
	}
	raw, err := json.Marshal(stored)
	if err != nil {
		return nil, qerrors.New(qerrors.CodeInvalidParam, fmt.Sprintf("%s 配置无效: 无法序列化", key))
	}
	if err := s.settingsRepo.Upsert(ctx, key, string(raw)); err != nil {
		return nil, fmt.Errorf("更新配置 %s: %w", key, err)
	}
	if err := s.fillOption(ctx, option); err != nil {
		return nil, err
	}
	return option, nil
}

// GetOCRAPIConfig 读取通用 OCR API 的有效配置：数据库优先，缺失字段回退环境变量。
func (s *systemConfigService) GetOCRAPIConfig(ctx context.Context) (OCRAPIConfig, error) {
	cfg := OCRAPIConfig{}
	raw, found, err := s.settingsRepo.Get(ctx, SettingKeyOCRAPIOpts)
	if err != nil {
		return cfg, wrapSettingReadError(err)
	}
	if found && raw != "" {
		_ = json.Unmarshal([]byte(raw), &cfg) // 结构损坏时忽略，走环境变量回退
	}
	if strings.TrimSpace(cfg.BaseURL) == "" {
		cfg.BaseURL = strings.TrimSpace(os.Getenv(EnvOCRAPIBaseURL))
	}
	if strings.TrimSpace(cfg.APIKey) == "" {
		cfg.APIKey = strings.TrimSpace(os.Getenv(EnvOCRAPIKey))
	}
	if strings.TrimSpace(cfg.Model) == "" {
		cfg.Model = strings.TrimSpace(os.Getenv(EnvOCRAPIModel))
	}
	return cfg, nil
}

// findConfigOption 按 key 查找配置项定义。
func (s *systemConfigService) findConfigOption(key string) (*ConfigOption, error) {
	options := buildConfigOptions()
	for i := range options {
		if options[i].Key == key {
			return &options[i], nil
		}
	}
	return nil, qerrors.New(qerrors.CodeInvalidParam, fmt.Sprintf("未知的配置项: %s", key))
}

// fillOption 将数据库/环境变量中的值回填到配置项定义。
func (s *systemConfigService) fillOption(ctx context.Context, option *ConfigOption) error {
	stored := map[string]string{}
	raw, found, err := s.settingsRepo.Get(ctx, option.Key)
	if err != nil {
		return wrapSettingReadError(err)
	}
	if found && raw != "" {
		_ = json.Unmarshal([]byte(raw), &stored)
	}
	for _, field := range option.Params.Fields {
		option.Value[field.Key] = stored[field.Key]
		if !field.Sensitive {
			continue
		}
		state := SensitiveState{Source: "none"}
		if stored[field.Key] != "" {
			state.Source = "database"
			state.Preview = maskSecret(stored[field.Key])
		} else if strings.TrimSpace(os.Getenv(field.Environment)) != "" {
			state.Source = "environment"
		}
		option.SensitiveState[field.Key] = state
	}
	return nil
}

// maskSecret 将敏感值脱敏为预览文本，如 "sk-abc***fghi"。
func maskSecret(value string) string {
	const maxPrefix, maxSuffix = 4, 4
	if len(value) <= maxPrefix+maxSuffix+3 {
		return strings.Repeat("*", 6)
	}
	prefix := value[:maxPrefix]
	suffix := value[len(value)-maxSuffix:]
	return prefix + "***" + suffix
}

// normalizeValue 校验配置键并归一化值为存储字符串。
func (s *systemConfigService) normalizeValue(key string, value any) (string, error) {
	if modelConfigKeys[key] {
		raw := strings.TrimSpace(fmt.Sprint(value))
		if raw == "" {
			return "", nil // 允许清空默认模型
		}
		if err := s.validateModel(key, raw); err != nil {
			return "", err
		}
		return raw, nil
	}
	if boolConfigKeys[key] {
		return normalizeBoolValue(key, value)
	}
	if plainStringConfigKeys[key] {
		raw := strings.TrimSpace(fmt.Sprint(value))
		return raw, nil
	}
	if key == SettingKeyOCRAPIOpts {
		return normalizeJSONValue(key, value)
	}
	return "", qerrors.New(qerrors.CodeInvalidParam, fmt.Sprintf("未知的配置项: %s", key))
}

// normalizeJSONValue 将任意 JSON 可序列化值归一化为紧凑 JSON 字符串。
func normalizeJSONValue(key string, value any) (string, error) {
	switch v := value.(type) {
	case nil:
		return "{}", nil
	case string:
		if strings.TrimSpace(v) == "" {
			return "{}", nil
		}
		var probe any
		if err := json.Unmarshal([]byte(v), &probe); err != nil {
			return "", qerrors.New(qerrors.CodeInvalidParam, fmt.Sprintf("%s 配置无效: 需要 JSON 对象", key))
		}
		return v, nil
	default:
		raw, err := json.Marshal(value)
		if err != nil {
			return "", qerrors.New(qerrors.CodeInvalidParam, fmt.Sprintf("%s 配置无效: 无法序列化", key))
		}
		return string(raw), nil
	}
}

// validateModel 校验模型 ID 存在、已启用且类型匹配。
func (s *systemConfigService) validateModel(key, raw string) error {
	id, err := strconv.ParseUint(raw, 10, 32)
	if err != nil {
		return qerrors.New(qerrors.CodeInvalidParam, fmt.Sprintf("%s 配置无效: 模型 ID 格式错误", key))
	}
	model, err := s.modelRepo.FindByID(uint(id))
	if err != nil {
		return fmt.Errorf("读取模型 %s: %w", key, err)
	}
	if model == nil {
		return qerrors.New(qerrors.CodeInvalidParam, fmt.Sprintf("%s 配置无效: 模型不存在", key))
	}
	if !model.Enabled {
		return qerrors.New(qerrors.CodeInvalidParam, fmt.Sprintf("%s 配置无效: 模型未启用", key))
	}
	if expected := modelTypeByConfigKey[key]; expected != "" && model.ModelType != expected {
		return qerrors.New(qerrors.CodeInvalidParam, fmt.Sprintf("%s 配置无效: 需要 %s 类型模型，当前为 %s", key, expected, model.ModelType))
	}
	return nil
}

// normalizeBoolValue 将布尔类配置值归一化为 "true"/"false"。
func normalizeBoolValue(key string, value any) (string, error) {
	var parsed bool
	switch v := value.(type) {
	case bool:
		parsed = v
	case string:
		b, err := strconv.ParseBool(strings.TrimSpace(v))
		if err != nil {
			return "", qerrors.New(qerrors.CodeInvalidParam, fmt.Sprintf("%s 配置无效: 需要布尔值", key))
		}
		parsed = b
	case float64: // JSON 数字被反序列化为 float64
		parsed = v != 0
	case nil:
		parsed = false
	default:
		return "", qerrors.New(qerrors.CodeInvalidParam, fmt.Sprintf("%s 配置无效: 需要布尔值", key))
	}
	return strconv.FormatBool(parsed), nil
}

// buildSystemConfigItems 返回各配置项的描述信息（前端设置页渲染用）。
func buildSystemConfigItems() map[string]SystemConfigItem {
	return map[string]SystemConfigItem{
		SettingKeyDefaultModel:          {Des: "默认对话模型"},
		SettingKeyFastModel:             {Des: "快速对话模型"},
		SettingKeyEmbedModel:            {Des: "嵌入模型"},
		SettingKeyEnableContentGuard:    {Des: "内容审查"},
		SettingKeyEnableContentGuardLLM: {Des: "内容审查 LLM"},
		SettingKeyContentGuardLLMModel:  {Des: "内容审查模型"},
		SettingKeyDefaultOCREngine:      {Des: "默认 OCR 方法"},
	}
}

func wrapSettingReadError(err error) error {
	return fmt.Errorf("读取系统配置: %w", err)
}
