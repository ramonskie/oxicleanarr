package handlers

import (
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

func TestRecoverPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		func() {
			defer recoverPanic("test")
			panic(errors.New("boom"))
		}()
	})
}
