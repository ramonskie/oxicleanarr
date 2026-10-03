package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/ramonskie/oxicleanarr/internal/config"
	"github.com/ramonskie/oxicleanarr/internal/models"
	"github.com/ramonskie/oxicleanarr/internal/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMediaHandler_ListMovies(t *testing.T) {
	t.Run("returns empty list when no movies", func(t *testing.T) {
		engine := newTestSyncEngineForAPI(t)
		handler := NewMediaHandler(engine)

		req := httptest.NewRequest(http.MethodGet, "/api/media/movies", nil)
		w := httptest.NewRecorder()

		handler.ListMovies(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

		var response map[string]interface{}
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)

		assert.Equal(t, float64(0), response["total"])
	})

	t.Run("returns only movies", func(t *testing.T) {
		engine := newTestSyncEngineForAPI(t)
		handler := NewMediaHandler(engine)

		// Add test media
		engine.GetMediaLibrary()["movie-1"] = models.Media{
			ID:    "movie-1",
			Type:  models.MediaTypeMovie,
			Title: "Test Movie 1",
		}
		engine.GetMediaLibrary()["movie-2"] = models.Media{
			ID:    "movie-2",
			Type:  models.MediaTypeMovie,
			Title: "Test Movie 2",
		}
		engine.GetMediaLibrary()["tv-1"] = models.Media{
			ID:    "tv-1",
			Type:  models.MediaTypeTVShow,
			Title: "Test Show",
		}

		req := httptest.NewRequest(http.MethodGet, "/api/media/movies", nil)
		w := httptest.NewRecorder()

		handler.ListMovies(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)

		assert.Equal(t, float64(2), response["total"])
		movies := response["items"].([]interface{})
		assert.Len(t, movies, 2)
	})

	t.Run("filters by leaving_soon status", func(t *testing.T) {
		engine := newTestSyncEngineForAPI(t)
		handler := NewMediaHandler(engine)

		now := time.Now()
		engine.GetMediaLibrary()["movie-1"] = models.Media{
			ID:           "movie-1",
			Type:         models.MediaTypeMovie,
			Title:        "Leaving Soon",
			DaysUntilDue: 7,
			DeleteAfter:  now.Add(7 * 24 * time.Hour),
		}
		engine.GetMediaLibrary()["movie-2"] = models.Media{
			ID:           "movie-2",
			Type:         models.MediaTypeMovie,
			Title:        "Not Leaving",
			DaysUntilDue: 0,
		}

		req := httptest.NewRequest(http.MethodGet, "/api/media/movies?status=leaving_soon", nil)
		w := httptest.NewRecorder()

		handler.ListMovies(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)

		assert.Equal(t, float64(1), response["total"])
	})

	t.Run("filters by excluded status", func(t *testing.T) {
		engine := newTestSyncEngineForAPI(t)
		handler := NewMediaHandler(engine)

		engine.GetMediaLibrary()["movie-1"] = models.Media{
			ID:         "movie-1",
			Type:       models.MediaTypeMovie,
			Title:      "Excluded Movie",
			IsExcluded: true,
		}
		engine.GetMediaLibrary()["movie-2"] = models.Media{
			ID:         "movie-2",
			Type:       models.MediaTypeMovie,
			Title:      "Normal Movie",
			IsExcluded: false,
		}

		req := httptest.NewRequest(http.MethodGet, "/api/media/movies?status=excluded", nil)
		w := httptest.NewRecorder()

		handler.ListMovies(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)

		assert.Equal(t, float64(1), response["total"])
	})
}

func TestMediaHandler_ListShows(t *testing.T) {
	t.Run("returns only TV shows", func(t *testing.T) {
		engine := newTestSyncEngineForAPI(t)
		handler := NewMediaHandler(engine)

		// Add test media
		engine.GetMediaLibrary()["tv-1"] = models.Media{
			ID:    "tv-1",
			Type:  models.MediaTypeTVShow,
			Title: "Test Show 1",
		}
		engine.GetMediaLibrary()["tv-2"] = models.Media{
			ID:    "tv-2",
			Type:  models.MediaTypeTVShow,
			Title: "Test Show 2",
		}
		engine.GetMediaLibrary()["movie-1"] = models.Media{
			ID:    "movie-1",
			Type:  models.MediaTypeMovie,
			Title: "Test Movie",
		}

		req := httptest.NewRequest(http.MethodGet, "/api/media/shows", nil)
		w := httptest.NewRecorder()

		handler.ListShows(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)

		assert.Equal(t, float64(2), response["total"])
		shows := response["items"].([]interface{})
		assert.Len(t, shows, 2)
	})
}

