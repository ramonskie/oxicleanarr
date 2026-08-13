package integration

import (
	"encoding/json"
	"net/http"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// The Leaving Soon plugin (jellyfin-plugin-leaving-soon) pulls leaving-soon data
// from provider apps and manages the symlink libraries itself. This test exercises
// the full flow against the OxiCleanarr provider:
//
//	Phase 1: 7d retention -> OxiCleanarr exposes leaving-soon items -> plugin
//	         creates symlinks + a Jellyfin library.
//	Phase 2: 0d retention -> OxiCleanarr has nothing leaving -> plugin removes
//	         symlinks and the library (hide_when_empty + double refresh).
//
// NOTE: This test assumes infrastructure is already running from TestInfrastructureSetup.

// testLeavingSoonLifecycle tests the plugin-driven leaving-soon library lifecycle.
// This is called from TestIntegrationSuite, not run standalone.
func testLeavingSoonLifecycle(t *testing.T) {
	absConfigPath, err := filepath.Abs(ConfigPath)
	require.NoError(t, err)
	require.FileExists(t, absConfigPath, "Config file not found")

	absComposeFile, err := filepath.Abs(ComposeFile)
	require.NoError(t, err)
	require.FileExists(t, absComposeFile, "Docker compose file not found")

	t.Logf("Config path: %s", absConfigPath)
	t.Logf("Compose file: %s", absComposeFile)

	// Create test client and authenticate
	client := NewTestClient(t, OxiCleanarrURL)
	t.Logf("Authenticating with OxiCleanarr...")
	client.Authenticate(AdminUsername, AdminPassword)

	jellyfinAPIKey := GetJellyfinAPIKey(t, absConfigPath)

	// The Leaving Soon plugin is installed during infrastructure setup
	// (SetupJellyfinForTest). Just make sure Jellyfin has it loaded.
	require.Eventually(t, func() bool {
		req, err := http.NewRequest(http.MethodGet, JellyfinURL+"/api/leaving-soon/status", nil)
		if err != nil {
			return false
		}
		req.Header.Set("X-Emby-Token", jellyfinAPIKey)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return false
		}
		resp.Body.Close()
		return resp.StatusCode == http.StatusOK
	}, 60*time.Second, 2*time.Second, "Leaving Soon plugin status endpoint should be ready")

	// Phase 1: Create symlinks with 7d retention
	t.Run("Phase1_CreateSymlinks", func(t *testing.T) {
		t.Logf("=== Phase 1: Creating Symlinks (7d retention) ===")

		UpdateDryRun(t, absConfigPath, false)
		UpdateRetentionPolicyBoth(t, absConfigPath, "7d", "7d")
		RestartOxiCleanarr(t, absComposeFile)
		client.Authenticate(AdminUsername, AdminPassword)

		// Sync OxiCleanarr so it evaluates retention and produces leaving-soon items
		// for BOTH movies and TV shows. Poll with the static API key (like the plugin
		// does), re-syncing until enough items appear — Jellyfin matching is
		// eventually-consistent, and this mirrors the old WaitForSymlinkCount retry.
		var movieCount, showCount int
		deadline := time.Now().Add(90 * time.Second)
		for {
			client.TriggerSync()
			time.Sleep(3 * time.Second)
			leavingSoon := GetLeavingSoonWithAPIKey(t, OxiCleanarrTestAPIKey)
			movieCount, showCount = 0, 0
			for _, item := range leavingSoon {
				if item["type"] == "show" {
					showCount++
				} else {
					movieCount++
				}
			}
			if movieCount >= LeavingSoonMovieSymlinks && showCount >= 1 {
				break
			}
			if time.Now().After(deadline) {
				require.FailNowf(t, "Leaving-soon items timeout",
					"Expected at least %d movies and 1 show leaving soon, got %d movies / %d shows",
					LeavingSoonMovieSymlinks, movieCount, showCount)
			}
		}
		t.Logf("✅ OxiCleanarr reports %d movies and %d shows leaving soon", movieCount, showCount)

		// Trigger the plugin's manual sync endpoint so it polls OxiCleanarr.
		TriggerLeavingSoonPluginSync(t, jellyfinAPIKey)

		// Wait for symlinks to appear in the container's movies and tv dirs.
		WaitForContainerSymlinkCount(t, "oxicleanarr-test-jellyfin", LeavingSoonBasePath, "movies", movieCount, 120*time.Second)
		WaitForContainerSymlinkCount(t, "oxicleanarr-test-jellyfin", LeavingSoonBasePath, "tv", showCount, 120*time.Second)

		// Verify both Jellyfin libraries were created.
		CheckJellyfinLibrary(t, jellyfinAPIKey, LeavingSoonMoviesLibrary, true)
		CheckJellyfinLibrary(t, jellyfinAPIKey, LeavingSoonTVLibrary, true)

		t.Logf("=== Phase 1 Complete ===")
	})

	// Phase 1b: one item leaves the leaving-soon feed and the plugin must drop only
	// that item's symlink. The plugin's reconciliation is feed-driven — it removes
	// symlinks for anything no longer reported, whether the item was deleted by
	// OxiCleanarr, excluded, or had its retention extended. Exclusion is used here
	// because it is reversible and does not delete shared test media that later
	// tests depend on. This is the same code path a real deletion exercises.
	t.Run("Phase1b_ItemLeavesFeed", func(t *testing.T) {
		t.Logf("=== Phase 1b: Excluding one movie, plugin should drop its symlink ===")

		mediaID, err := client.GetMediaByTitle("Inception")
		require.NoError(t, err, "Inception should be present in the library")

		// Exclude -> the movie leaves the leaving-soon feed.
		require.NoError(t, client.ExcludeMedia(mediaID, "leaving-soon reconciliation test"))
		client.TriggerSync()

		leavingSoon := GetLeavingSoonWithAPIKey(t, OxiCleanarrTestAPIKey)
		movieCount := 0
		for _, item := range leavingSoon {
			if item["type"] == "movie" {
				movieCount++
			}
		}
		require.Equal(t, LeavingSoonMovieSymlinks-1, movieCount, "excluded movie should leave the leaving-soon feed")

		// Plugin sync should remove only the excluded movie's symlink.
		TriggerLeavingSoonPluginSync(t, jellyfinAPIKey)
		WaitForContainerSymlinkCount(t, "oxicleanarr-test-jellyfin", LeavingSoonBasePath, "movies", movieCount, 120*time.Second)

		// Un-exclude -> the movie returns; the next sync recreates its symlink.
		require.NoError(t, client.RemoveExclusion(mediaID))
		client.TriggerSync()
		TriggerLeavingSoonPluginSync(t, jellyfinAPIKey)
		WaitForContainerSymlinkCount(t, "oxicleanarr-test-jellyfin", LeavingSoonBasePath, "movies", LeavingSoonMovieSymlinks, 120*time.Second)

		t.Logf("=== Phase 1b Complete ===")
	})

	// Phase 2: Cleanup with 0d retention
	t.Run("Phase2_CleanupSymlinks", func(t *testing.T) {
		t.Logf("=== Phase 2: Cleaning Up Symlinks (0d retention) ===")

		UpdateRetentionPolicyBoth(t, absConfigPath, "0d", "0d")
		time.Sleep(2 * time.Second)
		RestartOxiCleanarr(t, absComposeFile)
		client.Authenticate(AdminUsername, AdminPassword)
		client.TriggerSync()

		// OxiCleanarr should now have nothing leaving soon (movies AND shows).
		scheduledCount := client.GetScheduledCount()
		require.Equal(t, 0, scheduledCount, "Expected 0 items scheduled with 0d retention")
		t.Logf("✅ 0 items scheduled with 0d retention")

		// Trigger the plugin sync; it should remove symlinks + both libraries.
		TriggerLeavingSoonPluginSync(t, jellyfinAPIKey)

		WaitForContainerSymlinkCount(t, "oxicleanarr-test-jellyfin", LeavingSoonBasePath, "movies", 0, 120*time.Second)
		WaitForContainerSymlinkCount(t, "oxicleanarr-test-jellyfin", LeavingSoonBasePath, "tv", 0, 120*time.Second)

		// hide_when_empty defaults true: the libraries are deleted and user views update.
		CheckJellyfinLibrary(t, jellyfinAPIKey, LeavingSoonMoviesLibrary, false)
		CheckJellyfinLibrary(t, jellyfinAPIKey, LeavingSoonTVLibrary, false)
		CheckJellyfinUserViews(t, JellyfinURL, jellyfinAPIKey, LeavingSoonMoviesLibrary, false)
		CheckJellyfinUserViews(t, JellyfinURL, jellyfinAPIKey, LeavingSoonTVLibrary, false)

		t.Logf("=== Phase 2 Complete ===")
	})

	// Cleanup: restore retention to the base values (movie 7d, TV 120d) so later
	// tests see only the 7 movies in the leaving-soon window.
	t.Cleanup(func() {
		t.Logf("Test cleanup: Restoring retention to movie=7d tv=120d")
		UpdateRetentionPolicyBoth(t, absConfigPath, "7d", "120d")
	})
}

