package trace

import (
	"fmt"

	"Qavor/internal/model/entity"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Middleware 创建并管理完整 HTTP Span 生命周期
// 只追踪 TracedRoutes 中配置的路由（method + path 精确匹配）
// tracer 为 nil 或未启用时透传不做任何事
func Middleware(tracer *Tracer) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 检查路由白名单
		if !tracer.ShouldTrace(c.Request.Method, c.Request.URL.Path) {
			c.Next()
			return
		}

		meta := RequestMeta{
			TraceID:   validatedTraceID(c),
			RequestID: requestID(c),
			Method:    c.Request.Method,
			Path:      c.Request.URL.Path,
			EntryType: entity.EntryTypeHTTP,
		}
		ctx, span := tracer.StartRequest(c.Request.Context(), meta)
		c.Request = c.Request.WithContext(ctx)

		defer func() {
			if recovered := recover(); recovered != nil {
				span.End(SpanEnd{
					Status:       SpanStatusError,
					ErrorType:    "panic",
					ErrorMessage: fmt.Sprint(recovered),
				})
				panic(recovered)
			}
			span.End(SpanEnd{
				Status:     statusFromHTTP(c.Writer.Status()),
				Attributes: entity.JSON{"http.status_code": c.Writer.Status()},
			})
		}()

		c.Next()
	}
}

// requestID 从 X-Request-Id 头提取或生成新 UUID
func requestID(c *gin.Context) string {
	if id := c.GetHeader("X-Request-Id"); id != "" {
		return id
	}
	return uuid.New().String()
}

// validatedTraceID 从 X-Trace-Id 头提取并校验，非法时返回空串（Tracer 生成新 UUID）
func validatedTraceID(c *gin.Context) string {
	tid := c.GetHeader("X-Trace-Id")
	if tid == "" {
		return ""
	}
	if !ValidTraceID(tid) {
		return ""
	}
	return tid
}

// statusFromHTTP 将 HTTP 状态码映射为 Span 状态
func statusFromHTTP(code int) string {
	if code >= 400 {
		return SpanStatusError
	}
	return SpanStatusOK
}
