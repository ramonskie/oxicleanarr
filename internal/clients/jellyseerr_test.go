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

// TestJellyseerrIntegration runs integration tests against a real Jellyseerr instance
// Set PRUNARR_INTEGRATION_TEST=1 to enable these tests
func TestJellyseerrIntegration(t *testing.T) {
	if os.Getenv("PRUNARR_INTEGRATION_TEST") != "1" {
		t.Skip("Skipping integration test. Set PRUNARR_INTEGRATION_TEST=1 to run.")
	}

	// Load test configuration
	cfg, err := config.Load("../../config/prunarr.test.yaml")
	require.NoError(t, err, "Failed to load test config")
	require.True(t, cfg.Integrations.Jellyseerr.Enabled, "Jellyseerr must be enabled in test config")

	client := NewJellyseerrClient(cfg.Integrations.Jellyseerr)
	ctx := context.Background()

	t.Run("Ping", func(t *testing.T) {
		err := client.Ping(ctx)
		assert.NoError(t, err, "Should be able to ping Jellyseerr")
	})

	t.Run("GetRequests", func(t *testing.T) {
		requests, err := client.GetRequests(ctx)
		require.NoError(t, err, "Should be able to fetch requests")

		t.Logf("Found %d requests in Jellyseerr", len(requests))

		if len(requests) == 0 {
			t.Log("Warning: No requests found in Jellyseerr")
			return
		}

		// Validate first request structure
		request := requests[0]
		assert.NotZero(t, request.ID, "Request should have an ID")
		assert.NotEmpty(t, request.Type, "Request should have a type")
		assert.NotZero(t, request.Status, "Request should have a status")
		assert.False(t, request.CreatedAt.IsZero(), "Request should have a created date")

		t.Logf("Sample request:")
		t.Logf("  ID: %d", request.ID)
		t.Logf("  Type: %s", request.Type)
		t.Logf("  Status: %d", request.Status)
		t.Logf("  Created: %s", request.CreatedAt.Format(time.RFC3339))

		// Check media info
		if request.Media.TmdbId > 0 {
			t.Logf("  TMDB ID: %d", request.Media.TmdbId)
		}
		if request.Media.TvdbId > 0 {
			t.Logf("  TVDB ID: %d", request.Media.TvdbId)
		}

		// Check requester info
		assert.NotZero(t, request.RequestedBy.ID, "Request should have a requester ID")
		t.Logf("  Requested by: %s (ID: %d)", request.RequestedBy.Username, request.RequestedBy.ID)
		if request.RequestedBy.Email != "" {
			t.Logf("  Requester email: %s", request.RequestedBy.Email)
		}
	})

	t.Run("RequestTypesValidation", func(t *testing.T) {
		requests, err := client.GetRequests(ctx)
		require.NoError(t, err, "Should be able to fetch requests")

		if len(requests) == 0 {
			t.Skip("No requests available")
		}

		// Count by type
		typeCounts := make(map[string]int)
		statusCounts := make(map[int]int)

		for _, req := range requests {
			typeCounts[req.Type]++
			statusCounts[req.Status]++
		}

		t.Logf("Request breakdown:")
		t.Logf("  Total requests: %d", len(requests))
		t.Logf("  By type:")
		for reqType, count := range typeCounts {
			t.Logf("    %s: %d (%.1f%%)", reqType, count, float64(count)/float64(len(requests))*100)
		}
		t.Logf("  By status:")
		for status, count := range statusCounts {
			statusName := "unknown"
			switch status {
			case 1:
				statusName = "pending"
			case 2:
				statusName = "approved"
			case 3:
				statusName = "declined"
			}
			t.Logf("    Status %d (%s): %d (%.1f%%)", status, statusName, count, float64(count)/float64(len(requests))*100)
		}
	})

	t.Run("RequesterValidation", func(t *testing.T) {
		requests, err := client.GetRequests(ctx)
		require.NoError(t, err, "Should be able to fetch requests")

		if len(requests) == 0 {
			t.Skip("No requests available")
		}

		// Count unique requesters
		uniqueUsers := make(map[int]string)
		for _, req := range requests {
			uniqueUsers[req.RequestedBy.ID] = req.RequestedBy.Username
		}

		t.Logf("Requester statistics:")
		t.Logf("  Total unique users: %d", len(uniqueUsers))
		t.Logf("  Average requests per user: %.2f", float64(len(requests))/float64(len(uniqueUsers)))

		// Validate all requests have valid requester data
		for _, req := range requests {
			assert.NotZero(t, req.RequestedBy.ID, "All requests should have a requester ID")
			assert.NotEmpty(t, req.RequestedBy.Username, "All requests should have a requester username")
		}
	})

	t.Run("MediaIDsValidation", func(t *testing.T) {
		requests, err := client.GetRequests(ctx)
		require.NoError(t, err, "Should be able to fetch requests")

		if len(requests) == 0 {
			t.Skip("No requests available")
		}

		// Count requests with various IDs
		var (
			withTmdb int
			withTvdb int
		)

		for _, req := range requests {
			if req.Media.TmdbId > 0 {
				withTmdb++
			}
			if req.Media.TvdbId > 0 {
				withTvdb++
			}
		}

		t.Logf("Media ID coverage:")
		t.Logf("  Requests with TMDB ID: %d (%.1f%%)",
			withTmdb,
			float64(withTmdb)/float64(len(requests))*100)
		t.Logf("  Requests with TVDB ID: %d (%.1f%%)",
			withTvdb,
			float64(withTvdb)/float64(len(requests))*100)

		// At least some requests should have media IDs
		assert.True(t, withTmdb > 0 || withTvdb > 0, "At least some requests should have media IDs")
	})

	t.Run("PaginationHandling", func(t *testing.T) {
		// Test that pagination works correctly by fetching all requests
		requests, err := client.GetRequests(ctx)
		require.NoError(t, err, "Should handle pagination correctly")

		t.Logf("Pagination test:")
		t.Logf("  Total requests fetched: %d", len(requests))
		t.Logf("  Expected pagination: %d pages (50 items per page)", (len(requests)/50)+1)

		// Verify no duplicate IDs (would indicate pagination bug)
		seenIDs := make(map[int]bool)
		for _, req := range requests {
			assert.False(t, seenIDs[req.ID], "Should not have duplicate request IDs (pagination bug)")
			seenIDs[req.ID] = true
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
			_, err := client.GetRequests(ctx)
			results <- err
		}()

		// Collect results
		for i := 0; i < 2; i++ {
			err := <-results
			assert.NoError(t, err, "Concurrent request should succeed")
		}
	})
}

