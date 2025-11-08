package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ramonskie/oxicleanarr/internal/cache"
	"github.com/ramonskie/oxicleanarr/internal/config"
	"github.com/ramonskie/oxicleanarr/internal/services"
	"github.com/ramonskie/oxicleanarr/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper to create a test sync engine
func newTestSyncEngineForAPI(t *testing.T) *services.SyncEngine {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		Sync: config.SyncConfig{
			FullInterval:        3600,
			IncrementalInterval: 300,
			AutoStart:           false,
		},
		Rules: config.RulesConfig{
			MovieRetention: "90d",
			TVRetention:    "120d",
		},
	}

	// Set global config for tests that use config.Get()
	config.SetTestConfig(cfg)

	cacheInstance := cache.New()
	jobs, err := storage.NewJobsFile(tmpDir, 50)
	require.NoError(t, err)

	exclusions, err := storage.NewExclusionsFile(tmpDir)
	require.NoError(t, err)

	rules := services.NewRulesEngine(cfg, exclusions)
	engine := services.NewSyncEngine(cfg, cacheInstance, jobs, exclusions, rules)

	return engine
}

func TestSyncHandler_TriggerFullSync(t *testing.T) {
	t.Run("triggers full sync successfully", func(t *testing.T) {
		engine := newTestSyncEngineForAPI(t)
		handler := NewSyncHandler(engine)

		req := httptest.NewRequest(http.MethodPost, "/api/sync/full", nil)
		w := httptest.NewRecorder()

		handler.TriggerFullSync(w, req)

		assert.Equal(t, http.StatusAccepted, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

		var response map[string]interface{}
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)

		assert.True(t, response["success"].(bool))
		assert.Equal(t, "Full sync started", response["message"])
	})

	t.Run("accepts request with context", func(t *testing.T) {
		engine := newTestSyncEngineForAPI(t)
		handler := NewSyncHandler(engine)

		ctx := context.Background()
		req := httptest.NewRequest(http.MethodPost, "/api/sync/full", nil).WithContext(ctx)
		w := httptest.NewRecorder()

		handler.TriggerFullSync(w, req)

		assert.Equal(t, http.StatusAccepted, w.Code)
	})
}

func TestSyncHandler_TriggerIncrementalSync(t *testing.T) {
	t.Run("triggers incremental sync successfully", func(t *testing.T) {
		engine := newTestSyncEngineForAPI(t)
		handler := NewSyncHandler(engine)

		req := httptest.NewRequest(http.MethodPost, "/api/sync/incremental", nil)
		w := httptest.NewRecorder()

		handler.TriggerIncrementalSync(w, req)

		assert.Equal(t, http.StatusAccepted, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

		var response map[string]interface{}
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)

		assert.True(t, response["success"].(bool))
		assert.Equal(t, "Incremental sync started", response["message"])
	})
}

func TestSyncHandler_GetSyncStatus(t *testing.T) {
	t.Run("returns sync status when engine is stopped", func(t *testing.T) {
		engine := newTestSyncEngineForAPI(t)
		handler := NewSyncHandler(engine)

		req := httptest.NewRequest(http.MethodGet, "/api/sync/status", nil)
		w := httptest.NewRecorder()

		handler.GetSyncStatus(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

		var response services.SyncStatus
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)

		assert.False(t, response.Running)
		assert.Equal(t, 3600, response.FullInterval)
		assert.Equal(t, 300, response.IncrInterval)
		assert.Equal(t, 0, response.MediaCount)
	})

	t.Run("returns sync status when engine is running", func(t *testing.T) {
		engine := newTestSyncEngineForAPI(t)
		handler := NewSyncHandler(engine)

		// Start the engine
		err := engine.Start()
		require.NoError(t, err)
		defer engine.Stop()

		req := httptest.NewRequest(http.MethodGet, "/api/sync/status", nil)
		w := httptest.NewRecorder()

		handler.GetSyncStatus(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response services.SyncStatus
		err = json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)

		assert.True(t, response.Running)
	})
}

func TestSyncHandler_ExecuteDeletions(t *testing.T) {
	t.Run("returns empty when no scheduled deletions", func(t *testing.T) {
		engine := newTestSyncEngineForAPI(t)
		handler := NewSyncHandler(engine)

		req := httptest.NewRequest(http.MethodPost, "/api/deletions/execute", nil)
		w := httptest.NewRecorder()

		handler.ExecuteDeletions(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

		var response map[string]interface{}
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)

		assert.True(t, response["success"].(bool))
		assert.Equal(t, float64(0), response["scheduled_count"])
		assert.Nil(t, response["deleted_count"]) // Should not include deleted_count when no items
		assert.Equal(t, "No items scheduled for deletion", response["message"])
	})

	t.Run("returns dry-run preview when dry_run=true", func(t *testing.T) {
		engine := newTestSyncEngineForAPI(t)
		handler := NewSyncHandler(engine)

		// Trigger a full sync to populate some data
		ctx := context.Background()
		err := engine.FullSync(ctx)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, "/api/deletions/execute?dry_run=true", nil)
		w := httptest.NewRecorder()

		handler.ExecuteDeletions(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

		var response map[string]interface{}
		err = json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)

		assert.True(t, response["success"].(bool))
		if response["dry_run"] != nil {
			assert.True(t, response["dry_run"].(bool))
		}
	})

	t.Run("executes deletions when dry_run=false", func(t *testing.T) {
		engine := newTestSyncEngineForAPI(t)
		handler := NewSyncHandler(engine)

		// Trigger a full sync to populate some data
		ctx := context.Background()
		err := engine.FullSync(ctx)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, "/api/deletions/execute?dry_run=false", nil)
		w := httptest.NewRecorder()

		handler.ExecuteDeletions(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

		var response map[string]interface{}
		err = json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)

		assert.True(t, response["success"].(bool))
		// Message depends on whether there are deletions or not
		assert.NotEmpty(t, response["message"])

		// Verify response includes expected fields
		assert.NotNil(t, response["scheduled_count"])

		// If there are scheduled deletions, should have deleted_count
		scheduledCount := int(response["scheduled_count"].(float64))
		if scheduledCount > 0 {
			assert.NotNil(t, response["deleted_count"])
			assert.NotNil(t, response["failed_count"])
		} else {
			// No scheduled deletions means no deleted_count field
			assert.Nil(t, response["deleted_count"])
		}
	})

	t.Run("defaults to actual execution when no query param", func(t *testing.T) {
		engine := newTestSyncEngineForAPI(t)
		handler := NewSyncHandler(engine)

		req := httptest.NewRequest(http.MethodPost, "/api/deletions/execute", nil)
		w := httptest.NewRecorder()

		handler.ExecuteDeletions(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)

		assert.True(t, response["success"].(bool))
		// Should not have dry_run flag or it should be false
		if response["dry_run"] != nil {
			assert.False(t, response["dry_run"].(bool))
		}
	})
}
