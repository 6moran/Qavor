package api

import (
	"Qavor/internal/api/v1/auth"
	knowledgebase "Qavor/internal/api/v1/knowledge_base"
	knowledgefile "Qavor/internal/api/v1/knowledge_file"
	"Qavor/internal/api/v1/user"
	"Qavor/internal/middleware"
	"Qavor/internal/service"

	"github.com/gin-gonic/gin"
)

// Router 路由
type Router struct {
	userCtrl          *user.Controller
	authCtrl          *auth.Controller
	knowledgeBaseCtrl *knowledgebase.Controller
	knowledgeFileCtrl *knowledgefile.Controller
}

// NewRouter 创建路由
func NewRouter(
	userService service.UserService,
	authService service.AuthService,
	knowledgeBaseService service.KnowledgeBaseService,
	knowledgeFileService service.KnowledgeFileService,
) *Router {
	return &Router{
		userCtrl:          user.NewController(userService),
		authCtrl:          auth.NewController(authService, userService),
		knowledgeBaseCtrl: knowledgebase.NewController(knowledgeBaseService),
		knowledgeFileCtrl: knowledgefile.NewController(knowledgeFileService),
	}
}

// Setup 设置路由
func (r *Router) Setup(engine *gin.Engine) {
	// 全局中间件
	engine.Use(middleware.Recovery())
	engine.Use(middleware.Logger())
	engine.Use(middleware.CORS())

	// 健康检查
	engine.GET("/api/v1/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "ok",
			"message": "Qavor API is running",
		})
	})

	// API v1 路由组
	v1 := engine.Group("/api/v1")
	{
		// 认证路由
		r.authCtrl.RegisterRoutes(v1)

		// 用户路由
		r.userCtrl.RegisterRoutes(v1)

		// 知识库路由
		r.knowledgeBaseCtrl.RegisterRoutes(v1)
		r.knowledgeFileCtrl.RegisterRoutes(v1)
	}
}
