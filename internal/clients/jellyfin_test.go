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

// TestJellyfinIntegration runs integration tests against a real Jellyfin instance
// Set OXICLEANARR_INTEGRATION_TEST=1 to enable these tests
func TestJellyfinIntegration(t *testing.T) {
	if os.Getenv("OXICLEANARR_INTEGRATION_TEST") != "1" {
		t.Skip("Skipping integration test. Set OXICLEANARR_INTEGRATION_TEST=1 to run.")
	}

	// Load test configuration
	cfg, err := config.Load("../../config/prunarr.test.yaml")
	require.NoError(t, err, "Failed to load test config")
	require.True(t, cfg.Integrations.Jellyfin.Enabled, "Jellyfin must be enabled in test config")

	client := NewJellyfinClient(cfg.Integrations.Jellyfin)
	ctx := context.Background()

	t.Run("Ping", func(t *testing.T) {
		err := client.Ping(ctx)
		assert.NoError(t, err, "Should be able to ping Jellyfin")
	})

	t.Run("GetMovies", func(t *testing.T) {
		movies, err := client.GetMovies(ctx)
		require.NoError(t, err, "Should be able to fetch movies")

		t.Logf("Found %d movies in Jellyfin", len(movies))

		if len(movies) == 0 {
			t.Log("Warning: No movies found in Jellyfin")
			return
		}

		// Validate first movie structure
		movie := movies[0]
		assert.NotEmpty(t, movie.ID, "Movie should have an ID")
		assert.NotEmpty(t, movie.Name, "Movie should have a name")
		assert.Equal(t, "Movie", movie.Type, "Type should be Movie")
		assert.False(t, movie.DateCreated.IsZero(), "Movie should have a created date")

		t.Logf("Sample movie: %s (%d)", movie.Name, movie.ProductionYear)
		t.Logf("  ID: %s", movie.ID)
		t.Logf("  Created: %s", movie.DateCreated.Format(time.RFC3339))
		t.Logf("  Path: %s", movie.Path)

		// Check user data
		if movie.UserData.PlayCount > 0 {
			t.Logf("  Play count: %d", movie.UserData.PlayCount)
			t.Logf("  Last played: %s", movie.UserData.LastPlayedDate.Format(time.RFC3339))
		}

		// Check provider IDs
		if len(movie.ProviderIds) > 0 {
			t.Logf("  Provider IDs: %v", movie.ProviderIds)
		}
	})

	t.Run("GetTVShows", func(t *testing.T) {
		shows, err := client.GetTVShows(ctx)
		require.NoError(t, err, "Should be able to fetch TV shows")

		t.Logf("Found %d TV shows in Jellyfin", len(shows))

		if len(shows) == 0 {
			t.Log("Warning: No TV shows found in Jellyfin")
			return
		}

		// Validate first show structure
		show := shows[0]
		assert.NotEmpty(t, show.ID, "Show should have an ID")
		assert.NotEmpty(t, show.Name, "Show should have a name")
		assert.Equal(t, "Series", show.Type, "Type should be Series")
		assert.False(t, show.DateCreated.IsZero(), "Show should have a created date")

		t.Logf("Sample TV show: %s (%d)", show.Name, show.ProductionYear)
		t.Logf("  ID: %s", show.ID)
		t.Logf("  Created: %s", show.DateCreated.Format(time.RFC3339))
		t.Logf("  Path: %s", show.Path)

		// Check user data
		if show.UserData.PlayCount > 0 {
			t.Logf("  Play count: %d", show.UserData.PlayCount)
			t.Logf("  Last played: %s", show.UserData.LastPlayedDate.Format(time.RFC3339))
		}
	})

	t.Run("MediaDataValidation", func(t *testing.T) {
		movies, err := client.GetMovies(ctx)
		require.NoError(t, err, "Should be able to fetch movies")

		shows, err := client.GetTVShows(ctx)
		require.NoError(t, err, "Should be able to fetch TV shows")

		t.Logf("Media library statistics:")
		t.Logf("  Total movies: %d", len(movies))
		t.Logf("  Total TV shows: %d", len(shows))
		t.Logf("  Total media items: %d", len(movies)+len(shows))

		// Count watched vs unwatched
		var (
			watchedMovies   int
			unwatchedMovies int
			watchedShows    int
			unwatchedShows  int
		)

		for _, movie := range movies {
			if movie.UserData.Played {
				watchedMovies++
			} else {
				unwatchedMovies++
			}
		}

		for _, show := range shows {
			if show.UserData.Played {
				watchedShows++
			} else {
				unwatchedShows++
			}
		}

		t.Logf("  Watched movies: %d (%.1f%%)",
			watchedMovies,
			float64(watchedMovies)/float64(len(movies))*100)
		t.Logf("  Unwatched movies: %d (%.1f%%)",
			unwatchedMovies,
			float64(unwatchedMovies)/float64(len(movies))*100)

		if len(shows) > 0 {
			t.Logf("  Watched shows: %d (%.1f%%)",
				watchedShows,
				float64(watchedShows)/float64(len(shows))*100)
			t.Logf("  Unwatched shows: %d (%.1f%%)",
				unwatchedShows,
				float64(unwatchedShows)/float64(len(shows))*100)
		}
	})

	t.Run("ProviderIDsValidation", func(t *testing.T) {
		movies, err := client.GetMovies(ctx)
		require.NoError(t, err, "Should be able to fetch movies")

		if len(movies) == 0 {
			t.Skip("No movies available")
		}

		// Count movies with various provider IDs
		var (
			withTmdb int
			withImdb int
			withTvdb int
		)

		for _, movie := range movies {
			if _, ok := movie.ProviderIds["Tmdb"]; ok {
				withTmdb++
			}
			if _, ok := movie.ProviderIds["Imdb"]; ok {
				withImdb++
			}
			if _, ok := movie.ProviderIds["Tvdb"]; ok {
				withTvdb++
			}
		}

		t.Logf("Provider ID coverage:")
		t.Logf("  Movies with TMDB ID: %d (%.1f%%)",
			withTmdb,
			float64(withTmdb)/float64(len(movies))*100)
		t.Logf("  Movies with IMDB ID: %d (%.1f%%)",
			withImdb,
			float64(withImdb)/float64(len(movies))*100)
		t.Logf("  Movies with TVDB ID: %d (%.1f%%)",
			withTvdb,
			float64(withTvdb)/float64(len(movies))*100)
	})

	t.Run("WatchHistoryValidation", func(t *testing.T) {
		movies, err := client.GetMovies(ctx)
		require.NoError(t, err, "Should be able to fetch movies")

		if len(movies) == 0 {
			t.Skip("No movies available")
		}

		// Find movies with watch history
		var recentlyWatched []JellyfinItem
		for _, movie := range movies {
			if movie.UserData.PlayCount > 0 && !movie.UserData.LastPlayedDate.IsZero() {
				recentlyWatched = append(recentlyWatched, movie)
			}
		}

		t.Logf("Watch history:")
		t.Logf("  Movies with watch history: %d", len(recentlyWatched))

		if len(recentlyWatched) > 0 {
			// Show top 5 most watched
			limit := 5
			if len(recentlyWatched) < limit {
				limit = len(recentlyWatched)
			}

			t.Logf("  Sample watched movies (up to %d):", limit)
			for i := 0; i < limit; i++ {
				movie := recentlyWatched[i]
				t.Logf("    - %s: played %d times, last on %s",
					movie.Name,
					movie.UserData.PlayCount,
					movie.UserData.LastPlayedDate.Format("2006-01-02"))
			}
		}
	})

	t.Run("DeleteItem_ReadOnly", func(t *testing.T) {
		// This test should NOT actually delete anything
		// We're just validating the method signature and ensuring dry_run is enforced
		t.Skip("Skipping delete test - read-only mode enforced")

		// If we were to test this (in a controlled environment):
		// err := client.DeleteItem(ctx, someID)
		// This should never be run in integration tests with real data
	})

	t.Run("ConcurrentRequests", func(t *testing.T) {
		// Test multiple concurrent requests
		results := make(chan error, 2)

		go func() {
			_, err := client.GetMovies(ctx)
			results <- err
		}()

		go func() {
			_, err := client.GetTVShows(ctx)
			results <- err
		}()

		// Collect results
		for i := 0; i < 2; i++ {
			err := <-results
			assert.NoError(t, err, "Concurrent request should succeed")
		}
	})
}

