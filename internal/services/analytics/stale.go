package analytics

import "time"

// StaleCategory classifies why an item is considered stale.
type StaleCategory string

const (
	// CategoryNeverWatched marks content that has never been played.
	CategoryNeverWatched StaleCategory = "never_watched"
	// CategoryStale marks content watched at some point but not within the
	// staleness threshold.
	CategoryStale StaleCategory = "stale"
)

// StaleResult is the staleness classification for a single media item.
type StaleResult struct {
	// IsStale reports whether the item crossed the staleness threshold.
	IsStale bool
	// Category is never_watched for unplayed content, stale otherwise.
	Category StaleCategory
	// DaysStale is days since last watch, or days since added when never watched.
	DaysStale int
	// ThresholdDays echoes the threshold used.
	ThresholdDays int
}

// ComputeStale classifies an item as never_watched or stale.
//
// The clock starts at the last watch, or at the add date when the item has
// never been watched. An item is stale only once that clock exceeds
// thresholdDays, so a freshly added, never-watched item is not yet stale.
func ComputeStale(now, addedAt, lastWatched time.Time, thresholdDays int) StaleResult {
	basis := lastWatched
	category := CategoryStale
	if lastWatched.IsZero() {
		basis = addedAt
		category = CategoryNeverWatched
	}

	days := daysBetween(now, basis)
	return StaleResult{
		IsStale:       days > thresholdDays,
		Category:      category,
		DaysStale:     days,
		ThresholdDays: thresholdDays,
	}
}

// daysBetween returns whole days elapsed from t to now, clamped at zero.
func daysBetween(now, t time.Time) int {
	if t.IsZero() {
		return 0
	}
	days := int(now.Sub(t).Hours() / 24)
	if days < 0 {
		return 0
	}
	return days
}
