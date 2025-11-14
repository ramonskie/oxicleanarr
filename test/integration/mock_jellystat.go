package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"time"
)

// MockJellystatServer creates a mock HTTP server that simulates Jellystat API responses
type MockJellystatServer struct {
	Server    *httptest.Server
	MovieIDs  map[string]string // maps movie title to Jellyfin ID
	HistoryMu sync.RWMutex      // protects MovieIDs
}

// jellystatHistoryResponse matches the /api/getHistory endpoint response
// This matches clients.JellystatHistoryResponse from internal/clients/types.go
type jellystatHistoryResponse struct {
	CurrentPage int                    `json:"current_page"`
	Pages       int                    `json:"pages"`
	Size        int                    `json:"size"`
	Results     []jellystatHistoryItem `json:"results"`
}

// jellystatHistoryItem matches clients.JellystatHistoryItem
type jellystatHistoryItem struct {
	ID                   string    `json:"Id"`
	UserID               string    `json:"UserId"`
	UserName             string    `json:"UserName"`
	NowPlayingItemID     string    `json:"NowPlayingItemId"`
	NowPlayingItemName   string    `json:"NowPlayingItemName"`
	SeriesName           string    `json:"SeriesName"`
	EpisodeID            string    `json:"EpisodeId"`
	SeasonID             string    `json:"SeasonId"`
	PlaybackDuration     int       `json:"PlaybackDuration"`
	ActivityDateInserted time.Time `json:"ActivityDateInserted"`
}

// NewMockJellystatServer creates and starts a new mock Jellystat server
func NewMockJellystatServer() *MockJellystatServer {
	mock := &MockJellystatServer{
		MovieIDs: make(map[string]string),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", mock.handleRoot)
	mux.HandleFunc("/api/getHistory", mock.handleGetHistory)

	mock.Server = httptest.NewServer(mux)
	return mock
}

// SetMovieIDs configures the mock server with real Jellyfin movie IDs
// movieIDs maps movie title to Jellyfin ID (e.g., "Fight Club" -> "abc123...")
func (m *MockJellystatServer) SetMovieIDs(movieIDs map[string]string) {
	m.HistoryMu.Lock()
	defer m.HistoryMu.Unlock()
	m.MovieIDs = movieIDs
}

// Close shuts down the mock server
func (m *MockJellystatServer) Close() {
	if m.Server != nil {
		m.Server.Close()
	}
}

// URL returns the base URL of the mock server
func (m *MockJellystatServer) URL() string {
	return m.Server.URL
}

// handleRoot responds to / requests (health check)
func (m *MockJellystatServer) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("<html><body>Mock Jellystat</body></html>"))
}

// handleGetHistory responds to /api/getHistory requests with test watch history data
// This matches the actual Jellystat API format that OxiCleanarr expects
func (m *MockJellystatServer) handleGetHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	m.HistoryMu.RLock()
	defer m.HistoryMu.RUnlock()

	// Generate mock watch history for our 7 test movies
	// Maps different watch timestamps to test various retention scenarios
	now := time.Now()

	// Get real Jellyfin IDs (or use fallback if not configured)
	getFallbackID := func(title, fallback string) string {
		if id, ok := m.MovieIDs[title]; ok {
			return id
		}
		return fallback
	}

	// Note: NowPlayingItemID should match Jellyfin item IDs from the actual test setup
	// These will be matched during syncJellystat by comparing against media.JellyfinID
	historyItems := []jellystatHistoryItem{
		// Fight Club - watched 10 days ago
		{
			ID:                   "hist-1",
			UserID:               "user-1",
			UserName:             "testuser",
			NowPlayingItemID:     getFallbackID("Fight Club", "jellyfin-fight-club-id"),
			NowPlayingItemName:   "Fight Club",
			SeriesName:           "",
			EpisodeID:            "",
			SeasonID:             "",
			PlaybackDuration:     7200,
			ActivityDateInserted: now.Add(-10 * 24 * time.Hour),
		},
		// Pulp Fiction - watched 60 days ago (old)
		{
			ID:                   "hist-2",
			UserID:               "user-1",
			UserName:             "testuser",
			NowPlayingItemID:     getFallbackID("Pulp Fiction", "jellyfin-pulp-fiction-id"),
			NowPlayingItemName:   "Pulp Fiction",
			SeriesName:           "",
			EpisodeID:            "",
			SeasonID:             "",
			PlaybackDuration:     9240,
			ActivityDateInserted: now.Add(-60 * 24 * time.Hour),
		},
		// Inception - watched 5 days ago (recent)
		{
			ID:                   "hist-3",
			UserID:               "user-2",
			UserName:             "testuser2",
			NowPlayingItemID:     getFallbackID("Inception", "jellyfin-inception-id"),
			NowPlayingItemName:   "Inception",
			SeriesName:           "",
			EpisodeID:            "",
			SeasonID:             "",
			PlaybackDuration:     8880,
			ActivityDateInserted: now.Add(-5 * 24 * time.Hour),
		},
		// The Dark Knight - watched 30 days ago
		{
			ID:                   "hist-4",
			UserID:               "user-1",
			UserName:             "testuser",
			NowPlayingItemID:     getFallbackID("The Dark Knight", "jellyfin-dark-knight-id"),
			NowPlayingItemName:   "The Dark Knight",
			SeriesName:           "",
			EpisodeID:            "",
			SeasonID:             "",
			PlaybackDuration:     9120,
			ActivityDateInserted: now.Add(-30 * 24 * time.Hour),
		},
		// Interstellar - watched 45 days ago
		{
			ID:                   "hist-5",
			UserID:               "user-3",
			UserName:             "vipuser",
			NowPlayingItemID:     getFallbackID("Interstellar", "jellyfin-interstellar-id"),
			NowPlayingItemName:   "Interstellar",
			SeriesName:           "",
			EpisodeID:            "",
			SeasonID:             "",
			PlaybackDuration:     10140,
			ActivityDateInserted: now.Add(-45 * 24 * time.Hour),
		},
		// Forrest Gump - watched 90 days ago (very old)
		{
			ID:                   "hist-6",
			UserID:               "user-3",
			UserName:             "vipuser",
			NowPlayingItemID:     getFallbackID("Forrest Gump", "jellyfin-forrest-gump-id"),
			NowPlayingItemName:   "Forrest Gump",
			SeriesName:           "",
			EpisodeID:            "",
			SeasonID:             "",
			PlaybackDuration:     8520,
			ActivityDateInserted: now.Add(-90 * 24 * time.Hour),
		},
		// Schindler's List - never watched (no activity record)
		// This is intentionally omitted to test unwatched media behavior
	}

	// Return paginated response (single page for simplicity)
	response := jellystatHistoryResponse{
		CurrentPage: 1,
		Pages:       1,
		Size:        len(historyItems),
		Results:     historyItems,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
