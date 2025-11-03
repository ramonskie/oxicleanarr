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

// ==================== User-Based Rules Tests ====================

// Helper function to create media with user data
func createMockMediaWithUser(id string, mediaType models.MediaType, addedDaysAgo, lastWatchedDaysAgo int, userID *int, username, email *string) models.Media {
	media := createMockMedia(id, mediaType, addedDaysAgo, lastWatchedDaysAgo, true, false)
	media.RequestedByUserID = userID
	media.RequestedByUsername = username
	media.RequestedByEmail = email
	return media
}

// Helper function to create config with advanced user rules
func createConfigWithUserRules(advancedRules []config.AdvancedRule) *config.Config {
	return &config.Config{
		Rules: config.RulesConfig{
			MovieRetention: "90d",
			TVRetention:    "120d",
		},
		AdvancedRules: advancedRules,
		App: config.AppConfig{
			LeavingSoonDays: 14,
		},
	}
}

func TestRulesEngine_UserBased_UserIDMatching(t *testing.T) {
	userID := 123
	advancedRules := []config.AdvancedRule{
		{
			Name:    "User 123 Cleanup",
			Type:    "user",
			Enabled: true,
			Users: []config.UserRule{
				{
					UserID:    &userID,
					Retention: "7d",
				},
			},
		},
	}

	cfg := createConfigWithUserRules(advancedRules)
	exclusions := createMockExclusions()
	engine := NewRulesEngine(cfg, exclusions)

	t.Run("matches media by user ID and applies custom retention", func(t *testing.T) {
		username := "testuser"
		email := "test@example.com"
		media := createMockMediaWithUser("movie-1", models.MediaTypeMovie, 10, 10, &userID, &username, &email)

		shouldDelete, deleteAfter, reason := engine.EvaluateMedia(&media)

		assert.True(t, shouldDelete)
		assert.False(t, deleteAfter.IsZero())
		assert.Contains(t, reason, "user rule")
		assert.Contains(t, reason, "7d")

		// Verify deletion is based on 7d retention (10 days ago watched, 7d retention = should delete)
		expectedDeleteAfter := media.LastWatched.Add(7 * 24 * time.Hour)
		assert.WithinDuration(t, expectedDeleteAfter, deleteAfter, time.Second)
	})

	t.Run("does not match media with different user ID", func(t *testing.T) {
		differentUserID := 456
		username := "otheruser"
		email := "other@example.com"
		media := createMockMediaWithUser("movie-2", models.MediaTypeMovie, 10, 10, &differentUserID, &username, &email)

		shouldDelete, deleteAfter, reason := engine.EvaluateMedia(&media)

		// Should apply standard retention rules (90d for movies)
		// 10 days old is within 90d retention
		assert.False(t, shouldDelete)
		assert.False(t, deleteAfter.IsZero())
		assert.Equal(t, "within retention", reason)
	})

	t.Run("handles media with nil user ID", func(t *testing.T) {
		username := "testuser"
		email := "test@example.com"
		media := createMockMediaWithUser("movie-3", models.MediaTypeMovie, 10, 10, nil, &username, &email)

		shouldDelete, deleteAfter, reason := engine.EvaluateMedia(&media)

		// Should apply standard retention rules (90d for movies)
		// 10 days old is within 90d retention
		assert.False(t, shouldDelete)
		assert.False(t, deleteAfter.IsZero())
		assert.Equal(t, "within retention", reason)
	})
}

