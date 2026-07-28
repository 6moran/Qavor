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

// TokenPair 双 Token 响应
type TokenPair struct {
	AccessToken      string `json:"access_token"`       // 访问令牌
	RefreshToken     string `json:"refresh_token"`      // 刷新令牌
	AccessExpiresIn  int64  `json:"access_expires_in"`  // 访问令牌过期时间（秒）
	RefreshExpiresIn int64  `json:"refresh_expires_in"` // 刷新令牌过期时间（秒）
}

// GenerateAccessToken 生成访问令牌（短有效期）
func GenerateAccessToken(uid string) (string, error) {
	cfg := config.Get().JWT

	claims := CustomClaims{
		UID: uid,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(cfg.AccessExpire * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "qavor",
			Subject:   "access",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(cfg.Secret))
}

// GenerateRefreshToken 生成刷新令牌（长有效期）
func GenerateRefreshToken(uid string) (string, error) {
	cfg := config.Get().JWT

	claims := CustomClaims{
		UID: uid,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(cfg.RefreshExpire * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "qavor",
			Subject:   "refresh",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(cfg.Secret))
}

// GenerateTokenPair 生成 Token 对（访问令牌 + 刷新令牌）
func GenerateTokenPair(uid string) (*TokenPair, error) {
	accessToken, err := GenerateAccessToken(uid)
	if err != nil {
		return nil, err
	}

	refreshToken, err := GenerateRefreshToken(uid)
	if err != nil {
		return nil, err
	}

	cfg := config.Get().JWT
	return &TokenPair{
		AccessToken:      accessToken,
		RefreshToken:     refreshToken,
		AccessExpiresIn:  int64(cfg.AccessExpire * 3600),  // 转换为秒
		RefreshExpiresIn: int64(cfg.RefreshExpire * 3600), // 转换为秒
	}, nil
}

// ParseToken 解析 JWT Token
func ParseToken(tokenString string) (*CustomClaims, error) {
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
