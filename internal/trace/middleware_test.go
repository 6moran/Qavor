package trace

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestMiddlewareEndsHTTPSpan(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newFakeRepository()
	tracer := NewTracer(repo, Config{Enabled: true, TracedRoutes: []string{"POST /api/v1/agent/runs"}})
	r := gin.New()
	r.Use(Middleware(tracer))
	r.POST("/api/v1/agent/runs", func(c *gin.Context) { c.Status(http.StatusCreated) })

	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/runs", nil)
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)

	if len(repo.started) == 0 {
		t.Fatal("no span started")
	}
	if repo.started[0].Operation != "http.server" {
		t.Fatalf("operation = %q, want http.server", repo.started[0].Operation)
	}
	if repo.started[0].Kind != "http" {
		t.Fatalf("kind = %q, want http", repo.started[0].Kind)
	}
	if len(repo.ends) != 1 {
		t.Fatalf("expected 1 end, got %d", len(repo.ends))
	}
	end := repo.ends[0].end
	if end.Status != SpanStatusOK {
		t.Fatalf("status = %q, want %q", end.Status, SpanStatusOK)
	}
	if end.Attributes["http.status_code"] != http.StatusCreated {
		t.Fatalf("http.status_code = %v, want %d", end.Attributes["http.status_code"], http.StatusCreated)
	}
}

func TestMiddlewareCreatesTraceRecord(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newFakeRepository()
	tracer := NewTracer(repo, Config{Enabled: true, TracedRoutes: []string{"POST /api/v1/chat"}})
	r := gin.New()
	r.Use(Middleware(tracer))
	r.POST("/api/v1/chat", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })

	req := httptest.NewRequest(http.MethodPost, "/api/v1/chat", nil)
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)

	if len(repo.records) != 1 {
		t.Fatalf("expected 1 trace record, got %d", len(repo.records))
	}
}

func TestMiddlewareNonTracedRouteSkipped(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newFakeRepository()
	tracer := NewTracer(repo, Config{Enabled: true, TracedRoutes: []string{"POST /api/v1/chat"}})
	r := gin.New()
	r.Use(Middleware(tracer))
	r.GET("/api/v1/health", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)

	if len(repo.started) != 0 || len(repo.records) != 0 {
		t.Fatalf("non-traced route should not create spans/records, started=%d records=%d", len(repo.started), len(repo.records))
	}
}

func TestMiddlewareDisabledTracerNoOp(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newFakeRepository()
	tracer := NewTracer(repo, Config{Enabled: false, TracedRoutes: []string{"POST /api/v1/chat"}})
	r := gin.New()
	r.Use(Middleware(tracer))
	r.POST("/api/v1/chat", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodPost, "/api/v1/chat", nil)
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)

	if len(repo.started) != 0 {
		t.Fatalf("disabled tracer should not create spans, started=%d", len(repo.started))
	}
}

func TestMiddlewareNilTracerNoOp(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Middleware(nil))
	r.POST("/api/v1/chat", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodPost, "/api/v1/chat", nil)
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)
	// should not panic
}

func TestMiddlewareInvalidTraceIDGeneratesNew(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newFakeRepository()
	tracer := NewTracer(repo, Config{Enabled: true, TracedRoutes: []string{"POST /api/v1/chat"}})
	r := gin.New()
	r.Use(Middleware(tracer))
	r.POST("/api/v1/chat", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodPost, "/api/v1/chat", nil)
	req.Header.Set("X-Trace-Id", "not-a-uuid")
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)

	if len(repo.records) != 1 {
		t.Fatalf("expected 1 trace record, got %d", len(repo.records))
	}
	for tid := range repo.records {
		if !ValidTraceID(tid) {
			t.Fatalf("generated trace_id %q is not a valid UUID", tid)
		}
	}
}

func TestMiddlewareValidTraceIDUsed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newFakeRepository()
	tracer := NewTracer(repo, Config{Enabled: true, TracedRoutes: []string{"POST /api/v1/chat"}})
	r := gin.New()
	r.Use(Middleware(tracer))
	r.POST("/api/v1/chat", func(c *gin.Context) { c.Status(http.StatusOK) })

	wantTraceID := "11111111-1111-1111-1111-111111111111"
	req := httptest.NewRequest(http.MethodPost, "/api/v1/chat", nil)
	req.Header.Set("X-Trace-Id", wantTraceID)
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)

	if len(repo.records) != 1 {
		t.Fatalf("expected 1 trace record, got %d", len(repo.records))
	}
	if repo.records[wantTraceID] == nil {
		t.Fatalf("trace record with trace_id=%s not found", wantTraceID)
	}
}

func TestMiddlewarePanicEndsSpanAsError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newFakeRepository()
	tracer := NewTracer(repo, Config{Enabled: true, TracedRoutes: []string{"POST /api/v1/chat"}})
	r := gin.New()
	r.Use(gin.CustomRecovery(func(c *gin.Context, recovered any) {
		c.Status(http.StatusInternalServerError)
	}))
	r.Use(Middleware(tracer))
	r.POST("/api/v1/chat", func(c *gin.Context) { panic("boom") })

	req := httptest.NewRequest(http.MethodPost, "/api/v1/chat", nil)
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)

	if len(repo.ends) != 1 {
		t.Fatalf("expected 1 end, got %d", len(repo.ends))
	}
	end := repo.ends[0].end
	if end.Status != SpanStatusError {
		t.Fatalf("status = %q, want %q", end.Status, SpanStatusError)
	}
	if end.ErrorType != "panic" {
		t.Fatalf("error_type = %q, want panic", end.ErrorType)
	}
}

func TestMiddlewareInjectsSpanContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newFakeRepository()
	tracer := NewTracer(repo, Config{Enabled: true, TracedRoutes: []string{"POST /api/v1/chat"}})
	r := gin.New()
	r.Use(Middleware(tracer))

	var ctxSpanID string
	r.POST("/api/v1/chat", func(c *gin.Context) {
		sc, ok := SpanContextFromContext(c.Request.Context())
		if ok {
			ctxSpanID = sc.SpanID
		}
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/chat", nil)
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)

	if len(repo.started) == 0 {
		t.Fatal("no span started")
	}
	if ctxSpanID != repo.started[0].SpanID {
		t.Fatalf("ctx span_id = %q, want %q", ctxSpanID, repo.started[0].SpanID)
	}
}

func TestMiddlewarePropagatesRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newFakeRepository()
	tracer := NewTracer(repo, Config{Enabled: true, TracedRoutes: []string{"POST /api/v1/chat"}})
	r := gin.New()
	r.Use(Middleware(tracer))
	r.POST("/api/v1/chat", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodPost, "/api/v1/chat", nil)
	req.Header.Set("X-Request-Id", "req-123")
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)

	if len(repo.started) == 0 {
		t.Fatal("no span started")
	}
	if repo.started[0].RequestID != "req-123" {
		t.Fatalf("request_id = %q, want req-123", repo.started[0].RequestID)
	}
}

func TestStatusFromHTTP(t *testing.T) {
	tests := []struct {
		code int
		want string
	}{
		{200, SpanStatusOK},
		{201, SpanStatusOK},
		{301, SpanStatusOK},
		{400, SpanStatusError},
		{404, SpanStatusError},
		{500, SpanStatusError},
	}
	for _, tt := range tests {
		if got := statusFromHTTP(tt.code); got != tt.want {
			t.Errorf("statusFromHTTP(%d) = %q, want %q", tt.code, got, tt.want)
		}
	}
}

// 确保 context import 被使用（fakeRepository 方法签名需要）
var _ = context.Background