// TestJellyseerrClient_Unit runs unit tests that don't require a real Jellyseerr instance
func TestJellyseerrClient_Unit(t *testing.T) {
	t.Run("NewJellyseerrClient", func(t *testing.T) {
		cfg := config.JellyseerrConfig{
			BaseIntegrationConfig: config.BaseIntegrationConfig{
				URL:     "http://localhost:5055",
				APIKey:  "test-api-key",
				Timeout: "30s",
			},
		}

		client := NewJellyseerrClient(cfg)

		assert.NotNil(t, client, "Client should not be nil")
		assert.Equal(t, cfg.URL, client.baseURL, "Base URL should match config")
		assert.Equal(t, cfg.APIKey, client.apiKey, "API key should match config")
		assert.NotNil(t, client.client, "HTTP client should be initialized")
	})

	t.Run("NewJellyseerrClient_DefaultTimeout", func(t *testing.T) {
		cfg := config.JellyseerrConfig{
			BaseIntegrationConfig: config.BaseIntegrationConfig{
				URL:    "http://localhost:5055",
				APIKey: "test-api-key",
				// No timeout specified
			},
		}

		client := NewJellyseerrClient(cfg)

		assert.NotNil(t, client, "Client should not be nil")
		assert.Equal(t, 30*time.Second, client.client.Timeout, "Should use default 30s timeout")
	})

	t.Run("NewJellyseerrClient_InvalidTimeout", func(t *testing.T) {
		cfg := config.JellyseerrConfig{
			BaseIntegrationConfig: config.BaseIntegrationConfig{
				URL:     "http://localhost:5055",
				APIKey:  "test-api-key",
				Timeout: "invalid",
			},
		}

		client := NewJellyseerrClient(cfg)

		// Should fall back to default timeout
		assert.Equal(t, 30*time.Second, client.client.Timeout, "Should use default timeout for invalid value")
	})

	t.Run("NewJellyseerrClient_CustomTimeout", func(t *testing.T) {
		cfg := config.JellyseerrConfig{
			BaseIntegrationConfig: config.BaseIntegrationConfig{
				URL:     "http://localhost:5055",
				APIKey:  "test-api-key",
				Timeout: "45s",
			},
		}

		client := NewJellyseerrClient(cfg)

		assert.Equal(t, 45*time.Second, client.client.Timeout, "Should use custom timeout")
	})
}
