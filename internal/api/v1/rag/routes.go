package rag

import (
	"Qavor/internal/middleware"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册需要 JWT 认证的 RAG 路由。
func (ctrl *Controller) RegisterRoutes(v1 *gin.RouterGroup) {
	group := v1.Group("/rag")
	group.Use(middleware.Auth())
	group.POST("/answer", ctrl.Answer)
}
