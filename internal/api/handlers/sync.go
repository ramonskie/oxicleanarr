package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/ramonskie/prunarr/internal/services"
	"github.com/rs/zerolog/log"
)

// SyncHandler handles sync-related requests
type SyncHandler struct {
	syncEngine *services.SyncEngine
}

// NewSyncHandler creates a new SyncHandler
func NewSyncHandler(syncEngine *services.SyncEngine) *SyncHandler {
	return &SyncHandler{
		syncEngine: syncEngine,
	}
}

// TriggerFullSync handles POST /api/sync/full
func (h *SyncHandler) TriggerFullSync(w http.ResponseWriter, r *http.Request) {
	log.Info().Msg("Manual full sync triggered via API")

	// Use background context for async operation, not request context
	// which would be canceled when the response is sent
	go func() {
		ctx := context.Background()
		if err := h.syncEngine.FullSync(ctx); err != nil {
			log.Error().Err(err).Msg("Manual full sync failed")
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Full sync started",
	})
}

// TriggerIncrementalSync handles POST /api/sync/incremental
func (h *SyncHandler) TriggerIncrementalSync(w http.ResponseWriter, r *http.Request) {
	log.Info().Msg("Manual incremental sync triggered via API")

	// Use background context for async operation, not request context
	// which would be canceled when the response is sent
	go func() {
		ctx := context.Background()
		if err := h.syncEngine.IncrementalSync(ctx); err != nil {
			log.Error().Err(err).Msg("Manual incremental sync failed")
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Incremental sync started",
	})
}

// GetSyncStatus handles GET /api/sync/status
func (h *SyncHandler) GetSyncStatus(w http.ResponseWriter, r *http.Request) {
	status := h.syncEngine.GetStatus()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(status)
}

// ExecuteDeletions handles POST /api/deletions/execute
func (h *SyncHandler) ExecuteDeletions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Check for dry-run query parameter
	dryRun := r.URL.Query().Get("dry_run") == "true"

	if dryRun {
		log.Info().Msg("Manual deletion execution triggered in dry-run mode")
	} else {
		log.Info().Msg("Manual deletion execution triggered")
	}

	// Get current deletion candidates
	scheduledCount, candidates := h.syncEngine.CalculateDeletionInfo()

	if len(candidates) == 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":         true,
			"scheduled_count": 0,
			"message":         "No items scheduled for deletion",
			"candidates":      []map[string]interface{}{},
		})
		return
	}

	// If dry-run, just return the preview
	if dryRun {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":         true,
			"scheduled_count": scheduledCount,
			"dry_run":         true,
			"message":         "Dry-run preview: No deletions performed",
			"candidates":      candidates,
		})
		return
	}

	// Execute actual deletions
	deletedCount, deletedItems := h.syncEngine.ExecuteDeletions(ctx, candidates)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":         true,
		"scheduled_count": scheduledCount,
		"deleted_count":   deletedCount,
		"failed_count":    len(candidates) - deletedCount,
		"message":         "Deletion execution completed",
		"deleted_items":   deletedItems,
	})
}
