package rules

import (
	"time"

	"github.com/ramonskie/oxicleanarr/internal/config"
)

// TagRule matches media by Radarr/Sonarr tag.
// Can protect (retention: never, or require_watched not met) or schedule (retention: 30d).
type TagRule struct {
	rule config.AdvancedRule
}

// NewTagRule creates a TagRule from an AdvancedRule config entry.
func NewTagRule(rule config.AdvancedRule) *TagRule {
	return &TagRule{rule: rule}
}

func (r *TagRule) Name() string     { return r.rule.Name }
func (r *TagRule) Scope() RuleScope { return ScopeAll }

// Protect returns ProtectedByRule when:
//   - The tag matches AND retention is "never" (explicitly protect this item forever)
//   - The tag matches AND require_watched is true AND item is unwatched
//
// Note: "0d" is NOT treated as protection — it means zero retention (delete immediately
// based on base time). Only the literal string "never" triggers protection.
func (r *TagRule) Protect(ctx EvalContext) *ProtectionStatus {
	if !r.matchesMedia(ctx) {
		return nil
	}

	// retention: never — explicitly protect this item forever
	if r.rule.Retention == "never" {
		s := ProtectedByRule
		return &s
	}

	// require_watched: true and item not watched — protect until watched
	if r.rule.RequireWatched && ctx.Media.WatchCount == 0 {
		s := ProtectedByRule
		return &s
	}

	return nil
}

// Schedule returns the deletion time when the tag matches and retention is a valid duration.
// "0d" schedules immediate deletion (baseTime + 0 = base time, which is in the past).
// "never" is handled by Protect() and never reaches this method.
func (r *TagRule) Schedule(ctx EvalContext) (time.Time, ScheduleSource) {
	if !r.matchesMedia(ctx) {
		return time.Time{}, 0
	}

	duration, err := parseDuration(r.rule.Retention)
	if err != nil {
		return time.Time{}, 0 // invalid retention string
	}

	// "never" is handled by Protect() — no scheduling needed
	if r.rule.Retention == "never" {
		return time.Time{}, 0
	}

	baseTime, neverDelete := getRetentionBaseTime(ctx.Media, r.rule.RetentionBase, r.rule.UnwatchedBehavior, ctx.Config)
	if neverDelete {
		return time.Time{}, 0
	}

	// duration == 0 means "0d" — schedule at baseTime (immediate deletion)
	return baseTime.Add(duration), SourceTagRule
}

func (r *TagRule) matchesMedia(ctx EvalContext) bool {
	for _, tag := range ctx.Media.Tags {
		if equalsCaseInsensitive(tag, r.rule.Tag) {
			return true
		}
	}
	return false
}

// EnrichVerdict implements VerdictEnricher.
// Returns the retention value, retention base, and the tag label for UI display.
func (r *TagRule) EnrichVerdict(ctx EvalContext) (retentionValue, retentionBase, tagLabel string) {
	retentionBase = r.rule.RetentionBase
	if retentionBase == "" {
		retentionBase = ctx.Config.Rules.RetentionBase
		if retentionBase == "" {
			retentionBase = RetentionBaseLastWatchedOrAdded
		}
	}
	return r.rule.Retention, retentionBase, r.rule.Tag
}
