package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ramonskie/oxicleanarr/internal/config"
	"github.com/ramonskie/oxicleanarr/internal/utils"
)

func initTestJWT(t *testing.T) {
	t.Helper()
	if err := utils.InitJWT("middleware-test-secret-at-least-32-chars!!", 24*time.Hour); err != nil {
		t.Fatalf("InitJWT failed: %v", err)
	}
}

func makeProtectedHandler() http.Handler {
	return Auth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := GetUserFromContext(r.Context())
		if claims == nil {
			// Auth disabled - middleware passes through without claims
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("passthrough"))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("user:" + claims.Username))
	}))
}

func TestAuth_AuthorizationHeader(t *testing.T) {
	initTestJWT(t)
	token, err := utils.GenerateToken("admin")
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/media", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	makeProtectedHandler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}
	if body := w.Body.String(); body != "user:admin" {
		t.Errorf("Expected 'user:admin', got %q", body)
	}
}

func TestAuth_Cookie(t *testing.T) {
	initTestJWT(t)
	token, err := utils.GenerateToken("admin")
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/media", nil)
	req.AddCookie(&http.Cookie{Name: AuthCookieName, Value: token})
	w := httptest.NewRecorder()

	makeProtectedHandler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}
	if body := w.Body.String(); body != "user:admin" {
		t.Errorf("Expected 'user:admin', got %q", body)
	}
}

func TestAuth_MissingToken(t *testing.T) {
	initTestJWT(t)

	req := httptest.NewRequest(http.MethodGet, "/api/media", nil)
	w := httptest.NewRecorder()

	makeProtectedHandler().ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401, got %d", w.Code)
	}
}

func TestAuth_InvalidToken(t *testing.T) {
	initTestJWT(t)

	req := httptest.NewRequest(http.MethodGet, "/api/media", nil)
	req.Header.Set("Authorization", "Bearer not-a-valid-token")
	w := httptest.NewRecorder()

	makeProtectedHandler().ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401, got %d", w.Code)
	}
}

func TestAuth_MalformedHeader(t *testing.T) {
	initTestJWT(t)
	token, err := utils.GenerateToken("admin")
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	// "Basic <token>" should be rejected - only Bearer is supported
	req := httptest.NewRequest(http.MethodGet, "/api/media", nil)
	req.Header.Set("Authorization", "Basic "+token)
	w := httptest.NewRecorder()

	makeProtectedHandler().ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 for non-Bearer scheme, got %d", w.Code)
	}
}

func TestAuth_RejectsQueryParamToken(t *testing.T) {
	initTestJWT(t)
	token, err := utils.GenerateToken("admin")
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	// ?token= must NOT be accepted anymore (tokens in URLs leak to logs/history)
	req := httptest.NewRequest(http.MethodGet, "/api/logs?token="+token, nil)
	w := httptest.NewRecorder()

	makeProtectedHandler().ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 for ?token= query param, got %d", w.Code)
	}
}

func TestAuth_Disabled(t *testing.T) {
	initTestJWT(t)
	config.SetTestConfig(&config.Config{
		Admin: config.AdminConfig{DisableAuth: true},
	})
	defer config.SetTestConfig(nil)

	req := httptest.NewRequest(http.MethodGet, "/api/media", nil)
	w := httptest.NewRecorder()

	makeProtectedHandler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200 when auth disabled, got %d", w.Code)
	}
}

func TestGetTokenFromRequest_Priority(t *testing.T) {
	initTestJWT(t)
	token, err := utils.GenerateToken("admin")
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api", nil)
	req.AddCookie(&http.Cookie{Name: AuthCookieName, Value: "cookie-token"})
	req.Header.Set("Authorization", "Bearer "+token)

	if got := GetTokenFromRequest(req); got != "cookie-token" {
		t.Errorf("Expected cookie to take priority, got %q", got)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/api", nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	if got := GetTokenFromRequest(req2); got != token {
		t.Errorf("Expected header token, got %q", got)
	}

	req3 := httptest.NewRequest(http.MethodGet, "/api", nil)
	if got := GetTokenFromRequest(req3); got != "" {
		t.Errorf("Expected empty token, got %q", got)
	}
}

func makeAuthHandler() http.Handler {
	return Auth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
}

func TestAuth_AcceptsAPIKey(t *testing.T) {
	initTestJWT(t)
	config.SetTestConfig(&config.Config{
		Admin: config.AdminConfig{APIKey: "test-api-key"},
	})
	defer config.SetTestConfig(nil)

	req := httptest.NewRequest(http.MethodGet, "/api/media", nil)
	req.Header.Set("Authorization", "Bearer test-api-key")
	w := httptest.NewRecorder()

	makeAuthHandler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200 with API key, got %d", w.Code)
	}
}

func TestAuth_RejectsWrongAPIKey(t *testing.T) {
	initTestJWT(t)
	config.SetTestConfig(&config.Config{
		Admin: config.AdminConfig{APIKey: "test-api-key"},
	})
	defer config.SetTestConfig(nil)

	req := httptest.NewRequest(http.MethodGet, "/api/media", nil)
	req.Header.Set("Authorization", "Bearer wrong-key")
	w := httptest.NewRecorder()

	makeAuthHandler().ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 with wrong API key, got %d", w.Code)
	}
}

func TestAuth_APIKeyHeaderNotShadowedByStaleCookie(t *testing.T) {
	initTestJWT(t)
	config.SetTestConfig(&config.Config{
		Admin: config.AdminConfig{APIKey: "test-api-key"},
	})
	defer config.SetTestConfig(nil)

	req := httptest.NewRequest(http.MethodGet, "/api/media", nil)
	req.AddCookie(&http.Cookie{Name: AuthCookieName, Value: "stale-invalid-cookie"})
	req.Header.Set("Authorization", "Bearer test-api-key")
	w := httptest.NewRecorder()

	makeAuthHandler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200 with API key header despite stale cookie, got %d", w.Code)
	}
}

