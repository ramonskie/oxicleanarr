package services

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/ramonskie/oxicleanarr/internal/cache"
	"github.com/ramonskie/oxicleanarr/internal/clients"
	"github.com/ramonskie/oxicleanarr/internal/config"
	"github.com/ramonskie/oxicleanarr/internal/models"
	"github.com/ramonskie/oxicleanarr/internal/storage"
	"github.com/rs/zerolog/log"
)

// SyncEngine handles media synchronization and cleanup operations
type SyncEngine struct {
	config     *config.Config
	cache      *cache.Cache
	jobs       *storage.JobsFile
	exclusions *storage.ExclusionsFile
	rules      *RulesEngine

	jellyfinClient        *clients.JellyfinClient
	radarrClient          *clients.RadarrClient
	sonarrClient          *clients.SonarrClient
	jellyseerrClient      *clients.JellyseerrClient
	jellystatClient       *clients.JellystatClient
	symlinkLibraryManager *SymlinkLibraryManager

	mediaLibrary     map[string]models.Media
	mediaLibraryLock sync.RWMutex

	fullSyncTicker *time.Ticker
	incrSyncTicker *time.Ticker
	stopChan       chan struct{}
	running        bool
	runningLock    sync.Mutex
}

// NewSyncEngine creates a new sync engine
func NewSyncEngine(
	cfg *config.Config,
	cacheInstance *cache.Cache,
	jobs *storage.JobsFile,
	exclusions *storage.ExclusionsFile,
	rules *RulesEngine,
) *SyncEngine {
	engine := &SyncEngine{
		config:       cfg,
		cache:        cacheInstance,
		jobs:         jobs,
		exclusions:   exclusions,
		rules:        rules,
		mediaLibrary: make(map[string]models.Media),
		stopChan:     make(chan struct{}),
	}

	// Initialize clients based on config
	if cfg.Integrations.Jellyfin.Enabled {
		engine.jellyfinClient = clients.NewJellyfinClient(cfg.Integrations.Jellyfin)
	}
	if cfg.Integrations.Radarr.Enabled {
		engine.radarrClient = clients.NewRadarrClient(cfg.Integrations.Radarr)
	}
	if cfg.Integrations.Sonarr.Enabled {
		engine.sonarrClient = clients.NewSonarrClient(cfg.Integrations.Sonarr)
	}
	if cfg.Integrations.Jellyseerr.Enabled {
		engine.jellyseerrClient = clients.NewJellyseerrClient(cfg.Integrations.Jellyseerr)
	}
	if cfg.Integrations.Jellystat.Enabled {
		engine.jellystatClient = clients.NewJellystatClient(cfg.Integrations.Jellystat)
	}

	// Initialize symlink library manager if enabled
	if cfg.Integrations.Jellyfin.Enabled && cfg.Integrations.Jellyfin.SymlinkLibrary.Enabled {
		engine.symlinkLibraryManager = NewSymlinkLibraryManager(engine.jellyfinClient, cfg)
		log.Info().Msg("Symlink library manager initialized")
	}

	return engine
}

// Start begins the sync scheduler
func (e *SyncEngine) Start() error {
	e.runningLock.Lock()
	defer e.runningLock.Unlock()

	if e.running {
		return fmt.Errorf("sync engine already running")
	}

	e.running = true

	// Always read current config values (supports hot-reload)
	cfg := config.Get()
	fullInterval := time.Duration(cfg.Sync.FullInterval) * time.Minute
	incrInterval := time.Duration(cfg.Sync.IncrementalInterval) * time.Minute

	// Only start sync scheduler if auto-start is enabled
	if cfg.Sync.AutoStart {
		// Start full sync ticker
		e.fullSyncTicker = time.NewTicker(fullInterval)

		// Start incremental sync ticker
		e.incrSyncTicker = time.NewTicker(incrInterval)

		// Run initial full sync immediately
		go func() {
			ctx := context.Background()
			if err := e.FullSync(ctx); err != nil {
				log.Error().Err(err).Msg("Initial full sync failed")
			}
		}()

		// Start ticker goroutines
		go e.runFullSyncLoop()
		go e.runIncrementalSyncLoop()

		log.Info().
			Dur("full_interval", fullInterval).
			Dur("incr_interval", incrInterval).
			Bool("auto_start", true).
			Msg("Sync engine started with automatic scheduling")
	} else {
		log.Info().
			Bool("auto_start", false).
			Msg("Sync engine started in manual mode (no automatic scheduling)")
	}

	return nil
}

// Stop stops the sync scheduler
func (e *SyncEngine) Stop() {
	e.runningLock.Lock()
	defer e.runningLock.Unlock()

	if !e.running {
		return
	}

	e.running = false
	close(e.stopChan)

	if e.fullSyncTicker != nil {
		e.fullSyncTicker.Stop()
	}
	if e.incrSyncTicker != nil {
		e.incrSyncTicker.Stop()
	}

	log.Info().Msg("Sync engine stopped")
}