func TestMediaHandler_ListLeavingSoon(t *testing.T) {
	t.Run("returns media leaving soon", func(t *testing.T) {
		engine := newTestSyncEngineForAPI(t)
		handler := NewMediaHandler(engine)

		now := time.Now()

		// Media leaving soon
		engine.GetMediaLibrary()["movie-1"] = models.Media{
			ID:           "movie-1",
			Type:         models.MediaTypeMovie,
			Title:        "Leaving in 7 days",
			DaysUntilDue: 7,
			JellyfinID:   "jellyfin-guid-1",
			DeleteAfter:  now.Add(7 * 24 * time.Hour),
		}

		// Media leaving soon but with no Jellyfin match (should not appear -
		// a Jellyfin leaving-soon library cannot reference it)
		engine.GetMediaLibrary()["movie-2"] = models.Media{
			ID:           "movie-2",
			Type:         models.MediaTypeMovie,
			Title:        "Leaving but unmatched",
			DaysUntilDue: 5,
			DeleteAfter:  now.Add(5 * 24 * time.Hour),
		}

		// Media not leaving (0 or negative days)
		engine.GetMediaLibrary()["movie-3"] = models.Media{
			ID:           "movie-3",
			Type:         models.MediaTypeMovie,
			Title:        "Not leaving",
			DaysUntilDue: 0,
			JellyfinID:   "jellyfin-guid-3",
		}

		// Excluded media (should not appear)
		engine.GetMediaLibrary()["movie-4"] = models.Media{
			ID:           "movie-4",
			Type:         models.MediaTypeMovie,
			Title:        "Excluded but leaving",
			DaysUntilDue: 5,
			IsExcluded:   true,
			JellyfinID:   "jellyfin-guid-4",
		}

		req := httptest.NewRequest(http.MethodGet, "/api/media/leaving-soon", nil)
		w := httptest.NewRecorder()

		handler.ListLeavingSoon(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response models.LeavingSoonResponse
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)

		assert.Len(t, response.Items, 1)
		require.NotEmpty(t, response.Items)
		assert.Equal(t, "jellyfin-guid-1", response.Items[0].MediaServerID)
		assert.Equal(t, "movie", response.Items[0].Type)
		assert.Equal(t, "Leaving in 7 days", response.Items[0].Title)
	})

	t.Run("respects leaving_soon_days threshold", func(t *testing.T) {
		engine := newTestSyncEngineForAPI(t)
		handler := NewMediaHandler(engine)

		// Set leaving_soon_days to 14 days
		origCfg := config.Get()
		cfg := *origCfg
		cfg.App.LeavingSoonDays = 14
		config.SetTestConfig(&cfg)
		// Restore the original config so other tests aren't affected.
		t.Cleanup(func() { config.SetTestConfig(origCfg) })

		now := time.Now()

		// Media within threshold (5 days - should appear)
		engine.GetMediaLibrary()["movie-1"] = models.Media{
			ID:           "movie-1",
			Type:         models.MediaTypeMovie,
			Title:        "Leaving in 5 days",
			DaysUntilDue: 5,
			JellyfinID:   "jellyfin-guid-1",
			DeleteAfter:  now.Add(5 * 24 * time.Hour),
		}

		// Media within threshold (14 days exactly - should appear)
		engine.GetMediaLibrary()["movie-2"] = models.Media{
			ID:           "movie-2",
			Type:         models.MediaTypeMovie,
			Title:        "Leaving in 14 days",
			DaysUntilDue: 14,
			JellyfinID:   "jellyfin-guid-2",
			DeleteAfter:  now.Add(14 * 24 * time.Hour),
		}

		// Media outside threshold (29 days - should NOT appear)
		engine.GetMediaLibrary()["movie-3"] = models.Media{
			ID:           "movie-3",
			Type:         models.MediaTypeMovie,
			Title:        "Leaving in 29 days",
			DaysUntilDue: 29,
			JellyfinID:   "jellyfin-guid-3",
			DeleteAfter:  now.Add(29 * 24 * time.Hour),
		}

		// Media outside threshold (30 days - should NOT appear)
		engine.GetMediaLibrary()["movie-4"] = models.Media{
			ID:           "movie-4",
			Type:         models.MediaTypeMovie,
			Title:        "Leaving in 30 days",
			DaysUntilDue: 30,
			JellyfinID:   "jellyfin-guid-4",
			DeleteAfter:  now.Add(30 * 24 * time.Hour),
		}

		// Media with 0 days (should NOT appear)
		engine.GetMediaLibrary()["movie-5"] = models.Media{
			ID:           "movie-5",
			Type:         models.MediaTypeMovie,
			Title:        "Not scheduled for deletion",
			DaysUntilDue: 0,
			JellyfinID:   "jellyfin-guid-5",
		}

		// Media within threshold but with no Jellyfin match (should NOT appear)
		engine.GetMediaLibrary()["movie-6"] = models.Media{
			ID:           "movie-6",
			Type:         models.MediaTypeMovie,
			Title:        "Leaving but unmatched",
			DaysUntilDue: 3,
			DeleteAfter:  now.Add(3 * 24 * time.Hour),
		}

		req := httptest.NewRequest(http.MethodGet, "/api/media/leaving-soon", nil)
		w := httptest.NewRecorder()

		handler.ListLeavingSoon(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response models.LeavingSoonResponse
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)

		// Should only return items with 5 and 14 days (2 items)
		assert.Len(t, response.Items, 2)

		// Verify the correct items are returned (by deletion date)
		returnedDays := make(map[int]bool)
		for _, item := range response.Items {
			returnedDays[int(item.DeletionDate.Sub(now).Hours()/24)] = true
		}
		assert.True(t, returnedDays[5], "Item with 5 days should be returned")
		assert.True(t, returnedDays[14], "Item with 14 days should be returned")
		assert.False(t, returnedDays[29], "Item with 29 days should NOT be returned")
		assert.False(t, returnedDays[30], "Item with 30 days should NOT be returned")
		assert.False(t, returnedDays[3], "Unmatched item should NOT be returned")
	})
}

