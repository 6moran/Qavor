package response

import (
	"math"
	"net/http"

	"Qavor/pkg/errors"
	"Qavor/pkg/logger"

	"github.com/gin-gonic/gin"
)

// Response 统一响应结构
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Success 成功响应
func Success(c *gin.Context, data interface{}) {
	c.JSON(200, Response{
		Code:    errors.CodeSuccess,
		Message: errors.GetMessage(errors.CodeSuccess),
		Data:    data,
	})
}

// Error 错误响应
func Error(c *gin.Context, code int, message string) {
	httpStatus := getHTTPStatus(code)
	c.JSON(httpStatus, Response{
		Code:    code,
		Message: message,
	})
}

// bizErrorResponse 业务错误响应体（Detail 可选，仅连接测试 / 模型调用错误等场景输出）
type bizErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Detail  string `json:"detail,omitempty"`
}

// BizError 业务错误响应
func BizError(c *gin.Context, err error) {
	if bizErr, ok := err.(*errors.BizError); ok {
		httpStatus := getHTTPStatus(bizErr.Code)
		c.JSON(httpStatus, bizErrorResponse{
			Code:    bizErr.Code,
			Message: bizErr.Message,
			Detail:  bizErr.Detail,
		})
		return
	}
	logger.Error("BizError 断言失败，非业务错误")
	InternalServerError(c)
}

// BadRequest 400 错误
func BadRequest(c *gin.Context, message string) {
	c.JSON(http.StatusBadRequest, Response{
		Code:    errors.CodeBadRequest,
		Message: message,
	})
}

// Unauthorized 401 错误
func Unauthorized(c *gin.Context, message string) {
	c.JSON(http.StatusUnauthorized, Response{
		Code:    errors.CodeUnauthorized,
		Message: message,
	})
}

// NotFound 404 错误
func NotFound(c *gin.Context, message string) {
	c.JSON(http.StatusNotFound, Response{
		Code:    errors.CodeNotFound,
		Message: message,
	})
}

// getHTTPStatus 根据业务错误码获取对应的 HTTP 状态码
func getHTTPStatus(code int) int {
	switch {
	case code == errors.CodeSuccess:
		return 200
	case code >= 400 && code < 500:
		return code
	case code >= 1001 && code < 1010:
		return 401 // 认证相关错误
	case code >= 2001 && code < 2010:
		return 400 // 参数错误
	case code >= 3001 && code < 3010:
		return 404 // 资源错误
	case code >= 40001 && code < 40100:
		return 404 // 会话/消息错误
	case code >= 4001 && code < 5000:
		return 500 // LLM 错误
	case code >= 5001 && code < 6000:
		return 500 // 模型提供商错误
	case code >= 6001 && code < 7000:
		return 500 // SSE 错误
	default:
		return 500
	}
}

// InternalServerError 500 服务器内部错误（无参版本）
func InternalServerError(c *gin.Context) {
	c.JSON(http.StatusInternalServerError, Response{
		Code:    errors.CodeInternalError,
		Message: errors.GetMessage(errors.CodeInternalError),
	})
}

// InternalServerErrorWithDetail 500 服务器内部错误（带错误详情，仅非 release 模式返回原始错误）
func InternalServerErrorWithDetail(c *gin.Context, err error) {
	msg := errors.GetMessage(errors.CodeInternalError)
	if err != nil && gin.Mode() != gin.ReleaseMode {
		msg = err.Error()
	}
	c.JSON(http.StatusInternalServerError, Response{
		Code:    errors.CodeInternalError,
		Message: msg,
	})
}

// PageRequest 分页请求参数
type PageRequest struct {
	Page int `form:"page" binding:"required,min=1"`         // 页码，从1开始
	Size int `form:"size" binding:"required,min=1,max=100"` // 每页大小，最大100
}

// PageResponse 分页响应结构
type PageResponse struct {
	List      interface{} `json:"list"`       // 数据列表
	Total     int64       `json:"total"`      // 总记录数
	Page      int         `json:"page"`       // 当前页码
	Size      int         `json:"size"`       // 每页大小
	TotalPage int         `json:"total_page"` // 总页数
}

// NewPageResponse 创建分页响应
func NewPageResponse(list interface{}, total int64, page, size int) *PageResponse {
	totalPage := int(math.Ceil(float64(total) / float64(size)))
	return &PageResponse{
		List:      list,
		Total:     total,
		Page:      page,
		Size:      size,
		TotalPage: totalPage,
	}
}
