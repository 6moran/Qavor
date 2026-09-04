package ocr

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"Qavor/internal/ingestion"
	"Qavor/internal/service"

	"github.com/gin-gonic/gin"
)

type fakeOCRConfigProvider struct{}

func (fakeOCRConfigProvider) GetOCRAPIConfig(context.Context) (service.OCRAPIConfig, error) {
	return service.OCRAPIConfig{}, nil
}

type fakeParserHealthProvider struct {
	health ingestion.ParserHealth
}

func (p fakeParserHealthProvider) Health() ingestion.ParserHealth { return p.health }

func TestGetHealthReportsLiveParserPool(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name       string
		health     ingestion.ParserHealth
		wantStatus string
		wantRapid  string
	}{
		{
			name: "healthy pool",
			health: ingestion.ParserHealth{
				ConfiguredWorkers: 2,
				AvailableWorkers:  2,
				Capabilities:      map[string]bool{"docling": true, "rapidocr": true, "api_ocr": true},
			},
			wantStatus: "healthy",
			wantRapid:  "configured",
		},
		{
			name: "degraded when a worker is missing",
			health: ingestion.ParserHealth{
				ConfiguredWorkers: 2,
				AvailableWorkers:  1,
				Capabilities:      map[string]bool{"docling": true, "rapidocr": true, "api_ocr": false},
			},
			wantStatus: "degraded",
			wantRapid:  "configured",
		},
		{
			name: "degraded when docling is unavailable",
			health: ingestion.ParserHealth{
				ConfiguredWorkers: 2,
				AvailableWorkers:  2,
				Capabilities:      map[string]bool{"docling": false, "rapidocr": true, "api_ocr": false},
			},
			wantStatus: "degraded",
			wantRapid:  "configured",
		},
		{
			name: "unavailable without workers",
			health: ingestion.ParserHealth{
				ConfiguredWorkers: 2,
				AvailableWorkers:  0,
				Capabilities:      map[string]bool{"docling": false, "rapidocr": false, "api_ocr": false},
			},
			wantStatus: "unavailable",
			wantRapid:  "unavailable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			controller := NewController(fakeOCRConfigProvider{}, fakeParserHealthProvider{health: tt.health})
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/system/ocr/health", nil)

			controller.GetHealth(ctx)

			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
			}
			parserPool := decodeParserPool(t, recorder)
			if got := parserPool["status"]; got != tt.wantStatus {
				t.Fatalf("parser_pool status = %q, want %q", got, tt.wantStatus)
			}
			if got := decodeHealth(t, recorder)["rapid_ocr"]["status"]; got != tt.wantRapid {
				t.Fatalf("rapid_ocr status = %q, want %q", got, tt.wantRapid)
			}
		})
	}
}

func TestGetHealthReportsUnavailablePoolWithoutProvider(t *testing.T) {
	gin.SetMode(gin.TestMode)
	controller := NewController(fakeOCRConfigProvider{})
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/system/ocr/health", nil)

	controller.GetHealth(ctx)

	parserPool := decodeParserPool(t, recorder)
	if parserPool["status"] != "unavailable" || parserPool["available_workers"].(float64) != 0 {
		t.Fatalf("parser_pool = %#v, want unavailable with no workers", parserPool)
	}
}

func decodeParserPool(t *testing.T, recorder *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode health response: %v", err)
	}
	pool, ok := body["parser_pool"].(map[string]any)
	if !ok {
		t.Fatalf("parser_pool = %#v, want object", body["parser_pool"])
	}
	return pool
}

func decodeHealth(t *testing.T, recorder *httptest.ResponseRecorder) map[string]map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode health response: %v", err)
	}
	rawHealth, ok := body["health"].(map[string]any)
	if !ok {
		t.Fatalf("health = %#v, want object", body["health"])
	}
	health := make(map[string]map[string]any, len(rawHealth))
	for engine, raw := range rawHealth {
		entry, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("health[%q] = %#v, want object", engine, raw)
		}
		health[engine] = entry
	}
	return health
}