// GetLeavingSoonWithAPIKey fetches the leaving-soon items using the static API key,
// exactly as jellyfin-plugin-leaving-soon polls the endpoint (no JWT involved).
func GetLeavingSoonWithAPIKey(t *testing.T, apiKey string) []map[string]interface{} {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, OxiCleanarrURL+"/api/media/leaving-soon", nil)
	require.NoError(t, err, "Failed to create leaving-soon request")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err, "Failed to query OxiCleanarr leaving-soon API")
	defer resp.Body.Close()
	require.Equal(t, 200, resp.StatusCode, "leaving-soon API should return 200 with API key")

	var payload struct {
		Items []map[string]interface{} `json:"items"`
	}
	err = json.NewDecoder(resp.Body).Decode(&payload)
	require.NoError(t, err, "Failed to parse leaving-soon response")
	return payload.Items
}

// TriggerLeavingSoonPluginSync calls the plugin's manual sync endpoint.
func TriggerLeavingSoonPluginSync(t *testing.T, jellyfinAPIKey string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, JellyfinURL+"/api/leaving-soon/sync", nil)
	require.NoError(t, err, "Failed to create plugin sync request")
	req.Header.Set("X-Emby-Token", jellyfinAPIKey)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err, "Failed to trigger plugin sync")
	defer resp.Body.Close()
	require.Equal(t, 202, resp.StatusCode, "Plugin sync should return 202 Accepted")
	t.Logf("✅ Plugin sync triggered")
}