// RestartScheduler restarts the sync scheduler with updated intervals from config
// This is useful when config changes require updating the sync intervals without restarting the application
func (e *SyncEngine) RestartScheduler() error {
	log.Info().Msg("Restarting sync scheduler with updated intervals")

	e.runningLock.Lock()
	wasRunning := e.running
	e.runningLock.Unlock()

	// Only restart if it was running
	if !wasRunning {
		log.Info().Msg("Scheduler was not running, skipping restart")
		return nil
	}

	// Stop the scheduler
	e.Stop()

	// Wait briefly for goroutines to exit cleanly
	time.Sleep(100 * time.Millisecond)

	// Recreate the stop channel (since Stop() closed it)
	e.stopChan = make(chan struct{})

	// Restart with new config values
	if err := e.Start(); err != nil {
		return fmt.Errorf("failed to restart scheduler: %w", err)
	}

	// Log new intervals from current config
	cfg := config.Get()
	log.Info().
		Int("full_interval", cfg.Sync.FullInterval).
		Int("incr_interval", cfg.Sync.IncrementalInterval).
		Msg("Sync scheduler restarted successfully")

	return nil
}

// runFullSyncLoop runs full sync on schedule
func (e *SyncEngine) runFullSyncLoop() {
	for {
		select {
		case <-e.fullSyncTicker.C:
			ctx := context.Background()
			if err := e.FullSync(ctx); err != nil {
				log.Error().Err(err).Msg("Scheduled full sync failed")
			}
		case <-e.stopChan:
			return
		}
	}
}

// runIncrementalSyncLoop runs incremental sync on schedule
func (e *SyncEngine) runIncrementalSyncLoop() {
	for {
		select {
		case <-e.incrSyncTicker.C:
			ctx := context.Background()
			if err := e.IncrementalSync(ctx); err != nil {
				log.Error().Err(err).Msg("Scheduled incremental sync failed")
			}
		case <-e.stopChan:
			return
		}
	}
}

// FullSync performs a complete sync of all media
func (e *SyncEngine) FullSync(ctx context.Context) error {
	jobID := uuid.New().String()
	startTime := time.Now()

	log.Info().Str("job_id", jobID).Msg("Starting full sync")

	// Create job entry
	job := storage.Job{
		ID:        jobID,
		Type:      storage.JobTypeFullSync,
		Status:    storage.JobStatusRunning,
		StartedAt: startTime,
		Summary:   make(map[string]any),
	}

	if err := e.jobs.Add(job); err != nil {
		log.Warn().Err(err).Msg("Failed to create job entry")
	}

	// Sync all services
	movieCount := 0
	tvShowCount := 0
	var lastErr error

	// Sync movies from Radarr
	if e.radarrClient != nil {
		movies, err := e.syncRadarr(ctx)
		if err != nil {
			lastErr = err
			log.Error().Err(err).Msg("Failed to sync Radarr")
		} else {
			movieCount = len(movies)
		}
	}

	// Sync TV shows from Sonarr
	if e.sonarrClient != nil {
		shows, err := e.syncSonarr(ctx)
		if err != nil {
			lastErr = err
			log.Error().Err(err).Msg("Failed to sync Sonarr")
		} else {
			tvShowCount = len(shows)
		}
	}

	// Sync Jellyfin watch data
	if e.jellyfinClient != nil {
		if err := e.syncJellyfin(ctx); err != nil {
			lastErr = err
			log.Error().Err(err).Msg("Failed to sync Jellyfin")
		}
	}

	// Sync detailed watch history from Jellystat
	if e.jellystatClient != nil {
		if err := e.syncJellystat(ctx); err != nil {
			lastErr = err
			log.Error().Err(err).Msg("Failed to sync Jellystat")
		}
	}

	// Sync requested items from Jellyseerr
	if e.jellyseerrClient != nil {
		if err := e.syncJellyseerr(ctx); err != nil {
			lastErr = err
			log.Error().Err(err).Msg("Failed to sync Jellyseerr")
		}
	} else {
		// Check if user-based rules are configured but Jellyseerr is disabled
		cfg := config.Get()
		hasUserRules := false
		for _, rule := range cfg.AdvancedRules {
			if rule.Enabled && rule.Type == "user" {
				hasUserRules = true
				break
			}
		}
		if hasUserRules {
			log.Warn().
				Msg("User-based advanced rules are configured but Jellyseerr is disabled - user rules will not work without Jellyseerr integration")
		}
	}

	// Apply exclusions from file
	e.applyExclusions()

	// Apply retention rules to all media
	e.applyRetentionRules()

	// Sync symlink libraries for "Leaving Soon" items
	leavingSoonCount := 0
	if e.symlinkLibraryManager != nil {
		e.mediaLibraryLock.RLock()
		mediaLibraryCopy := make(map[string]models.Media, len(e.mediaLibrary))
		for k, v := range e.mediaLibrary {
			mediaLibraryCopy[k] = v
		}
		e.mediaLibraryLock.RUnlock()

		var err error
		leavingSoonCount, err = e.symlinkLibraryManager.SyncLibraries(ctx, mediaLibraryCopy)
		if err != nil {
			lastErr = err
			log.Error().Err(err).Msg("Failed to sync symlink libraries")
		}
	}

	// Calculate scheduled deletions and dry-run preview
	scheduledCount, wouldDelete := e.CalculateDeletionInfo()

	// Execute deletions if enabled and not in dry-run mode
	deletedCount := 0
	deletedItems := make([]map[string]interface{}, 0)
	if e.config.App.EnableDeletion && !e.config.App.DryRun && len(wouldDelete) > 0 {
		deletedCount, deletedItems = e.ExecuteDeletions(ctx, wouldDelete)
	}

	// Update job
	completedAt := time.Now()
	duration := completedAt.Sub(startTime)

	job.CompletedAt = &completedAt
	job.DurationMs = duration.Milliseconds()
	job.Summary["movies"] = movieCount
	job.Summary["tv_shows"] = tvShowCount
	job.Summary["total_media"] = e.GetMediaCount()
	job.Summary["scheduled_deletions"] = scheduledCount
	job.Summary["leaving_soon_count"] = leavingSoonCount
	job.Summary["dry_run"] = e.config.App.DryRun
	job.Summary["enable_deletion"] = e.config.App.EnableDeletion

	// Always add deletion candidates to job summary for UI display
	// In dry-run mode, these are candidates that would be deleted
	// Otherwise, these are candidates that will be deleted (if enable_deletion is true)
	if len(wouldDelete) > 0 {
		job.Summary["would_delete"] = wouldDelete
	}

	// Add actual deletions when executed
	if deletedCount > 0 {
		job.Summary["deleted_count"] = deletedCount
		job.Summary["deleted_items"] = deletedItems
	}

	if lastErr != nil {
		job.Status = storage.JobStatusFailed
		job.Error = lastErr.Error()
	} else {
		job.Status = storage.JobStatusCompleted
	}

	if err := e.jobs.Update(job); err != nil {
		log.Warn().Err(err).Msg("Failed to update job")
	}

	// Clear cache after full sync
	e.cache.Clear()

	log.Info().
		Str("job_id", jobID).
		Int("movies", movieCount).
		Int("tv_shows", tvShowCount).
		Int("scheduled_deletions", scheduledCount).
		Int("deleted_count", deletedCount).
		Bool("dry_run", e.config.App.DryRun).
		Bool("enable_deletion", e.config.App.EnableDeletion).
		Dur("duration", duration).
		Msg("Full sync completed")

	return lastErr
}

