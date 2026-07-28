package auth

import (
	"Qavor/internal/middleware"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册认证路由
func (ctrl *Controller) RegisterRoutes(r *gin.RouterGroup) {
	authGroup := r.Group("/auth")
	{
		// 公开接口（无需认证）
		authGroup.POST("/register", ctrl.Register)
		authGroup.POST("/login", ctrl.Login)
		authGroup.POST("/refresh", ctrl.RefreshToken)

		// 密码重置相关路由（无需认证）
		authGroup.POST("/reset-code/send", ctrl.SendResetCode)
		authGroup.POST("/password/reset", ctrl.ResetPassword)

		// 需要认证的接口
		authGroup.POST("/logout", middleware.Auth(), ctrl.Logout)
	}
}
