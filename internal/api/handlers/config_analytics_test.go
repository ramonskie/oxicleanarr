package handlers

import (
	"testing"

	"github.com/ramonskie/oxicleanarr/internal/config"
	"github.com/stretchr/testify/assert"
)

// TestApplyAnalyticsUpdate verifies a partial analytics update merges into the
// existing config instead of zeroing untouched fields.
func TestApplyAnalyticsUpdate(t *testing.T) {
	dst := config.AnalyticsConfig{
		Enabled:             true,
		StaleDays:           90,
		ROIPeriodDays:       90,
		IncludeAgeDecay:     true,
		SuggestDeletionDays: 180,
		ValueThresholds: config.ValueThresholdsConfig{
			Movie:   config.ValueThreshold{Low: 0.1, High: 0.5},
			Episode: config.ValueThreshold{Low: 0.5, High: 2.0},
			Show:    config.ValueThreshold{Low: 0.3, High: 1.0},
		},
	}

	enabled := false
	stale := 30
	low := 0.2
	applyAnalyticsUpdate(&dst, &UpdateAnalyticsConfig{
		Enabled:   &enabled,
		StaleDays: &stale,
		ValueThresholds: &UpdateValueThresholdsConfig{
			Movie: &UpdateValueThreshold{Low: &low},
		},
	})

	assert.False(t, dst.Enabled)
	assert.Equal(t, 30, dst.StaleDays)
	// Untouched fields must be preserved.
	assert.Equal(t, 90, dst.ROIPeriodDays)
	assert.True(t, dst.IncludeAgeDecay)
	assert.Equal(t, 180, dst.SuggestDeletionDays)
	assert.InDelta(t, 0.2, dst.ValueThresholds.Movie.Low, 0.001)
	assert.InDelta(t, 0.5, dst.ValueThresholds.Movie.High, 0.001)
	assert.InDelta(t, 0.5, dst.ValueThresholds.Episode.Low, 0.001)
}
