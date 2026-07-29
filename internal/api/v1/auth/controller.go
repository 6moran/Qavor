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

// Logout 使当前管理员的 JWT 立即失效。
func (ctrl *Controller) Logout(c *gin.Context) {
	token := middleware.GetTokenFromHeader(c)
	if token == "" {
		response.Unauthorized(c, "请提供认证令牌")
		return
	}
	if err := ctrl.authService.Logout(token); err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, nil)
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
