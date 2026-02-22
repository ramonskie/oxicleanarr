package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// testEpisodeCleanupLifecycle tests episode-level cleanup rules end-to-end
// against a real OxiCleanarr instance backed by a real Sonarr container.
//
// TV shows available (from sonarr_setup_test.go):
//   - Breaking Bad    — 3 seasons × 5 episodes = 15 total (ended)
//   - The Daily Show  — 1 season  × 15 episodes            (continuing)
//   - Game of Thrones — 2 seasons × 3 episodes = 6 total  (ended)
func testEpisodeCleanupLifecycle(t *testing.T) {
	t.Logf("=== Episode Cleanup Lifecycle Test ===")

	absConfigPath, err := filepath.Abs(ConfigPath)
	require.NoError(t, err)
	absComposeFile, err := filepath.Abs(ComposeFile)
	require.NoError(t, err)

	sonarrAPIKey := GetSonarrAPIKeyFromConfig(t, absConfigPath)

	client := NewTestClient(t, OxiCleanarrURL)
	client.Authenticate(AdminUsername, AdminPassword)

	// ── Scenario 1: oldest_first — keep last 10, delete 5 oldest ────────────
	t.Run("OldestFirst_DeletesExcessEpisodes", func(t *testing.T) {
		// Breaking Bad has 15 episodes across 3 seasons.
		// Rule: max_episodes=10, strategy=oldest_first
		// Expected: 5 oldest episode files deleted, show itself NOT deleted.

		UpdateConfigForEpisodeCleanupTest(t, absConfigPath, map[string]interface{}{
			"name":                    "Breaking Bad Rolling Window",
			"type":                    "episode",
			"enabled":                 true,
			"tag":                     "episode-test",
			"max_episodes":            10,
			"episode_delete_strategy": "oldest_first",
		})
		// Tag Breaking Bad in Sonarr so the rule matches
		TagSonarrSeries(t, SonarrURL, sonarrAPIKey, "Breaking Bad", "episode-test")
		RestartOxiCleanarr(t, absComposeFile)
		client.Authenticate(AdminUsername, AdminPassword)

		summary := client.TriggerSyncAndGetSummary()
		episodeFilesDeleted, _ := summary["episode_files_deleted"].(float64)

		require.Equal(t, float64(5), episodeFilesDeleted,
			"Expected 5 episode files deleted (15 total - 10 kept)")

		// Verify the show itself is still in Sonarr
		series := GetSonarrSeriesByTitle(t, SonarrURL, sonarrAPIKey, "Breaking Bad")
		require.NotNil(t, series, "Breaking Bad should still exist in Sonarr after episode cleanup")
	})

	// ── Scenario 2: by_age — delete episodes older than max_age ─────────────
	t.Run("ByAge_DeletesOldEpisodes", func(t *testing.T) {
		// Game of Thrones has 6 episodes. Sonarr reports their air dates.
		// Rule: max_age=1s (effectively "delete all"), strategy=by_age
		// Expected: all 6 episode files deleted, show NOT deleted.

		UpdateConfigForEpisodeCleanupTest(t, absConfigPath, map[string]interface{}{
			"name":                    "Game of Thrones Age Cleanup",
			"type":                    "episode",
			"enabled":                 true,
			"tag":                     "episode-age-test",
			"max_age":                 "1s",
			"episode_delete_strategy": "by_age",
		})
		TagSonarrSeries(t, SonarrURL, sonarrAPIKey, "Game of Thrones", "episode-age-test")
		RestartOxiCleanarr(t, absComposeFile)
		client.Authenticate(AdminUsername, AdminPassword)

		summary := client.TriggerSyncAndGetSummary()
		episodeFilesDeleted, _ := summary["episode_files_deleted"].(float64)

		require.GreaterOrEqual(t, int(episodeFilesDeleted), 1,
			"Expected at least 1 episode file deleted by age rule")

		series := GetSonarrSeriesByTitle(t, SonarrURL, sonarrAPIKey, "Game of Thrones")
		require.NotNil(t, series, "Game of Thrones should still exist in Sonarr after episode cleanup")
	})

	// ── Scenario 3: by_season_age + keep_latest_season ───────────────────────
	t.Run("BySeasonAge_KeepsLatestSeason", func(t *testing.T) {
		// Breaking Bad has 3 seasons. Rule: max_age=1s, keep_latest_season=true
		// Expected: seasons 1 and 2 deleted, season 3 (latest) kept.

		UpdateConfigForEpisodeCleanupTest(t, absConfigPath, map[string]interface{}{
			"name":                    "Breaking Bad Season Cleanup",
			"type":                    "episode",
			"enabled":                 true,
			"tag":                     "season-age-test",
			"max_age":                 "1s",
			"episode_delete_strategy": "by_season_age",
			"keep_latest_season":      true,
		})
		TagSonarrSeries(t, SonarrURL, sonarrAPIKey, "Breaking Bad", "season-age-test")
		RestartOxiCleanarr(t, absComposeFile)
		client.Authenticate(AdminUsername, AdminPassword)

		summary := client.TriggerSyncAndGetSummary()
		episodeFilesDeleted, _ := summary["episode_files_deleted"].(float64)

		// Seasons 1+2 = 10 episodes deleted; season 3 (5 episodes) kept
		require.Equal(t, float64(10), episodeFilesDeleted,
			"Expected 10 episode files deleted (seasons 1+2), season 3 kept")
	})

	// ── Scenario 4: exclude_continuing_series ────────────────────────────────
	t.Run("ExcludeContinuingSeries_SkipsOngoingShow", func(t *testing.T) {
		// The Daily Show is "continuing". Rule has exclude_continuing_series=true.
		// Expected: 0 episode files deleted (show is protected).

		UpdateConfigForEpisodeCleanupTest(t, absConfigPath, map[string]interface{}{
			"name":                      "Daily Show Cleanup",
			"type":                      "episode",
			"enabled":                   true,
			"tag":                       "daily-show-test",
			"max_episodes":              5,
			"episode_delete_strategy":   "oldest_first",
			"exclude_continuing_series": true,
		})
		TagSonarrSeries(t, SonarrURL, sonarrAPIKey, "The Daily Show", "daily-show-test")
		RestartOxiCleanarr(t, absComposeFile)
		client.Authenticate(AdminUsername, AdminPassword)

		summary := client.TriggerSyncAndGetSummary()
		episodeFilesDeleted, _ := summary["episode_files_deleted"].(float64)

		require.Equal(t, float64(0), episodeFilesDeleted,
			"Expected 0 deletions: The Daily Show is continuing and exclude_continuing_series=true")
	})

	// ── Cleanup ──────────────────────────────────────────────────────────────
	t.Logf("Cleaning up episode cleanup lifecycle test...")
	RemoveAdvancedRules(t, absConfigPath)
	// Reset enable_deletion to false so subsequent tests start from a known safe state.
	// UpdateConfigForEpisodeCleanupTest sets enable_deletion=true; we must undo that here.
	UpdateEnableDeletion(t, absConfigPath, false)
	// RestoreSonarrConfig intentionally NOT called — Sonarr stays enabled.
	// The base config has sonarr.enabled=true; disabling it after the test
	// breaks subsequent test runs that share the same infrastructure.
	RestartOxiCleanarr(t, absComposeFile)

	cleanupClient := NewTestClient(t, OxiCleanarrURL)
	cleanupClient.Authenticate(AdminUsername, AdminPassword)
	cleanupClient.TriggerSync()

	t.Logf("=== Episode Cleanup Lifecycle Test Complete ===")
}

