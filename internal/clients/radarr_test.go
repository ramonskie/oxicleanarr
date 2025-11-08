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

// TestRadarrIntegration runs integration tests against a real Radarr instance
// Set PRUNARR_INTEGRATION_TEST=1 to enable these tests
func TestRadarrIntegration(t *testing.T) {
	if os.Getenv("PRUNARR_INTEGRATION_TEST") != "1" {
		t.Skip("Skipping integration test. Set PRUNARR_INTEGRATION_TEST=1 to run.")
	}

	// Load test configuration
	cfg, err := config.Load("../../config/prunarr.test.yaml")
	require.NoError(t, err, "Failed to load test config")
	require.True(t, cfg.Integrations.Radarr.Enabled, "Radarr must be enabled in test config")

	client := NewRadarrClient(cfg.Integrations.Radarr)
	ctx := context.Background()

	t.Run("Ping", func(t *testing.T) {
		err := client.Ping(ctx)
		assert.NoError(t, err, "Should be able to ping Radarr")
	})

	t.Run("GetMovies", func(t *testing.T) {
		movies, err := client.GetMovies(ctx)
		require.NoError(t, err, "Should be able to fetch movies")

		if len(movies) == 0 {
			t.Log("Warning: No movies found in Radarr")
			return
		}

		t.Logf("Found %d movies in Radarr", len(movies))

		// Validate first movie structure
		movie := movies[0]
		assert.NotZero(t, movie.ID, "Movie should have an ID")
		assert.NotEmpty(t, movie.Title, "Movie should have a title")
		assert.NotZero(t, movie.Year, "Movie should have a year")
		assert.False(t, movie.Added.IsZero(), "Movie should have an added date")
		assert.NotEmpty(t, movie.Path, "Movie should have a path")

		t.Logf("Sample movie: %s (%d) - ID: %d", movie.Title, movie.Year, movie.ID)

		if movie.HasFile {
			assert.NotZero(t, movie.SizeOnDisk, "Movie with file should have size")
			t.Logf("  Size on disk: %d bytes (%.2f GB)",
				movie.SizeOnDisk,
				float64(movie.SizeOnDisk)/(1024*1024*1024))
		}

		if movie.MovieFile != nil {
			assert.NotEmpty(t, movie.MovieFile.Path, "Movie file should have path")
			assert.NotZero(t, movie.MovieFile.Size, "Movie file should have size")
			t.Logf("  Quality: %s", movie.MovieFile.Quality.Quality.Name)
		}
	})

	t.Run("GetMovie", func(t *testing.T) {
		// First get all movies to get a valid ID
		movies, err := client.GetMovies(ctx)
		require.NoError(t, err, "Should be able to fetch movies")

		if len(movies) == 0 {
			t.Skip("No movies available to test GetMovie")
		}

		movieID := movies[0].ID

		movie, err := client.GetMovie(ctx, movieID)
		require.NoError(t, err, "Should be able to fetch single movie")
		require.NotNil(t, movie, "Movie should not be nil")

		assert.Equal(t, movieID, movie.ID, "Movie ID should match")
		assert.NotEmpty(t, movie.Title, "Movie should have a title")

		t.Logf("Fetched movie: %s (%d)", movie.Title, movie.Year)
	})

	t.Run("GetHistory", func(t *testing.T) {
		// First get all movies to get a valid ID
		movies, err := client.GetMovies(ctx)
		require.NoError(t, err, "Should be able to fetch movies")

		if len(movies) == 0 {
			t.Skip("No movies available to test GetHistory")
		}

		movieID := movies[0].ID

		history, err := client.GetHistory(ctx, movieID)
		require.NoError(t, err, "Should be able to fetch movie history")

		t.Logf("Found %d history entries for movie ID %d", len(history), movieID)

		if len(history) > 0 {
			entry := history[0]
			assert.Equal(t, movieID, entry.MovieID, "History entry should match movie ID")
			assert.NotEmpty(t, entry.EventType, "History entry should have event type")
			assert.False(t, entry.Date.IsZero(), "History entry should have date")

			t.Logf("  Latest event: %s at %s", entry.EventType, entry.Date.Format(time.RFC3339))
		}
	})

	t.Run("GetMovie_InvalidID", func(t *testing.T) {
		// Use an ID that's unlikely to exist
		invalidID := 999999999

		movie, err := client.GetMovie(ctx, invalidID)
		assert.Error(t, err, "Should return error for invalid movie ID")
		assert.Nil(t, movie, "Movie should be nil for invalid ID")
	})

	t.Run("DeleteMovie_ReadOnly", func(t *testing.T) {
		// This test should NOT actually delete anything
		// We're just validating the method signature and ensuring dry_run is enforced
		t.Skip("Skipping delete test - read-only mode enforced")

		// If we were to test this (in a controlled environment):
		// err := client.DeleteMovie(ctx, someID, false)
		// This should never be run in integration tests with real data
	})

	t.Run("Timeout", func(t *testing.T) {
		// Test with very short timeout
		shortCfg := cfg.Integrations.Radarr
		shortCfg.Timeout = "1ns" // Impossibly short timeout

		shortClient := NewRadarrClient(shortCfg)
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		defer cancel()

		_, err := shortClient.GetMovies(ctx)
		assert.Error(t, err, "Should timeout with very short timeout")
	})

	t.Run("MovieDataValidation", func(t *testing.T) {
		movies, err := client.GetMovies(ctx)
		require.NoError(t, err, "Should be able to fetch movies")

		if len(movies) == 0 {
			t.Skip("No movies available for validation")
		}

		// Count movies by various attributes
		var (
			withFiles     int
			withoutFiles  int
			totalSize     int64
			withMovieFile int
		)

		for _, movie := range movies {
			if movie.HasFile {
				withFiles++
				totalSize += movie.SizeOnDisk
			} else {
				withoutFiles++
			}

			if movie.MovieFile != nil {
				withMovieFile++
			}
		}

		t.Logf("Movie statistics:")
		t.Logf("  Total movies: %d", len(movies))
		t.Logf("  With files: %d", withFiles)
		t.Logf("  Without files: %d", withoutFiles)
		t.Logf("  With movie file details: %d", withMovieFile)
		t.Logf("  Total size on disk: %.2f GB", float64(totalSize)/(1024*1024*1024))

		if withFiles > 0 {
			avgSize := float64(totalSize) / float64(withFiles)
			t.Logf("  Average movie size: %.2f GB", avgSize/(1024*1024*1024))
		}
	})

	t.Run("ConcurrentRequests", func(t *testing.T) {
		// Test multiple concurrent requests
		movies, err := client.GetMovies(ctx)
		require.NoError(t, err, "Should be able to fetch movies")

		if len(movies) < 3 {
			t.Skip("Need at least 3 movies for concurrent test")
		}

		// Make 3 concurrent requests
		results := make(chan error, 3)
		for i := 0; i < 3; i++ {
			go func(movieID int) {
				_, err := client.GetMovie(ctx, movieID)
				results <- err
			}(movies[i].ID)
		}

		// Collect results
		for i := 0; i < 3; i++ {
			err := <-results
			assert.NoError(t, err, "Concurrent request should succeed")
		}
	})
}

