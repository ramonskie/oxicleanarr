package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ramonskie/oxicleanarr/internal/services"
	"github.com/rs/zerolog/log"
)

// SystemHandler handles system-level operations
type SystemHandler struct {
	syncEngine   *services.SyncEngine
	shutdownCh   chan struct{}
	shutdownOnce sync.Once
	isRestarting atomic.Bool
}

// NewSystemHandler creates a new SystemHandler
func NewSystemHandler(syncEngine *services.SyncEngine, shutdownCh chan struct{}) *SystemHandler {
	return &SystemHandler{
		syncEngine: syncEngine,
		shutdownCh: shutdownCh,
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
		if err != io.EOF {
			// Malformed JSON is a client error, not a silent Force:false.
			log.Warn().Err(err).Msg("Invalid restart request body")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error":   "Invalid request body",
				"message": "Request body must be valid JSON",
			})
			return
		}
		// Empty body — default to a non-force restart.
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

	// Atomically claim the restart; a concurrent request will see it as claimed.
	if !h.isRestarting.CompareAndSwap(false, true) {
		log.Warn().Msg("Restart already in progress")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "Restart already in progress",
			"message": "Application is already restarting",
		})
		return
	}

	log.Info().Bool("force", req.Force).Msg("Application restart requested via API")

	// Send success response before shutting down.
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Application restart initiated. Server will be unavailable for a few seconds.",
		"status":  "restarting",
	})

	// Flush the response so the client actually receives it before the
	// process tears down. Waiting on a fixed sleep does not guarantee this.
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}

	// Trigger graceful shutdown in a separate goroutine. The shutdown signal
	// is sent unconditionally via a deferred sync.Once, so even if stopping
	// the sync engine panics (recovered below) the process still exits and the
	// restart actually happens instead of wedging in a "restarting" state.
	go func() {
		defer recoverPanic("application restart")
		defer h.shutdownOnce.Do(func() {
			close(h.shutdownCh)
		})

		log.Info().Msg("Initiating graceful shutdown for restart")

		// Stop sync engine first
		if h.syncEngine != nil {
			log.Info().Msg("Stopping sync engine")
			h.syncEngine.Stop()
		}
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
		"restarting": h.isRestarting.Load(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(info)
}

// GetDiskStatus handles GET /api/system/disk
func (h *SystemHandler) GetDiskStatus(w http.ResponseWriter, r *http.Request) {
	dm := h.syncEngine.GetDiskMonitor()
	if dm == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"enabled": false,
			"message": "Disk threshold monitoring is not enabled",
		})
		return
	}

	status := dm.GetStatus()
	if status == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"enabled": false,
			"message": "Disk threshold monitoring is disabled",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"enabled":            status.Enabled,
		"free_space_gb":      status.FreeSpaceGB,
		"total_space_gb":     status.TotalSpaceGB,
		"threshold_gb":       status.ThresholdGB,
		"threshold_breached": status.ThresholdBreached,
		"check_source":       status.CheckSource,
	})
}
