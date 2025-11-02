package clients

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/ramonskie/prunarr/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSonarrIntegration runs integration tests against a real Sonarr instance
// Set PRUNARR_INTEGRATION_TEST=1 to enable these tests
func TestSonarrIntegration(t *testing.T) {
	if os.Getenv("PRUNARR_INTEGRATION_TEST") != "1" {
		t.Skip("Skipping integration test. Set PRUNARR_INTEGRATION_TEST=1 to run.")
	}

	// Load test configuration
	cfg, err := config.Load("../../config/prunarr.test.yaml")
	require.NoError(t, err, "Failed to load test config")
	require.True(t, cfg.Integrations.Sonarr.Enabled, "Sonarr must be enabled in test config")

	client := NewSonarrClient(cfg.Integrations.Sonarr)
	ctx := context.Background()

	t.Run("Ping", func(t *testing.T) {
		err := client.Ping(ctx)
		assert.NoError(t, err, "Should be able to ping Sonarr")
	})

	t.Run("GetSeries", func(t *testing.T) {
		series, err := client.GetSeries(ctx)
		require.NoError(t, err, "Should be able to fetch series")

		if len(series) == 0 {
			t.Log("Warning: No TV series found in Sonarr")
			return
		}

		t.Logf("Found %d TV series in Sonarr", len(series))

		// Validate first series structure
		show := series[0]
		assert.NotZero(t, show.ID, "Series should have an ID")
		assert.NotEmpty(t, show.Title, "Series should have a title")
		assert.False(t, show.Added.IsZero(), "Series should have an added date")
		assert.NotEmpty(t, show.Path, "Series should have a path")

		t.Logf("Sample series: %s (%d) - ID: %d", show.Title, show.Year, show.ID)

		// Check statistics
		stats := show.Statistics
		t.Logf("  Episodes: %d/%d (available/total)",
			stats.EpisodeFileCount,
			stats.TotalEpisodeCount)

		if stats.SizeOnDisk > 0 {
			t.Logf("  Size on disk: %d bytes (%.2f GB)",
				stats.SizeOnDisk,
				float64(stats.SizeOnDisk)/(1024*1024*1024))
		}
	})

	t.Run("GetSeriesByID", func(t *testing.T) {
		// First get all series to get a valid ID
		allSeries, err := client.GetSeries(ctx)
		require.NoError(t, err, "Should be able to fetch series")

		if len(allSeries) == 0 {
			t.Skip("No series available to test GetSeriesByID")
		}

		seriesID := allSeries[0].ID

		series, err := client.GetSeriesByID(ctx, seriesID)
		require.NoError(t, err, "Should be able to fetch single series")
		require.NotNil(t, series, "Series should not be nil")

		assert.Equal(t, seriesID, series.ID, "Series ID should match")
		assert.NotEmpty(t, series.Title, "Series should have a title")

		t.Logf("Fetched series: %s (%d)", series.Title, series.Year)
	})

	t.Run("GetEpisodes", func(t *testing.T) {
		// First get all series to get a valid ID
		allSeries, err := client.GetSeries(ctx)
		require.NoError(t, err, "Should be able to fetch series")

		if len(allSeries) == 0 {
			t.Skip("No series available to test GetEpisodes")
		}

		// Find a series with episodes
		var seriesID int
		var seriesTitle string
		for _, s := range allSeries {
			if s.Statistics.EpisodeFileCount > 0 {
				seriesID = s.ID
				seriesTitle = s.Title
				break
			}
		}

		if seriesID == 0 {
			t.Skip("No series with episodes found")
		}

		episodes, err := client.GetEpisodes(ctx, seriesID)
		require.NoError(t, err, "Should be able to fetch episodes")

		t.Logf("Found %d episodes for series '%s' (ID: %d)",
			len(episodes), seriesTitle, seriesID)

		if len(episodes) > 0 {
			// Validate episode structure
			ep := episodes[0]
			assert.Equal(t, seriesID, ep.SeriesID, "Episode should belong to the series")
			assert.NotEmpty(t, ep.Title, "Episode should have a title")
			assert.NotZero(t, ep.SeasonNumber, "Episode should have season number")
			assert.NotZero(t, ep.EpisodeNumber, "Episode should have episode number")

			t.Logf("  Sample episode: S%02dE%02d - %s",
				ep.SeasonNumber, ep.EpisodeNumber, ep.Title)

			if ep.HasFile && ep.EpisodeFile != nil {
				t.Logf("    File size: %d bytes (%.2f MB)",
					ep.EpisodeFile.Size,
					float64(ep.EpisodeFile.Size)/(1024*1024))
				t.Logf("    Added: %s", ep.EpisodeFile.DateAdded.Format(time.RFC3339))
			}
		}
	})

	t.Run("GetSeriesByID_InvalidID", func(t *testing.T) {
		// Use an ID that's unlikely to exist
		invalidID := 999999999

		series, err := client.GetSeriesByID(ctx, invalidID)
		assert.Error(t, err, "Should return error for invalid series ID")
		assert.Nil(t, series, "Series should be nil for invalid ID")
	})

	t.Run("DeleteSeries_ReadOnly", func(t *testing.T) {
		// This test should NOT actually delete anything
		// We're just validating the method signature and ensuring dry_run is enforced
		t.Skip("Skipping delete test - read-only mode enforced")

		// If we were to test this (in a controlled environment):
		// err := client.DeleteSeries(ctx, someID, false)
		// This should never be run in integration tests with real data
	})

	t.Run("SeriesDataValidation", func(t *testing.T) {
		allSeries, err := client.GetSeries(ctx)
		require.NoError(t, err, "Should be able to fetch series")

		if len(allSeries) == 0 {
			t.Skip("No series available for validation")
		}

		// Count series by various attributes
		var (
			totalEpisodes     int
			totalEpisodeFiles int
			totalSize         int64
		)

		for _, series := range allSeries {
			totalEpisodes += series.Statistics.TotalEpisodeCount
			totalEpisodeFiles += series.Statistics.EpisodeFileCount
			totalSize += series.Statistics.SizeOnDisk
		}

		t.Logf("Series statistics:")
		t.Logf("  Total series: %d", len(allSeries))
		t.Logf("  Total episodes (all series): %d", totalEpisodes)
		t.Logf("  Total episode files: %d", totalEpisodeFiles)
		t.Logf("  Total size on disk: %.2f GB", float64(totalSize)/(1024*1024*1024))

		if totalEpisodeFiles > 0 {
			avgSize := float64(totalSize) / float64(totalEpisodeFiles)
			t.Logf("  Average episode size: %.2f MB", avgSize/(1024*1024))

			coverage := float64(totalEpisodeFiles) / float64(totalEpisodes) * 100
			t.Logf("  Episode coverage: %.1f%%", coverage)
		}
	})

	t.Run("EpisodesBySeasonValidation", func(t *testing.T) {
		allSeries, err := client.GetSeries(ctx)
		require.NoError(t, err, "Should be able to fetch series")

		if len(allSeries) == 0 {
			t.Skip("No series available")
		}

		// Find a series with episodes
		var seriesID int
		var seriesTitle string
		for _, s := range allSeries {
			if s.Statistics.EpisodeFileCount > 0 {
				seriesID = s.ID
				seriesTitle = s.Title
				break
			}
		}

		if seriesID == 0 {
			t.Skip("No series with episodes found")
		}

		episodes, err := client.GetEpisodes(ctx, seriesID)
		require.NoError(t, err, "Should be able to fetch episodes")

		// Group episodes by season
		seasonMap := make(map[int]int)
		for _, ep := range episodes {
			if ep.HasFile {
				seasonMap[ep.SeasonNumber]++
			}
		}

		t.Logf("Episodes per season for '%s':", seriesTitle)
		for season, count := range seasonMap {
			t.Logf("  Season %d: %d episodes", season, count)
		}
	})

	t.Run("ConcurrentRequests", func(t *testing.T) {
		// Test multiple concurrent requests
		allSeries, err := client.GetSeries(ctx)
		require.NoError(t, err, "Should be able to fetch series")

		if len(allSeries) < 3 {
			t.Skip("Need at least 3 series for concurrent test")
		}

		// Make 3 concurrent requests
		results := make(chan error, 3)
		for i := 0; i < 3; i++ {
			go func(seriesID int) {
				_, err := client.GetSeriesByID(ctx, seriesID)
				results <- err
			}(allSeries[i].ID)
		}

		// Collect results
		for i := 0; i < 3; i++ {
			err := <-results
			assert.NoError(t, err, "Concurrent request should succeed")
		}
	})
}