func TestRulesEngine_UserBased_UsernameMatching(t *testing.T) {
	advancedRules := []config.AdvancedRule{
		{
			Name:    "Username Cleanup",
			Type:    "user",
			Enabled: true,
			Users: []config.UserRule{
				{
					Username:  "JohnDoe",
					Retention: "14d",
				},
			},
		},
	}

	cfg := createConfigWithUserRules(advancedRules)
	exclusions := createMockExclusions()
	engine := NewRulesEngine(cfg, exclusions)

	t.Run("matches media by exact username", func(t *testing.T) {
		userID := 123
		username := "JohnDoe"
		email := "john@example.com"
		media := createMockMediaWithUser("movie-1", models.MediaTypeMovie, 20, 20, &userID, &username, &email)

		shouldDelete, deleteAfter, reason := engine.EvaluateMedia(&media)

		assert.True(t, shouldDelete)
		assert.False(t, deleteAfter.IsZero())
		assert.Contains(t, reason, "user rule")
		assert.Contains(t, reason, "14d")
	})

	t.Run("matches media by username case-insensitive", func(t *testing.T) {
		userID := 123
		username := "johndoe" // lowercase
		email := "john@example.com"
		media := createMockMediaWithUser("movie-2", models.MediaTypeMovie, 20, 20, &userID, &username, &email)

		shouldDelete, _, reason := engine.EvaluateMedia(&media)

		assert.True(t, shouldDelete)
		assert.Contains(t, reason, "user rule")
	})

	t.Run("matches media by username with different casing", func(t *testing.T) {
		userID := 123
		username := "JOHNDOE" // uppercase
		email := "john@example.com"
		media := createMockMediaWithUser("movie-3", models.MediaTypeMovie, 20, 20, &userID, &username, &email)

		shouldDelete, _, reason := engine.EvaluateMedia(&media)

		assert.True(t, shouldDelete)
		assert.Contains(t, reason, "user rule")
	})

	t.Run("does not match different username", func(t *testing.T) {
		userID := 456
		username := "JaneDoe"
		email := "jane@example.com"
		media := createMockMediaWithUser("movie-4", models.MediaTypeMovie, 20, 20, &userID, &username, &email)

		shouldDelete, _, reason := engine.EvaluateMedia(&media)

		// Should apply standard retention rules (20 days within 90d)
		assert.False(t, shouldDelete)
		assert.Equal(t, "within retention", reason)
	})

	t.Run("handles media with nil username", func(t *testing.T) {
		userID := 123
		email := "john@example.com"
		media := createMockMediaWithUser("movie-5", models.MediaTypeMovie, 20, 20, &userID, nil, &email)

		shouldDelete, _, reason := engine.EvaluateMedia(&media)

		// Should apply standard retention rules (20 days within 90d)
		assert.False(t, shouldDelete)
		assert.Equal(t, "within retention", reason)
	})
}

func TestRulesEngine_UserBased_EmailMatching(t *testing.T) {
	advancedRules := []config.AdvancedRule{
		{
			Name:    "Email Cleanup",
			Type:    "user",
			Enabled: true,
			Users: []config.UserRule{
				{
					Email:     "foo@bar.com",
					Retention: "3d",
				},
			},
		},
	}

	cfg := createConfigWithUserRules(advancedRules)
	exclusions := createMockExclusions()
	engine := NewRulesEngine(cfg, exclusions)

	t.Run("matches media by exact email", func(t *testing.T) {
		userID := 789
		username := "foobar"
		email := "foo@bar.com"
		media := createMockMediaWithUser("movie-1", models.MediaTypeMovie, 5, 5, &userID, &username, &email)

		shouldDelete, deleteAfter, reason := engine.EvaluateMedia(&media)

		assert.True(t, shouldDelete)
		assert.False(t, deleteAfter.IsZero())
		assert.Contains(t, reason, "user rule")
		assert.Contains(t, reason, "3d")
	})

	t.Run("matches media by email case-insensitive", func(t *testing.T) {
		userID := 789
		username := "foobar"
		email := "FOO@BAR.COM" // uppercase
		media := createMockMediaWithUser("movie-2", models.MediaTypeMovie, 5, 5, &userID, &username, &email)

		shouldDelete, _, reason := engine.EvaluateMedia(&media)

		assert.True(t, shouldDelete)
		assert.Contains(t, reason, "user rule")
	})

	t.Run("matches media by email with mixed case", func(t *testing.T) {
		userID := 789
		username := "foobar"
		email := "Foo@Bar.Com"
		media := createMockMediaWithUser("movie-3", models.MediaTypeMovie, 5, 5, &userID, &username, &email)

		shouldDelete, _, reason := engine.EvaluateMedia(&media)

		assert.True(t, shouldDelete)
		assert.Contains(t, reason, "user rule")
	})

	t.Run("does not match different email", func(t *testing.T) {
		userID := 999
		username := "other"
		email := "other@example.com"
		media := createMockMediaWithUser("movie-4", models.MediaTypeMovie, 5, 5, &userID, &username, &email)

		shouldDelete, _, reason := engine.EvaluateMedia(&media)

		// Should apply standard retention rules (5 days within 90d)
		assert.False(t, shouldDelete)
		assert.Equal(t, "within retention", reason)
	})

	t.Run("handles media with nil email", func(t *testing.T) {
		userID := 789
		username := "foobar"
		media := createMockMediaWithUser("movie-5", models.MediaTypeMovie, 5, 5, &userID, &username, nil)

		shouldDelete, _, reason := engine.EvaluateMedia(&media)

		// Should apply standard retention rules (5 days within 90d)
		assert.False(t, shouldDelete)
		assert.Equal(t, "within retention", reason)
	})
}

