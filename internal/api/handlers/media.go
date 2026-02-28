package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/ramonskie/oxicleanarr/internal/config"
	"github.com/ramonskie/oxicleanarr/internal/models"
	"github.com/ramonskie/oxicleanarr/internal/services"
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
		"items": movies,
		"total": len(movies),
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
		"items": shows,
		"total": len(shows),
	})
}

// ListLeavingSoon handles GET /api/media/leaving-soon
func (h *MediaHandler) ListLeavingSoon(w http.ResponseWriter, r *http.Request) {
	media := h.syncEngine.GetMediaList()

	// Get leaving_soon_days threshold from config
	cfg := config.Get()
	leavingSoonDays := cfg.App.LeavingSoonDays

	// Filter leaving soon items (items within the leaving_soon_days threshold)
	var leavingSoon []models.Media
	for _, item := range media {
		if item.DaysUntilDue > 0 && item.DaysUntilDue <= leavingSoonDays && !item.IsExcluded {
			leavingSoon = append(leavingSoon, item)
		}
	}

	// Sort by deletion date (earliest first)
	leavingSoon = sortMedia(leavingSoon, "delete_after", "asc")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"items": leavingSoon,
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

// ProxyPoster handles GET /api/media/{id}/poster
// Proxies the poster image from Jellyfin, adding the API key server-side
// so the frontend never sees the Jellyfin credentials.
// Optional query params: maxWidth (int), quality (int), type (Primary|Backdrop).
func (h *MediaHandler) ProxyPoster(w http.ResponseWriter, r *http.Request) {
	// Extract media ID from path
	path := strings.TrimPrefix(r.URL.Path, "/api/media/")
	parts := strings.Split(path, "/")
	if len(parts) < 2 || parts[0] == "" {
		http.Error(w, "Media ID required", http.StatusBadRequest)
		return
	}
	id := parts[0]

	// Look up the media item to get its JellyfinID
	media, found := h.syncEngine.GetMediaByID(id)
	if !found {
		http.Error(w, "Media not found", http.StatusNotFound)
		return
	}

	if media.JellyfinID == "" {
		http.Error(w, "No Jellyfin ID for this media", http.StatusNotFound)
		return
	}

	// Get the Jellyfin client
	jellyfinClient := h.syncEngine.GetJellyfinClient()
	if jellyfinClient == nil {
		http.Error(w, "Jellyfin integration not configured", http.StatusServiceUnavailable)
		return
	}

	// Parse optional query parameters
	imageType := r.URL.Query().Get("type")
	if imageType == "" {
		imageType = "Primary"
	}
	// Validate image type to prevent path traversal
	if imageType != "Primary" && imageType != "Backdrop" {
		http.Error(w, "Invalid image type", http.StatusBadRequest)
		return
	}

	maxWidth := 0
	if mw := r.URL.Query().Get("maxWidth"); mw != "" {
		if v, err := strconv.Atoi(mw); err == nil && v > 0 && v <= 1920 {
			maxWidth = v
		}
	}

	quality := 0
	if q := r.URL.Query().Get("quality"); q != "" {
		if v, err := strconv.Atoi(q); err == nil && v > 0 && v <= 100 {
			quality = v
		}
	}

	// Proxy the image from Jellyfin
	body, contentType, err := jellyfinClient.ProxyImage(r.Context(), media.JellyfinID, imageType, maxWidth, quality)
	if err != nil {
		log.Debug().Err(err).Str("media_id", id).Str("jellyfin_id", media.JellyfinID).Msg("Failed to proxy image")
		http.Error(w, "Image not available", http.StatusNotFound)
		return
	}
	defer body.Close()

	// Set caching headers (images rarely change)
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=86400") // 24 hours
	w.WriteHeader(http.StatusOK)

	if _, err := io.Copy(w, body); err != nil {
		log.Debug().Err(err).Str("media_id", id).Msg("Error streaming image to client")
	}
}

// AddManualLeavingSoon handles POST /api/media/{id}/manual-leaving-soon
func (h *MediaHandler) AddManualLeavingSoon(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	path := strings.TrimPrefix(r.URL.Path, "/api/media/")
	parts := strings.Split(path, "/")
	if len(parts) < 2 || parts[0] == "" {
		http.Error(w, "Media ID required", http.StatusBadRequest)
		return
	}
	id := parts[0]

	if err := h.syncEngine.AddManualLeavingSoon(ctx, id); err != nil {
		log.Error().Err(err).Str("media_id", id).Msg("Failed to add manual leaving soon flag")
		// Return 409 Conflict when item is protected
		if strings.HasPrefix(err.Error(), "conflict:") {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Manual leaving soon flag added",
	})
}

// RemoveManualLeavingSoon handles DELETE /api/media/{id}/manual-leaving-soon
func (h *MediaHandler) RemoveManualLeavingSoon(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	path := strings.TrimPrefix(r.URL.Path, "/api/media/")
	parts := strings.Split(path, "/")
	if len(parts) < 2 || parts[0] == "" {
		http.Error(w, "Media ID required", http.StatusBadRequest)
		return
	}
	id := parts[0]

	if err := h.syncEngine.RemoveManualLeavingSoon(ctx, id); err != nil {
		log.Error().Err(err).Str("media_id", id).Msg("Failed to remove manual leaving soon flag")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Manual leaving soon flag removed",
	})
}

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

// ListUnmatched handles GET /api/media/unmatched
func (h *MediaHandler) ListUnmatched(w http.ResponseWriter, r *http.Request) {
	media := h.syncEngine.GetMediaList()

	// Filter items with Jellyfin matching issues
	var unmatched []models.Media
	for _, item := range media {
		if item.JellyfinMatchStatus == "not_found" || item.JellyfinMatchStatus == "metadata_mismatch" {
			unmatched = append(unmatched, item)
		}
	}

	// Sort by status (mismatches first, then not_found) and then by title
	for i := 0; i < len(unmatched); i++ {
		for j := i + 1; j < len(unmatched); j++ {
			swap := false

			// Prioritize metadata_mismatch over not_found
			if unmatched[i].JellyfinMatchStatus == "not_found" && unmatched[j].JellyfinMatchStatus == "metadata_mismatch" {
				swap = true
			} else if unmatched[i].JellyfinMatchStatus == unmatched[j].JellyfinMatchStatus {
				// Same status, sort by title
				swap = unmatched[i].Title > unmatched[j].Title
			}

			if swap {
				unmatched[i], unmatched[j] = unmatched[j], unmatched[i]
			}
		}
	}

	log.Debug().
		Int("total", len(unmatched)).
		Msg("Listing unmatched media items")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"items": unmatched,
		"total": len(unmatched),
	})
}
