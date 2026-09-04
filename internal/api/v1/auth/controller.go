package auth

import (
	"net/http"
	"time"

	"Qavor/internal/middleware"
	"Qavor/internal/model/dto/request"
	"Qavor/internal/service"
	"Qavor/pkg/config"
	"Qavor/pkg/errors"
	"Qavor/pkg/jwt"
	"Qavor/pkg/logger"
	"Qavor/pkg/response"
	"Qavor/pkg/validator"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const refreshCookieName = "qavor_refresh_token"

// Controller 认证控制器
type Controller struct {
	authService service.AuthService
}

// Logout 使当前管理员的 JWT 立即失效。
func (ctrl *Controller) Logout(c *gin.Context) {
	clearRefreshCookie(c)
	token := middleware.GetTokenFromHeader(c)
	if token == "" {
		response.Success(c, nil)
		return
	}
	if err := ctrl.authService.Logout(token); err != nil {
		if errors.IsBizError(err) {
			logger.Warn("业务错误，登出失败", zap.Error(err))
			response.BizError(c, err)
		} else {
			logger.Error("登出失败", zap.Error(err))
			response.InternalServerError(c)
		}
		return
	}
	response.Success(c, nil)
}

// Refresh 使用 HttpOnly Cookie 中的刷新令牌换取新的访问令牌。
func (ctrl *Controller) Refresh(c *gin.Context) {
	refreshToken, err := c.Cookie(refreshCookieName)
	if err != nil || refreshToken == "" {
		response.Unauthorized(c, "刷新令牌缺失")
		return
	}
	resp, err := ctrl.authService.Refresh(refreshToken)
	if err != nil {
		if err == jwt.ErrTokenExpired || err == jwt.ErrTokenInvalid {
			clearRefreshCookie(c)
			response.Unauthorized(c, "刷新令牌已失效")
			return
		}
		logger.Error("刷新令牌失败", zap.Error(err))
		response.InternalServerError(c)
		return
	}
	setRefreshCookie(c, resp.RefreshToken)
	response.Success(c, resp)
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
		if errors.IsBizError(err) {
			logger.Warn("业务错误，登录失败", zap.Error(err))
			response.BizError(c, err)
		} else {
			logger.Error("登录失败", zap.Error(err))
			response.InternalServerError(c)
		}
		return
	}

	setRefreshCookie(c, resp.RefreshToken)
	response.Success(c, resp)
}

func refreshCookieMaxAge() int {
	hours := config.Get().JWT.RefreshExpireHours
	if hours <= 0 {
		hours = 168
	}
	return int(hours * time.Hour / time.Second)
}

func setRefreshCookie(c *gin.Context, token string) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(refreshCookieName, token, refreshCookieMaxAge(), "/api/v1/auth", "", c.Request.TLS != nil, true)
}

func clearRefreshCookie(c *gin.Context) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(refreshCookieName, "", -1, "/api/v1/auth", "", c.Request.TLS != nil, true)
}