// IncrementalSync performs a quick update of watch history
func (e *SyncEngine) IncrementalSync(ctx context.Context) error {
	jobID := uuid.New().String()
	startTime := time.Now()

	log.Debug().Str("job_id", jobID).Msg("Starting incremental sync")

	// Just update watch data from Jellyfin
	if e.jellyfinClient != nil {
		if err := e.syncJellyfin(ctx); err != nil {
			return fmt.Errorf("failed to sync Jellyfin: %w", err)
		}
	}

	duration := time.Since(startTime)
	log.Debug().
		Str("job_id", jobID).
		Dur("duration", duration).
		Msg("Incremental sync completed")

	return nil
}

// syncRadarr syncs movies from Radarr
func (e *SyncEngine) syncRadarr(ctx context.Context) ([]models.Media, error) {
	radarrMovies, err := e.radarrClient.GetMovies(ctx)
	if err != nil {
		return nil, err
	}

	// Fetch all tags to convert tag IDs to names
	radarrTags, err := e.radarrClient.GetTags(ctx)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to fetch Radarr tags, continuing without tags")
		radarrTags = []clients.RadarrTag{} // Continue without tags on error
	}

	// Build tag ID to name map
	tagMap := make(map[int]string, len(radarrTags))
	for _, tag := range radarrTags {
		tagMap[tag.ID] = tag.Label
	}

	mediaItems := make([]models.Media, 0, len(radarrMovies))

	e.mediaLibraryLock.Lock()
	defer e.mediaLibraryLock.Unlock()

	for _, rm := range radarrMovies {
		if !rm.HasFile {
			continue
		}

		mediaID := fmt.Sprintf("radarr-%d", rm.ID)
		media := models.Media{
			ID:       mediaID,
			Type:     models.MediaTypeMovie,
			Title:    rm.Title,
			Year:     rm.Year,
			AddedAt:  rm.Added,
			FilePath: rm.Path, // Default to directory path
			FileSize: rm.SizeOnDisk,
			RadarrID: rm.ID,
			TMDBID:   rm.TmdbId,
		}

		if rm.MovieFile != nil {
			media.QualityTag = rm.MovieFile.Quality.Quality.Name
			// Use actual file path if available
			if rm.MovieFile.Path != "" {
				media.FilePath = rm.MovieFile.Path
			}
		}

		// Convert tag IDs to tag names
		if len(rm.Tags) > 0 {
			media.Tags = make([]string, 0, len(rm.Tags))
			for _, tagID := range rm.Tags {
				if tagName, ok := tagMap[tagID]; ok {
					media.Tags = append(media.Tags, tagName)
				}
			}
		}

		e.mediaLibrary[mediaID] = media
		mediaItems = append(mediaItems, media)
	}

	return mediaItems, nil
}

