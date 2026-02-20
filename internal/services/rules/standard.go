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

// Protect handles two cases:
//  1. Requested items with no advanced rules configured — blanket protection
//     (preserves legacy behavior: requested media is safe when no user rules exist)
//  2. unwatched_behavior: never — protects unwatched items from deletion
func (r *StandardRule) Protect(ctx EvalContext) *ProtectionStatus {
	cfg := ctx.Config

	// Blanket protection for requested items when no advanced rules are configured.
	// When user-based rules exist, they take priority and this protection is skipped.
	if ctx.Media.IsRequested && len(cfg.AdvancedRules) == 0 {
		s := ProtectedRequested
		return &s
	}

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
func (r *StandardRule) Schedule(ctx EvalContext) (time.Time, ScheduleSource) {
	cfg := ctx.Config

	var retentionStr string
	if ctx.Media.Type == models.MediaTypeMovie {
		retentionStr = cfg.Rules.MovieRetention
	} else {
		retentionStr = cfg.Rules.TVRetention
	}

	duration, err := parseDuration(retentionStr)
	if err != nil || duration == 0 {
		return time.Time{}, 0
	}

	// getRetentionBaseTime returns zero only when unwatched_behavior is "never"
	// and the item is unwatched — meaning it should never be deleted.
	// In all other cases (including zero AddedAt), we proceed with the base time.
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
// Returns (time.Time{}, true) only when unwatched_behavior is "never" and the item
// is unwatched — callers must treat this as "do not schedule deletion".
// In all other cases (including zero AddedAt), returns the base time and false.
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
		return clampToNow(media.AddedAt), false

	case RetentionBaseLastWatched:
		if !media.LastWatched.IsZero() {
			return clampToNow(media.LastWatched), false
		}
		if unwatchedBehavior == UnwatchedBehaviorNever {
			return time.Time{}, true // never delete
		}
		return clampToNow(media.AddedAt), false

	default: // RetentionBaseLastWatchedOrAdded
		if !media.LastWatched.IsZero() {
			return clampToNow(media.LastWatched), false
		}
		return clampToNow(media.AddedAt), false
	}
}