func TestRulesEngine_UserBased_RequireWatchedFlag(t *testing.T) {
	userID := 100
	requireWatched := true
	advancedRules := []config.AdvancedRule{
		{
			Name:    "Require Watched Rule",
			Type:    "user",
			Enabled: true,
			Users: []config.UserRule{
				{
					UserID:         &userID,
					Retention:      "7d",
					RequireWatched: &requireWatched,
				},
			},
		},
	}

	cfg := createConfigWithUserRules(advancedRules)
	exclusions := createMockExclusions()
	engine := NewRulesEngine(cfg, exclusions)

	t.Run("deletes watched media past retention", func(t *testing.T) {
		username := "testuser"
		email := "test@example.com"
		// Added 30 days ago, watched 10 days ago (past 7d retention)
		media := createMockMediaWithUser("movie-1", models.MediaTypeMovie, 30, 10, &userID, &username, &email)

		shouldDelete, deleteAfter, reason := engine.EvaluateMedia(&media)

		assert.True(t, shouldDelete)
		assert.False(t, deleteAfter.IsZero())
		assert.Contains(t, reason, "user rule")
	})

	t.Run("does not delete watched media within retention", func(t *testing.T) {
		username := "testuser"
		email := "test@example.com"
		// Added 30 days ago, watched 3 days ago (within 7d retention)
		media := createMockMediaWithUser("movie-2", models.MediaTypeMovie, 30, 3, &userID, &username, &email)

		shouldDelete, deleteAfter, reason := engine.EvaluateMedia(&media)

		assert.False(t, shouldDelete)
		assert.False(t, deleteAfter.IsZero())
		assert.Equal(t, "user rule: within retention", reason)
	})

	t.Run("does not delete unwatched media when require_watched is true", func(t *testing.T) {
		username := "testuser"
		email := "test@example.com"
		// Added 30 days ago, never watched (past retention but unwatched)
		media := createMockMediaWithUser("movie-3", models.MediaTypeMovie, 30, -1, &userID, &username, &email)

		shouldDelete, deleteAfter, reason := engine.EvaluateMedia(&media)

		assert.False(t, shouldDelete)
		assert.True(t, deleteAfter.IsZero())
		assert.Contains(t, reason, "not watched")
	})
}

