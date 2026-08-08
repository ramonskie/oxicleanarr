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

// AuthCookieName is the name of the httpOnly cookie used to carry the JWT
// for the web UI. The cookie cannot be read by JavaScript, which removes the
// need to persist the token in localStorage.
const AuthCookieName = "oxicleanarr_token"

// GetTokenFromRequest extracts a JWT from the Authorization header or the
// auth cookie. The cookie is preferred for web UI / SSE connections.
func GetTokenFromRequest(r *http.Request) string {
	// Cookie (httpOnly, used by the web UI and EventSource)
	if c, err := r.Cookie(AuthCookieName); err == nil && c.Value != "" {
		return c.Value
	}

	// Authorization header (API consumers)
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return ""
	}
	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		return ""
	}
	return parts[1]
}

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

		token := GetTokenFromRequest(r)
		if token == "" {
			http.Error(w, `{"error": "Missing authorization header or cookie"}`, http.StatusUnauthorized)
			return
		}

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
