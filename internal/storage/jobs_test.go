package storage

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewJobsFile(t *testing.T) {
	t.Run("creates new jobs file", func(t *testing.T) {
		tmpDir := t.TempDir()

		jf, err := NewJobsFile(tmpDir, 100)

		require.NoError(t, err)
		assert.NotNil(t, jf)
		assert.Equal(t, "1.0", jf.Version)
		assert.NotNil(t, jf.Jobs)
		assert.Empty(t, jf.Jobs)
		assert.Equal(t, 100, jf.maxJobs)

		// Verify file path was set
		filePath := filepath.Join(tmpDir, "jobs.json")
		assert.Equal(t, filePath, jf.filePath)
	})

	t.Run("uses default maxJobs when 0 provided", func(t *testing.T) {
		tmpDir := t.TempDir()

		jf, err := NewJobsFile(tmpDir, 0)

		require.NoError(t, err)
		assert.Equal(t, 100, jf.maxJobs)
	})

	t.Run("loads existing jobs file", func(t *testing.T) {
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "jobs.json")

		// Create existing file with data
		testData := `{
  "version": "1.0",
  "jobs": [
    {
      "id": "job-1",
      "type": "full_sync",
      "status": "completed",
      "started_at": "2024-01-01T00:00:00Z",
      "completed_at": "2024-01-01T00:05:00Z",
      "duration_ms": 300000,
      "summary": {
        "movies": 100,
        "tv_shows": 50
      }
    }
  ]
}`
		err := os.WriteFile(filePath, []byte(testData), 0644)
		require.NoError(t, err)

		// Load the file
		jf, err := NewJobsFile(tmpDir, 100)

		require.NoError(t, err)
		assert.Len(t, jf.Jobs, 1)
		assert.Equal(t, "job-1", jf.Jobs[0].ID)
		assert.Equal(t, JobTypeFullSync, jf.Jobs[0].Type)
		assert.NotNil(t, jf.Jobs[0].CompletedAt)
		assert.True(t, jf.Jobs[0].CompletedAt.After(jf.Jobs[0].StartedAt))
	})

	t.Run("handles corrupted file gracefully", func(t *testing.T) {
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "jobs.json")

		// Write invalid JSON
		err := os.WriteFile(filePath, []byte("invalid json"), 0644)
		require.NoError(t, err)

		// Should still create file successfully, but with empty jobs
		jf, err := NewJobsFile(tmpDir, 100)

		require.NoError(t, err)
		assert.Empty(t, jf.Jobs)
	})
}

func TestJobsFile_Add(t *testing.T) {
	t.Run("adds job successfully", func(t *testing.T) {
		tmpDir := t.TempDir()
		jf, err := NewJobsFile(tmpDir, 100)
		require.NoError(t, err)

		job := Job{
			ID:        "job-1",
			Type:      JobTypeFullSync,
			Status:    JobStatusRunning,
			StartedAt: time.Now(),
			Summary:   make(map[string]any),
		}

		err = jf.Add(job)

		require.NoError(t, err)
		assert.Len(t, jf.Jobs, 1)
		assert.Equal(t, "job-1", jf.Jobs[0].ID)
	})

	t.Run("prepends new jobs (most recent first)", func(t *testing.T) {
		tmpDir := t.TempDir()
		jf, err := NewJobsFile(tmpDir, 100)
		require.NoError(t, err)

		job1 := Job{ID: "job-1", Type: JobTypeFullSync, Status: JobStatusRunning, StartedAt: time.Now()}
		job2 := Job{ID: "job-2", Type: JobTypeIncrementalSync, Status: JobStatusRunning, StartedAt: time.Now()}

		jf.Add(job1)
		jf.Add(job2)

		assert.Len(t, jf.Jobs, 2)
		assert.Equal(t, "job-2", jf.Jobs[0].ID) // Most recent first
		assert.Equal(t, "job-1", jf.Jobs[1].ID)
	})

	t.Run("respects maxJobs limit", func(t *testing.T) {
		tmpDir := t.TempDir()
		jf, err := NewJobsFile(tmpDir, 5) // Small limit for testing
		require.NoError(t, err)

		// Add more jobs than the limit
		for i := 0; i < 10; i++ {
			job := Job{
				ID:        string(rune('a' + i)),
				Type:      JobTypeFullSync,
				Status:    JobStatusCompleted,
				StartedAt: time.Now(),
			}
			err = jf.Add(job)
			require.NoError(t, err)
		}

		// Should only keep the last 5 jobs
		assert.Len(t, jf.Jobs, 5)
		// Most recent should be 'j' (the last one added)
		assert.Equal(t, "j", jf.Jobs[0].ID)
		// Oldest kept should be 'f'
		assert.Equal(t, "f", jf.Jobs[4].ID)
	})

	t.Run("persists to disk", func(t *testing.T) {
		tmpDir := t.TempDir()
		jf, err := NewJobsFile(tmpDir, 100)
		require.NoError(t, err)

		job := Job{
			ID:        "job-1",
			Type:      JobTypeFullSync,
			Status:    JobStatusCompleted,
			StartedAt: time.Now(),
		}

		err = jf.Add(job)
		require.NoError(t, err)

		// Load a new instance and verify persistence
		jf2, err := NewJobsFile(tmpDir, 100)
		require.NoError(t, err)

		assert.Len(t, jf2.Jobs, 1)
		assert.Equal(t, "job-1", jf2.Jobs[0].ID)
	})
}

