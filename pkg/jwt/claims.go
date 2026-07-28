package jwt

import "github.com/golang-jwt/jwt/v5"

// CustomClaims 自定义 JWT Claims
type CustomClaims struct {
	UID string `json:"uid"`
	jwt.RegisteredClaims
}

// GetUID 获取用户 UID
func (c *CustomClaims) GetUID() string {
	return c.UID
}
