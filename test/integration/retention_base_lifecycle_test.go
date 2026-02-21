package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// testRetentionBaseLifecycle tests all retention_base and unwatched_behavior modes
// end-to-end against a real OxiCleanarr instance with a mock Jellystat server.
//
// Test movies available (from radarr_setup_test.go / mock_jellystat.go):
//   - Fight Club        — watched 10 days ago
//   - Pulp Fiction      — watched 60 days ago
//   - Inception         — watched 5 days ago
//   - The Dark Knight   — watched 30 days ago
//   - Interstellar      — watched 45 days ago
//   - Forrest Gump      — watched 90 days ago
//   - Schindler's List  — never watched
func testRetentionBaseLifecycle(t *testing.T) {
	t.Logf("=== Retention Base Lifecycle Test ===")

	mockJellystat := NewMockJellystatServer()
	defer mockJellystat.Close()
	jellystatURL := mockJellystat.URL()
	t.Logf("Started mock Jellystat server at: %s", jellystatURL)

	absConfigPath, err := filepath.Abs(ConfigPath)
	require.NoError(t, err)
	require.FileExists(t, absConfigPath)

	absComposeFile, err := filepath.Abs(ComposeFile)
	require.NoError(t, err)

	client := NewTestClient(t, OxiCleanarrURL)
	client.Authenticate(AdminUsername, AdminPassword)

	// Map real Jellyfin IDs into the mock server so watch history matches
	movies := client.GetMovies()
	movieIDs := make(map[string]string)
	for _, movie := range movies {
		title, okTitle := movie["title"].(string)
		jellyfinID, okID := movie["jellyfin_id"].(string)
		if okTitle && okID && jellyfinID != "" {
			movieIDs[title] = jellyfinID
		}
	}
	mockJellystat.SetMovieIDs(movieIDs)
	t.Logf("Mapped %d movie IDs into mock Jellystat", len(movieIDs))

	// ── Scenario 1: Default mode (last_watched_or_added) — backward compat ──────
	t.Run("DefaultMode_LastWatchedOrAdded", func(t *testing.T) {
		t.Logf("Scenario 1: retention_base=last_watched_or_added (default), movie_retention=30d")

		// Config: default retention_base, 30d movie retention
		// Mock: Fight Club watched 10d ago, Pulp Fiction 60d ago, Schindler's List never
		// Expected (overdue = past deletion date):
		//   Pulp Fiction (60d > 30d since last watch) → overdue → scheduled
		//   Forrest Gump (90d > 30d since last watch) → overdue → scheduled
		//   Interstellar (45d > 30d since last watch) → overdue → scheduled
		//   The Dark Knight (30d == 30d) → borderline, may or may not appear
		//   Fight Club (10d < 30d) → NOT scheduled
		//   Inception (5d < 30d) → NOT scheduled
		//   Schindler's List (never watched, falls back to AddedAt) → depends on AddedAt
		//
		// NOTE: overdue items have DaysUntilDue <= 0 and do NOT appear in /api/media/leaving-soon.
		// We use the job summary's scheduled_deletions count instead.

		UpdateConfigForRetentionBaseTest(t, absConfigPath, jellystatURL, "", "", "")
		RestartOxiCleanarr(t, absComposeFile)
		client.Authenticate(AdminUsername, AdminPassword)

		// GetJobScheduledCount triggers sync internally and reads scheduled_deletions from job summary
		scheduledCount := client.GetJobScheduledCount()
		t.Logf("Scheduled items with default mode (30d retention): %d", scheduledCount)

		// At minimum Pulp Fiction (60d), Forrest Gump (90d), Interstellar (45d) should be scheduled
		require.GreaterOrEqual(t, scheduledCount, 3,
			"Expected at least 3 items overdue with 30d retention (Pulp Fiction 60d, Forrest Gump 90d, Interstellar 45d)")
	})

	// ── Scenario 2: last_watched + unwatched_behavior=never ──────────────────────
	t.Run("LastWatched_UnwatchedNever_ProtectsUnwatched", func(t *testing.T) {
		t.Logf("Scenario 2: retention_base=last_watched, unwatched_behavior=never, movie_retention=30d")

		// Expected:
		//   Schindler's List (never watched) → PROTECTED (unwatched_behavior=never)
		//   Pulp Fiction (60d > 30d since last watch) → overdue → scheduled
		//   Forrest Gump (90d > 30d) → overdue → scheduled
		//   Fight Club (10d < 30d) → NOT scheduled
		//
		// NOTE: overdue items are not in /api/media/leaving-soon; use job summary instead.

		UpdateConfigForRetentionBaseTest(t, absConfigPath, jellystatURL, "last_watched", "never", "")
		RestartOxiCleanarr(t, absComposeFile)
		client.Authenticate(AdminUsername, AdminPassword)

		// GetJobWouldDelete triggers sync and returns the would_delete list from the job summary
		scheduledItems := client.GetJobWouldDelete()
		scheduledCount := len(scheduledItems)
		t.Logf("Scheduled items with last_watched+never (30d retention): %d", scheduledCount)

		// Schindler's List must NOT appear in scheduled deletions
		for _, item := range scheduledItems {
			title, _ := item["title"].(string)
			require.NotEqual(t, "Schindler's List", title,
				"Schindler's List (never watched) must be protected with unwatched_behavior=never")
		}

		// Watched items that are overdue should still be scheduled
		require.GreaterOrEqual(t, scheduledCount, 2,
			"Expected at least 2 overdue watched items (Pulp Fiction 60d, Forrest Gump 90d)")
	})

	// ── Scenario 3: last_watched + unwatched_behavior=added + unwatched_retention ─
	t.Run("LastWatched_UnwatchedAdded_SeparateRetention", func(t *testing.T) {
		t.Logf("Scenario 3: retention_base=last_watched, unwatched_behavior=added, unwatched_retention=180d, movie_retention=30d")

		// Expected:
		//   Schindler's List (never watched, AddedAt < 180d ago) → NOT scheduled
		//   Pulp Fiction (60d > 30d since last watch) → overdue → scheduled
		//   Forrest Gump (90d > 30d) → overdue → scheduled
		//
		// NOTE: overdue items are not in /api/media/leaving-soon; use job summary instead.

		UpdateConfigForRetentionBaseTest(t, absConfigPath, jellystatURL, "last_watched", "added", "180d")
		RestartOxiCleanarr(t, absComposeFile)
		client.Authenticate(AdminUsername, AdminPassword)

		// GetJobWouldDelete triggers sync and returns the would_delete list from the job summary
		scheduledItems := client.GetJobWouldDelete()
		t.Logf("Scheduled items with unwatched_retention=180d: %d", len(scheduledItems))

		// Schindler's List should NOT be scheduled (added recently in test setup, well under 180d)
		for _, item := range scheduledItems {
			title, _ := item["title"].(string)
			require.NotEqual(t, "Schindler's List", title,
				"Schindler's List must not be scheduled: added recently, unwatched_retention=180d")
		}
	})

	// ── Scenario 4: retention_base=added (pure age-based, ignores watch) ─────────
	t.Run("Added_IgnoresWatchActivity", func(t *testing.T) {
		t.Logf("Scenario 4: retention_base=added, movie_retention=30d")

		// All movies were added recently in the test setup (within the last few minutes).
		// With retention_base=added and 30d retention, none should be overdue.
		// This verifies that the "added" mode uses AddedAt, not LastWatched.

		UpdateConfigForRetentionBaseTest(t, absConfigPath, jellystatURL, "added", "", "")
		RestartOxiCleanarr(t, absComposeFile)
		client.Authenticate(AdminUsername, AdminPassword)

		// GetJobScheduledCount triggers sync internally and reads scheduled_deletions from job summary
		scheduledCount := client.GetJobScheduledCount()
		t.Logf("Scheduled items with retention_base=added (30d, all movies added recently): %d", scheduledCount)

		// All movies were imported during test setup (minutes ago), so none should be overdue
		require.Equal(t, 0, scheduledCount,
			"With retention_base=added and 30d retention, recently-added movies should not be scheduled")
	})

	// ── Scenario 5: Per-rule tag override ────────────────────────────────────────
	t.Run("PerRule_TagRule_RetentionBaseOverride", func(t *testing.T) {
		t.Logf("Scenario 5: global retention_base=added, tag rule with retention_base=last_watched+never")

		// Config:
		//   global: retention_base=added, movie_retention=30d (all recently added → none overdue)
		//   tag rule: tag=oxitest, retention=90d, retention_base=last_watched, unwatched_behavior=never
		// Expected:
		//   Non-tagged movies → not scheduled (added recently, retention_base=added)
		//   Tagged+unwatched movie → protected by tag rule (unwatched_behavior=never)
		//   Tagged+watched movie (90d ago) → scheduled by tag rule
		// Since our test movies don't have the "oxitest" tag, none should be scheduled.

		UpdateConfigForRetentionBaseTagRuleTest(t, absConfigPath, jellystatURL)
		RestartOxiCleanarr(t, absComposeFile)
		client.Authenticate(AdminUsername, AdminPassword)

		// GetJobScheduledCount triggers sync internally and reads scheduled_deletions from job summary
		scheduledCount := client.GetJobScheduledCount()
		t.Logf("Scheduled items with per-rule tag override: %d", scheduledCount)

		// With global retention_base=added and all movies recently added, non-tagged movies
		// should not be scheduled. The tag rule only applies to tagged media.
		// Since our test movies don't have the "oxitest" tag, none should be scheduled.
		require.Equal(t, 0, scheduledCount,
			"No movies should be scheduled: global added mode (recently added) + no oxitest-tagged movies")
	})

	// ── Scenario 6: Watch activity resets TTL ────────────────────────────────────
	t.Run("WatchActivity_ResetsTTL", func(t *testing.T) {
		t.Logf("Scenario 6: watch activity resets TTL (core user story)")

		// Step 1: Set up with Pulp Fiction watched 60d ago → overdue with 30d retention.
		// NOTE: overdue items are not in /api/media/leaving-soon; use job summary instead.
		UpdateConfigForRetentionBaseTest(t, absConfigPath, jellystatURL, "", "", "")
		RestartOxiCleanarr(t, absComposeFile)
		client.Authenticate(AdminUsername, AdminPassword)

		// GetJobScheduledCount triggers sync and reads scheduled_deletions from job summary
		initialScheduled := client.GetJobScheduledCount()
		t.Logf("Step 1 — Initial scheduled count (Pulp Fiction 60d ago): %d", initialScheduled)
		require.Greater(t, initialScheduled, 0, "Expected at least one overdue item before TTL reset")

		// Step 2: Update mock to show Pulp Fiction watched 1 day ago (simulates re-watch)
		mockJellystat.SetWatchTimestamp("Pulp Fiction", time.Now().Add(-1*24*time.Hour))
		t.Logf("Step 2 — Updated Pulp Fiction last watched to 1 day ago")

		// Step 3: Trigger sync to pick up new watch data; read job summary
		afterResetItems := client.GetJobWouldDelete()
		afterResetScheduled := len(afterResetItems)
		t.Logf("Step 3 — Scheduled count after TTL reset: %d", afterResetScheduled)

		// Pulp Fiction should no longer be in scheduled deletions
		for _, item := range afterResetItems {
			title, _ := item["title"].(string)
			require.NotEqual(t, "Pulp Fiction", title,
				"Pulp Fiction must not be scheduled after re-watch 1 day ago with 30d retention")
		}

		// Reset mock back to original timestamps for cleanup
		mockJellystat.SetWatchTimestamp("Pulp Fiction", time.Now().Add(-60*24*time.Hour))
	})

	// ── Cleanup ──────────────────────────────────────────────────────────────────
	t.Logf("Cleaning up retention base lifecycle test...")
	RemoveAdvancedRules(t, absConfigPath)
	RestoreRetentionBaseConfig(t, absConfigPath)
	RestoreJellystatConfig(t, absConfigPath)
	RestartOxiCleanarr(t, absComposeFile)

	cleanupClient := NewTestClient(t, OxiCleanarrURL)
	cleanupClient.Authenticate(AdminUsername, AdminPassword)
	cleanupClient.TriggerSync()
	t.Logf("Re-synced after cleanup")

	t.Logf("=== Retention Base Lifecycle Test Complete ===")
}

