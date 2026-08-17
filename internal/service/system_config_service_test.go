package service

import (
	"context"
	"testing"

	"Qavor/internal/model/entity"
	qerrors "Qavor/pkg/errors"
)

func testChatModel(id uint) *entity.Model {
	return &entity.Model{BaseEntity: entity.BaseEntity{ID: id}, Name: "gpt-test", Enabled: true, ModelType: "chat"}
}

func testEmbeddingModel(id uint) *entity.Model {
	return &entity.Model{BaseEntity: entity.BaseEntity{ID: id}, Name: "bge-m3", Enabled: true, ModelType: "embedding"}
}

func newSystemConfigTestService(values map[string]string, models map[uint]*entity.Model) SystemConfigService {
	return NewSystemConfigService(
		&fakeSystemSettingRepository{values: values},
		&fakeRAGSettingsModelRepository{models: models},
	)
}

func TestSystemConfigService_Get(t *testing.T) {
	svc := newSystemConfigTestService(map[string]string{
		SettingKeyDefaultModel:          "3",
		SettingKeyEmbedModel:            "2",
		SettingKeyEnableContentGuard:    "true",
		SettingKeyEnableContentGuardLLM: "false",
	}, nil)

	cfg, err := svc.Get(context.Background())
	if err != nil {
		t.Fatalf("读取系统配置: %v", err)
	}
	if cfg.DefaultModel != "3" || cfg.EmbedModel != "2" {
		t.Fatalf("模型配置=%v", cfg)
	}
	if !cfg.EnableContentGuard || cfg.EnableContentGuardLLM {
		t.Fatalf("布尔配置解析错误=%v", cfg)
	}
	if cfg.ConfigItems == nil || cfg.ConfigItems[SettingKeyDefaultModel].Des == "" {
		t.Fatalf("配置项描述缺失: %v", cfg.ConfigItems)
	}
}

func TestSystemConfigService_GetEmpty(t *testing.T) {
	svc := newSystemConfigTestService(map[string]string{}, nil)
	cfg, err := svc.Get(context.Background())
	if err != nil {
		t.Fatalf("空配置读取: %v", err)
	}
	if cfg.DefaultModel != "" || cfg.EnableContentGuard {
		t.Fatalf("空配置解析错误=%v", cfg)
	}
}

func TestSystemConfigService_UpdateBatch(t *testing.T) {
	svc := newSystemConfigTestService(map[string]string{}, map[uint]*entity.Model{
		3: testChatModel(3),
		2: testEmbeddingModel(2),
	})

	cfg, err := svc.UpdateBatch(context.Background(), map[string]any{
		SettingKeyDefaultModel:       "3",
		SettingKeyEmbedModel:         "2",
		SettingKeyEnableContentGuard: true,
	})
	if err != nil {
		t.Fatalf("批量更新: %v", err)
	}
	if cfg.DefaultModel != "3" || cfg.EmbedModel != "2" || !cfg.EnableContentGuard {
		t.Fatalf("更新后配置=%v", cfg)
	}
}

func TestSystemConfigService_UpdateSingle(t *testing.T) {
	svc := newSystemConfigTestService(map[string]string{}, map[uint]*entity.Model{
		3: testChatModel(3),
	})
	cfg, err := svc.Update(context.Background(), SettingKeyDefaultModel, "3")
	if err != nil {
		t.Fatalf("单键更新: %v", err)
	}
	if cfg.DefaultModel != "3" {
		t.Fatalf("单键更新后配置=%v", cfg)
	}
}

func TestSystemConfigService_UpdateClearsModel(t *testing.T) {
	svc := newSystemConfigTestService(map[string]string{SettingKeyDefaultModel: "3"}, map[uint]*entity.Model{})
	cfg, err := svc.Update(context.Background(), SettingKeyDefaultModel, "")
	if err != nil {
		t.Fatalf("清空模型: %v", err)
	}
	if cfg.DefaultModel != "" {
		t.Fatalf("清空后配置=%v", cfg)
	}
}

func TestSystemConfigService_UpdateRejectsInvalidValue(t *testing.T) {
	tests := []struct {
		name   string
		key    string
		value  any
		models map[uint]*entity.Model
	}{
		{name: "未知配置键", key: "unknown_key", value: "x", models: map[uint]*entity.Model{}},
		{name: "模型不存在", key: SettingKeyDefaultModel, value: "99", models: map[uint]*entity.Model{}},
		{name: "模型未启用", key: SettingKeyDefaultModel, value: "3", models: map[uint]*entity.Model{3: {BaseEntity: entity.BaseEntity{ID: 3}, Enabled: false, ModelType: "chat"}}},
		{name: "类型不匹配", key: SettingKeyEmbedModel, value: "3", models: map[uint]*entity.Model{3: testChatModel(3)}},
		{name: "模型 ID 格式错误", key: SettingKeyDefaultModel, value: "openai/gpt-4", models: map[uint]*entity.Model{}},
		{name: "布尔值非法", key: SettingKeyEnableContentGuard, value: "not-a-bool", models: map[uint]*entity.Model{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newSystemConfigTestService(map[string]string{}, tt.models)
			_, err := svc.Update(context.Background(), tt.key, tt.value)
			if err == nil {
				t.Fatalf("期望拒绝非法配置 key=%s value=%v", tt.key, tt.value)
			}
			if !qerrors.IsBizError(err) {
				t.Fatalf("期望业务错误，实际=%v", err)
			}
		})
	}
}

