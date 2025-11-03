package services

import (
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/ramonskie/prunarr/internal/config"
	"github.com/ramonskie/prunarr/internal/models"
	"github.com/ramonskie/prunarr/internal/storage"
	"github.com/rs/zerolog/log"
)

// RulesEngine evaluates retention rules for media items
type RulesEngine struct {
	config     *config.Config
	exclusions *storage.ExclusionsFile
}

// NewRulesEngine creates a new rules engine
func NewRulesEngine(cfg *config.Config, exclusions *storage.ExclusionsFile) *RulesEngine {
	return &RulesEngine{
		config:     cfg,
		exclusions: exclusions,
	}
}

// EvaluateMedia determines if a media item should be deleted and when
func (e *RulesEngine) EvaluateMedia(media *models.Media) (shouldDelete bool, deleteAfter time.Time, reason string) {
	// Check if excluded
	if e.exclusions.IsExcluded(media.ID) {
		return false, time.Time{}, "excluded"
	}

	// Check user-based rules first (higher priority than standard rules)
	if media.IsRequested && (media.RequestedByUserID != nil || media.RequestedByUsername != nil || media.RequestedByEmail != nil) {
		// Try to match user-based rules
		matched, shouldDel, delAfter, userReason := e.evaluateUserBasedRules(media)
		if matched {
			return shouldDel, delAfter, userReason
		}
		// If no user rule matched, fall through to standard retention rules
	}

	// Check if requested without user data (blanket protection only when no user-based rules exist)
	if media.IsRequested && len(e.config.AdvancedRules) == 0 {
		return false, time.Time{}, "requested"
	}

	// Get retention period
	var retentionDuration time.Duration
	var err error

	if media.Type == models.MediaTypeMovie {
		retentionDuration, err = parseDuration(e.config.Rules.MovieRetention)
	} else {
		retentionDuration, err = parseDuration(e.config.Rules.TVRetention)
	}

	if err != nil {
		log.Warn().
			Err(err).
			Str("media_id", media.ID).
			Str("type", string(media.Type)).
			Msg("Failed to parse retention duration")
		return false, time.Time{}, "invalid retention"
	}

	// Check if retention is disabled (0d or "never")
	if retentionDuration == 0 {
		log.Debug().
			Str("media_id", media.ID).
			Str("type", string(media.Type)).
			Msg("Standard retention disabled for this media type")
		return false, time.Time{}, "retention disabled"
	}

	// Calculate deletion time based on last watched or added date
	var baseTime time.Time
	if !media.LastWatched.IsZero() {
		baseTime = media.LastWatched
	} else {
		baseTime = media.AddedAt
	}

	deleteAfter = baseTime.Add(retentionDuration)

	// Check if due for deletion
	if time.Now().After(deleteAfter) {
		reason = fmt.Sprintf("retention period expired (%s)", e.getRetentionString(media.Type))
		return true, deleteAfter, reason
	}

	return false, deleteAfter, "within retention"
}

// evaluateUserBasedRules checks if media matches any user-based advanced rules
func (e *RulesEngine) evaluateUserBasedRules(media *models.Media) (matched bool, shouldDelete bool, deleteAfter time.Time, reason string) {
	// No advanced rules configured
	if len(e.config.AdvancedRules) == 0 {
		return false, false, time.Time{}, ""
	}

	// Find user-based rules
	for _, rule := range e.config.AdvancedRules {
		if !rule.Enabled || rule.Type != "user" {
			continue
		}

		// Check each user in the rule
		for _, userRule := range rule.Users {
			// Match by user ID
			if userRule.UserID != nil && media.RequestedByUserID != nil && *userRule.UserID == *media.RequestedByUserID {
				return e.applyUserRule(media, &userRule, rule.Name)
			}

			// Match by username (case-insensitive)
			if userRule.Username != "" && media.RequestedByUsername != nil {
				if equalsCaseInsensitive(userRule.Username, *media.RequestedByUsername) {
					return e.applyUserRule(media, &userRule, rule.Name)
				}
			}

			// Match by email (case-insensitive)
			if userRule.Email != "" && media.RequestedByEmail != nil {
				if equalsCaseInsensitive(userRule.Email, *media.RequestedByEmail) {
					return e.applyUserRule(media, &userRule, rule.Name)
				}
			}
		}
	}

	return false, false, time.Time{}, ""
}

// applyUserRule applies a matched user rule to determine deletion
func (e *RulesEngine) applyUserRule(media *models.Media, userRule *config.UserRule, ruleName string) (matched bool, shouldDelete bool, deleteAfter time.Time, reason string) {
	// Parse retention period for this user
	retentionDuration, err := parseDuration(userRule.Retention)
	if err != nil {
		log.Warn().
			Err(err).
			Str("media_id", media.ID).
			Str("rule_name", ruleName).
			Str("retention", userRule.Retention).
			Msg("Failed to parse user rule retention duration")
		return true, false, time.Time{}, "invalid user rule retention"
	}

	// Check require_watched flag
	requireWatched := false
	if userRule.RequireWatched != nil {
		requireWatched = *userRule.RequireWatched
	}

	// If require_watched is true and media hasn't been watched, don't delete
	if requireWatched && media.WatchCount == 0 {
		log.Debug().
			Str("media_id", media.ID).
			Str("rule_name", ruleName).
			Msg("User rule requires watched, but media not watched - skipping deletion")
		return true, false, time.Time{}, "user rule: not watched yet"
	}

	// Calculate deletion time based on last watched or added date
	var baseTime time.Time
	if !media.LastWatched.IsZero() {
		baseTime = media.LastWatched
	} else {
		baseTime = media.AddedAt
	}

	deleteAfter = baseTime.Add(retentionDuration)

	// Check if due for deletion
	if time.Now().After(deleteAfter) {
		reason = fmt.Sprintf("user rule '%s' retention expired (%s)", ruleName, userRule.Retention)
		log.Info().
			Str("media_id", media.ID).
			Str("title", media.Title).
			Str("rule_name", ruleName).
			Str("retention", userRule.Retention).
			Time("delete_after", deleteAfter).
			Msg("Media matched user-based rule for deletion")
		return true, true, deleteAfter, reason
	}

	// Within retention period
	log.Debug().
		Str("media_id", media.ID).
		Str("rule_name", ruleName).
		Time("delete_after", deleteAfter).
		Msg("Media matched user rule but within retention period")
	return true, false, deleteAfter, "user rule: within retention"
}

