package system

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"Qavor/internal/service"
	"Qavor/pkg/config"
	qerrors "Qavor/pkg/errors"
	"Qavor/pkg/logger"

	"github.com/gin-gonic/gin"
)

func TestMain(m *testing.M) {
	_ = logger.Init(&config.LogConfig{
		Level:      "error",
		Filename:   filepath.Join(os.TempDir(), "qavor-system-controller-test.log"),
		MaxSize:    1,
		MaxBackups: 1,
		MaxAge:     1,
	})
	os.Exit(m.Run())
}

type fakeRAGSettingsService struct {
	settings *service.RAGSettings
	err      error
	updates  []*uint
}

func (s *fakeRAGSettingsService) Get(context.Context) (*service.RAGSettings, error) {
	return s.settings, s.err
}

func (s *fakeRAGSettingsService) UpdateRerankModel(_ context.Context, modelID *uint) (*service.RAGSettings, error) {
	if modelID == nil {
		s.updates = append(s.updates, nil)
	} else {
		id := *modelID
		s.updates = append(s.updates, &id)
	}
	return s.settings, s.err
}

func (s *fakeRAGSettingsService) RerankModelID(context.Context) (uint, bool, error) {
	return 0, false, nil
}

type fakeSystemConfigService struct {
	config  *service.SystemConfig
	err     error
	last    map[string]any
	options []service.ConfigOption
	ocrCfg  service.OCRAPIConfig
}

func (s *fakeSystemConfigService) Get(context.Context) (*service.SystemConfig, error) {
	return s.config, s.err
}

func (s *fakeSystemConfigService) Update(_ context.Context, key string, value any) (*service.SystemConfig, error) {
	s.last = map[string]any{key: value}
	return s.config, s.err
}

func (s *fakeSystemConfigService) UpdateBatch(_ context.Context, values map[string]any) (*service.SystemConfig, error) {
	s.last = values
	return s.config, s.err
}

func (s *fakeSystemConfigService) GetConfigOptions(context.Context) ([]service.ConfigOption, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.options, nil
}

func (s *fakeSystemConfigService) UpdateConfigOption(_ context.Context, key string, value map[string]string) (*service.ConfigOption, error) {
	if s.err != nil {
		return nil, s.err
	}
	for _, option := range s.options {
		if option.Key == key {
			return &option, nil
		}
	}
	return nil, nil
}

func (s *fakeSystemConfigService) GetOCRAPIConfig(context.Context) (service.OCRAPIConfig, error) {
	return s.ocrCfg, s.err
}

func testSystemRouter(svc service.RAGSettingsService, configSvc service.SystemConfigService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	ctrl := NewController(svc, configSvc)
	router.GET("/api/v1/system/rag-settings", ctrl.GetRAGSettings)
	router.PUT("/api/v1/system/rag-settings", ctrl.UpdateRAGSettings)
	router.GET("/api/v1/system/config", ctrl.GetConfig)
	router.POST("/api/v1/system/config", ctrl.UpdateConfig)
	router.POST("/api/v1/system/config/update", ctrl.UpdateConfigBatch)
	router.GET("/api/v1/system/config/options", ctrl.GetConfigOptions)
	router.PUT("/api/v1/system/config/options/:key", ctrl.UpdateConfigOption)
	return router
}

func decodeSystemResponse(t *testing.T, recorder *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("解析响应: %v body=%s", err, recorder.Body.String())
	}
	return body
}

func TestController_GetRAGSettings(t *testing.T) {
	id := uint(7)
	svc := &fakeRAGSettingsService{settings: &service.RAGSettings{RerankModelID: &id, RerankModelName: "bge-reranker-v2-m3"}}
	recorder := httptest.NewRecorder()
	testSystemRouter(svc, &fakeSystemConfigService{}).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/system/rag-settings", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("HTTP 状态=%d body=%s", recorder.Code, recorder.Body.String())
	}
	body := decodeSystemResponse(t, recorder)
	data := body["data"].(map[string]any)
	if data["rerank_model_id"] != float64(7) || data["rerank_model_name"] != "bge-reranker-v2-m3" {
		t.Fatalf("响应 data=%v", data)
	}
}

func TestController_UpdateRAGSettingsAcceptsIDAndNull(t *testing.T) {
	tests := []struct {
		name        string
		body        string
		wantModelID *uint
	}{
		{name: "选择模型", body: `{"rerank_model_id":7}`, wantModelID: systemUintPointer(7)},
		{name: "清空模型", body: `{"rerank_model_id":null}`, wantModelID: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &fakeRAGSettingsService{settings: &service.RAGSettings{}}
			recorder := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPut, "/api/v1/system/rag-settings", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")
			testSystemRouter(svc, &fakeSystemConfigService{}).ServeHTTP(recorder, req)
			if recorder.Code != http.StatusOK || len(svc.updates) != 1 {
				t.Fatalf("HTTP 状态=%d updates=%v body=%s", recorder.Code, svc.updates, recorder.Body.String())
			}
			got := svc.updates[0]
			if tt.wantModelID == nil {
				if got != nil {
					t.Fatalf("更新 ID=%v，期望 nil", *got)
				}
			} else if got == nil || *got != *tt.wantModelID {
				t.Fatalf("更新 ID=%v，期望 %d", got, *tt.wantModelID)
			}
		})
	}
}