func TestRulesEngine_UserBased_RequireWatchedFalse(t *testing.T) {
	userID := 200
	requireWatched := false
	advancedRules := []config.AdvancedRule{
		{
			Name:    "No Require Watched Rule",
			Type:    "user",
			Enabled: true,
			Users: []config.UserRule{
				{
					UserID:         &userID,
					Retention:      "7d",
					RequireWatched: &requireWatched, // Does not require watched
				},
			},
		},
	}

	cfg := createConfigWithUserRules(advancedRules)
	exclusions := createMockExclusions()
	engine := NewRulesEngine(cfg, exclusions)

	t.Run("deletes unwatched media past retention when require_watched is false", func(t *testing.T) {
		username := "testuser"
		email := "test@example.com"
		// Added 10 days ago, never watched (past 7d retention)
		media := createMockMediaWithUser("movie-1", models.MediaTypeMovie, 10, -1, &userID, &username, &email)

		shouldDelete, deleteAfter, reason := engine.EvaluateMedia(&media)

		assert.True(t, shouldDelete)
		assert.False(t, deleteAfter.IsZero())
		assert.Contains(t, reason, "user rule")
	})

	t.Run("does not delete unwatched media within retention", func(t *testing.T) {
		username := "testuser"
		email := "test@example.com"
		// Added 5 days ago, never watched (within 7d retention)
		media := createMockMediaWithUser("movie-2", models.MediaTypeMovie, 5, -1, &userID, &username, &email)

		shouldDelete, deleteAfter, reason := engine.EvaluateMedia(&media)

		assert.False(t, shouldDelete)
		assert.False(t, deleteAfter.IsZero())
		assert.Equal(t, "user rule: within retention", reason)
	})
}

func TestRulesEngine_UserBased_CustomRetentionPeriods(t *testing.T) {
	userID1 := 301
	userID2 := 302
	userID3 := 303

	advancedRules := []config.AdvancedRule{
		{
			Name:    "Multiple User Rules",
			Type:    "user",
			Enabled: true,
			Users: []config.UserRule{
				{
					UserID:    &userID1,
					Retention: "3d",
				},
				{
					UserID:    &userID2,
					Retention: "14d",
				},
				{
					UserID:    &userID3,
					Retention: "30d",
				},
			},
		},
	}

	cfg := createConfigWithUserRules(advancedRules)
	exclusions := createMockExclusions()
	engine := NewRulesEngine(cfg, exclusions)

	t.Run("applies 3d retention for user 301", func(t *testing.T) {
		username := "user301"
		email := "user301@example.com"
		media := createMockMediaWithUser("movie-1", models.MediaTypeMovie, 5, 5, &userID1, &username, &email)

		shouldDelete, _, reason := engine.EvaluateMedia(&media)

		assert.True(t, shouldDelete)
		assert.Contains(t, reason, "3d")
	})

	t.Run("applies 14d retention for user 302", func(t *testing.T) {
		username := "user302"
		email := "user302@example.com"
		media := createMockMediaWithUser("movie-2", models.MediaTypeMovie, 10, 10, &userID2, &username, &email)

		shouldDelete, _, reason := engine.EvaluateMedia(&media)

		// 10 days ago with 14d retention = within retention
		assert.False(t, shouldDelete)
		assert.Equal(t, "user rule: within retention", reason)
	})

	t.Run("applies 30d retention for user 303", func(t *testing.T) {
		username := "user303"
		email := "user303@example.com"
		media := createMockMediaWithUser("movie-3", models.MediaTypeMovie, 20, 20, &userID3, &username, &email)

		shouldDelete, _, reason := engine.EvaluateMedia(&media)

		// 20 days ago with 30d retention = within retention
		assert.False(t, shouldDelete)
		assert.Equal(t, "user rule: within retention", reason)
	})
}

