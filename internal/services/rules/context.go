package rules

import (
	"github.com/ramonskie/oxicleanarr/internal/config"
	"github.com/ramonskie/oxicleanarr/internal/models"
)

// EvalContext carries all inputs a rule needs to make a decision.
// Rules are pure functions of their EvalContext — no hidden dependencies.
type EvalContext struct {
	Media  *models.Media
	Config *config.Config

	// DiskStatus is nil when:
	//   a) disk threshold feature is disabled, OR
	//   b) evaluation is running in preview mode (EvaluateForPreview)
	// Rules must treat nil DiskStatus as "no disk constraint."
	DiskStatus *DiskStatus
}

// DiskStatus holds the current disk space state.
// Populated by DiskMonitor before each sync cycle.
type DiskStatus struct {
	Enabled           bool
	FreeSpaceGB       int
	TotalSpaceGB      int
	ThresholdGB       int
	ThresholdBreached bool
	CheckSource       string // "radarr", "sonarr", "lowest"
}

// DiskMonitor provides disk status to the rules engine.
// Implemented by the disk threshold feature; nil when feature is disabled.
type DiskMonitor interface {
	GetStatus() *DiskStatus
}