// ── Config helper functions ───────────────────────────────────────────────────

// UpdateConfigForRetentionBaseTest sets retention_base, unwatched_behavior, and
// unwatched_retention on the global rules section, and enables the mock Jellystat.
// Pass empty string to omit a field (server will use its default).
func UpdateConfigForRetentionBaseTest(t *testing.T, configPath, jellystatURL, retentionBase, unwatchedBehavior, unwatchedRetention string) {
	t.Helper()
	t.Logf("Updating config: retention_base=%q unwatched_behavior=%q unwatched_retention=%q",
		retentionBase, unwatchedBehavior, unwatchedRetention)

	content, err := os.ReadFile(configPath)
	require.NoError(t, err)

	var cfg map[string]interface{}
	err = yaml.Unmarshal(content, &cfg)
	require.NoError(t, err)

	// Enable mock Jellystat
	if jellystatURL != "" {
		integrations, ok := cfg["integrations"].(map[string]interface{})
		require.True(t, ok, "integrations section not found")
		jellystat, ok := integrations["jellystat"].(map[string]interface{})
		if !ok {
			jellystat = make(map[string]interface{})
			integrations["jellystat"] = jellystat
		}
		dockerURL := convertMockURLForDocker(jellystatURL)
		jellystat["enabled"] = true
		jellystat["url"] = dockerURL
	}

	// Set retention_base fields on rules section
	rules, ok := cfg["rules"].(map[string]interface{})
	require.True(t, ok, "rules section not found")

	// Always set a short movie_retention so overdue items appear quickly
	rules["movie_retention"] = "30d"
	rules["tv_retention"] = "60d"

	if retentionBase != "" {
		rules["retention_base"] = retentionBase
	} else {
		delete(rules, "retention_base")
	}
	if unwatchedBehavior != "" {
		rules["unwatched_behavior"] = unwatchedBehavior
	} else {
		delete(rules, "unwatched_behavior")
	}
	if unwatchedRetention != "" {
		rules["unwatched_retention"] = unwatchedRetention
	} else {
		delete(rules, "unwatched_retention")
	}

	newContent, err := yaml.Marshal(cfg)
	require.NoError(t, err)
	err = os.WriteFile(configPath, newContent, 0644)
	require.NoError(t, err)

	t.Logf("Config updated for retention base test")
}

