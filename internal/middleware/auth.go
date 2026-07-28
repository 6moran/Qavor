package middleware

import (
	"strings"

	"Qavor/pkg/cache"
	"Qavor/pkg/jwt"
	"Qavor/pkg/response"
	"github.com/gin-gonic/gin"
)

const (
	// ContextUID 用户 UID 上下文键
	ContextUID = "uid"
)

// GetTokenFromHeader 从 Header 获取 Token
// 参数:
//   - c: Gin 上下文
//
// 返回:
//   - string: Token 字符串（如果没有则返回空字符串）
func GetTokenFromHeader(c *gin.Context) string {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		return ""
	}

	// 解析 Bearer Token
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || parts[0] != "Bearer" {
		return ""
	}

	return parts[1]
}

// Auth JWT 认证中间件
// 用于保护需要登录才能访问的接口
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
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			response.Unauthorized(c, "令牌格式错误")
			c.Abort()
			return
		}

		token := parts[1]

		// 检查 Token 是否在黑名单中
		inBlacklist, err := cache.IsTokenInBlacklist(token)
		if err != nil {
			response.Unauthorized(c, "验证令牌失败")
			c.Abort()
			return
		}
		if inBlacklist {
			response.Unauthorized(c, "令牌已失效")
			c.Abort()
			return
		}

		// 解析 Token
		claims, err := jwt.ParseToken(token)
		if err != nil {
			response.Unauthorized(c, err.Error())
			c.Abort()
			return
		}

		// 验证是否为访问令牌
		if claims.Subject != "access" {
			response.Unauthorized(c, "无效的访问令牌")
			c.Abort()
			return
		}

		// 将用户信息存入上下文
		c.Set(ContextUID, claims.GetUID())

		c.Next()
	}
}

// GetUID 从上下文获取用户 UID
// 参数:
//   - c: Gin 上下文
//
// 返回:
//   - string: 用户 UID（如果不存在返回空字符串）
func GetUID(c *gin.Context) string {
	if uid, exists := c.Get(ContextUID); exists {
		return uid.(string)
	}
	return ""
}

// OptionalAuth 可选的 JWT 认证中间件
// 用于可选登录的接口，如果提供了 Token 则解析并设置用户信息到上下文，否则继续
// 适用场景：同一接口对登录/未登录用户展示不同内容（如评论区显示用户名）
//func OptionalAuth() gin.HandlerFunc {
//	return func(c *gin.Context) {
//		authHeader := c.GetHeader("Authorization")
//		if authHeader == "" {
//			c.Next()
//			return
//		}
//
//		parts := strings.SplitN(authHeader, " ", 2)
//		if len(parts) != 2 || parts[0] != "Bearer" {
//			c.Next()
//			return
//		}
//
//		token := parts[1]
//
//		// 检查 Token 是否在黑名单中
//		inBlacklist, err := cache.IsTokenInBlacklist(token)
//		if err != nil || inBlacklist {
//			c.Next()
//			return
//		}
//
//		claims, err := jwt.ParseToken(token)
//		if err == nil && claims.Subject == "access" {
//			c.Set(ContextUID, claims.GetUID())
//		}
//
//		c.Next()
//	}
//}