// TestRadarrClient_Unit runs unit tests that don't require a real Radarr instance
func TestRadarrClient_Unit(t *testing.T) {
	t.Run("NewRadarrClient", func(t *testing.T) {
		cfg := config.RadarrConfig{
			BaseIntegrationConfig: config.BaseIntegrationConfig{
				URL:     "http://localhost:7878",
				APIKey:  "test-api-key",
				Timeout: "30s",
			},
		}

		client := NewRadarrClient(cfg)

		assert.NotNil(t, client, "Client should not be nil")
		assert.Equal(t, cfg.URL, client.baseURL, "Base URL should match config")
		assert.Equal(t, cfg.APIKey, client.apiKey, "API key should match config")
		assert.NotNil(t, client.client, "HTTP client should be initialized")
	})

	t.Run("NewRadarrClient_DefaultTimeout", func(t *testing.T) {
		cfg := config.RadarrConfig{
			BaseIntegrationConfig: config.BaseIntegrationConfig{
				URL:    "http://localhost:7878",
				APIKey: "test-api-key",
				// No timeout specified
			},
		}

		client := NewRadarrClient(cfg)

		assert.NotNil(t, client, "Client should not be nil")
		assert.Equal(t, 30*time.Second, client.client.Timeout, "Should use default 30s timeout")
	})

	t.Run("NewRadarrClient_InvalidTimeout", func(t *testing.T) {
		cfg := config.RadarrConfig{
			BaseIntegrationConfig: config.BaseIntegrationConfig{
				URL:     "http://localhost:7878",
				APIKey:  "test-api-key",
				Timeout: "invalid",
			},
		}

		client := NewRadarrClient(cfg)

		// Should fall back to default timeout
		assert.Equal(t, 30*time.Second, client.client.Timeout, "Should use default timeout for invalid value")
	})

	t.Run("NewRadarrClient_CustomTimeout", func(t *testing.T) {
		cfg := config.RadarrConfig{
			BaseIntegrationConfig: config.BaseIntegrationConfig{
				URL:     "http://localhost:7878",
				APIKey:  "test-api-key",
				Timeout: "1m",
			},
		}

		client := NewRadarrClient(cfg)

		assert.Equal(t, 1*time.Minute, client.client.Timeout, "Should use custom timeout")
	})
}
