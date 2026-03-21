package integration

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"time"
)

// MockStreamystatsServer creates a mock HTTP server that simulates Streamystats API responses.
// It serves GET /api/get-item-details/{jellyfinItemId}?serverId=... returning per-item watch history.
type MockStreamystatsServer struct {
	Server         *httptest.Server
	MovieIDs       map[string]string    // maps movie title to Jellyfin ID
	WatchOverrides map[string]time.Time // per-title watch timestamp overrides (set by SetWatchTimestamp)
	HistoryMu      sync.RWMutex         // protects MovieIDs and WatchOverrides
}

// streamystatsItemDetailsResponse matches the /api/get-item-details/{id} endpoint response.
type streamystatsItemDetailsResponse struct {
	WatchHistory []streamystatsSessionItem `json:"watchHistory"`
}

// streamystatsSessionItem represents a single play session in the mock response.
type streamystatsSessionItem struct {
	UserID           string    `json:"userId"`
	LastActivityDate time.Time `json:"lastActivityDate"`
	PlayDuration     int       `json:"playDuration"` // seconds
	Completed        bool      `json:"completed"`
}

// NewMockStreamystatsServer creates and starts a new mock Streamystats server.
// The server binds to 0.0.0.0 (all interfaces) so it is reachable from Docker
// containers via host.docker.internal:<port>. The URL() method returns an
// http://127.0.0.1:<port> address so that convertMockURLForDocker can rewrite it.
func NewMockStreamystatsServer() *MockStreamystatsServer {
	mock := &MockStreamystatsServer{
		MovieIDs: make(map[string]string),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/get-item-details/", mock.handleGetItemDetails)

	listener, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		panic("mock_streamystats: failed to listen on 0.0.0.0: " + err.Error())
	}

	srv := httptest.NewUnstartedServer(mux)
	srv.Listener = listener
	srv.Start()

	_, port, _ := net.SplitHostPort(listener.Addr().String())
	srv.URL = "http://127.0.0.1:" + port

	mock.Server = srv
	return mock
}

// SetMovieIDs configures the mock server with real Jellyfin movie IDs.
// movieIDs maps movie title to Jellyfin ID (e.g., "Fight Club" -> "abc123...").
func (m *MockStreamystatsServer) SetMovieIDs(movieIDs map[string]string) {
	m.HistoryMu.Lock()
	defer m.HistoryMu.Unlock()
	m.MovieIDs = movieIDs
}

// SetWatchTimestamp overrides the watch timestamp for a specific movie title.
func (m *MockStreamystatsServer) SetWatchTimestamp(movieTitle string, watchedAt time.Time) {
	m.HistoryMu.Lock()
	defer m.HistoryMu.Unlock()

	if m.WatchOverrides == nil {
		m.WatchOverrides = make(map[string]time.Time)
	}
	m.WatchOverrides[movieTitle] = watchedAt
}

// Close shuts down the mock server.
func (m *MockStreamystatsServer) Close() {
	if m.Server != nil {
		m.Server.Close()
	}
}

// URL returns the base URL of the mock server.
func (m *MockStreamystatsServer) URL() string {
	return m.Server.URL
}

// handleGetItemDetails responds to GET /api/get-item-details/{jellyfinItemId} requests.
// Returns watch history for the requested item if it is one of the known test movies,
// or 404 if the item is not tracked (matching real Streamystats behaviour).
func (m *MockStreamystatsServer) handleGetItemDetails(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract item ID from path: /api/get-item-details/{id}
	path := strings.TrimPrefix(r.URL.Path, "/api/get-item-details/")
	jellyfinID := strings.TrimSuffix(path, "/")

	// Ping-check placeholder — return 404 (server is reachable, item unknown).
	if jellyfinID == "ping-check" {
		http.NotFound(w, r)
		return
	}

	m.HistoryMu.RLock()
	defer m.HistoryMu.RUnlock()

	now := time.Now()

	// Build reverse map: Jellyfin ID → movie title
	idToTitle := make(map[string]string, len(m.MovieIDs))
	for title, id := range m.MovieIDs {
		idToTitle[id] = title
	}

	// Per-movie default watch offsets matching the Jellystat mock for consistency.
	defaultOffsets := map[string]time.Duration{
		"Fight Club":      -10 * 24 * time.Hour,
		"Pulp Fiction":    -60 * 24 * time.Hour,
		"Inception":       -5 * 24 * time.Hour,
		"The Dark Knight": -30 * 24 * time.Hour,
		"Interstellar":    -45 * 24 * time.Hour,
		"Forrest Gump":    -90 * 24 * time.Hour,
	}

	// Per-movie default playback durations (seconds).
	defaultDurations := map[string]int{
		"Fight Club":      7200,
		"Pulp Fiction":    9240,
		"Inception":       8880,
		"The Dark Knight": 9120,
		"Interstellar":    10140,
		"Forrest Gump":    8520,
	}

	title, known := idToTitle[jellyfinID]
	if !known {
		// Use fallback IDs that match the Jellystat mock when real IDs aren't configured.
		fallbackIDs := map[string]string{
			"jellyfin-fight-club-id":   "Fight Club",
			"jellyfin-pulp-fiction-id": "Pulp Fiction",
			"jellyfin-inception-id":    "Inception",
			"jellyfin-dark-knight-id":  "The Dark Knight",
			"jellyfin-interstellar-id": "Interstellar",
			"jellyfin-forrest-gump-id": "Forrest Gump",
		}
		title, known = fallbackIDs[jellyfinID]
	}

	if !known {
		// Item not tracked — return 404 (not an error per Streamystats contract).
		http.NotFound(w, r)
		return
	}

	// Schindler's List is intentionally never watched.
	offset, hasOffset := defaultOffsets[title]
	if !hasOffset {
		http.NotFound(w, r)
		return
	}

	watchedAt := now.Add(offset)
	if override, ok := m.WatchOverrides[title]; ok {
		watchedAt = override
	}

	response := streamystatsItemDetailsResponse{
		WatchHistory: []streamystatsSessionItem{
			{
				UserID:           "user-1",
				LastActivityDate: watchedAt,
				PlayDuration:     defaultDurations[title],
				Completed:        true,
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
