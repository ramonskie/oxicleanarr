package utils

import (
	"errors"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	jwtSecret []byte
	jwtExpiry time.Duration
)

// ErrJWTNotInitialized is returned when token operations are attempted before InitJWT
var ErrJWTNotInitialized = errors.New("JWT not initialized")

// JWTClaims represents the JWT claims
type JWTClaims struct {
	Username string `json:"username"`
	jwt.RegisteredClaims
}

// InitJWT initializes JWT settings.
// The secret must be explicitly provided (via argument or the JWT_SECRET env var);
// there is no hardcoded fallback secret.
func InitJWT(secret string, expiry time.Duration) error {
	if secret == "" {
		secret = os.Getenv("JWT_SECRET")
	}
	if secret == "" {
		return errors.New("JWT secret is required: set JWT_SECRET env var or pass a secret to InitJWT")
	}
	jwtSecret = []byte(secret)

	if expiry == 0 {
		jwtExpiry = 24 * time.Hour
	} else {
		jwtExpiry = expiry
	}
	return nil
}

// GetJWTExpiry returns the configured token expiry duration
func GetJWTExpiry() time.Duration {
	return jwtExpiry
}

// GenerateToken generates a new JWT token for a user
func GenerateToken(username string) (string, error) {
	if jwtSecret == nil {
		return "", ErrJWTNotInitialized
	}

	claims := JWTClaims{
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(jwtExpiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

// ValidateToken validates a JWT token and returns the claims
func ValidateToken(tokenString string) (*JWTClaims, error) {
	if jwtSecret == nil {
		return nil, ErrJWTNotInitialized
	}

	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("invalid signing method")
		}
		return jwtSecret, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*JWTClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token")
}