// syncSonarr syncs TV shows from Sonarr
func (e *SyncEngine) syncSonarr(ctx context.Context) ([]models.Media, error) {
	sonarrSeries, err := e.sonarrClient.GetSeries(ctx)
	if err != nil {
		return nil, err
	}

	// Fetch all tags to convert tag IDs to names
	sonarrTags, err := e.sonarrClient.GetTags(ctx)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to fetch Sonarr tags, continuing without tags")
		sonarrTags = []clients.SonarrTag{} // Continue without tags on error
	}

	// Build tag ID to name map
	tagMap := make(map[int]string, len(sonarrTags))
	for _, tag := range sonarrTags {
		tagMap[tag.ID] = tag.Label
	}

	mediaItems := make([]models.Media, 0, len(sonarrSeries))

	e.mediaLibraryLock.Lock()
	defer e.mediaLibraryLock.Unlock()

	for _, ss := range sonarrSeries {
		if ss.Statistics.EpisodeFileCount == 0 {
			continue
		}

		mediaID := fmt.Sprintf("sonarr-%d", ss.ID)
		media := models.Media{
			ID:       mediaID,
			Type:     models.MediaTypeTVShow,
			Title:    ss.Title,
			Year:     ss.Year,
			AddedAt:  ss.Added,
			FilePath: ss.Path,
			FileSize: ss.Statistics.SizeOnDisk,
			SonarrID: ss.ID,
			TVDBID:   ss.TvdbId,
		}

		// Convert tag IDs to tag names
		if len(ss.Tags) > 0 {
			media.Tags = make([]string, 0, len(ss.Tags))
			for _, tagID := range ss.Tags {
				if tagName, ok := tagMap[tagID]; ok {
					media.Tags = append(media.Tags, tagName)
				}
			}
		}

		e.mediaLibrary[mediaID] = media
		mediaItems = append(mediaItems, media)
	}

	return mediaItems, nil
}

// syncJellyfin syncs watch data from Jellyfin
func (e *SyncEngine) syncJellyfin(ctx context.Context) error {
	// Get movies
	jellyfinMovies, err := e.jellyfinClient.GetMovies(ctx)
	if err != nil {
		return fmt.Errorf("fetching movies: %w", err)
	}

	e.mediaLibraryLock.Lock()
	defer e.mediaLibraryLock.Unlock()

	// Track matching statistics
	movieMatched := 0
	movieNotFound := 0
	movieMismatch := 0
	showMatched := 0
	showNotFound := 0
	showMismatch := 0

	// Build a map of Jellyfin movies by TMDB ID for quick lookup
	jellyfinMoviesByTMDB := make(map[string]*clients.JellyfinItem)
	jellyfinMoviesByTitle := make(map[string]*clients.JellyfinItem)
	for i := range jellyfinMovies {
		jm := &jellyfinMovies[i]
		if tmdbID := jm.ProviderIds["Tmdb"]; tmdbID != "" {
			jellyfinMoviesByTMDB[tmdbID] = jm
		}
		// Normalize title for fuzzy matching (lowercase, trim spaces)
		normalizedTitle := strings.ToLower(strings.TrimSpace(jm.Name))
		jellyfinMoviesByTitle[normalizedTitle] = jm
	}

	// Update watch data for movies and track mismatches
	for id, media := range e.mediaLibrary {
		if media.Type != models.MediaTypeMovie {
			continue
		}

		tmdbIDStr := strconv.Itoa(media.TMDBID)
		if jm, found := jellyfinMoviesByTMDB[tmdbIDStr]; found {
			// Exact match found
			media.JellyfinID = jm.ID
			media.WatchCount = jm.UserData.PlayCount
			if !jm.UserData.LastPlayedDate.IsZero() {
				media.LastWatched = jm.UserData.LastPlayedDate
			}
			media.JellyfinMatchStatus = "matched"
			media.JellyfinMismatchInfo = ""
			movieMatched++
		} else {
			// No exact match - check for potential metadata mismatch
			normalizedTitle := strings.ToLower(strings.TrimSpace(media.Title))
			if jm, found := jellyfinMoviesByTitle[normalizedTitle]; found {
				// Same title but different TMDB ID - metadata mismatch
				jellyfinTMDB := jm.ProviderIds["Tmdb"]
				media.JellyfinMatchStatus = "metadata_mismatch"
				media.JellyfinMismatchInfo = fmt.Sprintf("Jellyfin has wrong metadata (TMDB %s instead of %d)", jellyfinTMDB, media.TMDBID)
				movieMismatch++
				log.Warn().
					Str("title", media.Title).
					Int("radarr_tmdb_id", media.TMDBID).
					Str("jellyfin_tmdb_id", jellyfinTMDB).
					Msg("Metadata mismatch detected for movie")
			} else {
				// Not found in Jellyfin at all
				media.JellyfinMatchStatus = "not_found"
				media.JellyfinMismatchInfo = "Item not found in Jellyfin library"
				movieNotFound++
			}
		}
		e.mediaLibrary[id] = media
	}

	// Get TV shows
	jellyfinShows, err := e.jellyfinClient.GetTVShows(ctx)
	if err != nil {
		return fmt.Errorf("fetching TV shows: %w", err)
	}

	// Build a map of Jellyfin TV shows by TVDB ID for quick lookup
	jellyfinShowsByTVDB := make(map[string]*clients.JellyfinItem)
	jellyfinShowsByTitle := make(map[string]*clients.JellyfinItem)
	for i := range jellyfinShows {
		js := &jellyfinShows[i]
		if tvdbID := js.ProviderIds["Tvdb"]; tvdbID != "" {
			jellyfinShowsByTVDB[tvdbID] = js
		}
		// Normalize title for fuzzy matching
		normalizedTitle := strings.ToLower(strings.TrimSpace(js.Name))
		jellyfinShowsByTitle[normalizedTitle] = js
	}

	// Update watch data for TV shows and track mismatches
	for id, media := range e.mediaLibrary {
		if media.Type != models.MediaTypeTVShow {
			continue
		}

		tvdbIDStr := strconv.Itoa(media.TVDBID)
		if js, found := jellyfinShowsByTVDB[tvdbIDStr]; found {
			// Exact match found
			media.JellyfinID = js.ID
			media.WatchCount = js.UserData.PlayCount
			if !js.UserData.LastPlayedDate.IsZero() {
				media.LastWatched = js.UserData.LastPlayedDate
			}
			media.JellyfinMatchStatus = "matched"
			media.JellyfinMismatchInfo = ""
			showMatched++
		} else {
			// No exact match - check for potential metadata mismatch
			normalizedTitle := strings.ToLower(strings.TrimSpace(media.Title))
			if js, found := jellyfinShowsByTitle[normalizedTitle]; found {
				// Same title but different TVDB ID - metadata mismatch
				jellyfinTVDB := js.ProviderIds["Tvdb"]
				media.JellyfinMatchStatus = "metadata_mismatch"
				media.JellyfinMismatchInfo = fmt.Sprintf("Jellyfin has wrong metadata (TVDB %s instead of %d)", jellyfinTVDB, media.TVDBID)
				showMismatch++
				log.Warn().
					Str("title", media.Title).
					Int("sonarr_tvdb_id", media.TVDBID).
					Str("jellyfin_tvdb_id", jellyfinTVDB).
					Msg("Metadata mismatch detected for TV show")
			} else {
				// Not found in Jellyfin at all
				media.JellyfinMatchStatus = "not_found"
				media.JellyfinMismatchInfo = "Item not found in Jellyfin library"
				showNotFound++
			}
		}
		e.mediaLibrary[id] = media
	}

	// Log summary of Jellyfin matching results
	totalMovies := movieMatched + movieNotFound + movieMismatch
	totalShows := showMatched + showNotFound + showMismatch
	log.Info().
		Int("movie_matched", movieMatched).
		Int("movie_not_found", movieNotFound).
		Int("movie_mismatch", movieMismatch).
		Int("movie_total", totalMovies).
		Int("show_matched", showMatched).
		Int("show_not_found", showNotFound).
		Int("show_mismatch", showMismatch).
		Int("show_total", totalShows).
		Msg("Jellyfin sync completed")

	// Log warnings if there are mismatches or missing items
	if movieMismatch > 0 || showMismatch > 0 {
		log.Warn().
			Int("movies", movieMismatch).
			Int("shows", showMismatch).
			Msg("Metadata mismatches detected - items exist in Jellyfin but with incorrect TMDB/TVDB IDs")
	}
	if movieNotFound > 0 || showNotFound > 0 {
		log.Warn().
			Int("movies", movieNotFound).
			Int("shows", showNotFound).
			Msg("Items not found in Jellyfin - may not be imported yet or different library paths")
	}

	return nil
}

