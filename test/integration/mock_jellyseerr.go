package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"time"
)

// MockJellyseerrServer creates a mock HTTP server that simulates Jellyseerr API responses
type MockJellyseerrServer struct {
	Server *httptest.Server
}

// jellyseerrStatusResponse matches the /api/v1/status endpoint response
type jellyseerrStatusResponse struct {
	Version string `json:"version"`
}

// jellyseerrRequestResponse matches the /api/v1/request endpoint response
type jellyseerrRequestResponse struct {
	PageInfo struct {
		Pages    int `json:"pages"`
		PageSize int `json:"pageSize"`
		Results  int `json:"results"`
		Page     int `json:"page"`
	} `json:"pageInfo"`
	Results []jellyseerrRequest `json:"results"`
}

type jellyseerrRequest struct {
	ID          int                 `json:"id"`
	Status      int                 `json:"status"` // 2 = approved, 3 = available
	CreatedAt   string              `json:"createdAt"`
	UpdatedAt   string              `json:"updatedAt"`
	Type        string              `json:"type"` // "movie" or "tv"
	RequestedBy jellyseerrUser      `json:"requestedBy"`
	Media       jellyseerrMediaInfo `json:"media"`
}

type jellyseerrUser struct {
	ID          int    `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"displayName"`
}

type jellyseerrMediaInfo struct {
	ID        int    `json:"id"`
	TmdbID    int    `json:"tmdbId"`
	TvdbID    int    `json:"tvdbId,omitempty"`
	Status    int    `json:"status"` // 4 = available, 3 = processing
	MediaType string `json:"mediaType"`
}

// NewMockJellyseerrServer creates and starts a new mock Jellyseerr server
func NewMockJellyseerrServer() *MockJellyseerrServer {
	mock := &MockJellyseerrServer{}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/status", mock.handleStatus)
	mux.HandleFunc("/api/v1/request", mock.handleRequests)

	mock.Server = httptest.NewServer(mux)
	return mock
}

// Close shuts down the mock server
func (m *MockJellyseerrServer) Close() {
	if m.Server != nil {
		m.Server.Close()
	}
}

// URL returns the base URL of the mock server
func (m *MockJellyseerrServer) URL() string {
	return m.Server.URL
}

// handleStatus responds to /api/v1/status requests
func (m *MockJellyseerrServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	response := jellyseerrStatusResponse{
		Version: "1.9.0-mock",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleRequests responds to /api/v1/request requests with test data
func (m *MockJellyseerrServer) handleRequests(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Generate mock request data for our 7 test movies
	// Maps users to movies with different request dates
	now := time.Now()

	requests := []jellyseerrRequest{
		// Trial user (ID 100) - requested Fight Club 5 days ago
		{
			ID:        1,
			Status:    3, // available
			CreatedAt: now.Add(-5 * 24 * time.Hour).Format(time.RFC3339),
			UpdatedAt: now.Add(-5 * 24 * time.Hour).Format(time.RFC3339),
			Type:      "movie",
			RequestedBy: jellyseerrUser{
				ID:          100,
				Email:       "trial@example.com",
				DisplayName: "Trial User",
			},
			Media: jellyseerrMediaInfo{
				ID:        1,
				TmdbID:    550, // Fight Club
				Status:    4,   // available
				MediaType: "movie",
			},
		},
		// Trial user - requested Pulp Fiction 15 days ago
		{
			ID:        2,
			Status:    3,
			CreatedAt: now.Add(-15 * 24 * time.Hour).Format(time.RFC3339),
			UpdatedAt: now.Add(-15 * 24 * time.Hour).Format(time.RFC3339),
			Type:      "movie",
			RequestedBy: jellyseerrUser{
				ID:          100,
				Email:       "trial@example.com",
				DisplayName: "Trial User",
			},
			Media: jellyseerrMediaInfo{
				ID:        2,
				TmdbID:    680, // Pulp Fiction
				Status:    4,
				MediaType: "movie",
			},
		},
		// Premium user (ID 200) - requested Inception 10 days ago
		{
			ID:        3,
			Status:    3,
			CreatedAt: now.Add(-10 * 24 * time.Hour).Format(time.RFC3339),
			UpdatedAt: now.Add(-10 * 24 * time.Hour).Format(time.RFC3339),
			Type:      "movie",
			RequestedBy: jellyseerrUser{
				ID:          200,
				Email:       "premium@example.com",
				DisplayName: "Premium User",
			},
			Media: jellyseerrMediaInfo{
				ID:        3,
				TmdbID:    27205, // Inception
				Status:    4,
				MediaType: "movie",
			},
		},
		// Premium user - requested The Dark Knight 25 days ago
		{
			ID:        4,
			Status:    3,
			CreatedAt: now.Add(-25 * 24 * time.Hour).Format(time.RFC3339),
			UpdatedAt: now.Add(-25 * 24 * time.Hour).Format(time.RFC3339),
			Type:      "movie",
			RequestedBy: jellyseerrUser{
				ID:          200,
				Email:       "premium@example.com",
				DisplayName: "Premium User",
			},
			Media: jellyseerrMediaInfo{
				ID:        4,
				TmdbID:    155, // The Dark Knight
				Status:    4,
				MediaType: "movie",
			},
		},
		// VIP user (ID 300) - requested Interstellar 20 days ago
		{
			ID:        5,
			Status:    3,
			CreatedAt: now.Add(-20 * 24 * time.Hour).Format(time.RFC3339),
			UpdatedAt: now.Add(-20 * 24 * time.Hour).Format(time.RFC3339),
			Type:      "movie",
			RequestedBy: jellyseerrUser{
				ID:          300,
				Email:       "vip@example.com",
				DisplayName: "VIP User",
			},
			Media: jellyseerrMediaInfo{
				ID:        5,
				TmdbID:    157336, // Interstellar
				Status:    4,
				MediaType: "movie",
			},
		},
		// VIP user - requested Forrest Gump 40 days ago
		{
			ID:        6,
			Status:    3,
			CreatedAt: now.Add(-40 * 24 * time.Hour).Format(time.RFC3339),
			UpdatedAt: now.Add(-40 * 24 * time.Hour).Format(time.RFC3339),
			Type:      "movie",
			RequestedBy: jellyseerrUser{
				ID:          300,
				Email:       "vip@example.com",
				DisplayName: "VIP User",
			},
			Media: jellyseerrMediaInfo{
				ID:        6,
				TmdbID:    13, // Forrest Gump
				Status:    4,
				MediaType: "movie",
			},
		},
		// No request for Schindler's List (tmdbId: 424) - tests media without requests
	}

	response := jellyseerrRequestResponse{
		Results: requests,
	}
	response.PageInfo.Pages = 1
	response.PageInfo.PageSize = 20
	response.PageInfo.Results = len(requests)
	response.PageInfo.Page = 1

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
