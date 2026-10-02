package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ramonskie/oxicleanarr/internal/config"
	"github.com/ramonskie/oxicleanarr/internal/models"
	"github.com/ramonskie/oxicleanarr/internal/services/analytics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnalyticsHandlersRejectNilEngine(t *testing.T) {
	config.SetTestConfig(&config.Config{})
	t.Cleanup(func() { config.SetTestConfig(nil) })

	handler := NewAnalyticsHandler(nil)
	for name, fn := range map[string]http.HandlerFunc{
		"stale":       handler.GetStale,
		"roi":         handler.GetROI,
		"dead-weight": handler.GetDeadWeight,
	} {
		rec := httptest.NewRecorder()
		fn(rec, httptest.NewRequest(http.MethodGet, "/", nil))
		assert.Equal(t, http.StatusServiceUnavailable, rec.Code, name)
	}
}

func TestAccumulateStale(t *testing.T) {
	summary := staleSummary{}

	accumulateStale(&summary, analytics.CategoryNeverWatched, 100)
	accumulateStale(&summary, analytics.CategoryStale, 50)
	accumulateStale(&summary, analytics.CategoryStale, 25)

	assert.Equal(t, 1, summary.NeverWatched.Count)
	assert.Equal(t, int64(100), summary.NeverWatched.SizeBytes)
	assert.Equal(t, 2, summary.Stale.Count)
	assert.Equal(t, int64(75), summary.Stale.SizeBytes)
	assert.Equal(t, 3, summary.Total.Count)
	assert.Equal(t, int64(175), summary.Total.SizeBytes)
}

func TestAccumulateROI(t *testing.T) {
	summary := roiSummary{}

	accumulateROI(&summary, analytics.ROIResult{
		FileSizeGB: 10, TotalWatchHours: 5,
		ValueCategory: analytics.ValueLow, SuggestDeletion: true,
	})
	accumulateROI(&summary, analytics.ROIResult{
		FileSizeGB: 5, TotalWatchHours: 5,
		ValueCategory: analytics.ValueHigh,
	})

	assert.Equal(t, 2, summary.TotalItems)
	assert.InDelta(t, 15.0, summary.TotalStorageGB, 0.001)
	assert.InDelta(t, 10.0, summary.TotalWatchHours, 0.001)
	assert.Equal(t, 1, summary.LowValueItems)
	assert.InDelta(t, 10.0, summary.LowValueStorageGB, 0.001)
	assert.InDelta(t, 10.0, summary.PotentialSavingsGB, 0.001)
}

func TestROIInputFor(t *testing.T) {
	movie := models.Media{Type: models.MediaTypeMovie, FileSize: 10, TotalWatchSeconds: 5}
	assert.Equal(t, analytics.KindMovie, roiInputFor(movie).Kind)
	assert.Equal(t, int64(10), roiInputFor(movie).FileSizeBytes)

	show := models.Media{Type: models.MediaTypeTVShow}
	assert.Equal(t, analytics.KindShow, roiInputFor(show).Kind)
}

func TestOptionalTime(t *testing.T) {
	assert.Nil(t, optionalTime(time.Time{}))

	now := time.Now()
	got := optionalTime(now)
	require.NotNil(t, got)
	assert.Equal(t, now, *got)
}

func TestNewDeadWeightItem(t *testing.T) {
	media := models.Media{
		ID: "m1", Title: "Never Watched", Type: models.MediaTypeMovie, Year: 2001,
		FileSize: 123, IsExcluded: true, IsManualLeavingSoon: true,
	}

	got := newDeadWeightItem(media)

	assert.Equal(t, "m1", got.ID)
	assert.Equal(t, "Never Watched", got.Title)
	assert.Equal(t, int64(123), got.FileSize)
	assert.True(t, got.Excluded)
	assert.True(t, got.ManualLeavingSoon)
}

func TestRoundTo(t *testing.T) {
	assert.InDelta(t, 1.23, roundTo(1.2345, 2), 0.0001)
	assert.InDelta(t, 2.0, roundTo(1.9999, 2), 0.0001)
}
