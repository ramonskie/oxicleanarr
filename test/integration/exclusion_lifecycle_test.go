package integration

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// testExclusionLifecycle tests the complete exclusion functionality lifecycle
// Sub-Phase 1: Create symlinks with 7d retention (reuse existing Phase 1 setup)
// Sub-Phase 2: Exclude a movie and verify symlink removal + metadata preservation
// Sub-Phase 3: Remove exclusion and verify symlink restoration
// NOTE: This test assumes infrastructure is already running from TestInfrastructureSetup
// This is called from TestIntegrationSuite, not run standalone
func testExclusionLifecycle(t *testing.T) {
	// Validate paths exist
	absConfigPath, err := filepath.Abs(ConfigPath)
	require.NoError(t, err)
	require.FileExists(t, absConfigPath, "Config file not found")

	absComposeFile, err := filepath.Abs(ComposeFile)
	require.NoError(t, err)
	require.FileExists(t, absComposeFile, "Docker compose file not found")

	t.Logf("Config path: %s", absConfigPath)
	t.Logf("Compose file: %s", absComposeFile)

	// NOTE: This test assumes infrastructure is already running from TestInfrastructureSetup
	t.Logf("Assuming infrastructure already initialized by TestInfrastructureSetup")

	// Create test client
	client := NewTestClient(t, OxiCleanarrURL)

	// Authenticate
	t.Logf("Authenticating with OxiCleanarr...")
	client.Authenticate(AdminUsername, AdminPassword)
	t.Logf("Authentication successful")

	// Extract Jellyfin API key and library name for library verification
	jellyfinAPIKey := GetJellyfinAPIKey(t, absConfigPath)
	moviesLibraryName := GetMoviesLibraryName(t, absConfigPath)
	t.Logf("Movies library name: %s", moviesLibraryName)

	// Sub-Phase 1: Initial Setup - Create Symlinks (7d retention)
	t.Run("SubPhase1_InitialSetup", func(t *testing.T) {
		t.Logf("=== Sub-Phase 1: Initial Setup (7d retention) ===")

		// Step 1: Set dry_run to false to enable actual symlink creation
		UpdateDryRun(t, absConfigPath, false)

		// Step 2: Update retention policy to 7d
		UpdateRetentionPolicy(t, absConfigPath, "7d")

		// Step 3: Restart OxiCleanarr to reload config
		RestartOxiCleanarr(t, absComposeFile)

		// Step 4: Re-authenticate after restart
		client.Authenticate(AdminUsername, AdminPassword)

		// Step 5: Wait for config hot-reload to complete
		t.Logf("⏳ Waiting for config hot-reload to complete (movie_retention=7d)...")
		WaitForConfigValue(t, client, "rules.movie_retention", "7d")
		t.Logf("✅ Config hot-reload verified")

		// Step 6: Trigger full sync to populate library
		client.TriggerSync()

		// Step 7: Verify movie library is populated
		movieCount := client.GetMovieCount()
		t.Logf("Total movies in library: %d", movieCount)
		require.Greater(t, movieCount, 0, "No movies found in library")

		// Step 8: Verify scheduled deletions exist
		scheduledCount := client.GetScheduledCount()
		t.Logf("Items scheduled for deletion: %d", scheduledCount)
		require.Equal(t, Phase1Expected, scheduledCount, "Expected %d items scheduled for deletion with 7d retention", Phase1Expected)

		// Step 9: Wait for symlink creation to complete
		t.Logf("Waiting 5 seconds for symlink creation...")
		time.Sleep(5 * time.Second)

		// Step 10: Verify symlinks were created via Jellyfin plugin API
		t.Logf("Checking symlinks in: %s", SymlinkDir)
		CheckSymlinks(t, jellyfinAPIKey, SymlinkDir, Phase1Expected)

		// Step 11: Verify Jellyfin library was created
		CheckJellyfinLibrary(t, jellyfinAPIKey, moviesLibraryName, true)

		t.Logf("=== Sub-Phase 1 Complete: %d symlinks created ===", Phase1Expected)
	})

	// Sub-Phase 2: Exclude Movie and Verify Symlink Removal
	t.Run("SubPhase2_ExcludeMovie", func(t *testing.T) {
		t.Logf("=== Sub-Phase 2: Exclude Movie + Verify ===")

		// Step 1: Get "Fight Club (1999)" media ID
		mediaID, err := client.GetMediaByTitle("Fight Club")
		require.NoError(t, err, "Failed to find 'Fight Club' in media library")
		t.Logf("Found Fight Club media ID: %s", mediaID)

		// Step 2: Exclude the movie with a reason
		err = client.ExcludeMedia(mediaID, "User favorite")
		require.NoError(t, err, "Failed to exclude Fight Club")

		// Step 3: Trigger full sync to update symlinks
		t.Logf("Triggering sync to update symlinks after exclusion...")
		client.TriggerSync()

		// Step 4: Verify Fight Club still exists in OxiCleanarr with excluded=true
		details, err := client.GetMediaDetails(mediaID)
		require.NoError(t, err, "Failed to get media details")

		isExcluded, ok := details["excluded"].(bool)
		require.True(t, ok, "excluded field not found or wrong type")
		require.True(t, isExcluded, "Fight Club should be marked as excluded")
		t.Logf("✅ Fight Club is marked as excluded in OxiCleanarr")

		// Step 5: Verify scheduled deletions count (should be 6, excluding Fight Club)
		scheduledCount := client.GetScheduledCount()
		t.Logf("Items scheduled for deletion: %d", scheduledCount)
		expectedScheduled := Phase1Expected - 1
		require.Equal(t, expectedScheduled, scheduledCount, "Expected %d items scheduled (Fight Club excluded)", expectedScheduled)

		// Step 6: Wait for symlink update to complete
		t.Logf("Waiting 5 seconds for symlink removal...")
		time.Sleep(5 * time.Second)

		// Step 7: Verify only 6 symlinks remain (Fight Club removed) using plugin API
		expectedSymlinks := Phase1Expected - 1
		t.Logf("Checking symlinks via plugin API (expecting %d after exclusion)...", expectedSymlinks)
		CheckSymlinks(t, jellyfinAPIKey, SymlinkDir, expectedSymlinks)
		t.Logf("✅ Plugin API confirms %d symlinks (Fight Club excluded)", expectedSymlinks)

		t.Logf("=== Sub-Phase 2 Complete: Exclusion verified ===")
	})

	// Sub-Phase 3: Remove Exclusion and Verify Symlink Restoration
	t.Run("SubPhase3_RemoveExclusion", func(t *testing.T) {
		t.Logf("=== Sub-Phase 3: Remove Exclusion + Verify Restoration ===")

		// Step 1: Get Fight Club media ID again
		mediaID, err := client.GetMediaByTitle("Fight Club")
		require.NoError(t, err, "Failed to find 'Fight Club' in media library")

		// Step 2: Remove the exclusion
		err = client.RemoveExclusion(mediaID)
		require.NoError(t, err, "Failed to remove exclusion for Fight Club")

		// Step 3: Trigger full sync to restore symlink
		t.Logf("Triggering sync to restore symlink after removing exclusion...")
		client.TriggerSync()

		// Step 4: Verify Fight Club is no longer excluded
		details, err := client.GetMediaDetails(mediaID)
		require.NoError(t, err, "Failed to get media details")

		isExcluded, ok := details["excluded"].(bool)
		require.True(t, ok, "excluded field not found or wrong type")
		require.False(t, isExcluded, "Fight Club should no longer be excluded")
		t.Logf("✅ Fight Club exclusion removed in OxiCleanarr")

		// Step 5: Verify scheduled deletions count restored to 7
		scheduledCount := client.GetScheduledCount()
		t.Logf("Items scheduled for deletion: %d", scheduledCount)
		require.Equal(t, Phase1Expected, scheduledCount, "Expected %d items scheduled (all restored)", Phase1Expected)

		// Step 6: Wait for symlink restoration to complete
		t.Logf("Waiting 5 seconds for symlink restoration...")
		time.Sleep(5 * time.Second)

		// Step 7: Verify all 7 symlinks restored using plugin API
		t.Logf("Checking symlinks via plugin API (expecting %d after restoration)...", Phase1Expected)
		CheckSymlinks(t, jellyfinAPIKey, SymlinkDir, Phase1Expected)
		t.Logf("✅ Plugin API confirms %d symlinks (all restored including Fight Club)", Phase1Expected)

		t.Logf("=== Sub-Phase 3 Complete: Exclusion reversal verified ===")
	})

	// Cleanup: Restore original retention (optional)
	t.Cleanup(func() {
		t.Logf("Test cleanup: Restoring retention to 7d")
		UpdateRetentionPolicy(t, absConfigPath, "7d")
	})
}