// UpdateConfigForEpisodeCleanupTest writes a single episode advanced rule to config
// and enables the Sonarr integration pointing at the test container.
func UpdateConfigForEpisodeCleanupTest(t *testing.T, configPath string, rule map[string]interface{}) {
	t.Helper()
	content, err := os.ReadFile(configPath)
	require.NoError(t, err)

	var cfg map[string]interface{}
	require.NoError(t, yaml.Unmarshal(content, &cfg))

	// Ensure Sonarr integration is enabled (already wired by infrastructure setup)
	integrations, _ := cfg["integrations"].(map[string]interface{})
	sonarr, _ := integrations["sonarr"].(map[string]interface{})
	sonarr["enabled"] = true

	// Ensure deletion is active so episode files are actually removed
	app, _ := cfg["app"].(map[string]interface{})
	app["enable_deletion"] = true
	app["dry_run"] = false

	cfg["advanced_rules"] = []map[string]interface{}{rule}

	newContent, err := yaml.Marshal(cfg)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(configPath, newContent, 0644))
	t.Logf("Config updated for episode cleanup test: rule=%s", rule["name"])
}

// RestoreSonarrConfig is intentionally a no-op.
// The base config has sonarr.enabled=true and that must remain so for the
// rest of the test suite. This function is kept for call-site compatibility
// but performs no writes.
func RestoreSonarrConfig(t *testing.T, configPath string) {
	t.Helper()
	t.Logf("RestoreSonarrConfig: no-op — Sonarr stays enabled per base config")
}

