// Package knowledge_base 提供知识库元数据 CRUD 的 HTTP API。
package knowledge_base

import (
	"Qavor/internal/model/dto/request"
	"Qavor/internal/rag"
	"Qavor/internal/service"
	"Qavor/pkg/errors"
	"Qavor/pkg/logger"
	"Qavor/pkg/response"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Controller 负责知识库请求参数绑定、业务服务调用和统一响应转换
type Controller struct {
	service  service.KnowledgeBaseService
	querySvc service.KnowledgeQueryService
}

// NewController 创建知识库 API 控制器
func NewController(service service.KnowledgeBaseService, querySvc service.KnowledgeQueryService) *Controller {
	return &Controller{service: service, querySvc: querySvc}
}

// Create 创建知识库
func (ctrl *Controller) Create(c *gin.Context) {
	var req request.CreateKnowledgeBaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	result, err := ctrl.service.Create(&req)
	if err != nil {
		if errors.IsBizError(err) {
			logger.Warn("业务错误，创建知识库失败", zap.Error(err))
			response.BizError(c, err)
		} else {
			logger.Error("创建知识库失败", zap.Error(err))
			response.InternalServerError(c)
		}
		return
	}
	response.Success(c, result)
}

// List 获取知识库列表
func (ctrl *Controller) List(c *gin.Context) {
	var req request.KnowledgeBaseListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	result, err := ctrl.service.List(&req)
	if err != nil {
		if errors.IsBizError(err) {
			logger.Warn("业务错误，获取知识库列表失败", zap.Error(err))
			response.BizError(c, err)
		} else {
			logger.Error("获取知识库列表失败", zap.Error(err))
			response.InternalServerError(c)
		}
		return
	}
	response.Success(c, result)
}

// Get 根据路径中的 kb_id 获取知识库详情
func (ctrl *Controller) Get(c *gin.Context) {
	result, err := ctrl.service.Get(c.Param("kb_id"))
	if err != nil {
		if errors.IsBizError(err) {
			logger.Warn("业务错误，获取知识库详情失败", zap.Error(err))
			response.BizError(c, err)
		} else {
			logger.Error("获取知识库详情失败", zap.Error(err))
			response.InternalServerError(c)
		}
		return
	}
	response.Success(c, result)
}

// Update 根据 kb_id 更新知识库元数据
func (ctrl *Controller) Update(c *gin.Context) {
	var req request.UpdateKnowledgeBaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	result, err := ctrl.service.Update(c.Param("kb_id"), &req)
	if err != nil {
		if errors.IsBizError(err) {
			logger.Warn("业务错误，更新知识库失败", zap.Error(err))
			response.BizError(c, err)
		} else {
			logger.Error("更新知识库失败", zap.Error(err))
			response.InternalServerError(c)
		}
		return
	}
	response.Success(c, result)
}

// Delete 根据 kb_id 删除知识库记录
func (ctrl *Controller) Delete(c *gin.Context) {
	if err := ctrl.service.Delete(c.Param("kb_id")); err != nil {
		if errors.IsBizError(err) {
			logger.Warn("业务错误，删除知识库失败", zap.Error(err))
			response.BizError(c, err)
		} else {
			logger.Error("删除知识库失败", zap.Error(err))
			response.InternalServerError(c)
		}
		return
	}
	response.Success(c, nil)
}

// ChunkPresets 获取支持的分块预设列表。
// 响应格式与前端 useChunkPresetOptions 的 {value, label, description} 解析一致。
func (ctrl *Controller) ChunkPresets(c *gin.Context) {
	response.Success(c, map[string]any{"chunk_presets": rag.ChunkPresetList()})
}

// QueryTest 执行检索测试：携带查询与可选检索参数，不修改知识库配置。
func (ctrl *Controller) QueryTest(c *gin.Context) {
	var req request.QueryTestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	result, err := ctrl.querySvc.QueryTest(c.Request.Context(), c.Param("kb_id"), req.Query, req.Meta)
	if err != nil {
		ctrl.handleQueryServiceError(c, err, "检索测试失败")
		return
	}
	response.Success(c, result.Chunks)
}

// GetQueryParams 获取知识库检索参数选项与当前保存值。
func (ctrl *Controller) GetQueryParams(c *gin.Context) {
	result, err := ctrl.querySvc.GetQueryParams(c.Param("kb_id"))
	if err != nil {
		ctrl.handleQueryServiceError(c, err, "获取检索参数失败")
		return
	}
	response.Success(c, result)
}

// UpdateQueryParams 更新知识库检索参数（白名单过滤后持久化）。
func (ctrl *Controller) UpdateQueryParams(c *gin.Context) {
	var req request.UpdateQueryParamsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err := ctrl.querySvc.UpdateQueryParams(c.Param("kb_id"), req); err != nil {
		ctrl.handleQueryServiceError(c, err, "更新检索参数失败")
		return
	}
	response.Success(c, nil)
}

// GetSampleQuestions 获取知识库已生成的示例问题。
func (ctrl *Controller) GetSampleQuestions(c *gin.Context) {
	result, err := ctrl.querySvc.GetSampleQuestions(c.Param("kb_id"))
	if err != nil {
		ctrl.handleQueryServiceError(c, err, "获取示例问题失败")
		return
	}
	response.Success(c, result)
}

// GenerateSampleQuestions 用 AI 生成知识库测试问题并持久化。
func (ctrl *Controller) GenerateSampleQuestions(c *gin.Context) {
	var req request.GenerateSampleQuestionsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	result, err := ctrl.querySvc.GenerateSampleQuestions(c.Request.Context(), c.Param("kb_id"), req.Count)
	if err != nil {
		ctrl.handleQueryServiceError(c, err, "生成示例问题失败")
		return
	}
	response.Success(c, result)
}

// GenerateDescription 用 AI 生成或润色知识库描述（新建/编辑知识库表单的润色按钮）。
// 响应直接返回 {status, description}，与前端 AiTextarea 组件的解析约定保持一致。
func (ctrl *Controller) GenerateDescription(c *gin.Context) {
	var req request.GenerateDescriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	description, err := ctrl.querySvc.GenerateDescription(c.Request.Context(), &req)
	if err != nil {
		ctrl.handleQueryServiceError(c, err, "生成知识库描述失败")
		return
	}
	c.JSON(200, gin.H{"status": "success", "description": description})
}

// handleQueryServiceError 统一处理查询服务的业务错误与内部错误。
func (ctrl *Controller) handleQueryServiceError(c *gin.Context, err error, action string) {
	if errors.IsBizError(err) {
		logger.Warn("业务错误，"+action, zap.Error(err))
		response.BizError(c, err)
		return
	}
	logger.Error(action, zap.Error(err))
	response.InternalServerError(c)
}
