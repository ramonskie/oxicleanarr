package rules

import "time"

// ProtectionStatus describes why an item is protected from deletion.
// Used in Phase 1 output.
type ProtectionStatus int

const (
	ProtectedExcluded  ProtectionStatus = iota // Manual exclusion list
	ProtectedDiskOK                            // Disk threshold not breached
	ProtectedUnwatched                         // unwatched_behavior: never
	ProtectedRequested                         // Requested, no matching user rule
	ProtectedByRule                            // Rule explicitly protects (e.g. require_watched not met, retention: never)
	ProtectedNoRule                            // No rule matched, no deletion date
)

// ScheduleSource describes which rule determined the deletion date.
// Used in Phase 2 output and UI rendering.
type ScheduleSource int

const (
	SourceTagRule ScheduleSource = iota
	SourceUserRule
	SourceWatchedRule
	SourceStandardRetention
	SourceEpisodeRule
)

// RuleVerdict is the complete, structured output of rule evaluation.
// Replaces the (shouldDelete bool, deleteAfter time.Time, reason string) tuple.
type RuleVerdict struct {
	// ── Phase 1 output ───────────────────────────────────────────────
	IsProtected      bool
	ProtectionReason ProtectionStatus // meaningful only when IsProtected=true
	ProtectingRule   string           // rule name, if ProtectedByRule

	// ── Phase 2 output ───────────────────────────────────────────────
	// Only meaningful when IsProtected=false
	DeleteAfter    time.Time
	ScheduleSource ScheduleSource
	SchedulingRule string // rule name that determined the date
	RetentionBase  string // "last_watched", "added" — for UI display
	RetentionValue string // "30d", "90d" — for UI display
	TagLabel       string // tag label for SourceTagRule (e.g. "test-deletion")

	// ── Episode chain output ─────────────────────────────────────────
	// Only set when an EpisodeRule matched.
	// Empty slice = delete the whole item (standard behavior).
	// Non-empty = delete only these specific episode files.
	EpisodeFileIDs []int
}

// ShouldDelete returns true if the item is overdue for deletion right now.
func (v RuleVerdict) ShouldDelete() bool {
	if v.IsProtected {
		return false
	}
	if v.DeleteAfter.IsZero() {
		return false
	}
	return time.Now().After(v.DeleteAfter)
}

// HasEpisodeDeletions returns true if this verdict targets specific episode files.
func (v RuleVerdict) HasEpisodeDeletions() bool {
	return len(v.EpisodeFileIDs) > 0
}