func TestSystemConfigService_UpdateBatchSupportsOCREngine(t *testing.T) {
	svc := newSystemConfigTestService(map[string]string{}, nil)
	cfg, err := svc.UpdateBatch(context.Background(), map[string]any{
		SettingKeyDefaultOCREngine: "api_ocr",
	})
	if err != nil {
		t.Fatalf("更新默认 OCR 引擎: %v", err)
	}
	if cfg.DefaultOCREngine != "api_ocr" {
		t.Fatalf("更新后默认 OCR 引擎=%q", cfg.DefaultOCREngine)
	}
}

func TestSystemConfigService_GetConfigOptionsEmpty(t *testing.T) {
	svc := newSystemConfigTestService(map[string]string{}, nil)
	options, err := svc.GetConfigOptions(context.Background())
	if err != nil {
		t.Fatalf("读取配置项: %v", err)
	}
	if len(options) != 1 || options[0].Key != SettingKeyOCRAPIOpts {
		t.Fatalf("配置项=%+v", options)
	}
	option := options[0]
	if option.Name == "" || option.Description == "" || len(option.Params.Fields) != 3 {
		t.Fatalf("配置项定义不完整: %+v", option)
	}
	state := option.SensitiveState["api_key"]
	if state.Source != "none" {
		t.Fatalf("未配置时敏感字段状态=%+v", state)
	}
}

func TestSystemConfigService_UpdateConfigOptionStoresAndMasks(t *testing.T) {
	svc := newSystemConfigTestService(map[string]string{}, nil)
	option, err := svc.UpdateConfigOption(context.Background(), SettingKeyOCRAPIOpts, map[string]string{
		"base_url": "https://ocr.example.com/v1/recognize",
		"api_key":  "sk-abcdefgh12345678",
	})
	if err != nil {
		t.Fatalf("更新配置项: %v", err)
	}
	if option.Value["base_url"] != "https://ocr.example.com/v1/recognize" {
		t.Fatalf("更新后 value=%v", option.Value)
	}
	state := option.SensitiveState["api_key"]
	if state.Source != "database" || state.Preview == "" || state.Preview == "sk-abcdefgh12345678" {
		t.Fatalf("敏感字段状态=%+v", state)
	}
	if len(state.Preview) > 12 {
		t.Fatalf("预览脱敏过长: %q", state.Preview)
	}
}

func TestSystemConfigService_UpdateConfigOptionUnknownKey(t *testing.T) {
	svc := newSystemConfigTestService(map[string]string{}, nil)
	_, err := svc.UpdateConfigOption(context.Background(), "not_exist_opts", map[string]string{"a": "b"})
	if err == nil || !qerrors.IsBizError(err) {
		t.Fatalf("期望拒绝未知配置项，实际=%v", err)
	}
}

func TestSystemConfigService_GetOCRAPIConfigPrecedence(t *testing.T) {
	t.Run("数据库优先", func(t *testing.T) {
		svc := newSystemConfigTestService(map[string]string{
			SettingKeyOCRAPIOpts: `{"base_url":"https://db.example.com","api_key":"db-key"}`,
		}, nil)
		cfg, err := svc.GetOCRAPIConfig(context.Background())
		if err != nil {
			t.Fatalf("读取 OCR 配置: %v", err)
		}
		if cfg.BaseURL != "https://db.example.com" || cfg.APIKey != "db-key" {
			t.Fatalf("OCR 配置=%+v", cfg)
		}
	})
	t.Run("环境变量回退", func(t *testing.T) {
		t.Setenv(EnvOCRAPIBaseURL, "https://env.example.com")
		t.Setenv(EnvOCRAPIKey, "env-key")
		svc := newSystemConfigTestService(map[string]string{}, nil)
		cfg, err := svc.GetOCRAPIConfig(context.Background())
		if err != nil {
			t.Fatalf("读取 OCR 配置: %v", err)
		}
		if cfg.BaseURL != "https://env.example.com" || cfg.APIKey != "env-key" {
			t.Fatalf("OCR 配置=%+v", cfg)
		}
	})
	t.Run("数据库缺省字段回退环境变量", func(t *testing.T) {
		t.Setenv(EnvOCRAPIKey, "env-key")
		svc := newSystemConfigTestService(map[string]string{
			SettingKeyOCRAPIOpts: `{"base_url":"https://db.example.com"}`,
		}, nil)
		cfg, err := svc.GetOCRAPIConfig(context.Background())
		if err != nil {
			t.Fatalf("读取 OCR 配置: %v", err)
		}
		if cfg.BaseURL != "https://db.example.com" || cfg.APIKey != "env-key" {
			t.Fatalf("OCR 配置=%+v", cfg)
		}
	})
}
