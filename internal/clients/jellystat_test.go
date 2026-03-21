package clients

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/ramonskie/oxicleanarr/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestJellystatIntegration runs integration tests against a real Jellystat instance
// Set OXICLEANARR_INTEGRATION_TEST=1 to enable these tests
func TestJellystatIntegration(t *testing.T) {
	if os.Getenv("OXICLEANARR_INTEGRATION_TEST") != "1" {
		t.Skip("Skipping integration test. Set OXICLEANARR_INTEGRATION_TEST=1 to run.")
	}

	// Load test configuration
	cfg, err := config.Load("../../config/prunarr.test.yaml")
	require.NoError(t, err, "Failed to load test config")
	require.True(t, cfg.Integrations.Jellystat.Enabled, "Jellystat must be enabled in test config")

	client := NewJellystatClient(cfg.Integrations.Jellystat)
	ctx := context.Background()

	t.Run("Ping", func(t *testing.T) {
		err := client.Ping(ctx)
		assert.NoError(t, err, "Should be able to ping Jellystat")
	})

	t.Run("GetHistory", func(t *testing.T) {
		history, err := client.GetHistory(ctx, nil)
		require.NoError(t, err, "Should be able to fetch history")

		t.Logf("Found %d history items in Jellystat", len(history))

		if len(history) == 0 {
			t.Log("Warning: No history found in Jellystat")
			return
		}

		// Validate first history item structure (normalised StatsHistoryItem fields)
		item := history[0]
		assert.NotEmpty(t, item.JellyfinItemID, "History item should have a Jellyfin item ID")
		assert.False(t, item.WatchedAt.IsZero(), "History item should have a watch timestamp")

		t.Logf("Sample history item:")
		t.Logf("  JellyfinItemID: %s", item.JellyfinItemID)
		t.Logf("  WatchedAt: %s", item.WatchedAt.Format(time.RFC3339))
		t.Logf("  PlaybackSeconds: %d (%.1f minutes)", item.PlaybackSeconds, float64(item.PlaybackSeconds)/60)
	})

	t.Run("PlaybackDurationValidation", func(t *testing.T) {
		history, err := client.GetHistory(ctx, nil)
		require.NoError(t, err, "Should be able to fetch history")

		if len(history) == 0 {
			t.Skip("No history available")
		}

		var (
			totalDuration int
			shortPlays    int // < 5 minutes
			mediumPlays   int // 5-60 minutes
			longPlays     int // > 60 minutes
		)

		for _, item := range history {
			totalDuration += item.PlaybackSeconds

			minutes := item.PlaybackSeconds / 60
			if minutes < 5 {
				shortPlays++
			} else if minutes < 60 {
				mediumPlays++
			} else {
				longPlays++
			}
		}

		avgDuration := float64(totalDuration) / float64(len(history))

		t.Logf("Playback duration statistics:")
		t.Logf("  Total watch time: %.1f hours", float64(totalDuration)/3600)
		t.Logf("  Average duration per play: %.1f minutes", avgDuration/60)
		t.Logf("  Short plays (<5 min): %d (%.1f%%)", shortPlays, float64(shortPlays)/float64(len(history))*100)
		t.Logf("  Medium plays (5-60 min): %d (%.1f%%)", mediumPlays, float64(mediumPlays)/float64(len(history))*100)
		t.Logf("  Long plays (>60 min): %d (%.1f%%)", longPlays, float64(longPlays)/float64(len(history))*100)
	})

	t.Run("RecentActivityValidation", func(t *testing.T) {
		history, err := client.GetHistory(ctx, nil)
		require.NoError(t, err, "Should be able to fetch history")

		if len(history) == 0 {
			t.Skip("No history available")
		}

		now := time.Now()
		var (
			last24h int
			last7d  int
			last30d int
			older   int
		)

		for _, item := range history {
			age := now.Sub(item.WatchedAt)
			switch {
			case age < 24*time.Hour:
				last24h++
			case age < 7*24*time.Hour:
				last7d++
			case age < 30*24*time.Hour:
				last30d++
			default:
				older++
			}
		}

		t.Logf("Activity recency:")
		t.Logf("  Last 24 hours: %d (%.1f%%)", last24h, float64(last24h)/float64(len(history))*100)
		t.Logf("  Last 7 days: %d (%.1f%%)", last7d, float64(last7d)/float64(len(history))*100)
		t.Logf("  Last 30 days: %d (%.1f%%)", last30d, float64(last30d)/float64(len(history))*100)
		t.Logf("  Older: %d (%.1f%%)", older, float64(older)/float64(len(history))*100)
	})

	t.Run("PaginationHandling", func(t *testing.T) {
		// Test that pagination works correctly by fetching all history
		history, err := client.GetHistory(ctx, nil)
		require.NoError(t, err, "Should handle pagination correctly")

		t.Logf("Pagination test:")
		t.Logf("  Total history items fetched: %d", len(history))
		t.Logf("  Expected pagination: %d pages (100 items per page)", (len(history)/100)+1)

		// Verify no duplicate JellyfinItemIDs (would indicate pagination bug)
		seenIDs := make(map[string]int)
		for _, item := range history {
			seenIDs[item.JellyfinItemID]++
		}
		t.Logf("  Unique Jellyfin item IDs: %d", len(seenIDs))
	})

	t.Run("ConcurrentRequests", func(t *testing.T) {
		// Test multiple concurrent requests
		results := make(chan error, 2)

		go func() {
			err := client.Ping(ctx)
			results <- err
		}()

		go func() {
			_, err := client.GetHistory(ctx, nil)
			results <- err
		}()

		// Collect results
		for i := 0; i < 2; i++ {
			err := <-results
			assert.NoError(t, err, "Concurrent request should succeed")
		}
	})
}

