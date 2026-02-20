package rules

import "time"

// RuleScope controls what media types a rule applies to.
type RuleScope int

const (
	ScopeAll     RuleScope = iota // movies + TV shows (show-level)
	ScopeMovies                   // movies only
	ScopeTVShows                  // TV shows only (show-level)
	ScopeEpisode                  // TV episode files (episode-level, separate chain)
)

// VerdictEnricher is an optional interface rules may implement to provide
// additional metadata for the RuleVerdict (retention value, base, tag label).
// The engine calls EnrichVerdict after a scheduling match if the rule implements this.
type VerdictEnricher interface {
	EnrichVerdict(ctx EvalContext) (retentionValue, retentionBase, tagLabel string)
}

// Rule is the interface every rule type must implement.
//
// Rules participate in one or both evaluation phases:
//   - Protect(): Phase 1 — can this item be deleted?
//   - Schedule(): Phase 2 — when should it be deleted?
//
// A rule that only protects (e.g. ExclusionRule, DiskThresholdRule)
// returns (time.Time{}, 0) from Schedule().
//
// A rule that only schedules returns nil from Protect().
//
// A rule that does both (e.g. TagRule with retention:never OR a retention period)
// implements both methods fully.
type Rule interface {
	// Name returns the rule's display name for logging and UI.
	Name() string

	// Scope returns what media types this rule applies to.
	// The engine skips rules whose scope doesn't match the media type.
	Scope() RuleScope

	// Protect returns a non-nil ProtectionStatus if this rule
	// prevents deletion of the item in ctx.
	// Returning nil means "this rule does not protect this item."
	// Called in Phase 1. First non-nil return wins; remaining rules skipped.
	Protect(ctx EvalContext) *ProtectionStatus

	// Schedule returns the deletion time if this rule applies to the item.
	// Returning zero time means "this rule does not schedule this item."
	// Called in Phase 2. First non-zero return wins; remaining rules skipped.
	Schedule(ctx EvalContext) (time.Time, ScheduleSource)
}
