package rules

import (
	"time"

	"github.com/ramonskie/oxicleanarr/internal/storage"
)

// ExclusionRule protects items that are on the manual exclusion list.
// Always runs first — manual exclusions always win over everything else.
type ExclusionRule struct {
	exclusions *storage.ExclusionsFile
}

// NewExclusionRule creates an ExclusionRule backed by the given exclusions file.
func NewExclusionRule(exclusions *storage.ExclusionsFile) *ExclusionRule {
	return &ExclusionRule{exclusions: exclusions}
}

func (r *ExclusionRule) Name() string     { return "exclusion" }
func (r *ExclusionRule) Scope() RuleScope { return ScopeAll }

func (r *ExclusionRule) Protect(ctx EvalContext) *ProtectionStatus {
	if r.exclusions.IsExcluded(ctx.Media.ID) {
		s := ProtectedExcluded
		return &s
	}
	return nil
}

func (r *ExclusionRule) Schedule(ctx EvalContext) (time.Time, ScheduleSource) {
	return time.Time{}, 0 // Never schedules, only protects
}
