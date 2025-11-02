package services

import (
	"testing"
	"time"

	"github.com/ramonskie/prunarr/internal/config"
	"github.com/ramonskie/prunarr/internal/models"
	"github.com/ramonskie/prunarr/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper function to create a mock config
func createMockConfig(movieRetention, tvRetention string, leavingSoonDays int) *config.Config {
	return &config.Config{
		Rules: config.RulesConfig{
			MovieRetention: movieRetention,
			TVRetention:    tvRetention,
		},
		App: config.AppConfig{
			LeavingSoonDays: leavingSoonDays,
		},
	}
}

// Helper function to create a mock exclusions file
func createMockExclusions() *storage.ExclusionsFile {
	return &storage.ExclusionsFile{
		Version: "1.0",
		Items:   make(map[string]storage.ExclusionItem),
	}
}

// Helper function to create a mock media item
func createMockMedia(id string, mediaType models.MediaType, addedDaysAgo, lastWatchedDaysAgo int, isRequested, isExcluded bool) models.Media {
	now := time.Now()
	media := models.Media{
		ID:          id,
		Type:        mediaType,
		Title:       "Test Media " + id,
		Year:        2024,
		AddedAt:     now.AddDate(0, 0, -addedDaysAgo),
		IsRequested: isRequested,
		IsExcluded:  isExcluded,
		FileSize:    1024 * 1024 * 1024, // 1GB
	}

	if lastWatchedDaysAgo >= 0 {
		media.LastWatched = now.AddDate(0, 0, -lastWatchedDaysAgo)
		media.WatchCount = 1
	}

	return media
}

func TestParseDuration(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    time.Duration
		expectError bool
	}{
		{
			name:     "parse days",
			input:    "90d",
			expected: 90 * 24 * time.Hour,
		},
		{
			name:     "parse hours",
			input:    "24h",
			expected: 24 * time.Hour,
		},
		{
			name:     "parse minutes",
			input:    "30m",
			expected: 30 * time.Minute,
		},
		{
			name:     "parse seconds",
			input:    "60s",
			expected: 60 * time.Second,
		},
		{
			name:        "invalid format - no unit",
			input:       "90",
			expectError: true,
		},
		{
			name:        "invalid format - unknown unit",
			input:       "90x",
			expectError: true,
		},
		{
			name:        "empty string",
			input:       "",
			expectError: true,
		},
		{
			name:        "invalid number",
			input:       "abcd",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseDuration(tt.input)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestRulesEngine_EvaluateMedia_Exclusions(t *testing.T) {
	cfg := createMockConfig("90d", "120d", 14)
	exclusions := createMockExclusions()
	engine := NewRulesEngine(cfg, exclusions)

	t.Run("excluded media should not be deleted", func(t *testing.T) {
		// Add exclusion
		exclusions.Add(storage.ExclusionItem{
			ExternalID: "movie-1",
			Reason:     "user requested",
		})

		// Create old unwatched movie that would normally be deleted
		media := createMockMedia("movie-1", models.MediaTypeMovie, 200, -1, false, false)

		shouldDelete, deleteAfter, reason := engine.EvaluateMedia(&media)

		assert.False(t, shouldDelete)
		assert.True(t, deleteAfter.IsZero())
		assert.Equal(t, "excluded", reason)
	})
}

func TestRulesEngine_EvaluateMedia_Requested(t *testing.T) {
	cfg := createMockConfig("90d", "120d", 14)
	exclusions := createMockExclusions()
	engine := NewRulesEngine(cfg, exclusions)

	t.Run("requested media should not be deleted", func(t *testing.T) {
		// Create old unwatched movie that is requested
		media := createMockMedia("movie-1", models.MediaTypeMovie, 200, -1, true, false)

		shouldDelete, deleteAfter, reason := engine.EvaluateMedia(&media)

		assert.False(t, shouldDelete)
		assert.True(t, deleteAfter.IsZero())
		assert.Equal(t, "requested", reason)
	})
}

func TestRulesEngine_EvaluateMedia_MovieRetention(t *testing.T) {
	cfg := createMockConfig("90d", "120d", 14)
	exclusions := createMockExclusions()
	engine := NewRulesEngine(cfg, exclusions)

	t.Run("unwatched movie within retention should not be deleted", func(t *testing.T) {
		media := createMockMedia("movie-1", models.MediaTypeMovie, 30, -1, false, false)

		shouldDelete, deleteAfter, reason := engine.EvaluateMedia(&media)

		assert.False(t, shouldDelete)
		assert.False(t, deleteAfter.IsZero())
		assert.Equal(t, "within retention", reason)

		// Verify delete after is calculated correctly
		expectedDeleteAfter := media.AddedAt.Add(90 * 24 * time.Hour)
		assert.WithinDuration(t, expectedDeleteAfter, deleteAfter, time.Second)
	})

	t.Run("unwatched movie past retention should be deleted", func(t *testing.T) {
		media := createMockMedia("movie-1", models.MediaTypeMovie, 120, -1, false, false)

		shouldDelete, deleteAfter, reason := engine.EvaluateMedia(&media)

		assert.True(t, shouldDelete)
		assert.False(t, deleteAfter.IsZero())
		assert.Contains(t, reason, "retention period expired")
		assert.Contains(t, reason, "90d")
	})

	t.Run("watched movie uses last watched date", func(t *testing.T) {
		// Added 200 days ago but watched 30 days ago
		media := createMockMedia("movie-1", models.MediaTypeMovie, 200, 30, false, false)

		shouldDelete, deleteAfter, reason := engine.EvaluateMedia(&media)

		assert.False(t, shouldDelete)
		assert.Equal(t, "within retention", reason)

		// Delete after should be based on last watched, not added date
		expectedDeleteAfter := media.LastWatched.Add(90 * 24 * time.Hour)
		assert.WithinDuration(t, expectedDeleteAfter, deleteAfter, time.Second)
	})

	t.Run("watched movie past retention should be deleted", func(t *testing.T) {
		// Added 300 days ago and watched 100 days ago
		media := createMockMedia("movie-1", models.MediaTypeMovie, 300, 100, false, false)

		shouldDelete, deleteAfter, reason := engine.EvaluateMedia(&media)

		assert.True(t, shouldDelete)
		assert.False(t, deleteAfter.IsZero())
		assert.Contains(t, reason, "retention period expired")
	})
}

func TestRulesEngine_EvaluateMedia_TVRetention(t *testing.T) {
	cfg := createMockConfig("90d", "120d", 14)
	exclusions := createMockExclusions()
	engine := NewRulesEngine(cfg, exclusions)

	t.Run("unwatched TV show within retention should not be deleted", func(t *testing.T) {
		media := createMockMedia("tv-1", models.MediaTypeTVShow, 60, -1, false, false)

		shouldDelete, deleteAfter, reason := engine.EvaluateMedia(&media)

		assert.False(t, shouldDelete)
		assert.False(t, deleteAfter.IsZero())
		assert.Equal(t, "within retention", reason)

		// Verify delete after is calculated correctly (120d for TV)
		expectedDeleteAfter := media.AddedAt.Add(120 * 24 * time.Hour)
		assert.WithinDuration(t, expectedDeleteAfter, deleteAfter, time.Second)
	})

	t.Run("unwatched TV show past retention should be deleted", func(t *testing.T) {
		media := createMockMedia("tv-1", models.MediaTypeTVShow, 150, -1, false, false)

		shouldDelete, deleteAfter, reason := engine.EvaluateMedia(&media)

		assert.True(t, shouldDelete)
		assert.False(t, deleteAfter.IsZero())
		assert.Contains(t, reason, "retention period expired")
		assert.Contains(t, reason, "120d")
	})

	t.Run("watched TV show uses last watched date", func(t *testing.T) {
		// Added 300 days ago but watched 60 days ago
		media := createMockMedia("tv-1", models.MediaTypeTVShow, 300, 60, false, false)

		shouldDelete, deleteAfter, reason := engine.EvaluateMedia(&media)

		assert.False(t, shouldDelete)
		assert.Equal(t, "within retention", reason)

		// Delete after should be based on last watched
		expectedDeleteAfter := media.LastWatched.Add(120 * 24 * time.Hour)
		assert.WithinDuration(t, expectedDeleteAfter, deleteAfter, time.Second)
	})
}

func TestRulesEngine_GetDeletionCandidates(t *testing.T) {
	cfg := createMockConfig("90d", "120d", 14)
	exclusions := createMockExclusions()
	engine := NewRulesEngine(cfg, exclusions)

	// Create a mix of media items
	mediaList := []models.Media{
		createMockMedia("movie-1", models.MediaTypeMovie, 120, -1, false, false), // Should delete
		createMockMedia("movie-2", models.MediaTypeMovie, 30, -1, false, false),  // Within retention
		createMockMedia("movie-3", models.MediaTypeMovie, 150, -1, true, false),  // Requested
		createMockMedia("tv-1", models.MediaTypeTVShow, 150, -1, false, false),   // Should delete
		createMockMedia("tv-2", models.MediaTypeTVShow, 60, -1, false, false),    // Within retention
	}

	// Add exclusion for one item
	exclusions.Add(storage.ExclusionItem{
		ExternalID: "movie-4",
		Reason:     "test",
	})
	mediaList = append(mediaList, createMockMedia("movie-4", models.MediaTypeMovie, 200, -1, false, false)) // Excluded

	candidates := engine.GetDeletionCandidates(mediaList)

	t.Run("returns correct number of candidates", func(t *testing.T) {
		assert.Len(t, candidates, 2) // movie-1 and tv-1
	})

	t.Run("candidates have correct properties", func(t *testing.T) {
		// Find movie-1 candidate
		var movieCandidate *models.DeletionCandidate
		for i := range candidates {
			if candidates[i].Media.ID == "movie-1" {
				movieCandidate = &candidates[i]
				break
			}
		}

		require.NotNil(t, movieCandidate)
		assert.Contains(t, movieCandidate.Reason, "retention period expired")
		assert.False(t, movieCandidate.RetentionDue.IsZero())
		assert.Greater(t, movieCandidate.DaysOverdue, 0)
		assert.Equal(t, int64(1024*1024*1024), movieCandidate.SizeBytes)
	})

	t.Run("candidate days overdue calculated correctly", func(t *testing.T) {
		// movie-1 was added 120 days ago, retention is 90 days, so 30 days overdue
		var movieCandidate *models.DeletionCandidate
		for i := range candidates {
			if candidates[i].Media.ID == "movie-1" {
				movieCandidate = &candidates[i]
				break
			}
		}

		require.NotNil(t, movieCandidate)
		assert.GreaterOrEqual(t, movieCandidate.DaysOverdue, 29) // Allow for timing variance
		assert.LessOrEqual(t, movieCandidate.DaysOverdue, 31)
	})
}

func TestRulesEngine_GetLeavingSoon(t *testing.T) {
	cfg := createMockConfig("90d", "120d", 14)
	exclusions := createMockExclusions()
	engine := NewRulesEngine(cfg, exclusions)

	now := time.Now()

	// Create media items with various due dates
	mediaList := []models.Media{
		// Movie due in 10 days (should be leaving soon)
		createMockMedia("movie-1", models.MediaTypeMovie, 80, -1, false, false),
		// Movie due in 30 days (not leaving soon)
		createMockMedia("movie-2", models.MediaTypeMovie, 60, -1, false, false),
		// Movie due in 5 days (should be leaving soon)
		createMockMedia("movie-3", models.MediaTypeMovie, 85, -1, false, false),
		// Movie already past due (should not be in leaving soon)
		createMockMedia("movie-4", models.MediaTypeMovie, 120, -1, false, false),
		// TV show due in 12 days (should be leaving soon)
		createMockMedia("tv-1", models.MediaTypeTVShow, 108, -1, false, false),
	}

	leavingSoon := engine.GetLeavingSoon(mediaList)

	t.Run("returns correct number of leaving soon items", func(t *testing.T) {
		assert.GreaterOrEqual(t, len(leavingSoon), 2)
		assert.LessOrEqual(t, len(leavingSoon), 3)
	})

	t.Run("leaving soon items have correct properties", func(t *testing.T) {
		for _, item := range leavingSoon {
			assert.False(t, item.DeleteAfter.IsZero(), "DeleteAfter should be set")
			assert.Greater(t, item.DaysUntilDue, 0, "DaysUntilDue should be positive")
			assert.LessOrEqual(t, item.DaysUntilDue, 14, "DaysUntilDue should be <= threshold")
			assert.True(t, item.DeleteAfter.After(now), "DeleteAfter should be in the future")
		}
	})

	t.Run("does not include already due items", func(t *testing.T) {
		for _, item := range leavingSoon {
			assert.NotEqual(t, "movie-4", item.ID, "Past due items should not be included")
		}
	})

	t.Run("does not include items too far in future", func(t *testing.T) {
		for _, item := range leavingSoon {
			assert.NotEqual(t, "movie-2", item.ID, "Items beyond threshold should not be included")
		}
	})
}

func TestRulesEngine_GetLeavingSoon_CustomThreshold(t *testing.T) {
	cfg := createMockConfig("90d", "120d", 7) // 7 day threshold
	exclusions := createMockExclusions()
	engine := NewRulesEngine(cfg, exclusions)

	mediaList := []models.Media{
		// Movie due in 5 days (should be leaving soon with 7 day threshold)
		createMockMedia("movie-1", models.MediaTypeMovie, 85, -1, false, false),
		// Movie due in 10 days (should NOT be leaving soon with 7 day threshold)
		createMockMedia("movie-2", models.MediaTypeMovie, 80, -1, false, false),
	}

	leavingSoon := engine.GetLeavingSoon(mediaList)

	t.Run("respects custom threshold", func(t *testing.T) {
		// Should only include items within 7 days
		for _, item := range leavingSoon {
			assert.LessOrEqual(t, item.DaysUntilDue, 7)
		}
	})
}

func TestRulesEngine_InvalidRetentionFormat(t *testing.T) {
	cfg := createMockConfig("invalid", "120d", 14)
	exclusions := createMockExclusions()
	engine := NewRulesEngine(cfg, exclusions)

	media := createMockMedia("movie-1", models.MediaTypeMovie, 30, -1, false, false)

	shouldDelete, deleteAfter, reason := engine.EvaluateMedia(&media)

	t.Run("handles invalid retention gracefully", func(t *testing.T) {
		assert.False(t, shouldDelete)
		assert.True(t, deleteAfter.IsZero())
		assert.Equal(t, "invalid retention", reason)
	})
}

func TestRulesEngine_EdgeCases(t *testing.T) {
	cfg := createMockConfig("90d", "120d", 14)
	exclusions := createMockExclusions()
	engine := NewRulesEngine(cfg, exclusions)

	t.Run("media added exactly at retention boundary", func(t *testing.T) {
		media := createMockMedia("movie-1", models.MediaTypeMovie, 90, -1, false, false)

		shouldDelete, deleteAfter, reason := engine.EvaluateMedia(&media)

		// Should be on the boundary - might delete depending on exact timing
		if shouldDelete {
			assert.Contains(t, reason, "retention period expired")
		} else {
			assert.Equal(t, "within retention", reason)
		}
		assert.False(t, deleteAfter.IsZero(), "DeleteAfter should be set")
	})

	t.Run("empty media list", func(t *testing.T) {
		candidates := engine.GetDeletionCandidates([]models.Media{})
		assert.Empty(t, candidates)

		leavingSoon := engine.GetLeavingSoon([]models.Media{})
		assert.Empty(t, leavingSoon)
	})

	t.Run("media with zero time values", func(t *testing.T) {
		media := models.Media{
			ID:          "test-1",
			Type:        models.MediaTypeMovie,
			Title:       "Test",
			AddedAt:     time.Time{}, // Zero time
			LastWatched: time.Time{}, // Zero time
		}

		shouldDelete, deleteAfter, _ := engine.EvaluateMedia(&media)

		// With zero AddedAt, deleteAfter will be 90 days from year 1 (way in the past)
		// So it should be marked for deletion
		assert.True(t, shouldDelete)
		assert.False(t, deleteAfter.IsZero())
	})
}

func TestRulesEngine_ConcurrentAccess(t *testing.T) {
	cfg := createMockConfig("90d", "120d", 14)
	exclusions := createMockExclusions()
	engine := NewRulesEngine(cfg, exclusions)

	mediaList := []models.Media{
		createMockMedia("movie-1", models.MediaTypeMovie, 120, -1, false, false),
		createMockMedia("movie-2", models.MediaTypeMovie, 30, -1, false, false),
		createMockMedia("tv-1", models.MediaTypeTVShow, 150, -1, false, false),
	}

	t.Run("concurrent evaluation should not panic", func(t *testing.T) {
		done := make(chan bool, 3)

		// Run concurrent evaluations
		for i := 0; i < 3; i++ {
			go func() {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("Panic during concurrent evaluation: %v", r)
					}
					done <- true
				}()

				_ = engine.GetDeletionCandidates(mediaList)
				_ = engine.GetLeavingSoon(mediaList)
			}()
		}

		// Wait for all goroutines
		for i := 0; i < 3; i++ {
			<-done
		}
	})
}
