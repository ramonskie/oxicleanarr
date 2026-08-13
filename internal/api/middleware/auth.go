package middleware

import (
	"context"
	"crypto/subtle"
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

// Auth is a middleware that validates JWT tokens or the configured admin API key.
// If admin.disable_auth is true in config, authentication is bypassed.
func Auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if authentication is disabled
		cfg := config.Get()
		if cfg != nil && cfg.Admin.DisableAuth {
			log.Debug().Msg("Authentication disabled, bypassing auth middleware")
			next.ServeHTTP(w, r)
			return
		}

		var apiKey string
		if cfg != nil {
			apiKey = cfg.Admin.APIKey
		}

		// Try the cookie first (web UI), then the Authorization Bearer header, so a
		// valid API key in the header is not shadowed by a stale cookie.
		candidates := make([]string, 0, 2)
		if token := GetTokenFromRequest(r); token != "" {
			candidates = append(candidates, token)
		}
		// GetTokenFromRequest already falls back to the Bearer header when there is
		// no cookie, so only add the header token explicitly if it differs.
		if authHeader := r.Header.Get("Authorization"); authHeader != "" {
			if parts := strings.SplitN(authHeader, " ", 2); len(parts) == 2 && parts[0] == "Bearer" {
				if len(candidates) == 0 || candidates[0] != parts[1] {
					candidates = append(candidates, parts[1])
				}
			}
		}

		for _, token := range candidates {
			// Validate JWT (web UI / scripted login).
			if claims, err := utils.ValidateToken(token); err == nil {
				// Add claims to request context
				ctx := context.WithValue(r.Context(), userContextKey, claims)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// Fall back to the static admin API key for machine clients.
			// NOTE: this path does not inject user claims into the request context
			// (there is no user); GetUserFromContext returns nil for such requests.
			if apiKey != "" && subtle.ConstantTimeCompare([]byte(token), []byte(apiKey)) == 1 {
				next.ServeHTTP(w, r)
				return
			}
		}

		if len(candidates) == 0 {
			http.Error(w, `{"error": "Missing authorization header or cookie"}`, http.StatusUnauthorized)
			return
		}

		log.Debug().Msg("Invalid token or API key")
		http.Error(w, `{"error": "Invalid or expired token"}`, http.StatusUnauthorized)
		return
	})
}

// GetUserFromContext retrieves the user claims from the request context
func GetUserFromContext(ctx context.Context) *utils.JWTClaims {
	if claims, ok := ctx.Value(userContextKey).(*utils.JWTClaims); ok {
		return claims
	}
	return nil
}
