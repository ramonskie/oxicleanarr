package analytics

import (
	"testing"
	"time"

	"github.com/ramonskie/oxicleanarr/internal/config"
	"github.com/stretchr/testify/assert"
)

func testAnalyticsConfig() config.AnalyticsConfig {
	return config.AnalyticsConfig{
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
}

const gb = 1 << 30

func TestROI(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	cfg := testAnalyticsConfig()

	tests := []struct {
		name         string
		input        ROIInput
		wantPerGB    float64
		wantCategory ValueCategory
		wantScore    float64
		wantSuggest  bool
		wantDays     int
	}{
		{
			name: "movie at 0.5 hrs/GB is moderate and scores 100",
			input: ROIInput{
				FileSizeBytes: 10 * gb, TotalWatchSeconds: 5 * 3600,
				LastWatched: now.AddDate(0, 0, -10), Kind: KindMovie,
			},
			wantPerGB: 0.5, wantCategory: ValueModerate, wantScore: 100, wantDays: 10,
		},
		{
			name: "movie above 0.5 hrs/GB is high value",
			input: ROIInput{
				FileSizeBytes: 10 * gb, TotalWatchSeconds: 6 * 3600,
				LastWatched: now.AddDate(0, 0, -10), Kind: KindMovie,
			},
			wantPerGB: 0.6, wantCategory: ValueHigh, wantScore: 100, wantDays: 10,
		},
		{
			name: "movie below 0.1 hrs/GB is low value but recent enough to keep",
			input: ROIInput{
				FileSizeBytes: 10 * gb, TotalWatchSeconds: 1800, // 0.5h => 0.05/GB
				LastWatched: now.AddDate(0, 0, -10), Kind: KindMovie,
			},
			wantPerGB: 0.05, wantCategory: ValueLow, wantScore: 37, wantDays: 10,
		},
		{
			name: "never-watched low-value movie past window suggests deletion",
			input: ROIInput{
				FileSizeBytes: 10 * gb, AddedAt: now.AddDate(0, 0, -200), Kind: KindMovie,
			},
			wantCategory: ValueLow, wantScore: 0, wantSuggest: true, wantDays: -1,
		},
		{
			name: "never-watched low-value movie inside window is not suggested",
			input: ROIInput{
				FileSizeBytes: 10 * gb, AddedAt: now.AddDate(0, 0, -10), Kind: KindMovie,
			},
			wantCategory: ValueLow, wantScore: 0, wantSuggest: false, wantDays: -1,
		},
		{
			name: "low value with old watch past suggestion window suggests deletion",
			input: ROIInput{
				FileSizeBytes: 10 * gb, TotalWatchSeconds: 600,
				LastWatched: now.AddDate(0, 0, -200), Kind: KindMovie,
			},
			wantPerGB: 0.0167, wantCategory: ValueLow, wantScore: 10.3, wantSuggest: true, wantDays: 200,
		},
		{
			name: "episode at 2.0 hrs/GB is moderate and scores 100",
			input: ROIInput{
				FileSizeBytes: 10 * gb, TotalWatchSeconds: 20 * 3600,
				LastWatched: now.AddDate(0, 0, -10), Kind: KindEpisode,
			},
			wantPerGB: 2.0, wantCategory: ValueModerate, wantScore: 100, wantDays: 10,
		},
		{
			name: "episode below 0.5 hrs/GB is low value",
			input: ROIInput{
				FileSizeBytes: 10 * gb, TotalWatchSeconds: 4 * 3600, // 0.4/GB
				LastWatched: now.AddDate(0, 0, -10), Kind: KindEpisode,
			},
			wantPerGB: 0.4, wantCategory: ValueLow, wantScore: 44, wantDays: 10,
		},
		{
			name: "episode above 2.0 hrs/GB is high value",
			input: ROIInput{
				FileSizeBytes: 10 * gb, TotalWatchSeconds: 30 * 3600, // 3.0/GB
				LastWatched: now.AddDate(0, 0, -10), Kind: KindEpisode,
			},
			wantPerGB: 3.0, wantCategory: ValueHigh, wantScore: 100, wantDays: 10,
		},
		{
			name: "age decay reduces score after 90 days",
			input: ROIInput{
				FileSizeBytes: 10 * gb, TotalWatchSeconds: 5 * 3600,
				LastWatched: now.AddDate(0, 0, -190), Kind: KindMovie,
			},
			wantPerGB: 0.5, wantCategory: ValueModerate, wantScore: 80, wantDays: 190,
		},
		{
			name: "zero-size item is moderate with no score",
			input: ROIInput{
				FileSizeBytes: 0, TotalWatchSeconds: 5 * 3600,
				LastWatched: now.AddDate(0, 0, -10), Kind: KindMovie,
			},
			wantCategory: ValueModerate, wantScore: 0, wantDays: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ROI(now, tt.input, cfg)
			if tt.wantPerGB > 0 {
				assert.InDelta(t, tt.wantPerGB, got.WatchHoursPerGB, 0.001)
			}
			assert.Equal(t, tt.wantCategory, got.ValueCategory)
			assert.InDelta(t, tt.wantScore, got.ValueScore, 0.001)
			assert.Equal(t, tt.wantSuggest, got.SuggestDeletion)
			assert.Equal(t, tt.wantDays, got.DaysSinceLastWatch)
		})
	}
}

func TestROIAgeDecayDisabled(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	cfg := testAnalyticsConfig()
	cfg.IncludeAgeDecay = false

	// Same item as the 80-point decay case, but with decay off it keeps full
	// recency credit.
	got := ROI(now, ROIInput{
		FileSizeBytes: 10 * gb, TotalWatchSeconds: 5 * 3600,
		LastWatched: now.AddDate(0, 0, -190), Kind: KindMovie,
	}, cfg)

	assert.Equal(t, ValueModerate, got.ValueCategory)
	assert.InDelta(t, 100, got.ValueScore, 0.001)
}

func TestROIConfiguredThresholdsDriveScore(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	cfg := testAnalyticsConfig()
	// Raise the movie high bound: 1.0 hrs/GB should now map to the 70-point cap.
	cfg.ValueThresholds.Movie = config.ValueThreshold{Low: 0.2, High: 1.0}

	got := ROI(now, ROIInput{
		FileSizeBytes: 10 * gb, TotalWatchSeconds: 10 * 3600, // 1.0/GB
		LastWatched: now.AddDate(0, 0, -10), Kind: KindMovie,
	}, cfg)

	assert.Equal(t, ValueModerate, got.ValueCategory) // 1.0 is not > 1.0
	assert.InDelta(t, 100, got.ValueScore, 0.001)     // base 70 + recency 30
}
