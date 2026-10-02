package integration

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"time"
)

const (
	// tracearrMockDefaultAPIKey is the bearer key the mock accepts unless a test
	// overrides it with SetAPIKey. The prefix mirrors real Tracearr keys
	// (trr_pub_<base64url>).
	tracearrMockDefaultAPIKey = "trr_pub_mock_key"

	// tracearrMockMaxPageSize is the real v2 public API's pageSize ceiling.
	tracearrMockMaxPageSize = 100

	// tracearrMockDefaultPageSize is deliberately smaller than the 100 the client
	// always requests so the client's cursor pagination loop is exercised across
	// multiple pages even with a handful of fixtures. Raise it with SetPageSize
	// when a test needs a single page.
	tracearrMockDefaultPageSize = 3
)

// TracearrHistoryRecord is a single row from the Tracearr v2 public history
// endpoint. It carries the fields OxiCleanarr consumes (rating_key, server_type,
// started_at, duration_ms) plus the identifying metadata integration tests assert on.
type TracearrHistoryRecord struct {
	RatingKey  string     `json:"rating_key"`
	ServerID   string     `json:"server_id"`
	ServerType string     `json:"server_type"`
	MediaType  string     `json:"media_type"`
	StartedAt  time.Time  `json:"started_at"`
	StoppedAt  *time.Time `json:"stopped_at"`
	DurationMS int        `json:"duration_ms"`
	Watched    bool       `json:"watched"`
	State      string     `json:"state"`
	IMDbID     string     `json:"imdb_id"`
	TMDbID     string     `json:"tmdb_id"`
	TVDbID     string     `json:"tvdb_id"`
	MediaID    string     `json:"media_id"`
}

// tracearrHistoryMeta mirrors the meta envelope: {nextCursor,total,pageSize}.
// NextCursor is a pointer so the terminal page serialises as JSON null.
type tracearrHistoryMeta struct {
	NextCursor *string `json:"nextCursor"`
	Total      int     `json:"total"`
	PageSize   int     `json:"pageSize"`
}

// tracearrHistoryResponse is the /api/v2/public/history envelope.
type tracearrHistoryResponse struct {
	Data []TracearrHistoryRecord `json:"data"`
	Meta tracearrHistoryMeta     `json:"meta"`
}

// tracearrHealthServer is one media-server entry in the v1 health response.
// ID is the media-server UUID that the client uses as serverId/server_id.
type tracearrHealthServer struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Type          string `json:"type"`
	Online        bool   `json:"online"`
	Historical    bool   `json:"historical"`
	ActiveStreams int    `json:"activeStreams"`
}

// tracearrHealthResponse simulates GET /api/v1/public/health. Health is a v1
// endpoint; the v2 public API has no health route.
type tracearrHealthResponse struct {
	Status    string                 `json:"status"`
	Version   string                 `json:"version"`
	Timestamp string                 `json:"timestamp"`
	Servers   []tracearrHealthServer `json:"servers"`
}

// MockTracearrServer creates a mock HTTP server that simulates the Tracearr
// public API. It serves GET /api/v1/public/health and
// GET /api/v2/public/history?pageSize=&cursor= (bearer-keyed, cursor-paginated).
type MockTracearrServer struct {
	Server         *httptest.Server
	historyMu      sync.RWMutex            // protects apiKey, pageSize, records, movieIDs and watchOverrides
	apiKey         string                  // accepted bearer key
	pageSize       int                     // records returned per page
	records        []TracearrHistoryRecord // full ordered fixture set
	movieIDs       map[string]string       // maps movie title to Jellyfin item GUID
	watchOverrides map[string]time.Time    // per-title started_at overrides (set by SetWatchTimestamp)
}

