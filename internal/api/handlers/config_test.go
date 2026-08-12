package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ramonskie/oxicleanarr/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigHandler_GetConfig(t *testing.T) {
	loadTestConfig(t)
	handler := NewConfigHandler(nil)

	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	rec := httptest.NewRecorder()

	handler.GetConfig(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))

	// Admin password must never be exposed.
	admin, ok := body["admin"].(map[string]interface{})
	require.True(t, ok)
	_, hasPassword := admin["password"]
	assert.False(t, hasPassword, "admin password must be sanitized out")
	assert.Equal(t, "admin", admin["username"])

	// API keys must never be exposed; presence is reported via has_api_key.
	integrations, ok := body["integrations"].(map[string]interface{})
	require.True(t, ok)
	jf, ok := integrations["jellyfin"].(map[string]interface{})
	require.True(t, ok)
	_, hasKey := jf["api_key"]
	assert.False(t, hasKey, "API key must be sanitized out")
	assert.Equal(t, true, jf["has_api_key"], "has_api_key should reflect a configured key")
}

func TestConfigHandler_GetConfig_ConfigNil(t *testing.T) {
	config.SetTestConfig(nil)
	handler := NewConfigHandler(nil)

	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	rec := httptest.NewRecorder()

	handler.GetConfig(rec, req)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestConfigHandler_UpdateConfig(t *testing.T) {
	loadTestConfig(t)
	handler := NewConfigHandler(nil)

	t.Run("updates app settings", func(t *testing.T) {
		body := `{"app":{"leaving_soon_days":21}}`
		req := httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.UpdateConfig(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, 21, config.Get().App.LeavingSoonDays, "config must be persisted after reload")
	})

	t.Run("rejects malformed body", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(`{`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.UpdateConfig(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("rejects config failing validation", func(t *testing.T) {
		// Disabling the only enabled integration makes the config invalid.
		body := `{"integrations":{"jellyfin":{"enabled":false}}}`
		req := httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.UpdateConfig(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), "integration")

		// The failed update must not have been persisted.
		assert.True(t, config.Get().Integrations.Jellyfin.Enabled)
	})
}

func TestConfigHandler_UpdateConfig_ConfigNil(t *testing.T) {
	config.SetTestConfig(nil)
	handler := NewConfigHandler(nil)

	req := httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(`{"app":{"leaving_soon_days":21}}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.UpdateConfig(rec, req)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}
