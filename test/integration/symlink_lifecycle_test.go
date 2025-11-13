package integration

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const (
	ConfigPath     = "../assets/config/config.yaml"
	SymlinkDir     = "/data/media/leaving-soon" // Container path where symlinks are created
	ComposeFile    = "../assets/docker-compose.yml"
	Phase1Expected = 7 // Expected symlinks with 7d retention
)

// TestSymlinkLifecycle tests the complete symlink library lifecycle
// Phase 1: Create symlinks with 7d retention
// Phase 2: Cleanup symlinks with 0d retention
// NOTE: This test assumes infrastructure is already running from TestInfrastructure
// Run TestInfrastructure first to set up the environment
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

	// NOTE: This test assumes infrastructure is already running from TestInfrastructureSetup
	// It does NOT tear down and rebuild - it uses the existing environment
	t.Logf("Assuming infrastructure already initialized by TestInfrastructureSetup")

	// Create test client
	client := NewTestClient(t, OxiCleanarrURL)

	// Authenticate
	t.Logf("Authenticating with OxiCleanarr...")
	client.Authenticate(AdminUsername, AdminPassword)
	t.Logf("Authentication successful")

	// Get hide_when_empty setting
	hideWhenEmpty := GetHideWhenEmpty(t, absConfigPath)
	t.Logf("hide_when_empty setting: %v", hideWhenEmpty)

	// Extract Jellyfin API key and library name for library verification
	jellyfinAPIKey := GetJellyfinAPIKey(t, absConfigPath)
	moviesLibraryName := GetMoviesLibraryName(t, absConfigPath)
	t.Logf("Movies library name: %s", moviesLibraryName)

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

		// Step 4a: Wait for config hot-reload to complete
		t.Logf("⏳ Waiting for config hot-reload to complete (movie_retention=7d)...")
		WaitForConfigValue(t, client, "rules.movie_retention", "7d")
		t.Logf("✅ Config hot-reload verified")

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

		// Step 9: Verify symlinks were created via Jellyfin plugin API
		t.Logf("Checking symlinks in: %s", SymlinkDir)
		CheckSymlinks(t, jellyfinAPIKey, SymlinkDir, Phase1Expected)

		// Step 10: Verify Jellyfin library was created
		CheckJellyfinLibrary(t, jellyfinAPIKey, moviesLibraryName, true)

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

		// Step 7: Verify symlinks were removed via Jellyfin plugin API
		t.Logf("Checking symlinks in: %s", SymlinkDir)
		CheckSymlinks(t, jellyfinAPIKey, SymlinkDir, 0)

		// Step 8: Verify Jellyfin library state based on hide_when_empty
		if hideWhenEmpty {
			t.Logf("Expecting library to be deleted (hide_when_empty: true)")
			CheckJellyfinLibrary(t, jellyfinAPIKey, moviesLibraryName, false)
		} else {
			t.Logf("Expecting library to still exist (hide_when_empty: false)")
			CheckJellyfinLibrary(t, jellyfinAPIKey, moviesLibraryName, true)
		}

		t.Logf("=== Phase 2 Complete ===")
	})

	// Cleanup: Restore original retention (optional)
	t.Cleanup(func() {
		t.Logf("Test cleanup: Restoring retention to 7d")
		UpdateRetentionPolicy(t, absConfigPath, "7d")
	})
}
