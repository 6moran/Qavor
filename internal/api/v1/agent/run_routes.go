package agent

import (
	"Qavor/internal/middleware"

	"github.com/gin-gonic/gin"
)

// RegisterRunRoutes 注册 Run 流式与队列操作路由
// POST /agent/runs            创建 Run 并返回 SSE 流（携带 resume 时为断线重连）
// GET  /agent/runs/:runId     获取 Run 状态
// POST /agent/runs/:runId/cancel  取消 Run
// GET  /agent/requests/:requestId     获取请求详情
// POST /agent/requests/:requestId/cancel  取消排队请求
// POST /agent/requests/:requestId/steer   引导请求
// GET  /agent/thread/:threadId/requests   列出线程排队请求
// POST /agent/thread/:threadId/requests/continue  继续暂停队列
func RegisterRunRoutes(r *gin.RouterGroup, postStream *PostStreamHandler, runCtrl *RunController) {
	runGroup := r.Group("/agent")
	runGroup.Use(middleware.Auth())
	{
		runGroup.POST("/runs", postStream.CreateRunAndStream)
		runGroup.GET("/runs/:runId", runCtrl.GetRun)
		runGroup.POST("/runs/:runId/cancel", runCtrl.CancelRun)

		runGroup.GET("/requests/:requestId", runCtrl.GetRequest)
		runGroup.POST("/requests/:requestId/cancel", runCtrl.CancelRequest)
		runGroup.POST("/requests/:requestId/steer", runCtrl.SteerRequest)

		runGroup.GET("/thread/:threadId/requests", runCtrl.ListThreadRequests)
		runGroup.POST("/thread/:threadId/requests/continue", runCtrl.ContinueThreadQueue)
	}
}
