package auth

import (
	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册认证路由
func (ctrl *Controller) RegisterRoutes(r *gin.RouterGroup) {
	authGroup := r.Group("/auth")
	{
		authGroup.POST("/register", ctrl.Register)
		authGroup.POST("/login", ctrl.Login)
		authGroup.POST("/refresh", ctrl.RefreshToken)
	}
}
