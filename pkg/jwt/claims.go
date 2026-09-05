package jwt

import "github.com/golang-jwt/jwt/v5"

// CustomClaims 自定义 JWT Claims
type CustomClaims struct {
	jwt.RegisteredClaims
	TokenType string `json:"token_type,omitempty"`
}
