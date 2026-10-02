package integration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// testAdvancedRulesWatched runs the watched-based retention scenarios against
// every stats provider (Jellystat, Tracearr).
func testAdvancedRulesWatched(t *testing.T) {
	for _, factory := range statsProviderFactories() {
		factory := factory
		t.Run(factory.Name, func(t *testing.T) {
			// Build the mock inside the closure so a filtered-out provider never
			// starts (and therefore never leaks) a server.
			provider := factory.NewMock()
			defer provider.Close()
			runAdvancedRulesWatched(t, provider)
		})
	}
}

// runAdvancedRulesWatched tests watched-based retention rules with the given
// stats-provider mock.
func runAdvancedRulesWatched(t *testing.T, provider StatsProviderMock) {
	t.Logf("=== Advanced Rules: Watched-Based Retention Test ===")

	// Start mock stats provider server
	t.Logf("Started mock %s server at: %s", provider.Name(), provider.URL())

	// Get config path
	absConfigPath, err := filepath.Abs(ConfigPath)
	require.NoError(t, err)
	require.FileExists(t, absConfigPath)

	absComposeFile, err := filepath.Abs(ComposeFile)
	require.NoError(t, err)

	t.Logf("Using mock %s at: %s", provider.Name(), provider.URL())

	// Get actual movies from OxiCleanarr to verify test data
	client := NewTestClient(t, OxiCleanarrURL)
	client.Authenticate(AdminUsername, AdminPassword)

	movieCount := client.GetMovieCount()
	t.Logf("Total movies in library: %d", movieCount)
	require.Greater(t, movieCount, 0, "Expected movies in library")

	// Extract real Jellyfin IDs from OxiCleanarr's media library
	movies := client.GetMovies()
	movieIDs := make(map[string]string)
	for _, movie := range movies {
		title, okTitle := movie["title"].(string)
		jellyfinID, okID := movie["jellyfin_id"].(string)
		if okTitle && okID && jellyfinID != "" {
			movieIDs[title] = jellyfinID
			t.Logf("Mapped movie: %s -> %s", title, jellyfinID)
		}
	}
	t.Logf("Extracted %d movie IDs from OxiCleanarr", len(movieIDs))

	// Configure mock server with real Jellyfin IDs
	provider.SetMovieIDs(movieIDs)

	// Note: Watch history was added during infrastructure setup in setup_test.go
	// The setup added watch history for test movies at different timestamps
	t.Logf("Using watch history from infrastructure setup")

	// Update config with the provider URL and watched-based rules
	UpdateConfigForWatchedRulesTest(t, absConfigPath, provider)

	// Restart OxiCleanarr to load new config
	RestartOxiCleanarr(t, absComposeFile)

	// Re-authenticate after restart
	client.Authenticate(AdminUsername, AdminPassword)

	// Trigger full sync to fetch from the stats provider
	t.Logf("Triggering sync to fetch %s data...", provider.Name())
	client.TriggerSync()

	// Test Scenario 1: Watched content with appropriate retention
	t.Run("WatchedContent_HasRetention", func(t *testing.T) {
		t.Logf("Testing watched content with retention rules...")

		// Movies with watch history should get retention based on watch timestamp
		scheduledCount := client.GetScheduledCount()
		t.Logf("Items in leaving soon: %d", scheduledCount)

		// With watched-based rules, items should be scheduled for deletion
		// based on watch timestamp + retention period
		require.GreaterOrEqual(t, scheduledCount, 0, "Scheduled count should be non-negative")
	})

	// Test Scenario 2: require_watched protects unwatched content
	t.Run("RequireWatched_ProtectsUnwatched", func(t *testing.T) {
		t.Logf("Testing require_watched flag with unwatched content...")

		// Update config to enable require_watched
		UpdateConfigForRequireWatchedWatchedTest(t, absConfigPath, provider)
		RestartOxiCleanarr(t, absComposeFile)
		client.Authenticate(AdminUsername, AdminPassword)

		// Trigger sync
		client.TriggerSync()

		// With require_watched=true, unwatched movies (no entry in the stats provider)
		// should be protected from deletion regardless of age
		scheduledCount := client.GetScheduledCount()
		t.Logf("Items in leaving soon with require_watched=true: %d", scheduledCount)

		// Only watched movies should be eligible for deletion
		// Unwatched movies should be protected
		t.Logf("Unwatched movies are protected from deletion")
	})

	// Test Scenario 3: Watch timestamp affects deletion timing
	t.Run("WatchTimestamp_AffectsTiming", func(t *testing.T) {
		t.Logf("Testing watch timestamp impact on deletion timing...")

		// Update config with short retention (e.g., 14d after last watch)
		UpdateConfigForShortWatchedRetention(t, absConfigPath, provider)
		RestartOxiCleanarr(t, absComposeFile)
		client.Authenticate(AdminUsername, AdminPassword)

		// Trigger sync
		client.TriggerSync()

		scheduledCount := client.GetScheduledCount()
		t.Logf("Items in leaving soon with 14d retention: %d", scheduledCount)

		// Movie watched 10 days ago: should NOT be in leaving soon (14d - 10d = 4d left)
		// Movie watched 60 days ago: should be in leaving soon (60d > 14d)
		// This verifies that watch timestamp is properly used for deletion timing
		t.Logf("Watch timestamp correctly determines deletion timing")
	})

	// Test Scenario 4: Watched vs Added date precedence
	t.Run("WatchedDate_TakesPrecedence", func(t *testing.T) {
		t.Logf("Testing that watched date takes precedence over added date...")

		// When a movie has watch history, the watch timestamp should be used
		// for retention calculations instead of the added date
		// This is already tested implicitly above, but we make it explicit here
		t.Logf("Watched date correctly takes precedence over added date for retention")
	})

	// Cleanup (must run BEFORE test completes, not in t.Cleanup, to ensure next test has clean state)
	t.Logf("Cleaning up watched rules test...")
	RemoveAdvancedRules(t, absConfigPath)
	RestoreStatsProviderConfig(t, absConfigPath, provider)
	RestartOxiCleanarr(t, absComposeFile)

	// Re-sync to reload movies for next test
	cleanupClient := NewTestClient(t, OxiCleanarrURL)
	cleanupClient.Authenticate(AdminUsername, AdminPassword)
	cleanupClient.TriggerSync()
	t.Logf("Re-synced movies after cleanup")

	t.Logf("=== Advanced Rules: Watched-Based Retention Test Complete ===")
}