// equalsCaseInsensitive compares two strings case-insensitively
func equalsCaseInsensitive(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	return stringToLower(a) == stringToLower(b)
}

// stringToLower converts a string to lowercase manually to avoid imports
func stringToLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			result[i] = c + 32
		} else {
			result[i] = c
		}
	}
	return string(result)
}

// GetDeletionCandidates returns all media items ready for deletion
func (e *RulesEngine) GetDeletionCandidates(mediaList []models.Media) []models.DeletionCandidate {
	candidates := make([]models.DeletionCandidate, 0)

	for _, media := range mediaList {
		shouldDelete, deleteAfter, reason := e.EvaluateMedia(&media)
		if shouldDelete {
			daysOverdue := int(time.Since(deleteAfter).Hours() / 24)
			candidates = append(candidates, models.DeletionCandidate{
				Media:        media,
				Reason:       reason,
				RetentionDue: deleteAfter,
				DaysOverdue:  daysOverdue,
				SizeBytes:    media.FileSize,
			})
		}
	}

	log.Info().
		Int("total_media", len(mediaList)).
		Int("candidates", len(candidates)).
		Msg("Evaluated media for deletion")

	return candidates
}

// GetLeavingSoon returns media items that will be deleted soon
func (e *RulesEngine) GetLeavingSoon(mediaList []models.Media) []models.Media {
	leavingSoon := make([]models.Media, 0)
	leavingSoonDays := e.config.App.LeavingSoonDays

	for _, media := range mediaList {
		shouldDelete, deleteAfter, _ := e.EvaluateMedia(&media)
		if !shouldDelete && !deleteAfter.IsZero() {
			daysUntilDue := int(time.Until(deleteAfter).Hours() / 24)
			if daysUntilDue > 0 && daysUntilDue <= leavingSoonDays {
				media.DeleteAfter = deleteAfter
				media.DaysUntilDue = daysUntilDue
				media.DeletionReason = e.GenerateDeletionReason(&media, deleteAfter)
				leavingSoon = append(leavingSoon, media)
			}
		}
	}

	log.Debug().
		Int("leaving_soon", len(leavingSoon)).
		Int("threshold_days", leavingSoonDays).
		Msg("Found leaving soon media")

	return leavingSoon
}

// GenerateDeletionReason creates a human-readable explanation for why an item is scheduled for deletion
func (e *RulesEngine) GenerateDeletionReason(media *models.Media, deleteAfter time.Time) string {
	retentionPeriod := e.getRetentionString(media.Type)

	// Determine if based on last watched or added date
	var baseEvent string
	var baseDate time.Time
	if !media.LastWatched.IsZero() {
		baseEvent = "last watched"
		baseDate = media.LastWatched
	} else {
		baseEvent = "added"
		baseDate = media.AddedAt
	}

	// Format the base date nicely
	daysSinceBase := int(time.Since(baseDate).Hours() / 24)

	mediaType := "movie"
	if media.Type == models.MediaTypeTVShow {
		mediaType = "TV show"
	}

	return fmt.Sprintf("This %s was %s %d days ago. The retention policy for %ss is %s, meaning it will be deleted after that period of inactivity.",
		mediaType, baseEvent, daysSinceBase, mediaType, retentionPeriod)
}

// getRetentionString returns the human-readable retention period
func (e *RulesEngine) getRetentionString(mediaType models.MediaType) string {
	if mediaType == models.MediaTypeMovie {
		return e.config.Rules.MovieRetention
	}
	return e.config.Rules.TVRetention
}

// parseDuration parses duration strings like "90d", "24h", "30m", or special values "never"/"0d" to disable
func parseDuration(s string) (time.Duration, error) {
	if s == "" {
		return 0, fmt.Errorf("empty duration string")
	}

	// Handle special values to disable retention
	if s == "never" || s == "0d" {
		return 0, nil // Return 0 to indicate disabled
	}

	// Match patterns like "90d", "24h", "30m"
	re := regexp.MustCompile(`^(\d+)([dhms])$`)
	matches := re.FindStringSubmatch(s)

	if len(matches) != 3 {
		return 0, fmt.Errorf("invalid duration format: %s (expected format: 90d, 24h, 30m, or 'never')", s)
	}

	value, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, fmt.Errorf("invalid duration value: %w", err)
	}

	unit := matches[2]
	switch unit {
	case "d":
		return time.Duration(value) * 24 * time.Hour, nil
	case "h":
		return time.Duration(value) * time.Hour, nil
	case "m":
		return time.Duration(value) * time.Minute, nil
	case "s":
		return time.Duration(value) * time.Second, nil
	default:
		return 0, fmt.Errorf("unknown duration unit: %s", unit)
	}
}