func TestMediaHandler_ListLeavingSoonMedia(t *testing.T) {
	t.Run("returns rich media items for the UI", func(t *testing.T) {
		engine := newTestSyncEngineForAPI(t)
		handler := NewMediaHandler(engine)

		now := time.Now()

		// In window, matched
		engine.GetMediaLibrary()["movie-1"] = models.Media{
			ID:           "movie-1",
			Type:         models.MediaTypeMovie,
			Title:        "Leaving in 7 days",
			DaysUntilDue: 7,
			JellyfinID:   "jellyfin-guid-1",
			DeleteAfter:  now.Add(7 * 24 * time.Hour),
		}

		// In window, unmatched (UI still shows it - only the plugin contract drops it)
		engine.GetMediaLibrary()["movie-2"] = models.Media{
			ID:           "movie-2",
			Type:         models.MediaTypeMovie,
			Title:        "Leaving but unmatched",
			DaysUntilDue: 5,
			DeleteAfter:  now.Add(5 * 24 * time.Hour),
		}

		// Excluded (should not appear)
		engine.GetMediaLibrary()["movie-3"] = models.Media{
			ID:           "movie-3",
			Type:         models.MediaTypeMovie,
			Title:        "Excluded",
			DaysUntilDue: 5,
			IsExcluded:   true,
			DeleteAfter:  now.Add(5 * 24 * time.Hour),
		}

		// Not leaving (should not appear)
		engine.GetMediaLibrary()["movie-4"] = models.Media{
			ID:           "movie-4",
			Type:         models.MediaTypeMovie,
			Title:        "Not leaving",
			DaysUntilDue: 0,
		}

		req := httptest.NewRequest(http.MethodGet, "/api/media/leaving-soon/list", nil)
		w := httptest.NewRecorder()

		handler.ListLeavingSoonMedia(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response struct {
			Items []models.Media `json:"items"`
			Total int            `json:"total"`
		}
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)

		assert.Equal(t, 2, response.Total)
		assert.Len(t, response.Items, 2)

		// Sorted by deletion date (earliest first): movie-2 (5d) before movie-1 (7d)
		assert.Equal(t, "movie-2", response.Items[0].ID)
		assert.Equal(t, "movie-1", response.Items[1].ID)
	})
}

