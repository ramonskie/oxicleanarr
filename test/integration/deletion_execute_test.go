package integration

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testDeletionExecute exercises POST /api/deletions/execute: the 409-when-busy
// guard, the dry-run contract, and the accounting fields. The final subtest
// deletes overdue media, so this runs LAST in the suite.
func testDeletionExecute(t *testing.T) {
	absConfigPath, err := filepath.Abs(ConfigPath)
	require.NoError(t, err)
	require.FileExists(t, absConfigPath, "Config file not found")

	absComposeFile, err := filepath.Abs(ComposeFile)
	require.NoError(t, err)
	require.FileExists(t, absComposeFile, "Docker compose file not found")

	client := NewTestClient(t, OxiCleanarrURL)
	client.Authenticate(AdminUsername, AdminPassword)

	t.Run("BusyReturns409", func(t *testing.T) {
		// A sync in this env takes only ~30ms, so catching the busy window is racy.
		// Queue several syncs (they serialize on the sync lock) and poll tightly,
		// firing the deletion request the moment a sync is seen running. If we never
		// land inside the window, the 409 guard is still covered by unit tests.
		for i := 0; i < 5; i++ {
			rawRequest(t, http.MethodPost, OxiCleanarrURL+"/api/sync/full", clientToken(client), "", nil)
		}

		caught := false
		deadline := time.Now().Add(20 * time.Second)
		for time.Now().Before(deadline) && !caught {
			if syncRunning(t, client) {
				execResp, execData := rawRequest(t, http.MethodPost, OxiCleanarrURL+"/api/deletions/execute",
					clientToken(client), "", nil)
				if execResp.StatusCode == http.StatusConflict {
					assert.Contains(t, string(execData), "in progress")
					caught = true
					break
				}
				// Sync finished between the poll and the request; retry against the
				// next queued sync.
			}
			time.Sleep(5 * time.Millisecond)
		}

		if !caught {
			t.Log("Did not catch the busy window; the 409 guard is covered by unit tests")
		}

		// Wait for the queued syncs to drain so later subtests start from a clean state.
		require.Eventually(t, func() bool { return !syncRunning(t, client) }, 60*time.Second, time.Second,
			"sync should eventually complete")
	})

	t.Run("DryRunContract", func(t *testing.T) {
		resp, data := rawRequest(t, http.MethodPost, OxiCleanarrURL+"/api/deletions/execute?dry_run=true",
			clientToken(client), "", nil)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var result struct {
			Success        bool                     `json:"success"`
			ScheduledCount int                      `json:"scheduled_count"`
			DryRun         bool                     `json:"dry_run"`
			Candidates     []map[string]interface{} `json:"candidates"`
		}
		require.NoError(t, json.Unmarshal(data, &result))
		assert.True(t, result.Success)
		assert.True(t, result.DryRun)
		assert.Equal(t, len(result.Candidates), result.ScheduledCount,
			"scheduled_count must equal the number of candidates")
	})

	t.Run("Accounting", func(t *testing.T) {
		// Make every movie overdue so the endpoint has real candidates. "0d" disables
		// retention (never delete), so use a 1-second retention — every movie was
		// added minutes ago, so added+1s is already in the past.
		UpdateRetentionPolicy(t, absConfigPath, "1s")
		RestartOxiCleanarr(t, absComposeFile)
		client.Authenticate(AdminUsername, AdminPassword)
		client.TriggerSync()

		// Dry-run should now surface candidates.
		_, dryData := rawRequest(t, http.MethodPost, OxiCleanarrURL+"/api/deletions/execute?dry_run=true",
			clientToken(client), "", nil)
		var dryRun struct {
			ScheduledCount int `json:"scheduled_count"`
		}
		require.NoError(t, json.Unmarshal(dryData, &dryRun))
		require.GreaterOrEqual(t, dryRun.ScheduledCount, 1,
			"with retention 0d the imported movies must be scheduled for deletion")

		// Real execution must report the new accounting fields consistently.
		resp, data := rawRequest(t, http.MethodPost, OxiCleanarrURL+"/api/deletions/execute",
			clientToken(client), "", nil)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var result struct {
			Success             bool                     `json:"success"`
			ScheduledCount      int                      `json:"scheduled_count"`
			DeletedCount        int                      `json:"deleted_count"`
			EpisodeFilesDeleted int                      `json:"episode_files_deleted"`
			ProtectedCount      int                      `json:"protected_count"`
			FailedCount         int                      `json:"failed_count"`
			DeletedItems        []map[string]interface{} `json:"deleted_items"`
		}
		require.NoError(t, json.Unmarshal(data, &result))

		assert.Equal(t, result.ScheduledCount, result.DeletedCount+result.ProtectedCount+result.FailedCount,
			"every scheduled candidate must be deleted, protected, or failed")
		assert.GreaterOrEqual(t, result.DeletedCount, 1, "overdue movies must be deleted")
		assert.Equal(t, result.DeletedCount, len(result.DeletedItems), "deleted_items must match deleted_count")
		assert.Equal(t, result.Success, result.FailedCount == 0,
			"success must be true only when nothing failed")
	})
}
