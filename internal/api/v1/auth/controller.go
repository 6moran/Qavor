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
	userService service.UserService
}

// NewController 创建认证控制器
// 参数:
//   - authService: 认证服务
//   - userService: 用户服务
//
// 返回:
//   - *Controller: 认证控制器实例
func NewController(authService service.AuthService, userService service.UserService) *Controller {
	return &Controller{
		authService: authService,
		userService: userService,
	}
}

// Register 用户注册
// @Summary 用户注册
// @Description 创建新用户账号
// @Tags 认证
// @Accept json
// @Produce json
// @Param request body request.RegisterRequest true "注册信息"
// @Success 200 {object} response.Response
// @Router /api/v1/auth/register [post]
func (ctrl *Controller) Register(c *gin.Context) {
	var req request.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		code, message := validator.ErrorHandler(err)
		response.Error(c, code, message)
		return
	}

	if err := ctrl.userService.Register(&req); err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, nil)
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

// RefreshToken 刷新访问令牌
// @Summary 刷新访问令牌
// @Description 使用刷新令牌获取新的访问令牌和刷新令牌
// @Tags 认证
// @Accept json
// @Produce json
// @Param request body request.RefreshTokenRequest true "刷新令牌"
// @Success 200 {object} response.Response{data=response.TokenRefreshResponse}
// @Router /api/v1/auth/refresh [post]
func (ctrl *Controller) RefreshToken(c *gin.Context) {
	var req request.RefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		code, message := validator.ErrorHandler(err)
		response.Error(c, code, message)
		return
	}

	resp, err := ctrl.authService.RefreshToken(req.RefreshToken)
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, resp)
}

// SendResetCode 发送重置验证码
// @Summary 发送重置验证码
// @Description 发送密码重置验证码到邮箱
// @Tags 认证
// @Accept json
// @Produce json
// @Param request body request.SendResetCodeRequest true "邮箱地址"
// @Success 200 {object} response.Response{data=response.ResetCodeResponse}
// @Router /api/v1/auth/reset-code/send [post]
func (ctrl *Controller) SendResetCode(c *gin.Context) {
	var req request.SendResetCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		code, message := validator.ErrorHandler(err)
		response.Error(c, code, message)
		return
	}

	resp, err := ctrl.authService.SendResetCode(&req)
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, resp)
}

// ResetPassword 重置密码
// @Summary 重置密码
// @Description 使用验证码重置密码
// @Tags 认证
// @Accept json
// @Produce json
// @Param request body request.ResetPasswordRequest true "重置信息"
// @Success 200 {object} response.Response
// @Router /api/v1/auth/password/reset [post]
func (ctrl *Controller) ResetPassword(c *gin.Context) {
	var req request.ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		code, message := validator.ErrorHandler(err)
		response.Error(c, code, message)
		return
	}

	if err := ctrl.authService.ResetPassword(&req); err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, nil)
}
