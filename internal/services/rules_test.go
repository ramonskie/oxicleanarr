package services

import (
	"testing"
	"time"

	"github.com/ramonskie/oxicleanarr/internal/config"
	"github.com/ramonskie/oxicleanarr/internal/models"
	"github.com/ramonskie/oxicleanarr/internal/storage"
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
		assert.Equal(t, "user rule 'Require Watched Rule' within retention (7d)", reason)
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
		assert.Equal(t, "user rule 'No Require Watched Rule' within retention (7d)", reason)
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
		assert.Equal(t, "user rule 'Multiple User Rules' within retention (14d)", reason)
	})

	t.Run("applies 30d retention for user 303", func(t *testing.T) {
		username := "user303"
		email := "user303@example.com"
		media := createMockMediaWithUser("movie-3", models.MediaTypeMovie, 20, 20, &userID3, &username, &email)

		shouldDelete, _, reason := engine.EvaluateMedia(&media)

		// 20 days ago with 30d retention = within retention
		assert.False(t, shouldDelete)
		assert.Equal(t, "user rule 'Multiple User Rules' within retention (30d)", reason)
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

func TestRulesEngine_GenerateDeletionReason(t *testing.T) {
	now := time.Now()

	t.Run("standard movie retention - watched", func(t *testing.T) {
		cfg := createMockConfig("90d", "120d", 14)
		exclusions := createMockExclusions()
		engine := NewRulesEngine(cfg, exclusions)

		media := models.Media{
			ID:          "movie-1",
			Type:        models.MediaTypeMovie,
			Title:       "Test Movie",
			AddedAt:     now.AddDate(0, 0, -100),
			LastWatched: now.AddDate(0, 0, -50), // Watched 50 days ago
			WatchCount:  3,
		}

		deleteAfter := now.AddDate(0, 0, 40) // 90 days from last watched
		reason := engine.GenerateDeletionReason(&media, deleteAfter, "within retention")

		assert.Contains(t, reason, "This movie was last watched 50 days ago")
		assert.Contains(t, reason, "retention policy for movies is 90d")
		assert.Contains(t, reason, "will be deleted after that period of inactivity")
	})

	t.Run("standard movie retention - never watched", func(t *testing.T) {
		cfg := createMockConfig("90d", "120d", 14)
		exclusions := createMockExclusions()
		engine := NewRulesEngine(cfg, exclusions)

		media := models.Media{
			ID:         "movie-2",
			Type:       models.MediaTypeMovie,
			Title:      "Unwatched Movie",
			AddedAt:    now.AddDate(0, 0, -30), // Added 30 days ago
			WatchCount: 0,
		}

		deleteAfter := now.AddDate(0, 0, 60) // 90 days from added date
		reason := engine.GenerateDeletionReason(&media, deleteAfter, "within retention")

		assert.Contains(t, reason, "This movie was added 30 days ago")
		assert.Contains(t, reason, "retention policy for movies is 90d")
		assert.Contains(t, reason, "will be deleted after that period of inactivity")
	})

	t.Run("standard TV show retention - watched", func(t *testing.T) {
		cfg := createMockConfig("90d", "120d", 14)
		exclusions := createMockExclusions()
		engine := NewRulesEngine(cfg, exclusions)

		media := models.Media{
			ID:          "tv-1",
			Type:        models.MediaTypeTVShow,
			Title:       "Test TV Show",
			AddedAt:     now.AddDate(0, 0, -150),
			LastWatched: now.AddDate(0, 0, -70), // Watched 70 days ago
			WatchCount:  10,
		}

		deleteAfter := now.AddDate(0, 0, 50) // 120 days from last watched
		reason := engine.GenerateDeletionReason(&media, deleteAfter, "within retention")

		assert.Contains(t, reason, "This TV show was last watched 70 days ago")
		assert.Contains(t, reason, "retention policy for TV shows is 120d")
		assert.Contains(t, reason, "will be deleted after that period of inactivity")
	})

	t.Run("standard TV show retention - never watched", func(t *testing.T) {
		cfg := createMockConfig("90d", "120d", 14)
		exclusions := createMockExclusions()
		engine := NewRulesEngine(cfg, exclusions)

		media := models.Media{
			ID:         "tv-2",
			Type:       models.MediaTypeTVShow,
			Title:      "Unwatched TV Show",
			AddedAt:    now.AddDate(0, 0, -45), // Added 45 days ago
			WatchCount: 0,
		}

		deleteAfter := now.AddDate(0, 0, 75) // 120 days from added date
		reason := engine.GenerateDeletionReason(&media, deleteAfter, "within retention")

		assert.Contains(t, reason, "This TV show was added 45 days ago")
		assert.Contains(t, reason, "retention policy for TV shows is 120d")
		assert.Contains(t, reason, "will be deleted after that period of inactivity")
	})

	t.Run("user rule within retention - watched movie", func(t *testing.T) {
		cfg := createMockConfig("90d", "120d", 14)
		exclusions := createMockExclusions()
		engine := NewRulesEngine(cfg, exclusions)

		media := models.Media{
			ID:          "movie-3",
			Type:        models.MediaTypeMovie,
			Title:       "Trial User Movie",
			AddedAt:     now.AddDate(0, 0, -50),
			LastWatched: now.AddDate(0, 0, -22), // Watched 22 days ago
			WatchCount:  1,
		}

		deleteAfter := now.AddDate(0, 0, 8)                           // 30 days from last watched
		reasonStr := "user rule 'Trial Users' within retention (30d)" // From applyUserRule
		reason := engine.GenerateDeletionReason(&media, deleteAfter, reasonStr)

		assert.Contains(t, reason, "This movie was last watched 22 days ago")
		assert.Contains(t, reason, "matches the 'Trial Users' user rule with 30d retention")
		assert.Contains(t, reason, "will be deleted after that period of inactivity")
		assert.NotContains(t, reason, "90d") // Should NOT mention standard retention
	})

	t.Run("user rule retention expired - watched movie", func(t *testing.T) {
		cfg := createMockConfig("90d", "120d", 14)
		exclusions := createMockExclusions()
		engine := NewRulesEngine(cfg, exclusions)

		media := models.Media{
			ID:          "movie-4",
			Type:        models.MediaTypeMovie,
			Title:       "Overdue Movie",
			AddedAt:     now.AddDate(0, 0, -100),
			LastWatched: now.AddDate(0, 0, -45), // Watched 45 days ago
			WatchCount:  2,
		}

		deleteAfter := now.AddDate(0, 0, -15)                          // Should have been deleted 15 days ago
		reasonStr := "user rule 'Trial Users' retention expired (30d)" // From applyUserRule
		reason := engine.GenerateDeletionReason(&media, deleteAfter, reasonStr)

		assert.Contains(t, reason, "This movie was last watched 45 days ago")
		assert.Contains(t, reason, "matched the 'Trial Users' user rule with 30d retention")
		assert.Contains(t, reason, "is now scheduled for deletion")
		assert.NotContains(t, reason, "will be deleted") // Different wording for overdue
	})

	t.Run("user rule within retention - unwatched TV show", func(t *testing.T) {
		cfg := createMockConfig("90d", "120d", 14)
		exclusions := createMockExclusions()
		engine := NewRulesEngine(cfg, exclusions)

		media := models.Media{
			ID:         "tv-3",
			Type:       models.MediaTypeTVShow,
			Title:      "Trial User TV Show",
			AddedAt:    now.AddDate(0, 0, -10), // Added 10 days ago
			WatchCount: 0,
		}

		deleteAfter := now.AddDate(0, 0, 20) // 30 days from added date
		reasonStr := "user rule 'Short Retention' within retention (30d)"
		reason := engine.GenerateDeletionReason(&media, deleteAfter, reasonStr)

		assert.Contains(t, reason, "This TV show was added 10 days ago")
		assert.Contains(t, reason, "matches the 'Short Retention' user rule with 30d retention")
		assert.Contains(t, reason, "will be deleted after that period of inactivity")
		assert.NotContains(t, reason, "120d") // Should NOT mention standard TV retention
	})

	t.Run("user rule retention expired - unwatched TV show", func(t *testing.T) {
		cfg := createMockConfig("90d", "120d", 14)
		exclusions := createMockExclusions()
		engine := NewRulesEngine(cfg, exclusions)

		media := models.Media{
			ID:         "tv-4",
			Type:       models.MediaTypeTVShow,
			Title:      "Overdue TV Show",
			AddedAt:    now.AddDate(0, 0, -50), // Added 50 days ago
			WatchCount: 0,
		}

		deleteAfter := now.AddDate(0, 0, -20) // Should have been deleted 20 days ago
		reasonStr := "user rule 'Short Retention' retention expired (30d)"
		reason := engine.GenerateDeletionReason(&media, deleteAfter, reasonStr)

		assert.Contains(t, reason, "This TV show was added 50 days ago")
		assert.Contains(t, reason, "matched the 'Short Retention' user rule with 30d retention")
		assert.Contains(t, reason, "is now scheduled for deletion")
	})

	t.Run("different retention periods", func(t *testing.T) {
		testCases := []struct {
			name              string
			movieRetention    string
			tvRetention       string
			mediaType         models.MediaType
			expectedRetention string
		}{
			{
				name:              "7d movie retention",
				movieRetention:    "7d",
				tvRetention:       "30d",
				mediaType:         models.MediaTypeMovie,
				expectedRetention: "7d",
			},
			{
				name:              "30d movie retention",
				movieRetention:    "30d",
				tvRetention:       "60d",
				mediaType:         models.MediaTypeMovie,
				expectedRetention: "30d",
			},
			{
				name:              "180d TV retention",
				movieRetention:    "90d",
				tvRetention:       "180d",
				mediaType:         models.MediaTypeTVShow,
				expectedRetention: "180d",
			},
			{
				name:              "365d movie retention",
				movieRetention:    "365d",
				tvRetention:       "90d",
				mediaType:         models.MediaTypeMovie,
				expectedRetention: "365d",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				cfg := createMockConfig(tc.movieRetention, tc.tvRetention, 14)
				exclusions := createMockExclusions()
				engine := NewRulesEngine(cfg, exclusions)

				media := models.Media{
					ID:         "test-media",
					Type:       tc.mediaType,
					Title:      "Test",
					AddedAt:    now.AddDate(0, 0, -20),
					WatchCount: 0,
				}

				deleteAfter := now.AddDate(0, 0, 30)
				reason := engine.GenerateDeletionReason(&media, deleteAfter, "within retention")

				assert.Contains(t, reason, tc.expectedRetention, "Should mention the correct retention period")
			})
		}
	})

	t.Run("fallback when user rule parsing fails", func(t *testing.T) {
		cfg := createMockConfig("90d", "120d", 14)
		exclusions := createMockExclusions()
		engine := NewRulesEngine(cfg, exclusions)

		media := models.Media{
			ID:          "movie-5",
			Type:        models.MediaTypeMovie,
			Title:       "Malformed Rule Movie",
			AddedAt:     now.AddDate(0, 0, -30),
			LastWatched: now.AddDate(0, 0, -15),
			WatchCount:  1,
		}

		deleteAfter := now.AddDate(0, 0, 15)
		// Malformed reason string (missing quotes or parentheses)
		reasonStr := "user rule something wrong"
		reason := engine.GenerateDeletionReason(&media, deleteAfter, reasonStr)

		// Should fall back to basic format
		assert.Contains(t, reason, "This movie was last watched 15 days ago")
		assert.Contains(t, reason, reasonStr)
	})
}

func TestRulesEngine_EvaluateTagBasedRules(t *testing.T) {
	now := time.Now()

	t.Run("no tag rules configured", func(t *testing.T) {
		cfg := createMockConfig("90d", "120d", 14)
		exclusions := createMockExclusions()
		engine := NewRulesEngine(cfg, exclusions)

		media := createMockMedia("movie-1", models.MediaTypeMovie, 30, -1, false, false)
		media.Tags = []string{"test-tag"}

		shouldDelete, deleteAfter, reason := engine.EvaluateMedia(&media)

		// Should fall through to standard retention rules
		assert.False(t, shouldDelete)
		assert.Equal(t, "within retention", reason)
		assert.False(t, deleteAfter.IsZero())
	})

	t.Run("tag rule matches - within retention", func(t *testing.T) {
		cfg := createMockConfig("90d", "120d", 14)
		cfg.AdvancedRules = []config.AdvancedRule{
			{
				Name:      "Demo Content",
				Type:      "tag",
				Enabled:   true,
				Tag:       "demo",
				Retention: "30d",
			},
		}
		exclusions := createMockExclusions()
		engine := NewRulesEngine(cfg, exclusions)

		media := createMockMedia("movie-1", models.MediaTypeMovie, 15, -1, false, false)
		media.Tags = []string{"demo"}

		shouldDelete, deleteAfter, reason := engine.EvaluateMedia(&media)

		assert.False(t, shouldDelete)
		assert.False(t, deleteAfter.IsZero())
		assert.Contains(t, reason, "tag rule 'Demo Content'")
		assert.Contains(t, reason, "within retention")
		assert.Contains(t, reason, "30d")

		// Verify delete after is calculated correctly (15 days ago + 30 days retention = 15 days from now)
		expectedDeleteAfter := media.AddedAt.Add(30 * 24 * time.Hour)
		assert.WithinDuration(t, expectedDeleteAfter, deleteAfter, time.Second)
	})

	t.Run("tag rule matches - past retention", func(t *testing.T) {
		cfg := createMockConfig("90d", "120d", 14)
		cfg.AdvancedRules = []config.AdvancedRule{
			{
				Name:      "Demo Content",
				Type:      "tag",
				Enabled:   true,
				Tag:       "demo",
				Retention: "30d",
			},
		}
		exclusions := createMockExclusions()
		engine := NewRulesEngine(cfg, exclusions)

		media := createMockMedia("movie-1", models.MediaTypeMovie, 45, -1, false, false)
		media.Tags = []string{"demo"}

		shouldDelete, deleteAfter, reason := engine.EvaluateMedia(&media)

		assert.True(t, shouldDelete)
		assert.False(t, deleteAfter.IsZero())
		assert.Contains(t, reason, "tag rule 'Demo Content'")
		assert.Contains(t, reason, "retention expired")
		assert.Contains(t, reason, "30d")

		// Verify delete after is in the past (45 days ago + 30 days retention = 15 days ago)
		expectedDeleteAfter := media.AddedAt.Add(30 * 24 * time.Hour)
		assert.WithinDuration(t, expectedDeleteAfter, deleteAfter, time.Second)
		assert.True(t, now.After(deleteAfter))
	})

	t.Run("tag rule matches - case insensitive", func(t *testing.T) {
		cfg := createMockConfig("90d", "120d", 14)
		cfg.AdvancedRules = []config.AdvancedRule{
			{
				Name:      "Test Content",
				Type:      "tag",
				Enabled:   true,
				Tag:       "DEMO",
				Retention: "30d",
			},
		}
		exclusions := createMockExclusions()
		engine := NewRulesEngine(cfg, exclusions)

		// Media has lowercase tag, rule has uppercase
		media := createMockMedia("movie-1", models.MediaTypeMovie, 15, -1, false, false)
		media.Tags = []string{"demo"}

		shouldDelete, deleteAfter, reason := engine.EvaluateMedia(&media)

		assert.False(t, shouldDelete)
		assert.Contains(t, reason, "tag rule 'Test Content'")
		assert.False(t, deleteAfter.IsZero())
	})

	t.Run("tag rule matches - multiple tags on media", func(t *testing.T) {
		cfg := createMockConfig("90d", "120d", 14)
		cfg.AdvancedRules = []config.AdvancedRule{
			{
				Name:      "Demo Content",
				Type:      "tag",
				Enabled:   true,
				Tag:       "demo",
				Retention: "30d",
			},
		}
		exclusions := createMockExclusions()
		engine := NewRulesEngine(cfg, exclusions)

		media := createMockMedia("movie-1", models.MediaTypeMovie, 15, -1, false, false)
		media.Tags = []string{"other-tag", "demo", "another-tag"}

		shouldDelete, deleteAfter, reason := engine.EvaluateMedia(&media)

		assert.False(t, shouldDelete)
		assert.Contains(t, reason, "tag rule 'Demo Content'")
		assert.False(t, deleteAfter.IsZero())
	})

	t.Run("tag rule does not match - different tag", func(t *testing.T) {
		cfg := createMockConfig("90d", "120d", 14)
		cfg.AdvancedRules = []config.AdvancedRule{
			{
				Name:      "Demo Content",
				Type:      "tag",
				Enabled:   true,
				Tag:       "demo",
				Retention: "30d",
			},
		}
		exclusions := createMockExclusions()
		engine := NewRulesEngine(cfg, exclusions)

		media := createMockMedia("movie-1", models.MediaTypeMovie, 15, -1, false, false)
		media.Tags = []string{"other-tag"}

		shouldDelete, deleteAfter, reason := engine.EvaluateMedia(&media)

		// Should fall through to standard retention rules
		assert.False(t, shouldDelete)
		assert.Equal(t, "within retention", reason)
		assert.False(t, deleteAfter.IsZero())
	})

	t.Run("tag rule disabled", func(t *testing.T) {
		cfg := createMockConfig("90d", "120d", 14)
		cfg.AdvancedRules = []config.AdvancedRule{
			{
				Name:      "Demo Content",
				Type:      "tag",
				Enabled:   false, // Disabled
				Tag:       "demo",
				Retention: "30d",
			},
		}
		exclusions := createMockExclusions()
		engine := NewRulesEngine(cfg, exclusions)

		media := createMockMedia("movie-1", models.MediaTypeMovie, 15, -1, false, false)
		media.Tags = []string{"demo"}

		shouldDelete, deleteAfter, reason := engine.EvaluateMedia(&media)

		// Should fall through to standard retention rules (rule is disabled)
		assert.False(t, shouldDelete)
		assert.Equal(t, "within retention", reason)
		assert.False(t, deleteAfter.IsZero())
	})

	t.Run("tag rule uses last watched date when available", func(t *testing.T) {
		cfg := createMockConfig("90d", "120d", 14)
		cfg.AdvancedRules = []config.AdvancedRule{
			{
				Name:      "Demo Content",
				Type:      "tag",
				Enabled:   true,
				Tag:       "demo",
				Retention: "30d",
			},
		}
		exclusions := createMockExclusions()
		engine := NewRulesEngine(cfg, exclusions)

		// Added 100 days ago but watched 10 days ago
		media := createMockMedia("movie-1", models.MediaTypeMovie, 100, 10, false, false)
		media.Tags = []string{"demo"}

		shouldDelete, deleteAfter, reason := engine.EvaluateMedia(&media)

		assert.False(t, shouldDelete)
		assert.Contains(t, reason, "tag rule 'Demo Content'")
		assert.Contains(t, reason, "within retention")

		// Delete after should be based on last watched (10 days ago + 30 days = 20 days from now)
		expectedDeleteAfter := media.LastWatched.Add(30 * 24 * time.Hour)
		assert.WithinDuration(t, expectedDeleteAfter, deleteAfter, time.Second)
	})

	t.Run("tag rule priority over standard retention", func(t *testing.T) {
		cfg := createMockConfig("90d", "120d", 14)
		cfg.AdvancedRules = []config.AdvancedRule{
			{
				Name:      "Quick Delete",
				Type:      "tag",
				Enabled:   true,
				Tag:       "quick-delete",
				Retention: "7d", // Much shorter than standard 90d
			},
		}
		exclusions := createMockExclusions()
		engine := NewRulesEngine(cfg, exclusions)

		// Media added 30 days ago - within standard 90d retention but past tag rule 7d retention
		media := createMockMedia("movie-1", models.MediaTypeMovie, 30, -1, false, false)
		media.Tags = []string{"quick-delete"}

		shouldDelete, deleteAfter, reason := engine.EvaluateMedia(&media)

		// Tag rule should take priority and mark for deletion
		assert.True(t, shouldDelete)
		assert.Contains(t, reason, "tag rule 'Quick Delete'")
		assert.Contains(t, reason, "retention expired")
		assert.Contains(t, reason, "7d")

		// Would be within retention under standard rules (30 < 90)
		// But tag rule overrides with 7d retention
		expectedDeleteAfter := media.AddedAt.Add(7 * 24 * time.Hour)
		assert.WithinDuration(t, expectedDeleteAfter, deleteAfter, time.Second)
		assert.True(t, now.After(deleteAfter))
	})

	t.Run("multiple tag rules - first match wins", func(t *testing.T) {
		cfg := createMockConfig("90d", "120d", 14)
		cfg.AdvancedRules = []config.AdvancedRule{
			{
				Name:      "Quick Delete",
				Type:      "tag",
				Enabled:   true,
				Tag:       "quick",
				Retention: "7d",
			},
			{
				Name:      "Slow Delete",
				Type:      "tag",
				Enabled:   true,
				Tag:       "slow",
				Retention: "365d",
			},
		}
		exclusions := createMockExclusions()
		engine := NewRulesEngine(cfg, exclusions)

		// Media has both tags - should use first matching rule
		media := createMockMedia("movie-1", models.MediaTypeMovie, 30, -1, false, false)
		media.Tags = []string{"quick", "slow"}

		shouldDelete, _, reason := engine.EvaluateMedia(&media)

		assert.True(t, shouldDelete)
		assert.Contains(t, reason, "Quick Delete") // First rule name
		assert.Contains(t, reason, "7d")           // First rule retention
	})

	t.Run("tag rule with TV show media", func(t *testing.T) {
		cfg := createMockConfig("90d", "120d", 14)
		cfg.AdvancedRules = []config.AdvancedRule{
			{
				Name:      "Test Shows",
				Type:      "tag",
				Enabled:   true,
				Tag:       "test-show",
				Retention: "14d",
			},
		}
		exclusions := createMockExclusions()
		engine := NewRulesEngine(cfg, exclusions)

		media := createMockMedia("tv-1", models.MediaTypeTVShow, 20, -1, false, false)
		media.Tags = []string{"test-show"}

		shouldDelete, deleteAfter, reason := engine.EvaluateMedia(&media)

		assert.True(t, shouldDelete)
		assert.Contains(t, reason, "tag rule 'Test Shows'")
		assert.Contains(t, reason, "14d")

		expectedDeleteAfter := media.AddedAt.Add(14 * 24 * time.Hour)
		assert.WithinDuration(t, expectedDeleteAfter, deleteAfter, time.Second)
	})
}

func TestRulesEngine_TagRulePriority(t *testing.T) {
	t.Run("tag rules override exclusions=false, requested=false", func(t *testing.T) {
		cfg := createMockConfig("90d", "120d", 14)
		cfg.AdvancedRules = []config.AdvancedRule{
			{
				Name:      "Demo Content",
				Type:      "tag",
				Enabled:   true,
				Tag:       "demo",
				Retention: "7d",
			},
		}
		exclusions := createMockExclusions()
		engine := NewRulesEngine(cfg, exclusions)

		// Old media that would normally be deleted, but has tag rule
		media := createMockMedia("movie-1", models.MediaTypeMovie, 30, -1, false, false)
		media.Tags = []string{"demo"}

		shouldDelete, _, reason := engine.EvaluateMedia(&media)

		// Tag rule applies (30 > 7 days)
		assert.True(t, shouldDelete)
		assert.Contains(t, reason, "tag rule 'Demo Content'")
	})

	t.Run("exclusions still override tag rules", func(t *testing.T) {
		cfg := createMockConfig("90d", "120d", 14)
		cfg.AdvancedRules = []config.AdvancedRule{
			{
				Name:      "Demo Content",
				Type:      "tag",
				Enabled:   true,
				Tag:       "demo",
				Retention: "7d",
			},
		}
		exclusions := createMockExclusions()
		exclusions.Add(storage.ExclusionItem{
			ExternalID: "movie-1",
			Reason:     "user keep",
		})
		engine := NewRulesEngine(cfg, exclusions)

		// Old media with tag, but excluded
		media := createMockMedia("movie-1", models.MediaTypeMovie, 30, -1, false, false)
		media.Tags = []string{"demo"}

		shouldDelete, deleteAfter, reason := engine.EvaluateMedia(&media)

		// Exclusion takes priority
		assert.False(t, shouldDelete)
		assert.True(t, deleteAfter.IsZero())
		assert.Equal(t, "excluded", reason)
	})

	t.Run("tag_rules_override_requested_status", func(t *testing.T) {
		cfg := createMockConfig("90d", "120d", 14)
		cfg.AdvancedRules = []config.AdvancedRule{
			{
				Name:      "Demo Content",
				Type:      "tag",
				Enabled:   true,
				Tag:       "demo",
				Retention: "7d",
			},
		}
		exclusions := createMockExclusions()
		engine := NewRulesEngine(cfg, exclusions)

		// Old media with tag, but requested
		media := createMockMedia("movie-1", models.MediaTypeMovie, 30, -1, true, false)
		media.Tags = []string{"demo"}

		shouldDelete, _, reason := engine.EvaluateMedia(&media)

		// Tag rule takes priority over requested status (when advanced rules exist)
		assert.True(t, shouldDelete)
		assert.Contains(t, reason, "tag rule 'Demo Content'")
	})
}

func TestGenerateDeletionReason_TagBasedRules(t *testing.T) {
	now := time.Now()

	t.Run("tag rule within retention - unwatched movie", func(t *testing.T) {
		cfg := createMockConfig("90d", "120d", 14)
		exclusions := createMockExclusions()
		engine := NewRulesEngine(cfg, exclusions)

		media := models.Media{
			ID:         "movie-1",
			Type:       models.MediaTypeMovie,
			Title:      "Test Movie",
			AddedAt:    now.AddDate(0, 0, -10),
			WatchCount: 0,
			Tags:       []string{"demo"},
		}

		deleteAfter := now.AddDate(0, 0, 20)
		reasonStr := "tag rule 'Demo Content' (tag: demo) within retention (30d)"
		reason := engine.GenerateDeletionReason(&media, deleteAfter, reasonStr)

		assert.Contains(t, reason, "This movie was added 10 days ago")
		assert.Contains(t, reason, "It matches the 'Demo Content' tag rule (tag: demo)")
		assert.Contains(t, reason, "with 30d retention")
		assert.Contains(t, reason, "meaning it will be deleted after that period of inactivity")
	})

	t.Run("tag rule retention expired - watched movie", func(t *testing.T) {
		cfg := createMockConfig("90d", "120d", 14)
		exclusions := createMockExclusions()
		engine := NewRulesEngine(cfg, exclusions)

		media := models.Media{
			ID:          "movie-1",
			Type:        models.MediaTypeMovie,
			Title:       "Test Movie",
			AddedAt:     now.AddDate(0, 0, -50),
			LastWatched: now.AddDate(0, 0, -20),
			WatchCount:  3,
			Tags:        []string{"quick-delete"},
		}

		deleteAfter := now.AddDate(0, 0, -5)
		reasonStr := "tag rule 'Quick Delete' (tag: quick-delete) retention expired (14d)"
		reason := engine.GenerateDeletionReason(&media, deleteAfter, reasonStr)

		assert.Contains(t, reason, "This movie was last watched 20 days ago")
		assert.Contains(t, reason, "It matched the 'Quick Delete' tag rule (tag: quick-delete)")
		assert.Contains(t, reason, "with 14d retention")
		assert.Contains(t, reason, "is now scheduled for deletion")
	})

	t.Run("tag rule with TV show", func(t *testing.T) {
		cfg := createMockConfig("90d", "120d", 14)
		exclusions := createMockExclusions()
		engine := NewRulesEngine(cfg, exclusions)

		media := models.Media{
			ID:         "tv-1",
			Type:       models.MediaTypeTVShow,
			Title:      "Test Show",
			AddedAt:    now.AddDate(0, 0, -30),
			WatchCount: 0,
			Tags:       []string{"test-show"},
		}

		deleteAfter := now.AddDate(0, 0, -9)
		reasonStr := "tag rule 'Test Shows' (tag: test-show) retention expired (21d)"
		reason := engine.GenerateDeletionReason(&media, deleteAfter, reasonStr)

		assert.Contains(t, reason, "This TV show was added 30 days ago")
		assert.Contains(t, reason, "It matched the 'Test Shows' tag rule (tag: test-show)")
		assert.Contains(t, reason, "with 21d retention")
		assert.Contains(t, reason, "is now scheduled for deletion")
	})
}