func TestJobsFile_Update(t *testing.T) {
	t.Run("updates existing job", func(t *testing.T) {
		tmpDir := t.TempDir()
		jf, err := NewJobsFile(tmpDir, 100)
		require.NoError(t, err)

		// Add initial job
		job := Job{
			ID:        "job-1",
			Type:      JobTypeFullSync,
			Status:    JobStatusRunning,
			StartedAt: time.Now(),
		}
		jf.Add(job)

		// Update job
		completedAt := time.Now()
		job.Status = JobStatusCompleted
		job.CompletedAt = &completedAt
		job.DurationMs = 5000

		err = jf.Update(job)

		require.NoError(t, err)
		assert.Equal(t, JobStatusCompleted, jf.Jobs[0].Status)
		assert.NotNil(t, jf.Jobs[0].CompletedAt)
		assert.Equal(t, int64(5000), jf.Jobs[0].DurationMs)
	})

	t.Run("updating non-existent job is safe", func(t *testing.T) {
		tmpDir := t.TempDir()
		jf, err := NewJobsFile(tmpDir, 100)
		require.NoError(t, err)

		job := Job{
			ID:     "non-existent",
			Status: JobStatusCompleted,
		}

		err = jf.Update(job)

		require.NoError(t, err)
		assert.Empty(t, jf.Jobs)
	})

	t.Run("persists update to disk", func(t *testing.T) {
		tmpDir := t.TempDir()
		jf, err := NewJobsFile(tmpDir, 100)
		require.NoError(t, err)

		// Add and update job
		job := Job{ID: "job-1", Status: JobStatusRunning, StartedAt: time.Now()}
		jf.Add(job)
		job.Status = JobStatusCompleted
		jf.Update(job)

		// Load a new instance and verify update was persisted
		jf2, err := NewJobsFile(tmpDir, 100)
		require.NoError(t, err)

		assert.Equal(t, JobStatusCompleted, jf2.Jobs[0].Status)
	})
}

func TestJobsFile_Get(t *testing.T) {
	t.Run("retrieves existing job", func(t *testing.T) {
		tmpDir := t.TempDir()
		jf, err := NewJobsFile(tmpDir, 100)
		require.NoError(t, err)

		job := Job{
			ID:        "job-1",
			Type:      JobTypeFullSync,
			Status:    JobStatusCompleted,
			StartedAt: time.Now(),
		}
		jf.Add(job)

		retrieved, found := jf.Get("job-1")

		assert.True(t, found)
		assert.Equal(t, "job-1", retrieved.ID)
		assert.Equal(t, JobTypeFullSync, retrieved.Type)
	})

	t.Run("returns false for non-existent job", func(t *testing.T) {
		tmpDir := t.TempDir()
		jf, err := NewJobsFile(tmpDir, 100)
		require.NoError(t, err)

		_, found := jf.Get("non-existent")

		assert.False(t, found)
	})
}

func TestJobsFile_GetAll(t *testing.T) {
	t.Run("returns all jobs", func(t *testing.T) {
		tmpDir := t.TempDir()
		jf, err := NewJobsFile(tmpDir, 100)
		require.NoError(t, err)

		for i := 0; i < 5; i++ {
			job := Job{
				ID:        string(rune('a' + i)),
				Status:    JobStatusCompleted,
				StartedAt: time.Now(),
			}
			jf.Add(job)
		}

		jobs := jf.GetAll()

		assert.Len(t, jobs, 5)
	})

	t.Run("returns empty slice when no jobs", func(t *testing.T) {
		tmpDir := t.TempDir()
		jf, err := NewJobsFile(tmpDir, 100)
		require.NoError(t, err)

		jobs := jf.GetAll()

		assert.Empty(t, jobs)
		assert.NotNil(t, jobs)
	})

	t.Run("returns copy not reference", func(t *testing.T) {
		tmpDir := t.TempDir()
		jf, err := NewJobsFile(tmpDir, 100)
		require.NoError(t, err)

		job := Job{ID: "job-1", Status: JobStatusCompleted, StartedAt: time.Now()}
		jf.Add(job)

		jobs := jf.GetAll()
		jobs[0].Status = JobStatusFailed // Modify the copy

		// Original should be unchanged
		assert.Equal(t, JobStatusCompleted, jf.Jobs[0].Status)
	})
}

