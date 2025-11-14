package integration

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const (
	TestMovieTitle = "Pulp Fiction"
	TestTagLabel   = "test-deletion"
)

// testDeletionLifecycle tests scheduled deletion with enable_deletion toggle
// Uses a single movie (Pulp Fiction) with an advanced tag-based rule
// Phase 1: enable_deletion=false - verify scheduling without execution
// Phase 2: enable_deletion=true - verify actual deletion from all systems
// NOTE: This test assumes infrastructure is already running from TestInfrastructureSetup
// This is called from TestIntegrationSuite, not run standalone
func testDeletionLifecycle(t *testing.T) {
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

	// Extract API keys for direct service calls
	jellyfinAPIKey := GetJellyfinAPIKey(t, absConfigPath)
	radarrAPIKey := GetRadarrAPIKeyFromYAMLConfig(t, absConfigPath)

	// Setup: Create tag in Radarr and apply to Pulp Fiction
	var tagID int
	var movieID int

	t.Logf("=== Setup: Creating tag and applying to movie ===")

	// Create tag in Radarr
	tagID = CreateRadarrTag(t, RadarrURL, radarrAPIKey, TestTagLabel)
	t.Logf("Created Radarr tag ID %d: %s", tagID, TestTagLabel)

	// Find Pulp Fiction in Radarr
	movie := GetRadarrMovieByTitle(t, RadarrURL, radarrAPIKey, TestMovieTitle)
	require.NotNil(t, movie, "Pulp Fiction not found in Radarr")
	movieID = movie.ID
	t.Logf("Found Pulp Fiction in Radarr with ID: %d", movieID)

	// Tag the movie
	TagRadarrMovie(t, RadarrURL, radarrAPIKey, movieID, tagID)
	t.Logf("Tagged Pulp Fiction with tag: %s", TestTagLabel)

	// Add advanced rule to config
	rule := AdvancedRuleConfig{
		Name:           "Test Deletion Tag",
		Type:           "tag",
		Enabled:        true,
		Tag:            TestTagLabel,
		Retention:      "0d",
		RequireWatched: false,
	}
	AddAdvancedRule(t, absConfigPath, rule)
	t.Logf("Added advanced rule to config")

	// Restart OxiCleanarr to load new config
	RestartOxiCleanarr(t, absComposeFile)
	t.Logf("OxiCleanarr restarted with advanced rule")

	// Re-authenticate after restart
	client.Authenticate(AdminUsername, AdminPassword)

	// Trigger initial sync to populate media library
	t.Logf("Triggering initial sync to load tagged movie...")
	client.TriggerSync()
	time.Sleep(3 * time.Second) // Wait for sync to complete

	t.Logf("=== Setup Complete ===")

	// Phase 1: enable_deletion=false (Scheduled but NOT Deleted)
	t.Run("Phase1_EnableDeletionFalse", func(t *testing.T) {
		t.Logf("=== Phase 1: enable_deletion=false (Scheduled but NOT Deleted) ===")

		// Step 1: Set retention to 999d to prevent untagged movies from being scheduled
		UpdateRetentionPolicy(t, absConfigPath, "999d")

		// Step 2: Ensure enable_deletion is false
		UpdateEnableDeletion(t, absConfigPath, false)
		RestartOxiCleanarr(t, absComposeFile)
		client.Authenticate(AdminUsername, AdminPassword)

		// Step 3: Wait for config hot-reload to complete
		t.Logf("⏳ Waiting for config hot-reload to complete (movie_retention=999d)...")
		WaitForConfigValue(t, client, "rules.movie_retention", "999d")
		t.Logf("✅ Config hot-reload verified")

		// Step 4: Trigger sync and wait for completion
		t.Logf("Triggering sync with enable_deletion=false...")
		client.TriggerSync()

		// Step 5: Wait for job to complete and get result
		job, err := client.WaitForJobCompletion(30 * time.Second)
		require.NoError(t, err, "Failed to wait for job completion")

		summary, ok := job["summary"].(map[string]interface{})
		require.True(t, ok, "Job summary not found")

		// Verify scheduled_deletions count
		scheduledDeletions, ok := summary["scheduled_deletions"].(float64)
		require.True(t, ok, "scheduled_deletions field not found")
		require.Equal(t, float64(1), scheduledDeletions, "Expected 1 item scheduled for deletion")
		t.Logf("✅ Job shows scheduled_deletions: 1")

		// Verify enable_deletion flag
		enableDeletion, ok := summary["enable_deletion"].(bool)
		require.True(t, ok, "enable_deletion field not found")
		require.False(t, enableDeletion, "enable_deletion should be false")
		t.Logf("✅ Job shows enable_deletion: false")

		// Verify would_delete array exists
		wouldDelete, ok := summary["would_delete"].([]interface{})
		require.True(t, ok, "would_delete array not found")
		require.Equal(t, 1, len(wouldDelete), "Expected 1 item in would_delete array")

		// Check first item is Pulp Fiction
		candidate := wouldDelete[0].(map[string]interface{})
		title, _ := candidate["title"].(string)
		require.Contains(t, title, "Pulp Fiction", "Expected Pulp Fiction in would_delete")

		// Log candidate details for debugging Phase 2
		t.Logf("✅ would_delete array contains Pulp Fiction")
		t.Logf("   Candidate details: ID=%v, Title=%v, Year=%v", candidate["id"], candidate["title"], candidate["year"])
		if radarrID, exists := candidate["radarr_id"]; exists {
			t.Logf("   RadarrID=%v (needed for deletion)", radarrID)
		} else {
			t.Logf("   ⚠️  RadarrID not present in candidate (may cause Phase 2 deletion to fail)")
		}

		// Verify deleted_count field does NOT exist
		_, hasDeletedCount := summary["deleted_count"]
		require.False(t, hasDeletedCount, "deleted_count should NOT exist when enable_deletion=false")
		t.Logf("✅ Job has NO deleted_count field (deletion not executed)")

		// Step 6: Verify Pulp Fiction still exists in OxiCleanarr
		movieCount := client.GetMovieCount()
		require.Equal(t, 7, movieCount, "Expected all 7 movies to still exist")
		t.Logf("✅ All 7 movies still exist in OxiCleanarr")

		// Step 7: Verify leaving-soon is empty (Pulp Fiction is overdue for IMMEDIATE deletion, not "leaving soon")
		scheduledCount := client.GetScheduledCount()
		require.Equal(t, 0, scheduledCount, "Expected 0 items in leaving-soon (Pulp Fiction overdue for immediate deletion)")
		t.Logf("✅ Leaving-soon is empty (Pulp Fiction overdue for immediate deletion, not 'leaving soon')")

		// Step 8: Verify 0 symlinks exist (Pulp Fiction is overdue, not in "leaving soon" symlink library)
		t.Logf("Checking symlinks via plugin API...")
		CheckSymlinks(t, jellyfinAPIKey, SymlinkDir, 0)
		t.Logf("✅ Plugin API confirms 0 symlinks (Pulp Fiction overdue, not in leaving-soon library)")

		// Step 9: Verify Pulp Fiction still exists in Radarr
		exists := VerifyMovieExistsInRadarr(t, RadarrURL, radarrAPIKey, TestMovieTitle)
		require.True(t, exists, "Pulp Fiction should still exist in Radarr")
		t.Logf("✅ Pulp Fiction still exists in Radarr")

		// Step 10: Verify media details show deletion info
		mediaID, err := client.GetMediaByTitle(TestMovieTitle)
		require.NoError(t, err, "Failed to find Pulp Fiction")

		details, err := client.GetMediaDetails(mediaID)
		require.NoError(t, err, "Failed to get media details")

		// Check deletion_date exists
		deletionDate, ok := details["deletion_date"]
		require.True(t, ok && deletionDate != nil, "deletion_date should be set")
		t.Logf("✅ Media has deletion_date timestamp: %v", deletionDate)

		// Check deletion_reason mentions the tag
		deletionReason, ok := details["deletion_reason"].(string)
		require.True(t, ok, "deletion_reason should exist")
		require.Contains(t, deletionReason, TestTagLabel, "deletion_reason should mention tag")
		t.Logf("✅ Media has deletion_reason mentioning tag: %s", deletionReason)

		t.Logf("=== Phase 1 Complete: Scheduling verified, deletion NOT executed ===")
	})

	// Phase 2: enable_deletion=true (Actual Deletion)
	t.Run("Phase2_EnableDeletionTrue", func(t *testing.T) {
		t.Logf("=== Phase 2: enable_deletion=true (Actual Deletion) ===")

		// Step 1: Set enable_deletion to true
		UpdateEnableDeletion(t, absConfigPath, true)
		RestartOxiCleanarr(t, absComposeFile)
		client.Authenticate(AdminUsername, AdminPassword)

		// Step 2: Trigger sync (this will execute deletion)
		t.Logf("Triggering sync with enable_deletion=true...")
		client.TriggerSync()

		// Step 3: Wait for job to complete and verify deletion executed
		job, err := client.WaitForJobCompletion(30 * time.Second)
		require.NoError(t, err, "Failed to wait for job completion")

		// Debug: Print the entire job to understand its structure
		t.Logf("DEBUG: Full job response: %+v", job)

		summary, ok := job["summary"].(map[string]interface{})
		if !ok {
			t.Logf("DEBUG: job[\"summary\"] type: %T, value: %+v", job["summary"], job["summary"])
		}
		require.True(t, ok, "Job summary not found")

		// Verify enable_deletion flag
		enableDeletion, ok := summary["enable_deletion"].(bool)
		require.True(t, ok, "enable_deletion field not found")
		require.True(t, enableDeletion, "enable_deletion should be true")
		t.Logf("✅ Job shows enable_deletion: true")

		// Verify scheduled_deletions count
		scheduledDeletions, ok := summary["scheduled_deletions"].(float64)
		require.True(t, ok, "scheduled_deletions field not found")
		require.Equal(t, float64(1), scheduledDeletions, "Expected 1 item scheduled")
		t.Logf("✅ Job shows scheduled_deletions: 1")

		// Verify deleted_count exists and equals 1
		deletedCount, ok := summary["deleted_count"].(float64)
		require.True(t, ok, "deleted_count field should exist")
		require.Equal(t, float64(1), deletedCount, "Expected 1 item deleted")
		t.Logf("✅ Job shows deleted_count: 1 (deletion executed)")

		// Verify deleted_items array exists
		deletedItems, ok := summary["deleted_items"].([]interface{})
		require.True(t, ok, "deleted_items array not found")
		require.Equal(t, 1, len(deletedItems), "Expected 1 item in deleted_items")

		// Check deleted item details
		deletedItem := deletedItems[0].(map[string]interface{})
		title, _ := deletedItem["title"].(string)
		require.Contains(t, title, "Pulp Fiction", "Expected Pulp Fiction in deleted_items")

		year, _ := deletedItem["year"].(float64)
		require.Equal(t, float64(1994), year, "Expected year 1994")

		fileSize, _ := deletedItem["file_size"].(float64)
		require.Greater(t, fileSize, float64(0), "File size should be > 0")

		reason, _ := deletedItem["reason"].(string)
		require.Contains(t, reason, TestTagLabel, "Deletion reason should mention tag")

		t.Logf("✅ deleted_items contains Pulp Fiction with correct details")

		// Step 4: Verify Pulp Fiction removed from OxiCleanarr
		movieCount := client.GetMovieCount()
		require.Equal(t, 6, movieCount, "Expected 6 movies (Pulp Fiction deleted)")
		t.Logf("✅ OxiCleanarr now shows 6 movies (Pulp Fiction removed)")

		// Step 5: Verify leaving-soon is empty
		scheduledCount := client.GetScheduledCount()
		require.Equal(t, 0, scheduledCount, "Expected 0 items in leaving-soon after deletion")
		t.Logf("✅ Leaving-soon is now empty")

		// Step 6: Verify symlinks cleaned up
		t.Logf("Checking symlinks via plugin API...")
		CheckSymlinks(t, jellyfinAPIKey, SymlinkDir, 0)
		t.Logf("✅ Plugin API confirms 0 symlinks (cleaned up)")

		// Step 7: Verify Pulp Fiction deleted from Radarr
		exists := VerifyMovieExistsInRadarr(t, RadarrURL, radarrAPIKey, TestMovieTitle)
		require.False(t, exists, "Pulp Fiction should be deleted from Radarr")
		t.Logf("✅ Pulp Fiction deleted from Radarr")

		// Step 8: Verify Pulp Fiction no longer in OxiCleanarr media library
		_, err = client.GetMediaByTitle(TestMovieTitle)
		require.Error(t, err, "Pulp Fiction should not be found in OxiCleanarr")
		t.Logf("✅ Pulp Fiction not found in OxiCleanarr media library")

		t.Logf("=== Phase 2 Complete: Deletion executed successfully ===")
	})

	// Cleanup: Restore safe config state
	t.Cleanup(func() {
		t.Logf("=== Cleanup: Restoring config to safe state ===")

		// Remove advanced rules
		RemoveAdvancedRules(t, absConfigPath)
		t.Logf("Removed advanced rules from config")

		// Restore retention policy to 7d
		UpdateRetentionPolicy(t, absConfigPath, "7d")
		t.Logf("Restored retention policy to 7d")

		// Set enable_deletion back to false for safety
		UpdateEnableDeletion(t, absConfigPath, false)
		t.Logf("Set enable_deletion back to false")

		// Delete tag from Radarr
		DeleteRadarrTag(t, RadarrURL, radarrAPIKey, tagID)
		t.Logf("Deleted tag from Radarr")

		// Restart OxiCleanarr to apply clean config
		RestartOxiCleanarr(t, absComposeFile)
		t.Logf("OxiCleanarr restarted with clean config")

		t.Logf("=== Cleanup Complete ===")
	})
}
