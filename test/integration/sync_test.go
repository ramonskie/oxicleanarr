package integration

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testSyncQueueing verifies that a full sync triggered while another is running
// is accepted (queued behind it via acquireSyncRunLock) and that both runs
// complete without deadlock or a 409 rejection.
func testSyncQueueing(t *testing.T) {
	client := NewTestClient(t, OxiCleanarrURL)
	client.Authenticate(AdminUsername, AdminPassword)

	// Trigger a full sync, then a second one immediately while the first runs.
	resp1, _ := rawRequest(t, http.MethodPost, OxiCleanarrURL+"/api/sync/full", clientToken(client), "", nil)
	require.Equal(t, http.StatusAccepted, resp1.StatusCode, "first sync must be accepted")

	resp2, _ := rawRequest(t, http.MethodPost, OxiCleanarrURL+"/api/sync/full", clientToken(client), "", nil)
	require.Equal(t, http.StatusAccepted, resp2.StatusCode, "second sync must be accepted (queued)")

	// Both runs must finish; the queued one must not deadlock behind the first.
	require.Eventually(t, func() bool { return !syncRunning(t, client) }, 120*time.Second, time.Second,
		"queued syncs must both complete")

	assert.False(t, syncRunning(t, client), "sync engine must be idle after queued syncs complete")
}