// syncJellyseerr syncs requested items from Jellyseerr
func (e *SyncEngine) syncJellyseerr(ctx context.Context) error {
	requests, err := e.jellyseerrClient.GetRequests(ctx)
	if err != nil {
		return err
	}

	e.mediaLibraryLock.Lock()
	defer e.mediaLibraryLock.Unlock()

	// Mark requested items
	for _, req := range requests {
		// Status 2 = approved, 5 = available (approved + downloaded)
		// We want both because status 5 means the request is fulfilled and in the library
		if req.Status != 2 && req.Status != 5 {
			continue
		}

		// Find matching media by TMDB/TVDB ID
		for id, media := range e.mediaLibrary {
			matched := false
			if media.Type == models.MediaTypeMovie && media.TMDBID == req.Media.TmdbId {
				matched = true
			} else if media.Type == models.MediaTypeTVShow && media.TVDBID == req.Media.TvdbId {
				matched = true
			}

			if matched {
				media.IsRequested = true
				// Populate requester user information
				if req.RequestedBy.ID > 0 {
					media.RequestedByUserID = &req.RequestedBy.ID
				}
				// Use DisplayName first, fallback to JellyfinUsername, then Email
				username := req.RequestedBy.DisplayName
				if username == "" {
					username = req.RequestedBy.JellyfinUsername
				}
				if username == "" {
					username = req.RequestedBy.Username
				}
				if username != "" {
					media.RequestedByUsername = &username
				}
				if req.RequestedBy.Email != "" {
					media.RequestedByEmail = &req.RequestedBy.Email
				}

				log.Debug().
					Str("media_title", media.Title).
					Str("display_name", req.RequestedBy.DisplayName).
					Str("jellyfin_username", req.RequestedBy.JellyfinUsername).
					Str("username", req.RequestedBy.Username).
					Str("resolved_username", username).
					Str("email", req.RequestedBy.Email).
					Msg("Matched Jellyseerr request to media")

				e.mediaLibrary[id] = media
				break
			}
		}
	}

	log.Info().
		Int("total_requests", len(requests)).
		Msg("Jellyseerr sync completed")

	return nil
}

