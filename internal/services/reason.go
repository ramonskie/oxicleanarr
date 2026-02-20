package services

import (
	"fmt"
	"time"

	"github.com/ramonskie/oxicleanarr/internal/models"
	"github.com/ramonskie/oxicleanarr/internal/services/rules"
)

// FormatDeletionReason converts a structured RuleVerdict into a
// human-readable explanation for the UI.
// Adding a new rule type = adding one case here. No string parsing.
func FormatDeletionReason(v rules.RuleVerdict, media *models.Media) string {
	if v.IsProtected {
		switch v.ProtectionReason {
		case rules.ProtectedExcluded:
			return "Manually excluded from deletion."
		case rules.ProtectedDiskOK:
			return "Disk space is adequate — rules are dormant."
		case rules.ProtectedUnwatched:
			return "Never delete unwatched content (unwatched_behavior: never)."
		case rules.ProtectedRequested:
			return "Protected: item was requested and no matching user rule applies."
		case rules.ProtectedByRule:
			if v.ProtectingRule != "" {
				return fmt.Sprintf("Protected by rule '%s'.", v.ProtectingRule)
			}
			return "Protected by a configured rule."
		default:
			return "No deletion scheduled."
		}
	}

	if v.HasEpisodeDeletions() {
		return fmt.Sprintf("Episode rule '%s': %d episode files scheduled for cleanup.",
			v.SchedulingRule, len(v.EpisodeFileIDs))
	}

	mediaType := "movie"
	if media.Type == models.MediaTypeTVShow {
		mediaType = "TV show"
	}

	baseDesc := formatRetentionBase(v.RetentionBase, media)

	switch v.ScheduleSource {
	case rules.SourceTagRule:
		tagInfo := v.TagLabel
		if tagInfo == "" {
			tagInfo = v.SchedulingRule
		}
		return fmt.Sprintf("This %s matches tag rule '%s' (tag: %s, %s retention, %s).",
			mediaType, v.SchedulingRule, tagInfo, v.RetentionValue, baseDesc)
	case rules.SourceUserRule:
		return fmt.Sprintf("This %s was requested and matches user rule '%s' (%s retention, %s).",
			mediaType, v.SchedulingRule, v.RetentionValue, baseDesc)
	case rules.SourceWatchedRule:
		return fmt.Sprintf("This %s matches watched rule '%s' (%s retention, %s).",
			mediaType, v.SchedulingRule, v.RetentionValue, baseDesc)
	case rules.SourceStandardRetention:
		return fmt.Sprintf("This %s uses standard %s retention (%s, %s).",
			mediaType, mediaType, v.RetentionValue, baseDesc)
	default:
		return fmt.Sprintf("This %s is scheduled for deletion.", mediaType)
	}
}

// formatRetentionBase builds a human-readable description of the retention base time.
func formatRetentionBase(retentionBase string, media *models.Media) string {
	switch retentionBase {
	case rules.RetentionBaseLastWatched, rules.RetentionBaseLastWatchedOrAdded:
		if !media.LastWatched.IsZero() {
			days := int(time.Since(media.LastWatched).Hours() / 24)
			return fmt.Sprintf("last watched %d days ago", days)
		}
		days := int(time.Since(media.AddedAt).Hours() / 24)
		return fmt.Sprintf("added %d days ago (never watched)", days)
	case rules.RetentionBaseAdded:
		days := int(time.Since(media.AddedAt).Hours() / 24)
		return fmt.Sprintf("added %d days ago", days)
	default:
		return "based on activity"
	}
}
