package request

// RefreshTokenRequest 刷新 Token 请求
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// LogoutRequest 登出请求
type LogoutRequest struct {
	AccessToken  string `json:"access_token"`  // 访问令牌（可选，用于加入黑名单）
	RefreshToken string `json:"refresh_token"` // 刷新令牌（必须，用于删除）
}

// LoginRequest 登录请求
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// RegisterRequest 注册请求
type RegisterRequest struct {
	Nickname        string `json:"nickname" binding:"required,min=2,max=50"`
	Password        string `json:"password" binding:"required,min=6,max=50"`
	ConfirmPassword string `json:"confirm_password" binding:"required,eqfield=Password"`
	Email           string `json:"email" binding:"required,email"`
}

// SendResetCodeRequest 发送重置验证码请求
type SendResetCodeRequest struct {
	Email string `json:"email" binding:"required,email"`
}

// ResetPasswordRequest 重置密码请求（包含验证码验证）
type ResetPasswordRequest struct {
	Email           string `json:"email" binding:"required,email"`
	Code            string `json:"code" binding:"required,len=6"`
	NewPassword     string `json:"new_password" binding:"required,min=6,max=50"`
	ConfirmPassword string `json:"confirm_password" binding:"required,eqfield=NewPassword"`
}
