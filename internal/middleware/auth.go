package middleware

import (
	"strings"

	"Qavor/pkg/cache"
	"Qavor/pkg/jwt"
	"Qavor/pkg/logger"
	"Qavor/pkg/response"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

var (
	parseAuthToken            = jwt.ParseToken
	isTokenBlacklisted        = cache.IsTokenInBlacklist
	warnBlacklistCheckFailure = func(err error) {
		logger.Warn("Token 黑名单检查失败，降级为自然过期", zap.Error(err))
	}
)

// GetTokenFromHeader 提取 Authorization Header 中的 Bearer Token。
func GetTokenFromHeader(c *gin.Context) string {
	parts := strings.SplitN(c.GetHeader("Authorization"), " ", 2)
	if len(parts) != 2 || parts[0] != "Bearer" || parts[1] == "" {
		return ""
	}
	return parts[1]
}

// Auth JWT 认证中间件
func Auth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从 Header 获取 Authorization
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			response.Unauthorized(c, "请提供认证令牌")
			c.Abort()
			return
		}

		// 解析 Bearer Token
		token := GetTokenFromHeader(c)
		if token == "" {
			response.Unauthorized(c, "令牌格式错误")
			c.Abort()
			return
		}

		// 解析 Token
		_, err := parseAuthToken(token)
		if err != nil {
			response.Unauthorized(c, err.Error())
			c.Abort()
			return
		}

		blacklisted, err := isTokenBlacklisted(token)
		if err != nil {
			warnBlacklistCheckFailure(err)
		} else if blacklisted {
			response.Unauthorized(c, "令牌已失效")
			c.Abort()
			return
		}

		c.Next()
	}
}

// OptionalAuth 可选的 JWT 认证中间件
func OptionalAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.Next()
			return
		}

		token := GetTokenFromHeader(c)
		if token == "" {
			c.Next()
			return
		}

		_, _ = parseAuthToken(token)

		c.Next()
	}
}
