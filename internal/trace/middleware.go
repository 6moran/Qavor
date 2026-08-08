package trace

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Trace Gin 全局中间件：从 X-Trace-Id 恢复或生成 TraceID，注入 trace 上下文。
// 注册在 Logger 之后；trace.enabled=false 时透传不做任何事。
func Trace() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !Enabled() {
			c.Next()
			return
		}
		traceID := c.GetHeader("X-Trace-Id")
		if traceID == "" {
			traceID = uuid.New().String()
		}
		c.Request = c.Request.WithContext(WithTraceContext(c.Request.Context(), &TraceContext{
			TraceID: traceID,
		}))
		c.Next()
	}
}
