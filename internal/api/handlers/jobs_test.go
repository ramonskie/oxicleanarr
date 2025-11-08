package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ramonskie/oxicleanarr/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJobsHandler_ListJobs(t *testing.T) {
	t.Run("returns empty list when no jobs", func(t *testing.T) {
		tmpDir := t.TempDir()
		jobs, err := storage.NewJobsFile(tmpDir, 50)
		require.NoError(t, err)

		handler := NewJobsHandler(jobs)

		req := httptest.NewRequest(http.MethodGet, "/api/jobs", nil)
		w := httptest.NewRecorder()

		handler.ListJobs(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

		var response map[string]interface{}
		err = json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)

		assert.Equal(t, float64(0), response["total"])
		jobsList := response["jobs"].([]interface{})
		assert.Empty(t, jobsList)
	})

	t.Run("returns all jobs", func(t *testing.T) {
		tmpDir := t.TempDir()
		jobs, err := storage.NewJobsFile(tmpDir, 50)
		require.NoError(t, err)

		// Add test jobs
		now := time.Now()
		for i := 1; i <= 3; i++ {
			job := storage.Job{
				ID:          "job-" + string(rune('0'+i)),
				Type:        storage.JobTypeFullSync,
				Status:      storage.JobStatusCompleted,
				StartedAt:   now.Add(-time.Duration(i) * time.Hour),
				CompletedAt: &now,
				Summary:     map[string]any{"movies": i * 10},
			}
			jobs.Add(job)
		}

		handler := NewJobsHandler(jobs)

		req := httptest.NewRequest(http.MethodGet, "/api/jobs", nil)
		w := httptest.NewRecorder()

		handler.ListJobs(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err = json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)

		assert.Equal(t, float64(3), response["total"])
		jobsList := response["jobs"].([]interface{})
		assert.Len(t, jobsList, 3)
	})
}

func TestJobsHandler_GetJob(t *testing.T) {
	t.Run("returns job by ID", func(t *testing.T) {
		tmpDir := t.TempDir()
		jobs, err := storage.NewJobsFile(tmpDir, 50)
		require.NoError(t, err)

		// Add test job
		now := time.Now()
		testJob := storage.Job{
			ID:          "test-job-123",
			Type:        storage.JobTypeFullSync,
			Status:      storage.JobStatusCompleted,
			StartedAt:   now,
			CompletedAt: &now,
			DurationMs:  1234,
			Summary:     map[string]any{"movies": 50, "tv_shows": 30},
		}
		jobs.Add(testJob)

		handler := NewJobsHandler(jobs)

		req := httptest.NewRequest(http.MethodGet, "/api/jobs/test-job-123", nil)
		w := httptest.NewRecorder()

		handler.GetJob(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

		var response storage.Job
		err = json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)

		assert.Equal(t, "test-job-123", response.ID)
		assert.Equal(t, storage.JobTypeFullSync, response.Type)
		assert.Equal(t, storage.JobStatusCompleted, response.Status)
		assert.Equal(t, int64(1234), response.DurationMs)
	})

	t.Run("returns 404 for non-existent job", func(t *testing.T) {
		tmpDir := t.TempDir()
		jobs, err := storage.NewJobsFile(tmpDir, 50)
		require.NoError(t, err)

		handler := NewJobsHandler(jobs)

		req := httptest.NewRequest(http.MethodGet, "/api/jobs/non-existent", nil)
		w := httptest.NewRecorder()

		handler.GetJob(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("returns 400 for missing job ID", func(t *testing.T) {
		tmpDir := t.TempDir()
		jobs, err := storage.NewJobsFile(tmpDir, 50)
		require.NoError(t, err)

		handler := NewJobsHandler(jobs)

		req := httptest.NewRequest(http.MethodGet, "/api/jobs/", nil)
		w := httptest.NewRecorder()

		handler.GetJob(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestJobsHandler_GetLatestJob(t *testing.T) {
	t.Run("returns latest job", func(t *testing.T) {
		tmpDir := t.TempDir()
		jobs, err := storage.NewJobsFile(tmpDir, 50)
		require.NoError(t, err)

		// Add test jobs
		now := time.Now()
		job1 := storage.Job{
			ID:        "job-1",
			Type:      storage.JobTypeFullSync,
			Status:    storage.JobStatusCompleted,
			StartedAt: now.Add(-2 * time.Hour),
		}
		job2 := storage.Job{
			ID:        "job-2",
			Type:      storage.JobTypeIncrementalSync,
			Status:    storage.JobStatusCompleted,
			StartedAt: now.Add(-1 * time.Hour), // Most recent
		}

		jobs.Add(job1)
		jobs.Add(job2) // This should be the latest

		handler := NewJobsHandler(jobs)

		req := httptest.NewRequest(http.MethodGet, "/api/jobs/latest", nil)
		w := httptest.NewRecorder()

		handler.GetLatestJob(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response storage.Job
		err = json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)

		assert.Equal(t, "job-2", response.ID)
	})

	t.Run("returns 404 when no jobs", func(t *testing.T) {
		tmpDir := t.TempDir()
		jobs, err := storage.NewJobsFile(tmpDir, 50)
		require.NoError(t, err)

		handler := NewJobsHandler(jobs)

		req := httptest.NewRequest(http.MethodGet, "/api/jobs/latest", nil)
		w := httptest.NewRecorder()

		handler.GetLatestJob(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}
