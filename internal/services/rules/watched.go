package rules

import (
	"time"

	"github.com/ramonskie/oxicleanarr/internal/config"
)

// WatchedRule applies to all media based on watch status.
// Can protect (require_watched not met) or schedule based on retention.
type WatchedRule struct {
	rule config.AdvancedRule
}

// NewWatchedRule creates a WatchedRule from an AdvancedRule config entry.
func NewWatchedRule(rule config.AdvancedRule) *WatchedRule {
	return &WatchedRule{rule: rule}
}

func (r *WatchedRule) Name() string     { return r.rule.Name }
func (r *WatchedRule) Scope() RuleScope { return ScopeAll }

// Protect returns ProtectedByRule when require_watched is true and the item is unwatched.
func (r *WatchedRule) Protect(ctx EvalContext) *ProtectionStatus {
	if r.rule.RequireWatched && ctx.Media.WatchCount == 0 {
		s := ProtectedByRule
		return &s
	}
	return nil
}

// Schedule returns the deletion time based on the rule's retention period.
func (r *WatchedRule) Schedule(ctx EvalContext) (time.Time, ScheduleSource) {
	duration, err := parseDuration(r.rule.Retention)
	if err != nil || duration == 0 {
		return time.Time{}, 0
	}

	baseTime, neverDelete := getRetentionBaseTime(ctx.Media, r.rule.RetentionBase, r.rule.UnwatchedBehavior, ctx.Config)
	if neverDelete {
		return time.Time{}, 0
	}

	return baseTime.Add(duration), SourceWatchedRule
}

// EnrichVerdict implements VerdictEnricher.
func (r *WatchedRule) EnrichVerdict(ctx EvalContext) (retentionValue, retentionBase, tagLabel string) {
	retentionBase = r.rule.RetentionBase
	if retentionBase == "" {
		retentionBase = ctx.Config.Rules.RetentionBase
		if retentionBase == "" {
			retentionBase = RetentionBaseLastWatchedOrAdded
		}
	}
	return r.rule.Retention, retentionBase, ""
}