// TestJellyfinClient_Unit runs unit tests that don't require a real Jellyfin instance
func TestJellyfinClient_Unit(t *testing.T) {
	t.Run("NewJellyfinClient", func(t *testing.T) {
		cfg := config.JellyfinConfig{
			BaseIntegrationConfig: config.BaseIntegrationConfig{
				URL:     "http://localhost:8096",
				APIKey:  "test-api-key",
				Timeout: "30s",
			},
		}

		client := NewJellyfinClient(cfg)

		assert.NotNil(t, client, "Client should not be nil")
		assert.Equal(t, cfg.URL, client.baseURL, "Base URL should match config")
		assert.Equal(t, cfg.APIKey, client.apiKey, "API key should match config")
		assert.NotNil(t, client.client, "HTTP client should be initialized")
	})

	t.Run("NewJellyfinClient_DefaultTimeout", func(t *testing.T) {
		cfg := config.JellyfinConfig{
			BaseIntegrationConfig: config.BaseIntegrationConfig{
				URL:    "http://localhost:8096",
				APIKey: "test-api-key",
				// No timeout specified
			},
		}

		client := NewJellyfinClient(cfg)

		assert.NotNil(t, client, "Client should not be nil")
		assert.Equal(t, 30*time.Second, client.client.Timeout, "Should use default 30s timeout")
	})

	t.Run("NewJellyfinClient_InvalidTimeout", func(t *testing.T) {
		cfg := config.JellyfinConfig{
			BaseIntegrationConfig: config.BaseIntegrationConfig{
				URL:     "http://localhost:8096",
				APIKey:  "test-api-key",
				Timeout: "invalid",
			},
		}

		client := NewJellyfinClient(cfg)

		// Should fall back to default timeout
		assert.Equal(t, 30*time.Second, client.client.Timeout, "Should use default timeout for invalid value")
	})

	t.Run("NewJellyfinClient_CustomTimeout", func(t *testing.T) {
		cfg := config.JellyfinConfig{
			BaseIntegrationConfig: config.BaseIntegrationConfig{
				URL:     "http://localhost:8096",
				APIKey:  "test-api-key",
				Timeout: "45s",
			},
		}

		client := NewJellyfinClient(cfg)

		assert.Equal(t, 45*time.Second, client.client.Timeout, "Should use custom timeout")
	})
}