// UpdateConfigForRetentionBaseTagRuleTest sets a global retention_base=added config
// plus a tag-based advanced rule with retention_base=last_watched, unwatched_behavior=never.
func UpdateConfigForRetentionBaseTagRuleTest(t *testing.T, configPath, jellystatURL string) {
	t.Helper()
	t.Logf("Updating config for per-rule tag retention_base override test")

	content, err := os.ReadFile(configPath)
	require.NoError(t, err)

	var cfg map[string]interface{}
	err = yaml.Unmarshal(content, &cfg)
	require.NoError(t, err)

	// Enable mock Jellystat
	integrations, ok := cfg["integrations"].(map[string]interface{})
	require.True(t, ok, "integrations section not found")
	jellystat, ok := integrations["jellystat"].(map[string]interface{})
	if !ok {
		jellystat = make(map[string]interface{})
		integrations["jellystat"] = jellystat
	}
	dockerURL := convertMockURLForDocker(jellystatURL)
	jellystat["enabled"] = true
	jellystat["url"] = dockerURL

	// Global: retention_base=added (pure age-based)
	rules, ok := cfg["rules"].(map[string]interface{})
	require.True(t, ok, "rules section not found")
	rules["movie_retention"] = "30d"
	rules["tv_retention"] = "60d"
	rules["retention_base"] = "added"
	delete(rules, "unwatched_behavior")
	delete(rules, "unwatched_retention")

	// Tag rule: oxitest tag → last_watched + never (per-rule override)
	cfg["advanced_rules"] = []map[string]interface{}{
		{
			"name":               "oxitest-premium",
			"type":               "tag",
			"enabled":            true,
			"tag":                "oxitest",
			"retention":          "90d",
			"retention_base":     "last_watched",
			"unwatched_behavior": "never",
		},
	}

	newContent, err := yaml.Marshal(cfg)
	require.NoError(t, err)
	err = os.WriteFile(configPath, newContent, 0644)
	require.NoError(t, err)

	t.Logf("Config updated for per-rule tag retention_base override test")
}