// WaitForContainerSymlinkCount polls a directory inside the Jellyfin container until
// the number of symlink entries matches the expected count.
func WaitForContainerSymlinkCount(t *testing.T, container, basePath, subDir string, expectedCount int, maxWait time.Duration) {
	t.Helper()
	dir := filepath.Join(basePath, subDir)
	deadline := time.Now().Add(maxWait)
	for {
		count, err := ContainerSymlinkCount(container, dir)
		if err == nil && count == expectedCount {
			t.Logf("Symlink count correct in %s: %d (expected: %d)", dir, count, expectedCount)
			return
		}
		if time.Now().After(deadline) {
			lastCount := -1
			if err == nil {
				lastCount = count
			}
			require.Failf(t, "Symlink count timeout",
				"Expected %d symlinks in %s, got %d (err=%v)", expectedCount, dir, lastCount, err)
			return
		}
		time.Sleep(5 * time.Second)
	}
}

// ContainerSymlinkCount counts symlink entries in a directory inside a container.
func ContainerSymlinkCount(container, dir string) (int, error) {
	// docker exec ls -la; count lines whose permissions start with 'l'
	out, err := exec.Command("docker", "exec", container, "ls", "-la", dir).CombinedOutput()
	if err != nil {
		return 0, err
	}
	count := 0
	for _, line := range strings.Split(string(out), "\n") {
		if len(line) > 0 && line[0] == 'l' {
			count++
		}
	}
	return count, nil
}
