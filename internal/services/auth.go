package services

import (
	"errors"

	"github.com/ramonskie/oxicleanarr/internal/config"
	"github.com/ramonskie/oxicleanarr/internal/utils"
)

var (
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrEmptyPassword      = errors.New("password must not be empty")
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

	// Check password (bcrypt hash or legacy plaintext)
	if !utils.CheckPassword(s.cfg.Admin.Password, password) {
		return "", ErrInvalidCredentials
	}

	// Generate JWT token
	token, err := utils.GenerateToken(username)
	if err != nil {
		return "", err
	}

	return token, nil
}

// ChangePassword changes the admin password.
// The new password is stored as a bcrypt hash in memory.
func (s *AuthService) ChangePassword(currentPassword, newPassword string) error {
	// Verify current password
	if !utils.CheckPassword(s.cfg.Admin.Password, currentPassword) {
		return ErrInvalidCredentials
	}

	if newPassword == "" {
		return ErrEmptyPassword
	}

	// Hash the new password before storing (never store plaintext)
	hash, err := utils.HashPassword(newPassword)
	if err != nil {
		return err
	}
	s.cfg.Admin.Password = hash

	// Note: In a complete implementation, you'd want to persist this to the config file
	// This would require access to the config file writer

	return nil
}

// ValidateToken validates a JWT token
func (s *AuthService) ValidateToken(token string) (*utils.JWTClaims, error) {
	return utils.ValidateToken(token)
}
