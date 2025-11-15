package services

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ramonskie/oxicleanarr/internal/config"
	"github.com/ramonskie/oxicleanarr/internal/models"
	"github.com/ramonskie/oxicleanarr/internal/storage"
	"github.com/rs/zerolog/log"
)

// RulesEngine evaluates retention rules for media items
type RulesEngine struct {
	config     *config.Config
	exclusions *storage.ExclusionsFile
	useGlobal  bool // If true, always fetch from config.Get() for hot-reload
}

// NewRulesEngine creates a new rules engine
func NewRulesEngine(cfg *config.Config, exclusions *storage.ExclusionsFile) *RulesEngine {
	return &RulesEngine{
		config:     cfg,
		exclusions: exclusions,
		useGlobal:  false, // Default to using passed config (for testing)
	}
}

// UseGlobalConfig enables fetching config from config.Get() for hot-reload support
func (e *RulesEngine) UseGlobalConfig() {
	e.useGlobal = true
}

// getConfig returns the appropriate config based on useGlobal setting
func (e *RulesEngine) getConfig() *config.Config {
	if e.useGlobal {
		return config.Get()
	}
	return e.config
}

// EvaluateMedia determines if a media item should be deleted and when
func (e *RulesEngine) EvaluateMedia(media *models.Media) (shouldDelete bool, deleteAfter time.Time, reason string) {
	// Check if excluded
	if e.exclusions.IsExcluded(media.ID) {
		return false, time.Time{}, "excluded"
	}

	// Check tag-based rules first (highest priority)
	if len(media.Tags) > 0 {
		matched, shouldDel, delAfter, tagReason := e.evaluateTagBasedRules(media)
		if matched {
			return shouldDel, delAfter, tagReason
		}
	}

	// Check user-based rules (higher priority than standard rules)
	if media.IsRequested && (media.RequestedByUserID != nil || media.RequestedByUsername != nil || media.RequestedByEmail != nil) {
		// Try to match user-based rules
		matched, shouldDel, delAfter, userReason := e.evaluateUserBasedRules(media)
		if matched {
			return shouldDel, delAfter, userReason
		}
		// If no user rule matched, fall through to standard retention rules
	}

	// Check watched-based rules (higher priority than standard rules)
	{
		matched, shouldDel, delAfter, watchedReason := e.evaluateWatchedRules(media)
		if matched {
			return shouldDel, delAfter, watchedReason
		}
	}

	// Get latest config for hot-reload support
	cfg := e.getConfig()

	// DEBUG: Log current retention values being used
	log.Debug().
		Str("media_id", media.ID).
		Str("media_type", string(media.Type)).
		Str("movie_retention", cfg.Rules.MovieRetention).
		Str("tv_retention", cfg.Rules.TVRetention).
		Bool("use_global", e.useGlobal).
		Msg("Rules engine evaluating media with current config")

	// Check if requested without user data (blanket protection only when no user-based rules exist)
	if media.IsRequested && len(cfg.AdvancedRules) == 0 {
		return false, time.Time{}, "requested"
	}

	// Get retention period
	var retentionDuration time.Duration
	var err error

	if media.Type == models.MediaTypeMovie {
		retentionDuration, err = parseDuration(cfg.Rules.MovieRetention)
	} else {
		retentionDuration, err = parseDuration(cfg.Rules.TVRetention)
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
	// Get latest config for hot-reload support
	cfg := e.getConfig()

	// No advanced rules configured
	if len(cfg.AdvancedRules) == 0 {
		return false, false, time.Time{}, ""
	}

	// Check if any user-based rules exist
	hasUserRules := false
	for _, rule := range cfg.AdvancedRules {
		if rule.Enabled && rule.Type == "user" {
			hasUserRules = true
			break
		}
	}

	// If user-based rules exist but media has no user data, log a warning
	if hasUserRules && media.IsRequested && media.RequestedByUserID == nil && media.RequestedByUsername == nil && media.RequestedByEmail == nil {
		log.Warn().
			Str("media_id", media.ID).
			Str("title", media.Title).
			Msg("User-based rules are configured but requested media has no user information - Jellyseerr may be disabled or not syncing properly")
	}

	// Find user-based rules
	for _, rule := range cfg.AdvancedRules {
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
	reason = fmt.Sprintf("user rule '%s' within retention (%s)", ruleName, userRule.Retention)
	log.Debug().
		Str("media_id", media.ID).
		Str("rule_name", ruleName).
		Time("delete_after", deleteAfter).
		Msg("Media matched user rule but within retention period")
	return true, false, deleteAfter, reason
}

// evaluateTagBasedRules checks if media matches any tag-based advanced rules
func (e *RulesEngine) evaluateTagBasedRules(media *models.Media) (matched bool, shouldDelete bool, deleteAfter time.Time, reason string) {
	// Get latest config for hot-reload support
	cfg := e.getConfig()

	// No advanced rules configured
	if len(cfg.AdvancedRules) == 0 {
		return false, false, time.Time{}, ""
	}

	// Find tag-based rules
	for _, rule := range cfg.AdvancedRules {
		if !rule.Enabled || rule.Type != "tag" || rule.Tag == "" {
			continue
		}

		// Check if media has this tag (case-insensitive match)
		hasTag := false
		for _, mediaTag := range media.Tags {
			if equalsCaseInsensitive(mediaTag, rule.Tag) {
				hasTag = true
				break
			}
		}

		if !hasTag {
			continue
		}

		// Media matches this tag rule
		log.Debug().
			Str("media_id", media.ID).
			Str("title", media.Title).
			Str("rule_name", rule.Name).
			Str("tag", rule.Tag).
			Msg("Media matched tag-based rule")

		// Parse retention period for this tag
		retentionDuration, err := parseDuration(rule.Retention)
		if err != nil {
			log.Warn().
				Err(err).
				Str("media_id", media.ID).
				Str("rule_name", rule.Name).
				Str("retention", rule.Retention).
				Msg("Failed to parse tag rule retention duration")
			return true, false, time.Time{}, "invalid tag rule retention"
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
			reason = fmt.Sprintf("tag rule '%s' (tag: %s) retention expired (%s)", rule.Name, rule.Tag, rule.Retention)
			log.Info().
				Str("media_id", media.ID).
				Str("title", media.Title).
				Str("rule_name", rule.Name).
				Str("tag", rule.Tag).
				Str("retention", rule.Retention).
				Time("delete_after", deleteAfter).
				Msg("Media matched tag-based rule for deletion")
			return true, true, deleteAfter, reason
		}

		// Within retention period
		reason = fmt.Sprintf("tag rule '%s' (tag: %s) within retention (%s)", rule.Name, rule.Tag, rule.Retention)
		log.Debug().
			Str("media_id", media.ID).
			Str("rule_name", rule.Name).
			Str("tag", rule.Tag).
			Time("delete_after", deleteAfter).
			Msg("Media matched tag rule but within retention period")
		return true, false, deleteAfter, reason
	}

	return false, false, time.Time{}, ""
}

// evaluateWatchedRules checks if media matches watched-based advanced rules
func (e *RulesEngine) evaluateWatchedRules(media *models.Media) (matched bool, shouldDelete bool, deleteAfter time.Time, reason string) {
	// Get latest config for hot-reload support
	cfg := e.getConfig()

	// No advanced rules configured
	if len(cfg.AdvancedRules) == 0 {
		return false, false, time.Time{}, ""
	}

	// Find watched-based rules
	for _, rule := range cfg.AdvancedRules {
		if !rule.Enabled || rule.Type != "watched" {
			continue
		}

		// Media matches this watched rule
		log.Debug().
			Str("media_id", media.ID).
			Str("title", media.Title).
			Str("rule_name", rule.Name).
			Msg("Media matched watched-based rule")

		// Parse retention period for this rule
		retentionDuration, err := parseDuration(rule.Retention)
		if err != nil {
			log.Warn().
				Err(err).
				Str("media_id", media.ID).
				Str("rule_name", rule.Name).
				Str("retention", rule.Retention).
				Msg("Failed to parse watched rule retention duration")
			return true, false, time.Time{}, "invalid watched rule retention"
		}

		// Check require_watched flag (at rule level)
		if rule.RequireWatched && media.WatchCount == 0 {
			log.Debug().
				Str("media_id", media.ID).
				Str("rule_name", rule.Name).
				Int("watch_count", media.WatchCount).
				Msg("Watched rule requires watched, but media not watched - skipping deletion")
			return true, false, time.Time{}, "watched rule: not watched yet"
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
			reason = fmt.Sprintf("watched rule '%s' retention expired (%s)", rule.Name, rule.Retention)
			log.Info().
				Str("media_id", media.ID).
				Str("title", media.Title).
				Str("rule_name", rule.Name).
				Str("retention", rule.Retention).
				Time("delete_after", deleteAfter).
				Msg("Media matched watched-based rule for deletion")
			return true, true, deleteAfter, reason
		}

		// Within retention period
		reason = fmt.Sprintf("watched rule '%s' within retention (%s)", rule.Name, rule.Retention)
		log.Debug().
			Str("media_id", media.ID).
			Str("rule_name", rule.Name).
			Time("delete_after", deleteAfter).
			Msg("Media matched watched rule but within retention period")
		return true, false, deleteAfter, reason
	}

	return false, false, time.Time{}, ""
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
	// Get latest config for hot-reload support
	cfg := e.getConfig()

	leavingSoon := make([]models.Media, 0)
	leavingSoonDays := cfg.App.LeavingSoonDays

	for _, media := range mediaList {
		shouldDelete, deleteAfter, reason := e.EvaluateMedia(&media)
		if !shouldDelete && !deleteAfter.IsZero() {
			daysUntilDue := int(time.Until(deleteAfter).Hours() / 24)
			if daysUntilDue > 0 && daysUntilDue <= leavingSoonDays {
				media.DeleteAfter = deleteAfter
				media.DaysUntilDue = daysUntilDue
				media.DeletionReason = e.GenerateDeletionReason(&media, deleteAfter, reason)
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
func (e *RulesEngine) GenerateDeletionReason(media *models.Media, deleteAfter time.Time, reason string) string {
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

	// Check if this is a tag-based rule (reason contains "tag rule")
	if len(reason) > 9 && reason[:8] == "tag rule" {
		// Extract rule name, tag, and retention from reason like:
		// - "tag rule 'Demo Content' (tag: prunarr-test) retention expired (1d)" -> overdue
		// - "tag rule 'Demo Content' (tag: prunarr-test) within retention (1d)" -> leaving soon

		// Find the rule name between single quotes
		start := -1
		end := -1
		for i := 9; i < len(reason); i++ {
			if reason[i] == '\'' {
				if start == -1 {
					start = i + 1
				} else {
					end = i
					break
				}
			}
		}

		// Find tag name after "tag: "
		tagStart := -1
		tagEnd := -1
		tagPrefix := "tag: "
		tagIdx := strings.Index(reason, tagPrefix)
		if tagIdx != -1 {
			tagStart = tagIdx + len(tagPrefix)
			// Find end of tag (before closing parenthesis)
			for i := tagStart; i < len(reason); i++ {
				if reason[i] == ')' {
					tagEnd = i
					break
				}
			}
		}

		// Find retention period in the last set of parentheses
		retentionStart := -1
		retentionEnd := -1
		for i := len(reason) - 1; i >= 0; i-- {
			if reason[i] == ')' && retentionEnd == -1 {
				retentionEnd = i
			} else if reason[i] == '(' && retentionEnd != -1 {
				retentionStart = i + 1
				break
			}
		}

		if start != -1 && end != -1 && tagStart != -1 && tagEnd != -1 && retentionStart != -1 && retentionEnd != -1 {
			ruleName := reason[start:end]
			tagName := reason[tagStart:tagEnd]
			retention := reason[retentionStart:retentionEnd]

			// Check if this is an expired rule or within retention
			isExpired := strings.Contains(reason, "retention expired")

			if isExpired {
				// Item is overdue for deletion
				return fmt.Sprintf("This %s was %s %d days ago. It matched the '%s' tag rule (tag: %s) with %s retention and is now scheduled for deletion.",
					mediaType, baseEvent, daysSinceBase, ruleName, tagName, retention)
			} else {
				// Item is leaving soon (within retention but will be deleted)
				return fmt.Sprintf("This %s was %s %d days ago. It matches the '%s' tag rule (tag: %s) with %s retention, meaning it will be deleted after that period of inactivity.",
					mediaType, baseEvent, daysSinceBase, ruleName, tagName, retention)
			}
		}

		// Fallback if parsing fails
		return fmt.Sprintf("This %s was %s %d days ago. %s.",
			mediaType, baseEvent, daysSinceBase, reason)
	}

	// Check if this is a user-based rule (reason contains "user rule")
	if len(reason) > 10 && reason[:9] == "user rule" {
		// Extract rule name and retention from reason like:
		// - "user rule 'Trial Users' retention expired (30d)" -> overdue
		// - "user rule 'Trial Users' within retention (30d)" -> leaving soon

		// Find the rule name between single quotes
		start := -1
		end := -1
		for i := 10; i < len(reason); i++ {
			if reason[i] == '\'' {
				if start == -1 {
					start = i + 1
				} else {
					end = i
					break
				}
			}
		}

		// Find retention period in parentheses
		retentionStart := -1
		retentionEnd := -1
		for i := 0; i < len(reason); i++ {
			if reason[i] == '(' {
				retentionStart = i + 1
			} else if reason[i] == ')' {
				retentionEnd = i
				break
			}
		}

		if start != -1 && end != -1 && retentionStart != -1 && retentionEnd != -1 {
			ruleName := reason[start:end]
			retention := reason[retentionStart:retentionEnd]

			// Check if this is an expired rule or within retention
			isExpired := strings.Contains(reason, "retention expired")

			if isExpired {
				// Item is overdue for deletion
				return fmt.Sprintf("This %s was %s %d days ago. It matched the '%s' user rule with %s retention and is now scheduled for deletion.",
					mediaType, baseEvent, daysSinceBase, ruleName, retention)
			} else {
				// Item is leaving soon (within retention but will be deleted)
				return fmt.Sprintf("This %s was %s %d days ago. It matches the '%s' user rule with %s retention, meaning it will be deleted after that period of inactivity.",
					mediaType, baseEvent, daysSinceBase, ruleName, retention)
			}
		}

		// Fallback if parsing fails
		return fmt.Sprintf("This %s was %s %d days ago. %s.",
			mediaType, baseEvent, daysSinceBase, reason)
	}

	// Check if this is a watched-based rule (reason contains "watched rule")
	if len(reason) > 13 && reason[:12] == "watched rule" {
		// Extract rule name and retention from reason like:
		// - "watched rule 'Auto Clean' retention expired (30d)" -> overdue
		// - "watched rule 'Auto Clean' within retention (30d)" -> leaving soon

		// Find the rule name between single quotes
		start := -1
		end := -1
		for i := 13; i < len(reason); i++ {
			if reason[i] == '\'' {
				if start == -1 {
					start = i + 1
				} else {
					end = i
					break
				}
			}
		}

		// Find retention period in parentheses
		retentionStart := -1
		retentionEnd := -1
		for i := 0; i < len(reason); i++ {
			if reason[i] == '(' {
				retentionStart = i + 1
			} else if reason[i] == ')' {
				retentionEnd = i
				break
			}
		}

		if start != -1 && end != -1 && retentionStart != -1 && retentionEnd != -1 {
			ruleName := reason[start:end]
			retention := reason[retentionStart:retentionEnd]

			// Check if this is an expired rule or within retention
			isExpired := strings.Contains(reason, "retention expired")

			if isExpired {
				// Item is overdue for deletion
				return fmt.Sprintf("This %s was %s %d days ago. It matched the '%s' watched rule with %s retention and is now scheduled for deletion.",
					mediaType, baseEvent, daysSinceBase, ruleName, retention)
			} else {
				// Item is leaving soon (within retention but will be deleted)
				return fmt.Sprintf("This %s was %s %d days ago. It matches the '%s' watched rule with %s retention, meaning it will be deleted after that period of inactivity.",
					mediaType, baseEvent, daysSinceBase, ruleName, retention)
			}
		}

		// Fallback if parsing fails
		return fmt.Sprintf("This %s was %s %d days ago. %s.",
			mediaType, baseEvent, daysSinceBase, reason)
	}

	// Standard retention rule
	retentionPeriod := e.getRetentionString(media.Type)
	return fmt.Sprintf("This %s was %s %d days ago. The retention policy for %ss is %s, meaning it will be deleted after that period of inactivity.",
		mediaType, baseEvent, daysSinceBase, mediaType, retentionPeriod)
}

// getRetentionString returns the human-readable retention period
func (e *RulesEngine) getRetentionString(mediaType models.MediaType) string {
	// Get latest config for hot-reload support
	cfg := e.getConfig()

	if mediaType == models.MediaTypeMovie {
		return cfg.Rules.MovieRetention
	}
	return cfg.Rules.TVRetention
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
