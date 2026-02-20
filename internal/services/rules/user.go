package rules

import (
	"time"

	"github.com/ramonskie/oxicleanarr/internal/config"
)

// UserRule matches media by Jellyseerr requester.
// Can protect (require_watched not met) or schedule based on per-user retention.
type UserRule struct {
	rule config.AdvancedRule
}

// NewUserRule creates a UserRule from an AdvancedRule config entry.
func NewUserRule(rule config.AdvancedRule) *UserRule {
	return &UserRule{rule: rule}
}

func (r *UserRule) Name() string     { return r.rule.Name }
func (r *UserRule) Scope() RuleScope { return ScopeAll }

// Protect returns ProtectedByRule when the user matches and require_watched is true
// but the item has not been watched.
func (r *UserRule) Protect(ctx EvalContext) *ProtectionStatus {
	userRule := r.matchedUserRule(ctx)
	if userRule == nil {
		return nil
	}

	requireWatched := userRule.RequireWatched != nil && *userRule.RequireWatched
	if requireWatched && ctx.Media.WatchCount == 0 {
		s := ProtectedByRule
		return &s
	}

	return nil
}

// Schedule returns the deletion time when the user matches and has a valid retention period.
func (r *UserRule) Schedule(ctx EvalContext) (time.Time, ScheduleSource) {
	userRule := r.matchedUserRule(ctx)
	if userRule == nil {
		return time.Time{}, 0
	}

	duration, err := parseDuration(userRule.Retention)
	if err != nil || duration == 0 {
		return time.Time{}, 0
	}

	// User rules use last_watched_or_added base time by default.
	// Per-rule retention_base override is supported via r.rule.RetentionBase (future).
	baseTime, neverDelete := getRetentionBaseTime(ctx.Media, r.rule.RetentionBase, r.rule.UnwatchedBehavior, ctx.Config)
	if neverDelete {
		return time.Time{}, 0
	}

	return baseTime.Add(duration), SourceUserRule
}

// matchedUserRule returns the first UserRule entry that matches the media's requester,
// or nil if the media is not requested or no entry matches.
func (r *UserRule) matchedUserRule(ctx EvalContext) *config.UserRule {
	if !ctx.Media.IsRequested {
		return nil
	}
	for i := range r.rule.Users {
		u := &r.rule.Users[i]
		if u.UserID != nil && ctx.Media.RequestedByUserID != nil && *u.UserID == *ctx.Media.RequestedByUserID {
			return u
		}
		if u.Username != "" && ctx.Media.RequestedByUsername != nil && equalsCaseInsensitive(u.Username, *ctx.Media.RequestedByUsername) {
			return u
		}
		if u.Email != "" && ctx.Media.RequestedByEmail != nil && equalsCaseInsensitive(u.Email, *ctx.Media.RequestedByEmail) {
			return u
		}
	}
	return nil
}

// EnrichVerdict implements VerdictEnricher.
func (r *UserRule) EnrichVerdict(ctx EvalContext) (retentionValue, retentionBase, tagLabel string) {
	retentionBase = r.rule.RetentionBase
	if retentionBase == "" {
		retentionBase = ctx.Config.Rules.RetentionBase
		if retentionBase == "" {
			retentionBase = RetentionBaseLastWatchedOrAdded
		}
	}
	userRule := r.matchedUserRule(ctx)
	if userRule != nil {
		return userRule.Retention, retentionBase, ""
	}
	return r.rule.Retention, retentionBase, ""
}
