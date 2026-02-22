package rules

import (
	"sort"
	"time"

	"github.com/ramonskie/oxicleanarr/internal/clients"
	"github.com/ramonskie/oxicleanarr/internal/config"
	"github.com/ramonskie/oxicleanarr/internal/models"
	"github.com/rs/zerolog/log"
)

// EpisodeRule evaluates episode-specific cleanup rules for TV shows.
// It lives in the separate episode chain — it never sees the DiskThresholdRule gate.
// Episode rules populate RuleVerdict.EpisodeFileIDs with specific Sonarr episode file
// IDs to delete rather than scheduling whole-item deletion.
type EpisodeRule struct {
	rule         config.AdvancedRule
	sonarrClient *clients.SonarrClient
}

// NewEpisodeRule creates an EpisodeRule from an AdvancedRule config entry.
func NewEpisodeRule(rule config.AdvancedRule, sonarr *clients.SonarrClient) *EpisodeRule {
	return &EpisodeRule{rule: rule, sonarrClient: sonarr}
}

func (r *EpisodeRule) Name() string     { return r.rule.Name }
func (r *EpisodeRule) Scope() RuleScope { return ScopeEpisode }

// Protect always returns nil — episode rules never protect, only schedule.
// Protection for TV shows is handled by the standard protection chain.
func (r *EpisodeRule) Protect(_ EvalContext) *ProtectionStatus {
	return nil
}

// Schedule evaluates episode-specific rules and populates ctx.Media.EpisodeFileIDs.
// Returns (time.Now(), SourceEpisodeRule) when episodes are found to delete,
// or (time.Time{}, 0) when no episodes match.
func (r *EpisodeRule) Schedule(ctx EvalContext) (time.Time, ScheduleSource) {
	if ctx.Media.Type != models.MediaTypeTVShow {
		return time.Time{}, 0
	}

	if !r.showMatchesRule(ctx) {
		return time.Time{}, 0
	}

	// Check continuing series protection
	if r.rule.ExcludeContinuingSeries {
		series, err := r.sonarrClient.GetSeriesByID(ctx.Ctx, ctx.Media.SonarrID)
		if err != nil {
			log.Warn().Err(err).Str("media_id", ctx.Media.ID).
				Msg("Failed to fetch series status, skipping episode rule for safety")
			return time.Time{}, 0
		}
		if series.Status == "continuing" {
			log.Debug().Str("title", ctx.Media.Title).
				Msg("Show is continuing, episode rule skipped")
			return time.Time{}, 0
		}
	}

	// Fetch all episodes for this show
	episodes, err := r.sonarrClient.GetEpisodes(ctx.Ctx, ctx.Media.SonarrID)
	if err != nil {
		log.Warn().Err(err).Str("media_id", ctx.Media.ID).
			Msg("Failed to fetch episodes for rule evaluation")
		return time.Time{}, 0
	}

	// Apply the configured strategy
	var toDelete []int
	switch r.rule.EpisodeDeleteStrategy {
	case "oldest_first":
		toDelete = r.applyOldestFirstStrategy(episodes)
	case "by_age":
		toDelete = r.applyByAgeStrategy(episodes)
	case "by_season_age":
		toDelete = r.applyBySeasonAgeStrategy(episodes)
	default:
		// Auto-select strategy based on which criterion is configured
		if r.rule.MaxEpisodes > 0 {
			toDelete = r.applyOldestFirstStrategy(episodes)
		} else if r.rule.MaxAge != "" {
			toDelete = r.applyByAgeStrategy(episodes)
		}
	}

	if len(toDelete) == 0 {
		return time.Time{}, 0
	}

	log.Info().
		Str("title", ctx.Media.Title).
		Str("rule", r.rule.Name).
		Int("episode_files_to_delete", len(toDelete)).
		Msg("Episode rule matched, scheduling episode file deletions")

	// Store episode file IDs in media for SyncEngine to act on
	ctx.Media.EpisodeFileIDs = toDelete

	return time.Now(), SourceEpisodeRule
}

// showMatchesRule returns true if the media item matches this rule's tag filter.
// If no tag is configured, the rule applies to all TV shows.
func (r *EpisodeRule) showMatchesRule(ctx EvalContext) bool {
	if r.rule.Tag == "" {
		return true
	}
	for _, tag := range ctx.Media.Tags {
		if equalsCaseInsensitive(tag, r.rule.Tag) {
			return true
		}
	}
	return false
}

// applyOldestFirstStrategy keeps the newest max_episodes episodes and marks the rest for deletion.
// Episodes are sorted by air date (newest first); episodes beyond max_episodes are deleted.
func (r *EpisodeRule) applyOldestFirstStrategy(episodes []clients.SonarrEpisode) []int {
	withFiles := filterHasFile(episodes)

	if len(r.rule.SeasonNumbers) > 0 {
		withFiles = filterBySeason(withFiles, r.rule.SeasonNumbers)
	}

	if len(withFiles) <= r.rule.MaxEpisodes {
		return nil // under limit, nothing to delete
	}

	// Sort by air date, newest first
	sort.Slice(withFiles, func(i, j int) bool {
		return episodeAirDate(withFiles[i]).After(episodeAirDate(withFiles[j]))
	})

	var toDelete []int
	for i := r.rule.MaxEpisodes; i < len(withFiles); i++ {
		toDelete = append(toDelete, withFiles[i].EpisodeFileID)
	}
	return toDelete
}