func TestRulesEngine_UserBased_PriorityOverStandardRules(t *testing.T) {
	userID := 400
	advancedRules := []config.AdvancedRule{
		{
			Name:    "User Priority Rule",
			Type:    "user",
			Enabled: true,
			Users: []config.UserRule{
				{
					UserID:    &userID,
					Retention: "7d", // Shorter than standard 90d movie retention
				},
			},
		},
	}

	cfg := createConfigWithUserRules(advancedRules)
	exclusions := createMockExclusions()
	engine := NewRulesEngine(cfg, exclusions)

	t.Run("user rule overrides standard movie retention", func(t *testing.T) {
		username := "testuser"
		email := "test@example.com"
		// Added/watched 10 days ago - would be safe with 90d standard retention
		// but should delete with 7d user rule
		media := createMockMediaWithUser("movie-1", models.MediaTypeMovie, 10, 10, &userID, &username, &email)

		shouldDelete, _, reason := engine.EvaluateMedia(&media)

		assert.True(t, shouldDelete)
		assert.Contains(t, reason, "user rule")
		assert.Contains(t, reason, "7d")
		// Should NOT mention 90d standard retention
		assert.NotContains(t, reason, "90d")
	})

	t.Run("standard retention applies when no user rule matches", func(t *testing.T) {
		differentUserID := 999
		username := "otheruser"
		email := "other@example.com"
		// Same scenario but different user - should apply standard 90d retention
		media := createMockMediaWithUser("movie-2", models.MediaTypeMovie, 10, 10, &differentUserID, &username, &email)

		shouldDelete, _, reason := engine.EvaluateMedia(&media)

		// Should apply standard retention rules (10 days within 90d)
		assert.False(t, shouldDelete)
		assert.Equal(t, "within retention", reason)
	})
}

func TestRulesEngine_UserBased_FallbackToStandardRules(t *testing.T) {
	userID := 500
	advancedRules := []config.AdvancedRule{
		{
			Name:    "Specific User Only",
			Type:    "user",
			Enabled: true,
			Users: []config.UserRule{
				{
					UserID:    &userID,
					Retention: "7d",
				},
			},
		},
	}

	cfg := createConfigWithUserRules(advancedRules)
	exclusions := createMockExclusions()
	engine := NewRulesEngine(cfg, exclusions)

	t.Run("applies standard rules when no user rule matches and media is old", func(t *testing.T) {
		differentUserID := 999
		username := "nonmatchinguser"
		email := "nonmatching@example.com"
		// Media is 200 days old - past 90d standard retention
		media := createMockMediaWithUser("movie-1", models.MediaTypeMovie, 200, 200, &differentUserID, &username, &email)

		shouldDelete, deleteAfter, reason := engine.EvaluateMedia(&media)

		// Should be deleted by standard rules (200 days > 90d)
		assert.True(t, shouldDelete)
		assert.False(t, deleteAfter.IsZero())
		assert.Equal(t, "retention period expired (90d)", reason)
	})

	t.Run("applies standard rules when no user rule matches and media is within retention", func(t *testing.T) {
		differentUserID := 888
		username := "anotheruser"
		email := "another@example.com"
		// Media is 50 days old - within 90d standard retention
		media := createMockMediaWithUser("movie-2", models.MediaTypeMovie, 50, 50, &differentUserID, &username, &email)

		shouldDelete, _, reason := engine.EvaluateMedia(&media)

		// Should be kept by standard rules (50 days < 90d)
		assert.False(t, shouldDelete)
		assert.Equal(t, "within retention", reason)
	})

	t.Run("applies standard rules even with user data present", func(t *testing.T) {
		differentUserID := 777
		username := "yetanotheruser"
		email := "yetanother@example.com"
		// Media is 150 days old - past 90d standard retention
		media := createMockMediaWithUser("movie-3", models.MediaTypeMovie, 150, 150, &differentUserID, &username, &email)

		shouldDelete, _, reason := engine.EvaluateMedia(&media)

		// Should be deleted by standard rules (150 days > 90d)
		assert.True(t, shouldDelete)
		assert.Equal(t, "retention period expired (90d)", reason)
	})
}

