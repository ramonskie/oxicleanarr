package rules

import (
	"testing"
	"time"

	"github.com/ramonskie/oxicleanarr/internal/config"
	"github.com/ramonskie/oxicleanarr/internal/models"
	"github.com/ramonskie/oxicleanarr/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── Helpers ──────────────────────────────────────────────────────────────────

func mockConfig(movieRetention, tvRetention string, leavingSoonDays int) *config.Config {
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

func mockExclusions() *storage.ExclusionsFile {
	return &storage.ExclusionsFile{
		Version: "1.0",
		Items:   make(map[string]storage.ExclusionItem),
	}
}

// mockMedia creates a media item. lastWatchedDaysAgo < 0 means never watched.
func mockMedia(id string, mediaType models.MediaType, addedDaysAgo, lastWatchedDaysAgo int, isRequested bool) models.Media {
	now := time.Now()
	media := models.Media{
		ID:          id,
		Type:        mediaType,
		Title:       "Test Media " + id,
		Year:        2024,
		AddedAt:     now.AddDate(0, 0, -addedDaysAgo),
		IsRequested: isRequested,
		FileSize:    1024 * 1024 * 1024,
	}
	if lastWatchedDaysAgo >= 0 {
		media.LastWatched = now.AddDate(0, 0, -lastWatchedDaysAgo)
		media.WatchCount = 1
	}
	return media
}

func mockMediaWithUser(id string, mediaType models.MediaType, addedDaysAgo, lastWatchedDaysAgo int, userID *int, username, email *string) models.Media {
	media := mockMedia(id, mediaType, addedDaysAgo, lastWatchedDaysAgo, true)
	media.RequestedByUserID = userID
	media.RequestedByUsername = username
	media.RequestedByEmail = email
	return media
}

func configWithUserRules(advancedRules []config.AdvancedRule) *config.Config {
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

// buildEngine constructs a RulesEngine directly from config + exclusions,
// bypassing config.Get() (which requires a loaded global config in tests).
func buildEngine(cfg *config.Config, exclusions *storage.ExclusionsFile) *RulesEngine {
	e := &RulesEngine{}

	e.protectionRules = []Rule{
		NewExclusionRule(exclusions),
		NewDiskThresholdRule(),
	}

	for _, rule := range cfg.AdvancedRules {
		if !rule.Enabled {
			continue
		}
		switch rule.Type {
		case "tag":
			tr := NewTagRule(rule)
			e.protectionRules = append(e.protectionRules, tr)
			e.schedulingRules = append(e.schedulingRules, tr)
		case "user":
			ur := NewUserRule(rule)
			e.protectionRules = append(e.protectionRules, ur)
			e.schedulingRules = append(e.schedulingRules, ur)
		case "watched":
			wr := NewWatchedRule(rule)
			e.protectionRules = append(e.protectionRules, wr)
			e.schedulingRules = append(e.schedulingRules, wr)
		}
	}

	sr := NewStandardRule()
	e.protectionRules = append(e.protectionRules, sr)
	e.schedulingRules = append(e.schedulingRules, sr)

	return e
}

// eval runs Evaluate with the given config injected into EvalContext directly,
// bypassing config.Get().
func eval(e *RulesEngine, cfg *config.Config, media *models.Media) RuleVerdict {
	ctx := EvalContext{
		Media:      media,
		Config:     cfg,
		DiskStatus: nil,
	}
	return e.evaluateWithContext(ctx)
}

// leavingSoon filters a media list to items leaving within threshold days,
// mirroring the old GetLeavingSoon behaviour using EvaluateForPreview semantics.
func leavingSoon(e *RulesEngine, cfg *config.Config, mediaList []models.Media) []models.Media {
	threshold := cfg.App.LeavingSoonDays
	result := make([]models.Media, 0)
	for _, m := range mediaList {
		v := eval(e, cfg, &m)
		if v.IsProtected || v.DeleteAfter.IsZero() {
			continue
		}
		daysUntil := int(time.Until(v.DeleteAfter).Hours() / 24)
		if daysUntil > 0 && daysUntil <= threshold {
			m.DeleteAfter = v.DeleteAfter
			m.DaysUntilDue = daysUntil
			result = append(result, m)
		}
	}
	return result
}

// deletionCandidates filters a media list to items overdue for deletion.
func deletionCandidates(e *RulesEngine, cfg *config.Config, mediaList []models.Media) []models.Media {
	result := make([]models.Media, 0)
	for _, m := range mediaList {
		v := eval(e, cfg, &m)
		if v.ShouldDelete() {
			result = append(result, m)
		}
	}
	return result
}

// ── parseDuration ─────────────────────────────────────────────────────────────

func TestParseDuration(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    time.Duration
		expectError bool
	}{
		{"parse days", "90d", 90 * 24 * time.Hour, false},
		{"parse hours", "24h", 24 * time.Hour, false},
		{"parse minutes", "30m", 30 * time.Minute, false},
		{"parse seconds", "60s", 60 * time.Second, false},
		{"invalid format - no unit", "90", 0, true},
		{"invalid format - unknown unit", "90x", 0, true},
		{"empty string", "", 0, true},
		{"invalid number", "abcd", 0, true},
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

func TestParseDuration_DisabledValues(t *testing.T) {
	for _, input := range []string{"never", "0d"} {
		t.Run(input, func(t *testing.T) {
			d, err := parseDuration(input)
			require.NoError(t, err)
			assert.Equal(t, time.Duration(0), d)
		})
	}
}

// ── Exclusions ────────────────────────────────────────────────────────────────

func TestEngine_Exclusions(t *testing.T) {
	cfg := mockConfig("90d", "120d", 14)
	excl := mockExclusions()
	engine := buildEngine(cfg, excl)

	excl.Add(storage.ExclusionItem{ExternalID: "movie-1", Reason: "user requested"})
	media := mockMedia("movie-1", models.MediaTypeMovie, 200, -1, false)

	v := eval(engine, cfg, &media)

	assert.True(t, v.IsProtected)
	assert.Equal(t, ProtectedExcluded, v.ProtectionReason)
	assert.Equal(t, "exclusion", v.ProtectingRule)
	assert.True(t, v.DeleteAfter.IsZero())
}

// ── Requested protection ──────────────────────────────────────────────────────

func TestEngine_RequestedProtection(t *testing.T) {
	cfg := mockConfig("90d", "120d", 14)
	excl := mockExclusions()
	engine := buildEngine(cfg, excl)

	// Requested with no advanced rules → protected
	media := mockMedia("movie-1", models.MediaTypeMovie, 200, -1, true)
	v := eval(engine, cfg, &media)

	assert.True(t, v.IsProtected)
	assert.Equal(t, ProtectedRequested, v.ProtectionReason)
	assert.True(t, v.DeleteAfter.IsZero())
}

// ── Standard movie retention ──────────────────────────────────────────────────

func TestEngine_MovieRetention(t *testing.T) {
	cfg := mockConfig("90d", "120d", 14)
	excl := mockExclusions()
	engine := buildEngine(cfg, excl)

	t.Run("unwatched within retention", func(t *testing.T) {
		media := mockMedia("movie-1", models.MediaTypeMovie, 30, -1, false)
		v := eval(engine, cfg, &media)

		assert.False(t, v.IsProtected)
		assert.False(t, v.DeleteAfter.IsZero())
		assert.False(t, v.ShouldDelete())
		assert.Equal(t, SourceStandardRetention, v.ScheduleSource)

		expected := media.AddedAt.Add(90 * 24 * time.Hour)
		assert.WithinDuration(t, expected, v.DeleteAfter, time.Second)
	})

	t.Run("unwatched past retention", func(t *testing.T) {
		media := mockMedia("movie-1", models.MediaTypeMovie, 120, -1, false)
		v := eval(engine, cfg, &media)

		assert.False(t, v.IsProtected)
		assert.True(t, v.ShouldDelete())
		assert.Equal(t, SourceStandardRetention, v.ScheduleSource)
	})

	t.Run("watched uses last watched date", func(t *testing.T) {
		// Added 200 days ago, watched 30 days ago
		media := mockMedia("movie-1", models.MediaTypeMovie, 200, 30, false)
		v := eval(engine, cfg, &media)

		assert.False(t, v.IsProtected)
		assert.False(t, v.ShouldDelete())
		assert.Equal(t, SourceStandardRetention, v.ScheduleSource)

		expected := media.LastWatched.Add(90 * 24 * time.Hour)
		assert.WithinDuration(t, expected, v.DeleteAfter, time.Second)
	})

	t.Run("watched past retention", func(t *testing.T) {
		// Added 300 days ago, watched 100 days ago
		media := mockMedia("movie-1", models.MediaTypeMovie, 300, 100, false)
		v := eval(engine, cfg, &media)

		assert.True(t, v.ShouldDelete())
		assert.Equal(t, SourceStandardRetention, v.ScheduleSource)
	})
}

// ── Standard TV retention ─────────────────────────────────────────────────────

func TestEngine_TVRetention(t *testing.T) {
	cfg := mockConfig("90d", "120d", 14)
	excl := mockExclusions()
	engine := buildEngine(cfg, excl)

	t.Run("unwatched within retention", func(t *testing.T) {
		media := mockMedia("tv-1", models.MediaTypeTVShow, 60, -1, false)
		v := eval(engine, cfg, &media)

		assert.False(t, v.IsProtected)
		assert.False(t, v.ShouldDelete())
		assert.Equal(t, SourceStandardRetention, v.ScheduleSource)

		expected := media.AddedAt.Add(120 * 24 * time.Hour)
		assert.WithinDuration(t, expected, v.DeleteAfter, time.Second)
	})

	t.Run("unwatched past retention", func(t *testing.T) {
		media := mockMedia("tv-1", models.MediaTypeTVShow, 150, -1, false)
		v := eval(engine, cfg, &media)

		assert.True(t, v.ShouldDelete())
		assert.Equal(t, SourceStandardRetention, v.ScheduleSource)
	})

	t.Run("watched uses last watched date", func(t *testing.T) {
		// Added 300 days ago, watched 60 days ago
		media := mockMedia("tv-1", models.MediaTypeTVShow, 300, 60, false)
		v := eval(engine, cfg, &media)

		assert.False(t, v.ShouldDelete())
		assert.Equal(t, SourceStandardRetention, v.ScheduleSource)

		expected := media.LastWatched.Add(120 * 24 * time.Hour)
		assert.WithinDuration(t, expected, v.DeleteAfter, time.Second)
	})
}

// ── Deletion candidates / leaving soon ───────────────────────────────────────

func TestEngine_DeletionCandidates(t *testing.T) {
	cfg := mockConfig("90d", "120d", 14)
	excl := mockExclusions()
	engine := buildEngine(cfg, excl)

	mediaList := []models.Media{
		mockMedia("movie-1", models.MediaTypeMovie, 120, -1, false), // should delete
		mockMedia("movie-2", models.MediaTypeMovie, 30, -1, false),  // within retention
		mockMedia("movie-3", models.MediaTypeMovie, 150, -1, true),  // requested → protected
		mockMedia("tv-1", models.MediaTypeTVShow, 150, -1, false),   // should delete
		mockMedia("tv-2", models.MediaTypeTVShow, 60, -1, false),    // within retention
	}
	excl.Add(storage.ExclusionItem{ExternalID: "movie-4", Reason: "test"})
	mediaList = append(mediaList, mockMedia("movie-4", models.MediaTypeMovie, 200, -1, false)) // excluded

	candidates := deletionCandidates(engine, cfg, mediaList)

	t.Run("correct count", func(t *testing.T) {
		assert.Len(t, candidates, 2) // movie-1 and tv-1
	})

	t.Run("correct IDs", func(t *testing.T) {
		ids := make([]string, len(candidates))
		for i, c := range candidates {
			ids[i] = c.ID
		}
		assert.Contains(t, ids, "movie-1")
		assert.Contains(t, ids, "tv-1")
	})
}

func TestEngine_LeavingSoon(t *testing.T) {
	cfg := mockConfig("90d", "120d", 14)
	excl := mockExclusions()
	engine := buildEngine(cfg, excl)

	now := time.Now()
	_ = now

	mediaList := []models.Media{
		mockMedia("movie-1", models.MediaTypeMovie, 80, -1, false),  // due in ~10 days
		mockMedia("movie-2", models.MediaTypeMovie, 60, -1, false),  // due in ~30 days (not leaving soon)
		mockMedia("movie-3", models.MediaTypeMovie, 85, -1, false),  // due in ~5 days
		mockMedia("movie-4", models.MediaTypeMovie, 120, -1, false), // already past due
		mockMedia("tv-1", models.MediaTypeTVShow, 108, -1, false),   // due in ~12 days
	}

	soon := leavingSoon(engine, cfg, mediaList)

	t.Run("correct count", func(t *testing.T) {
		assert.GreaterOrEqual(t, len(soon), 2)
		assert.LessOrEqual(t, len(soon), 3)
	})

	t.Run("items have positive days until due", func(t *testing.T) {
		for _, item := range soon {
			assert.Greater(t, item.DaysUntilDue, 0)
			assert.LessOrEqual(t, item.DaysUntilDue, 14)
		}
	})

	t.Run("does not include past-due items", func(t *testing.T) {
		for _, item := range soon {
			assert.NotEqual(t, "movie-4", item.ID)
		}
	})

	t.Run("does not include items too far in future", func(t *testing.T) {
		for _, item := range soon {
			assert.NotEqual(t, "movie-2", item.ID)
		}
	})
}

func TestEngine_LeavingSoon_CustomThreshold(t *testing.T) {
	cfg := mockConfig("90d", "120d", 7)
	excl := mockExclusions()
	engine := buildEngine(cfg, excl)

	mediaList := []models.Media{
		mockMedia("movie-1", models.MediaTypeMovie, 85, -1, false), // due in ~5 days
		mockMedia("movie-2", models.MediaTypeMovie, 80, -1, false), // due in ~10 days
	}

	soon := leavingSoon(engine, cfg, mediaList)

	for _, item := range soon {
		assert.LessOrEqual(t, item.DaysUntilDue, 7)
	}
}

// ── Invalid / disabled retention ──────────────────────────────────────────────

func TestEngine_InvalidRetention(t *testing.T) {
	cfg := mockConfig("invalid", "120d", 14)
	excl := mockExclusions()
	engine := buildEngine(cfg, excl)

	media := mockMedia("movie-1", models.MediaTypeMovie, 30, -1, false)
	v := eval(engine, cfg, &media)

	// Invalid retention → StandardRule.Schedule returns zero → ProtectedNoRule
	assert.True(t, v.IsProtected)
	assert.Equal(t, ProtectedNoRule, v.ProtectionReason)
	assert.True(t, v.DeleteAfter.IsZero())
}

func TestEngine_DisabledMovieRetention(t *testing.T) {
	t.Run("never", func(t *testing.T) {
		cfg := mockConfig("never", "120d", 14)
		excl := mockExclusions()
		engine := buildEngine(cfg, excl)

		media := mockMedia("movie-1", models.MediaTypeMovie, 365, -1, false)
		v := eval(engine, cfg, &media)

		assert.True(t, v.IsProtected)
		assert.Equal(t, ProtectedNoRule, v.ProtectionReason)
		assert.True(t, v.DeleteAfter.IsZero())
	})

	t.Run("0d", func(t *testing.T) {
		cfg := mockConfig("0d", "120d", 14)
		excl := mockExclusions()
		engine := buildEngine(cfg, excl)

		media := mockMedia("movie-1", models.MediaTypeMovie, 365, -1, false)
		v := eval(engine, cfg, &media)

		assert.True(t, v.IsProtected)
		assert.Equal(t, ProtectedNoRule, v.ProtectionReason)
	})

	t.Run("TV retention still applies", func(t *testing.T) {
		cfg := mockConfig("never", "120d", 14)
		excl := mockExclusions()
		engine := buildEngine(cfg, excl)

		media := mockMedia("tv-1", models.MediaTypeTVShow, 150, -1, false)
		v := eval(engine, cfg, &media)

		assert.True(t, v.ShouldDelete())
		assert.Equal(t, SourceStandardRetention, v.ScheduleSource)
	})
}

func TestEngine_DisabledTVRetention(t *testing.T) {
	t.Run("never", func(t *testing.T) {
		cfg := mockConfig("90d", "never", 14)
		excl := mockExclusions()
		engine := buildEngine(cfg, excl)

		media := mockMedia("tv-1", models.MediaTypeTVShow, 365, -1, false)
		v := eval(engine, cfg, &media)

		assert.True(t, v.IsProtected)
		assert.Equal(t, ProtectedNoRule, v.ProtectionReason)
	})

	t.Run("movie retention still applies", func(t *testing.T) {
		cfg := mockConfig("90d", "never", 14)
		excl := mockExclusions()
		engine := buildEngine(cfg, excl)

		media := mockMedia("movie-1", models.MediaTypeMovie, 120, -1, false)
		v := eval(engine, cfg, &media)

		assert.True(t, v.ShouldDelete())
		assert.Equal(t, SourceStandardRetention, v.ScheduleSource)
	})
}

func TestEngine_BothRetentionsDisabled(t *testing.T) {
	t.Run("both disabled", func(t *testing.T) {
		cfg := mockConfig("never", "never", 14)
		excl := mockExclusions()
		engine := buildEngine(cfg, excl)

		movie := mockMedia("movie-1", models.MediaTypeMovie, 365, -1, false)
		v := eval(engine, cfg, &movie)
		assert.True(t, v.IsProtected)
		assert.Equal(t, ProtectedNoRule, v.ProtectionReason)

		tv := mockMedia("tv-1", models.MediaTypeTVShow, 365, -1, false)
		v = eval(engine, cfg, &tv)
		assert.True(t, v.IsProtected)
		assert.Equal(t, ProtectedNoRule, v.ProtectionReason)
	})

	t.Run("user rules still apply when standard retention disabled", func(t *testing.T) {
		userID := 100
		cfg := &config.Config{
			Rules: config.RulesConfig{
				MovieRetention: "never",
				TVRetention:    "never",
			},
			AdvancedRules: []config.AdvancedRule{
				{Name: "User Rule", Type: "user", Enabled: true, Users: []config.UserRule{
					{UserID: &userID, Retention: "7d"},
				}},
			},
			App: config.AppConfig{LeavingSoonDays: 14},
		}
		excl := mockExclusions()
		engine := buildEngine(cfg, excl)

		username := "testuser"
		email := "test@example.com"
		media := mockMediaWithUser("movie-1", models.MediaTypeMovie, 10, 10, &userID, &username, &email)
		v := eval(engine, cfg, &media)

		assert.True(t, v.ShouldDelete())
		assert.Equal(t, SourceUserRule, v.ScheduleSource)
	})

	t.Run("non-matching user with disabled standard retention", func(t *testing.T) {
		userID := 100
		cfg := &config.Config{
			Rules: config.RulesConfig{
				MovieRetention: "never",
				TVRetention:    "never",
			},
			AdvancedRules: []config.AdvancedRule{
				{Name: "User Rule", Type: "user", Enabled: true, Users: []config.UserRule{
					{UserID: &userID, Retention: "7d"},
				}},
			},
			App: config.AppConfig{LeavingSoonDays: 14},
		}
		excl := mockExclusions()
		engine := buildEngine(cfg, excl)

		differentUserID := 999
		username := "other"
		email := "other@example.com"
		media := mockMediaWithUser("movie-1", models.MediaTypeMovie, 365, -1, &differentUserID, &username, &email)
		v := eval(engine, cfg, &media)

		assert.True(t, v.IsProtected)
		assert.Equal(t, ProtectedNoRule, v.ProtectionReason)
	})
}

func TestEngine_DisabledRetention_NoCandidates(t *testing.T) {
	cfg := mockConfig("never", "never", 14)
	excl := mockExclusions()
	engine := buildEngine(cfg, excl)

	mediaList := []models.Media{
		mockMedia("movie-1", models.MediaTypeMovie, 365, -1, false),
		mockMedia("tv-1", models.MediaTypeTVShow, 365, -1, false),
	}

	assert.Empty(t, deletionCandidates(engine, cfg, mediaList))
	assert.Empty(t, leavingSoon(engine, cfg, mediaList))
}

// ── Edge cases ────────────────────────────────────────────────────────────────

func TestEngine_EdgeCases(t *testing.T) {
	cfg := mockConfig("90d", "120d", 14)
	excl := mockExclusions()
	engine := buildEngine(cfg, excl)

	t.Run("at retention boundary", func(t *testing.T) {
		media := mockMedia("movie-1", models.MediaTypeMovie, 90, -1, false)
		v := eval(engine, cfg, &media)
		// May or may not delete depending on exact timing
		assert.False(t, v.DeleteAfter.IsZero())
	})

	t.Run("empty media list", func(t *testing.T) {
		assert.Empty(t, deletionCandidates(engine, cfg, []models.Media{}))
		assert.Empty(t, leavingSoon(engine, cfg, []models.Media{}))
	})

	t.Run("zero time values", func(t *testing.T) {
		media := models.Media{
			ID:      "test-1",
			Type:    models.MediaTypeMovie,
			Title:   "Test",
			AddedAt: time.Time{}, // zero time → 90d from year 1 = way in the past
		}
		v := eval(engine, cfg, &media)
		assert.True(t, v.ShouldDelete())
	})
}

func TestEngine_ConcurrentAccess(t *testing.T) {
	cfg := mockConfig("90d", "120d", 14)
	excl := mockExclusions()
	engine := buildEngine(cfg, excl)

	mediaList := []models.Media{
		mockMedia("movie-1", models.MediaTypeMovie, 120, -1, false),
		mockMedia("movie-2", models.MediaTypeMovie, 30, -1, false),
		mockMedia("tv-1", models.MediaTypeTVShow, 150, -1, false),
	}

	done := make(chan bool, 3)
	for i := 0; i < 3; i++ {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("panic during concurrent evaluation: %v", r)
				}
				done <- true
			}()
			_ = deletionCandidates(engine, cfg, mediaList)
			_ = leavingSoon(engine, cfg, mediaList)
		}()
	}
	for i := 0; i < 3; i++ {
		<-done
	}
}

// ── User-based rules ──────────────────────────────────────────────────────────

func TestEngine_UserRule_UserIDMatching(t *testing.T) {
	userID := 123
	cfg := configWithUserRules([]config.AdvancedRule{
		{Name: "User 123 Cleanup", Type: "user", Enabled: true, Users: []config.UserRule{
			{UserID: &userID, Retention: "7d"},
		}},
	})
	excl := mockExclusions()
	engine := buildEngine(cfg, excl)

	t.Run("matches by user ID and applies custom retention", func(t *testing.T) {
		username := "testuser"
		email := "test@example.com"
		// Added 10 days ago, watched 10 days ago → past 7d retention
		media := mockMediaWithUser("movie-1", models.MediaTypeMovie, 10, 10, &userID, &username, &email)
		v := eval(engine, cfg, &media)

		assert.True(t, v.ShouldDelete())
		assert.Equal(t, SourceUserRule, v.ScheduleSource)

		expected := media.LastWatched.Add(7 * 24 * time.Hour)
		assert.WithinDuration(t, expected, v.DeleteAfter, time.Second)
	})

	t.Run("does not match different user ID", func(t *testing.T) {
		differentUserID := 456
		username := "other"
		email := "other@example.com"
		media := mockMediaWithUser("movie-2", models.MediaTypeMovie, 10, 10, &differentUserID, &username, &email)
		v := eval(engine, cfg, &media)

		// Falls through to standard 90d retention — 10 days is within
		assert.False(t, v.ShouldDelete())
		assert.Equal(t, SourceStandardRetention, v.ScheduleSource)
	})

	t.Run("nil user ID falls through to standard retention", func(t *testing.T) {
		username := "testuser"
		email := "test@example.com"
		media := mockMediaWithUser("movie-3", models.MediaTypeMovie, 10, 10, nil, &username, &email)
		v := eval(engine, cfg, &media)

		assert.False(t, v.ShouldDelete())
		assert.Equal(t, SourceStandardRetention, v.ScheduleSource)
	})
}

func TestEngine_UserRule_UsernameMatching(t *testing.T) {
	cfg := configWithUserRules([]config.AdvancedRule{
		{Name: "Username Cleanup", Type: "user", Enabled: true, Users: []config.UserRule{
			{Username: "JohnDoe", Retention: "14d"},
		}},
	})
	excl := mockExclusions()
	engine := buildEngine(cfg, excl)

	userID := 123
	email := "john@example.com"

	t.Run("exact match", func(t *testing.T) {
		username := "JohnDoe"
		media := mockMediaWithUser("movie-1", models.MediaTypeMovie, 20, 20, &userID, &username, &email)
		v := eval(engine, cfg, &media)
		assert.True(t, v.ShouldDelete())
		assert.Equal(t, SourceUserRule, v.ScheduleSource)
	})

	t.Run("case-insensitive lowercase", func(t *testing.T) {
		username := "johndoe"
		media := mockMediaWithUser("movie-2", models.MediaTypeMovie, 20, 20, &userID, &username, &email)
		v := eval(engine, cfg, &media)
		assert.True(t, v.ShouldDelete())
		assert.Equal(t, SourceUserRule, v.ScheduleSource)
	})

	t.Run("case-insensitive uppercase", func(t *testing.T) {
		username := "JOHNDOE"
		media := mockMediaWithUser("movie-3", models.MediaTypeMovie, 20, 20, &userID, &username, &email)
		v := eval(engine, cfg, &media)
		assert.True(t, v.ShouldDelete())
		assert.Equal(t, SourceUserRule, v.ScheduleSource)
	})

	t.Run("different username falls through", func(t *testing.T) {
		differentID := 456
		username := "JaneDoe"
		media := mockMediaWithUser("movie-4", models.MediaTypeMovie, 20, 20, &differentID, &username, &email)
		v := eval(engine, cfg, &media)
		assert.False(t, v.ShouldDelete())
		assert.Equal(t, SourceStandardRetention, v.ScheduleSource)
	})

	t.Run("nil username falls through", func(t *testing.T) {
		media := mockMediaWithUser("movie-5", models.MediaTypeMovie, 20, 20, &userID, nil, &email)
		v := eval(engine, cfg, &media)
		assert.False(t, v.ShouldDelete())
	})
}

func TestEngine_UserRule_EmailMatching(t *testing.T) {
	cfg := configWithUserRules([]config.AdvancedRule{
		{Name: "Email Cleanup", Type: "user", Enabled: true, Users: []config.UserRule{
			{Email: "foo@bar.com", Retention: "3d"},
		}},
	})
	excl := mockExclusions()
	engine := buildEngine(cfg, excl)

	userID := 789
	username := "foobar"

	t.Run("exact match", func(t *testing.T) {
		email := "foo@bar.com"
		media := mockMediaWithUser("movie-1", models.MediaTypeMovie, 5, 5, &userID, &username, &email)
		v := eval(engine, cfg, &media)
		assert.True(t, v.ShouldDelete())
		assert.Equal(t, SourceUserRule, v.ScheduleSource)
	})

	t.Run("case-insensitive uppercase", func(t *testing.T) {
		email := "FOO@BAR.COM"
		media := mockMediaWithUser("movie-2", models.MediaTypeMovie, 5, 5, &userID, &username, &email)
		v := eval(engine, cfg, &media)
		assert.True(t, v.ShouldDelete())
		assert.Equal(t, SourceUserRule, v.ScheduleSource)
	})

	t.Run("case-insensitive mixed", func(t *testing.T) {
		email := "Foo@Bar.Com"
		media := mockMediaWithUser("movie-3", models.MediaTypeMovie, 5, 5, &userID, &username, &email)
		v := eval(engine, cfg, &media)
		assert.True(t, v.ShouldDelete())
		assert.Equal(t, SourceUserRule, v.ScheduleSource)
	})

	t.Run("different email falls through", func(t *testing.T) {
		differentID := 999
		otherUsername := "other"
		email := "other@example.com"
		media := mockMediaWithUser("movie-4", models.MediaTypeMovie, 5, 5, &differentID, &otherUsername, &email)
		v := eval(engine, cfg, &media)
		assert.False(t, v.ShouldDelete())
	})

	t.Run("nil email falls through", func(t *testing.T) {
		media := mockMediaWithUser("movie-5", models.MediaTypeMovie, 5, 5, &userID, &username, nil)
		v := eval(engine, cfg, &media)
		assert.False(t, v.ShouldDelete())
	})
}

func TestEngine_UserRule_RequireWatched(t *testing.T) {
	userID := 100
	requireWatched := true
	cfg := configWithUserRules([]config.AdvancedRule{
		{Name: "Require Watched Rule", Type: "user", Enabled: true, Users: []config.UserRule{
			{UserID: &userID, Retention: "7d", RequireWatched: &requireWatched},
		}},
	})
	excl := mockExclusions()
	engine := buildEngine(cfg, excl)

	username := "testuser"
	email := "test@example.com"

	t.Run("watched past retention → delete", func(t *testing.T) {
		// Added 30 days ago, watched 10 days ago → past 7d
		media := mockMediaWithUser("movie-1", models.MediaTypeMovie, 30, 10, &userID, &username, &email)
		v := eval(engine, cfg, &media)
		assert.True(t, v.ShouldDelete())
		assert.Equal(t, SourceUserRule, v.ScheduleSource)
	})

	t.Run("watched within retention → keep", func(t *testing.T) {
		// Watched 3 days ago → within 7d
		media := mockMediaWithUser("movie-2", models.MediaTypeMovie, 30, 3, &userID, &username, &email)
		v := eval(engine, cfg, &media)
		assert.False(t, v.ShouldDelete())
		assert.False(t, v.IsProtected)
		assert.Equal(t, SourceUserRule, v.ScheduleSource)
	})

	t.Run("unwatched → protected", func(t *testing.T) {
		// Never watched
		media := mockMediaWithUser("movie-3", models.MediaTypeMovie, 30, -1, &userID, &username, &email)
		v := eval(engine, cfg, &media)
		assert.True(t, v.IsProtected)
		assert.Equal(t, ProtectedByRule, v.ProtectionReason)
	})
}

func TestEngine_UserRule_RequireWatchedFalse(t *testing.T) {
	userID := 200
	requireWatched := false
	cfg := configWithUserRules([]config.AdvancedRule{
		{Name: "No Require Watched", Type: "user", Enabled: true, Users: []config.UserRule{
			{UserID: &userID, Retention: "7d", RequireWatched: &requireWatched},
		}},
	})
	excl := mockExclusions()
	engine := buildEngine(cfg, excl)

	username := "testuser"
	email := "test@example.com"

	t.Run("unwatched past retention → delete", func(t *testing.T) {
		media := mockMediaWithUser("movie-1", models.MediaTypeMovie, 10, -1, &userID, &username, &email)
		v := eval(engine, cfg, &media)
		assert.True(t, v.ShouldDelete())
		assert.Equal(t, SourceUserRule, v.ScheduleSource)
	})

	t.Run("unwatched within retention → keep", func(t *testing.T) {
		media := mockMediaWithUser("movie-2", models.MediaTypeMovie, 5, -1, &userID, &username, &email)
		v := eval(engine, cfg, &media)
		assert.False(t, v.ShouldDelete())
		assert.Equal(t, SourceUserRule, v.ScheduleSource)
	})
}

func TestEngine_UserRule_MultipleUsers(t *testing.T) {
	userID1, userID2, userID3 := 301, 302, 303
	cfg := configWithUserRules([]config.AdvancedRule{
		{Name: "Multiple User Rules", Type: "user", Enabled: true, Users: []config.UserRule{
			{UserID: &userID1, Retention: "3d"},
			{UserID: &userID2, Retention: "14d"},
			{UserID: &userID3, Retention: "30d"},
		}},
	})
	excl := mockExclusions()
	engine := buildEngine(cfg, excl)

	t.Run("3d retention for user 301", func(t *testing.T) {
		username := "user301"
		email := "user301@example.com"
		media := mockMediaWithUser("movie-1", models.MediaTypeMovie, 5, 5, &userID1, &username, &email)
		v := eval(engine, cfg, &media)
		assert.True(t, v.ShouldDelete())
		assert.Equal(t, SourceUserRule, v.ScheduleSource)
	})

	t.Run("14d retention for user 302 — within", func(t *testing.T) {
		username := "user302"
		email := "user302@example.com"
		media := mockMediaWithUser("movie-2", models.MediaTypeMovie, 10, 10, &userID2, &username, &email)
		v := eval(engine, cfg, &media)
		assert.False(t, v.ShouldDelete())
		assert.Equal(t, SourceUserRule, v.ScheduleSource)
	})

	t.Run("30d retention for user 303 — within", func(t *testing.T) {
		username := "user303"
		email := "user303@example.com"
		media := mockMediaWithUser("movie-3", models.MediaTypeMovie, 20, 20, &userID3, &username, &email)
		v := eval(engine, cfg, &media)
		assert.False(t, v.ShouldDelete())
		assert.Equal(t, SourceUserRule, v.ScheduleSource)
	})
}

func TestEngine_UserRule_PriorityOverStandard(t *testing.T) {
	userID := 400
	cfg := configWithUserRules([]config.AdvancedRule{
		{Name: "User Priority Rule", Type: "user", Enabled: true, Users: []config.UserRule{
			{UserID: &userID, Retention: "7d"},
		}},
	})
	excl := mockExclusions()
	engine := buildEngine(cfg, excl)

	t.Run("user rule overrides standard 90d retention", func(t *testing.T) {
		username := "testuser"
		email := "test@example.com"
		// 10 days old — safe under 90d standard, but past 7d user rule
		media := mockMediaWithUser("movie-1", models.MediaTypeMovie, 10, 10, &userID, &username, &email)
		v := eval(engine, cfg, &media)

		assert.True(t, v.ShouldDelete())
		assert.Equal(t, SourceUserRule, v.ScheduleSource)
	})

	t.Run("standard retention applies when no user rule matches", func(t *testing.T) {
		differentUserID := 999
		username := "other"
		email := "other@example.com"
		media := mockMediaWithUser("movie-2", models.MediaTypeMovie, 10, 10, &differentUserID, &username, &email)
		v := eval(engine, cfg, &media)

		assert.False(t, v.ShouldDelete())
		assert.Equal(t, SourceStandardRetention, v.ScheduleSource)
	})
}

func TestEngine_UserRule_FallbackToStandard(t *testing.T) {
	userID := 500
	cfg := configWithUserRules([]config.AdvancedRule{
		{Name: "Specific User Only", Type: "user", Enabled: true, Users: []config.UserRule{
			{UserID: &userID, Retention: "7d"},
		}},
	})
	excl := mockExclusions()
	engine := buildEngine(cfg, excl)

	t.Run("non-matching user past standard retention → delete", func(t *testing.T) {
		differentUserID := 999
		username := "nonmatching"
		email := "nonmatching@example.com"
		media := mockMediaWithUser("movie-1", models.MediaTypeMovie, 200, 200, &differentUserID, &username, &email)
		v := eval(engine, cfg, &media)

		assert.True(t, v.ShouldDelete())
		assert.Equal(t, SourceStandardRetention, v.ScheduleSource)
	})

	t.Run("non-matching user within standard retention → keep", func(t *testing.T) {
		differentUserID := 888
		username := "another"
		email := "another@example.com"
		media := mockMediaWithUser("movie-2", models.MediaTypeMovie, 50, 50, &differentUserID, &username, &email)
		v := eval(engine, cfg, &media)

		assert.False(t, v.ShouldDelete())
		assert.Equal(t, SourceStandardRetention, v.ScheduleSource)
	})
}

func TestEngine_UserRule_Disabled(t *testing.T) {
	userID := 600
	cfg := configWithUserRules([]config.AdvancedRule{
		{Name: "Disabled Rule", Type: "user", Enabled: false, Users: []config.UserRule{
			{UserID: &userID, Retention: "1d"},
		}},
	})
	excl := mockExclusions()
	engine := buildEngine(cfg, excl)

	username := "testuser"
	email := "test@example.com"
	media := mockMediaWithUser("movie-1", models.MediaTypeMovie, 10, 10, &userID, &username, &email)
	v := eval(engine, cfg, &media)

	// Disabled rule ignored → standard 90d retention → 10 days is within
	assert.False(t, v.ShouldDelete())
	assert.Equal(t, SourceStandardRetention, v.ScheduleSource)
}

func TestEngine_UserRule_FirstMatchWins(t *testing.T) {
	userID := 700
	cfg := configWithUserRules([]config.AdvancedRule{
		{Name: "First Rule", Type: "user", Enabled: true, Users: []config.UserRule{
			{UserID: &userID, Retention: "5d"},
		}},
		{Name: "Second Rule", Type: "user", Enabled: true, Users: []config.UserRule{
			{UserID: &userID, Retention: "30d"},
		}},
	})
	excl := mockExclusions()
	engine := buildEngine(cfg, excl)

	username := "testuser"
	email := "test@example.com"
	media := mockMediaWithUser("movie-1", models.MediaTypeMovie, 10, 10, &userID, &username, &email)
	v := eval(engine, cfg, &media)

	// First rule (5d) wins → should delete (10 > 5)
	assert.True(t, v.ShouldDelete())
	assert.Equal(t, SourceUserRule, v.ScheduleSource)
	assert.Equal(t, "First Rule", v.SchedulingRule)
}

// ── Tag-based rules ───────────────────────────────────────────────────────────

func TestEngine_TagRule_NoRulesConfigured(t *testing.T) {
	cfg := mockConfig("90d", "120d", 14)
	excl := mockExclusions()
	engine := buildEngine(cfg, excl)

	media := mockMedia("movie-1", models.MediaTypeMovie, 30, -1, false)
	media.Tags = []string{"test-tag"}
	v := eval(engine, cfg, &media)

	// Falls through to standard retention
	assert.False(t, v.ShouldDelete())
	assert.Equal(t, SourceStandardRetention, v.ScheduleSource)
}

func TestEngine_TagRule_WithinRetention(t *testing.T) {
	cfg := mockConfig("90d", "120d", 14)
	cfg.AdvancedRules = []config.AdvancedRule{
		{Name: "Demo Content", Type: "tag", Enabled: true, Tag: "demo", Retention: "30d"},
	}
	excl := mockExclusions()
	engine := buildEngine(cfg, excl)

	media := mockMedia("movie-1", models.MediaTypeMovie, 15, -1, false)
	media.Tags = []string{"demo"}
	v := eval(engine, cfg, &media)

	assert.False(t, v.ShouldDelete())
	assert.False(t, v.IsProtected)
	assert.Equal(t, SourceTagRule, v.ScheduleSource)
	assert.Equal(t, "Demo Content", v.SchedulingRule)

	expected := media.AddedAt.Add(30 * 24 * time.Hour)
	assert.WithinDuration(t, expected, v.DeleteAfter, time.Second)
}

func TestEngine_TagRule_PastRetention(t *testing.T) {
	cfg := mockConfig("90d", "120d", 14)
	cfg.AdvancedRules = []config.AdvancedRule{
		{Name: "Demo Content", Type: "tag", Enabled: true, Tag: "demo", Retention: "30d"},
	}
	excl := mockExclusions()
	engine := buildEngine(cfg, excl)

	media := mockMedia("movie-1", models.MediaTypeMovie, 45, -1, false)
	media.Tags = []string{"demo"}
	v := eval(engine, cfg, &media)

	assert.True(t, v.ShouldDelete())
	assert.Equal(t, SourceTagRule, v.ScheduleSource)

	expected := media.AddedAt.Add(30 * 24 * time.Hour)
	assert.WithinDuration(t, expected, v.DeleteAfter, time.Second)
}

func TestEngine_TagRule_CaseInsensitive(t *testing.T) {
	cfg := mockConfig("90d", "120d", 14)
	cfg.AdvancedRules = []config.AdvancedRule{
		{Name: "Test Content", Type: "tag", Enabled: true, Tag: "DEMO", Retention: "30d"},
	}
	excl := mockExclusions()
	engine := buildEngine(cfg, excl)

	media := mockMedia("movie-1", models.MediaTypeMovie, 15, -1, false)
	media.Tags = []string{"demo"} // lowercase vs uppercase rule tag
	v := eval(engine, cfg, &media)

	assert.False(t, v.ShouldDelete())
	assert.Equal(t, SourceTagRule, v.ScheduleSource)
}

func TestEngine_TagRule_MultipleTags(t *testing.T) {
	cfg := mockConfig("90d", "120d", 14)
	cfg.AdvancedRules = []config.AdvancedRule{
		{Name: "Demo Content", Type: "tag", Enabled: true, Tag: "demo", Retention: "30d"},
	}
	excl := mockExclusions()
	engine := buildEngine(cfg, excl)

	media := mockMedia("movie-1", models.MediaTypeMovie, 15, -1, false)
	media.Tags = []string{"other-tag", "demo", "another-tag"}
	v := eval(engine, cfg, &media)

	assert.Equal(t, SourceTagRule, v.ScheduleSource)
}

func TestEngine_TagRule_NoMatch(t *testing.T) {
	cfg := mockConfig("90d", "120d", 14)
	cfg.AdvancedRules = []config.AdvancedRule{
		{Name: "Demo Content", Type: "tag", Enabled: true, Tag: "demo", Retention: "30d"},
	}
	excl := mockExclusions()
	engine := buildEngine(cfg, excl)

	media := mockMedia("movie-1", models.MediaTypeMovie, 15, -1, false)
	media.Tags = []string{"other-tag"}
	v := eval(engine, cfg, &media)

	// Falls through to standard retention
	assert.Equal(t, SourceStandardRetention, v.ScheduleSource)
}

func TestEngine_TagRule_Disabled(t *testing.T) {
	cfg := mockConfig("90d", "120d", 14)
	cfg.AdvancedRules = []config.AdvancedRule{
		{Name: "Demo Content", Type: "tag", Enabled: false, Tag: "demo", Retention: "30d"},
	}
	excl := mockExclusions()
	engine := buildEngine(cfg, excl)

	media := mockMedia("movie-1", models.MediaTypeMovie, 15, -1, false)
	media.Tags = []string{"demo"}
	v := eval(engine, cfg, &media)

	// Disabled rule → falls through to standard retention
	assert.Equal(t, SourceStandardRetention, v.ScheduleSource)
}

func TestEngine_TagRule_UsesLastWatched(t *testing.T) {
	cfg := mockConfig("90d", "120d", 14)
	cfg.AdvancedRules = []config.AdvancedRule{
		{Name: "Demo Content", Type: "tag", Enabled: true, Tag: "demo", Retention: "30d"},
	}
	excl := mockExclusions()
	engine := buildEngine(cfg, excl)

	// Added 100 days ago, watched 10 days ago
	media := mockMedia("movie-1", models.MediaTypeMovie, 100, 10, false)
	media.Tags = []string{"demo"}
	v := eval(engine, cfg, &media)

	assert.False(t, v.ShouldDelete())
	assert.Equal(t, SourceTagRule, v.ScheduleSource)

	expected := media.LastWatched.Add(30 * 24 * time.Hour)
	assert.WithinDuration(t, expected, v.DeleteAfter, time.Second)
}

func TestEngine_TagRule_PriorityOverStandard(t *testing.T) {
	cfg := mockConfig("90d", "120d", 14)
	cfg.AdvancedRules = []config.AdvancedRule{
		{Name: "Quick Delete", Type: "tag", Enabled: true, Tag: "quick-delete", Retention: "7d"},
	}
	excl := mockExclusions()
	engine := buildEngine(cfg, excl)

	// 30 days old — within standard 90d but past tag rule 7d
	media := mockMedia("movie-1", models.MediaTypeMovie, 30, -1, false)
	media.Tags = []string{"quick-delete"}
	v := eval(engine, cfg, &media)

	assert.True(t, v.ShouldDelete())
	assert.Equal(t, SourceTagRule, v.ScheduleSource)
}

func TestEngine_TagRule_FirstMatchWins(t *testing.T) {
	cfg := mockConfig("90d", "120d", 14)
	cfg.AdvancedRules = []config.AdvancedRule{
		{Name: "Quick Delete", Type: "tag", Enabled: true, Tag: "quick", Retention: "7d"},
		{Name: "Slow Delete", Type: "tag", Enabled: true, Tag: "slow", Retention: "365d"},
	}
	excl := mockExclusions()
	engine := buildEngine(cfg, excl)

	media := mockMedia("movie-1", models.MediaTypeMovie, 30, -1, false)
	media.Tags = []string{"quick", "slow"}
	v := eval(engine, cfg, &media)

	assert.True(t, v.ShouldDelete())
	assert.Equal(t, "Quick Delete", v.SchedulingRule)
}

func TestEngine_TagRule_TVShow(t *testing.T) {
	cfg := mockConfig("90d", "120d", 14)
	cfg.AdvancedRules = []config.AdvancedRule{
		{Name: "Test Shows", Type: "tag", Enabled: true, Tag: "test-show", Retention: "14d"},
	}
	excl := mockExclusions()
	engine := buildEngine(cfg, excl)

	media := mockMedia("tv-1", models.MediaTypeTVShow, 20, -1, false)
	media.Tags = []string{"test-show"}
	v := eval(engine, cfg, &media)

	assert.True(t, v.ShouldDelete())
	assert.Equal(t, SourceTagRule, v.ScheduleSource)

	expected := media.AddedAt.Add(14 * 24 * time.Hour)
	assert.WithinDuration(t, expected, v.DeleteAfter, time.Second)
}

func TestEngine_TagRule_ExclusionOverridesTag(t *testing.T) {
	cfg := mockConfig("90d", "120d", 14)
	cfg.AdvancedRules = []config.AdvancedRule{
		{Name: "Demo Content", Type: "tag", Enabled: true, Tag: "demo", Retention: "7d"},
	}
	excl := mockExclusions()
	excl.Add(storage.ExclusionItem{ExternalID: "movie-1", Reason: "user keep"})
	engine := buildEngine(cfg, excl)

	media := mockMedia("movie-1", models.MediaTypeMovie, 30, -1, false)
	media.Tags = []string{"demo"}
	v := eval(engine, cfg, &media)

	// Exclusion wins over tag rule
	assert.True(t, v.IsProtected)
	assert.Equal(t, ProtectedExcluded, v.ProtectionReason)
}

func TestEngine_TagRule_OverridesRequestedStatus(t *testing.T) {
	cfg := mockConfig("90d", "120d", 14)
	cfg.AdvancedRules = []config.AdvancedRule{
		{Name: "Demo Content", Type: "tag", Enabled: true, Tag: "demo", Retention: "7d"},
	}
	excl := mockExclusions()
	engine := buildEngine(cfg, excl)

	// Requested AND has tag — tag rule should win (advanced rules exist)
	media := mockMedia("movie-1", models.MediaTypeMovie, 30, -1, true)
	media.Tags = []string{"demo"}
	v := eval(engine, cfg, &media)

	assert.True(t, v.ShouldDelete())
	assert.Equal(t, SourceTagRule, v.ScheduleSource)
}

func TestEngine_TagRule_RetentionZeroDays_ImmediateDeletion(t *testing.T) {
	cfg := mockConfig("90d", "120d", 14)
	cfg.AdvancedRules = []config.AdvancedRule{
		{Name: "Delete Now", Type: "tag", Enabled: true, Tag: "delete-now", Retention: "0d"},
	}
	excl := mockExclusions()
	engine := buildEngine(cfg, excl)

	// "0d" means zero retention — schedule at baseTime (always in the past → immediate deletion)
	media := mockMedia("movie-1", models.MediaTypeMovie, 1, -1, false) // added just 1 day ago
	media.Tags = []string{"delete-now"}
	v := eval(engine, cfg, &media)

	// Must NOT be protected
	assert.False(t, v.IsProtected, "0d retention should NOT protect the item")
	// Must be scheduled (deleteAfter = addedAt + 0 = addedAt, which is in the past)
	assert.False(t, v.DeleteAfter.IsZero(), "0d retention should set a deletion date")
	assert.True(t, v.ShouldDelete(), "0d retention should result in immediate deletion")
	assert.Equal(t, SourceTagRule, v.ScheduleSource)
	assert.Equal(t, "Delete Now", v.SchedulingRule)
}

func TestEngine_TagRule_RetentionNever_Protects(t *testing.T) {
	cfg := mockConfig("90d", "120d", 14)
	cfg.AdvancedRules = []config.AdvancedRule{
		{Name: "Keep Forever", Type: "tag", Enabled: true, Tag: "keep", Retention: "never"},
	}
	excl := mockExclusions()
	engine := buildEngine(cfg, excl)

	// Very old media with "keep" tag
	media := mockMedia("movie-1", models.MediaTypeMovie, 500, 400, false)
	media.Tags = []string{"keep"}
	v := eval(engine, cfg, &media)

	assert.True(t, v.IsProtected)
	assert.Equal(t, ProtectedByRule, v.ProtectionReason)
	assert.Equal(t, "Keep Forever", v.ProtectingRule)
}

func TestEngine_TagRule_RequireWatched_Protects(t *testing.T) {
	cfg := mockConfig("90d", "120d", 14)
	cfg.AdvancedRules = []config.AdvancedRule{
		{Name: "Watch First", Type: "tag", Enabled: true, Tag: "watch-first", Retention: "7d", RequireWatched: true},
	}
	excl := mockExclusions()
	engine := buildEngine(cfg, excl)

	// Unwatched media with tag — should be protected
	media := mockMedia("movie-1", models.MediaTypeMovie, 30, -1, false)
	media.Tags = []string{"watch-first"}
	v := eval(engine, cfg, &media)

	assert.True(t, v.IsProtected)
	assert.Equal(t, ProtectedByRule, v.ProtectionReason)
}

// ── DiskThreshold — engine integration (gate wired through RulesEngine) ───────

// mockDiskMonitor implements the DiskMonitor interface for testing.
type mockDiskMonitor struct {
	status *DiskStatus
}

func (m *mockDiskMonitor) GetStatus() *DiskStatus { return m.status }

// buildEngineWithDisk constructs a RulesEngine with a wired DiskMonitor,
// exercising the full Evaluate() path including getDiskStatus().
func buildEngineWithDisk(cfg *config.Config, exclusions *storage.ExclusionsFile, monitor DiskMonitor) *RulesEngine {
	e := buildEngine(cfg, exclusions)
	e.diskMonitor = monitor
	return e
}

// evalFull calls engine.Evaluate() (the public method that reads config.Get() and
// getDiskStatus()) after setting the global config so config.Get() is non-nil.
// t.Cleanup restores the previous global config.
func evalFull(t *testing.T, e *RulesEngine, cfg *config.Config, media *models.Media) RuleVerdict {
	t.Helper()
	config.SetTestConfig(cfg)
	t.Cleanup(func() { config.SetTestConfig(nil) })
	return e.Evaluate(media)
}

// evalFullPreview is like evalFull but calls EvaluateForPreview.
func evalFullPreview(t *testing.T, e *RulesEngine, cfg *config.Config, media *models.Media) RuleVerdict {
	t.Helper()
	config.SetTestConfig(cfg)
	t.Cleanup(func() { config.SetTestConfig(nil) })
	return e.EvaluateForPreview(media)
}

func TestEngine_DiskThreshold_GateBlocksWhenDiskOK(t *testing.T) {
	cfg := mockConfig("90d", "120d", 14)
	excl := mockExclusions()
	monitor := &mockDiskMonitor{status: &DiskStatus{
		Enabled:           true,
		FreeSpaceGB:       600,
		ThresholdGB:       500,
		ThresholdBreached: false,
		CheckSource:       "radarr",
	}}
	engine := buildEngineWithDisk(cfg, excl, monitor)

	// Movie added 120 days ago — would normally be past 90d retention
	media := mockMedia("movie-1", models.MediaTypeMovie, 120, -1, false)
	v := evalFull(t, engine, cfg, &media)

	assert.True(t, v.IsProtected, "disk OK should protect media from deletion")
	assert.Equal(t, ProtectedDiskOK, v.ProtectionReason)
	assert.Equal(t, "disk_threshold", v.ProtectingRule)
	assert.True(t, v.DeleteAfter.IsZero(), "no deletion date should be set when disk is OK")
}

func TestEngine_DiskThreshold_GateOpensWhenBreached(t *testing.T) {
	cfg := mockConfig("90d", "120d", 14)
	excl := mockExclusions()
	monitor := &mockDiskMonitor{status: &DiskStatus{
		Enabled:           true,
		FreeSpaceGB:       400,
		ThresholdGB:       500,
		ThresholdBreached: true,
		CheckSource:       "radarr",
	}}
	engine := buildEngineWithDisk(cfg, excl, monitor)

	// Movie added 120 days ago — past 90d retention, gate is open
	media := mockMedia("movie-1", models.MediaTypeMovie, 120, -1, false)
	v := evalFull(t, engine, cfg, &media)

	assert.False(t, v.IsProtected, "breached threshold should allow deletion")
	assert.False(t, v.DeleteAfter.IsZero(), "deletion date should be set when threshold breached")
	assert.True(t, v.ShouldDelete(), "overdue media should be scheduled for deletion")
	assert.Equal(t, SourceStandardRetention, v.ScheduleSource)
}

func TestEngine_DiskThreshold_DisabledIsTransparent(t *testing.T) {
	cfg := mockConfig("90d", "120d", 14)
	excl := mockExclusions()
	// DiskStatus present but Enabled=false — should be fully transparent
	monitor := &mockDiskMonitor{status: &DiskStatus{
		Enabled:           false,
		ThresholdBreached: false,
	}}
	engine := buildEngineWithDisk(cfg, excl, monitor)

	// Movie past retention — standard rule should apply
	media := mockMedia("movie-1", models.MediaTypeMovie, 120, -1, false)
	v := evalFull(t, engine, cfg, &media)

	assert.False(t, v.IsProtected)
	assert.True(t, v.ShouldDelete(), "disabled disk gate should not block standard retention")
	assert.Equal(t, SourceStandardRetention, v.ScheduleSource)
}

func TestEngine_DiskThreshold_NilMonitorIsTransparent(t *testing.T) {
	cfg := mockConfig("90d", "120d", 14)
	excl := mockExclusions()
	// nil diskMonitor = feature disabled entirely
	engine := buildEngineWithDisk(cfg, excl, nil)

	media := mockMedia("movie-1", models.MediaTypeMovie, 120, -1, false)
	v := evalFull(t, engine, cfg, &media)

	assert.False(t, v.IsProtected)
	assert.True(t, v.ShouldDelete(), "nil disk monitor should not block standard retention")
	assert.Equal(t, SourceStandardRetention, v.ScheduleSource)
}

func TestEngine_DiskThreshold_ExclusionBeatsGate(t *testing.T) {
	cfg := mockConfig("90d", "120d", 14)
	excl := mockExclusions()
	excl.Add(storage.ExclusionItem{ExternalID: "movie-1", Reason: "user keep"})

	// Disk is OK — gate would protect, but exclusion should win first
	monitor := &mockDiskMonitor{status: &DiskStatus{
		Enabled:           true,
		ThresholdBreached: false,
	}}
	engine := buildEngineWithDisk(cfg, excl, monitor)

	media := mockMedia("movie-1", models.MediaTypeMovie, 120, -1, false)
	v := evalFull(t, engine, cfg, &media)

	assert.True(t, v.IsProtected)
	// Exclusion rule runs before disk threshold — it must win
	assert.Equal(t, ProtectedExcluded, v.ProtectionReason)
	assert.Equal(t, "exclusion", v.ProtectingRule)
}

func TestEngine_DiskThreshold_AppliesToTVShows(t *testing.T) {
	cfg := mockConfig("90d", "120d", 14)
	excl := mockExclusions()
	monitor := &mockDiskMonitor{status: &DiskStatus{
		Enabled:           true,
		FreeSpaceGB:       600,
		ThresholdGB:       500,
		ThresholdBreached: false,
	}}
	engine := buildEngineWithDisk(cfg, excl, monitor)

	// TV show past 120d retention — gate should block it too (ScopeAll)
	media := mockMedia("tv-1", models.MediaTypeTVShow, 150, -1, false)
	v := evalFull(t, engine, cfg, &media)

	assert.True(t, v.IsProtected, "disk gate should apply to TV shows (ScopeAll)")
	assert.Equal(t, ProtectedDiskOK, v.ProtectionReason)
	assert.Equal(t, "disk_threshold", v.ProtectingRule)
}

func TestEngine_DiskThreshold_PreviewBypassesGate(t *testing.T) {
	cfg := mockConfig("90d", "120d", 14)
	excl := mockExclusions()
	// Disk is healthy — gate would block in normal Evaluate()
	monitor := &mockDiskMonitor{status: &DiskStatus{
		Enabled:           true,
		FreeSpaceGB:       600,
		ThresholdGB:       500,
		ThresholdBreached: false,
	}}
	engine := buildEngineWithDisk(cfg, excl, monitor)

	// Movie past retention — EvaluateForPreview must ignore the gate
	media := mockMedia("movie-1", models.MediaTypeMovie, 120, -1, false)

	// Normal Evaluate: gate blocks
	v := evalFull(t, engine, cfg, &media)
	assert.True(t, v.IsProtected, "Evaluate should be blocked by disk gate")

	// Preview: gate bypassed — item should be scheduled
	vPreview := evalFullPreview(t, engine, cfg, &media)
	assert.False(t, vPreview.IsProtected, "EvaluateForPreview must bypass disk gate")
	assert.False(t, vPreview.DeleteAfter.IsZero(), "preview should show deletion date regardless of disk state")
	assert.Equal(t, SourceStandardRetention, vPreview.ScheduleSource)
}

func TestEngine_DiskThreshold_GateBlocksWithinRetention(t *testing.T) {
	cfg := mockConfig("90d", "120d", 14)
	excl := mockExclusions()
	monitor := &mockDiskMonitor{status: &DiskStatus{
		Enabled:           true,
		ThresholdBreached: false,
	}}
	engine := buildEngineWithDisk(cfg, excl, monitor)

	// Movie within retention — would be scheduled but not yet due
	// With disk OK gate, it should still be protected (gate fires first)
	media := mockMedia("movie-1", models.MediaTypeMovie, 30, -1, false)
	v := evalFull(t, engine, cfg, &media)

	assert.True(t, v.IsProtected)
	assert.Equal(t, ProtectedDiskOK, v.ProtectionReason)
}

func TestEngine_DiskThreshold_BreachedAllowsMultipleMediaTypes(t *testing.T) {
	cfg := mockConfig("90d", "120d", 14)
	excl := mockExclusions()
	monitor := &mockDiskMonitor{status: &DiskStatus{
		Enabled:           true,
		FreeSpaceGB:       200,
		ThresholdGB:       500,
		ThresholdBreached: true,
	}}
	engine := buildEngineWithDisk(cfg, excl, monitor)

	movie := mockMedia("movie-1", models.MediaTypeMovie, 120, -1, false)
	tv := mockMedia("tv-1", models.MediaTypeTVShow, 150, -1, false)

	vMovie := evalFull(t, engine, cfg, &movie)
	vTV := evalFull(t, engine, cfg, &tv)

	assert.True(t, vMovie.ShouldDelete(), "movie should be deletable when threshold breached")
	assert.True(t, vTV.ShouldDelete(), "TV show should be deletable when threshold breached")
}

// ── DiskThreshold (shell — always nil DiskStatus in tests) ────────────────────

func TestDiskThresholdRule_NilStatus(t *testing.T) {
	r := NewDiskThresholdRule()
	ctx := EvalContext{
		Media:      &models.Media{},
		Config:     &config.Config{},
		DiskStatus: nil,
	}
	assert.Nil(t, r.Protect(ctx), "nil DiskStatus must return nil (no protection)")
}

func TestDiskThresholdRule_DisabledFeature(t *testing.T) {
	r := NewDiskThresholdRule()
	ctx := EvalContext{
		Media:  &models.Media{},
		Config: &config.Config{},
		DiskStatus: &DiskStatus{
			Enabled: false,
		},
	}
	assert.Nil(t, r.Protect(ctx))
}

func TestDiskThresholdRule_ThresholdNotBreached(t *testing.T) {
	r := NewDiskThresholdRule()
	ctx := EvalContext{
		Media:  &models.Media{},
		Config: &config.Config{},
		DiskStatus: &DiskStatus{
			Enabled:           true,
			ThresholdBreached: false,
		},
	}
	result := r.Protect(ctx)
	require.NotNil(t, result)
	assert.Equal(t, ProtectedDiskOK, *result)
}

func TestDiskThresholdRule_ThresholdBreached(t *testing.T) {
	r := NewDiskThresholdRule()
	ctx := EvalContext{
		Media:  &models.Media{},
		Config: &config.Config{},
		DiskStatus: &DiskStatus{
			Enabled:           true,
			ThresholdBreached: true,
		},
	}
	assert.Nil(t, r.Protect(ctx), "breached threshold must return nil (allow deletion)")
}
