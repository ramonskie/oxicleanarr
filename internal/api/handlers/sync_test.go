package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ramonskie/prunarr/internal/cache"
	"github.com/ramonskie/prunarr/internal/config"
	"github.com/ramonskie/prunarr/internal/services"
	"github.com/ramonskie/prunarr/internal/storage"
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
