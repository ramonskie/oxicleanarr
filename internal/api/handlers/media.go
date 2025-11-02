package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/ramonskie/prunarr/internal/models"
	"github.com/ramonskie/prunarr/internal/services"
	"github.com/rs/zerolog/log"
)

// MediaHandler handles media-related requests
type MediaHandler struct {
	syncEngine *services.SyncEngine
}

// NewMediaHandler creates a new MediaHandler
func NewMediaHandler(syncEngine *services.SyncEngine) *MediaHandler {
	return &MediaHandler{
		syncEngine: syncEngine,
	}
}

// ListMovies handles GET /api/media/movies
func (h *MediaHandler) ListMovies(w http.ResponseWriter, r *http.Request) {
	// Get query parameters
	sortBy := r.URL.Query().Get("sort_by")      // e.g., "title", "added_at", "delete_after"
	order := r.URL.Query().Get("order")         // "asc" or "desc"
	filterStatus := r.URL.Query().Get("status") // "all", "leaving_soon", "excluded"

	media := h.syncEngine.GetMediaList()

	// Filter movies only
	var movies []models.Media
	for _, item := range media {
		if item.Type == models.MediaTypeMovie {
			// Apply status filter
			if filterStatus == "leaving_soon" && item.DaysUntilDue <= 0 {
				continue
			}
			if filterStatus == "excluded" && !item.IsExcluded {
				continue
			}
			movies = append(movies, item)
		}
	}

	// Sort movies
	movies = sortMedia(movies, sortBy, order)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"movies": movies,
		"total":  len(movies),
	})
}

// ListShows handles GET /api/media/shows
func (h *MediaHandler) ListShows(w http.ResponseWriter, r *http.Request) {
	// Get query parameters
	sortBy := r.URL.Query().Get("sort_by")
	order := r.URL.Query().Get("order")
	filterStatus := r.URL.Query().Get("status")

	media := h.syncEngine.GetMediaList()

	// Filter shows only
	var shows []models.Media
	for _, item := range media {
		if item.Type == models.MediaTypeTVShow {
			// Apply status filter
			if filterStatus == "leaving_soon" && item.DaysUntilDue <= 0 {
				continue
			}
			if filterStatus == "excluded" && !item.IsExcluded {
				continue
			}
			shows = append(shows, item)
		}
	}

	// Sort shows
	shows = sortMedia(shows, sortBy, order)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"shows": shows,
		"total": len(shows),
	})
}

// ListLeavingSoon handles GET /api/media/leaving-soon
func (h *MediaHandler) ListLeavingSoon(w http.ResponseWriter, r *http.Request) {
	media := h.syncEngine.GetMediaList()

	// Filter leaving soon items (items with positive DaysUntilDue, meaning they'll be deleted soon)
	var leavingSoon []models.Media
	for _, item := range media {
		if item.DaysUntilDue > 0 && !item.IsExcluded {
			leavingSoon = append(leavingSoon, item)
		}
	}

	// Sort by deletion date (earliest first)
	leavingSoon = sortMedia(leavingSoon, "delete_after", "asc")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"media": leavingSoon,
		"total": len(leavingSoon),
	})
}

// GetMediaItem handles GET /api/media/{id}
func (h *MediaHandler) GetMediaItem(w http.ResponseWriter, r *http.Request) {
	// Extract ID from path
	path := strings.TrimPrefix(r.URL.Path, "/api/media/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "Media ID required", http.StatusBadRequest)
		return
	}
	id := parts[0]

	media, found := h.syncEngine.GetMediaByID(id)
	if !found {
		http.Error(w, "Media not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(media)
}

// AddExclusion handles POST /api/media/{id}/exclude
func (h *MediaHandler) AddExclusion(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Extract ID from path
	path := strings.TrimPrefix(r.URL.Path, "/api/media/")
	parts := strings.Split(path, "/")
	if len(parts) < 2 || parts[0] == "" {
		http.Error(w, "Media ID required", http.StatusBadRequest)
		return
	}
	id := parts[0]

	// Parse request body for reason
	var reqBody struct {
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		log.Debug().Err(err).Msg("No exclusion reason provided")
	}

	if err := h.syncEngine.AddExclusion(ctx, id, reqBody.Reason); err != nil {
		log.Error().Err(err).Str("media_id", id).Msg("Failed to add exclusion")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Exclusion added",
	})
}

// RemoveExclusion handles DELETE /api/media/{id}/exclude
func (h *MediaHandler) RemoveExclusion(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Extract ID from path
	path := strings.TrimPrefix(r.URL.Path, "/api/media/")
	parts := strings.Split(path, "/")
	if len(parts) < 2 || parts[0] == "" {
		http.Error(w, "Media ID required", http.StatusBadRequest)
		return
	}
	id := parts[0]

	if err := h.syncEngine.RemoveExclusion(ctx, id); err != nil {
		log.Error().Err(err).Str("media_id", id).Msg("Failed to remove exclusion")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Exclusion removed",
	})
}

// DeleteMedia handles DELETE /api/media/{id}
func (h *MediaHandler) DeleteMedia(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Extract ID from path
	path := strings.TrimPrefix(r.URL.Path, "/api/media/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "Media ID required", http.StatusBadRequest)
		return
	}
	id := parts[0]

	// Check for dry run
	dryRun := r.URL.Query().Get("dry_run") == "true"

	if err := h.syncEngine.DeleteMedia(ctx, id, dryRun); err != nil {
		log.Error().Err(err).Str("media_id", id).Bool("dry_run", dryRun).Msg("Failed to delete media")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	message := "Media deleted successfully"
	if dryRun {
		message = "Dry run: Media would be deleted"
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": message,
		"dry_run": dryRun,
	})
}

// sortMedia sorts media by the given field and order
func sortMedia(media []models.Media, sortBy, order string) []models.Media {
	if len(media) == 0 {
		return media
	}

	// Simple bubble sort for now (can be optimized with sort.Slice)
	result := make([]models.Media, len(media))
	copy(result, media)

	for i := 0; i < len(result); i++ {
		for j := i + 1; j < len(result); j++ {
			swap := false

			switch sortBy {
			case "title":
				if order == "desc" {
					swap = result[i].Title < result[j].Title
				} else {
					swap = result[i].Title > result[j].Title
				}
			case "added_at":
				if order == "desc" {
					swap = result[i].AddedAt.Before(result[j].AddedAt)
				} else {
					swap = result[i].AddedAt.After(result[j].AddedAt)
				}
			case "delete_after":
				// Handle zero deletion dates
				if result[i].DeleteAfter.IsZero() && !result[j].DeleteAfter.IsZero() {
					swap = order != "desc"
				} else if !result[i].DeleteAfter.IsZero() && result[j].DeleteAfter.IsZero() {
					swap = order == "desc"
				} else if !result[i].DeleteAfter.IsZero() && !result[j].DeleteAfter.IsZero() {
					if order == "desc" {
						swap = result[i].DeleteAfter.Before(result[j].DeleteAfter)
					} else {
						swap = result[i].DeleteAfter.After(result[j].DeleteAfter)
					}
				}
			default:
				// Default sort by added date
				swap = result[i].AddedAt.After(result[j].AddedAt)
			}

			if swap {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	return result
}