// applyByAgeStrategy deletes episodes whose air date is older than max_age.
func (r *EpisodeRule) applyByAgeStrategy(episodes []clients.SonarrEpisode) []int {
	maxAge, err := parseDuration(r.rule.MaxAge)
	if err != nil {
		log.Warn().Err(err).Str("rule", r.rule.Name).Msg("Invalid max_age in episode rule")
		return nil
	}
	cutoff := time.Now().Add(-maxAge)

	var toDelete []int
	for _, ep := range episodes {
		if !ep.HasFile {
			continue
		}
		if len(r.rule.SeasonNumbers) > 0 && !containsInt(r.rule.SeasonNumbers, ep.SeasonNumber) {
			continue
		}
		airDate := episodeAirDate(ep)
		if airDate.IsZero() {
			continue // skip episodes with no air date
		}
		if airDate.Before(cutoff) {
			toDelete = append(toDelete, ep.EpisodeFileID)
		}
	}
	return toDelete
}

// applyBySeasonAgeStrategy deletes entire seasons whose oldest episode exceeds max_age.
// When keep_latest_season is true, the highest-numbered season is always preserved.
// If season_numbers is configured, only those seasons are considered for deletion.
func (r *EpisodeRule) applyBySeasonAgeStrategy(episodes []clients.SonarrEpisode) []int {
	maxAge, err := parseDuration(r.rule.MaxAge)
	if err != nil {
		log.Warn().Err(err).Str("rule", r.rule.Name).Msg("Invalid max_age in episode rule")
		return nil
	}
	cutoff := time.Now().Add(-maxAge)

	// Group episodes by season
	seasonMap := groupBySeason(episodes)

	// Find the latest (highest-numbered) season across all seasons (not filtered),
	// so keep_latest_season applies to the true latest season of the show.
	latestSeason := 0
	for seasonNum := range seasonMap {
		if seasonNum > latestSeason {
			latestSeason = seasonNum
		}
	}

	var toDelete []int
	for seasonNum, seasonEpisodes := range seasonMap {
		// Apply season_numbers filter: skip seasons not in the configured list.
		if len(r.rule.SeasonNumbers) > 0 && !containsInt(r.rule.SeasonNumbers, seasonNum) {
			continue
		}

		// Always keep the latest season when configured
		if r.rule.KeepLatestSeason && seasonNum == latestSeason {
			continue
		}

		// Find the oldest air date in this season.
		// Initial value is time.Now() — if all episodes lack an air date, oldestDate
		// stays at Now() which is never before the cutoff, so the season is preserved.
		// This is the correct safe default: prefer keeping over deleting on missing data.
		oldestDate := time.Now()
		for _, ep := range seasonEpisodes {
			airDate := episodeAirDate(ep)
			if !airDate.IsZero() && airDate.Before(oldestDate) {
				oldestDate = airDate
			}
		}

		// If the oldest episode in this season exceeds max_age, delete the entire season
		if oldestDate.Before(cutoff) {
			for _, ep := range seasonEpisodes {
				if ep.HasFile {
					toDelete = append(toDelete, ep.EpisodeFileID)
				}
			}
		}
	}
	return toDelete
}

// episodeAirDate returns the best available air date for an episode.
// Prefers AirDateUTC (full timestamp); falls back to parsing the AirDate string.
func episodeAirDate(ep clients.SonarrEpisode) time.Time {
	if !ep.AirDateUTC.IsZero() {
		return ep.AirDateUTC
	}
	if ep.AirDate != "" {
		t, err := time.Parse("2006-01-02", ep.AirDate)
		if err == nil {
			return t
		}
	}
	return time.Time{}
}

// filterHasFile returns only episodes that have a downloaded file.
func filterHasFile(episodes []clients.SonarrEpisode) []clients.SonarrEpisode {
	result := make([]clients.SonarrEpisode, 0, len(episodes))
	for _, ep := range episodes {
		if ep.HasFile {
			result = append(result, ep)
		}
	}
	return result
}

// filterBySeason returns only episodes belonging to the specified season numbers.
func filterBySeason(episodes []clients.SonarrEpisode, seasons []int) []clients.SonarrEpisode {
	result := make([]clients.SonarrEpisode, 0, len(episodes))
	for _, ep := range episodes {
		if containsInt(seasons, ep.SeasonNumber) {
			result = append(result, ep)
		}
	}
	return result
}

// groupBySeason groups episodes by their season number.
func groupBySeason(episodes []clients.SonarrEpisode) map[int][]clients.SonarrEpisode {
	m := make(map[int][]clients.SonarrEpisode)
	for _, ep := range episodes {
		m[ep.SeasonNumber] = append(m[ep.SeasonNumber], ep)
	}
	return m
}

// containsInt returns true if the slice contains the given integer.
func containsInt(slice []int, value int) bool {
	for _, v := range slice {
		if v == value {
			return true
		}
	}
	return false
}