// syncJellystat syncs detailed watch history from Jellystat
func (e *SyncEngine) syncJellystat(ctx context.Context) error {
	history, err := e.jellystatClient.GetHistory(ctx)
	if err != nil {
		return err
	}

	e.mediaLibraryLock.Lock()
	defer e.mediaLibraryLock.Unlock()

	// Create a map of Jellyfin ID to most recent watch date and watch count
	// This gives us the most accurate "last watched" timestamp per item
	// Jellystat is the authoritative source for watch history
	lastWatchedMap := make(map[string]time.Time)
	watchCountMap := make(map[string]int)

	for _, item := range history {
		jellyfinID := item.NowPlayingItemID

		// Track the most recent watch for each Jellyfin item
		if existing, found := lastWatchedMap[jellyfinID]; !found || item.ActivityDateInserted.After(existing) {
			lastWatchedMap[jellyfinID] = item.ActivityDateInserted
		}

		// Count how many times this item appears in history
		watchCountMap[jellyfinID]++
	}

	// Update media library with accurate watch data from Jellystat
	// Jellystat overrides Jellyfin's watch count as the authoritative source
	updatedCount := 0
	for id, media := range e.mediaLibrary {
		if media.JellyfinID == "" {
			continue
		}

		if lastWatched, found := lastWatchedMap[media.JellyfinID]; found {
			// Update both LastWatched and WatchCount from Jellystat
			// This makes Jellystat the single source of truth for watch history
			updated := false

			if media.LastWatched.IsZero() || lastWatched.After(media.LastWatched) {
				media.LastWatched = lastWatched
				updated = true
			}

			// Always set WatchCount from Jellystat to override Jellyfin's potentially incorrect value
			if watchCount := watchCountMap[media.JellyfinID]; watchCount > 0 {
				media.WatchCount = watchCount
				updated = true
			}

			if updated {
				e.mediaLibrary[id] = media
				updatedCount++
			}
		}
	}

	log.Info().
		Int("total_history_items", len(history)).
		Int("updated_media", updatedCount).
		Msg("Jellystat sync completed")

	return nil
}

// GetMediaList returns all synced media items
func (e *SyncEngine) GetMediaList() []models.Media {
	e.mediaLibraryLock.RLock()
	defer e.mediaLibraryLock.RUnlock()

	items := make([]models.Media, 0, len(e.mediaLibrary))
	for _, media := range e.mediaLibrary {
		items = append(items, media)
	}

	return items
}

// GetMediaByID returns a specific media item
func (e *SyncEngine) GetMediaByID(id string) (models.Media, bool) {
	e.mediaLibraryLock.RLock()
	defer e.mediaLibraryLock.RUnlock()

	media, found := e.mediaLibrary[id]
	return media, found
}

// GetMediaCount returns the total number of synced media items
func (e *SyncEngine) GetMediaCount() int {
	e.mediaLibraryLock.RLock()
	defer e.mediaLibraryLock.RUnlock()

	return len(e.mediaLibrary)
}

// applyRetentionRules evaluates retention rules for all media items
func (e *SyncEngine) applyRetentionRules() {
	e.mediaLibraryLock.Lock()
	defer e.mediaLibraryLock.Unlock()

	for id, media := range e.mediaLibrary {
		_, deleteAfter, reason := e.rules.EvaluateMedia(&media)

		// Update media with deletion date
		media.DeleteAfter = deleteAfter
		if !deleteAfter.IsZero() {
			daysUntilDue := int(time.Until(deleteAfter).Hours() / 24)
			media.DaysUntilDue = daysUntilDue

			// Set deletion reason for all items with deletion dates (both future and overdue)
			media.DeletionReason = e.rules.GenerateDeletionReason(&media, deleteAfter, reason)
		} else {
			// Clear deletion info when no deletion is scheduled
			media.DaysUntilDue = 0
			media.DeletionReason = ""
		}

		e.mediaLibrary[id] = media
	}

	log.Debug().Int("media_count", len(e.mediaLibrary)).Msg("Applied retention rules to media")
}

// ReapplyRetentionRules re-evaluates retention rules for all media items
// This is useful after config changes to update deletion dates without a full sync
func (e *SyncEngine) ReapplyRetentionRules() {
	log.Info().Msg("Reapplying retention rules after config change")
	e.applyRetentionRules()
	log.Info().Msg("Retention rules reapplied successfully")
}

// applyExclusions applies exclusions from the exclusions file to all media items
func (e *SyncEngine) applyExclusions() {
	e.mediaLibraryLock.Lock()
	defer e.mediaLibraryLock.Unlock()

	excludedCount := 0
	for id, media := range e.mediaLibrary {
		// Check if this media ID is in the exclusions list
		isExcluded := e.exclusions.IsExcluded(id)

		// Update the media's exclusion status
		if media.IsExcluded != isExcluded {
			media.IsExcluded = isExcluded
			e.mediaLibrary[id] = media
			if isExcluded {
				excludedCount++
			}
		}
	}

	log.Debug().
		Int("media_count", len(e.mediaLibrary)).
		Int("excluded_count", excludedCount).
		Msg("Applied exclusions to media")
}

