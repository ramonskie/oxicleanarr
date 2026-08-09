package clients

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ramonskie/oxicleanarr/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStreamystatsGetHistory(t *testing.T) {
	t.Run("returns history for tracked items", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"watchHistory":[{"userId":"u1","lastActivityDate":"2024-01-01T00:00:00Z","playDuration":3600,"completed":true}]}`))
		}))
		defer server.Close()

		client := NewStreamystatsClient(config.StreamystatsConfig{
			BaseIntegrationConfig: config.BaseIntegrationConfig{
				URL:    server.URL,
				APIKey: "key",
			},
			ServerID: "srv",
		})

		history, err := client.GetHistory(context.Background(), []string{"item-1", "item-2"})

		require.NoError(t, err)
		require.Len(t, history, 2)

		// Order is non-deterministic (concurrent fetches); check membership.
		ids := map[string]bool{}
		for _, h := range history {
			ids[h.JellyfinItemID] = true
			assert.Equal(t, 3600, h.PlaybackSeconds)
		}
		assert.True(t, ids["item-1"])
		assert.True(t, ids["item-2"])
	})

	t.Run("returns empty when no items requested", func(t *testing.T) {
		client := NewStreamystatsClient(config.StreamystatsConfig{})
		history, err := client.GetHistory(context.Background(), nil)
		require.NoError(t, err)
		assert.Nil(t, history)
	})

	t.Run("returns error when all item fetches fail", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "boom", http.StatusInternalServerError)
		}))
		defer server.Close()

		client := NewStreamystatsClient(config.StreamystatsConfig{
			BaseIntegrationConfig: config.BaseIntegrationConfig{
				URL:    server.URL,
				APIKey: "key",
			},
			ServerID: "srv",
		})

		history, err := client.GetHistory(context.Background(), []string{"item-1", "item-2"})

		require.Error(t, err)
		assert.Nil(t, history)
	})

	t.Run("returns partial history when some fetches fail", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/get-item-details/broken" {
				http.Error(w, "boom", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"watchHistory":[{"userId":"u1","lastActivityDate":"2024-01-01T00:00:00Z","playDuration":3600,"completed":true}]}`))
		}))
		defer server.Close()

		client := NewStreamystatsClient(config.StreamystatsConfig{
			BaseIntegrationConfig: config.BaseIntegrationConfig{
				URL:    server.URL,
				APIKey: "key",
			},
			ServerID: "srv",
		})

		// One item fails; the other succeeds. Partial results are returned.
		history, err := client.GetHistory(context.Background(), []string{"broken", "item-2"})

		require.NoError(t, err)
		require.Len(t, history, 1)
		assert.Equal(t, "item-2", history[0].JellyfinItemID)
	})

	t.Run("treats 404 as no history, not error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.NotFound(w, r)
		}))
		defer server.Close()

		client := NewStreamystatsClient(config.StreamystatsConfig{
			BaseIntegrationConfig: config.BaseIntegrationConfig{
				URL:    server.URL,
				APIKey: "key",
			},
			ServerID: "srv",
		})

		history, err := client.GetHistory(context.Background(), []string{"item-1"})

		require.NoError(t, err)
		assert.Empty(t, history)
	})

	t.Run("filters zero-valued watch timestamps", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"watchHistory":[{"userId":"u1","lastActivityDate":"0001-01-01T00:00:00Z","playDuration":3600,"completed":true}]}`))
		}))
		defer server.Close()

		client := NewStreamystatsClient(config.StreamystatsConfig{
			BaseIntegrationConfig: config.BaseIntegrationConfig{
				URL:    server.URL,
				APIKey: "key",
			},
			ServerID: "srv",
		})

		history, err := client.GetHistory(context.Background(), []string{"item-1"})

		require.NoError(t, err)
		require.Len(t, history, 1)
		assert.True(t, history[0].WatchedAt.Equal(time.Time{}))
	})
}
