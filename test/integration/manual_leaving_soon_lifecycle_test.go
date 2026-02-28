package integration

import (
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// testManualLeavingSoonLifecycle tests the complete manual leaving soon lifecycle
// Sub-Phase 1: Initial Setup - verify 7d retention produces Phase1Expected scheduled items
// Sub-Phase 2: Flag a movie ("Fight Club") as manual leaving soon → verify it appears in leaving-soon list
// Sub-Phase 3: Conflict check — try to flag an excluded movie → expect 409 Conflict
// Sub-Phase 4: Remove manual flag → verify item returns to normal rule evaluation
// NOTE: This test assumes infrastructure is already running from TestInfrastructureSetup
// This is called from TestIntegrationSuite, not run standalone
func testManualLeavingSoonLifecycle(t *testing.T) {
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

	// Extract Jellyfin API key for symlink verification
	jellyfinAPIKey := GetJellyfinAPIKey(t, absConfigPath)

	// Sub-Phase 1: Initial Setup - verify baseline state with 7d retention
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
		t.Logf("⏳ Waiting for config hot-reload (movie_retention=7d)...")
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
		require.Equal(t, Phase1Expected, scheduledCount,
			"Expected %d items scheduled for deletion with 7d retention", Phase1Expected)

		t.Logf("=== Sub-Phase 1 Complete: baseline verified (%d scheduled) ===", Phase1Expected)
	})

	// Sub-Phase 2: Flag a movie as manual leaving soon and verify it appears in leaving-soon list
	t.Run("SubPhase2_FlagManualLeavingSoon", func(t *testing.T) {
		t.Logf("=== Sub-Phase 2: Flag Movie as Manual Leaving Soon ===")

		// Step 1: Use a very long retention so Fight Club is NOT in the leaving-soon window normally.
		// With "365d" retention only watched items with >365d ago would be scheduled.
		// All 7 test movies have add dates that already trigger 7d retention, so we need a
		// retention long enough that NONE are scheduled by the rule engine alone.
		// We use "3650d" (10 years) to ensure the leaving-soon window is empty initially.
		UpdateRetentionPolicy(t, absConfigPath, "3650d")
		RestartOxiCleanarr(t, absComposeFile)
		client.Authenticate(AdminUsername, AdminPassword)

		WaitForConfigValue(t, client, "rules.movie_retention", "3650d")
		t.Logf("✅ Retention set to 3650d (no items should be scheduled by rule engine)")

		// Step 2: Sync so the library is rebuilt with the new retention
		client.TriggerSync()

		scheduledBeforeFlag := client.GetScheduledCount()
		t.Logf("Scheduled count before flag (with 3650d retention): %d", scheduledBeforeFlag)

		// Step 3: Obtain Fight Club media ID
		mediaID, err := client.GetMediaByTitle("Fight Club")
		require.NoError(t, err, "Failed to find 'Fight Club' in media library")
		t.Logf("Found Fight Club media ID: %s", mediaID)

		// Step 4: Flag the movie as manual leaving soon
		err = client.AddManualLeavingSoon(mediaID)
		require.NoError(t, err, "Failed to flag Fight Club as manual leaving soon")
		t.Logf("✅ Fight Club flagged as manual leaving soon")

		// Step 5: Trigger sync so applyManualLeavingSoon() runs
		client.TriggerSync()

		// Step 6: Verify Fight Club is marked as manual_leaving_soon in media details
		details, err := client.GetMediaDetails(mediaID)
		require.NoError(t, err, "Failed to get media details")

		isManual, ok := details["manual_leaving_soon"].(bool)
		require.True(t, ok, "manual_leaving_soon field not found or wrong type in media details")
		require.True(t, isManual, "Fight Club should be marked as manual_leaving_soon")
		t.Logf("✅ Fight Club has manual_leaving_soon=true in media details")

		// Step 7: Verify Fight Club now appears in the leaving-soon list
		// (it should be the only item since the rule engine sees 3650d retention)
		scheduledAfterFlag := client.GetScheduledCount()
		t.Logf("Scheduled count after flag: %d", scheduledAfterFlag)
		require.Equal(t, scheduledBeforeFlag+1, scheduledAfterFlag,
			"Expected exactly 1 more item in leaving-soon after manually flagging Fight Club")

		// Step 8: Verify deletion_date is set to approximately now + leaving_soon_days
		// We just assert it is non-nil / non-zero; exact timing not enforced here.
		deleteAfter, hasDeleteAfter := details["deletion_date"]
		require.True(t, hasDeleteAfter && deleteAfter != nil && deleteAfter != "",
			"deletion_date should be set after manual leaving soon flag")
		t.Logf("✅ Fight Club deletion_date: %v", deleteAfter)

		t.Logf("=== Sub-Phase 2 Complete: manual leaving soon flag verified ===")
	})

	// Sub-Phase 3: Conflict check - excluded item cannot be flagged as manual leaving soon
	t.Run("SubPhase3_ConflictWithExclusion", func(t *testing.T) {
		t.Logf("=== Sub-Phase 3: Conflict Check (Excluded Item) ===")

		// Step 1: Reset to 7d retention for clarity
		UpdateRetentionPolicy(t, absConfigPath, "7d")
		RestartOxiCleanarr(t, absComposeFile)
		client.Authenticate(AdminUsername, AdminPassword)
		WaitForConfigValue(t, client, "rules.movie_retention", "7d")
		client.TriggerSync()

		// Step 2: Find a different movie to exclude (use "The Matrix" to avoid side effects on Fight Club)
		// The Matrix is in the test dataset
		var conflictMovieID string
		var conflictMovieErr error

		// Try The Matrix first, fall back to Fight Club if not found
		conflictMovieID, conflictMovieErr = client.GetMediaByTitle("The Matrix")
		if conflictMovieErr != nil {
			t.Logf("'The Matrix' not found, falling back to 'Fight Club'")
			conflictMovieID, conflictMovieErr = client.GetMediaByTitle("Fight Club")
			require.NoError(t, conflictMovieErr, "Could not find a movie for conflict test")
		}
		t.Logf("Using movie ID %s for conflict test", conflictMovieID)

		// Step 3: First, ensure it is not currently flagged as manual leaving soon
		_ = client.RemoveManualLeavingSoon(conflictMovieID) // ignore error if not flagged

		// Step 4: Exclude the movie
		err := client.ExcludeMedia(conflictMovieID, "Conflict test exclusion")
		require.NoError(t, err, "Failed to exclude movie for conflict test")
		t.Logf("✅ Movie excluded")

		// Step 5: Attempt to flag the excluded movie as manual leaving soon
		// Expect HTTP 409 Conflict
		resp, err := client.AddManualLeavingSoonRaw(conflictMovieID)
		require.NoError(t, err, "HTTP request itself should not fail")
		defer resp.Body.Close()

		require.Equal(t, http.StatusConflict, resp.StatusCode,
			"Flagging an excluded item as manual leaving soon should return 409 Conflict")
		t.Logf("✅ Got expected 409 Conflict when trying to flag an excluded item")

		// Step 6: Clean up - remove the exclusion
		err = client.RemoveExclusion(conflictMovieID)
		require.NoError(t, err, "Failed to remove test exclusion")
		t.Logf("✅ Test exclusion removed")

		t.Logf("=== Sub-Phase 3 Complete: Conflict check verified ===")
	})

	// Sub-Phase 4: Remove manual leaving soon flag and verify item returns to normal evaluation
	t.Run("SubPhase4_RemoveManualFlag", func(t *testing.T) {
		t.Logf("=== Sub-Phase 4: Remove Manual Flag + Verify Restoration ===")

		// Step 1: Use 3650d retention again so the rule engine does NOT schedule items
		UpdateRetentionPolicy(t, absConfigPath, "3650d")
		RestartOxiCleanarr(t, absComposeFile)
		client.Authenticate(AdminUsername, AdminPassword)
		WaitForConfigValue(t, client, "rules.movie_retention", "3650d")

		// Step 2: Trigger sync; with 3650d retention the leaving-soon list should be empty
		client.TriggerSync()
		scheduledBefore := client.GetScheduledCount()
		t.Logf("Scheduled count at start of Sub-Phase 4: %d", scheduledBefore)

		// Step 3: Get Fight Club media ID
		mediaID, err := client.GetMediaByTitle("Fight Club")
		require.NoError(t, err, "Failed to find 'Fight Club' in media library")

		// Step 4: Flag it as manual leaving soon
		err = client.AddManualLeavingSoon(mediaID)
		require.NoError(t, err, "Failed to flag Fight Club as manual leaving soon")

		// Step 5: Sync and verify it appears in the leaving-soon list
		client.TriggerSync()
		scheduledWithFlag := client.GetScheduledCount()
		t.Logf("Scheduled count after flagging: %d", scheduledWithFlag)
		require.Equal(t, scheduledBefore+1, scheduledWithFlag,
			"Fight Club should appear in leaving-soon after being manually flagged")

		// Step 6: Remove the manual leaving soon flag
		err = client.RemoveManualLeavingSoon(mediaID)
		require.NoError(t, err, "Failed to remove manual leaving soon flag from Fight Club")
		t.Logf("✅ Manual leaving soon flag removed")

		// Step 7: Trigger sync to rebuild library state
		t.Logf("Triggering sync after removing flag...")
		client.TriggerSync()

		// Step 8: Verify Fight Club is no longer marked as manual_leaving_soon
		details, err := client.GetMediaDetails(mediaID)
		require.NoError(t, err, "Failed to get media details")

		isManual, ok := details["manual_leaving_soon"].(bool)
		if ok {
			require.False(t, isManual, "Fight Club should no longer be manual_leaving_soon")
		}
		// If field is absent (false/omitted), that is also acceptable
		t.Logf("✅ Fight Club manual_leaving_soon flag is cleared")

		// Step 9: Verify scheduled count returns to the pre-flag value
		scheduledAfterRemoval := client.GetScheduledCount()
		t.Logf("Scheduled count after removing flag: %d", scheduledAfterRemoval)
		require.Equal(t, scheduledBefore, scheduledAfterRemoval,
			"After removing the manual flag the leaving-soon count should return to baseline")

		t.Logf("=== Sub-Phase 4 Complete: Flag removal and restoration verified ===")
	})

	// Cleanup: Restore original retention (7d) so subsequent tests start from a known state
	t.Cleanup(func() {
		t.Logf("Test cleanup: Restoring retention to 7d")
		UpdateRetentionPolicy(t, absConfigPath, "7d")

		// Best-effort: ensure no manual flags remain for Fight Club
		if mediaID, err := client.GetMediaByTitle("Fight Club"); err == nil {
			_ = client.RemoveManualLeavingSoon(mediaID)
		}

		// Suppress unused variable warning for jellyfinAPIKey
		_ = jellyfinAPIKey

		// Give the system a moment before the next test
		time.Sleep(1 * time.Second)
	})
}
