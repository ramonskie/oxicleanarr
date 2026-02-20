package integration

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// testDiskThresholdLifecycle tests the disk threshold gate end-to-end against
// the live Docker stack.
//
// The test exploits the fact that Radarr's /api/v3/diskspace reports the actual
// free space of the container's filesystem (typically tens of GB in a Docker
// volume). By choosing threshold values that are either far above or far below
// that real free space we can reliably force the gate open or closed without
// needing to mock anything.
//
// All phases use a 1s retention so that movies are immediately past their
// retention window (they were added today). This lets us test the gate
// behaviour without waiting for real time to pass.
//
// Phase 1 — Gate CLOSED (disk OK, no scheduling):
//
//	retention=1s, threshold=1 GB  →  real free space >> 1 GB  →  NOT breached  →  gate closed
//	Even though retention has expired, the gate blocks scheduling; scheduled_deletions == 0.
//
// Phase 2 — Gate OPEN (disk low, scheduling active):
//
//	retention=1s, threshold=999999 GB  →  real free space << 999999 GB  →  BREACHED  →  gate open
//	Movies past retention are scheduled; scheduled_deletions > 0.
//
// Phase 3 — Gate DISABLED (feature off, standard retention applies):
//
//	retention=1s, disk_threshold removed from config  →  standard retention applies normally
//	scheduled_deletions > 0 (same as without the feature).
//
// NOTE: This test assumes infrastructure is already running from TestInfrastructureSetup.
// It is called from TestIntegrationSuite, not run standalone.
func testDiskThresholdLifecycle(t *testing.T) {
	absConfigPath, err := filepath.Abs(ConfigPath)
	require.NoError(t, err)
	require.FileExists(t, absConfigPath, "Config file not found")

	absComposeFile, err := filepath.Abs(ComposeFile)
	require.NoError(t, err)
	require.FileExists(t, absComposeFile, "Docker compose file not found")

	t.Logf("Config path: %s", absConfigPath)
	t.Logf("Compose file: %s", absComposeFile)
	t.Logf("Assuming infrastructure already initialized by TestInfrastructureSetup")

	// Ensure we restore a clean config state regardless of what happens.
	t.Cleanup(func() {
		t.Logf("=== Cleanup: restoring config to safe state ===")
		UpdateDiskThreshold(t, absConfigPath, false, 0, "")
		UpdateRetentionPolicy(t, absConfigPath, "7d")
		RestartOxiCleanarr(t, absComposeFile)
		t.Logf("=== Cleanup complete ===")
	})

	client := NewTestClient(t, OxiCleanarrURL)
	client.Authenticate(AdminUsername, AdminPassword)

	// ── Phase 1: Gate CLOSED — threshold below real free space ───────────────
	t.Run("Phase1_GateClosed_DiskOK", func(t *testing.T) {
		t.Logf("=== Phase 1: disk_threshold enabled, threshold=1GB (gate CLOSED) ===")

		// Use 1s retention so movies are immediately past their retention window.
		// The disk gate (threshold=1GB, not breached) should block all scheduling
		// even though the retention has expired.
		UpdateRetentionPolicy(t, absConfigPath, "1s")

		// threshold=1 GB: real free space is always >> 1 GB in the test container,
		// so the threshold is NOT breached → gate is closed → nothing scheduled.
		UpdateDiskThreshold(t, absConfigPath, true, 1, "radarr")

		RestartOxiCleanarr(t, absComposeFile)
		client.Authenticate(AdminUsername, AdminPassword)

		t.Logf("Triggering sync with disk gate CLOSED (threshold=1GB)...")
		client.TriggerSync()

		job, err := client.WaitForJobCompletion(30 * time.Second)
		require.NoError(t, err, "Failed to wait for job completion")

		summary, ok := job["summary"].(map[string]interface{})
		require.True(t, ok, "Job summary not found")

		scheduledDeletions, ok := summary["scheduled_deletions"].(float64)
		require.True(t, ok, "scheduled_deletions field not found in summary")

		require.Equal(t, float64(0), scheduledDeletions,
			"Disk gate CLOSED: no movies should be scheduled for deletion (threshold not breached)")
		t.Logf("✅ Phase 1 passed: scheduled_deletions=0 (disk gate blocked all scheduling)")

		// Also verify the /api/system/disk endpoint reports the correct state.
		diskStatus, err := client.GetDiskStatus()
		require.NoError(t, err, "Failed to query disk status endpoint")

		enabled, _ := diskStatus["enabled"].(bool)
		require.True(t, enabled, "disk_threshold should be reported as enabled")

		thresholdBreached, _ := diskStatus["threshold_breached"].(bool)
		require.False(t, thresholdBreached,
			"Disk status should report threshold NOT breached (free space >> 1GB)")
		t.Logf("✅ /api/system/disk reports threshold_breached=false")
	})

	// ── Phase 2: Gate OPEN — threshold far above real free space ─────────────
	t.Run("Phase2_GateOpen_DiskBreached", func(t *testing.T) {
		t.Logf("=== Phase 2: disk_threshold enabled, threshold=999999GB (gate OPEN) ===")

		// Keep 1s retention so movies are past their retention window.
		// threshold=999999 GB: real free space is always << 999999 GB,
		// so the threshold IS breached → gate is open → standard retention applies.
		UpdateRetentionPolicy(t, absConfigPath, "1s")
		UpdateDiskThreshold(t, absConfigPath, true, 999999, "radarr")

		RestartOxiCleanarr(t, absComposeFile)
		client.Authenticate(AdminUsername, AdminPassword)

		t.Logf("Triggering sync with disk gate OPEN (threshold=999999GB)...")
		client.TriggerSync()

		job, err := client.WaitForJobCompletion(30 * time.Second)
		require.NoError(t, err, "Failed to wait for job completion")

		summary, ok := job["summary"].(map[string]interface{})
		require.True(t, ok, "Job summary not found")

		scheduledDeletions, ok := summary["scheduled_deletions"].(float64)
		require.True(t, ok, "scheduled_deletions field not found in summary")

		require.Greater(t, scheduledDeletions, float64(0),
			"Disk gate OPEN: movies past 1s retention should be scheduled for deletion")
		t.Logf("✅ Phase 2 passed: scheduled_deletions=%.0f (disk gate open, standard retention active)",
			scheduledDeletions)

		// Verify the /api/system/disk endpoint reports the breached state.
		diskStatus, err := client.GetDiskStatus()
		require.NoError(t, err, "Failed to query disk status endpoint")

		enabled, _ := diskStatus["enabled"].(bool)
		require.True(t, enabled, "disk_threshold should be reported as enabled")

		thresholdBreached, _ := diskStatus["threshold_breached"].(bool)
		require.True(t, thresholdBreached,
			"Disk status should report threshold BREACHED (free space << 999999GB)")
		t.Logf("✅ /api/system/disk reports threshold_breached=true")
	})

	// ── Phase 3: Feature DISABLED — standard retention applies unchanged ──────
	t.Run("Phase3_FeatureDisabled_StandardRetention", func(t *testing.T) {
		t.Logf("=== Phase 3: disk_threshold disabled (standard retention applies) ===")

		// Keep 1s retention so movies are past their retention window.
		// Remove disk_threshold from config entirely — standard retention applies.
		UpdateRetentionPolicy(t, absConfigPath, "1s")
		UpdateDiskThreshold(t, absConfigPath, false, 0, "")

		RestartOxiCleanarr(t, absComposeFile)
		client.Authenticate(AdminUsername, AdminPassword)

		t.Logf("Triggering sync with disk_threshold disabled...")
		client.TriggerSync()

		job, err := client.WaitForJobCompletion(30 * time.Second)
		require.NoError(t, err, "Failed to wait for job completion")

		summary, ok := job["summary"].(map[string]interface{})
		require.True(t, ok, "Job summary not found")

		scheduledDeletions, ok := summary["scheduled_deletions"].(float64)
		require.True(t, ok, "scheduled_deletions field not found in summary")

		require.Greater(t, scheduledDeletions, float64(0),
			"Feature disabled: standard 7d retention should schedule movies normally")
		t.Logf("✅ Phase 3 passed: scheduled_deletions=%.0f (standard retention unaffected)",
			scheduledDeletions)

		// /api/system/disk should report the feature as disabled.
		diskStatus, err := client.GetDiskStatus()
		require.NoError(t, err, "Failed to query disk status endpoint")

		enabled, _ := diskStatus["enabled"].(bool)
		require.False(t, enabled, "disk_threshold should be reported as disabled")
		t.Logf("✅ /api/system/disk reports enabled=false")
	})
}
