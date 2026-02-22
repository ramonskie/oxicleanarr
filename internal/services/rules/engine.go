package rules

import (
	"context"

	"github.com/ramonskie/oxicleanarr/internal/clients"
	"github.com/ramonskie/oxicleanarr/internal/config"
	"github.com/ramonskie/oxicleanarr/internal/models"
	"github.com/ramonskie/oxicleanarr/internal/storage"
	"github.com/rs/zerolog/log"
)

// RulesEngine orchestrates two-phase rule evaluation.
// It is safe for concurrent use after full construction.
// Construction is a two-step process: NewRulesEngine builds the base engine,
// then SetSonarrClient and SetDiskMonitor inject late-bound dependencies.
// No mutation occurs after both injections are complete.
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

	// episodeRuleConfigs holds episode rule configs pending Sonarr client injection.
	// Populated during NewRulesEngine; converted to EpisodeRule instances by SetSonarrClient.
	episodeRuleConfigs []config.AdvancedRule

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
			// Episode rules require a Sonarr client, injected later via SetSonarrClient().
			// Store the config now; EpisodeRule instances are created on injection.
			e.episodeRuleConfigs = append(e.episodeRuleConfigs, rule)
			log.Debug().Str("rule", rule.Name).Msg("Episode rule registered (awaiting Sonarr client injection)")
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
func (e *RulesEngine) Evaluate(ctx context.Context, media *models.Media) RuleVerdict {
	evalCtx := EvalContext{
		Ctx:        ctx,
		Media:      media,
		Config:     config.Get(),
		DiskStatus: e.getDiskStatus(),
	}
	return e.evaluateWithContext(evalCtx)
}

// EvaluateForPreview runs evaluation with the disk threshold gate disabled.
// Used by GetLeavingSoon() to show what WOULD be deleted if threshold was breached.
// Thread-safe: passes nil DiskStatus in context, no shared state mutation.
func (e *RulesEngine) EvaluateForPreview(ctx context.Context, media *models.Media) RuleVerdict {
	evalCtx := EvalContext{
		Ctx:        ctx,
		Media:      media,
		Config:     config.Get(),
		DiskStatus: nil, // nil = DiskThresholdRule returns nil (no protection)
	}
	return e.evaluateWithContext(evalCtx)
}

// evaluateWithContext is the internal implementation shared by Evaluate and EvaluateForPreview.
func (e *RulesEngine) evaluateWithContext(ctx EvalContext) RuleVerdict {
	// ── PHASE 1: PROTECTION ──────────────────────────────────────────
	// Explicit exclusions always win — even over episode rules.
	// For other protection verdicts (disk OK, unwatched, etc.) we defer returning
	// so the episode chain can run independently (it bypasses the disk gate by design).
	var phase1Verdict *RuleVerdict
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
			v := RuleVerdict{
				IsProtected:      true,
				ProtectionReason: *status,
				ProtectingRule:   rule.Name(),
			}
			// Explicit exclusion is absolute — return immediately, skip episode chain.
			if *status == ProtectedExcluded {
				return v
			}
			phase1Verdict = &v
			break
		}
	}

	// ── EPISODE CHAIN (TV shows only, independent of Phase 1) ────────
	// Episode rules run regardless of disk pressure — they are count/age-based,
	// not disk-pressure-based. They must run before Phase 2 so that the standard
	// scheduling rule (tv_retention) cannot pre-empt them.
	//
	// EpisodeFileIDs are captured directly from the rule's Schedule() return via
	// ctx.Media — reset to nil before each rule to avoid stale values from cache
	// or previous rule iterations leaking into the verdict.
	if ctx.Media.Type == models.MediaTypeTVShow && len(e.episodeRules) > 0 {
		for _, rule := range e.episodeRules {
			ctx.Media.EpisodeFileIDs = nil // clear before each rule to avoid stale carry-over
			deleteAfter, source := rule.Schedule(ctx)
			episodeFileIDs := ctx.Media.EpisodeFileIDs // capture what this rule produced
			if !deleteAfter.IsZero() || len(episodeFileIDs) > 0 {
				log.Debug().
					Str("media_id", ctx.Media.ID).
					Str("rule", rule.Name()).
					Int("episode_file_count", len(episodeFileIDs)).
					Msg("Episode rule fired")
				return RuleVerdict{
					IsProtected:    false,
					DeleteAfter:    deleteAfter,
					ScheduleSource: source,
					SchedulingRule: rule.Name(),
					EpisodeFileIDs: episodeFileIDs,
				}
			}
		}
	}

	// Return Phase 1 protection verdict now that the episode chain has had its chance.
	if phase1Verdict != nil {
		return *phase1Verdict
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

	// No rule matched — no deletion scheduled
	return RuleVerdict{
		IsProtected:      true,
		ProtectionReason: ProtectedNoRule,
	}
}

// SetDiskMonitor injects a DiskMonitor into the engine after construction.
// Called by SyncEngine after it creates the DiskMonitor so that Evaluate()
// can gate scheduling decisions on real disk status.
func (e *RulesEngine) SetDiskMonitor(m DiskMonitor) {
	e.diskMonitor = m
}

// SetSonarrClient injects a SonarrClient and instantiates any pending episode rules.
// Called by SyncEngine after it creates the SonarrClient.
func (e *RulesEngine) SetSonarrClient(sonarr *clients.SonarrClient) {
	for _, ruleCfg := range e.episodeRuleConfigs {
		e.episodeRules = append(e.episodeRules, NewEpisodeRule(ruleCfg, sonarr))
		log.Info().Str("rule", ruleCfg.Name).Msg("Episode rule instantiated with Sonarr client")
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
