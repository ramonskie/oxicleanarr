package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/ramonskie/oxicleanarr/internal/storage"
)

// JobsHandler handles job history requests
type JobsHandler struct {
	jobs *storage.JobsFile
}

// NewJobsHandler creates a new JobsHandler
func NewJobsHandler(jobs *storage.JobsFile) *JobsHandler {
	return &JobsHandler{
		jobs: jobs,
	}
}

// ListJobs handles GET /api/jobs
func (h *JobsHandler) ListJobs(w http.ResponseWriter, r *http.Request) {
	// Get all jobs or recent N jobs
	jobs := h.jobs.GetAll()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"jobs":  jobs,
		"total": len(jobs),
	})
}

// GetJob handles GET /api/jobs/{id}
func (h *JobsHandler) GetJob(w http.ResponseWriter, r *http.Request) {
	// Extract ID from path
	path := strings.TrimPrefix(r.URL.Path, "/api/jobs/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "Job ID required", http.StatusBadRequest)
		return
	}
	id := parts[0]

	job, found := h.jobs.Get(id)
	if !found {
		http.Error(w, "Job not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(job)
}

// GetLatestJob handles GET /api/jobs/latest
func (h *JobsHandler) GetLatestJob(w http.ResponseWriter, r *http.Request) {
	job, found := h.jobs.GetLatest()
	if !found {
		http.Error(w, "No jobs found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(job)
}
