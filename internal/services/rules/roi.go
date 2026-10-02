package rules

import (
	"time"

	"github.com/ramonskie/oxicleanarr/internal/config"
	"github.com/ramonskie/oxicleanarr/internal/models"
	"github.com/ramonskie/oxicleanarr/internal/services/analytics"
)

// DefaultSuggestDeletionDays mirrors the analytics config default.
const DefaultSuggestDeletionDays = 180

// RoiRule schedules deletion for low-value content: storage whose watch hours
// per GB (or per-rule cutoff) are too low and which has not been watched within
// the suggestion window. Everything else is protected, so the rule expresses
// "delete only low-ROI content". The rule is inert unless both analytics and a
// stats provider are enabled — without watch data every item would look
// low-value.
type RoiRule struct {
	rule config.AdvancedRule
}

// NewROIRule creates a RoiRule from an AdvancedRule config entry.
func NewROIRule(rule config.AdvancedRule) *RoiRule { return &RoiRule{rule: rule} }

func (r *RoiRule) Name() string     { return r.rule.Name }
func (r *RoiRule) Scope() RuleScope { return ScopeAll }

// Protect keeps content that the ROI analysis does not flag for deletion.
func (r *RoiRule) Protect(ctx EvalContext) *ProtectionStatus {
	if !r.eligible(ctx.Config) {
		return nil
	}
	if !r.suggestsDeletion(ctx, time.Now()) {
		s := ProtectedByRule
		return &s
	}
	return nil
}

// Schedule returns the date the item became eligible for deletion (last watch,
// or add date when never watched, plus the suggestion window).
func (r *RoiRule) Schedule(ctx EvalContext) (time.Time, ScheduleSource) {
	if !r.eligible(ctx.Config) || !r.suggestsDeletion(ctx, time.Now()) {
		return time.Time{}, 0
	}

	basis := ctx.Media.LastWatched
	if basis.IsZero() {
		basis = ctx.Media.AddedAt
	}
	if basis.IsZero() {
		return time.Time{}, 0
	}

	return basis.AddDate(0, 0, suggestDays(ctx.Config)), SourceRoiRule
}

// EnrichVerdict implements VerdictEnricher.
func (r *RoiRule) EnrichVerdict(_ EvalContext) (retentionValue, retentionBase, tagLabel string) {
	return "low value", "roi", ""
}

// eligible reports whether ROI evaluation is meaningful: analytics must be on
// and a watch-history provider configured.
func (r *RoiRule) eligible(cfg *config.Config) bool {
	return analyticsEnabled(cfg) && statsProviderEnabled(cfg)
}

func (r *RoiRule) suggestsDeletion(ctx EvalContext, now time.Time) bool {
	cfg := ctx.Config
	if cfg == nil || ctx.Media == nil {
		return false
	}

	result := analytics.ROI(now, roiInputForMedia(ctx.Media), cfg.Analytics)

	lowValue := result.ValueCategory == analytics.ValueLow
	// The override only applies to real files: a zero-size item has no ROI
	// signal (the category path treats it as moderate).
	if r.rule.MinWatchHoursPerGB > 0 && ctx.Media.FileSize > 0 {
		lowValue = result.WatchHoursPerGB < r.rule.MinWatchHoursPerGB
	}

	return lowValue && result.OldEnough
}

// roiInputForMedia maps a media item to the analytics ROI input. OxiCleanarr
// stores items at movie/show level, so TV shows use the show band.
func roiInputForMedia(media *models.Media) analytics.ROIInput {
	kind := analytics.KindShow
	if media.Type == models.MediaTypeMovie {
		kind = analytics.KindMovie
	}
	return analytics.ROIInput{
		FileSizeBytes:     media.FileSize,
		TotalWatchSeconds: media.TotalWatchSeconds,
		LastWatched:       media.LastWatched,
		AddedAt:           media.AddedAt,
		Kind:              kind,
	}
}

func suggestDays(cfg *config.Config) int {
	if cfg != nil && cfg.Analytics.SuggestDeletionDays > 0 {
		return cfg.Analytics.SuggestDeletionDays
	}
	return DefaultSuggestDeletionDays
}

// statsProviderEnabled reports whether any watch-history provider is configured.
func statsProviderEnabled(cfg *config.Config) bool {
	if cfg == nil {
		return false
	}
	return cfg.Integrations.Tracearr.Enabled ||
		cfg.Integrations.Jellystat.Enabled ||
		cfg.Integrations.Streamystats.Enabled
}
