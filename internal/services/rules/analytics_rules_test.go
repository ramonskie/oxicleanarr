package rules

import (
	"testing"
	"time"

	"github.com/ramonskie/oxicleanarr/internal/config"
	"github.com/ramonskie/oxicleanarr/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func analyticsConfig() *config.Config {
	return &config.Config{
		Integrations: config.IntegrationsConfig{
			Tracearr: config.TracearrConfig{
				BaseIntegrationConfig: config.BaseIntegrationConfig{Enabled: true},
			},
		},
		Analytics: config.AnalyticsConfig{
			Enabled:             true,
			StaleDays:           90,
			IncludeAgeDecay:     true,
			SuggestDeletionDays: 180,
			ValueThresholds: config.ValueThresholdsConfig{
				Movie:   config.ValueThreshold{Low: 0.1, High: 0.5},
				Episode: config.ValueThreshold{Low: 0.5, High: 2.0},
				Show:    config.ValueThreshold{Low: 0.3, High: 1.0},
			},
		},
	}
}

func TestStaleRule(t *testing.T) {
	now := time.Now()
	cfg := analyticsConfig()

	t.Run("never-watched item is stale and due", func(t *testing.T) {
		media := &models.Media{Type: models.MediaTypeMovie, AddedAt: now.AddDate(0, 0, -100)}
		rule := NewStaleRule(config.AdvancedRule{Name: "stale", Type: "stale", Enabled: true})
		ctx := EvalContext{Media: media, Config: cfg}

		assert.Nil(t, rule.Protect(ctx))

		due, source := rule.Schedule(ctx)
		assert.Equal(t, SourceStaleRule, source)
		require.False(t, due.IsZero())
		assert.True(t, due.Before(now), "due date should already be in the past")
	})

	t.Run("recently watched item is protected", func(t *testing.T) {
		media := &models.Media{
			Type:        models.MediaTypeMovie,
			AddedAt:     now.AddDate(0, 0, -300),
			LastWatched: now.AddDate(0, 0, -10),
		}
		rule := NewStaleRule(config.AdvancedRule{Name: "stale", Type: "stale", Enabled: true})
		ctx := EvalContext{Media: media, Config: cfg}

		assert.NotNil(t, rule.Protect(ctx))
		due, source := rule.Schedule(ctx)
		assert.True(t, due.IsZero())
		assert.Equal(t, ScheduleSource(0), source)
	})

	t.Run("grace period delays an already-stale item", func(t *testing.T) {
		media := &models.Media{
			Type:        models.MediaTypeMovie,
			AddedAt:     now.AddDate(0, 0, -300),
			LastWatched: now.AddDate(0, 0, -100),
		}
		rule := NewStaleRule(config.AdvancedRule{
			Name: "stale", Type: "stale", Enabled: true, Retention: "7d",
		})
		ctx := EvalContext{Media: media, Config: cfg}

		assert.Nil(t, rule.Protect(ctx))
		due, source := rule.Schedule(ctx)
		assert.Equal(t, SourceStaleRule, source)
		require.False(t, due.IsZero())
		assert.True(t, due.After(now), "grace period should push the due date into the future")
	})

	t.Run("disabled analytics makes the rule inert", func(t *testing.T) {
		media := &models.Media{Type: models.MediaTypeMovie, AddedAt: now.AddDate(0, 0, -300)}
		rule := NewStaleRule(config.AdvancedRule{Name: "stale", Type: "stale", Enabled: true})
		disabled := analyticsConfig()
		disabled.Analytics.Enabled = false
		ctx := EvalContext{Media: media, Config: disabled}

		assert.Nil(t, rule.Protect(ctx))
		due, source := rule.Schedule(ctx)
		assert.True(t, due.IsZero())
		assert.Equal(t, ScheduleSource(0), source)
	})

	t.Run("per-rule threshold overrides analytics default", func(t *testing.T) {
		media := &models.Media{
			Type:        models.MediaTypeMovie,
			AddedAt:     now.AddDate(0, 0, -300),
			LastWatched: now.AddDate(0, 0, -40),
		}
		// 30d threshold makes a 40d-old watch stale, unlike the 90d default.
		rule := NewStaleRule(config.AdvancedRule{Name: "stale", Type: "stale", Enabled: true, StaleDays: 30})
		ctx := EvalContext{Media: media, Config: cfg}

		assert.Nil(t, rule.Protect(ctx))
	})
}

const gigabyte = 1 << 30

func TestRoiRule(t *testing.T) {
	now := time.Now()
	cfg := analyticsConfig()

	t.Run("never-watched low-value item is scheduled", func(t *testing.T) {
		media := &models.Media{
			Type:     models.MediaTypeMovie,
			AddedAt:  now.AddDate(0, 0, -200),
			FileSize: 10 * gigabyte,
		}
		rule := NewROIRule(config.AdvancedRule{Name: "roi", Type: "roi", Enabled: true})
		ctx := EvalContext{Media: media, Config: cfg}

		assert.Nil(t, rule.Protect(ctx))
		due, source := rule.Schedule(ctx)
		assert.Equal(t, SourceRoiRule, source)
		require.False(t, due.IsZero())
		assert.True(t, due.Before(now))
	})

	t.Run("high-value item is protected", func(t *testing.T) {
		media := &models.Media{
			Type:              models.MediaTypeMovie,
			AddedAt:           now.AddDate(0, 0, -200),
			LastWatched:       now.AddDate(0, 0, -10),
			FileSize:          10 * gigabyte,
			TotalWatchSeconds: 5 * 3600, // 0.5 hrs/GB => moderate
		}
		rule := NewROIRule(config.AdvancedRule{Name: "roi", Type: "roi", Enabled: true})
		ctx := EvalContext{Media: media, Config: cfg}

		assert.NotNil(t, rule.Protect(ctx))
		due, _ := rule.Schedule(ctx)
		assert.True(t, due.IsZero())
	})

	t.Run("no stats provider makes the rule inert", func(t *testing.T) {
		media := &models.Media{
			Type: models.MediaTypeMovie, AddedAt: now.AddDate(0, 0, -300), FileSize: 10 * gigabyte,
		}
		rule := NewROIRule(config.AdvancedRule{Name: "roi", Type: "roi", Enabled: true})
		noProvider := analyticsConfig()
		noProvider.Integrations = config.IntegrationsConfig{}
		ctx := EvalContext{Media: media, Config: noProvider}

		assert.Nil(t, rule.Protect(ctx))
		due, _ := rule.Schedule(ctx)
		assert.True(t, due.IsZero())
	})

	t.Run("min hours per GB override flags a moderate item", func(t *testing.T) {
		media := &models.Media{
			Type:              models.MediaTypeMovie,
			AddedAt:           now.AddDate(0, 0, -300),
			LastWatched:       now.AddDate(0, 0, -200),
			FileSize:          10 * gigabyte,
			TotalWatchSeconds: 3 * 3600, // 0.3 hrs/GB => moderate by category
		}
		rule := NewROIRule(config.AdvancedRule{
			Name: "roi", Type: "roi", Enabled: true, MinWatchHoursPerGB: 0.5,
		})
		ctx := EvalContext{Media: media, Config: cfg}

		assert.Nil(t, rule.Protect(ctx))
	})
}