// TagSonarrSeries creates a tag in Sonarr (if it doesn't exist) and applies it
// to the named series. Mirrors CreateRadarrTag + TagRadarrMovie in helpers.go.
func TagSonarrSeries(t *testing.T, sonarrURL, apiKey, seriesTitle, tagLabel string) {
	t.Helper()
	t.Logf("Tagging Sonarr series %q with tag %q", seriesTitle, tagLabel)

	httpClient := &http.Client{Timeout: 10 * time.Second}

	// Create (or retrieve) the tag
	tagID := createOrGetSonarrTag(t, httpClient, sonarrURL, apiKey, tagLabel)

	// Find the series by title
	series := GetSonarrSeriesByTitle(t, sonarrURL, apiKey, seriesTitle)
	require.NotNil(t, series)

	// Fetch the full series object so we can PUT it back with the tag appended
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/api/v3/series/%d", sonarrURL, series.ID), nil)
	require.NoError(t, err)
	req.Header.Set("X-Api-Key", apiKey)

	resp, err := httpClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var fullSeries map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&fullSeries))

	// Append tag ID (avoid duplicates)
	tags, _ := fullSeries["tags"].([]interface{})
	alreadyTagged := false
	for _, v := range tags {
		if int(v.(float64)) == tagID {
			alreadyTagged = true
			break
		}
	}
	if !alreadyTagged {
		tags = append(tags, float64(tagID))
	}
	fullSeries["tags"] = tags

	// PUT the updated series back
	jsonData, err := json.Marshal(fullSeries)
	require.NoError(t, err)

	req, err = http.NewRequest(http.MethodPut, fmt.Sprintf("%s/api/v3/series/%d", sonarrURL, series.ID), bytes.NewBuffer(jsonData))
	require.NoError(t, err)
	req.Header.Set("X-Api-Key", apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err = httpClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.True(t, resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusAccepted,
		"Failed to tag Sonarr series (status %d)", resp.StatusCode)

	t.Logf("Tagged Sonarr series %q (ID %d) with tag %q (ID %d)", seriesTitle, series.ID, tagLabel, tagID)
}

// createOrGetSonarrTag creates a tag in Sonarr and returns its ID.
// If the tag already exists it returns the existing ID.
func createOrGetSonarrTag(t *testing.T, httpClient *http.Client, sonarrURL, apiKey, tagLabel string) int {
	t.Helper()

	tagData := map[string]string{"label": tagLabel}
	jsonData, err := json.Marshal(tagData)
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, sonarrURL+"/api/v3/tag", bytes.NewBuffer(jsonData))
	require.NoError(t, err)
	req.Header.Set("X-Api-Key", apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// 201 Created — new tag
	if resp.StatusCode == http.StatusCreated {
		var tag struct {
			ID    int    `json:"id"`
			Label string `json:"label"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&tag))
		t.Logf("Created Sonarr tag %q (ID %d)", tagLabel, tag.ID)
		return tag.ID
	}

	// 400 / 409 — tag may already exist; fetch the list and find it
	req2, err := http.NewRequest(http.MethodGet, sonarrURL+"/api/v3/tag", nil)
	require.NoError(t, err)
	req2.Header.Set("X-Api-Key", apiKey)

	resp2, err := httpClient.Do(req2)
	require.NoError(t, err)
	defer resp2.Body.Close()
	require.Equal(t, http.StatusOK, resp2.StatusCode)

	var tags []struct {
		ID    int    `json:"id"`
		Label string `json:"label"`
	}
	require.NoError(t, json.NewDecoder(resp2.Body).Decode(&tags))

	for _, tag := range tags {
		if strings.EqualFold(tag.Label, tagLabel) {
			t.Logf("Reusing existing Sonarr tag %q (ID %d)", tagLabel, tag.ID)
			return tag.ID
		}
	}

	require.Failf(t, "Sonarr tag not found", "Could not create or find tag %q", tagLabel)
	return 0
}

// GetSonarrAPIKeyFromConfig reads the Sonarr API key from the OxiCleanarr config file.
// Mirrors GetRadarrAPIKeyFromYAMLConfig in helpers.go.
func GetSonarrAPIKeyFromConfig(t *testing.T, configPath string) string {
	t.Helper()

	content, err := os.ReadFile(configPath)
	require.NoError(t, err)

	lines := strings.Split(string(content), "\n")
	inSonarrSection := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "sonarr:") {
			inSonarrSection = true
			continue
		}

		if inSonarrSection && strings.HasPrefix(trimmed, "api_key:") {
			value := strings.TrimPrefix(trimmed, "api_key:")
			value = strings.TrimSpace(value)

			// Remove surrounding quotes if present
			if strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") {
				value = strings.Trim(value, "\"")
			}

			if value != "" {
				return value
			}
		}

		// Exit section on a new top-level key
		if inSonarrSection && strings.HasSuffix(trimmed, ":") && !strings.HasPrefix(line, " ") {
			break
		}
	}

	require.Failf(t, "API key extraction failed", "Failed to extract Sonarr API key from config at %s", configPath)
	return ""
}
