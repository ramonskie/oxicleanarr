package integration

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const (
	ConfigPath     = "../assets/config/config.yaml"
	SymlinkDir     = "../assets/leaving-soon"
	ComposeFile    = "../assets/docker-compose.yml"
	Phase1Expected = 7 // Expected symlinks with 7d retention
)

// TestSymlinkLifecycle tests the complete symlink library lifecycle
// Phase 1: Create symlinks with 7d retention
// Phase 2: Cleanup symlinks with 0d retention
func TestSymlinkLifecycle(t *testing.T) {
	// Validate paths exist
	absConfigPath, err := filepath.Abs(ConfigPath)
	require.NoError(t, err)
	require.FileExists(t, absConfigPath, "Config file not found")

	absComposeFile, err := filepath.Abs(ComposeFile)
	require.NoError(t, err)
	require.FileExists(t, absComposeFile, "Docker compose file not found")

	absSymlinkDir, err := filepath.Abs(SymlinkDir)
	require.NoError(t, err)

	t.Logf("Config path: %s", absConfigPath)
	t.Logf("Compose file: %s", absComposeFile)
	t.Logf("Symlink directory: %s", absSymlinkDir)

	// Cleanup test environment (stop containers, delete databases, restart)
	t.Logf("Cleaning up test environment...")
	err = CleanupTestEnvironment(t, absComposeFile)
	require.NoError(t, err, "Failed to clean test environment")

	// Wait for services to be ready after cleanup
	t.Logf("Waiting for services to start...")
	time.Sleep(10 * time.Second)

	// Setup Jellyfin (automated initialization)
	t.Logf("Setting up Jellyfin...")
	userID, jellyfinAPIKey, err := SetupJellyfinForTest(t, JellyfinURL, AdminUsername, AdminPassword, absComposeFile)
	require.NoError(t, err, "Failed to setup Jellyfin")
	t.Logf("Jellyfin setup complete - UserID: %s, API Key: %s", userID, jellyfinAPIKey[:8]+"...")

	// Ensure Jellyfin has a movie library for testing
	t.Logf("Ensuring Jellyfin movie library exists...")
	err = EnsureJellyfinLibrary(t, JellyfinURL, jellyfinAPIKey, "Movies", "/media/movies", "movies")
	require.NoError(t, err, "Failed to ensure Jellyfin movie library")

	// Read Radarr API key from container (auto-generated on first run)
	t.Logf("Reading Radarr API key from container...")
	radarrAPIKey, err := GetRadarrAPIKeyFromContainer(t, "oxicleanarr-test-radarr")
	require.NoError(t, err, "Failed to read Radarr API key from container")
	t.Logf("Radarr API key: %s...", radarrAPIKey[:8])

	// Wait for Radarr to be fully initialized
	t.Logf("Waiting for Radarr to be ready...")
	err = WaitForRadarr(t, RadarrURL, radarrAPIKey)
	require.NoError(t, err, "Failed to wait for Radarr")
	t.Logf("Radarr is ready")

	// Setup Radarr with test movies
	t.Logf("Setting up Radarr with test movies...")
	err = EnsureRadarrMoviesExist(t, RadarrURL, radarrAPIKey)
	require.NoError(t, err, "Failed to setup Radarr")
	t.Logf("Radarr setup complete")

	// Get Jellyfin library ID for movie library
	t.Logf("Getting Jellyfin library ID...")
	libraryID, err := GetJellyfinLibraryID(t, JellyfinURL, jellyfinAPIKey, "Movies")
	require.NoError(t, err, "Failed to get Jellyfin library ID")
	t.Logf("Jellyfin library ID: %s", libraryID)

	// Trigger Jellyfin library scan to import movies from Radarr
	t.Logf("Triggering Jellyfin library scan to import movies from Radarr...")
	err = TriggerJellyfinLibraryScan(t, JellyfinURL, jellyfinAPIKey, libraryID)
	require.NoError(t, err, "Failed to trigger Jellyfin library scan")

	// Wait for Jellyfin to match all 7 movies
	t.Logf("Waiting for Jellyfin to match all 7 movies...")
	err = WaitForJellyfinMovies(t, JellyfinURL, jellyfinAPIKey, 7, libraryID)
	require.NoError(t, err, "Failed to wait for Jellyfin movie matching")
	t.Logf("Jellyfin has successfully matched all 7 movies")

	// Update OxiCleanarr config with API keys
	t.Logf("Updating OxiCleanarr config with API keys...")
	UpdateConfigAPIKeys(t, absConfigPath, jellyfinAPIKey, radarrAPIKey)
	t.Logf("Config updated with API keys")

	// Create test client
	client := NewTestClient(t, OxiCleanarrURL)

	// Authenticate
	client.Authenticate(AdminUsername, AdminPassword)

	// Get hide_when_empty setting
	hideWhenEmpty := GetHideWhenEmpty(t, absConfigPath)
	t.Logf("hide_when_empty setting: %v", hideWhenEmpty)

	// Phase 1: Create Symlinks (7d retention)
	t.Run("Phase1_CreateSymlinks", func(t *testing.T) {
		t.Logf("=== Phase 1: Creating Symlinks (7d retention) ===")

		// Step 1: Set dry_run to false to enable actual symlink creation
		UpdateDryRun(t, absConfigPath, false)

		// Step 2: Update retention policy to 7d
		UpdateRetentionPolicy(t, absConfigPath, "7d")

		// Step 3: Restart OxiCleanarr to reload config
		RestartOxiCleanarr(t, absComposeFile)

		// Step 4: Re-authenticate after restart
		client.Authenticate(AdminUsername, AdminPassword)

		// Step 5: Trigger full sync to populate library
		client.TriggerSync()

		// Step 6: Verify movie library is populated
		movieCount := client.GetMovieCount()
		t.Logf("Total movies in library: %d", movieCount)
		require.Greater(t, movieCount, 0, "No movies found in library")

		// Step 7: Verify scheduled deletions exist
		scheduledCount := client.GetScheduledCount()
		t.Logf("Items scheduled for deletion: %d", scheduledCount)
		require.Greater(t, scheduledCount, 0, "Expected items scheduled for deletion with 7d retention")

		// Step 8: Wait for symlink creation to complete
		t.Logf("Waiting 5 seconds for symlink creation...")
		time.Sleep(5 * time.Second)

		// Step 9: Verify symlinks were created
		t.Logf("Checking symlinks in: %s", absSymlinkDir)
		CheckSymlinks(t, absSymlinkDir, Phase1Expected)

		// Step 10: Verify Jellyfin library was created
		CheckJellyfinLibrary(t, jellyfinAPIKey, true)

		t.Logf("=== Phase 1 Complete ===")
	})

	// Phase 2: Cleanup Symlinks (0d retention)
	t.Run("Phase2_CleanupSymlinks", func(t *testing.T) {
		t.Logf("=== Phase 2: Cleaning Up Symlinks (0d retention) ===")

		// Step 1: Update retention policy to 0d (no retention)
		UpdateRetentionPolicy(t, absConfigPath, "0d")

		// Step 1.5: Wait for filesystem to flush the write
		t.Logf("Waiting 2 seconds for filesystem sync...")
		time.Sleep(2 * time.Second)

		// Step 2: Restart OxiCleanarr to reload config
		RestartOxiCleanarr(t, absComposeFile)

		// Step 3: Re-authenticate after restart
		client.Authenticate(AdminUsername, AdminPassword)

		// Step 4: Trigger full sync
		client.TriggerSync()

		// Step 5: Verify no items scheduled for deletion
		scheduledCount := client.GetScheduledCount()
		t.Logf("Items scheduled for deletion: %d", scheduledCount)
		require.Equal(t, 0, scheduledCount, "Expected 0 items scheduled with 0d retention")

		// Step 6: Wait for cleanup to complete
		t.Logf("Waiting 5 seconds for cleanup...")
		time.Sleep(5 * time.Second)

		// Step 7: Verify symlinks were removed
		t.Logf("Checking symlinks in: %s", absSymlinkDir)
		CheckSymlinks(t, absSymlinkDir, 0)

		// Step 8: Verify Jellyfin library state based on hide_when_empty
		if hideWhenEmpty {
			t.Logf("Expecting library to be deleted (hide_when_empty: true)")
			CheckJellyfinLibrary(t, jellyfinAPIKey, false)
		} else {
			t.Logf("Expecting library to still exist (hide_when_empty: false)")
			CheckJellyfinLibrary(t, jellyfinAPIKey, true)
		}

		t.Logf("=== Phase 2 Complete ===")
	})

	// Cleanup: Restore original retention (optional)
	t.Cleanup(func() {
		t.Logf("Test cleanup: Restoring retention to 7d")
		UpdateRetentionPolicy(t, absConfigPath, "7d")
	})
}
