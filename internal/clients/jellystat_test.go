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
		history, err := client.GetHistory(ctx)
		require.NoError(t, err, "Should be able to fetch history")

		t.Logf("Found %d history items in Jellystat", len(history))

		if len(history) == 0 {
			t.Log("Warning: No history found in Jellystat")
			return
		}

		// Validate first history item structure
		item := history[0]
		assert.NotEmpty(t, item.ID, "History item should have an ID")
		assert.NotEmpty(t, item.UserID, "History item should have a user ID")
		assert.NotEmpty(t, item.UserName, "History item should have a username")
		assert.NotEmpty(t, item.NowPlayingItemID, "History item should have an item ID")
		assert.NotEmpty(t, item.NowPlayingItemName, "History item should have an item name")
		assert.False(t, item.ActivityDateInserted.IsZero(), "History item should have a date")

		t.Logf("Sample history item:")
		t.Logf("  ID: %s", item.ID)
		t.Logf("  User: %s (ID: %s)", item.UserName, item.UserID)
		t.Logf("  Item: %s (ID: %s)", item.NowPlayingItemName, item.NowPlayingItemID)
		t.Logf("  Date: %s", item.ActivityDateInserted.Format(time.RFC3339))
		t.Logf("  Duration: %d seconds (%.1f minutes)", item.PlaybackDuration, float64(item.PlaybackDuration)/60)

		// Check if it's a TV show
		if item.SeriesName != "" {
			t.Logf("  Series: %s", item.SeriesName)
			t.Logf("  Season ID: %s", item.SeasonID)
			t.Logf("  Episode ID: %s", item.EpisodeID)
		}
	})

	t.Run("MediaTypeValidation", func(t *testing.T) {
		history, err := client.GetHistory(ctx)
		require.NoError(t, err, "Should be able to fetch history")

		if len(history) == 0 {
			t.Skip("No history available")
		}

		// Count movies vs TV shows
		var (
			movieItems int
			tvItems    int
		)

		for _, item := range history {
			if item.SeriesName != "" {
				tvItems++
			} else {
				movieItems++
			}
		}

		t.Logf("Media type breakdown:")
		t.Logf("  Total history items: %d", len(history))
		t.Logf("  Movies: %d (%.1f%%)", movieItems, float64(movieItems)/float64(len(history))*100)
		t.Logf("  TV episodes: %d (%.1f%%)", tvItems, float64(tvItems)/float64(len(history))*100)
	})

	t.Run("UserActivityValidation", func(t *testing.T) {
		history, err := client.GetHistory(ctx)
		require.NoError(t, err, "Should be able to fetch history")

		if len(history) == 0 {
			t.Skip("No history available")
		}

		// Count unique users and their activity
		userActivity := make(map[string]struct {
			username string
			count    int
			duration int
		})

		for _, item := range history {
			if entry, exists := userActivity[item.UserID]; exists {
				entry.count++
				entry.duration += item.PlaybackDuration
				userActivity[item.UserID] = entry
			} else {
				userActivity[item.UserID] = struct {
					username string
					count    int
					duration int
				}{
					username: item.UserName,
					count:    1,
					duration: item.PlaybackDuration,
				}
			}
		}

		t.Logf("User activity statistics:")
		t.Logf("  Total unique users: %d", len(userActivity))
		t.Logf("  Average plays per user: %.2f", float64(len(history))/float64(len(userActivity)))

		// Show top 5 most active users
		type userStat struct {
			userID   string
			username string
			count    int
			duration int
		}
		var topUsers []userStat
		for userID, activity := range userActivity {
			topUsers = append(topUsers, userStat{
				userID:   userID,
				username: activity.username,
				count:    activity.count,
				duration: activity.duration,
			})
		}

		// Sort by count (simple bubble sort for small lists)
		for i := 0; i < len(topUsers)-1; i++ {
			for j := 0; j < len(topUsers)-i-1; j++ {
				if topUsers[j].count < topUsers[j+1].count {
					topUsers[j], topUsers[j+1] = topUsers[j+1], topUsers[j]
				}
			}
		}

		limit := 5
		if len(topUsers) < limit {
			limit = len(topUsers)
		}

		t.Logf("  Top %d most active users:", limit)
		for i := 0; i < limit; i++ {
			user := topUsers[i]
			t.Logf("    %d. %s: %d plays, %.1f hours total",
				i+1,
				user.username,
				user.count,
				float64(user.duration)/3600)
		}
	})

	t.Run("PlaybackDurationValidation", func(t *testing.T) {
		history, err := client.GetHistory(ctx)
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
			totalDuration += item.PlaybackDuration

			minutes := item.PlaybackDuration / 60
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
		history, err := client.GetHistory(ctx)
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
			age := now.Sub(item.ActivityDateInserted)
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
		history, err := client.GetHistory(ctx)
		require.NoError(t, err, "Should handle pagination correctly")

		t.Logf("Pagination test:")
		t.Logf("  Total history items fetched: %d", len(history))
		t.Logf("  Expected pagination: %d pages (100 items per page)", (len(history)/100)+1)

		// Verify no duplicate IDs (would indicate pagination bug)
		seenIDs := make(map[string]bool)
		for _, item := range history {
			assert.False(t, seenIDs[item.ID], "Should not have duplicate history IDs (pagination bug)")
			seenIDs[item.ID] = true
		}
	})

	t.Run("ConcurrentRequests", func(t *testing.T) {
		// Test multiple concurrent requests
		results := make(chan error, 2)

		go func() {
			err := client.Ping(ctx)
			results <- err
		}()

		go func() {
			_, err := client.GetHistory(ctx)
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