func TestRulesEngine_UserBased_DisabledRule(t *testing.T) {
	userID := 600
	advancedRules := []config.AdvancedRule{
		{
			Name:    "Disabled User Rule",
			Type:    "user",
			Enabled: false, // Rule is disabled
			Users: []config.UserRule{
				{
					UserID:    &userID,
					Retention: "1d", // Very short retention
				},
			},
		},
	}

	cfg := createConfigWithUserRules(advancedRules)
	exclusions := createMockExclusions()
	engine := NewRulesEngine(cfg, exclusions)

	t.Run("ignores disabled user rule and applies standard rules", func(t *testing.T) {
		username := "testuser"
		email := "test@example.com"
		// Media is 10 days old - within 90d standard retention
		media := createMockMediaWithUser("movie-1", models.MediaTypeMovie, 10, 10, &userID, &username, &email)

		shouldDelete, _, reason := engine.EvaluateMedia(&media)

		// Should fall back to standard rules since rule is disabled (10 days < 90d)
		assert.False(t, shouldDelete)
		assert.Equal(t, "within retention", reason)
	})
}

func TestRulesEngine_UserBased_MultipleRulesFirstMatch(t *testing.T) {
	userID := 700
	advancedRules := []config.AdvancedRule{
		{
			Name:    "First User Rule",
			Type:    "user",
			Enabled: true,
			Users: []config.UserRule{
				{
					UserID:    &userID,
					Retention: "5d",
				},
			},
		},
		{
			Name:    "Second User Rule",
			Type:    "user",
			Enabled: true,
			Users: []config.UserRule{
				{
					UserID:    &userID,
					Retention: "30d", // Different retention
				},
			},
		},
	}

	cfg := createConfigWithUserRules(advancedRules)
	exclusions := createMockExclusions()
	engine := NewRulesEngine(cfg, exclusions)

	t.Run("applies first matching user rule", func(t *testing.T) {
		username := "testuser"
		email := "test@example.com"
		media := createMockMediaWithUser("movie-1", models.MediaTypeMovie, 10, 10, &userID, &username, &email)

		shouldDelete, _, reason := engine.EvaluateMedia(&media)

		// Should use first rule (5d retention) and delete
		assert.True(t, shouldDelete)
		assert.Contains(t, reason, "5d")
		assert.NotContains(t, reason, "30d")
	})
}

// ==================== Disabled Retention Tests ====================

