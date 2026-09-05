package jwt

import (
	"errors"
	"time"

	"Qavor/pkg/config"
	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrTokenInvalid = errors.New("token 无效")
	ErrTokenExpired = errors.New("token 已过期")
)

const (
	accessTokenType  = "access"
	refreshTokenType = "refresh"
)

// RemainingTTL 返回 Token 从 now 起的剩余有效时间。
func RemainingTTL(claims *CustomClaims, now time.Time) time.Duration {
	if claims == nil || claims.ExpiresAt == nil {
		return 0
	}
	ttl := claims.ExpiresAt.Time.Sub(now)
	if ttl < 0 {
		return 0
	}
	return ttl
}

// GenerateToken 生成 JWT Token
func GenerateToken() (string, error) {
	cfg := config.Get().JWT

	claims := CustomClaims{
		TokenType: accessTokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(cfg.ExpireHours * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "qavor",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(cfg.Secret))
}

// GenerateRefreshToken 生成仅用于换取访问令牌的刷新令牌。
func GenerateRefreshToken() (string, error) {
	cfg := config.Get().JWT
	expireHours := cfg.RefreshExpireHours
	if expireHours <= 0 {
		expireHours = 168
	}
	claims := CustomClaims{
		TokenType: refreshTokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expireHours * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "qavor",
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(cfg.Secret))
}

// ParseToken 解析 JWT Token
func ParseToken(tokenString string) (*CustomClaims, error) {
	claims, err := parse(tokenString)
	if err != nil {
		return nil, err
	}
	// 允许旧版未带 token_type 的访问令牌平滑过渡。
	if claims.TokenType != "" && claims.TokenType != accessTokenType {
		return nil, ErrTokenInvalid
	}
	return claims, nil
}

// ParseRefreshToken 校验刷新令牌。
func ParseRefreshToken(tokenString string) (*CustomClaims, error) {
	claims, err := parse(tokenString)
	if err != nil {
		return nil, err
	}
	if claims.TokenType != refreshTokenType {
		return nil, ErrTokenInvalid
	}
	return claims, nil
}

func parse(tokenString string) (*CustomClaims, error) {
	cfg := config.Get().JWT

	token, err := jwt.ParseWithClaims(tokenString, &CustomClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(cfg.Secret), nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		return nil, ErrTokenInvalid
	}

	if claims, ok := token.Claims.(*CustomClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, ErrTokenInvalid
}
