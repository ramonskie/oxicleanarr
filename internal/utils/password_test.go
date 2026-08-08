package utils

import (
	"strings"
	"testing"
)

func TestHashAndCheckPassword(t *testing.T) {
	hash, err := HashPassword("supersecret")
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}

	if !strings.HasPrefix(hash, "$2") {
		t.Errorf("Expected bcrypt hash prefix, got %q", hash)
	}
	if hash == "supersecret" {
		t.Error("Hash must not equal plaintext")
	}

	if !CheckPassword(hash, "supersecret") {
		t.Error("CheckPassword should match correct password")
	}
	if CheckPassword(hash, "wrongpassword") {
		t.Error("CheckPassword should reject wrong password")
	}
	if CheckPassword(hash, "") {
		t.Error("CheckPassword should reject empty password")
	}
}

func TestIsBcryptHash(t *testing.T) {
	tests := []struct {
		value    string
		expected bool
	}{
		{"$2a$10$abcdefghijklmnopqrstuv", true},
		{"$2b$10$abcdefghijklmnopqrstuv", true},
		{"$2y$10$abcdefghijklmnopqrstuv", true},
		{"plaintext-password", false},
		{"", false},
		{"$2z$10$abcdefghijklmnopqrstuv", false},
	}
	for _, tt := range tests {
		if got := IsBcryptHash(tt.value); got != tt.expected {
			t.Errorf("IsBcryptHash(%q) = %v, want %v", tt.value, got, tt.expected)
		}
	}
}

func TestCheckPassword_LegacyPlaintext(t *testing.T) {
	if !CheckPassword("plaintext", "plaintext") {
		t.Error("Legacy plaintext comparison should match")
	}
	if CheckPassword("plaintext", "wrong") {
		t.Error("Legacy plaintext comparison should reject wrong password")
	}
	if CheckPassword("", "anything") {
		t.Error("Empty stored password should never match")
	}
}

func TestGenerateRandomSecret(t *testing.T) {
	s1, err := GenerateRandomSecret(32)
	if err != nil {
		t.Fatalf("GenerateRandomSecret failed: %v", err)
	}
	if len(s1) != 64 {
		t.Errorf("Expected 64 hex chars for 32 bytes, got %d", len(s1))
	}

	s2, _ := GenerateRandomSecret(32)
	if s1 == s2 {
		t.Error("Two generated secrets should differ")
	}
}
