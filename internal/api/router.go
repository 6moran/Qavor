package api

import (
	agentctrl "Qavor/internal/api/v1/agent"
	"Qavor/internal/api/v1/auth"
	chatctrl "Qavor/internal/api/v1/chat"
	knowledgebase "Qavor/internal/api/v1/knowledge_base"
	knowledgefile "Qavor/internal/api/v1/knowledge_file"
	"Qavor/internal/api/v1/model_provider"
	"Qavor/internal/middleware"
	"Qavor/internal/service"

	"github.com/gin-gonic/gin"
)

// Router 路由
type Router struct {
	authCtrl          *auth.Controller
	knowledgeBaseCtrl *knowledgebase.Controller
	knowledgeFileCtrl *knowledgefile.Controller
	providerCtrl      *model_provider.Controller
	agentCtrl         *agentctrl.Controller
	chatCtrl          *chatctrl.Controller
}

// NewRouter 创建路由
func NewRouter(
	authService service.AuthService,
	knowledgeBaseService service.KnowledgeBaseService,
	knowledgeFileService service.KnowledgeFileService,
	providerService service.ModelProviderService,
	agentService service.AgentService,
	chatCtrl *chatctrl.Controller,
) *Router {
	return &Router{
		authCtrl:          auth.NewController(authService),
		knowledgeBaseCtrl: knowledgebase.NewController(knowledgeBaseService),
		knowledgeFileCtrl: knowledgefile.NewController(knowledgeFileService),
		providerCtrl:      model_provider.NewController(providerService),
		agentCtrl:         agentctrl.NewController(agentService),
		chatCtrl:          chatCtrl,
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

		// 知识库路由
		r.knowledgeBaseCtrl.RegisterRoutes(v1)
		r.knowledgeFileCtrl.RegisterRoutes(v1)

		// 模型提供商路由
		r.providerCtrl.RegisterRoutes(v1)

		// 智能体路由
		r.agentCtrl.RegisterRoutes(v1)

		// 聊天路由
		r.chatCtrl.RegisterRoutes(v1)
	}
}
