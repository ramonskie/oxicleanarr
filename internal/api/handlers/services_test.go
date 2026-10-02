package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ramonskie/oxicleanarr/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServiceStatusHandler_CheckStatus_CompletesWithUnreachableService(t *testing.T) {
	// A service with an unresolvable URL must not block the handler: the
	// per-service goroutine releases wg.Done() even when the ping fails.
	cfg := &config.Config{}
	cfg.Integrations.Streamystats.Enabled = true
	cfg.Integrations.Streamystats.URL = "http://127.0.0.1:1"

	config.SetTestConfig(cfg)
	defer config.SetTestConfig(nil)

	h := NewServiceStatusHandler()
	req := httptest.NewRequest(http.MethodGet, "/api/system/services", nil)
	rec := httptest.NewRecorder()

	require.NotPanics(t, func() { h.CheckStatus(rec, req) })
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestServiceStatusHandler_CheckStatus_IncludesTracearr(t *testing.T) {
	// Tracearr must appear in the status list and its ping failure must be
	// sanitized (no host/port or API key leaks into the response).
	cfg := &config.Config{}
	cfg.Integrations.Tracearr.Enabled = true
	cfg.Integrations.Tracearr.URL = "http://127.0.0.1:1"
	cfg.Integrations.Tracearr.APIKey = "trr_pub_supersecret"
	cfg.Integrations.Tracearr.ServerID = "tracearr-server-uuid"

	config.SetTestConfig(cfg)
	defer config.SetTestConfig(nil)

	h := NewServiceStatusHandler()
	req := httptest.NewRequest(http.MethodGet, "/api/system/services", nil)
	rec := httptest.NewRecorder()

	require.NotPanics(t, func() { h.CheckStatus(rec, req) })
	require.Equal(t, http.StatusOK, rec.Code)

	var resp ServiceStatusResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))

	var tracearr *ServiceStatus
	for i := range resp.Services {
		if resp.Services[i].Name == "Tracearr" {
			tracearr = &resp.Services[i]
			break
		}
	}
	require.NotNil(t, tracearr, "Tracearr status must appear in the response")
	assert.True(t, tracearr.Enabled)
	assert.False(t, tracearr.Online)
	assert.NotEmpty(t, tracearr.Error, "failed ping must report a sanitized error")
	assert.NotContains(t, tracearr.Error, "127.0.0.1", "host/port must be scrubbed")
	assert.NotContains(t, rec.Body.String(), "trr_pub_supersecret", "api key must never leak")
}

func TestRecoverPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		func() {
			defer recoverPanic("test")
			panic(errors.New("boom"))
		}()
	})
}
