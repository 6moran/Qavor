package trace

import (
	"context"
	"strings"
	"time"

	"Qavor/internal/model/entity"
	"Qavor/pkg/logger"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Tracer 统一 Span 创建入口，持有 SpanWriter 和配置
type Tracer struct {
	writer SpanWriter
	cfg    Config
}

// NewTracer 创建 Tracer，writer 为 nil 时所有操作为 no-op
func NewTracer(writer SpanWriter, cfg Config) *Tracer {
	if cfg.Retention <= 0 {
		cfg.Retention = 7 * 24 * time.Hour
	}
	return &Tracer{writer: writer, cfg: cfg}
}

// ShouldTrace 判断请求是否应被追踪（method + path 精确匹配 TracedRoutes）
func (t *Tracer) ShouldTrace(method, path string) bool {
	if t == nil || !t.cfg.Enabled || t.writer == nil {
		return false
	}
	target := method + " " + path
	for _, route := range t.cfg.TracedRoutes {
		if route == target {
			return true
		}
	}
	return false
}

// StartSpan 创建并写入 Span，返回注入 SpanContext 的新 context
// 未启用或父 SpanContext 未采样时返回 no-op Span
func (t *Tracer) StartSpan(ctx context.Context, spec SpanSpec) (context.Context, *Span) {
	if t == nil || !t.cfg.Enabled || t.writer == nil {
		return ctx, noopSpan()
	}
	parent, hasParent := SpanContextFromContext(ctx)
	if hasParent && !parent.Sampled {
		return ctx, noopSpan()
	}
	spec.InputSummary = t.sanitizeText(spec.InputSummary)
	record := buildSpanRecord(spec, parent, hasParent)
	if record.TraceID == "" {
		// 无 TraceID 且无父 Span：生成新 TraceID 并标记采样
		record.TraceID = uuid.New().String()
	}
	writeCtx := context.WithoutCancel(ctx)
	if err := t.writer.StartSpan(writeCtx, &record); err != nil {
		logger.Warn("trace: 创建 span 失败",
			zap.String("span_id", record.SpanID),
			zap.String("trace_id", record.TraceID),
			zap.String("operation", record.Operation),
			zap.Error(err))
	}
	span := newSpan(t, writeCtx, record)
	newCtx := WithSpanContext(ctx, SpanContext{
		TraceID:   record.TraceID,
		SpanID:    record.SpanID,
		RequestID: record.RequestID,
		RunID:     record.RunID,
		Sampled:   true,
	})
	return newCtx, span
}

// StartRequest 创建 HTTP 请求级 TraceRecord + http.server Span
// TraceID 优先使用 meta.TraceID（已校验），否则生成新 UUID
func (t *Tracer) StartRequest(ctx context.Context, meta RequestMeta) (context.Context, *Span) {
	if t == nil || !t.cfg.Enabled || t.writer == nil {
		return ctx, noopSpan()
	}
	traceID := meta.TraceID
	if traceID == "" {
		traceID = uuid.New().String()
	}
	now := time.Now()
	record := &entity.TraceRecord{
		TraceID:        traceID,
		RequestID:      meta.RequestID,
		ConversationID: meta.ConversationID,
		QuerySummary:   t.sanitizeText(meta.QuerySummary),
		EntryType:      meta.EntryType,
		CreatedAt:      now,
		ExpiresAt:      now.Add(t.cfg.Retention),
	}
	if record.EntryType == "" {
		record.EntryType = entity.EntryTypeHTTP
	}
	writeCtx := context.WithoutCancel(ctx)
	if err := t.writer.CreateTrace(writeCtx, record); err != nil {
		logger.Warn("trace: 创建 trace record 失败",
			zap.String("trace_id", traceID),
			zap.Error(err))
	}
	attrs := entity.JSON{
		"http.method": meta.Method,
		"http.path":   meta.Path,
	}
	return t.StartSpan(ctx, SpanSpec{
		TraceID:      traceID,
		RequestID:    meta.RequestID,
		RunID:        meta.RunID,
		Kind:         "http",
		Operation:    "http.server",
		DisplayName:  meta.Method + " " + meta.Path,
		InputSummary: truncate(meta.QuerySummary, t.cfg.MaxContentLength),
		Attributes:   attrs,
	})
}

func (t *Tracer) sanitizeText(value string) string {
	if t == nil {
		return ""
	}
	return (Sanitizer{Mode: t.ContentMode(), MaxRunes: t.MaxContentLength()}).Text(value)
}

// StartSpanIfTraced 仅在 ctx 已携带 SpanContext（即处于某条被追踪的链路内）时创建 Span
// 否则返回 no-op Span，用于检索/重排等"手动埋点"场景：不生成无 TraceRecord 的孤立 Span
func (t *Tracer) StartSpanIfTraced(ctx context.Context, spec SpanSpec) (context.Context, *Span) {
	if t == nil || !t.cfg.Enabled || t.writer == nil {
		return ctx, noopSpan()
	}
	if _, ok := SpanContextFromContext(ctx); !ok {
		return ctx, noopSpan()
	}
	return t.StartSpan(ctx, spec)
}

// UpdateRequestMetadata 在请求体解析后补全 TraceRecord，不让 Middleware 读取或重放请求体
func (t *Tracer) UpdateRequestMetadata(ctx context.Context, conversationID uint, query, entryType string) {
	if t == nil || !t.cfg.Enabled || t.writer == nil {
		return
	}
	spanContext, ok := SpanContextFromContext(ctx)
	if !ok || !spanContext.Sampled || spanContext.TraceID == "" {
		return
	}
	if err := t.writer.UpdateTraceMetadata(context.WithoutCancel(ctx), spanContext.TraceID, TraceMetadata{
		ConversationID: conversationID,
		QuerySummary:   t.sanitizeText(query),
		EntryType:      entryType,
	}); err != nil {
		logger.Warn("trace: 补全 trace metadata 失败", zap.String("trace_id", spanContext.TraceID), zap.Error(err))
	}
}

// ContentMode 返回内容模式（none/summary）
func (t *Tracer) ContentMode() string {
	if t == nil || t.cfg.ContentMode == "" {
		return "summary"
	}
	return strings.ToLower(t.cfg.ContentMode)
}

// MaxContentLength 返回最大内容长度
func (t *Tracer) MaxContentLength() int {
	if t == nil || t.cfg.MaxContentLength <= 0 {
		return 500
	}
	return t.cfg.MaxContentLength
}
