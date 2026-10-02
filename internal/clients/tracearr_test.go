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

// testTracearrServerID is the media-server UUID used by tests that do not
// exercise server scoping directly. server_id is required by the client.
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

	t.Run("returns error when server_id is empty", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Error("GetHistory must not issue a request when server_id is empty")
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client := newTestTracearrClient(t, server.URL, "")

		history, err := client.GetHistory(context.Background(), nil)

		require.Error(t, err)
		assert.Nil(t, history)
		assert.Contains(t, err.Error(), "server_id is required")
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
			require.Equal(t, "/api/v2/public/health", r.URL.Path)
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
