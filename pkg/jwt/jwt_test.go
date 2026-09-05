package jwt

import (
	"path/filepath"
	"testing"

	"Qavor/pkg/config"
)

func TestRefreshTokenIsSeparateFromAccessToken(t *testing.T) {
	loaded, err := config.Load(filepath.Join("..", "..", "configs", "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	loaded.JWT.Secret = "test-secret"
	loaded.JWT.ExpireHours = 2
	loaded.JWT.RefreshExpireHours = 168

	accessToken, err := GenerateToken()
	if err != nil {
		t.Fatal(err)
	}
	refreshToken, err := GenerateRefreshToken()
	if err != nil {
		t.Fatal(err)
	}

	if _, err := ParseToken(refreshToken); err != ErrTokenInvalid {
		t.Fatalf("ParseToken(refresh) error = %v, want ErrTokenInvalid", err)
	}
	if _, err := ParseRefreshToken(accessToken); err != ErrTokenInvalid {
		t.Fatalf("ParseRefreshToken(access) error = %v, want ErrTokenInvalid", err)
	}
	if _, err := ParseRefreshToken(refreshToken); err != nil {
		t.Fatalf("ParseRefreshToken(refresh) error = %v", err)
	}
}
