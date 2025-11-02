package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ramonskie/prunarr/internal/models"
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
		movies := response["movies"].([]interface{})
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
		shows := response["shows"].([]interface{})
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
			DeleteAfter:  now.Add(7 * 24 * time.Hour),
		}

		// Media not leaving (0 or negative days)
		engine.GetMediaLibrary()["movie-2"] = models.Media{
			ID:           "movie-2",
			Type:         models.MediaTypeMovie,
			Title:        "Not leaving",
			DaysUntilDue: 0,
		}

		// Excluded media (should not appear)
		engine.GetMediaLibrary()["movie-3"] = models.Media{
			ID:           "movie-3",
			Type:         models.MediaTypeMovie,
			Title:        "Excluded but leaving",
			DaysUntilDue: 5,
			IsExcluded:   true,
		}

		req := httptest.NewRequest(http.MethodGet, "/api/media/leaving-soon", nil)
		w := httptest.NewRecorder()

		handler.ListLeavingSoon(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)

		assert.Equal(t, float64(1), response["total"])
		media := response["media"].([]interface{})
		assert.Len(t, media, 1)
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