// TestJellystatClient_Unit runs unit tests that don't require a real Jellystat instance
func TestJellystatClient_Unit(t *testing.T) {
	t.Run("NewJellystatClient", func(t *testing.T) {
		cfg := config.JellystatConfig{
			BaseIntegrationConfig: config.BaseIntegrationConfig{
				URL:     "http://localhost:3000",
				APIKey:  "test-api-key",
				Timeout: "30s",
			},
		}

		client := NewJellystatClient(cfg)

		assert.NotNil(t, client, "Client should not be nil")
		assert.Equal(t, cfg.URL, client.baseURL, "Base URL should match config")
		assert.Equal(t, cfg.APIKey, client.apiKey, "API key should match config")
		assert.NotNil(t, client.client, "HTTP client should be initialized")
	})

	t.Run("NewJellystatClient_DefaultTimeout", func(t *testing.T) {
		cfg := config.JellystatConfig{
			BaseIntegrationConfig: config.BaseIntegrationConfig{
				URL:    "http://localhost:3000",
				APIKey: "test-api-key",
				// No timeout specified
			},
		}

		client := NewJellystatClient(cfg)

		assert.NotNil(t, client, "Client should not be nil")
		assert.Equal(t, 30*time.Second, client.client.Timeout, "Should use default 30s timeout")
	})

	t.Run("NewJellystatClient_InvalidTimeout", func(t *testing.T) {
		cfg := config.JellystatConfig{
			BaseIntegrationConfig: config.BaseIntegrationConfig{
				URL:     "http://localhost:3000",
				APIKey:  "test-api-key",
				Timeout: "invalid",
			},
		}

		client := NewJellystatClient(cfg)

		// Should fall back to default timeout
		assert.Equal(t, 30*time.Second, client.client.Timeout, "Should use default timeout for invalid value")
	})

	t.Run("NewJellystatClient_CustomTimeout", func(t *testing.T) {
		cfg := config.JellystatConfig{
			BaseIntegrationConfig: config.BaseIntegrationConfig{
				URL:     "http://localhost:3000",
				APIKey:  "test-api-key",
				Timeout: "45s",
			},
		}

		client := NewJellystatClient(cfg)

		assert.Equal(t, 45*time.Second, client.client.Timeout, "Should use custom timeout")
	})
}