// NewMockTracearrServer creates and starts a new mock Tracearr server.
// The server binds to 0.0.0.0 (all interfaces) so it is reachable from Docker
// containers via host.docker.internal:<port>. The URL() method returns an
// http://127.0.0.1:<port> address so that convertMockURLForDocker can rewrite it.
func NewMockTracearrServer() *MockTracearrServer {
	mock := &MockTracearrServer{
		apiKey:         tracearrMockDefaultAPIKey,
		pageSize:       tracearrMockDefaultPageSize,
		records:        DefaultTracearrHistoryRecords(),
		movieIDs:       make(map[string]string),
		watchOverrides: make(map[string]time.Time),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/public/health", mock.handleHealth)
	mux.HandleFunc("/api/v2/public/history", mock.handleHistory)

	listener, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		panic("mock_tracearr: failed to listen on 0.0.0.0: " + err.Error())
	}

	srv := httptest.NewUnstartedServer(mux)
	srv.Listener = listener
	srv.Start()

	_, port, _ := net.SplitHostPort(listener.Addr().String())
	srv.URL = "http://127.0.0.1:" + port

	mock.Server = srv
	return mock
}

// SetAPIKey overrides the bearer key the mock accepts. Tests configure the
// OxiCleanarr Tracearr api_key to match this value.
func (m *MockTracearrServer) SetAPIKey(apiKey string) {
	m.historyMu.Lock()
	defer m.historyMu.Unlock()
	m.apiKey = apiKey
}

// APIKey returns the bearer key currently accepted by the mock.
func (m *MockTracearrServer) APIKey() string {
	m.historyMu.RLock()
	defer m.historyMu.RUnlock()
	return m.apiKey
}

// SetHistoryRecords replaces the ordered history fixtures served by the mock.
// The slice is copied so later caller-side mutation cannot race the handler's
// locked reads.
func (m *MockTracearrServer) SetHistoryRecords(records []TracearrHistoryRecord) {
	m.historyMu.Lock()
	defer m.historyMu.Unlock()
	m.records = append([]TracearrHistoryRecord(nil), records...)
}

// SetMovieIDs configures the mock with real Jellyfin item GUIDs and rebuilds the
// scenario fixtures. movieIDs maps movie title to Jellyfin GUID (e.g.
// "Fight Club" -> "abc123..."). Titles with no mapped GUID are omitted because
// the client cannot reconcile them against the media library. The map is copied
// so later caller-side mutation cannot race the handler's locked reads.
func (m *MockTracearrServer) SetMovieIDs(movieIDs map[string]string) {
	m.historyMu.Lock()
	defer m.historyMu.Unlock()
	copied := make(map[string]string, len(movieIDs))
	for title, id := range movieIDs {
		copied[title] = id
	}
	m.movieIDs = copied
	m.records = m.buildScenarioRecordsLocked()
}

// SetWatchTimestamp overrides the started_at value for a specific movie title
// and rebuilds the scenario fixtures. This lets individual test scenarios
// simulate re-watch events without rebuilding the entire mock server. Pass the
// movie title as it appears in the scenario dataset (e.g. "Pulp Fiction").
func (m *MockTracearrServer) SetWatchTimestamp(title string, watchedAt time.Time) {
	m.historyMu.Lock()
	defer m.historyMu.Unlock()
	if m.watchOverrides == nil {
		m.watchOverrides = make(map[string]time.Time)
	}
	m.watchOverrides[title] = watchedAt
	m.records = m.buildScenarioRecordsLocked()
}

// SetPageSize controls how many records a single page returns, clamped to
// 1..tracearrMockMaxPageSize. Lower it to force multi-page cursor traversal.
func (m *MockTracearrServer) SetPageSize(size int) {
	m.historyMu.Lock()
	defer m.historyMu.Unlock()
	m.pageSize = clampTracearrPageSize(size)
}

// Close shuts down the mock server.
func (m *MockTracearrServer) Close() {
	if m.Server != nil {
		m.Server.Close()
	}
}

// URL returns the base URL of the mock server.
func (m *MockTracearrServer) URL() string {
	return m.Server.URL
}

