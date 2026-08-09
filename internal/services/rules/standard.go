package rules

import (
	"time"

	"github.com/ramonskie/oxicleanarr/internal/config"
	"github.com/ramonskie/oxicleanarr/internal/models"
)

// RetentionBase constants — canonical values used across all rules.
const (
	RetentionBaseLastWatchedOrAdded = "last_watched_or_added" // default
	RetentionBaseLastWatched        = "last_watched"
	RetentionBaseAdded              = "added"
)

// UnwatchedBehavior constants.
const (
	UnwatchedBehaviorAdded = "added" // default: use AddedAt for unwatched items
	UnwatchedBehaviorNever = "never" // never delete unwatched items
)

// StandardRule is the default retention rule.
// It is always last in both the protection and scheduling chains.
type StandardRule struct{}

// NewStandardRule creates a StandardRule.
func NewStandardRule() *StandardRule { return &StandardRule{} }

func (r *StandardRule) Name() string     { return "standard_retention" }
func (r *StandardRule) Scope() RuleScope { return ScopeAll }

// Protect handles unwatched_behavior: never — protects unwatched items from deletion.
func (r *StandardRule) Protect(ctx EvalContext) *ProtectionStatus {
	cfg := ctx.Config

	// unwatched_behavior: never — protect unwatched items from deletion
	if cfg.Rules.RetentionBase == RetentionBaseLastWatched &&
		cfg.Rules.UnwatchedBehavior == UnwatchedBehaviorNever &&
		ctx.Media.LastWatched.IsZero() {
		s := ProtectedUnwatched
		return &s
	}

	return nil
}

// Schedule applies movie_retention / tv_retention using the configured base time.
// When retention_base=last_watched and unwatched_behavior=added and unwatched_retention
// is configured, unwatched items use unwatched_retention instead of movie/tv_retention.
func (r *StandardRule) Schedule(ctx EvalContext) (time.Time, ScheduleSource) {
	cfg := ctx.Config

	var retentionStr string
	if ctx.Media.Type == models.MediaTypeMovie {
		retentionStr = cfg.Rules.MovieRetention
	} else {
		retentionStr = cfg.Rules.TVRetention
	}

	// Use unwatched_retention for unwatched items when configured.
	// Only applies when retention_base=last_watched AND unwatched_behavior=added.
	retentionBase := cfg.Rules.RetentionBase
	if retentionBase == "" {
		retentionBase = RetentionBaseLastWatchedOrAdded
	}
	unwatchedBehavior := cfg.Rules.UnwatchedBehavior
	if unwatchedBehavior == "" {
		unwatchedBehavior = UnwatchedBehaviorAdded
	}
	if retentionBase == RetentionBaseLastWatched &&
		unwatchedBehavior == UnwatchedBehaviorAdded &&
		cfg.Rules.UnwatchedRetention != "" &&
		ctx.Media.LastWatched.IsZero() {
		retentionStr = cfg.Rules.UnwatchedRetention
	}

	duration, err := parseDuration(retentionStr)
	if err != nil || duration == 0 {
		return time.Time{}, 0
	}

	// getRetentionBaseTime returns zero only when no valid base time exists
	// (unwatched_behavior "never", or zero AddedAt/LastWatched) — meaning the item
	// should never be scheduled for deletion.
	baseTime, neverDelete := getRetentionBaseTime(ctx.Media, cfg.Rules.RetentionBase, cfg.Rules.UnwatchedBehavior, cfg)
	if neverDelete {
		return time.Time{}, 0
	}

	return baseTime.Add(duration), SourceStandardRetention
}

// EnrichVerdict implements VerdictEnricher.
func (r *StandardRule) EnrichVerdict(ctx EvalContext) (retentionValue, retentionBase, tagLabel string) {
	cfg := ctx.Config
	retentionBase = cfg.Rules.RetentionBase
	if retentionBase == "" {
		retentionBase = RetentionBaseLastWatchedOrAdded
	}
	if ctx.Media.Type == models.MediaTypeMovie {
		return cfg.Rules.MovieRetention, retentionBase, ""
	}
	return cfg.Rules.TVRetention, retentionBase, ""
}

// getRetentionBaseTime returns the base time for retention calculation and whether
// the item should never be deleted.
//
// retentionBase and unwatchedBehavior default to global config values when empty.
// Returns (time.Time{}, true) when no valid base time exists — zero AddedAt, or
// unwatched_behavior "never" with an unwatched item. Callers must treat this as
// "do not schedule deletion" (fail-safe: never delete based on a fabricated past date).
func getRetentionBaseTime(media *models.Media, retentionBase, unwatchedBehavior string, cfg *config.Config) (time.Time, bool) {
	if retentionBase == "" {
		retentionBase = cfg.Rules.RetentionBase
		if retentionBase == "" {
			retentionBase = RetentionBaseLastWatchedOrAdded
		}
	}
	if unwatchedBehavior == "" {
		unwatchedBehavior = cfg.Rules.UnwatchedBehavior
		if unwatchedBehavior == "" {
			unwatchedBehavior = UnwatchedBehaviorAdded
		}
	}

	switch retentionBase {
	case RetentionBaseAdded:
		return retentionBaseTime(media.AddedAt)

	case RetentionBaseLastWatched:
		if !media.LastWatched.IsZero() {
			return retentionBaseTime(media.LastWatched)
		}
		if unwatchedBehavior == UnwatchedBehaviorNever {
			return time.Time{}, true // never delete
		}
		return retentionBaseTime(media.AddedAt)

	default: // RetentionBaseLastWatchedOrAdded
		if !media.LastWatched.IsZero() {
			return retentionBaseTime(media.LastWatched)
		}
		return retentionBaseTime(media.AddedAt)
	}
}

// retentionBaseTime validates a base time. A zero time (missing AddedAt/LastWatched)
// is not a valid retention base — returning (time.Time{}, true) defers deletion
// rather than scheduling it from a fabricated year-1 timestamp.
func retentionBaseTime(t time.Time) (time.Time, bool) {
	if t.IsZero() {
		return time.Time{}, true
	}
	return clampToNow(t), false
}