func TestJobsFile_GetRecent(t *testing.T) {
	t.Run("returns N most recent jobs", func(t *testing.T) {
		tmpDir := t.TempDir()
		jf, err := NewJobsFile(tmpDir, 100)
		require.NoError(t, err)

		for i := 0; i < 10; i++ {
			job := Job{
				ID:        string(rune('a' + i)),
				Status:    JobStatusCompleted,
				StartedAt: time.Now(),
			}
			jf.Add(job)
		}

		jobs := jf.GetRecent(3)

		assert.Len(t, jobs, 3)
		// Should be most recent (last added)
		assert.Equal(t, "j", jobs[0].ID)
		assert.Equal(t, "i", jobs[1].ID)
		assert.Equal(t, "h", jobs[2].ID)
	})

	t.Run("handles N greater than available jobs", func(t *testing.T) {
		tmpDir := t.TempDir()
		jf, err := NewJobsFile(tmpDir, 100)
		require.NoError(t, err)

		job := Job{ID: "job-1", Status: JobStatusCompleted, StartedAt: time.Now()}
		jf.Add(job)

		jobs := jf.GetRecent(10)

		assert.Len(t, jobs, 1)
	})

	t.Run("returns empty slice when no jobs", func(t *testing.T) {
		tmpDir := t.TempDir()
		jf, err := NewJobsFile(tmpDir, 100)
		require.NoError(t, err)

		jobs := jf.GetRecent(5)

		assert.Empty(t, jobs)
		assert.NotNil(t, jobs)
	})
}

func TestJobsFile_GetLatest(t *testing.T) {
	t.Run("returns most recent job", func(t *testing.T) {
		tmpDir := t.TempDir()
		jf, err := NewJobsFile(tmpDir, 100)
		require.NoError(t, err)

		job1 := Job{ID: "job-1", Status: JobStatusCompleted, StartedAt: time.Now()}
		job2 := Job{ID: "job-2", Status: JobStatusCompleted, StartedAt: time.Now()}
		jf.Add(job1)
		jf.Add(job2)

		latest, found := jf.GetLatest()

		assert.True(t, found)
		assert.Equal(t, "job-2", latest.ID)
	})

	t.Run("returns false when no jobs", func(t *testing.T) {
		tmpDir := t.TempDir()
		jf, err := NewJobsFile(tmpDir, 100)
		require.NoError(t, err)

		_, found := jf.GetLatest()

		assert.False(t, found)
	})
}

func TestJobsFile_JobSummary(t *testing.T) {
	t.Run("handles job summary with various types", func(t *testing.T) {
		tmpDir := t.TempDir()
		jf, err := NewJobsFile(tmpDir, 100)
		require.NoError(t, err)

		job := Job{
			ID:        "job-1",
			Type:      JobTypeFullSync,
			Status:    JobStatusCompleted,
			StartedAt: time.Now(),
			Summary: map[string]any{
				"movies":      100,
				"tv_shows":    50,
				"total_size":  int64(1024 * 1024 * 1024),
				"message":     "sync completed",
				"has_errors":  false,
				"error_count": 0,
			},
		}

		err = jf.Add(job)
		require.NoError(t, err)

		// Load and verify
		jf2, err := NewJobsFile(tmpDir, 100)
		require.NoError(t, err)

		retrieved, found := jf2.Get("job-1")
		require.True(t, found)
		assert.NotNil(t, retrieved.Summary)
		assert.Equal(t, float64(100), retrieved.Summary["movies"]) // JSON numbers become float64
		assert.Equal(t, "sync completed", retrieved.Summary["message"])
	})
}

func TestJobsFile_ConcurrentAccess(t *testing.T) {
	t.Run("handles concurrent reads and writes", func(t *testing.T) {
		tmpDir := t.TempDir()
		jf, err := NewJobsFile(tmpDir, 100)
		require.NoError(t, err)

		// Add initial jobs
		for i := 0; i < 10; i++ {
			job := Job{
				ID:        string(rune('a' + i)),
				Status:    JobStatusCompleted,
				StartedAt: time.Now(),
			}
			jf.Add(job)
		}

		done := make(chan bool, 6)

		// Concurrent reads
		for i := 0; i < 3; i++ {
			go func() {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("Panic during concurrent read: %v", r)
					}
					done <- true
				}()

				for j := 0; j < 100; j++ {
					_ = jf.GetAll()
					_, _ = jf.Get("a")
					_, _ = jf.GetLatest()
					_ = jf.GetRecent(5)
				}
			}()
		}

		// Concurrent writes
		for i := 0; i < 3; i++ {
			go func(id int) {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("Panic during concurrent write: %v", r)
					}
					done <- true
				}()

				for j := 0; j < 10; j++ {
					job := Job{
						ID:        string(rune('k' + id)),
						Status:    JobStatusRunning,
						StartedAt: time.Now(),
					}
					_ = jf.Add(job)

					// Update the job
					completedAt := time.Now()
					job.Status = JobStatusCompleted
					job.CompletedAt = &completedAt
					_ = jf.Update(job)
				}
			}(i)
		}

		// Wait for all goroutines
		for i := 0; i < 6; i++ {
			<-done
		}
	})
}
