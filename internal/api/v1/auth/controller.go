package auth

import (
	"Qavor/internal/middleware"
	"Qavor/internal/model/dto/request"
	"Qavor/internal/service"
	"Qavor/pkg/response"
	"Qavor/pkg/validator"

	"github.com/gin-gonic/gin"
)

// Controller 认证控制器
type Controller struct {
	authService service.AuthService
}

// NewController 创建认证控制器
func NewController(authService service.AuthService) *Controller {
	return &Controller{
		authService: authService,
	}
}

// Login 用户登录
// @Summary 用户登录
// @Description 用户登录获取 Token
// @Tags 认证
// @Accept json
// @Produce json
// @Param request body request.LoginRequest true "登录信息"
// @Success 200 {object} response.Response{data=response.LoginResponse}
// @Router /api/v1/auth/login [post]
func (ctrl *Controller) Login(c *gin.Context) {
	var req request.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		code, message := validator.ErrorHandler(err)
		response.Error(c, code, message)
		return
	}

	resp, err := ctrl.authService.Login(&req)
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, resp)
}

// Logout 用户登出
// @Summary 用户登出
// @Description 用户登出，使 Token 失效
// @Tags 认证
// @Accept json
// @Produce json
// @Param request body request.LogoutRequest true "登出信息"
// @Success 200 {object} response.Response
// @Router /api/v1/auth/logout [post]
func (ctrl *Controller) Logout(c *gin.Context) {
	var req request.LogoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		code, message := validator.ErrorHandler(err)
		response.Error(c, code, message)
		return
	}

	// 如果请求体中没有 Token，尝试从 Header 获取
	if req.AccessToken == "" {
		req.AccessToken = middleware.GetTokenFromHeader(c)
	}

	if err := ctrl.authService.Logout(req.AccessToken, req.RefreshToken); err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, nil)
}
