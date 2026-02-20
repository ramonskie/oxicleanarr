package rules

import (
	"github.com/ramonskie/oxicleanarr/internal/config"
	"github.com/ramonskie/oxicleanarr/internal/models"
	"github.com/ramonskie/oxicleanarr/internal/storage"
	"github.com/rs/zerolog/log"
)

// RulesEngine orchestrates two-phase rule evaluation.
// It is safe for concurrent use — no mutable state after construction.
type RulesEngine struct {
	// protectionRules are evaluated in Phase 1 (in order).
	// First rule returning non-nil ProtectionStatus wins.
	protectionRules []Rule

	// schedulingRules are evaluated in Phase 2 (in order).
	// First rule returning non-zero time wins.
	schedulingRules []Rule

	// episodeRules run in a separate chain for TV shows.
	// They bypass the disk threshold gate.
	episodeRules []Rule

	diskMonitor DiskMonitor // nil if disk threshold disabled
}

// NewRulesEngine constructs the engine with the canonical rule order.
// Called once at startup; rules are immutable after construction.
func NewRulesEngine(exclusions *storage.ExclusionsFile, diskMonitor DiskMonitor) *RulesEngine {
	cfg := config.Get()

	e := &RulesEngine{
		diskMonitor: diskMonitor,
	}

	// ── Protection rules (Phase 1 order) ─────────────────────────────
	e.protectionRules = []Rule{
		NewExclusionRule(exclusions), // 1. Always first
		NewDiskThresholdRule(),       // 2. Global gate
	}

	// ── Advanced rules from config (tag → user → watched → episode) ──
	for _, rule := range cfg.AdvancedRules {
		if !rule.Enabled {
			continue
		}
		switch rule.Type {
		case "tag":
			tr := NewTagRule(rule)
			e.protectionRules = append(e.protectionRules, tr) // tags can protect
			e.schedulingRules = append(e.schedulingRules, tr) // tags can schedule
		case "user":
			ur := NewUserRule(rule)
			e.protectionRules = append(e.protectionRules, ur)
			e.schedulingRules = append(e.schedulingRules, ur)
		case "watched":
			wr := NewWatchedRule(rule)
			e.protectionRules = append(e.protectionRules, wr)
			e.schedulingRules = append(e.schedulingRules, wr)
		case "episode":
			// Episode rules go into the separate chain (future: NewEpisodeRule(rule))
			log.Debug().Str("rule", rule.Name).Msg("Episode rule configured (implementation pending)")
		}
	}

	// Standard retention is always last in both chains
	sr := NewStandardRule()
	e.protectionRules = append(e.protectionRules, sr) // handles unwatched_behavior: never
	e.schedulingRules = append(e.schedulingRules, sr)

	return e
}

// Evaluate runs full two-phase evaluation for a media item.
// The disk threshold gate is active when a DiskMonitor is configured.
func (e *RulesEngine) Evaluate(media *models.Media) RuleVerdict {
	ctx := EvalContext{
		Media:      media,
		Config:     config.Get(),
		DiskStatus: e.getDiskStatus(),
	}
	return e.evaluateWithContext(ctx)
}

// EvaluateForPreview runs evaluation with the disk threshold gate disabled.
// Used by GetLeavingSoon() to show what WOULD be deleted if threshold was breached.
// Thread-safe: passes nil DiskStatus in context, no shared state mutation.
func (e *RulesEngine) EvaluateForPreview(media *models.Media) RuleVerdict {
	ctx := EvalContext{
		Media:      media,
		Config:     config.Get(),
		DiskStatus: nil, // nil = DiskThresholdRule returns nil (no protection)
	}
	return e.evaluateWithContext(ctx)
}

// evaluateWithContext is the internal implementation shared by Evaluate and EvaluateForPreview.
func (e *RulesEngine) evaluateWithContext(ctx EvalContext) RuleVerdict {
	// ── PHASE 1: PROTECTION ──────────────────────────────────────────
	for _, rule := range e.protectionRules {
		if !scopeMatches(rule.Scope(), ctx.Media.Type) {
			continue
		}
		if status := rule.Protect(ctx); status != nil {
			log.Debug().
				Str("media_id", ctx.Media.ID).
				Str("rule", rule.Name()).
				Int("protection_status", int(*status)).
				Msg("Media protected from deletion")
			return RuleVerdict{
				IsProtected:      true,
				ProtectionReason: *status,
				ProtectingRule:   rule.Name(),
			}
		}
	}

	// ── PHASE 2: SCHEDULING ──────────────────────────────────────────
	for _, rule := range e.schedulingRules {
		if !scopeMatches(rule.Scope(), ctx.Media.Type) {
			continue
		}
		deleteAfter, source := rule.Schedule(ctx)
		if !deleteAfter.IsZero() {
			log.Debug().
				Str("media_id", ctx.Media.ID).
				Str("rule", rule.Name()).
				Time("delete_after", deleteAfter).
				Msg("Media scheduled for deletion")
			verdict := RuleVerdict{
				IsProtected:    false,
				DeleteAfter:    deleteAfter,
				ScheduleSource: source,
				SchedulingRule: rule.Name(),
			}
			if enricher, ok := rule.(VerdictEnricher); ok {
				verdict.RetentionValue, verdict.RetentionBase, verdict.TagLabel = enricher.EnrichVerdict(ctx)
			}
			return verdict
		}
	}

	// ── EPISODE CHAIN (TV shows only) ────────────────────────────────
	if ctx.Media.Type == models.MediaTypeTVShow {
		for _, rule := range e.episodeRules {
			deleteAfter, source := rule.Schedule(ctx)
			if !deleteAfter.IsZero() || len(ctx.Media.EpisodeFileIDs) > 0 {
				return RuleVerdict{
					IsProtected:    false,
					DeleteAfter:    deleteAfter,
					ScheduleSource: source,
					SchedulingRule: rule.Name(),
					EpisodeFileIDs: ctx.Media.EpisodeFileIDs,
				}
			}
		}
	}

	// No rule matched — no deletion scheduled
	return RuleVerdict{
		IsProtected:      true,
		ProtectionReason: ProtectedNoRule,
	}
}

// getDiskStatus returns the current disk status for use in EvalContext.
// Returns nil if disk monitoring is disabled.
func (e *RulesEngine) getDiskStatus() *DiskStatus {
	if e.diskMonitor == nil {
		return nil
	}
	return e.diskMonitor.GetStatus()
}
