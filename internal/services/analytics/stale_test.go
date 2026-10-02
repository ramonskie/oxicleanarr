package analytics

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestComputeStale(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name          string
		addedAt       time.Time
		lastWatched   time.Time
		thresholdDays int
		wantStale     bool
		wantCategory  StaleCategory
		wantDays      int
	}{
		{
			name:          "never watched past threshold is stale",
			addedAt:       now.AddDate(0, 0, -100),
			thresholdDays: 90,
			wantStale:     true,
			wantCategory:  CategoryNeverWatched,
			wantDays:      100,
		},
		{
			name:          "fresh never-watched item is not stale",
			addedAt:       now.AddDate(0, 0, -30),
			thresholdDays: 90,
			wantStale:     false,
			wantCategory:  CategoryNeverWatched,
			wantDays:      30,
		},
		{
			name:          "watched recently is not stale",
			addedAt:       now.AddDate(0, 0, -300),
			lastWatched:   now.AddDate(0, 0, -30),
			thresholdDays: 90,
			wantStale:     false,
			wantCategory:  CategoryStale,
			wantDays:      30,
		},
		{
			name:          "watched beyond threshold is stale",
			addedAt:       now.AddDate(0, 0, -300),
			lastWatched:   now.AddDate(0, 0, -120),
			thresholdDays: 90,
			wantStale:     true,
			wantCategory:  CategoryStale,
			wantDays:      120,
		},
		{
			name:          "exactly at threshold is not stale",
			addedAt:       now.AddDate(0, 0, -300),
			lastWatched:   now.AddDate(0, 0, -90),
			thresholdDays: 90,
			wantStale:     false,
			wantCategory:  CategoryStale,
			wantDays:      90,
		},
		{
			name:          "missing dates are not stale",
			thresholdDays: 90,
			wantStale:     false,
			wantCategory:  CategoryNeverWatched,
			wantDays:      0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ComputeStale(now, tt.addedAt, tt.lastWatched, tt.thresholdDays)
			assert.Equal(t, tt.wantStale, got.IsStale)
			assert.Equal(t, tt.wantCategory, got.Category)
			assert.Equal(t, tt.wantDays, got.DaysStale)
			assert.Equal(t, tt.thresholdDays, got.ThresholdDays)
		})
	}
}
