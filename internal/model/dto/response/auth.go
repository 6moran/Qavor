package response

import "time"

// ResetCodeResponse 重置验证码响应
type ResetCodeResponse struct {
	ExpiresIn int `json:"expires_in"` // 过期时间（秒）
}

// TokenRefreshResponse Token 刷新响应
type TokenRefreshResponse struct {
	AccessToken      string    `json:"access_token"`       // 新的访问令牌
	RefreshToken     string    `json:"refresh_token"`      // 新的刷新令牌
	AccessExpiresIn  int64     `json:"access_expires_in"`  // 访问令牌过期时间（秒）
	RefreshExpiresIn int64     `json:"refresh_expires_in"` // 刷新令牌过期时间（秒）
	AccessExpiresAt  time.Time `json:"access_expires_at"`  // 访问令牌过期时间
	RefreshExpiresAt time.Time `json:"refresh_expires_at"` // 刷新令牌过期时间
}

// LogoutResponse 登出响应
type LogoutResponse struct {
	Message string `json:"message"`
}
