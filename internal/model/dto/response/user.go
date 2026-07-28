package response

import "time"

// UserResponse 用户响应
type UserResponse struct {
	Nickname  string    `json:"nickname"`
	UID       string    `json:"uid"`
	Email     string    `json:"email"`
	Avatar    string    `json:"avatar"`
	Status    int       `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// LoginResponse 登录响应
type LoginResponse struct {
	AccessToken      string       `json:"access_token"`       // 访问令牌
	RefreshToken     string       `json:"refresh_token"`      // 刷新令牌
	AccessExpiresIn  int64        `json:"access_expires_in"`  // 访问令牌过期时间（秒）
	RefreshExpiresIn int64        `json:"refresh_expires_in"` // 刷新令牌过期时间（秒）
	ExpiresIn        int64        `json:"expires_in"`         // 访问令牌过期时间（秒），兼容字段
	IsFirstRun       bool         `json:"is_first_run"`       // 是否首次运行
	User             UserResponse `json:"user"`
}

// UserInfoResponse 用户信息响应
type UserInfoResponse struct {
	UserID   uint   `json:"user_id"`
	Nickname string `json:"nickname"`
}