// handleHealth responds to GET /api/v1/public/health with a 200 JSON body
// listing the mock media servers. The single jellyfin entry carries
// tracearrTestServerID so zero-config auto-detection resolves to it. A missing
// or wrong bearer key yields 401.
func (m *MockTracearrServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !m.authorized(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tracearrHealthResponse{
		Status:    "ok",
		Version:   "2.0.0-mock",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Servers: []tracearrHealthServer{
			{
				ID:            tracearrTestServerID,
				Name:          "Mock Jellyfin",
				Type:          "jellyfin",
				Online:        true,
				Historical:    true,
				ActiveStreams: 0,
			},
			{
				ID:            "mock-plex-1",
				Name:          "Mock Plex",
				Type:          "plex",
				Online:        true,
				Historical:    true,
				ActiveStreams: 0,
			},
		},
	})
}

// handleHistory responds to GET /api/v2/public/history. It honours the pageSize
// query parameter (capped at 100) and an opaque cursor, returning the next slice
// of fixtures and a meta.nextCursor that is null on the final page. The server
// always emits the fixtures in their configured order, so a caller that follows
// nextCursor to completion sees every row exactly once.
func (m *MockTracearrServer) handleHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !m.authorized(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	m.historyMu.RLock()
	requestedPageSize := parseTracearrPageSize(r.URL.Query().Get("pageSize"))
	effectivePageSize := m.pageSize
	if requestedPageSize > 0 && requestedPageSize < effectivePageSize {
		effectivePageSize = requestedPageSize
	}
	cursor := r.URL.Query().Get("cursor")
	records := append([]TracearrHistoryRecord(nil), m.records...)
	m.historyMu.RUnlock()

	// Enforce media-server scoping. Real Tracearr scopes history to the
	// serverId query param; a client that omits or mis-scopes it must receive
	// no history rather than another server's rows. Respond 200 with an empty
	// data array so the client sees a valid, empty page.
	if r.URL.Query().Get("serverId") != tracearrTestServerID {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tracearrHistoryResponse{
			Data: []TracearrHistoryRecord{},
			Meta: tracearrHistoryMeta{Total: 0, PageSize: effectivePageSize},
		})
		return
	}

	start, err := decodeTracearrCursor(cursor)
	if err != nil {
		http.Error(w, "invalid cursor", http.StatusBadRequest)
		return
	}
	if start > len(records) {
		start = len(records)
	}
	end := start + effectivePageSize
	if end > len(records) {
		end = len(records)
	}

	var nextCursor *string
	if end < len(records) {
		next := encodeTracearrCursor(end)
		nextCursor = &next
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tracearrHistoryResponse{
		Data: records[start:end],
		Meta: tracearrHistoryMeta{
			NextCursor: nextCursor,
			Total:      len(records),
			PageSize:   effectivePageSize,
		},
	})
}

// authorized reports whether the request carries the configured bearer key.
func (m *MockTracearrServer) authorized(r *http.Request) bool {
	m.historyMu.RLock()
	apiKey := m.apiKey
	m.historyMu.RUnlock()
	return r.Header.Get("Authorization") == "Bearer "+apiKey
}

