package utils

import (
	"testing"
	"time"
)

func TestInitJWT_RequiresSecret(t *testing.T) {
	t.Run("returns error when no secret provided and env unset", func(t *testing.T) {
		t.Setenv("JWT_SECRET", "")
		err := InitJWT("", 24*time.Hour)
		if err == nil {
			t.Fatal("Expected error when no secret is provided")
		}
	})

	t.Run("uses env var fallback", func(t *testing.T) {
		t.Setenv("JWT_SECRET", "env-secret-at-least-32-chars-long!!")
		if err := InitJWT("", 24*time.Hour); err != nil {
			t.Fatalf("Expected no error with env secret, got %v", err)
		}
	})

	t.Run("uses explicit secret", func(t *testing.T) {
		if err := InitJWT("explicit-secret-at-least-32-chars-long", 24*time.Hour); err != nil {
			t.Fatalf("Expected no error with explicit secret, got %v", err)
		}
	})
}

func TestGenerateToken_NotInitialized(t *testing.T) {
	// Reset global state by leaving it uninitialized
	jwtSecret = nil
	if _, err := GenerateToken("admin"); err != ErrJWTNotInitialized {
		t.Errorf("Expected ErrJWTNotInitialized, got %v", err)
	}
}

func TestValidateToken_NotInitialized(t *testing.T) {
	jwtSecret = nil
	if _, err := ValidateToken("some.token.value"); err != ErrJWTNotInitialized {
		t.Errorf("Expected ErrJWTNotInitialized, got %v", err)
	}
}

func TestGenerateAndValidateToken(t *testing.T) {
	if err := InitJWT("roundtrip-secret-at-least-32-chars-long!", 1*time.Hour); err != nil {
		t.Fatalf("InitJWT failed: %v", err)
	}

	token, err := GenerateToken("admin")
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	claims, err := ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken failed: %v", err)
	}
	if claims.Username != "admin" {
		t.Errorf("Expected username 'admin', got %q", claims.Username)
	}
}

func TestValidateToken_WrongSecret(t *testing.T) {
	if err := InitJWT("secret-a-for-testing-at-least-32-chars", time.Hour); err != nil {
		t.Fatalf("InitJWT failed: %v", err)
	}
	token, err := GenerateToken("admin")
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	// Re-init with a different secret
	if err := InitJWT("secret-b-for-testing-at-least-32-chars", time.Hour); err != nil {
		t.Fatalf("InitJWT failed: %v", err)
	}
	if _, err := ValidateToken(token); err == nil {
		t.Error("Expected error when validating token with wrong secret")
	}
}

func TestValidateToken_Expired(t *testing.T) {
	if err := InitJWT("expiry-secret-at-least-32-chars-long!!", -time.Minute); err != nil {
		t.Fatalf("InitJWT failed: %v", err)
	}
	token, err := GenerateToken("admin")
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}
	if _, err := ValidateToken(token); err == nil {
		t.Error("Expected error for expired token")
	}
}

func TestValidateToken_RejectsAlgNone(t *testing.T) {
	if err := InitJWT("alg-none-secret-at-least-32-chars-long", time.Hour); err != nil {
		t.Fatalf("InitJWT failed: %v", err)
	}
	// Hand-craft a token using "none" algorithm
	invalid := "eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0.eyJ1c2VybmFtZSI6ImFkbWluIn0."
	if _, err := ValidateToken(invalid); err == nil {
		t.Error("Expected rejection of alg=none token")
	}
}

func TestGetJWTExpiry(t *testing.T) {
	if err := InitJWT("expiry-getter-secret-at-least-32-chars", 90*time.Minute); err != nil {
		t.Fatalf("InitJWT failed: %v", err)
	}
	if got := GetJWTExpiry(); got != 90*time.Minute {
		t.Errorf("Expected expiry 90m, got %v", got)
	}
}
