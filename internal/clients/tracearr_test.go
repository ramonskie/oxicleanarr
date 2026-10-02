package clients

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/ramonskie/oxicleanarr/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testTracearrServerID is the media-server UUID used by tests that set an
// explicit server_id (which skips auto-detection).
const testTracearrServerID = "11111111-2222-3333-4444-555555555555"

// newTestTracearrClient returns a client pointed at the given test server URL.
func newTestTracearrClient(t *testing.T, serverURL string, serverID string) *TracearrClient {
	t.Helper()
	return NewTracearrClient(config.TracearrConfig{
		BaseIntegrationConfig: config.BaseIntegrationConfig{
			URL:    serverURL,
			APIKey: "trr_pub_test",
		},
		ServerID: serverID,
	})
}

func TestTracearrGetHistory(t *testing.T) {
	t.Run("maps fields correctly on happy path", func(t *testing.T) {
		var gotAuth string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, "/api/v2/public/history", r.URL.Path)
			gotAuth = r.Header.Get("Authorization")

			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"data":[{"rating_key":"abc-123","server_type":"jellyfin","started_at":"2024-01-01T10:00:00Z","duration_ms":3600000}],"meta":{"nextCursor":null}}`))
		}))
		defer server.Close()

		client := newTestTracearrClient(t, server.URL, testTracearrServerID)

		history, err := client.GetHistory(context.Background(), []string{"ignored"})

		require.NoError(t, err)
		require.Len(t, history, 1)

		assert.Equal(t, "abc-123", history[0].JellyfinItemID)
		assert.Equal(t, time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC), history[0].WatchedAt)
		assert.Equal(t, 3600, history[0].PlaybackSeconds)
		assert.Equal(t, "Bearer trr_pub_test", gotAuth)
	})

	t.Run("ignores itemIDs and always requests pageSize 100", func(t *testing.T) {
		var gotPageSize string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPageSize = r.URL.Query().Get("pageSize")
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"data":[],"meta":{"nextCursor":null}}`))
		}))
		defer server.Close()

		client := newTestTracearrClient(t, server.URL, testTracearrServerID)

		history, err := client.GetHistory(context.Background(), []string{"a", "b"})

		require.NoError(t, err)
		assert.Empty(t, history)
		assert.Equal(t, "100", gotPageSize)
	})

	t.Run("follows cursor across two pages", func(t *testing.T) {
		var mu sync.Mutex
		var cursors []string

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cursor := r.URL.Query().Get("cursor")
			mu.Lock()
			cursors = append(cursors, cursor)
			mu.Unlock()

			w.Header().Set("Content-Type", "application/json")
			switch cursor {
			case "":
				w.Write([]byte(`{"data":[{"rating_key":"item-1","server_type":"jellyfin","started_at":"2024-01-01T00:00:00Z","duration_ms":1000}],"meta":{"nextCursor":"page-2"}}`))
			case "page-2":
				w.Write([]byte(`{"data":[{"rating_key":"item-2","server_type":"jellyfin","started_at":"2024-01-02T00:00:00Z","duration_ms":2000}],"meta":{"nextCursor":null}}`))
			default:
				t.Errorf("unexpected cursor %q", cursor)
				w.WriteHeader(http.StatusBadRequest)
			}
		}))
		defer server.Close()

		client := newTestTracearrClient(t, server.URL, testTracearrServerID)

		history, err := client.GetHistory(context.Background(), nil)

		require.NoError(t, err)
		require.Len(t, history, 2)
		assert.Equal(t, "item-1", history[0].JellyfinItemID)
		assert.Equal(t, "item-2", history[1].JellyfinItemID)

		mu.Lock()
		defer mu.Unlock()
		assert.Equal(t, []string{"", "page-2"}, cursors)
	})

	t.Run("tolerates absent meta and null nextCursor", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			// No meta at all — pagination must terminate immediately.
			w.Write([]byte(`{"data":[{"rating_key":"item-1","server_type":"jellyfin","started_at":"2024-01-01T00:00:00Z","duration_ms":1000}]}`))
		}))
		defer server.Close()

		client := newTestTracearrClient(t, server.URL, testTracearrServerID)

		history, err := client.GetHistory(context.Background(), nil)

		require.NoError(t, err)
		require.Len(t, history, 1)
		assert.Equal(t, "item-1", history[0].JellyfinItemID)
	})

	t.Run("skips rows with empty or null rating_key", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"data":[` +
				`{"rating_key":"","server_type":"jellyfin","started_at":"2024-01-01T00:00:00Z","duration_ms":1000},` +
				`{"rating_key":null,"server_type":"jellyfin","started_at":"2024-01-01T00:00:00Z","duration_ms":1000},` +
				`{"server_type":"jellyfin","started_at":"2024-01-01T00:00:00Z","duration_ms":1000},` +
				`{"rating_key":"kept","server_type":"jellyfin","started_at":"2024-01-01T00:00:00Z","duration_ms":1000}` +
				`],"meta":{"nextCursor":null}}`))
		}))
		defer server.Close()

		client := newTestTracearrClient(t, server.URL, testTracearrServerID)

		history, err := client.GetHistory(context.Background(), nil)

		require.NoError(t, err)
		require.Len(t, history, 1)
		assert.Equal(t, "kept", history[0].JellyfinItemID)
	})

	t.Run("skips rows whose server_type is not jellyfin", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"data":[` +
				`{"rating_key":"plex-1","server_type":"plex","started_at":"2024-01-01T00:00:00Z","duration_ms":1000},` +
				`{"rating_key":"emby-1","server_type":"emby","started_at":"2024-01-01T00:00:00Z","duration_ms":1000},` +
				`{"rating_key":"empty-type","server_type":"","started_at":"2024-01-01T00:00:00Z","duration_ms":1000},` +
				`{"rating_key":"jelly-1","server_type":"jellyfin","started_at":"2024-01-01T00:00:00Z","duration_ms":1000}` +
				`],"meta":{"nextCursor":null}}`))
		}))
		defer server.Close()

		client := newTestTracearrClient(t, server.URL, testTracearrServerID)

		history, err := client.GetHistory(context.Background(), nil)

		require.NoError(t, err)
		require.Len(t, history, 1)
		assert.Equal(t, "jelly-1", history[0].JellyfinItemID)
	})

	t.Run("returns error on non-200 status", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "boom", http.StatusInternalServerError)
		}))
		defer server.Close()

		client := newTestTracearrClient(t, server.URL, testTracearrServerID)

		history, err := client.GetHistory(context.Background(), nil)

		require.Error(t, err)
		assert.Nil(t, history)
		assert.Contains(t, err.Error(), "unexpected status code: 500")
	})

	t.Run("adds serverId query param when ServerID is set", func(t *testing.T) {
		var gotServerID string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotServerID = r.URL.Query().Get("serverId")
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"data":[],"meta":{"nextCursor":null}}`))
		}))
		defer server.Close()

		client := newTestTracearrClient(t, server.URL, "11111111-2222-3333-4444-555555555555")

		_, err := client.GetHistory(context.Background(), nil)

		require.NoError(t, err)
		assert.Equal(t, "11111111-2222-3333-4444-555555555555", gotServerID)
	})

	t.Run("explicit server_id does not call health", func(t *testing.T) {
		var mu sync.Mutex
		healthCalled := false
		var gotServerID string

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch r.URL.Path {
			case "/api/v1/public/health":
				mu.Lock()
				healthCalled = true
				mu.Unlock()
				w.WriteHeader(http.StatusOK)
			case "/api/v2/public/history":
				mu.Lock()
				gotServerID = r.URL.Query().Get("serverId")
				mu.Unlock()
				w.Write([]byte(`{"data":[],"meta":{"nextCursor":null}}`))
			default:
				t.Errorf("unexpected path %q", r.URL.Path)
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer server.Close()

		client := newTestTracearrClient(t, server.URL, "explicit-server-id")

		_, err := client.GetHistory(context.Background(), nil)
		require.NoError(t, err)

		mu.Lock()
		defer mu.Unlock()
		assert.False(t, healthCalled, "explicit server_id must skip health discovery")
		assert.Equal(t, "explicit-server-id", gotServerID)
	})

	t.Run("auto-detects a single jellyfin server, caches it, and scopes history", func(t *testing.T) {
		var mu sync.Mutex
		healthCalls := 0
		var gotServerIDs []string

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch r.URL.Path {
			case "/api/v1/public/health":
				mu.Lock()
				healthCalls++
				mu.Unlock()
				// Mixed-case type exercises case-insensitive matching.
				w.Write([]byte(`{"status":"ok","version":"2.0.0","servers":[` +
					`{"id":"discovered-jellyfin","name":"Home Jellyfin","type":"JellyFin","online":true,"historical":true,"activeStreams":0},` +
					`{"id":"plex-1","name":"Plex","type":"plex","online":true,"historical":false,"activeStreams":0}]}`))
			case "/api/v2/public/history":
				mu.Lock()
				gotServerIDs = append(gotServerIDs, r.URL.Query().Get("serverId"))
				mu.Unlock()
				w.Write([]byte(`{"data":[],"meta":{"nextCursor":null}}`))
			default:
				t.Errorf("unexpected path %q", r.URL.Path)
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer server.Close()

		client := newTestTracearrClient(t, server.URL, "")

		_, err := client.GetHistory(context.Background(), nil)
		require.NoError(t, err)
		// Second call must reuse the cached id, not re-query health.
		_, err = client.GetHistory(context.Background(), nil)
		require.NoError(t, err)

		mu.Lock()
		defer mu.Unlock()
		assert.Equal(t, 1, healthCalls, "health must be queried once and cached")
		assert.Equal(t, []string{"discovered-jellyfin", "discovered-jellyfin"}, gotServerIDs)
	})

	t.Run("errors when no jellyfin server is found", func(t *testing.T) {
		historyCalled := false
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch r.URL.Path {
			case "/api/v1/public/health":
				w.Write([]byte(`{"status":"ok","servers":[{"id":"plex-1","name":"Plex","type":"plex","online":true,"historical":true,"activeStreams":0}]}`))
			case "/api/v2/public/history":
				historyCalled = true
				w.Write([]byte(`{"data":[],"meta":{"nextCursor":null}}`))
			default:
				t.Errorf("unexpected path %q", r.URL.Path)
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer server.Close()

		client := newTestTracearrClient(t, server.URL, "")

		history, err := client.GetHistory(context.Background(), nil)

		require.Error(t, err)
		assert.Nil(t, history)
		assert.Contains(t, err.Error(), "no Jellyfin server found")
		assert.Contains(t, err.Error(), "set server_id explicitly")
		assert.False(t, historyCalled, "history must not be fetched without a resolved server")
	})

	t.Run("errors when multiple jellyfin servers are found", func(t *testing.T) {
		historyCalled := false
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch r.URL.Path {
			case "/api/v1/public/health":
				w.Write([]byte(`{"status":"ok","servers":[` +
					`{"id":"jelly-1","name":"Alpha","type":"jellyfin","online":true,"historical":true,"activeStreams":0},` +
					`{"id":"jelly-2","name":"Beta","type":"jellyfin","online":true,"historical":true,"activeStreams":0}]}`))
			case "/api/v2/public/history":
				historyCalled = true
				w.Write([]byte(`{"data":[],"meta":{"nextCursor":null}}`))
			default:
				t.Errorf("unexpected path %q", r.URL.Path)
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer server.Close()

		client := newTestTracearrClient(t, server.URL, "")

		history, err := client.GetHistory(context.Background(), nil)

		require.Error(t, err)
		assert.Nil(t, history)
		assert.Contains(t, err.Error(), "multiple Jellyfin servers found")
		assert.Contains(t, err.Error(), "Alpha")
		assert.Contains(t, err.Error(), "jelly-1")
		assert.Contains(t, err.Error(), "Beta")
		assert.Contains(t, err.Error(), "jelly-2")
		assert.Contains(t, err.Error(), "set server_id explicitly")
		assert.False(t, historyCalled, "history must not be fetched without a resolved server")
	})

	t.Run("paginates history using the discovered server id", func(t *testing.T) {
		var mu sync.Mutex
		var gotServerIDs []string
		var cursors []string

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch r.URL.Path {
			case "/api/v1/public/health":
				w.Write([]byte(`{"status":"ok","servers":[{"id":"auto-jelly","name":"Auto","type":"jellyfin","online":true,"historical":true,"activeStreams":0}]}`))
			case "/api/v2/public/history":
				cursor := r.URL.Query().Get("cursor")
				mu.Lock()
				gotServerIDs = append(gotServerIDs, r.URL.Query().Get("serverId"))
				cursors = append(cursors, cursor)
				mu.Unlock()
				switch cursor {
				case "":
					w.Write([]byte(`{"data":[{"rating_key":"item-1","server_type":"jellyfin","started_at":"2024-01-01T00:00:00Z","duration_ms":1000}],"meta":{"nextCursor":"page-2"}}`))
				case "page-2":
					w.Write([]byte(`{"data":[{"rating_key":"item-2","server_type":"jellyfin","started_at":"2024-01-02T00:00:00Z","duration_ms":2000}],"meta":{"nextCursor":null}}`))
				default:
					t.Errorf("unexpected cursor %q", cursor)
					w.WriteHeader(http.StatusBadRequest)
				}
			default:
				t.Errorf("unexpected path %q", r.URL.Path)
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer server.Close()

		client := newTestTracearrClient(t, server.URL, "")

		history, err := client.GetHistory(context.Background(), nil)

		require.NoError(t, err)
		require.Len(t, history, 2)
		assert.Equal(t, "item-1", history[0].JellyfinItemID)
		assert.Equal(t, "item-2", history[1].JellyfinItemID)

		mu.Lock()
		defer mu.Unlock()
		assert.Equal(t, []string{"", "page-2"}, cursors)
		assert.Equal(t, []string{"auto-jelly", "auto-jelly"}, gotServerIDs)
	})

	t.Run("concurrent first calls discover the server exactly once", func(t *testing.T) {
		var mu sync.Mutex
		healthCalls := 0
		var gotServerIDs []string

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch r.URL.Path {
			case "/api/v1/public/health":
				mu.Lock()
				healthCalls++
				mu.Unlock()
				// Widen the race window so both goroutines would reach
				// discovery without the mutex.
				time.Sleep(20 * time.Millisecond)
				w.Write([]byte(`{"status":"ok","servers":[{"id":"race-jelly","name":"Race","type":"jellyfin","online":true,"historical":true,"activeStreams":0}]}`))
			case "/api/v2/public/history":
				mu.Lock()
				gotServerIDs = append(gotServerIDs, r.URL.Query().Get("serverId"))
				mu.Unlock()
				w.Write([]byte(`{"data":[{"rating_key":"item-1","server_id":"race-jelly","server_type":"jellyfin","started_at":"2024-01-01T00:00:00Z","duration_ms":1000}],"meta":{"nextCursor":null}}`))
			default:
				t.Errorf("unexpected path %q", r.URL.Path)
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer server.Close()

		client := newTestTracearrClient(t, server.URL, "")

		const goroutines = 2
		results := make([][]StatsHistoryItem, goroutines)
		errs := make([]error, goroutines)

		var wg sync.WaitGroup
		start := make(chan struct{})
		for i := 0; i < goroutines; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				<-start
				results[idx], errs[idx] = client.GetHistory(context.Background(), nil)
			}(i)
		}
		close(start)
		wg.Wait()

		for i := 0; i < goroutines; i++ {
			require.NoError(t, errs[i])
			require.Len(t, results[i], 1)
			assert.Equal(t, "item-1", results[i][0].JellyfinItemID)
		}

		mu.Lock()
		defer mu.Unlock()
		assert.Equal(t, 1, healthCalls, "concurrent first calls must trigger a single discovery")
		require.Len(t, gotServerIDs, goroutines)
		for _, id := range gotServerIDs {
			assert.Equal(t, "race-jelly", id)
		}
	})

	t.Run("errors when health returns no servers", func(t *testing.T) {
		historyCalled := false
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch r.URL.Path {
			case "/api/v1/public/health":
				w.Write([]byte(`{"status":"ok","servers":[]}`))
			case "/api/v2/public/history":
				historyCalled = true
				w.Write([]byte(`{"data":[],"meta":{"nextCursor":null}}`))
			default:
				t.Errorf("unexpected path %q", r.URL.Path)
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer server.Close()

		client := newTestTracearrClient(t, server.URL, "")

		history, err := client.GetHistory(context.Background(), nil)

		require.Error(t, err)
		assert.Nil(t, history)
		assert.Contains(t, err.Error(), "no Jellyfin server found")
		assert.False(t, historyCalled, "history must not be fetched without a resolved server")
	})

	t.Run("skips rows whose server_id differs from the resolved id", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"data":[` +
				`{"rating_key":"other-server","server_id":"some-other-server","server_type":"jellyfin","started_at":"2024-01-01T00:00:00Z","duration_ms":1000},` +
				`{"rating_key":"blank-id","server_id":"","server_type":"jellyfin","started_at":"2024-01-01T00:00:00Z","duration_ms":1000},` +
				`{"rating_key":"kept","server_id":"` + testTracearrServerID + `","server_type":"jellyfin","started_at":"2024-01-01T00:00:00Z","duration_ms":1000}` +
				`],"meta":{"nextCursor":null}}`))
		}))
		defer server.Close()

		client := newTestTracearrClient(t, server.URL, testTracearrServerID)

		history, err := client.GetHistory(context.Background(), nil)

		require.NoError(t, err)
		require.Len(t, history, 2)
		assert.Equal(t, "blank-id", history[0].JellyfinItemID)
		assert.Equal(t, "kept", history[1].JellyfinItemID)
	})

	t.Run("accepts mixed-case and whitespace-padded server_type", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"data":[` +
				`{"rating_key":"mixed-case","server_type":"Jellyfin","started_at":"2024-01-01T00:00:00Z","duration_ms":1000},` +
				`{"rating_key":"padded","server_type":"  jElLyFiN  ","started_at":"2024-01-01T00:00:00Z","duration_ms":1000}` +
				`],"meta":{"nextCursor":null}}`))
		}))
		defer server.Close()

		client := newTestTracearrClient(t, server.URL, testTracearrServerID)

		history, err := client.GetHistory(context.Background(), nil)

		require.NoError(t, err)
		require.Len(t, history, 2)
		assert.Equal(t, "mixed-case", history[0].JellyfinItemID)
		assert.Equal(t, "padded", history[1].JellyfinItemID)
	})

	t.Run("skips rows whose started_at is null", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"data":[` +
				`{"rating_key":"null-date","server_type":"jellyfin","started_at":null,"duration_ms":1000},` +
				`{"rating_key":"absent-date","server_type":"jellyfin","duration_ms":1000},` +
				`{"rating_key":"kept","server_type":"jellyfin","started_at":"2024-01-01T00:00:00Z","duration_ms":1000}` +
				`],"meta":{"nextCursor":null}}`))
		}))
		defer server.Close()

		client := newTestTracearrClient(t, server.URL, testTracearrServerID)

		history, err := client.GetHistory(context.Background(), nil)

		require.NoError(t, err)
		require.Len(t, history, 1)
		assert.Equal(t, "kept", history[0].JellyfinItemID)
	})

	t.Run("stops when the cursor does not advance", func(t *testing.T) {
		var mu sync.Mutex
		calls := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			calls++
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			// Always echo the same cursor — must not loop forever.
			w.Write([]byte(`{"data":[{"rating_key":"item-1","server_type":"jellyfin","started_at":"2024-01-01T00:00:00Z","duration_ms":1000}],"meta":{"nextCursor":"stuck"}}`))
		}))
		defer server.Close()

		client := newTestTracearrClient(t, server.URL, testTracearrServerID)

		history, err := client.GetHistory(context.Background(), nil)

		require.NoError(t, err)
		assert.Len(t, history, 2)

		mu.Lock()
		defer mu.Unlock()
		assert.Equal(t, 2, calls)
	})
}

func TestTracearrPing(t *testing.T) {
	t.Run("returns nil on 200", func(t *testing.T) {
		var gotAuth string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, "/api/v1/public/health", r.URL.Path)
			gotAuth = r.Header.Get("Authorization")
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client := newTestTracearrClient(t, server.URL, testTracearrServerID)

		err := client.Ping(context.Background())

		require.NoError(t, err)
		assert.Equal(t, "Bearer trr_pub_test", gotAuth)
	})

	t.Run("returns error on 401", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		}))
		defer server.Close()

		client := newTestTracearrClient(t, server.URL, testTracearrServerID)

		err := client.Ping(context.Background())

		require.Error(t, err)
		assert.Contains(t, err.Error(), "unexpected status code: 401")
	})
}
