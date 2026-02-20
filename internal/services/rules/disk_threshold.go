package rules

import "time"

// DiskThresholdRule is a global gate that protects all items when disk space is adequate.
// When ctx.DiskStatus is nil (preview mode or feature disabled), returns nil — no protection.
//
// Full implementation: FEATURE_DISK_THRESHOLD_RULES
type DiskThresholdRule struct{}

// NewDiskThresholdRule creates a DiskThresholdRule.
func NewDiskThresholdRule() *DiskThresholdRule {
	return &DiskThresholdRule{}
}

func (r *DiskThresholdRule) Name() string     { return "disk_threshold" }
func (r *DiskThresholdRule) Scope() RuleScope { return ScopeAll }

// Protect returns ProtectedDiskOK when disk status is available and threshold is not breached.
// Returns nil when DiskStatus is nil (feature disabled or preview mode) — no protection applied.
func (r *DiskThresholdRule) Protect(ctx EvalContext) *ProtectionStatus {
	if ctx.DiskStatus == nil || !ctx.DiskStatus.Enabled {
		return nil
	}
	if !ctx.DiskStatus.ThresholdBreached {
		s := ProtectedDiskOK
		return &s
	}
	return nil
}

func (r *DiskThresholdRule) Schedule(ctx EvalContext) (time.Time, ScheduleSource) {
	return time.Time{}, 0 // Never schedules, only gates
}