// CalculateDeletionInfo calculates scheduled deletions and returns dry-run preview
func (e *SyncEngine) CalculateDeletionInfo() (int, []map[string]interface{}) {
	e.mediaLibraryLock.RLock()
	defer e.mediaLibraryLock.RUnlock()

	scheduledCount := 0
	wouldDelete := make([]map[string]interface{}, 0)
	now := time.Now()

	for _, media := range e.mediaLibrary {
		// Skip excluded items
		if media.IsExcluded {
			continue
		}

		// Check if deletion date has passed
		if !media.DeleteAfter.IsZero() && now.After(media.DeleteAfter) {
			scheduledCount++
			daysOverdue := int(now.Sub(media.DeleteAfter).Hours() / 24)
			candidate := map[string]interface{}{
				"id":           media.ID,
				"title":        media.Title,
				"year":         media.Year,
				"type":         media.Type,
				"file_size":    media.FileSize,
				"delete_after": media.DeleteAfter,
				"days_overdue": daysOverdue,
				"reason":       media.DeletionReason,
				"last_watched": media.LastWatched,
				// Requester information
				"is_requested":          media.IsRequested,
				"requested_by_user_id":  media.RequestedByUserID,
				"requested_by_username": media.RequestedByUsername,
				"requested_by_email":    media.RequestedByEmail,
			}
			wouldDelete = append(wouldDelete, candidate)
		}
	}

	return scheduledCount, wouldDelete
}

// ExecuteDeletions performs actual deletion of overdue media items
func (e *SyncEngine) ExecuteDeletions(ctx context.Context, candidates []map[string]interface{}) (int, []map[string]interface{}) {
	deletedCount := 0
	deletedItems := make([]map[string]interface{}, 0)

	log.Info().
		Int("candidates", len(candidates)).
		Msg("Executing deletions for overdue items")

	for _, candidate := range candidates {
		mediaID, ok := candidate["id"].(string)
		if !ok {
			log.Warn().Interface("candidate", candidate).Msg("Invalid media ID in deletion candidate")
			continue
		}

		// Attempt deletion
		if err := e.DeleteMedia(ctx, mediaID, false); err != nil {
			log.Error().
				Err(err).
				Str("media_id", mediaID).
				Str("title", candidate["title"].(string)).
				Msg("Failed to delete media")
			continue
		}

		// Track successful deletion
		deletedCount++
		deletedItems = append(deletedItems, candidate)

		log.Info().
			Str("media_id", mediaID).
			Str("title", candidate["title"].(string)).
			Msg("Successfully deleted media")
	}

	log.Info().
		Int("deleted", deletedCount).
		Int("failed", len(candidates)-deletedCount).
		Msg("Deletion execution completed")

	return deletedCount, deletedItems
}

// GetMediaLibrary returns the internal media library map (for testing purposes)
func (e *SyncEngine) GetMediaLibrary() map[string]models.Media {
	e.mediaLibraryLock.RLock()
	defer e.mediaLibraryLock.RUnlock()

	return e.mediaLibrary
}

// DeleteMedia performs actual deletion of media
func (e *SyncEngine) DeleteMedia(ctx context.Context, mediaID string, dryRun bool) error {
	media, found := e.GetMediaByID(mediaID)
	if !found {
		return fmt.Errorf("media not found: %s", mediaID)
	}

	if dryRun {
		log.Info().
			Str("media_id", mediaID).
			Str("title", media.Title).
			Msg("DRY RUN: Would delete media")
		return nil
	}

	// Step 1: Delete from Radarr/Sonarr (which also deletes the actual files)
	deletedFromService := false
	if media.RadarrID > 0 && e.radarrClient != nil {
		if err := e.radarrClient.DeleteMovie(ctx, media.RadarrID, true); err != nil {
			return fmt.Errorf("deleting from Radarr: %w", err)
		}
		deletedFromService = true
		log.Info().
			Str("media_id", mediaID).
			Str("title", media.Title).
			Int("radarr_id", media.RadarrID).
			Msg("Deleted movie from Radarr")
	}

	if media.SonarrID > 0 && e.sonarrClient != nil {
		if err := e.sonarrClient.DeleteSeries(ctx, media.SonarrID, true); err != nil {
			return fmt.Errorf("deleting from Sonarr: %w", err)
		}
		deletedFromService = true
		log.Info().
			Str("media_id", mediaID).
			Str("title", media.Title).
			Int("sonarr_id", media.SonarrID).
			Msg("Deleted series from Sonarr")
	}

	// Step 2: Trigger Jellyfin library refresh to detect file removal
	// NOTE: We do NOT call jellyfinClient.DeleteItem() because Jellyfin should
	// automatically detect the file is gone when we scan the library.
	// Radarr/Sonarr are responsible for file deletion.
	if deletedFromService && e.jellyfinClient != nil {
		if err := e.jellyfinClient.RefreshLibrary(ctx, false); err != nil {
			log.Warn().
				Err(err).
				Str("media_id", mediaID).
				Str("title", media.Title).
				Msg("Failed to trigger Jellyfin library refresh after deletion (non-fatal)")
			// Don't return error - the files are deleted, Jellyfin will catch up eventually
		} else {
			log.Info().
				Str("media_id", mediaID).
				Str("title", media.Title).
				Msg("Triggered Jellyfin library refresh after deletion")
		}
	}

	// Step 3: Remove from internal media library
	e.mediaLibraryLock.Lock()
	delete(e.mediaLibrary, mediaID)
	e.mediaLibraryLock.Unlock()

	log.Info().
		Str("media_id", mediaID).
		Str("title", media.Title).
		Msg("Media deleted successfully")

	return nil
}

