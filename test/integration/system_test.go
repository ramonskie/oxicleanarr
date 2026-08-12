package integration

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testSystemEndpoints exercises system API behaviors: restart input validation
// and service-status error sanitization.
func testSystemEndpoints(t *testing.T) {
	t.Run("RestartRejectsMalformedJSON", func(t *testing.T) {
		token := loginToken(t)
		resp, _ := rawRequest(t, http.MethodPost, OxiCleanarrURL+"/api/system/restart", token,
			"application/json", []byte(`{"force":`))
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "malformed restart body must be rejected with 400")

		// The app must still be up after the rejected restart.
		healthResp, err := http.Get(OxiCleanarrURL + "/health")
		require.NoError(t, err)
		healthResp.Body.Close()
		assert.Equal(t, http.StatusOK, healthResp.StatusCode, "app must stay up after a rejected restart")
	})

	t.Run("RestartRejectsBusySync", func(t *testing.T) {
		client := NewTestClient(t, OxiCleanarrURL)
		client.Authenticate(AdminUsername, AdminPassword)

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
			// Non-force restart while a sync is running must be rejected.
			restartResp, restartData := rawRequest(t, http.MethodPost, OxiCleanarrURL+"/api/system/restart",
				clientToken(client), "", nil)
			assert.Equal(t, http.StatusConflict, restartResp.StatusCode,
				"non-force restart while a sync is running must return 409")
			assert.Contains(t, string(restartData), "running")
		} else {
			t.Log("Sync completed before the busy window; skipping the restart 409 assertion")
		}

		require.Eventually(t, func() bool { return !syncRunning(t, client) }, 60*time.Second, time.Second,
			"sync should eventually complete")
	})

	t.Run("ServicesStatusSanitizesErrors", func(t *testing.T) {
		client := NewTestClient(t, OxiCleanarrURL)
		client.Authenticate(AdminUsername, AdminPassword)

		resp, data := rawRequest(t, http.MethodGet, OxiCleanarrURL+"/api/system/services", clientToken(client),
			"", nil)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var result struct {
			Services []struct {
				Name    string `json:"name"`
				Enabled bool   `json:"enabled"`
				Online  bool   `json:"online"`
				Error   string `json:"error"`
			} `json:"services"`
		}
		require.NoError(t, json.Unmarshal(data, &result))
		require.NotEmpty(t, result.Services, "services list must not be empty")

		// The core integrations are running in the test env, so they must be online.
		onlineByName := map[string]bool{}
		for _, s := range result.Services {
			onlineByName[s.Name] = s.Online
			if s.Error == "" {
				continue
			}
			// Any error must be the generic sanitized form: no URL, scheme,
			// host:port, or IP detail may leak to the public API.
			assert.NotContains(t, s.Error, "://", "service error must not contain a URL scheme")
			assert.NotContains(t, s.Error, "http", "service error must not contain an http URL")
			assert.NotContains(t, s.Error, "host.docker.internal", "service error must not leak docker hostname")
			assert.NotContains(t, s.Error, "oxicleanarr-test-", "service error must not leak container hostname")
		}
		assert.True(t, onlineByName["Jellyfin"], "Jellyfin should be online in the test env")
		assert.True(t, onlineByName["Radarr"], "Radarr should be online in the test env")
		assert.True(t, onlineByName["Sonarr"], "Sonarr should be online in the test env")
	})
}
