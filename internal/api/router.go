package api

import (
	agentctrl "Qavor/internal/api/v1/agent"
	"Qavor/internal/api/v1/auth"
	chatctrl "Qavor/internal/api/v1/chat"
	"Qavor/internal/api/v1/conversation"
	dashboardctrl "Qavor/internal/api/v1/dashboard"
	evaluationctrl "Qavor/internal/api/v1/evaluation"
	knowledgebase "Qavor/internal/api/v1/knowledge_base"
	knowledgefile "Qavor/internal/api/v1/knowledge_file"
	mcpserverctrl "Qavor/internal/api/v1/mcp_server"
	"Qavor/internal/api/v1/message"
	"Qavor/internal/api/v1/model"
	ocrctrl "Qavor/internal/api/v1/ocr"
	processingjob "Qavor/internal/api/v1/processing_job"
	ragctrl "Qavor/internal/api/v1/rag"
	ssectrl "Qavor/internal/api/v1/sse"
	systemctrl "Qavor/internal/api/v1/system"
	toolctrl "Qavor/internal/api/v1/tool"
	tracectrl "Qavor/internal/api/v1/trace"
	workspaceapi "Qavor/internal/api/v1/workspace"
	"Qavor/internal/middleware"
	"Qavor/internal/service"
	skillapi "Qavor/internal/skill/api"
	"Qavor/internal/tool"
	tracepkg "Qavor/internal/trace"

	"github.com/gin-gonic/gin"
)

// Router 路由
type Router struct {
	authCtrl          *auth.Controller
	knowledgeBaseCtrl *knowledgebase.Controller
	knowledgeFileCtrl *knowledgefile.Controller
	processingJobCtrl *processingjob.Controller
	modelCtrl         *model.Controller
	conversationCtrl  *conversation.Controller
	messageCtrl       *message.Controller
	agentCtrl         *agentctrl.Controller
	chatCtrl          *chatctrl.Controller
	ragCtrl           *ragctrl.Controller
	systemCtrl        *systemctrl.Controller
	toolCtrl          *toolctrl.Controller
	skillCtrl         *skillapi.Controller
	sseCtrl           *ssectrl.Controller
	mcpServerCtrl     *mcpserverctrl.Controller
	ocrCtrl           *ocrctrl.Controller
	postStreamHandler *agentctrl.PostStreamHandler
	runController     *agentctrl.RunController
	traceCtrl         *tracectrl.Controller
	dashboardCtrl     *dashboardctrl.Controller
	workspaceCtrl     *workspaceapi.Controller
	evaluationCtrl    *evaluationctrl.Controller
	tracer            *tracepkg.Tracer
}

// NewRouter 创建路由
func NewRouter(
	authService service.AuthService,
	knowledgeBaseService service.KnowledgeBaseService,
	knowledgeFileService service.KnowledgeFileService,
	processingJobService service.ProcessingJobService,
	modelService service.ModelService,
	conversationService service.ConversationService,
	messageService service.MessageService,
	agentService service.AgentService,
	agentOpts agentctrl.OptionsProvider,
	agentCacheInvalidator agentctrl.AgentCacheInvalidator,
	chatCtrl *chatctrl.Controller,
	ragCtrl *ragctrl.Controller,
	systemCtrl *systemctrl.Controller,
	toolRegistry *tool.Registry,
	skillCtrl *skillapi.Controller,
	sseCtrl *ssectrl.Controller,
	mcpServerCtrl *mcpserverctrl.Controller,
	postStreamHandler *agentctrl.PostStreamHandler,
	runController *agentctrl.RunController,
	traceCtrl *tracectrl.Controller,
	dashboardService service.DashboardService,
	workspaceCtrl *workspaceapi.Controller,
	knowledgeQueryService service.KnowledgeQueryService,
	tracer *tracepkg.Tracer,
	evaluationCtrl *evaluationctrl.Controller,
) *Router {
	return &Router{
		authCtrl:          auth.NewController(authService),
		knowledgeBaseCtrl: knowledgebase.NewController(knowledgeBaseService, knowledgeQueryService),
		knowledgeFileCtrl: knowledgefile.NewController(knowledgeFileService),
		processingJobCtrl: processingjob.NewController(processingJobService),
		modelCtrl:         model.NewController(modelService),
		conversationCtrl:  conversation.NewController(conversationService),
		messageCtrl:       message.NewController(messageService),
		agentCtrl:         agentctrl.NewController(agentService, agentOpts, agentCacheInvalidator),
		chatCtrl:          chatCtrl,
		ragCtrl:           ragCtrl,
		systemCtrl:        systemCtrl,
		toolCtrl:          toolctrl.NewController(toolRegistry),
		sseCtrl:           sseCtrl,
		skillCtrl:         skillCtrl,
		mcpServerCtrl:     mcpServerCtrl,
		postStreamHandler: postStreamHandler,
		runController:     runController,
		traceCtrl:         traceCtrl,
		dashboardCtrl:     dashboardctrl.NewController(dashboardService),
		workspaceCtrl:     workspaceCtrl,
		evaluationCtrl:    evaluationCtrl,
		tracer:            tracer,
	}
}

// Setup 设置路由
func (r *Router) Setup(engine *gin.Engine) {
	// 全局中间件
	engine.Use(middleware.Recovery())
	engine.Use(middleware.Logger())
	engine.Use(tracepkg.Middleware(r.tracer))
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
		r.processingJobCtrl.RegisterRoutes(v1)

		// 智能体路由
		r.agentCtrl.RegisterRoutes(v1)

		// Run 流式与队列操作路由
		if r.postStreamHandler != nil && r.runController != nil {
			agentctrl.RegisterRunRoutes(v1, r.postStreamHandler, r.runController)
		}

		// 聊天路由
		r.chatCtrl.RegisterRoutes(v1)

		// 模型路由
		r.modelCtrl.RegisterRoutes(v1)

		// 会话路由
		r.conversationCtrl.RegisterRoutes(v1)

		// 消息路由
		r.messageCtrl.RegisterRoutes(v1)

		// RAG 路由
		if r.ragCtrl != nil {
			r.ragCtrl.RegisterRoutes(v1)
		}
		// 全局 RAG 设置路由
		if r.systemCtrl != nil {
			r.systemCtrl.RegisterRoutes(v1)
		}
		// 工具路由
		r.toolCtrl.RegisterRoutes(v1)
		// OCR 引擎路由
		if r.ocrCtrl == nil {
			r.ocrCtrl = ocrctrl.NewController()
		}
		r.ocrCtrl.RegisterRoutes(v1)

		// Skill 路由
		r.skillCtrl.RegisterRoutes(v1)

		// MCP 服务器路由
		r.mcpServerCtrl.RegisterRoutes(v1)

		// SSE 流式服务路由
		ssectrl.RegisterRoutes(v1, r.sseCtrl)

		// 工作区路由
		r.workspaceCtrl.RegisterRoutes(v1)

		// 链路追踪路由
		if r.traceCtrl != nil {
			r.traceCtrl.RegisterRoutes(v1)
		}

		// 仪表盘路由
		if r.dashboardCtrl != nil {
			r.dashboardCtrl.RegisterRoutes(v1)
		}

		// 评估路由
		if r.evaluationCtrl != nil {
			r.evaluationCtrl.RegisterRoutes(v1)
		}
	}
}