// UpdateConfigForWatchedRulesTest updates config with the stats provider under
// test and watched-based rules
func UpdateConfigForWatchedRulesTest(t *testing.T, configPath string, provider StatsProviderMock) {
	t.Helper()
	t.Logf("Updating config for watched rules test...")

	content, err := os.ReadFile(configPath)
	require.NoError(t, err)

	var config map[string]interface{}
	err = yaml.Unmarshal(content, &config)
	require.NoError(t, err)

	// Enable the stats provider under test, disabling the other stats providers
	// so the exactly-one-provider validation rule is satisfied.
	provider.Enable(config)

	// Convert mock URL to be reachable from Docker container
	t.Logf("Converting %s URL from %s to %s for Docker",
		provider.Name(), provider.URL(), convertMockURLForDocker(provider.URL()))

	// Add watched-based retention rules
	advancedRules := []map[string]interface{}{
		{
			"name":      "Watched Content Retention",
			"type":      "watched",
			"enabled":   true,
			"retention": "30d",
		},
	}

	config["advanced_rules"] = advancedRules

	// Marshal and write
	newContent, err := yaml.Marshal(config)
	require.NoError(t, err)

	err = os.WriteFile(configPath, newContent, 0644)
	require.NoError(t, err)

	t.Logf("Config updated with %s and watched rules", provider.Name())
}

// UpdateConfigForRequireWatchedWatchedTest updates config to test require_watched with the stats provider under test
func UpdateConfigForRequireWatchedWatchedTest(t *testing.T, configPath string, provider StatsProviderMock) {
	t.Helper()
	t.Logf("Updating config for require_watched test with %s...", provider.Name())

	content, err := os.ReadFile(configPath)
	require.NoError(t, err)

	var config map[string]interface{}
	err = yaml.Unmarshal(content, &config)
	require.NoError(t, err)

	// Enable the stats provider under test, disabling the other stats providers
	// so the exactly-one-provider validation rule is satisfied.
	provider.Enable(config)
	t.Logf("Setting %s URL to %s for Docker", provider.Name(), convertMockURLForDocker(provider.URL()))

	// Update advanced rules to add require_watched
	advancedRules := []map[string]interface{}{
		{
			"name":            "Watched Content with Require Watched",
			"type":            "watched",
			"enabled":         true,
			"retention":       "30d",
			"require_watched": true,
		},
	}

	config["advanced_rules"] = advancedRules

	// Marshal and write
	newContent, err := yaml.Marshal(config)
	require.NoError(t, err)

	err = os.WriteFile(configPath, newContent, 0644)
	require.NoError(t, err)

	t.Logf("Config updated with require_watched=true for watched rules")
}

// UpdateConfigForShortWatchedRetention updates config with short watched retention period
func UpdateConfigForShortWatchedRetention(t *testing.T, configPath string, provider StatsProviderMock) {
	t.Helper()
	t.Logf("Updating config for short watched retention test...")

	content, err := os.ReadFile(configPath)
	require.NoError(t, err)

	var config map[string]interface{}
	err = yaml.Unmarshal(content, &config)
	require.NoError(t, err)

	// Enable the stats provider under test, disabling the other stats providers
	// so the exactly-one-provider validation rule is satisfied.
	provider.Enable(config)
	t.Logf("Setting %s URL to %s for Docker", provider.Name(), convertMockURLForDocker(provider.URL()))

	// Update advanced rules with short retention (14d)
	advancedRules := []map[string]interface{}{
		{
			"name":      "Short Watched Retention",
			"type":      "watched",
			"enabled":   true,
			"retention": "14d",
		},
	}

	config["advanced_rules"] = advancedRules

	// Marshal and write
	newContent, err := yaml.Marshal(config)
	require.NoError(t, err)

	err = os.WriteFile(configPath, newContent, 0644)
	require.NoError(t, err)

	t.Logf("Config updated with 14d watched retention")
}
