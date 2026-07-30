package middleware

import (
	"time"

	"Qavor/pkg/logger"
	"github.com/gin-gonic/gin"
)

// Logger 日志中间件
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 开始时间
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		// 处理请求
		c.Next()

		// 结束时间
		end := time.Now()
		latency := end.Sub(start)

		requestErrors := ""
		if len(c.Errors) > 0 {
			requestErrors = c.Errors.String()
		}

		logger.HTTPRequest(
			c.Request.Method,
			path,
			query,
			c.Writer.Status(),
			latency,
			c.ClientIP(),
			c.Request.UserAgent(),
			requestErrors,
		)
	}
}
