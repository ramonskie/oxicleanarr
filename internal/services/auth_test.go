package services

import (
	"testing"
	"time"

	"github.com/ramonskie/oxicleanarr/internal/config"
	"github.com/ramonskie/oxicleanarr/internal/utils"
)

func setupAuthService(t *testing.T, password string) *AuthService {
	t.Helper()
	if err := utils.InitJWT("auth-service-test-secret-at-least-32-chars", 24*time.Hour); err != nil {
		t.Fatalf("InitJWT failed: %v", err)
	}
	cfg := &config.Config{
		Admin: config.AdminConfig{
			Username: "admin",
			Password: password,
		},
	}
	return NewAuthService(cfg)
}

func TestLogin_Success(t *testing.T) {
	svc := setupAuthService(t, "testpassword")

	token, err := svc.Login("admin", "testpassword")
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}
	if token == "" {
		t.Error("Expected non-empty token")
	}

	claims, err := svc.ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken failed: %v", err)
	}
	if claims.Username != "admin" {
		t.Errorf("Expected username admin, got %q", claims.Username)
	}
}

func TestLogin_WithBcryptStoredPassword(t *testing.T) {
	hash, err := utils.HashPassword("bcryptpassword")
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}
	svc := setupAuthService(t, hash)

	if _, err := svc.Login("admin", "bcryptpassword"); err != nil {
		t.Errorf("Login with bcrypt-stored password failed: %v", err)
	}
	if _, err := svc.Login("admin", "wrongpassword"); err != ErrInvalidCredentials {
		t.Errorf("Expected ErrInvalidCredentials for wrong password, got %v", err)
	}
}

func TestLogin_WrongCredentials(t *testing.T) {
	svc := setupAuthService(t, "testpassword")

	tests := []struct {
		name     string
		username string
		password string
	}{
		{"wrong password", "admin", "wrong"},
		{"wrong username", "other", "testpassword"},
		{"empty username", "", "testpassword"},
		{"empty password", "admin", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := svc.Login(tt.username, tt.password); err != ErrInvalidCredentials {
				t.Errorf("Expected ErrInvalidCredentials, got %v", err)
			}
		})
	}
}

func TestChangePassword_HashesNewPassword(t *testing.T) {
	svc := setupAuthService(t, "oldpassword")

	if err := svc.ChangePassword("oldpassword", "newpassword"); err != nil {
		t.Fatalf("ChangePassword failed: %v", err)
	}

	// New password must work with the service
	if _, err := svc.Login("admin", "newpassword"); err != nil {
		t.Errorf("Login with new password failed: %v", err)
	}
	// Old password must no longer work
	if _, err := svc.Login("admin", "oldpassword"); err != ErrInvalidCredentials {
		t.Errorf("Expected old password to be rejected, got %v", err)
	}
	// Stored value must be a bcrypt hash, not plaintext
	if !utils.IsBcryptHash(svc.cfg.Admin.Password) {
		t.Error("ChangePassword must store a bcrypt hash, not plaintext")
	}
	if svc.cfg.Admin.Password == "newpassword" {
		t.Error("Stored password must not equal plaintext")
	}
}

func TestChangePassword_WrongCurrent(t *testing.T) {
	svc := setupAuthService(t, "oldpassword")

	if err := svc.ChangePassword("wrongcurrent", "newpassword"); err != ErrInvalidCredentials {
		t.Errorf("Expected ErrInvalidCredentials, got %v", err)
	}
}

func TestChangePassword_EmptyNewPassword(t *testing.T) {
	svc := setupAuthService(t, "oldpassword")

	if err := svc.ChangePassword("oldpassword", ""); err != ErrEmptyPassword {
		t.Errorf("Expected ErrEmptyPassword, got %v", err)
	}
}
