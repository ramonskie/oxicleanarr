package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ramonskie/oxicleanarr/internal/config"
	"github.com/ramonskie/oxicleanarr/internal/services"
	"github.com/ramonskie/oxicleanarr/internal/utils"
	"golang.org/x/crypto/bcrypt"
)

func setupAuthHandler(t *testing.T) (*AuthHandler, *config.Config) {
	t.Helper()

	// Initialize JWT for testing
	utils.InitJWT("test-secret-key-for-testing-min-32-chars", 24*time.Hour)

	// Create test config with hashed password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte("testpassword"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}

	cfg := &config.Config{
		Admin: config.AdminConfig{
			Username: "admin",
			Password: string(hashedPassword),
		},
	}

	authService := services.NewAuthService(cfg)
	handler := NewAuthHandler(authService)

	return handler, cfg
}

func TestAuthHandler_Login(t *testing.T) {
	tests := []struct {
		name           string
		body           interface{}
		expectedStatus int
		checkToken     bool
	}{
		{
			name: "successful login with valid credentials",
			body: LoginRequest{
				Username: "admin",
				Password: "testpassword",
			},
			expectedStatus: http.StatusOK,
			checkToken:     true,
		},
		{
			name: "rejects invalid password",
			body: LoginRequest{
				Username: "admin",
				Password: "wrongpassword",
			},
			expectedStatus: http.StatusUnauthorized,
			checkToken:     false,
		},
		{
			name: "rejects invalid username",
			body: LoginRequest{
				Username: "wronguser",
				Password: "testpassword",
			},
			expectedStatus: http.StatusUnauthorized,
			checkToken:     false,
		},
		{
			name: "rejects empty username",
			body: LoginRequest{
				Username: "",
				Password: "testpassword",
			},
			expectedStatus: http.StatusUnauthorized,
			checkToken:     false,
		},
		{
			name: "rejects empty password",
			body: LoginRequest{
				Username: "admin",
				Password: "",
			},
			expectedStatus: http.StatusUnauthorized,
			checkToken:     false,
		},
		{
			name:           "rejects invalid JSON",
			body:           `{"username": "admin"`, // Invalid JSON
			expectedStatus: http.StatusBadRequest,
			checkToken:     false,
		},
		{
			name:           "rejects empty request body",
			body:           LoginRequest{},
			expectedStatus: http.StatusUnauthorized,
			checkToken:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler, _ := setupAuthHandler(t)

			// Create request body
			var body []byte
			var err error
			if str, ok := tt.body.(string); ok {
				body = []byte(str)
			} else {
				body, err = json.Marshal(tt.body)
				if err != nil {
					t.Fatalf("Failed to marshal request body: %v", err)
				}
			}

			req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handler.Login(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			// Check response content type
			if contentType := w.Header().Get("Content-Type"); contentType != "application/json" {
				t.Errorf("Expected Content-Type application/json, got %s", contentType)
			}

			// Check token in response
			if tt.checkToken {
				var resp LoginResponse
				if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				if resp.Token == "" {
					t.Error("Expected non-empty token in response")
				}

				// Validate the token structure (basic check)
				if len(resp.Token) < 20 {
					t.Error("Token appears to be too short")
				}
			}

			// Check error message for failed logins
			if tt.expectedStatus == http.StatusUnauthorized {
				var resp ErrorResponse
				if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
					t.Fatalf("Failed to decode error response: %v", err)
				}

				if resp.Error == "" {
					t.Error("Expected error message in response")
				}
			}

			// Check error message for bad requests
			if tt.expectedStatus == http.StatusBadRequest {
				var resp ErrorResponse
				if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
					t.Fatalf("Failed to decode error response: %v", err)
				}

				if resp.Error != "Invalid request body" {
					t.Errorf("Expected 'Invalid request body' error, got %s", resp.Error)
				}
			}
		})
	}
}

func TestAuthHandler_Login_TokenValidation(t *testing.T) {
	handler, _ := setupAuthHandler(t)

	// Perform successful login
	loginReq := LoginRequest{
		Username: "admin",
		Password: "testpassword",
	}

	body, err := json.Marshal(loginReq)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Login(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}

	var resp LoginResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Validate the token using the auth service
	claims, err := handler.authService.ValidateToken(resp.Token)
	if err != nil {
		t.Fatalf("Token validation failed: %v", err)
	}

	if claims == nil {
		t.Fatal("Expected non-nil claims")
	}

	if claims.Username != "admin" {
		t.Errorf("Expected username 'admin' in token claims, got %s", claims.Username)
	}
}

func TestAuthHandler_Login_ConcurrentRequests(t *testing.T) {
	handler, _ := setupAuthHandler(t)

	// Test that concurrent login requests work correctly
	const numRequests = 10
	done := make(chan bool, numRequests)

	for i := 0; i < numRequests; i++ {
		go func() {
			loginReq := LoginRequest{
				Username: "admin",
				Password: "testpassword",
			}

			body, err := json.Marshal(loginReq)
			if err != nil {
				t.Errorf("Failed to marshal request: %v", err)
				done <- false
				return
			}

			req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handler.Login(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("Expected status 200, got %d", w.Code)
				done <- false
				return
			}

			var resp LoginResponse
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Errorf("Failed to decode response: %v", err)
				done <- false
				return
			}

			if resp.Token == "" {
				t.Error("Expected non-empty token")
				done <- false
				return
			}

			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < numRequests; i++ {
		success := <-done
		if !success {
			t.Error("One or more concurrent requests failed")
		}
	}
}
