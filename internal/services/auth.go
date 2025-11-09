package services

import (
	"errors"

	"github.com/ramonskie/oxicleanarr/internal/config"
	"github.com/ramonskie/oxicleanarr/internal/utils"
)

var (
	ErrInvalidCredentials = errors.New("invalid username or password")
)

// AuthService handles authentication operations
type AuthService struct {
	cfg *config.Config
}

// NewAuthService creates a new AuthService
func NewAuthService(cfg *config.Config) *AuthService {
	return &AuthService{
		cfg: cfg,
	}
}

// Login authenticates a user and returns a JWT token
func (s *AuthService) Login(username, password string) (string, error) {
	// Check username
	if username != s.cfg.Admin.Username {
		return "", ErrInvalidCredentials
	}

	// Check password (plain text comparison)
	if password != s.cfg.Admin.Password {
		return "", ErrInvalidCredentials
	}

	// Generate JWT token
	token, err := utils.GenerateToken(username)
	if err != nil {
		return "", err
	}

	return token, nil
}

// ChangePassword changes the admin password
func (s *AuthService) ChangePassword(currentPassword, newPassword string) error {
	// Verify current password (plain text comparison)
	if currentPassword != s.cfg.Admin.Password {
		return ErrInvalidCredentials
	}

	// Update config (in memory) - plain text
	s.cfg.Admin.Password = newPassword

	// Note: In a complete implementation, you'd want to persist this to the config file
	// This would require access to the config file writer

	return nil
}

// ValidateToken validates a JWT token
func (s *AuthService) ValidateToken(token string) (*utils.JWTClaims, error) {
	return utils.ValidateToken(token)
}