// DefaultTracearrHistoryRecords returns the default fixture set: five Jellyfin
// rows (the consumable ones) plus one Plex row and one empty-rating_key Jellyfin
// row that the client must skip, so the client's filtering is exercised. The
// lifecycle tests replace this set through SetMovieIDs, which rebuilds the
// fixtures with the real Jellyfin GUIDs and appends the skip rows last.
func DefaultTracearrHistoryRecords() []TracearrHistoryRecord {
	now := time.Now()

	return []TracearrHistoryRecord{
		{
			RatingKey:  "jellyfin-fight-club-id",
			ServerID:   tracearrTestServerID,
			ServerType: "jellyfin",
			MediaType:  "movie",
			StartedAt:  now.Add(-10 * 24 * time.Hour),
			DurationMS: 7200000,
			Watched:    true,
			State:      "watched",
			IMDbID:     "tt0137523",
			TMDbID:     "550",
			MediaID:    "11111111-1111-1111-1111-111111111111",
		},
		{
			RatingKey:  "jellyfin-pulp-fiction-id",
			ServerID:   tracearrTestServerID,
			ServerType: "jellyfin",
			MediaType:  "movie",
			StartedAt:  now.Add(-60 * 24 * time.Hour),
			DurationMS: 9240000,
			Watched:    true,
			State:      "watched",
			IMDbID:     "tt0110912",
			TMDbID:     "680",
			MediaID:    "22222222-2222-2222-2222-222222222222",
		},
		{
			RatingKey:  "jellyfin-inception-id",
			ServerID:   tracearrTestServerID,
			ServerType: "jellyfin",
			MediaType:  "movie",
			StartedAt:  now.Add(-5 * 24 * time.Hour),
			DurationMS: 8880000,
			Watched:    true,
			State:      "watched",
			IMDbID:     "tt1375666",
			TMDbID:     "27205",
			MediaID:    "33333333-3333-3333-3333-333333333333",
		},
		{
			// Wrong media server: the client must filter this row out.
			RatingKey:  "plex-item-1",
			ServerID:   "mock-plex-1",
			ServerType: "plex",
			MediaType:  "movie",
			StartedAt:  now.Add(-15 * 24 * time.Hour),
			DurationMS: 6000000,
			Watched:    true,
			State:      "watched",
			IMDbID:     "tt0000001",
			MediaID:    "44444444-4444-4444-4444-444444444444",
		},
		{
			// Missing rating_key: the client must skip this row.
			RatingKey:  "",
			ServerID:   tracearrTestServerID,
			ServerType: "jellyfin",
			MediaType:  "movie",
			StartedAt:  now.Add(-20 * 24 * time.Hour),
			DurationMS: 5400000,
			Watched:    false,
			State:      "stopped",
			IMDbID:     "tt0000002",
			MediaID:    "55555555-5555-5555-5555-555555555555",
		},
		{
			RatingKey:  "jellyfin-dark-knight-id",
			ServerID:   tracearrTestServerID,
			ServerType: "jellyfin",
			MediaType:  "movie",
			StartedAt:  now.Add(-30 * 24 * time.Hour),
			DurationMS: 9120000,
			Watched:    true,
			State:      "watched",
			IMDbID:     "tt0468569",
			TMDbID:     "155",
			MediaID:    "66666666-6666-6666-6666-666666666666",
		},
		{
			RatingKey:  "jellyfin-interstellar-id",
			ServerID:   tracearrTestServerID,
			ServerType: "jellyfin",
			MediaType:  "movie",
			StartedAt:  now.Add(-45 * 24 * time.Hour),
			DurationMS: 10140000,
			Watched:    true,
			State:      "watched",
			IMDbID:     "tt0816692",
			TMDbID:     "157336",
			MediaID:    "77777777-7777-7777-7777-777777777777",
		},
	}
}

// tracearrScenarioMovie describes one movie in the watch-history scenario that
// mirrors the Jellystat mock: offset is how long ago it was watched, and the
// remaining fields populate the identifying metadata integration tests assert on.
type tracearrScenarioMovie struct {
	title      string
	offset     time.Duration
	durationMS int
	imdbID     string
	tmdbID     string
	mediaID    string
}

// tracearrScenarioMovies is the watch-history scenario dataset shared with the
// Jellystat mock. It covers the 6 watched test movies; Schindler's List is
// intentionally absent (it is never watched, so the client must see no row).
var tracearrScenarioMovies = []tracearrScenarioMovie{
	{"Fight Club", 10 * 24 * time.Hour, 7200000, "tt0137523", "550", "11111111-1111-1111-1111-111111111111"},
	{"Pulp Fiction", 60 * 24 * time.Hour, 9240000, "tt0110912", "680", "22222222-2222-2222-2222-222222222222"},
	{"Inception", 5 * 24 * time.Hour, 8880000, "tt1375666", "27205", "33333333-3333-3333-3333-333333333333"},
	{"The Dark Knight", 30 * 24 * time.Hour, 9120000, "tt0468569", "155", "66666666-6666-6666-6666-666666666666"},
	{"Interstellar", 45 * 24 * time.Hour, 10140000, "tt0816692", "157336", "77777777-7777-7777-7777-777777777777"},
	{"Forrest Gump", 90 * 24 * time.Hour, 8520000, "tt0109830", "13", "88888888-8888-8888-8888-888888888888"},
}

