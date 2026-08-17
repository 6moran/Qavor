package request

// LoginRequest 单实例管理员登录请求。
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// RefreshTokenRequest 刷新 Token 请求
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// LogoutRequest 登出请求
type LogoutRequest struct {
	AccessToken  string `json:"access_token"`  // 访问令牌（可选，用于加入黑名单）
	RefreshToken string `json:"refresh_token"` // 刷新令牌（必须，用于删除）
}