// AddExclusion adds a media item to the exclusion list
func (e *SyncEngine) AddExclusion(ctx context.Context, mediaID, reason string) error {
	media, found := e.GetMediaByID(mediaID)
	if !found {
		return fmt.Errorf("media not found: %s", mediaID)
	}

	// Determine external ID
	externalID := mediaID
	externalType := "unknown"

	if media.RadarrID > 0 {
		externalID = fmt.Sprintf("radarr-%d", media.RadarrID)
		externalType = "radarr"
	} else if media.SonarrID > 0 {
		externalID = fmt.Sprintf("sonarr-%d", media.SonarrID)
		externalType = "sonarr"
	}

	exclusion := storage.ExclusionItem{
		ExternalID:   externalID,
		ExternalType: externalType,
		MediaType:    string(media.Type),
		Title:        media.Title,
		ExcludedAt:   time.Now(),
		ExcludedBy:   "api",
		Reason:       reason,
	}

	if err := e.exclusions.Add(exclusion); err != nil {
		return fmt.Errorf("adding exclusion: %w", err)
	}

	// Update media library
	e.mediaLibraryLock.Lock()
	media.IsExcluded = true
	e.mediaLibrary[mediaID] = media
	e.mediaLibraryLock.Unlock()

	log.Info().
		Str("media_id", mediaID).
		Str("title", media.Title).
		Str("reason", reason).
		Msg("Media excluded from deletion")

	return nil
}

// RemoveExclusion removes a media item from the exclusion list
func (e *SyncEngine) RemoveExclusion(ctx context.Context, mediaID string) error {
	media, found := e.GetMediaByID(mediaID)
	if !found {
		return fmt.Errorf("media not found: %s", mediaID)
	}

	// Determine external ID
	externalID := mediaID
	if media.RadarrID > 0 {
		externalID = fmt.Sprintf("radarr-%d", media.RadarrID)
	} else if media.SonarrID > 0 {
		externalID = fmt.Sprintf("sonarr-%d", media.SonarrID)
	}

	if err := e.exclusions.Remove(externalID); err != nil {
		return fmt.Errorf("removing exclusion: %w", err)
	}

	// Update media library
	e.mediaLibraryLock.Lock()
	media.IsExcluded = false
	e.mediaLibrary[mediaID] = media
	e.mediaLibraryLock.Unlock()

	log.Info().
		Str("media_id", mediaID).
		Str("title", media.Title).
		Msg("Media exclusion removed")

	return nil
}

// SyncStatus represents the current sync engine status
type SyncStatus struct {
	Running       bool      `json:"running"`
	MediaCount    int       `json:"media_count"`
	LastFullSync  time.Time `json:"last_full_sync,omitempty"`
	LastIncrSync  time.Time `json:"last_incr_sync,omitempty"`
	FullInterval  int       `json:"full_interval_minutes"`
	IncrInterval  int       `json:"incr_interval_minutes"`
	MoviesCount   int       `json:"movies_count"`
	TVShowsCount  int       `json:"tv_shows_count"`
	ExcludedCount int       `json:"excluded_count"`
}

// GetStatus returns the current sync engine status
func (e *SyncEngine) GetStatus() SyncStatus {
	e.runningLock.Lock()
	running := e.running
	e.runningLock.Unlock()

	e.mediaLibraryLock.RLock()
	defer e.mediaLibraryLock.RUnlock()

	status := SyncStatus{
		Running:      running,
		MediaCount:   len(e.mediaLibrary),
		FullInterval: e.config.Sync.FullInterval,
		IncrInterval: e.config.Sync.IncrementalInterval,
	}

	// Count movies, TV shows, and excluded items
	for _, media := range e.mediaLibrary {
		if media.Type == models.MediaTypeMovie {
			status.MoviesCount++
		} else if media.Type == models.MediaTypeTVShow {
			status.TVShowsCount++
		}
		if media.IsExcluded {
			status.ExcludedCount++
		}
	}

	// Get last sync times from jobs
	// Note: This is a simple implementation; you might want to cache these values
	jobs := e.jobs.GetRecent(10)
	for _, job := range jobs {
		if job.Type == "full_sync" && job.CompletedAt != nil {
			if status.LastFullSync.IsZero() || job.CompletedAt.After(status.LastFullSync) {
				status.LastFullSync = *job.CompletedAt
			}
		} else if job.Type == "incremental_sync" && job.CompletedAt != nil {
			if status.LastIncrSync.IsZero() || job.CompletedAt.After(status.LastIncrSync) {
				status.LastIncrSync = *job.CompletedAt
			}
		}
	}

	return status
}
