package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/ramonskie/oxicleanarr/internal/config"
	"github.com/ramonskie/oxicleanarr/internal/utils"
	"github.com/rs/zerolog/log"
)

type contextKey string

const (
	userContextKey contextKey = "user"
)

// Auth is a middleware that validates JWT tokens
// If admin.disable_auth is true in config, authentication is bypassed
func Auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if authentication is disabled
		cfg := config.Get()
		if cfg != nil && cfg.Admin.DisableAuth {
			log.Debug().Msg("Authentication disabled, bypassing auth middleware")
			next.ServeHTTP(w, r)
			return
		}

		// Get token from Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, `{"error": "Missing authorization header"}`, http.StatusUnauthorized)
			return
		}

		// Extract token from "Bearer <token>"
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			http.Error(w, `{"error": "Invalid authorization header format"}`, http.StatusUnauthorized)
			return
		}

		token := parts[1]

		// Validate token
		claims, err := utils.ValidateToken(token)
		if err != nil {
			log.Debug().Err(err).Msg("Invalid token")
			http.Error(w, `{"error": "Invalid or expired token"}`, http.StatusUnauthorized)
			return
		}

		// Add claims to request context
		ctx := context.WithValue(r.Context(), userContextKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetUserFromContext retrieves the user claims from the request context
func GetUserFromContext(ctx context.Context) *utils.JWTClaims {
	if claims, ok := ctx.Value(userContextKey).(*utils.JWTClaims); ok {
		return claims
	}
	return nil
}