func TestController_UpdateRAGSettingsReturnsBusinessErrorShape(t *testing.T) {
	svc := &fakeRAGSettingsService{
		settings: &service.RAGSettings{},
		err:      qerrors.New(qerrors.CodeInvalidParam, "Rerank 模型不存在"),
	}
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/system/rag-settings", bytes.NewBufferString(`{"rerank_model_id":99}`))
	req.Header.Set("Content-Type", "application/json")
	testSystemRouter(svc, &fakeSystemConfigService{}).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("HTTP 状态=%d body=%s", recorder.Code, recorder.Body.String())
	}
	body := decodeSystemResponse(t, recorder)
	if body["code"] != float64(qerrors.CodeInvalidParam) || body["message"] != "Rerank 模型不存在" {
		t.Fatalf("业务错误响应=%v", body)
	}
	if _, found := body["data"]; found {
		t.Fatalf("错误响应不应包含 data: %v", body)
	}
}

func TestController_GetSystemConfig(t *testing.T) {
	svc := &fakeSystemConfigService{config: &service.SystemConfig{
		DefaultModel:       "3",
		EmbedModel:         "2",
		EnableContentGuard: true,
	}}
	recorder := httptest.NewRecorder()
	testSystemRouter(&fakeRAGSettingsService{}, svc).
		ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/system/config", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("HTTP 状态=%d body=%s", recorder.Code, recorder.Body.String())
	}
	body := decodeSystemResponse(t, recorder)
	data := body["data"].(map[string]any)
	if data["default_model"] != "3" || data["embed_model"] != "2" || data["enable_content_guard"] != true {
		t.Fatalf("响应 data=%v", data)
	}
}

func TestController_UpdateSystemConfigBatch(t *testing.T) {
	svc := &fakeSystemConfigService{config: &service.SystemConfig{DefaultModel: "7"}}
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/system/config/update", bytes.NewBufferString(`{"default_model":"7","fast_model":"8"}`))
	req.Header.Set("Content-Type", "application/json")
	testSystemRouter(&fakeRAGSettingsService{}, svc).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("HTTP 状态=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if len(svc.last) != 2 || svc.last["default_model"] != "7" || svc.last["fast_model"] != "8" {
		t.Fatalf("批量更新参数=%v", svc.last)
	}
}

func TestController_UpdateSystemConfigSingle(t *testing.T) {
	svc := &fakeSystemConfigService{config: &service.SystemConfig{}}
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/system/config", bytes.NewBufferString(`{"key":"enable_content_guard","value":true}`))
	req.Header.Set("Content-Type", "application/json")
	testSystemRouter(&fakeRAGSettingsService{}, svc).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("HTTP 状态=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if len(svc.last) != 1 || svc.last["enable_content_guard"] != true {
		t.Fatalf("单键更新参数=%v", svc.last)
	}
}

func TestController_GetConfigOptions(t *testing.T) {
	svc := &fakeSystemConfigService{options: []service.ConfigOption{
		{Key: "ocr.api_opts", Name: "通用 OCR API", Value: map[string]string{}},
	}}
	recorder := httptest.NewRecorder()
	testSystemRouter(&fakeRAGSettingsService{}, svc).
		ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/system/config/options", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("HTTP 状态=%d body=%s", recorder.Code, recorder.Body.String())
	}
	body := decodeSystemResponse(t, recorder)
	data := body["data"].(map[string]any)
	options := data["options"].([]any)
	if len(options) != 1 {
		t.Fatalf("options=%v", data["options"])
	}
	first := options[0].(map[string]any)
	if first["key"] != "ocr.api_opts" || first["name"] != "通用 OCR API" {
		t.Fatalf("option=%v", first)
	}
}

func TestController_UpdateConfigOption(t *testing.T) {
	svc := &fakeSystemConfigService{options: []service.ConfigOption{
		{Key: "ocr.api_opts", Name: "通用 OCR API", Value: map[string]string{}},
	}}
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/system/config/options/ocr.api_opts", bytes.NewBufferString(`{"value":{"base_url":"https://ocr.example.com/v1/recognize","api_key":"sk-123456"}}`))
	req.Header.Set("Content-Type", "application/json")
	testSystemRouter(&fakeRAGSettingsService{}, svc).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("HTTP 状态=%d body=%s", recorder.Code, recorder.Body.String())
	}
	body := decodeSystemResponse(t, recorder)
	data := body["data"].(map[string]any)
	option := data["option"].(map[string]any)
	if option["key"] != "ocr.api_opts" {
		t.Fatalf("option=%v", option)
	}
}

func systemUintPointer(value uint) *uint {
	return &value
}