func TestParseDuration_DisabledValues(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected time.Duration
	}{
		{
			name:     "parse never - disables retention",
			input:    "never",
			expected: 0,
		},
		{
			name:     "parse 0d - disables retention",
			input:    "0d",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseDuration(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRulesEngine_DisabledMovieRetention(t *testing.T) {
	t.Run("movie retention disabled with 'never'", func(t *testing.T) {
		cfg := createMockConfig("never", "120d", 14)
		exclusions := createMockExclusions()
		engine := NewRulesEngine(cfg, exclusions)

		// Very old unwatched movie - would normally be deleted
		media := createMockMedia("movie-1", models.MediaTypeMovie, 365, -1, false, false)

		shouldDelete, deleteAfter, reason := engine.EvaluateMedia(&media)

		assert.False(t, shouldDelete)
		assert.True(t, deleteAfter.IsZero())
		assert.Equal(t, "retention disabled", reason)
	})

	t.Run("movie retention disabled with '0d'", func(t *testing.T) {
		cfg := createMockConfig("0d", "120d", 14)
		exclusions := createMockExclusions()
		engine := NewRulesEngine(cfg, exclusions)

		// Very old unwatched movie - would normally be deleted
		media := createMockMedia("movie-1", models.MediaTypeMovie, 365, -1, false, false)

		shouldDelete, deleteAfter, reason := engine.EvaluateMedia(&media)

		assert.False(t, shouldDelete)
		assert.True(t, deleteAfter.IsZero())
		assert.Equal(t, "retention disabled", reason)
	})

	t.Run("TV retention still applies when movie retention disabled", func(t *testing.T) {
		cfg := createMockConfig("never", "120d", 14)
		exclusions := createMockExclusions()
		engine := NewRulesEngine(cfg, exclusions)

		// Old TV show past 120d retention
		media := createMockMedia("tv-1", models.MediaTypeTVShow, 150, -1, false, false)

		shouldDelete, deleteAfter, reason := engine.EvaluateMedia(&media)

		assert.True(t, shouldDelete)
		assert.False(t, deleteAfter.IsZero())
		assert.Contains(t, reason, "retention period expired")
		assert.Contains(t, reason, "120d")
	})
}

func TestRulesEngine_DisabledTVRetention(t *testing.T) {
	t.Run("TV retention disabled with 'never'", func(t *testing.T) {
		cfg := createMockConfig("90d", "never", 14)
		exclusions := createMockExclusions()
		engine := NewRulesEngine(cfg, exclusions)

		// Very old unwatched TV show - would normally be deleted
		media := createMockMedia("tv-1", models.MediaTypeTVShow, 365, -1, false, false)

		shouldDelete, deleteAfter, reason := engine.EvaluateMedia(&media)

		assert.False(t, shouldDelete)
		assert.True(t, deleteAfter.IsZero())
		assert.Equal(t, "retention disabled", reason)
	})

	t.Run("TV retention disabled with '0d'", func(t *testing.T) {
		cfg := createMockConfig("90d", "0d", 14)
		exclusions := createMockExclusions()
		engine := NewRulesEngine(cfg, exclusions)

		// Very old unwatched TV show - would normally be deleted
		media := createMockMedia("tv-1", models.MediaTypeTVShow, 365, -1, false, false)

		shouldDelete, deleteAfter, reason := engine.EvaluateMedia(&media)

		assert.False(t, shouldDelete)
		assert.True(t, deleteAfter.IsZero())
		assert.Equal(t, "retention disabled", reason)
	})

	t.Run("movie retention still applies when TV retention disabled", func(t *testing.T) {
		cfg := createMockConfig("90d", "never", 14)
		exclusions := createMockExclusions()
		engine := NewRulesEngine(cfg, exclusions)

		// Old movie past 90d retention
		media := createMockMedia("movie-1", models.MediaTypeMovie, 120, -1, false, false)

		shouldDelete, deleteAfter, reason := engine.EvaluateMedia(&media)

		assert.True(t, shouldDelete)
		assert.False(t, deleteAfter.IsZero())
		assert.Contains(t, reason, "retention period expired")
		assert.Contains(t, reason, "90d")
	})
}

func TestRulesEngine_BothRetentionsDisabled(t *testing.T) {
	t.Run("both movie and TV retention disabled", func(t *testing.T) {
		cfg := createMockConfig("never", "never", 14)
		exclusions := createMockExclusions()
		engine := NewRulesEngine(cfg, exclusions)

		// Very old movie
		movie := createMockMedia("movie-1", models.MediaTypeMovie, 365, -1, false, false)
		shouldDelete, deleteAfter, reason := engine.EvaluateMedia(&movie)

		assert.False(t, shouldDelete)
		assert.True(t, deleteAfter.IsZero())
		assert.Equal(t, "retention disabled", reason)

		// Very old TV show
		tv := createMockMedia("tv-1", models.MediaTypeTVShow, 365, -1, false, false)
		shouldDelete, deleteAfter, reason = engine.EvaluateMedia(&tv)

		assert.False(t, shouldDelete)
		assert.True(t, deleteAfter.IsZero())
		assert.Equal(t, "retention disabled", reason)
	})

	t.Run("user rules still apply when standard retention disabled", func(t *testing.T) {
		userID := 100
		advancedRules := []config.AdvancedRule{
			{
				Name:    "User Rule",
				Type:    "user",
				Enabled: true,
				Users: []config.UserRule{
					{
						UserID:    &userID,
						Retention: "7d",
					},
				},
			},
		}

		cfg := &config.Config{
			Rules: config.RulesConfig{
				MovieRetention: "never", // Standard retention disabled
				TVRetention:    "never", // Standard retention disabled
			},
			AdvancedRules: advancedRules,
			App: config.AppConfig{
				LeavingSoonDays: 14,
			},
		}
		exclusions := createMockExclusions()
		engine := NewRulesEngine(cfg, exclusions)

		username := "testuser"
		email := "test@example.com"
		// Media matches user rule and is past 7d retention
		media := createMockMediaWithUser("movie-1", models.MediaTypeMovie, 10, 10, &userID, &username, &email)

		shouldDelete, deleteAfter, reason := engine.EvaluateMedia(&media)

		// User rule should apply even though standard retention is disabled
		assert.True(t, shouldDelete)
		assert.False(t, deleteAfter.IsZero())
		assert.Contains(t, reason, "user rule")
		assert.Contains(t, reason, "7d")
	})

	t.Run("non-matching users with disabled standard retention", func(t *testing.T) {
		userID := 100
		advancedRules := []config.AdvancedRule{
			{
				Name:    "User Rule",
				Type:    "user",
				Enabled: true,
				Users: []config.UserRule{
					{
						UserID:    &userID,
						Retention: "7d",
					},
				},
			},
		}

		cfg := &config.Config{
			Rules: config.RulesConfig{
				MovieRetention: "never", // Standard retention disabled
				TVRetention:    "never", // Standard retention disabled
			},
			AdvancedRules: advancedRules,
			App: config.AppConfig{
				LeavingSoonDays: 14,
			},
		}
		exclusions := createMockExclusions()
		engine := NewRulesEngine(cfg, exclusions)

		differentUserID := 999
		username := "otheruser"
		email := "other@example.com"
		// Media does NOT match user rule, very old
		media := createMockMediaWithUser("movie-1", models.MediaTypeMovie, 365, -1, &differentUserID, &username, &email)

		shouldDelete, deleteAfter, reason := engine.EvaluateMedia(&media)

		// Standard retention is disabled, so should NOT delete
		assert.False(t, shouldDelete)
		assert.True(t, deleteAfter.IsZero())
		assert.Equal(t, "retention disabled", reason)
	})
}

func TestRulesEngine_DisabledRetention_GetDeletionCandidates(t *testing.T) {
	t.Run("no candidates when retention disabled", func(t *testing.T) {
		cfg := createMockConfig("never", "never", 14)
		exclusions := createMockExclusions()
		engine := NewRulesEngine(cfg, exclusions)

		mediaList := []models.Media{
			createMockMedia("movie-1", models.MediaTypeMovie, 365, -1, false, false),
			createMockMedia("movie-2", models.MediaTypeMovie, 180, -1, false, false),
			createMockMedia("tv-1", models.MediaTypeTVShow, 365, -1, false, false),
			createMockMedia("tv-2", models.MediaTypeTVShow, 180, -1, false, false),
		}

		candidates := engine.GetDeletionCandidates(mediaList)

		assert.Empty(t, candidates, "Should have no candidates when retention is disabled")
	})
}

func TestRulesEngine_DisabledRetention_GetLeavingSoon(t *testing.T) {
	t.Run("no leaving soon items when retention disabled", func(t *testing.T) {
		cfg := createMockConfig("never", "never", 14)
		exclusions := createMockExclusions()
		engine := NewRulesEngine(cfg, exclusions)

		mediaList := []models.Media{
			createMockMedia("movie-1", models.MediaTypeMovie, 75, -1, false, false), // Would be leaving soon with normal retention
			createMockMedia("tv-1", models.MediaTypeTVShow, 105, -1, false, false),  // Would be leaving soon with normal retention
		}

		leavingSoon := engine.GetLeavingSoon(mediaList)

		assert.Empty(t, leavingSoon, "Should have no leaving soon items when retention is disabled")
	})
}
