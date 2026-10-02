package analytics

import (
	"math"
	"time"

	"github.com/ramonskie/oxicleanarr/internal/config"
)

// ValueCategory classifies an item's return on investment.
type ValueCategory string

const (
	// ValueLow marks content that consumes storage without proportional watch time.
	ValueLow ValueCategory = "low_value"
	// ValueModerate sits between the low and high thresholds.
	ValueModerate ValueCategory = "moderate_value"
	// ValueHigh marks content with strong watch time per GB.
	ValueHigh ValueCategory = "high_value"
)

// MediaKind selects the ROI threshold band for an item.
type MediaKind string

const (
	KindMovie   MediaKind = "movie"
	KindEpisode MediaKind = "episode"
	KindShow    MediaKind = "show"
)

// Score tuning constants mirror Tracearr's library ROI route: the base score
// tops out at 70 points, and a recency contribution adds up to 30.
const (
	maxBaseScore    = 70.0
	maxRecencyScore = 30.0
	ageDecayPerDay  = 0.2
	secondsPerHour  = 3600.0
	bytesPerGB      = 1 << 30
	// defaultDecayStartDays is the age-decay onset when analytics.stale_days
	// is unset.
	defaultDecayStartDays = 90
)

// ROIInput is the raw per-item data needed to compute ROI.
type ROIInput struct {
	FileSizeBytes     int64
	TotalWatchSeconds int64
	LastWatched       time.Time
	AddedAt           time.Time
	Kind              MediaKind
}

// ROIResult is the computed ROI for one item.
type ROIResult struct {
	FileSizeGB         float64
	TotalWatchHours    float64
	WatchHoursPerGB    float64
	DaysSinceLastWatch int // -1 when the item has never been watched
	ValueScore         float64
	ValueCategory      ValueCategory
	// OldEnough reports whether the item has gone long enough without a watch
	// to clear the suggestion window (never-watched items are measured from
	// their add date).
	OldEnough       bool
	SuggestDeletion bool
}

// ROI computes watch-hours-per-GB, a 0-100 value score, a value category and a
// deletion suggestion for a single item. It mirrors Tracearr's ROI route with
// thresholds supplied by configuration.
func ROI(now time.Time, in ROIInput, cfg config.AnalyticsConfig) ROIResult {
	gb := float64(in.FileSizeBytes) / bytesPerGB
	hours := float64(in.TotalWatchSeconds) / secondsPerHour

	perGB := 0.0
	if gb > 0 {
		perGB = hours / gb
	}

	daysSince := -1
	if !in.LastWatched.IsZero() {
		daysSince = daysBetween(now, in.LastWatched)
	}

	result := ROIResult{
		FileSizeGB:         gb,
		TotalWatchHours:    hours,
		WatchHoursPerGB:    round3(perGB),
		DaysSinceLastWatch: daysSince,
	}

	// A zero-size item carries no ROI signal; Tracearr classifies it moderate.
	if gb <= 0 {
		result.ValueCategory = ValueModerate
		return result
	}

	result.ValueCategory = classifyValue(in.Kind, perGB, cfg.ValueThresholds)
	result.ValueScore = valueScore(in.Kind, perGB, daysSince, cfg)
	result.OldEnough = oldEnoughForDeletion(daysSince, daysBetween(now, in.AddedAt), cfg.SuggestDeletionDays)
	result.SuggestDeletion = result.ValueCategory == ValueLow && result.OldEnough
	return result
}

// oldEnoughForDeletion reports whether a low-value item has gone long enough
// without a watch to be a deletion suggestion. Never-watched items are measured
// from their add date.
func oldEnoughForDeletion(daysSinceLastWatch, daysSinceAdded, threshold int) bool {
	if daysSinceLastWatch < 0 {
		return daysSinceAdded > threshold
	}
	return daysSinceLastWatch > threshold
}

// classifyValue maps watch-hours-per-GB to a category using the kind's band.
func classifyValue(kind MediaKind, perGB float64, thresholds config.ValueThresholdsConfig) ValueCategory {
	band := thresholdFor(kind, thresholds)
	if band.High > 0 && perGB > band.High {
		return ValueHigh
	}
	if band.Low > 0 && perGB < band.Low {
		return ValueLow
	}
	return ValueModerate
}

// thresholdFor selects the configured band for a media kind, defaulting to the
// show band for anything that is not a movie or episode.
func thresholdFor(kind MediaKind, thresholds config.ValueThresholdsConfig) config.ValueThreshold {
	switch kind {
	case KindMovie:
		return thresholds.Movie
	case KindEpisode:
		return thresholds.Episode
	default:
		return thresholds.Show
	}
}

// valueScore returns a 0-100 composite of watch value and recency.
func valueScore(kind MediaKind, perGB float64, daysSince int, cfg config.AnalyticsConfig) float64 {
	base := math.Min(maxBaseScore, perGB*baseFactor(kind, cfg.ValueThresholds))
	if base < 0 {
		base = 0
	}

	decayStart := cfg.StaleDays
	if decayStart <= 0 {
		decayStart = defaultDecayStartDays
	}

	recency := maxRecencyScore
	switch {
	case daysSince < 0:
		recency = 0 // never watched earns no recency credit
	case cfg.IncludeAgeDecay && daysSince > decayStart:
		recency = maxRecencyScore - float64(daysSince-decayStart)*ageDecayPerDay
		if recency < 0 {
			recency = 0
		}
	}

	score := math.Max(0, math.Min(100, base+recency))
	return round1(score)
}

// baseFactor scales watch-hours-per-GB so each kind's configured High bound maps
// to the 70-point base cap. When a band's High bound is unset (zero), the
// Tracearr default factor for that kind is used.
func baseFactor(kind MediaKind, thresholds config.ValueThresholdsConfig) float64 {
	if high := thresholdFor(kind, thresholds).High; high > 0 {
		return maxBaseScore / high
	}
	switch kind {
	case KindMovie:
		return 140
	case KindEpisode:
		return 35
	default:
		return 70
	}
}

func round1(v float64) float64 {
	return math.Round(v*10) / 10
}

func round3(v float64) float64 {
	return math.Round(v*1000) / 1000
}