// TestSonarrClient_Unit runs unit tests that don't require a real Sonarr instance
func TestSonarrClient_Unit(t *testing.T) {
	t.Run("NewSonarrClient", func(t *testing.T) {
		cfg := config.SonarrConfig{
			BaseIntegrationConfig: config.BaseIntegrationConfig{
				URL:     "http://localhost:8989",
				APIKey:  "test-api-key",
				Timeout: "30s",
			},
		}

		client := NewSonarrClient(cfg)

		assert.NotNil(t, client, "Client should not be nil")
		assert.Equal(t, cfg.URL, client.baseURL, "Base URL should match config")
		assert.Equal(t, cfg.APIKey, client.apiKey, "API key should match config")
		assert.NotNil(t, client.client, "HTTP client should be initialized")
	})

	t.Run("NewSonarrClient_DefaultTimeout", func(t *testing.T) {
		cfg := config.SonarrConfig{
			BaseIntegrationConfig: config.BaseIntegrationConfig{
				URL:    "http://localhost:8989",
				APIKey: "test-api-key",
				// No timeout specified
			},
		}

		client := NewSonarrClient(cfg)

		assert.NotNil(t, client, "Client should not be nil")
		assert.Equal(t, 30*time.Second, client.client.Timeout, "Should use default 30s timeout")
	})

	t.Run("NewSonarrClient_InvalidTimeout", func(t *testing.T) {
		cfg := config.SonarrConfig{
			BaseIntegrationConfig: config.BaseIntegrationConfig{
				URL:     "http://localhost:8989",
				APIKey:  "test-api-key",
				Timeout: "invalid",
			},
		}

		client := NewSonarrClient(cfg)

		// Should fall back to default timeout
		assert.Equal(t, 30*time.Second, client.client.Timeout, "Should use default timeout for invalid value")
	})

	t.Run("NewSonarrClient_CustomTimeout", func(t *testing.T) {
		cfg := config.SonarrConfig{
			BaseIntegrationConfig: config.BaseIntegrationConfig{
				URL:     "http://localhost:8989",
				APIKey:  "test-api-key",
				Timeout: "2m",
			},
		}

		client := NewSonarrClient(cfg)

		assert.Equal(t, 2*time.Minute, client.client.Timeout, "Should use custom timeout")
	})
}