// RestoreRetentionBaseConfig removes retention_base, unwatched_behavior, and
// unwatched_retention from the rules section, restoring default behaviour.
func RestoreRetentionBaseConfig(t *testing.T, configPath string) {
	t.Helper()
	t.Logf("Restoring retention_base config to defaults")

	content, err := os.ReadFile(configPath)
	require.NoError(t, err)

	var cfg map[string]interface{}
	err = yaml.Unmarshal(content, &cfg)
	require.NoError(t, err)

	rules, ok := cfg["rules"].(map[string]interface{})
	if ok {
		delete(rules, "retention_base")
		delete(rules, "unwatched_behavior")
		delete(rules, "unwatched_retention")
		// Restore default retention periods
		rules["movie_retention"] = "90d"
		rules["tv_retention"] = "120d"
	}

	newContent, err := yaml.Marshal(cfg)
	require.NoError(t, err)
	err = os.WriteFile(configPath, newContent, 0644)
	require.NoError(t, err)

	t.Logf("Retention_base config restored to defaults")
}

// ── TestClient helpers ────────────────────────────────────────────────────────

// GetScheduledItems returns the full list of items scheduled for deletion.
func (tc *TestClient) GetScheduledItems() []map[string]interface{} {
	resp, err := tc.Get("/api/media/leaving-soon")
	require.NoError(tc.t, err)
	defer resp.Body.Close()

	require.Equal(tc.t, 200, resp.StatusCode)

	var result struct {
		Items []map[string]interface{} `json:"items"`
	}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(tc.t, err)

	return result.Items
}