func TestMediaHandler_GetMediaItem(t *testing.T) {
	t.Run("returns media by ID", func(t *testing.T) {
		engine := newTestSyncEngineForAPI(t)
		handler := NewMediaHandler(engine)

		// Add test media
		engine.GetMediaLibrary()["movie-123"] = models.Media{
			ID:       "movie-123",
			Type:     models.MediaTypeMovie,
			Title:    "Test Movie",
			Year:     2023,
			RadarrID: 1,
		}

		req := httptest.NewRequest(http.MethodGet, "/api/media/movie-123", nil)
		w := httptest.NewRecorder()

		handler.GetMediaItem(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

		var response models.Media
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)

		assert.Equal(t, "movie-123", response.ID)
		assert.Equal(t, "Test Movie", response.Title)
		assert.Equal(t, 2023, response.Year)
	})

	t.Run("returns 404 for non-existent media", func(t *testing.T) {
		engine := newTestSyncEngineForAPI(t)
		handler := NewMediaHandler(engine)

		req := httptest.NewRequest(http.MethodGet, "/api/media/non-existent", nil)
		w := httptest.NewRecorder()

		handler.GetMediaItem(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("returns 400 for missing media ID", func(t *testing.T) {
		engine := newTestSyncEngineForAPI(t)
		handler := NewMediaHandler(engine)

		req := httptest.NewRequest(http.MethodGet, "/api/media/", nil)
		w := httptest.NewRecorder()

		handler.GetMediaItem(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestMediaHandler_AddExclusion(t *testing.T) {
	t.Run("adds exclusion successfully", func(t *testing.T) {
		engine := newTestSyncEngineForAPI(t)
		handler := NewMediaHandler(engine)

		// Add test media
		engine.GetMediaLibrary()["movie-123"] = models.Media{
			ID:       "movie-123",
			Type:     models.MediaTypeMovie,
			Title:    "Test Movie",
			RadarrID: 1,
		}

		reqBody := map[string]string{
			"reason": "User favorite",
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/api/media/movie-123/exclude", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.AddExclusion(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)

		assert.True(t, response["success"].(bool))
		assert.Equal(t, "Exclusion added", response["message"])

		// Verify exclusion was added
		media, _ := engine.GetMediaByID("movie-123")
		assert.True(t, media.IsExcluded)
	})

	t.Run("handles missing reason gracefully", func(t *testing.T) {
		engine := newTestSyncEngineForAPI(t)
		handler := NewMediaHandler(engine)

		// Add test media
		engine.GetMediaLibrary()["movie-123"] = models.Media{
			ID:       "movie-123",
			Type:     models.MediaTypeMovie,
			Title:    "Test Movie",
			RadarrID: 1,
		}

		req := httptest.NewRequest(http.MethodPost, "/api/media/movie-123/exclude", bytes.NewReader([]byte("{}")))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.AddExclusion(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("returns 500 for non-existent media", func(t *testing.T) {
		engine := newTestSyncEngineForAPI(t)
		handler := NewMediaHandler(engine)

		req := httptest.NewRequest(http.MethodPost, "/api/media/non-existent/exclude", bytes.NewReader([]byte("{}")))
		w := httptest.NewRecorder()

		handler.AddExclusion(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("returns 400 for missing media ID", func(t *testing.T) {
		engine := newTestSyncEngineForAPI(t)
		handler := NewMediaHandler(engine)

		req := httptest.NewRequest(http.MethodPost, "/api/media//exclude", bytes.NewReader([]byte("{}")))
		w := httptest.NewRecorder()

		handler.AddExclusion(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestMediaHandler_RemoveExclusion(t *testing.T) {
	t.Run("removes exclusion successfully", func(t *testing.T) {
		engine := newTestSyncEngineForAPI(t)
		handler := NewMediaHandler(engine)

		// Add test media with exclusion
		engine.GetMediaLibrary()["movie-123"] = models.Media{
			ID:         "movie-123",
			Type:       models.MediaTypeMovie,
			Title:      "Test Movie",
			RadarrID:   1,
			IsExcluded: true,
		}

		req := httptest.NewRequest(http.MethodDelete, "/api/media/movie-123/exclude", nil)
		w := httptest.NewRecorder()

		handler.RemoveExclusion(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)

		assert.True(t, response["success"].(bool))
		assert.Equal(t, "Exclusion removed", response["message"])

		// Verify exclusion was removed
		media, _ := engine.GetMediaByID("movie-123")
		assert.False(t, media.IsExcluded)
	})

	t.Run("returns 500 for non-existent media", func(t *testing.T) {
		engine := newTestSyncEngineForAPI(t)
		handler := NewMediaHandler(engine)

		req := httptest.NewRequest(http.MethodDelete, "/api/media/non-existent/exclude", nil)
		w := httptest.NewRecorder()

		handler.RemoveExclusion(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

func TestMediaHandler_DeleteMedia(t *testing.T) {
	t.Run("deletes media in dry run mode", func(t *testing.T) {
		engine := newTestSyncEngineForAPI(t)
		handler := NewMediaHandler(engine)

		// Add test media
		engine.GetMediaLibrary()["movie-123"] = models.Media{
			ID:       "movie-123",
			Type:     models.MediaTypeMovie,
			Title:    "Test Movie",
			RadarrID: 1,
		}

		req := httptest.NewRequest(http.MethodDelete, "/api/media/movie-123?dry_run=true", nil)
		w := httptest.NewRecorder()

		handler.DeleteMedia(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)

		assert.True(t, response["success"].(bool))
		assert.True(t, response["dry_run"].(bool))
		assert.Contains(t, response["message"], "Dry run")

		// Media should still exist
		_, found := engine.GetMediaByID("movie-123")
		assert.True(t, found)
	})

	t.Run("returns 500 for non-existent media", func(t *testing.T) {
		engine := newTestSyncEngineForAPI(t)
		handler := NewMediaHandler(engine)

		req := httptest.NewRequest(http.MethodDelete, "/api/media/non-existent", nil)
		w := httptest.NewRecorder()

		handler.DeleteMedia(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("returns 400 for missing media ID", func(t *testing.T) {
		engine := newTestSyncEngineForAPI(t)
		handler := NewMediaHandler(engine)

		req := httptest.NewRequest(http.MethodDelete, "/api/media/", nil)
		w := httptest.NewRecorder()

		handler.DeleteMedia(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

// stubMatchService is a scripted matchService for handler tests. It returns the
// configured outcomes without any real arr/Jellyfin I/O, so the tests exercise
// only the handler's status mapping and response shaping. analyzeCalls counts
// invocation of AnalyzeJellyfinMatch so a test can prove a refusal does not
// re-derive the analysis.
type stubMatchService struct {
	analysis     *services.MatchAnalysis
	analyzeErr   error
	analyzeCalls int

	fixResult *services.FixResult
	fixErr    error

	// fixReplaceImages records the replaceImages argument of every
	// FixJellyfinMatch call so a test can assert the handler's body defaulting.
	fixReplaceImages []bool
}

func (s *stubMatchService) AnalyzeJellyfinMatch(_ context.Context, _ string) (*services.MatchAnalysis, error) {
	s.analyzeCalls++
	return s.analysis, s.analyzeErr
}

func (s *stubMatchService) FixJellyfinMatch(_ context.Context, _ string, replaceImages bool) (*services.FixResult, error) {
	s.fixReplaceImages = append(s.fixReplaceImages, replaceImages)
	return s.fixResult, s.fixErr
}

// newMatchTestHandler builds a handler backed by a real sync engine (so the
// media lookup and 404 path are exercised) but with the analysis/fix outcomes
// stubbed.
func newMatchTestHandler(t *testing.T, stub *stubMatchService) *MediaHandler {
	t.Helper()

	engine := newTestSyncEngineForAPI(t)
	engine.GetMediaLibrary()["radarr-306"] = models.Media{
		ID: "radarr-306", Type: models.MediaTypeMovie, Title: "Long Distance", Year: 2024,
	}

	handler := NewMediaHandler(engine)
	handler.matchService = stub
	return handler
}

// newMatchTestRouter mounts the two match endpoints on a chi router so
// chi.URLParam resolves the {id} segment exactly as it does in production.
func newMatchTestRouter(h *MediaHandler) http.Handler {
	r := chi.NewRouter()
	r.Get("/api/media/{id}/match-analysis", h.GetMatchAnalysis)
	r.Post("/api/media/{id}/fix-match", h.FixMatch)
	return r
}

func TestMediaHandler_GetMatchAnalysis(t *testing.T) {
	jellyfinWrong := &services.MatchAnalysis{
		Verdict:    services.VerdictJellyfinWrong,
		Confidence: 0.90,
		Evidence:   []string{"filename matches the arr identity; Jellyfin is the outlier"},
	}

	t.Run("returns the verdict, confidence and evidence", func(t *testing.T) {
		handler := newMatchTestHandler(t, &stubMatchService{analysis: jellyfinWrong})
		router := newMatchTestRouter(handler)

		req := httptest.NewRequest(http.MethodGet, "/api/media/radarr-306/match-analysis", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

		var resp struct {
			Analysis services.MatchAnalysis `json:"analysis"`
			Fixed    bool                   `json:"fixed"`
		}
		require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
		assert.Equal(t, services.VerdictJellyfinWrong, resp.Analysis.Verdict)
		assert.InDelta(t, 0.90, resp.Analysis.Confidence, 1e-9)
		assert.NotEmpty(t, resp.Analysis.Evidence)
		assert.False(t, resp.Fixed, "an analysis request must never report a fix")
	})

	t.Run("returns 404 when the media item is unknown", func(t *testing.T) {
		handler := newMatchTestHandler(t, &stubMatchService{analysis: jellyfinWrong})
		router := newMatchTestRouter(handler)

		req := httptest.NewRequest(http.MethodGet, "/api/media/does-not-exist/match-analysis", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("returns 422 when no Jellyfin item exists", func(t *testing.T) {
		handler := newMatchTestHandler(t, &stubMatchService{
			analyzeErr: fmt.Errorf("analyzing match: %w", services.ErrJellyfinItemNotFound),
		})
		router := newMatchTestRouter(handler)

		req := httptest.NewRequest(http.MethodGet, "/api/media/radarr-306/match-analysis", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
		var resp matchErrorResponse
		require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
		assert.Contains(t, resp.Error, "no Jellyfin item found")
	})
}

func TestMediaHandler_FixMatch(t *testing.T) {
	jellyfinWrong := &services.MatchAnalysis{
		Verdict:    services.VerdictJellyfinWrong,
		Confidence: 0.92,
		Evidence:   []string{"episode titles agree for 5 shared pairs"},
	}

	t.Run("fixes the match and returns the re-identified item", func(t *testing.T) {
		handler := newMatchTestHandler(t, &stubMatchService{
			fixResult: &services.FixResult{
				MediaID:            "radarr-306",
				JellyfinID:         "jf-distant",
				Title:              "Long Distance",
				AppliedProviderIDs: map[string]string{"Tmdb": "605722"},
				Analysis:           *jellyfinWrong,
			},
		})
		router := newMatchTestRouter(handler)

		req := httptest.NewRequest(http.MethodPost, "/api/media/radarr-306/fix-match", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp struct {
			Analysis     services.MatchAnalysis `json:"analysis"`
			Fixed        bool                   `json:"fixed"`
			JellyfinID   string                 `json:"jellyfin_id"`
			MatchedTitle string                 `json:"matched_title"`
			ProviderIDs  map[string]string      `json:"provider_ids"`
		}
		require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
		assert.True(t, resp.Fixed)
		assert.Equal(t, "jf-distant", resp.JellyfinID)
		assert.Equal(t, "Long Distance", resp.MatchedTitle)
		assert.Equal(t, "605722", resp.ProviderIDs["Tmdb"])
		assert.Equal(t, services.VerdictJellyfinWrong, resp.Analysis.Verdict)
	})

	t.Run("refuses with 409 and the carried analysis when the arr is wrong", func(t *testing.T) {
		arrWrong := &services.MatchAnalysis{
			Verdict:    services.VerdictArrWrong,
			Confidence: 0.88,
			Evidence:   []string{"filename matches the Jellyfin identity"},
		}
		stub := &stubMatchService{
			fixErr: &services.MatchRefusalError{
				Err:      services.ErrMatchArrWrong,
				Analysis: arrWrong,
			},
		}
		handler := newMatchTestHandler(t, stub)
		router := newMatchTestRouter(handler)

		req := httptest.NewRequest(http.MethodPost, "/api/media/radarr-306/fix-match", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusConflict, w.Code)
		var resp matchErrorResponse
		require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
		assert.Contains(t, resp.Error, "Sonarr/Radarr")
		require.NotNil(t, resp.Analysis)
		assert.Equal(t, services.VerdictArrWrong, resp.Analysis.Verdict)
		assert.InDelta(t, 0.88, resp.Analysis.Confidence, 1e-9)
		assert.Equal(t, arrWrong.Evidence, resp.Analysis.Evidence)
		assert.Zero(t, stub.analyzeCalls, "a refusal must not re-run adjudication")
	})

	t.Run("returns 409 and the carried analysis when the match is ambiguous", func(t *testing.T) {
		ambiguous := &services.MatchAnalysis{
			Verdict:    services.VerdictAmbiguous,
			Confidence: 0.40,
			Evidence:   []string{"evidence favours neither side"},
		}
		stub := &stubMatchService{
			fixErr: &services.MatchRefusalError{
				Err:      services.ErrMatchAmbiguous,
				Analysis: ambiguous,
			},
		}
		handler := newMatchTestHandler(t, stub)
		router := newMatchTestRouter(handler)

		req := httptest.NewRequest(http.MethodPost, "/api/media/radarr-306/fix-match", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusConflict, w.Code)
		var resp matchErrorResponse
		require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
		require.NotNil(t, resp.Analysis)
		assert.Equal(t, services.VerdictAmbiguous, resp.Analysis.Verdict)
		assert.Zero(t, stub.analyzeCalls, "a refusal must not re-run adjudication")
	})

	t.Run("returns 409 and the carried analysis when remote search finds no match", func(t *testing.T) {
		jellyfinWrong := &services.MatchAnalysis{
			Verdict:    services.VerdictJellyfinWrong,
			Confidence: 0.70,
			Evidence:   []string{"remote search returned no candidate with the arr provider id"},
		}
		stub := &stubMatchService{
			fixErr: &services.MatchRefusalError{
				Err:      services.ErrRemoteSearchNoMatch,
				Analysis: jellyfinWrong,
			},
		}
		handler := newMatchTestHandler(t, stub)
		router := newMatchTestRouter(handler)

		req := httptest.NewRequest(http.MethodPost, "/api/media/radarr-306/fix-match", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusConflict, w.Code)
		var resp matchErrorResponse
		require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
		require.NotNil(t, resp.Analysis)
		assert.Equal(t, services.VerdictJellyfinWrong, resp.Analysis.Verdict)
		assert.Zero(t, stub.analyzeCalls, "a refusal must not re-run adjudication")
	})

	t.Run("omits analysis when the refusal genuinely carries none", func(t *testing.T) {
		stub := &stubMatchService{
			fixErr: &services.MatchRefusalError{Err: services.ErrMatchAmbiguous},
		}
		handler := newMatchTestHandler(t, stub)
		router := newMatchTestRouter(handler)

		req := httptest.NewRequest(http.MethodPost, "/api/media/radarr-306/fix-match", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusConflict, w.Code)
		var resp struct {
			Error    string                  `json:"error"`
			Analysis *services.MatchAnalysis `json:"analysis"`
		}
		require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
		assert.Contains(t, resp.Error, "ambiguous")
		assert.Nil(t, resp.Analysis)
		assert.Zero(t, stub.analyzeCalls, "a refusal must not re-run adjudication")
	})

	t.Run("returns 422 when no Jellyfin item exists", func(t *testing.T) {
		handler := newMatchTestHandler(t, &stubMatchService{
			fixErr: fmt.Errorf("fixing match: %w", services.ErrJellyfinItemNotFound),
		})
		router := newMatchTestRouter(handler)

		req := httptest.NewRequest(http.MethodPost, "/api/media/radarr-306/fix-match", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
		var resp matchErrorResponse
		require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
		assert.Contains(t, resp.Error, "may not be scanned")
	})

	t.Run("returns 404 when the media item is unknown", func(t *testing.T) {
		handler := newMatchTestHandler(t, &stubMatchService{})
		router := newMatchTestRouter(handler)

		req := httptest.NewRequest(http.MethodPost, "/api/media/does-not-exist/fix-match", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("defaults replace_images to true when the body is absent", func(t *testing.T) {
		stub := &stubMatchService{fixResult: &services.FixResult{Analysis: *jellyfinWrong}}
		handler := newMatchTestHandler(t, stub)
		router := newMatchTestRouter(handler)

		req := httptest.NewRequest(http.MethodPost, "/api/media/radarr-306/fix-match", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		require.Equal(t, []bool{true}, stub.fixReplaceImages,
			"an absent body must default replace_images to true")
	})

	t.Run("honours an explicit replace_images false", func(t *testing.T) {
		stub := &stubMatchService{fixResult: &services.FixResult{Analysis: *jellyfinWrong}}
		handler := newMatchTestHandler(t, stub)
		router := newMatchTestRouter(handler)

		req := httptest.NewRequest(http.MethodPost, "/api/media/radarr-306/fix-match",
			bytes.NewReader([]byte(`{"replace_images":false}`)))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		require.Equal(t, []bool{false}, stub.fixReplaceImages,
			"an explicit false must disable image replacement")
	})

	t.Run("honours an explicit replace_images true", func(t *testing.T) {
		stub := &stubMatchService{fixResult: &services.FixResult{Analysis: *jellyfinWrong}}
		handler := newMatchTestHandler(t, stub)
		router := newMatchTestRouter(handler)

		req := httptest.NewRequest(http.MethodPost, "/api/media/radarr-306/fix-match",
			bytes.NewReader([]byte(`{"replace_images":true}`)))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		require.Equal(t, []bool{true}, stub.fixReplaceImages)
	})

	t.Run("rejects a malformed body with 400 before fixing", func(t *testing.T) {
		stub := &stubMatchService{fixResult: &services.FixResult{Analysis: *jellyfinWrong}}
		handler := newMatchTestHandler(t, stub)
		router := newMatchTestRouter(handler)

		req := httptest.NewRequest(http.MethodPost, "/api/media/radarr-306/fix-match",
			bytes.NewReader([]byte(`{"replace_images":`)))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Empty(t, stub.fixReplaceImages,
			"a malformed body must be rejected before any fix is attempted")
	})

	t.Run("rejects an oversized body with 400 before fixing", func(t *testing.T) {
		stub := &stubMatchService{fixResult: &services.FixResult{Analysis: *jellyfinWrong}}
		handler := newMatchTestHandler(t, stub)
		router := newMatchTestRouter(handler)

		// Valid JSON with an ignored field, larger than maxMatchBodyBytes. Without
		// the bound the decoder would discard the unknown field and proceed to a
		// 200, so a 400 here proves the body was actually capped.
		oversized := []byte(`{"padding":"` + strings.Repeat("a", maxMatchBodyBytes) + `"}`)
		require.Greater(t, len(oversized), maxMatchBodyBytes)
		req := httptest.NewRequest(http.MethodPost, "/api/media/radarr-306/fix-match",
			bytes.NewReader(oversized))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Empty(t, stub.fixReplaceImages,
			"an oversized body must be rejected before any fix is attempted")
	})
}
