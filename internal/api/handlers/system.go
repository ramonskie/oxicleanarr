package handlers

import (
	"encoding/json"
	"net/http"
	"os"
	"time"

	"github.com/ramonskie/oxicleanarr/internal/services"
	"github.com/rs/zerolog/log"
)

// SystemHandler handles system-level operations
type SystemHandler struct {
	syncEngine   *services.SyncEngine
	shutdownCh   chan struct{}
	isRestarting bool
}

// NewSystemHandler creates a new SystemHandler
func NewSystemHandler(syncEngine *services.SyncEngine, shutdownCh chan struct{}) *SystemHandler {
	return &SystemHandler{
		syncEngine:   syncEngine,
		shutdownCh:   shutdownCh,
		isRestarting: false,
	}
}

// RestartRequest represents a restart request
type RestartRequest struct {
	Force bool `json:"force,omitempty"` // Force restart even if sync is running
}

// Restart handles POST /api/system/restart
func (h *SystemHandler) Restart(w http.ResponseWriter, r *http.Request) {
	var req RestartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Allow empty body (default to non-force restart)
		req.Force = false
	}

	// Check if sync is running
	status := h.syncEngine.GetStatus()
	if status.Running && !req.Force {
		log.Warn().Msg("Restart requested but sync engine is running")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "Sync engine is currently running",
			"message": "A sync operation is in progress. Wait for it to complete or use force=true to restart anyway.",
			"running": true,
		})
		return
	}

	if h.isRestarting {
		log.Warn().Msg("Restart already in progress")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "Restart already in progress",
			"message": "Application is already restarting",
		})
		return
	}

	h.isRestarting = true

	log.Info().Bool("force", req.Force).Msg("Application restart requested via API")

	// Send success response before shutting down
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Application restart initiated. Server will be unavailable for a few seconds.",
		"status":  "restarting",
	})

	// Trigger graceful shutdown in a separate goroutine
	go func() {
		// Give time for the response to be sent
		time.Sleep(500 * time.Millisecond)

		log.Info().Msg("Initiating graceful shutdown for restart")

		// Stop sync engine first
		if h.syncEngine != nil {
			log.Info().Msg("Stopping sync engine")
			h.syncEngine.Stop()
		}

		// Signal shutdown to main
		close(h.shutdownCh)
	}()
}

// HealthCheck handles GET /api/system/health
func (h *SystemHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	status := h.syncEngine.GetStatus()

	health := map[string]interface{}{
		"status":       "healthy",
		"sync_running": status.Running,
		"media_count":  status.MediaCount,
		"timestamp":    time.Now().UTC(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(health)
}

// GetInfo handles GET /api/system/info
func (h *SystemHandler) GetInfo(w http.ResponseWriter, r *http.Request) {
	hostname, _ := os.Hostname()

	info := map[string]interface{}{
		"hostname":   hostname,
		"pid":        os.Getpid(),
		"go_version": os.Getenv("GO_VERSION"),
		"restarting": h.isRestarting,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(info)
}
