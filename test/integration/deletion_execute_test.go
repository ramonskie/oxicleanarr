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
		// Start a full sync and catch it while it holds the syncRunMu lock.
		resp, _ := rawRequest(t, http.MethodPost, OxiCleanarrURL+"/api/sync/full", clientToken(client),
			"", nil)
		require.Equal(t, http.StatusAccepted, resp.StatusCode)

		runningSeen := false
		deadline := time.Now().Add(10 * time.Second)
		for time.Now().Before(deadline) {
			if syncRunning(t, client) {
				runningSeen = true
				break
			}
			time.Sleep(100 * time.Millisecond)
		}

		if runningSeen {
			execResp, execData := rawRequest(t, http.MethodPost, OxiCleanarrURL+"/api/deletions/execute",
				clientToken(client), "", nil)
			assert.Equal(t, http.StatusConflict, execResp.StatusCode,
				"deletion execution while a sync is running must return 409")
			assert.Contains(t, string(execData), "in progress")
		} else {
			t.Log("Sync completed before the busy window; skipping the 409 assertion (covered by unit tests)")
		}

		// Wait for the sync to finish so later subtests start from a clean state.
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
		// Make every movie overdue so the endpoint has real candidates.
		UpdateRetentionPolicy(t, absConfigPath, "0d")
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
