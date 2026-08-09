package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// JobStatus represents the status of a sync job
type JobStatus string

const (
	JobStatusPending   JobStatus = "pending"
	JobStatusRunning   JobStatus = "running"
	JobStatusCompleted JobStatus = "completed"
	JobStatusFailed    JobStatus = "failed"
)

// JobType represents the type of sync job
type JobType string

const (
	JobTypeFullSync        JobType = "full_sync"
	JobTypeIncrementalSync JobType = "incremental_sync"
)

// Job represents a sync job
type Job struct {
	ID          string         `json:"id"`
	Type        JobType        `json:"type"`
	Status      JobStatus      `json:"status"`
	StartedAt   time.Time      `json:"started_at"`
	CompletedAt *time.Time     `json:"completed_at,omitempty"`
	DurationMs  int64          `json:"duration_ms"`
	Summary     map[string]any `json:"summary,omitempty"`
	Error       string         `json:"error,omitempty"`
}

// JobsFile represents the jobs.json structure
type JobsFile struct {
	Version  string `json:"version"`
	Jobs     []Job  `json:"jobs"`
	mu       sync.RWMutex
	filePath string
	maxJobs  int
}

// NewJobsFile creates or loads a jobs file
func NewJobsFile(dataPath string, maxJobs int) (*JobsFile, error) {
	filePath := filepath.Join(dataPath, "jobs.json")

	// Create data directory if it doesn't exist
	if err := os.MkdirAll(dataPath, 0755); err != nil {
		return nil, err
	}

	if maxJobs == 0 {
		maxJobs = 100 // Default to keeping last 100 jobs
	}

	jf := &JobsFile{
		Version:  "1.0",
		Jobs:     make([]Job, 0),
		filePath: filePath,
		maxJobs:  maxJobs,
	}

	// Try to load existing file
	if _, err := os.Stat(filePath); err == nil {
		if err := jf.load(); err != nil {
			// Preserve the corrupt file for manual recovery instead of
			// silently starting fresh and overwriting it on the next save.
			if backup, backupErr := backupCorruptFile(filePath); backupErr != nil {
				log.Error().Err(err).Err(backupErr).
					Msg("Failed to load jobs file; corrupt backup also failed, starting fresh")
			} else {
				log.Error().Err(err).Str("backup", backup).
					Msg("Failed to load jobs file; corrupt file preserved, starting fresh")
			}
		}
	}

	return jf, nil
}

// Add adds a new job to the file
func (jf *JobsFile) Add(job Job) error {
	jf.mu.Lock()
	defer jf.mu.Unlock()

	// Prepend new job (most recent first), keeping only maxJobs
	next := make([]Job, 0, len(jf.Jobs)+1)
	next = append(next, job)
	next = append(next, jf.Jobs...)
	if len(next) > jf.maxJobs {
		next = next[:jf.maxJobs]
	}

	if err := jf.persist(next); err != nil {
		return err
	}

	jf.Jobs = next
	return nil
}

// Update updates an existing job
func (jf *JobsFile) Update(job Job) error {
	jf.mu.Lock()
	defer jf.mu.Unlock()

	next := make([]Job, len(jf.Jobs))
	copy(next, jf.Jobs)

	updated := false
	for i := range next {
		if next[i].ID == job.ID {
			next[i] = job
			updated = true
			break
		}
	}

	if !updated {
		return nil
	}

	if err := jf.persist(next); err != nil {
		return err
	}

	jf.Jobs = next
	return nil
}

// Get retrieves a job by ID
func (jf *JobsFile) Get(id string) (Job, bool) {
	jf.mu.RLock()
	defer jf.mu.RUnlock()

	for _, job := range jf.Jobs {
		if job.ID == id {
			return job, true
		}
	}
	return Job{}, false
}

// GetAll returns all jobs
func (jf *JobsFile) GetAll() []Job {
	jf.mu.RLock()
	defer jf.mu.RUnlock()

	jobs := make([]Job, len(jf.Jobs))
	copy(jobs, jf.Jobs)
	return jobs
}

// GetRecent returns the N most recent jobs
func (jf *JobsFile) GetRecent(n int) []Job {
	jf.mu.RLock()
	defer jf.mu.RUnlock()

	if n > len(jf.Jobs) {
		n = len(jf.Jobs)
	}

	jobs := make([]Job, n)
	copy(jobs, jf.Jobs[:n])
	return jobs
}

// GetLatest returns the most recent job
func (jf *JobsFile) GetLatest() (Job, bool) {
	jf.mu.RLock()
	defer jf.mu.RUnlock()

	if len(jf.Jobs) == 0 {
		return Job{}, false
	}

	return jf.Jobs[0], true
}

// load reads the jobs file from disk
func (jf *JobsFile) load() error {
	data, err := os.ReadFile(jf.filePath)
	if err != nil {
		return err
	}

	var temp struct {
		Version string `json:"version"`
		Jobs    []Job  `json:"jobs"`
	}

	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	jf.Version = temp.Version
	jf.Jobs = temp.Jobs

	log.Info().Int("count", len(jf.Jobs)).Msg("Loaded jobs from file")
	return nil
}

// persist atomically writes the given jobs to disk. Callers hold jf.mu.
// A struct constructed without a file path (e.g. in tests) is in-memory only.
func (jf *JobsFile) persist(jobs []Job) error {
	if jf.filePath == "" {
		return nil
	}

	data := struct {
		Version string `json:"version"`
		Jobs    []Job  `json:"jobs"`
	}{
		Version: jf.Version,
		Jobs:    jobs,
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	if err := writeFileAtomic(jf.filePath, jsonData, 0644); err != nil {
		return err
	}

	log.Debug().Int("count", len(jobs)).Msg("Saved jobs to file")
	return nil
}
