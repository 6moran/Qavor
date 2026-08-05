package rag

import (
	"context"
	"time"

	"Qavor/internal/model/dto/request"
	"Qavor/internal/service"
	"Qavor/pkg/errors"
	"Qavor/pkg/logger"
	"Qavor/pkg/response"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Controller RAG 问答控制器。
type Controller struct {
	service service.RAGService
	timeout time.Duration
}

// NewController 创建 RAG 控制器。
func NewController(s service.RAGService, timeoutSeconds int) *Controller {
	if timeoutSeconds <= 0 {
		timeoutSeconds = 60
	}
	return &Controller{service: s, timeout: time.Duration(timeoutSeconds) * time.Second}
}

// Answer 处理 POST /api/v1/rag/answer。
func (ctrl *Controller) Answer(c *gin.Context) {
	var req request.RAGAnswerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// 60 秒整体超时，避免长时间阻塞 Worker / API 线程。
	ctx, cancel := context.WithTimeout(c.Request.Context(), ctrl.timeout)
	defer cancel()

	result, err := ctrl.service.Answer(ctx, req.KnowledgeBaseIDs, req.Query)
	if err != nil {
		if errors.IsBizError(err) {
			logger.Warn("业务错误，RAG 问答失败", zap.Error(err))
			response.BizError(c, err)
		} else {
			logger.Error("RAG 问答失败", zap.Error(err))
			response.InternalServerError(c)
		}
		return
	}
	response.Success(c, result)
}