// buildScenarioRecordsLocked rebuilds the fixture set from the configured
// Jellyfin GUID mappings. A scenario movie with no mapped GUID is omitted
// entirely; started_at is now minus the scenario offset unless a per-title
// override was set via SetWatchTimestamp. The two skip rows (a Plex row and a
// Jellyfin row missing its rating_key) are appended so the client's filtering is
// still exercised. Callers must hold historyMu.
func (m *MockTracearrServer) buildScenarioRecordsLocked() []TracearrHistoryRecord {
	now := time.Now()
	records := make([]TracearrHistoryRecord, 0, len(tracearrScenarioMovies)+2)

	for _, movie := range tracearrScenarioMovies {
		ratingKey, ok := m.movieIDs[movie.title]
		if !ok || ratingKey == "" {
			continue
		}

		startedAt := now.Add(-movie.offset)
		if override, ok := m.watchOverrides[movie.title]; ok {
			startedAt = override
		}

		records = append(records, TracearrHistoryRecord{
			RatingKey:  ratingKey,
			ServerID:   tracearrTestServerID,
			ServerType: "jellyfin",
			MediaType:  "movie",
			StartedAt:  startedAt,
			DurationMS: movie.durationMS,
			Watched:    true,
			State:      "watched",
			IMDbID:     movie.imdbID,
			TMDbID:     movie.tmdbID,
			MediaID:    movie.mediaID,
		})
	}

	// Wrong media server: the client must filter this row out.
	records = append(records, TracearrHistoryRecord{
		RatingKey:  "plex-item-1",
		ServerID:   "mock-plex-1",
		ServerType: "plex",
		MediaType:  "movie",
		StartedAt:  now.Add(-15 * 24 * time.Hour),
		DurationMS: 6000000,
		Watched:    true,
		State:      "watched",
		IMDbID:     "tt0000001",
		MediaID:    "44444444-4444-4444-4444-444444444444",
	})

	// Missing rating_key: the client must skip this row.
	records = append(records, TracearrHistoryRecord{
		RatingKey:  "",
		ServerID:   tracearrTestServerID,
		ServerType: "jellyfin",
		MediaType:  "movie",
		StartedAt:  now.Add(-20 * 24 * time.Hour),
		DurationMS: 5400000,
		Watched:    false,
		State:      "stopped",
		IMDbID:     "tt0000002",
		MediaID:    "55555555-5555-5555-5555-555555555555",
	})

	return records
}

// parseTracearrPageSize parses and clamps a requested pageSize. Zero means the
// caller did not ask for a specific size.
func parseTracearrPageSize(raw string) int {
	if raw == "" {
		return 0
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0
	}
	return clampTracearrPageSize(n)
}

// clampTracearrPageSize bounds a page size to 1..tracearrMockMaxPageSize.
func clampTracearrPageSize(size int) int {
	if size < 1 {
		return 1
	}
	if size > tracearrMockMaxPageSize {
		return tracearrMockMaxPageSize
	}
	return size
}

// encodeTracearrCursor turns an offset into an opaque cursor. base64url matches
// the shape of a real Tracearr cursor; the client treats it as opaque.
func encodeTracearrCursor(offset int) string {
	return base64.RawURLEncoding.EncodeToString([]byte(strconv.Itoa(offset)))
}

// decodeTracearrCursor resolves an opaque cursor to its start offset. An empty
// cursor is the first page.
func decodeTracearrCursor(cursor string) (int, error) {
	if cursor == "" {
		return 0, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return 0, fmt.Errorf("mock_tracearr: decoding cursor: %w", err)
	}
	offset, err := strconv.Atoi(string(raw))
	if err != nil {
		return 0, fmt.Errorf("mock_tracearr: parsing cursor offset: %w", err)
	}
	if offset < 0 {
		return 0, fmt.Errorf("mock_tracearr: negative cursor offset %d", offset)
	}
	return offset, nil
}
