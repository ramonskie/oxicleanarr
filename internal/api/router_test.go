package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ramonskie/oxicleanarr/internal/config"
	"github.com/ramonskie/oxicleanarr/internal/services"
	"github.com/ramonskie/oxicleanarr/internal/utils"
)

func setupRouter(t *testing.T, cfg *config.Config) http.Handler {
	t.Helper()
	if err := utils.InitJWT("router-test-secret-at-least-32-chars-long!!", 24*time.Hour); err != nil {
		t.Fatalf("InitJWT failed: %v", err)
	}
	config.SetTestConfig(cfg)

	authService := services.NewAuthService(cfg)
	return NewRouter(&RouterDependencies{
		AuthService: authService,
		ShutdownCh:  make(chan struct{}),
	})
}

func TestRouter_AuthRoutesRegistered(t *testing.T) {
	cfg := &config.Config{
		Admin: config.AdminConfig{Username: "admin", Password: "testpassword"},
	}
	router := setupRouter(t, cfg)

	tests := []struct {
		name   string
		method string
		path   string
	}{
		{"login", http.MethodPost, "/api/auth/login"},
		{"logout", http.MethodPost, "/api/auth/logout"},
		{"me", http.MethodGet, "/api/auth/me"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			// These are registered (not 404); auth behavior is tested elsewhere
			if w.Code == http.StatusNotFound {
				t.Errorf("Route %s %s not registered (404)", tt.method, tt.path)
			}
		})
	}
}

func TestRouter_ProtectedRouteRejectsNoToken(t *testing.T) {
	cfg := &config.Config{
		Admin: config.AdminConfig{Username: "admin", Password: "testpassword"},
	}
	router := setupRouter(t, cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/media/movies", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 for unauthenticated request, got %d", w.Code)
	}
}

func TestRouter_ProtectedRouteWithCookie(t *testing.T) {
	cfg := &config.Config{
		Admin: config.AdminConfig{Username: "admin", Password: "testpassword"},
	}
	router := setupRouter(t, cfg)

	token, err := utils.GenerateToken("admin")
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/media/movies", nil)
	req.AddCookie(&http.Cookie{Name: "oxicleanarr_token", Value: token})
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code == http.StatusUnauthorized {
		t.Errorf("Expected authenticated request to not be 401")
	}
}

func TestRouter_NoCORSByDefault(t *testing.T) {
	cfg := &config.Config{
		Admin: config.AdminConfig{Username: "admin", Password: "testpassword"},
	}
	router := setupRouter(t, cfg)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("Origin", "https://evil.example.com")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Expected no CORS header by default, got %q", got)
	}
}

func TestRouter_CORSWithConfiguredOrigins(t *testing.T) {
	cfg := &config.Config{
		Admin: config.AdminConfig{Username: "admin", Password: "testpassword"},
		Server: config.ServerConfig{
			CorsOrigins: []string{"https://media.example.com"},
		},
	}
	router := setupRouter(t, cfg)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("Origin", "https://media.example.com")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "https://media.example.com" {
		t.Errorf("Expected configured origin in CORS header, got %q", got)
	}

	// Disallowed origin should not be echoed
	req2 := httptest.NewRequest(http.MethodGet, "/health", nil)
	req2.Header.Set("Origin", "https://evil.example.com")
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)
	if got := w2.Header().Get("Access-Control-Allow-Origin"); got == "https://evil.example.com" {
		t.Error("Disallowed origin must not be echoed in CORS header")
	}
}
