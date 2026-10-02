package rules

import (
	"fmt"
	"time"

	"github.com/ramonskie/oxicleanarr/internal/config"
	"github.com/ramonskie/oxicleanarr/internal/services/analytics"
)

// DefaultStaleDays mirrors the analytics config default when neither the rule
// nor the global analytics config sets a threshold.
const DefaultStaleDays = 90

// StaleRule deletes content that has gone unwatched for longer than a
// configurable threshold. Non-stale items are protected, so the rule expresses
// "delete only stale content". An optional retention period adds a grace
// window on top of the staleness threshold.
type StaleRule struct {
	rule config.AdvancedRule
}

// NewStaleRule creates a StaleRule from an AdvancedRule config entry.
func NewStaleRule(rule config.AdvancedRule) *StaleRule { return &StaleRule{rule: rule} }

func (r *StaleRule) Name() string     { return r.rule.Name }
func (r *StaleRule) Scope() RuleScope { return ScopeAll }

// Protect keeps non-stale content. When analytics is disabled the rule is inert
// (it neither protects nor schedules), so standard retention still applies.
func (r *StaleRule) Protect(ctx EvalContext) *ProtectionStatus {
	if !analyticsEnabled(ctx.Config) {
		return nil
	}
	if !r.result(ctx, time.Now()).IsStale {
		s := ProtectedByRule
		return &s
	}
	return nil
}

// Schedule returns the date the item crosses the staleness threshold plus any
// configured grace period. Already-stale items become due immediately when no
// grace is set; with a grace period they become due grace-from-now.
func (r *StaleRule) Schedule(ctx EvalContext) (time.Time, ScheduleSource) {
	if !analyticsEnabled(ctx.Config) {
		return time.Time{}, 0
	}

	now := time.Now()
	result := r.result(ctx, now)
	if !result.IsStale {
		return time.Time{}, 0
	}

	basis := ctx.Media.LastWatched
	if basis.IsZero() {
		basis = ctx.Media.AddedAt
	}
	if basis.IsZero() {
		return time.Time{}, 0
	}

	grace, err := parseDuration(r.rule.Retention)
	if err != nil {
		grace = 0
	}

	due := basis.AddDate(0, 0, result.ThresholdDays)
	if grace > 0 {
		if due.Before(now) {
			due = now
		}
		due = due.Add(grace)
	}
	return due, SourceStaleRule
}

// analyticsEnabled reports whether the analytics feature is active.
func analyticsEnabled(cfg *config.Config) bool {
	return cfg != nil && cfg.Analytics.Enabled
}

// EnrichVerdict implements VerdictEnricher.
func (r *StaleRule) EnrichVerdict(ctx EvalContext) (retentionValue, retentionBase, tagLabel string) {
	return fmt.Sprintf("%dd stale", r.threshold(ctx.Config)), "stale", ""
}

func (r *StaleRule) result(ctx EvalContext, now time.Time) analytics.StaleResult {
	return analytics.ComputeStale(now, ctx.Media.AddedAt, ctx.Media.LastWatched, r.threshold(ctx.Config))
}

func (r *StaleRule) threshold(cfg *config.Config) int {
	if r.rule.StaleDays > 0 {
		return r.rule.StaleDays
	}
	if cfg != nil && cfg.Analytics.StaleDays > 0 {
		return cfg.Analytics.StaleDays
	}
	return DefaultStaleDays
}
