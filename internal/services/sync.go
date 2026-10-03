package services

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/ramonskie/oxicleanarr/internal/cache"
	"github.com/ramonskie/oxicleanarr/internal/clients"
	"github.com/ramonskie/oxicleanarr/internal/config"
	"github.com/ramonskie/oxicleanarr/internal/models"
	"github.com/ramonskie/oxicleanarr/internal/services/rules"
	"github.com/ramonskie/oxicleanarr/internal/storage"
	"github.com/rs/zerolog/log"
)

// SyncEngine handles media synchronization and cleanup operations
type SyncEngine struct {
	config            *config.Config
	cache             *cache.Cache
	jobs              *storage.JobsFile
	exclusions        *storage.ExclusionsFile
	manualLeavingSoon *storage.ManualLeavingSoonFile
	rules             *rules.RulesEngine
	diskMonitor       *DiskMonitor

	jellyfinClient   *clients.JellyfinClient
	radarrClient     *clients.RadarrClient
	sonarrClient     *clients.SonarrClient
	jellyseerrClient *clients.JellyseerrClient
	statsClient      clients.StatsProvider

	mediaLibrary     map[string]models.Media
	mediaLibraryLock sync.RWMutex

	fullSyncTicker *time.Ticker
	incrSyncTicker *time.Ticker
	stopChan       chan struct{}
	running        bool
	runningLock    sync.Mutex

	// syncRunMu serializes manual/scheduled sync invocations so a full and an
	// incremental sync (or two fulls) can't overlap and race the media library.
	syncRunMu sync.Mutex
}

// NewSyncEngine creates a new sync engine
func NewSyncEngine(
	cfg *config.Config,
	cacheInstance *cache.Cache,
	jobs *storage.JobsFile,
	exclusions *storage.ExclusionsFile,
	manualLeavingSoon *storage.ManualLeavingSoonFile,
	rulesEngine *rules.RulesEngine,
) *SyncEngine {
	engine := &SyncEngine{
		config:            cfg,
		cache:             cacheInstance,
		jobs:              jobs,
		exclusions:        exclusions,
		manualLeavingSoon: manualLeavingSoon,
		rules:             rulesEngine,
		mediaLibrary:      make(map[string]models.Media),
		stopChan:          make(chan struct{}),
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
		// Inject Sonarr client into rules engine so episode rules can make API calls.
		rulesEngine.SetSonarrClient(engine.sonarrClient)
	}
	if cfg.Integrations.Jellyseerr.Enabled {
		engine.jellyseerrClient = clients.NewJellyseerrClient(cfg.Integrations.Jellyseerr)
	}
	if cfg.Integrations.Jellystat.Enabled {
		engine.statsClient = clients.NewJellystatClient(cfg.Integrations.Jellystat)
	}
	if cfg.Integrations.Streamystats.Enabled {
		engine.statsClient = clients.NewStreamystatsClient(cfg.Integrations.Streamystats)
	}
	if cfg.Integrations.Tracearr.Enabled {
		engine.statsClient = clients.NewTracearrClient(cfg.Integrations.Tracearr)
	}

	// Initialize disk monitor if disk threshold feature is enabled.
	// Inject it into the rules engine so that Evaluate() can gate on real disk status.
	if cfg.App.DiskThreshold.Enabled {
		engine.diskMonitor = NewDiskMonitor(engine.radarrClient, engine.sonarrClient)
		rulesEngine.SetDiskMonitor(engine.diskMonitor)
		log.Info().Msg("Disk monitor initialized")
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
		goRecover(func() {
			ctx := context.Background()
			if err := e.FullSync(ctx); err != nil {
				log.Error().Err(err).Msg("Initial full sync failed")
			}
		})

		// Start ticker goroutines
		goRecover(e.runFullSyncLoop)
		goRecover(e.runIncrementalSyncLoop)

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
			runSyncSafe("full", func() error {
				ctx := context.Background()
				return e.FullSync(ctx)
			})
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
			runSyncSafe("incremental", func() error {
				ctx := context.Background()
				return e.IncrementalSync(ctx)
			})
		case <-e.stopChan:
			return
		}
	}
}

// acquireSyncRunLock serializes sync runs on syncRunMu. If another sync holds
// the lock, it waits (the scheduler and manual triggers queue behind a running
// sync) and logs once so the queued wait is visible in the server log.
func (e *SyncEngine) acquireSyncRunLock() {
	if e.syncRunMu.TryLock() {
		return
	}
	log.Info().Msg("A sync is already running; this sync is queued and will start when it finishes")
	e.syncRunMu.Lock()
}

// FullSync performs a complete sync of all media
func (e *SyncEngine) FullSync(ctx context.Context) error {
	// Only one sync run at a time: a manual full sync while a scheduled one is
	// running would otherwise race the media library and rules evaluation.
	e.acquireSyncRunLock()
	defer e.syncRunMu.Unlock()

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
	var syncErrs []error

	// Sync movies from Radarr
	if e.radarrClient != nil {
		movies, err := e.syncRadarr(ctx)
		if err != nil {
			syncErrs = append(syncErrs, err)
			log.Error().Err(err).Msg("Failed to sync Radarr")
		} else {
			movieCount = len(movies)
		}
	}

	// Sync TV shows from Sonarr
	if e.sonarrClient != nil {
		shows, err := e.syncSonarr(ctx)
		if err != nil {
			syncErrs = append(syncErrs, err)
			log.Error().Err(err).Msg("Failed to sync Sonarr")
		} else {
			tvShowCount = len(shows)
		}
	}

	// Sync Jellyfin watch data
	if e.jellyfinClient != nil {
		if err := e.syncJellyfin(ctx); err != nil {
			syncErrs = append(syncErrs, err)
			log.Error().Err(err).Msg("Failed to sync Jellyfin")
		}
	}

	// Sync detailed watch history from the active stats provider (Jellystat, Streamystats, or Tracearr)
	if e.statsClient != nil {
		if err := e.syncStats(ctx); err != nil {
			syncErrs = append(syncErrs, err)
			log.Error().Err(err).Msg("Failed to sync stats provider")
		}
	}

	// Sync requested items from Jellyseerr
	if e.jellyseerrClient != nil {
		if err := e.syncJellyseerr(ctx); err != nil {
			syncErrs = append(syncErrs, err)
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

	// Update disk status before applying retention rules (non-fatal on failure)
	if e.diskMonitor != nil {
		if err := e.diskMonitor.Update(ctx); err != nil {
			log.Warn().Err(err).Msg("Failed to update disk status, rules will use last known state")
		}
	}

	// Apply retention rules to all media
	e.applyRetentionRules(ctx)

	// Apply manual leaving soon overrides (fixed DeleteAfter, set at flag time)
	e.applyManualLeavingSoon()

	// Count items in the leaving-soon window for the job summary (used by the UI).
	// Matches what the UI's leaving-soon page shows (GET /api/media/leaving-soon/list):
	// in-window and not excluded. Unlike the plugin contract, the UI list does not
	// require a Jellyfin id, so the count intentionally does not either.
	leavingSoonCount := 0
	{
		cfg := config.Get()
		e.mediaLibraryLock.RLock()
		for _, media := range e.mediaLibrary {
			if !media.IsExcluded && media.DaysUntilDue > 0 && media.DaysUntilDue <= cfg.App.LeavingSoonDays {
				leavingSoonCount++
			}
		}
		e.mediaLibraryLock.RUnlock()
	}

	// Calculate scheduled deletions and dry-run preview
	scheduledCount, wouldDelete := e.CalculateDeletionInfo()

	// Execute deletions if enabled and not in dry-run mode
	deletedCount := 0
	episodeFilesDeleted := 0
	protectedCount := 0
	failedCount := 0
	deletedItems := make([]map[string]interface{}, 0)
	if e.config.App.EnableDeletion && !e.config.App.DryRun && len(wouldDelete) > 0 {
		deletedCount, _, episodeFilesDeleted, protectedCount, failedCount, deletedItems = e.ExecuteDeletions(ctx, wouldDelete)
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
	if episodeFilesDeleted > 0 {
		job.Summary["episode_files_deleted"] = episodeFilesDeleted
	}
	if failedCount > 0 {
		job.Summary["failed_count"] = failedCount
	}
	if protectedCount > 0 {
		job.Summary["protected_count"] = protectedCount
	}

	if len(syncErrs) > 0 {
		job.Status = storage.JobStatusFailed
		job.Error = errors.Join(syncErrs...).Error()
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

	return errors.Join(syncErrs...)
}

// IncrementalSync performs a quick update of watch history
func (e *SyncEngine) IncrementalSync(ctx context.Context) error {
	// Serialize with full syncs (and other incremental syncs) so they can't
	// overlap and race the media library.
	e.acquireSyncRunLock()
	defer e.syncRunMu.Unlock()

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

	log.Info().
		Int("imported", len(mediaItems)).
		Int("total_from_radarr", len(radarrMovies)).
		Int("skipped_no_file", len(radarrMovies)-len(mediaItems)).
		Msg("Radarr sync completed")

	return mediaItems, nil
}
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

	log.Info().
		Int("imported", len(mediaItems)).
		Int("total_from_sonarr", len(sonarrSeries)).
		Int("skipped_no_episodes", len(sonarrSeries)-len(mediaItems)).
		Msg("Sonarr sync completed")

	return mediaItems, nil
}

// jellyfinNotScannedReason is the cheap, human-readable reason attached to an
// item that has no Jellyfin identity match and no path conflict. It tells the
// user the file is likely not imported/scanned yet rather than mis-identified.
const jellyfinNotScannedReason = "Not present in Jellyfin library (file may not be scanned)"

// normalizeJellyfinPathKey canonicalizes a filesystem path for map comparison:
// backslashes fold to slashes, redundant separators collapse, and case folds.
// An empty path stays empty so it can never collide with a real entry. It uses
// the host-independent path package so Windows-style paths normalise the same
// way on every host.
func normalizeJellyfinPathKey(p string) string {
	if p == "" {
		return ""
	}
	return strings.ToLower(path.Clean(strings.ReplaceAll(p, "\\", "/")))
}

// jellyfinParentPathKey returns the normalized parent directory of a Jellyfin
// item path. It folds Windows separators to POSIX before deriving the directory
// so a Windows-style path on a Linux host still yields a real parent (instead
// of the "." that filepath.Dir returns for an unrecognised separator). Returns
// empty when there is no meaningful parent.
func jellyfinParentPathKey(itemPath string) string {
	dir := path.Dir(strings.ReplaceAll(itemPath, "\\", "/"))
	if dir == "" || dir == "." || dir == "/" {
		return ""
	}
	return normalizeJellyfinPathKey(dir)
}

// buildJellyfinPathIndex maps normalized paths to the Jellyfin item at that
// path. Both the item's full path and its parent directory are indexed: arr
// clients report a media folder (Radarr movie path, Sonarr series path) while
// Jellyfin reports a movie's file path, so matching on the directory bridges
// the two spellings without touching Jellyfin.
//
// A parent directory is indexed only when exactly one item claims it. When two
// items share a folder (e.g. a featurette or sample beside the feature) the
// directory is dropped rather than letting first-wins resolve to the wrong
// item. Exact item paths always take precedence over parent-directory entries.
func buildJellyfinPathIndex(items []clients.JellyfinItem) map[string]*clients.JellyfinItem {
	exact := make(map[string]*clients.JellyfinItem, len(items))
	for i := range items {
		if key := normalizeJellyfinPathKey(items[i].Path); key != "" {
			exact[key] = &items[i]
		}
	}

	parents := make(map[string]*clients.JellyfinItem, len(items))
	ambiguous := make(map[string]struct{})
	for i := range items {
		dir := jellyfinParentPathKey(items[i].Path)
		if dir == "" {
			continue
		}
		if _, collision := parents[dir]; collision {
			ambiguous[dir] = struct{}{}
			continue
		}
		parents[dir] = &items[i]
	}

	index := make(map[string]*clients.JellyfinItem, len(exact)+len(parents))
	for dir, item := range parents {
		if _, skip := ambiguous[dir]; skip {
			continue
		}
		index[dir] = item
	}
	// Exact paths win over a parent-directory entry sharing the same key.
	for key, item := range exact {
		index[key] = item
	}
	return index
}

// lookupJellyfinPath resolves a normalized path against items, giving an exact
// item.Path match precedence over the parent-directory bridge, and returning nil
// when neither resolves.
func lookupJellyfinPath(items []clients.JellyfinItem, normalizedPath string) *clients.JellyfinItem {
	if normalizedPath == "" {
		return nil
	}
	for i := range items {
		if normalizeJellyfinPathKey(items[i].Path) == normalizedPath {
			return &items[i]
		}
	}
	return buildJellyfinPathIndex(items)[normalizedPath]
}

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
	// Path index lets a movie whose Jellyfin metadata was mis-identified (so the
	// TMDB ID does not match) still be recognised as the same file on disk.
	jellyfinMoviesByPath := buildJellyfinPathIndex(jellyfinMovies)

	// Update watch data for movies and track mismatches
	for id, media := range e.mediaLibrary {
		if media.Type != models.MediaTypeMovie {
			continue
		}

		tmdbIDStr := strconv.Itoa(media.TMDBID)
		if jm, found := jellyfinMoviesByTMDB[tmdbIDStr]; found {
			// Exact match found
			media.JellyfinID = jm.ID
			// Jellyfin only returns UserData with a user context; these library
			// queries send no userId, so UserData is absent and PlayCount reads
			// as 0. Only overwrite when Jellyfin actually reports plays, so a
			// sync (notably the incremental one, which never runs the stats
			// provider) cannot clobber stats-provided watch data.
			if jm.UserData.PlayCount > 0 {
				media.WatchCount = jm.UserData.PlayCount
			}
			if !jm.UserData.LastPlayedDate.IsZero() {
				media.LastWatched = jm.UserData.LastPlayedDate
			}
			// Build image URLs (without API key — proxy adds it at request time)
			media.HasPoster = true
			media.JellyfinMatchStatus = "matched"
			media.JellyfinMismatchInfo = ""
			media.JellyfinMatchReason = ""
			media.JellyfinConflictID = ""
			media.JellyfinVerdict = ""
			movieMatched++
		} else {
			// No exact match - check for potential metadata mismatch
			normalizedTitle := strings.ToLower(strings.TrimSpace(media.Title))
			if jm, found := jellyfinMoviesByTitle[normalizedTitle]; found {
				// Same title but different TMDB ID - metadata mismatch. Record the
				// matched item's id so a later fix can re-identify it by id even
				// when arr and Jellyfin library roots differ.
				jellyfinTMDB := jm.ProviderIds["Tmdb"]
				media.JellyfinMatchStatus = "metadata_mismatch"
				media.JellyfinMismatchInfo = fmt.Sprintf("Jellyfin has wrong metadata (TMDB %s instead of %d)", jellyfinTMDB, media.TMDBID)
				media.JellyfinMatchReason = media.JellyfinMismatchInfo
				media.JellyfinConflictID = jm.ID
				media.JellyfinID = ""
				media.HasPoster = false
				media.JellyfinVerdict = ""
				movieMismatch++
				log.Warn().
					Str("title", media.Title).
					Int("radarr_tmdb_id", media.TMDBID).
					Str("jellyfin_tmdb_id", jellyfinTMDB).
					Msg("Metadata mismatch detected for movie")
			} else if jm, found := jellyfinMoviesByPath[normalizeJellyfinPathKey(media.FilePath)]; found {
				// Same file on disk, different identity: Jellyfin mis-identified it.
				// Classification only — never re-identify or attach watch data here.
				jellyfinTMDB := jm.ProviderIds["Tmdb"]
				media.JellyfinMatchStatus = "metadata_mismatch"
				media.JellyfinMismatchInfo = fmt.Sprintf("Jellyfin has wrong metadata (TMDB %s instead of %d)", jellyfinTMDB, media.TMDBID)
				media.JellyfinMatchReason = fmt.Sprintf(
					"Jellyfin identifies the file at this path as %q (TMDB %s), not %q (TMDB %d)",
					jm.Name, jellyfinTMDB, media.Title, media.TMDBID)
				media.JellyfinConflictID = jm.ID
				media.JellyfinID = ""
				media.HasPoster = false
				media.JellyfinVerdict = ""
				movieMismatch++
				log.Warn().
					Str("title", media.Title).
					Int("radarr_tmdb_id", media.TMDBID).
					Str("jellyfin_title", jm.Name).
					Str("jellyfin_tmdb_id", jellyfinTMDB).
					Str("jellyfin_item_id", jm.ID).
					Msg("Path conflict detected for movie")
			} else {
				// Not found in Jellyfin at all
				media.JellyfinMatchStatus = "not_found"
				media.JellyfinMismatchInfo = "Item not found in Jellyfin library"
				media.JellyfinMatchReason = jellyfinNotScannedReason
				media.JellyfinConflictID = ""
				media.JellyfinID = ""
				media.HasPoster = false
				media.JellyfinVerdict = ""
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
	// Path index catches series whose Jellyfin metadata was mis-identified so
	// the TVDB ID no longer matches, even though the files are the same.
	jellyfinShowsByPath := buildJellyfinPathIndex(jellyfinShows)

	// Update watch data for TV shows and track mismatches
	for id, media := range e.mediaLibrary {
		if media.Type != models.MediaTypeTVShow {
			continue
		}

		tvdbIDStr := strconv.Itoa(media.TVDBID)
		if js, found := jellyfinShowsByTVDB[tvdbIDStr]; found {
			// Exact match found
			media.JellyfinID = js.ID
			// Jellyfin series UserData is absent without a user context and its
			// series-level PlayCount is 0 even when episodes are watched. Never
			// zero the stats-provided aggregate here — see the movie branch.
			if js.UserData.PlayCount > 0 {
				media.WatchCount = js.UserData.PlayCount
			}
			if !js.UserData.LastPlayedDate.IsZero() {
				media.LastWatched = js.UserData.LastPlayedDate
			}
			// Flag poster availability (proxy fetches from Jellyfin at request time)
			media.HasPoster = true
			media.JellyfinMatchStatus = "matched"
			media.JellyfinMismatchInfo = ""
			media.JellyfinMatchReason = ""
			media.JellyfinConflictID = ""
			media.JellyfinVerdict = ""
			showMatched++
		} else {
			// No exact match - check for potential metadata mismatch
			normalizedTitle := strings.ToLower(strings.TrimSpace(media.Title))
			if js, found := jellyfinShowsByTitle[normalizedTitle]; found {
				// Same title but different TVDB ID - metadata mismatch. Record the
				// matched item's id so a later fix can re-identify it by id even
				// when arr and Jellyfin library roots differ.
				jellyfinTVDB := js.ProviderIds["Tvdb"]
				media.JellyfinMatchStatus = "metadata_mismatch"
				media.JellyfinMismatchInfo = fmt.Sprintf("Jellyfin has wrong metadata (TVDB %s instead of %d)", jellyfinTVDB, media.TVDBID)
				media.JellyfinMatchReason = media.JellyfinMismatchInfo
				media.JellyfinConflictID = js.ID
				media.JellyfinID = ""
				media.HasPoster = false
				media.JellyfinVerdict = ""
				showMismatch++
				log.Warn().
					Str("title", media.Title).
					Int("sonarr_tvdb_id", media.TVDBID).
					Str("jellyfin_tvdb_id", jellyfinTVDB).
					Msg("Metadata mismatch detected for TV show")
			} else if js, found := jellyfinShowsByPath[normalizeJellyfinPathKey(media.FilePath)]; found {
				// Same files on disk, different identity: Jellyfin mis-identified it.
				// Classification only — never re-identify or attach watch data here.
				jellyfinTVDB := js.ProviderIds["Tvdb"]
				media.JellyfinMatchStatus = "metadata_mismatch"
				media.JellyfinMismatchInfo = fmt.Sprintf("Jellyfin has wrong metadata (TVDB %s instead of %d)", jellyfinTVDB, media.TVDBID)
				media.JellyfinMatchReason = fmt.Sprintf(
					"Jellyfin identifies the files at this path as %q (TVDB %s), not %q (TVDB %d)",
					js.Name, jellyfinTVDB, media.Title, media.TVDBID)
				media.JellyfinConflictID = js.ID
				media.JellyfinID = ""
				media.HasPoster = false
				media.JellyfinVerdict = ""
				showMismatch++
				log.Warn().
					Str("title", media.Title).
					Int("sonarr_tvdb_id", media.TVDBID).
					Str("jellyfin_title", js.Name).
					Str("jellyfin_tvdb_id", jellyfinTVDB).
					Str("jellyfin_item_id", js.ID).
					Msg("Path conflict detected for TV show")
			} else {
				// Not found in Jellyfin at all
				media.JellyfinMatchStatus = "not_found"
				media.JellyfinMismatchInfo = "Item not found in Jellyfin library"
				media.JellyfinMatchReason = jellyfinNotScannedReason
				media.JellyfinConflictID = ""
				media.JellyfinID = ""
				media.HasPoster = false
				media.JellyfinVerdict = ""
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

// Jellyfin match-fix errors. They are sentinels so the API layer can map each
// outcome to a precise HTTP status without string matching.
var (
	// ErrJellyfinItemNotFound means no Jellyfin library item could be resolved
	// for the media (no stored conflict id and no path match). The file is
	// likely not scanned yet, so there is nothing to re-identify.
	ErrJellyfinItemNotFound = errors.New("jellyfin item not found (file may not be scanned)")
	// ErrMatchArrWrong means the adjudicator concluded the Sonarr/Radarr entry
	// is the outlier. The fix belongs in the arr; Jellyfin is left untouched.
	ErrMatchArrWrong = errors.New("arr (Sonarr/Radarr) identity is wrong; fix it in the arr")
	// ErrMatchAmbiguous means the evidence does not clearly favour either side,
	// so the fix is refused without an explicit override.
	ErrMatchAmbiguous = errors.New("match is ambiguous; refusing to modify Jellyfin")
	// ErrRemoteSearchNoMatch means Jellyfin's remote search returned no candidate
	// carrying the arr's provider id.
	ErrRemoteSearchNoMatch = errors.New("no Jellyfin remote-search result matched the arr identity")
)

// MatchRefusalError is returned when a Fix Match request declines to mutate
// Jellyfin. It wraps one of the exported sentinels so errors.Is still matches
// ErrMatchArrWrong / ErrMatchAmbiguous / ErrRemoteSearchNoMatch, and it carries
// the adjudication that justified the refusal so a caller can present that
// evidence without re-running the analysis.
type MatchRefusalError struct {
	// Err is the exported sentinel the refusal maps to.
	Err error
	// Analysis is the adjudication behind the refusal; nil when the failure
	// happened before an analysis could be produced.
	Analysis *MatchAnalysis
}

// Error implements error, appending the verdict when an analysis is carried.
func (e *MatchRefusalError) Error() string {
	if e.Analysis != nil {
		return fmt.Sprintf("%s (verdict=%s)", e.Err.Error(), e.Analysis.Verdict)
	}
	return e.Err.Error()
}

// Unwrap exposes the wrapped sentinel to errors.Is / errors.As.
func (e *MatchRefusalError) Unwrap() error { return e.Err }

// FixResult is the outcome of a successful FixJellyfinMatch: the re-identified
// Jellyfin item, the identity that was applied, and the analysis that justified it.
type FixResult struct {
	MediaID            string            `json:"media_id"`
	JellyfinID         string            `json:"jellyfin_id"`
	Title              string            `json:"title"`
	AppliedProviderIDs map[string]string `json:"applied_provider_ids,omitempty"`
	Analysis           MatchAnalysis     `json:"analysis"`
}

// arrMatchIdentity is the arr side of the comparison: its identity, a
// representative file path, and (for TV) its episode list.
type arrMatchIdentity struct {
	Identity Identity
	FilePath string
	Episodes []Episode
}

// AnalyzeJellyfinMatch adjudicates which side holds the wrong identity for a
// media item. It gathers the arr identity, resolves the conflicting Jellyfin
// item, and delegates the actual reasoning to the pure AnalyzeMatch. It never
// mutates Jellyfin and is only ever called by an explicit user request.
func (e *SyncEngine) AnalyzeJellyfinMatch(ctx context.Context, mediaID string) (*MatchAnalysis, error) {
	media, found := e.GetMediaByID(mediaID)
	if !found {
		return nil, fmt.Errorf("media not found: %s", mediaID)
	}
	if e.jellyfinClient == nil {
		return nil, fmt.Errorf("analyzing match for %s: jellyfin integration is disabled", mediaID)
	}

	item, err := e.resolveJellyfinItem(ctx, media)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, fmt.Errorf("analyzing match for %s: %w", mediaID, ErrJellyfinItemNotFound)
	}

	arr, err := e.buildArrIdentity(ctx, media)
	if err != nil {
		return nil, err
	}
	analysis, err := e.analyzeJellyfinMatch(ctx, media, item, arr)
	if err != nil {
		return nil, err
	}

	e.setJellyfinVerdict(mediaID, analysis.Verdict)
	return &analysis, nil
}

// FixJellyfinMatch re-identifies the Jellyfin item against the arr identity when
// the adjudicator finds Jellyfin is the outlier. It refuses arr_wrong and
// ambiguous verdicts, applies the matching RemoteSearch result (optionally
// replacing the item's images, per replaceImages), then re-runs the read-only
// Jellyfin sync so the item flips to "matched". Only an explicit, user-confirmed
// request may call it; no sync or job path invokes it.
func (e *SyncEngine) FixJellyfinMatch(ctx context.Context, mediaID string, replaceImages bool) (*FixResult, error) {
	media, found := e.GetMediaByID(mediaID)
	if !found {
		return nil, fmt.Errorf("media not found: %s", mediaID)
	}
	if e.jellyfinClient == nil {
		return nil, fmt.Errorf("fixing match for %s: jellyfin integration is disabled", mediaID)
	}

	item, err := e.resolveJellyfinItem(ctx, media)
	if err != nil {
		return nil, err
	}
	if item == nil {
		refusal := fmt.Errorf("fixing match for %s: %w", mediaID, ErrJellyfinItemNotFound)
		logMatchRefusal(mediaID, refusal, nil)
		return nil, refusal
	}

	arr, err := e.buildArrIdentity(ctx, media)
	if err != nil {
		return nil, err
	}
	analysis, err := e.analyzeJellyfinMatch(ctx, media, item, arr)
	if err != nil {
		return nil, err
	}

	switch analysis.Verdict {
	case VerdictJellyfinWrong:
		// Jellyfin is the outlier; it is safe to re-identify it.
	case VerdictArrWrong:
		refusal := &MatchRefusalError{Err: ErrMatchArrWrong, Analysis: &analysis}
		logMatchRefusal(mediaID, refusal, &analysis)
		return nil, refusal
	default:
		refusal := &MatchRefusalError{Err: ErrMatchAmbiguous, Analysis: &analysis}
		logMatchRefusal(mediaID, refusal, &analysis)
		return nil, refusal
	}

	result, providerIDs, err := e.findJellyfinRemoteMatch(ctx, media, arr.Identity)
	if err != nil {
		if errors.Is(err, ErrRemoteSearchNoMatch) {
			refusal := &MatchRefusalError{Err: ErrRemoteSearchNoMatch, Analysis: &analysis}
			logMatchRefusal(mediaID, refusal, &analysis)
			return nil, refusal
		}
		return nil, err
	}
	if err := e.jellyfinClient.ApplyRemoteSearch(ctx, item.ID, result, replaceImages); err != nil {
		return nil, fmt.Errorf("applying remote search to Jellyfin item %s: %w", item.ID, err)
	}
	// Jellyfin has now been re-identified. Clear the stale mismatch diagnosis
	// immediately and independently of the re-sync below: a re-sync failure must
	// not leave the item holding the prior jellyfin_wrong verdict, mismatch reason
	// and conflict id after a successful fix. Only the diagnosis fields are
	// touched; watch counts and play state are preserved.
	e.clearJellyfinDiagnosis(mediaID)
	// Jellyfin is already mutated at this point. A re-sync failure must not turn
	// a successful fix into a 500, so log it and fall back to the identity that
	// was just applied rather than failing the request.
	if err := e.syncJellyfin(ctx); err != nil {
		log.Warn().Err(err).Str("media_id", mediaID).
			Msg("post-fix Jellyfin re-sync failed; Jellyfin was already updated")
	}

	updated, found := e.GetMediaByID(mediaID)
	if !found {
		log.Warn().Str("media_id", mediaID).
			Msg("media item disappeared during post-fix re-sync; reporting the applied identity")
		updated = models.Media{ID: mediaID, Title: media.Title}
	}
	if updated.JellyfinID == "" {
		// The item is re-identified in place; report its id even when the
		// refreshed sync could not yet observe the change.
		updated.JellyfinID = item.ID
	}

	log.Info().
		Str("media_id", mediaID).
		Str("jellyfin_item_id", item.ID).
		Str("verdict", string(analysis.Verdict)).
		Float64("confidence", analysis.Confidence).
		Int("matched_episodes", analysis.matchedEpisodes).
		Bool("replace_images", replaceImages).
		Msg("Re-identified Jellyfin item after manual Fix Match")

	return &FixResult{
		MediaID:            mediaID,
		JellyfinID:         updated.JellyfinID,
		Title:              updated.Title,
		AppliedProviderIDs: providerIDs,
		Analysis:           analysis,
	}, nil
}

// logMatchRefusal records why a manual Fix Match declined to touch Jellyfin. It
// is emitted at warn so a refused fix is visible in the server log even though
// the handler can only return a 4xx the user may not fully read. analysis is nil
// when the refusal happened before an adjudication existed (no Jellyfin item).
func logMatchRefusal(mediaID string, err error, analysis *MatchAnalysis) {
	event := log.Warn().Str("media_id", mediaID).Str("reason", err.Error())
	if analysis != nil {
		event = event.
			Str("verdict", string(analysis.Verdict)).
			Float64("confidence", analysis.Confidence).
			Int("matched_episodes", analysis.matchedEpisodes)
	}
	event.Msg("Refused manual Fix Match")
}

// analyzeJellyfinMatch turns the arr identity and an already-resolved Jellyfin
// item into adjudicator input and runs the pure AnalyzeMatch.
func (e *SyncEngine) analyzeJellyfinMatch(ctx context.Context, media models.Media, item *clients.JellyfinItem, arr arrMatchIdentity) (MatchAnalysis, error) {
	input := MatchAnalysisInput{
		MediaType:   media.Type,
		Arr:         arr.Identity,
		Jellyfin:    jellyfinIdentity(media.Type, item),
		FilePath:    arr.FilePath,
		ArrEpisodes: arr.Episodes,
	}

	// episodeFetchErr records a failed empty-list retry so the evidence below
	// can distinguish "Jellyfin really has no episodes" from "the episode fetch
	// failed" instead of masking an API failure as a considered no-data result.
	var episodeFetchErr error
	if media.Type == models.MediaTypeTVShow {
		episodes, err := e.jellyfinClient.GetEpisodes(ctx, item.ID)
		if err != nil {
			return MatchAnalysis{}, fmt.Errorf("fetching Jellyfin episodes for series %s: %w", item.ID, err)
		}
		// A Jellyfin series can transiently report an empty episode list while
		// it is mid-refresh (e.g. immediately after a library scan or a manual
		// fix). Re-fetch once before adjudicating so a transient empty list is
		// not read as "no shared episodes" and silently turned into ambiguity.
		if len(episodes) == 0 {
			log.Debug().Str("jellyfin_item_id", item.ID).
				Msg("Jellyfin returned no episodes for a series; retrying once")
			retry, retryErr := e.jellyfinClient.GetEpisodes(ctx, item.ID)
			if retryErr != nil {
				episodeFetchErr = retryErr
				log.Warn().Err(retryErr).Str("jellyfin_item_id", item.ID).
					Msg("Jellyfin episode retry failed; adjudicating without episode data")
			} else {
				episodes = retry
			}
		}
		input.JellyfinEpisodes = jellyfinEpisodesToEvidence(episodes)
	}

	analysis := AnalyzeMatch(input)
	if media.Type == models.MediaTypeTVShow {
		// Derive the shared-episode count for refusal logging. Pure computation
		// over the already-fetched inputs; no additional I/O.
		_, matched, _ := episodeAgreement(input.ArrEpisodes, input.JellyfinEpisodes)
		analysis.matchedEpisodes = matched
	}
	// Surface the conflicting Jellyfin item and its provider ids explicitly in
	// the evidence so the UI can show exactly what Jellyfin currently thinks.
	analysis.Evidence = append(analysis.Evidence, fmt.Sprintf(
		"conflicting Jellyfin item %s is %q (%s)", item.ID, item.Name, describeProviderIDs(item.ProviderIds)))
	// When episode data is genuinely absent after the retry, say so explicitly
	// rather than letting the verdict read as a considered "ambiguous". A failed
	// retry is reported as an API failure, not as a no-episodes result.
	if media.Type == models.MediaTypeTVShow && len(input.JellyfinEpisodes) == 0 {
		if episodeFetchErr != nil {
			analysis.Evidence = append(analysis.Evidence,
				"Jellyfin episode fetch failed: "+episodeFetchErr.Error())
		} else {
			analysis.Evidence = append(analysis.Evidence, "Jellyfin returned no episodes for this series")
		}
	}
	return analysis, nil
}

// buildArrIdentity gathers the Sonarr/Radarr side of the comparison. It prefers
// live arr data (Radarr GetMovie / Sonarr GetSeriesByID + GetEpisodes) and falls
// back to the synced media fields when no arr client is available.
func (e *SyncEngine) buildArrIdentity(ctx context.Context, media models.Media) (arrMatchIdentity, error) {
	if media.Type == models.MediaTypeTVShow {
		return e.buildTVArrIdentity(ctx, media)
	}
	return e.buildMovieArrIdentity(ctx, media)
}

// buildMovieArrIdentity resolves the Radarr movie identity and its file path.
func (e *SyncEngine) buildMovieArrIdentity(ctx context.Context, media models.Media) (arrMatchIdentity, error) {
	identity := Identity{
		Title:      media.Title,
		Year:       media.Year,
		ProviderID: providerRef("tmdb", media.TMDBID),
	}
	filePath := media.FilePath

	if e.radarrClient == nil || media.RadarrID <= 0 {
		return arrMatchIdentity{Identity: identity, FilePath: filePath}, nil
	}

	movie, err := e.radarrClient.GetMovie(ctx, media.RadarrID)
	if err != nil {
		return arrMatchIdentity{}, fmt.Errorf("fetching Radarr movie %d: %w", media.RadarrID, err)
	}
	identity.Title = firstNonEmpty(movie.Title, identity.Title)
	identity.Year = firstNonZero(movie.Year, identity.Year)
	identity.ProviderID = providerRef("tmdb", firstNonZero(movie.TmdbId, media.TMDBID))
	if movie.MovieFile != nil && movie.MovieFile.Path != "" {
		filePath = movie.MovieFile.Path
	}
	return arrMatchIdentity{Identity: identity, FilePath: filePath}, nil
}

// buildTVArrIdentity resolves the Sonarr series identity and its episodes.
func (e *SyncEngine) buildTVArrIdentity(ctx context.Context, media models.Media) (arrMatchIdentity, error) {
	identity := Identity{
		Title:      media.Title,
		Year:       media.Year,
		ProviderID: providerRef("tvdb", media.TVDBID),
	}
	filePath := media.FilePath
	var episodes []Episode

	if e.sonarrClient == nil || media.SonarrID <= 0 {
		return arrMatchIdentity{Identity: identity, FilePath: filePath, Episodes: episodes}, nil
	}

	series, err := e.sonarrClient.GetSeriesByID(ctx, media.SonarrID)
	if err != nil {
		return arrMatchIdentity{}, fmt.Errorf("fetching Sonarr series %d: %w", media.SonarrID, err)
	}
	identity.Title = firstNonEmpty(series.Title, identity.Title)
	identity.Year = firstNonZero(series.Year, identity.Year)
	identity.ProviderID = providerRef("tvdb", firstNonZero(series.TvdbId, media.TVDBID))

	sonarrEpisodes, err := e.sonarrClient.GetEpisodes(ctx, media.SonarrID)
	if err != nil {
		return arrMatchIdentity{}, fmt.Errorf("fetching Sonarr episodes for series %d: %w", media.SonarrID, err)
	}
	episodes = sonarrEpisodesToEvidence(sonarrEpisodes)
	if path := firstEpisodeFile(sonarrEpisodes); path != "" {
		filePath = path
	}
	return arrMatchIdentity{Identity: identity, FilePath: filePath, Episodes: episodes}, nil
}

// resolveJellyfinItem locates the Jellyfin item involved in the conflict. It
// prefers the stored conflict id (set by sync classification) and falls back to
// a normalized path lookup. A miss returns (nil, nil).
func (e *SyncEngine) resolveJellyfinItem(ctx context.Context, media models.Media) (*clients.JellyfinItem, error) {
	var (
		items []clients.JellyfinItem
		err   error
	)
	if media.Type == models.MediaTypeTVShow {
		items, err = e.jellyfinClient.GetTVShows(ctx)
	} else {
		items, err = e.jellyfinClient.GetMovies(ctx)
	}
	if err != nil {
		return nil, fmt.Errorf("listing Jellyfin items: %w", err)
	}

	if id := media.JellyfinConflictID; id != "" {
		for i := range items {
			if items[i].ID == id {
				return &items[i], nil
			}
		}
	}

	if item := lookupJellyfinPath(items, normalizeJellyfinPathKey(media.FilePath)); item != nil {
		return item, nil
	}
	return nil, nil
}

// jellyfinIdentity maps a Jellyfin item onto the adjudicator's Identity, using
// the provider key appropriate to the media type.
func jellyfinIdentity(mediaType models.MediaType, item *clients.JellyfinItem) Identity {
	key := "Tmdb"
	if mediaType == models.MediaTypeTVShow {
		key = "Tvdb"
	}
	return Identity{
		Title:      item.Name,
		Year:       item.ProductionYear,
		ProviderID: providerRefFromMap(item.ProviderIds, key),
	}
}

// jellyfinEpisodesToEvidence normalizes Jellyfin episodes for adjudication.
func jellyfinEpisodesToEvidence(episodes []clients.JellyfinEpisode) []Episode {
	result := make([]Episode, 0, len(episodes))
	for _, ep := range episodes {
		result = append(result, Episode{
			Season:  ep.ParentIndexNumber,
			Episode: ep.IndexNumber,
			Title:   ep.Name,
			Path:    ep.Path,
		})
	}
	return result
}

// sonarrEpisodesToEvidence normalizes Sonarr episodes. Episodes with a file on
// disk are preferred; if none carry a file, all episodes are used so title
// agreement can still be measured.
func sonarrEpisodesToEvidence(episodes []clients.SonarrEpisode) []Episode {
	withFile := make([]Episode, 0, len(episodes))
	for _, ep := range episodes {
		if ep.EpisodeFile == nil {
			continue
		}
		withFile = append(withFile, Episode{
			Season:  ep.SeasonNumber,
			Episode: ep.EpisodeNumber,
			Title:   ep.Title,
			Path:    ep.EpisodeFile.Path,
		})
	}
	if len(withFile) > 0 {
		return withFile
	}

	all := make([]Episode, 0, len(episodes))
	for _, ep := range episodes {
		all = append(all, Episode{Season: ep.SeasonNumber, Episode: ep.EpisodeNumber, Title: ep.Title})
	}
	return all
}

// firstEpisodeFile returns the first on-disk episode file path, if any.
func firstEpisodeFile(episodes []clients.SonarrEpisode) string {
	for _, ep := range episodes {
		if ep.EpisodeFile != nil && ep.EpisodeFile.Path != "" {
			return ep.EpisodeFile.Path
		}
	}
	return ""
}

// providerRef formats a provider id as "key:value" (e.g. "tmdb:605722").
func providerRef(key string, id int) string {
	if id <= 0 {
		return ""
	}
	return key + ":" + strconv.Itoa(id)
}

// providerRefFromMap formats a provider id held in a Jellyfin ProviderIds map.
func providerRefFromMap(ids map[string]string, key string) string {
	value := ids[key]
	if value == "" {
		return ""
	}
	return strings.ToLower(key) + ":" + value
}

// providerKeyValue is one remote-search provider filter.
type providerKeyValue struct {
	key   string
	value string
}

// remoteSearchProviderCandidates returns the provider ids to try, in priority
// order, for the arr identity. The resolved identity's provider id is
// authoritative; the synced media ids are used only when the identity carries
// none (e.g. the arr client could not be reached). Movies use TMDB; shows try
// TVDB then TMDB.
func remoteSearchProviderCandidates(media models.Media, identity Identity) []providerKeyValue {
	if identity.ProviderID != "" {
		return []providerKeyValue{splitProviderRef(identity.ProviderID)}
	}

	candidates := make([]providerKeyValue, 0, 2)
	if media.Type == models.MediaTypeTVShow {
		if ref := providerRef("tvdb", media.TVDBID); ref != "" {
			candidates = append(candidates, splitProviderRef(ref))
		}
		if ref := providerRef("tmdb", media.TMDBID); ref != "" {
			candidates = append(candidates, splitProviderRef(ref))
		}
		return candidates
	}
	if ref := providerRef("tmdb", media.TMDBID); ref != "" {
		candidates = append(candidates, splitProviderRef(ref))
	}
	return candidates
}

// findJellyfinRemoteMatch searches Jellyfin for the arr identity and returns the
// first candidate whose provider ids match one of the arr's provider ids.
func (e *SyncEngine) findJellyfinRemoteMatch(ctx context.Context, media models.Media, identity Identity) (clients.RemoteSearchResult, map[string]string, error) {
	candidates := remoteSearchProviderCandidates(media, identity)
	if len(candidates) == 0 {
		return clients.RemoteSearchResult{}, nil, fmt.Errorf("finding Jellyfin match for %s: %w", media.ID, ErrRemoteSearchNoMatch)
	}

	for _, candidate := range candidates {
		filter := map[string]string{candidate.key: candidate.value}
		var (
			results []clients.RemoteSearchResult
			err     error
		)
		if media.Type == models.MediaTypeTVShow {
			results, err = e.jellyfinClient.RemoteSearchSeries(ctx, identity.Title, identity.Year, filter)
		} else {
			results, err = e.jellyfinClient.RemoteSearchMovie(ctx, identity.Title, identity.Year, filter)
		}
		if err != nil {
			return clients.RemoteSearchResult{}, nil, fmt.Errorf("remote searching Jellyfin for %s: %w", media.ID, err)
		}
		if match, ok := selectRemoteSearchResult(results, candidate.key, candidate.value); ok {
			return match, match.ProviderIds, nil
		}
	}
	return clients.RemoteSearchResult{}, nil, fmt.Errorf("finding Jellyfin match for %s: %w", media.ID, ErrRemoteSearchNoMatch)
}

// selectRemoteSearchResult returns the first candidate whose ProviderIds carry
// the requested provider key/value.
func selectRemoteSearchResult(results []clients.RemoteSearchResult, key, value string) (clients.RemoteSearchResult, bool) {
	for _, result := range results {
		for resultKey, resultValue := range result.ProviderIds {
			if strings.EqualFold(resultKey, key) && resultValue == value {
				return result, true
			}
		}
	}
	return clients.RemoteSearchResult{}, false
}

// splitProviderRef parses a "key:value" provider ref into a remote-search filter.
func splitProviderRef(ref string) providerKeyValue {
	key, value, _ := strings.Cut(ref, ":")
	return providerKeyValue{key: key, value: value}
}

// describeProviderIDs renders a Jellyfin ProviderIds map for evidence messages.
func describeProviderIDs(ids map[string]string) string {
	if len(ids) == 0 {
		return "no provider ids"
	}
	known := []string{"Tmdb", "Tvdb", "Imdb"}
	parts := make([]string, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))
	for _, key := range known {
		if value, ok := ids[key]; ok {
			parts = append(parts, key+"="+value)
			seen[key] = struct{}{}
		}
	}
	for key, value := range ids {
		if _, ok := seen[key]; ok {
			continue
		}
		parts = append(parts, key+"="+value)
	}
	return strings.Join(parts, ", ")
}

// setJellyfinVerdict stores the adjudicator verdict on the in-memory media item
// so the UI can show it. It never touches Jellyfin.
func (e *SyncEngine) setJellyfinVerdict(mediaID string, verdict MatchVerdict) {
	e.mediaLibraryLock.Lock()
	defer e.mediaLibraryLock.Unlock()
	if media, ok := e.mediaLibrary[mediaID]; ok {
		media.JellyfinVerdict = string(verdict)
		e.mediaLibrary[mediaID] = media
	}
}

// clearJellyfinDiagnosis drops the adjudicator verdict and path-conflict
// diagnosis from the in-memory item once Jellyfin has been re-identified. It is
// deliberately independent of the post-apply re-sync so a re-sync failure
// cannot leave a stale jellyfin_wrong verdict behind. WatchCount and
// LastWatched are intentionally left untouched.
func (e *SyncEngine) clearJellyfinDiagnosis(mediaID string) {
	e.mediaLibraryLock.Lock()
	defer e.mediaLibraryLock.Unlock()
	if media, ok := e.mediaLibrary[mediaID]; ok {
		media.JellyfinVerdict = ""
		media.JellyfinMatchReason = ""
		media.JellyfinConflictID = ""
		e.mediaLibrary[mediaID] = media
	}
}

// firstNonEmpty returns the first non-empty string. It keeps live arr values
// ahead of synced fallbacks without nested conditionals.
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

// firstNonZero returns the first non-zero int (used for year/provider ids).
func firstNonZero(values ...int) int {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
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
					Str("resolved_username", username).
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

// syncStats syncs detailed watch history from the active stats provider (Jellystat, Streamystats, or Tracearr).
func (e *SyncEngine) syncStats(ctx context.Context) error {
	// Collect Jellyfin IDs of all known media items so that item-scoped providers
	// (e.g. Streamystats) can query only the relevant items.
	e.mediaLibraryLock.RLock()
	jellyfinIDs := make([]string, 0, len(e.mediaLibrary))
	for _, media := range e.mediaLibrary {
		if media.JellyfinID != "" {
			jellyfinIDs = append(jellyfinIDs, media.JellyfinID)
		}
	}
	e.mediaLibraryLock.RUnlock()

	history, err := e.statsClient.GetHistory(ctx, jellyfinIDs)
	if err != nil {
		return err
	}

	e.mediaLibraryLock.Lock()
	defer e.mediaLibraryLock.Unlock()

	// Minimum engagement for a play to count toward watch-time analytics.
	// Matches Tracearr's 2-minute gate so accidental plays are excluded.
	const minPlaybackSeconds = 120

	// Watch-time analytics are windowed by analytics.roi_period_days so ROI
	// reflects recent usage. Raw watch count and last-watched stay lifetime.
	var roiCutoff time.Time
	if cfg := config.Get(); cfg != nil && cfg.Analytics.ROIPeriodDays > 0 {
		roiCutoff = time.Now().AddDate(0, 0, -cfg.Analytics.ROIPeriodDays)
	}

	// Per-item aggregates: most recent watch timestamp, raw watch count, and
	// gated (>= minPlaybackSeconds) distinct plays with their summed time.
	type statsAgg struct {
		lastWatched  time.Time
		watchCount   int
		gatedPlays   int
		totalSeconds int64
		seenPlays    map[string]struct{}
	}
	aggs := make(map[string]*statsAgg)

	// playKey returns a stable per-play key. Providers that expose a play
	// identity (Tracearr reference_id, Jellystat session id) let resume chains
	// and duplicate rows collapse; providers without one fall back to a
	// synthetic key so each record still counts exactly once.
	playKey := func(item clients.StatsHistoryItem, seq int) string {
		if item.PlayID != "" {
			// Scope the play id to its item: episode rows share one aggregation
			// bucket (the series), so a provider reusing a play id across two
			// different episodes must not dedupe them against each other.
			return item.JellyfinItemID + "|" + item.PlayID
		}
		return fmt.Sprintf("%s|%d|%d|%d", item.JellyfinItemID, item.WatchedAt.UnixNano(), item.PlaybackSeconds, seq)
	}

	for seq, item := range history {
		// Attribute episode-level records to their series so TV plays update the
		// show. Providers that already key history by series (Jellystat) leave
		// SeriesID empty and fall back to the item id (JellyfinItemID), which is
		// the movie id for films and the series id for shows.
		aggKey := item.JellyfinItemID
		if item.SeriesID != "" {
			aggKey = item.SeriesID
		}

		agg := aggs[aggKey]
		if agg == nil {
			agg = &statsAgg{seenPlays: make(map[string]struct{})}
			aggs[aggKey] = agg
		}

		if agg.lastWatched.IsZero() || item.WatchedAt.After(agg.lastWatched) {
			agg.lastWatched = item.WatchedAt
		}
		agg.watchCount++

		if item.PlaybackSeconds < minPlaybackSeconds {
			continue
		}
		if !roiCutoff.IsZero() && item.WatchedAt.Before(roiCutoff) {
			continue
		}
		key := playKey(item, seq)
		if _, seen := agg.seenPlays[key]; seen {
			continue
		}
		agg.seenPlays[key] = struct{}{}
		agg.gatedPlays++
		agg.totalSeconds += int64(item.PlaybackSeconds)
	}

	// Update media library with accurate watch data from the stats provider.
	updatedCount := 0
	for id, media := range e.mediaLibrary {
		if media.JellyfinID == "" {
			continue
		}

		agg, found := aggs[media.JellyfinID]
		if !found {
			continue
		}

		updated := false

		if !agg.lastWatched.IsZero() && (media.LastWatched.IsZero() || agg.lastWatched.After(media.LastWatched)) {
			media.LastWatched = agg.lastWatched
			updated = true
		}

		if agg.watchCount > 0 {
			media.WatchCount = agg.watchCount
			updated = true
		}

		if media.GatedPlayCount != agg.gatedPlays {
			media.GatedPlayCount = agg.gatedPlays
			updated = true
		}

		if media.TotalWatchSeconds != agg.totalSeconds {
			media.TotalWatchSeconds = agg.totalSeconds
			updated = true
		}

		if updated {
			e.mediaLibrary[id] = media
			updatedCount++
		}
	}

	log.Info().
		Int("total_history_items", len(history)).
		Int("updated_media", updatedCount).
		Msg("Stats provider sync completed")

	return nil
}

// GetDiskMonitor returns the disk monitor instance (may be nil if disabled).
func (e *SyncEngine) GetDiskMonitor() *DiskMonitor {
	return e.diskMonitor
}

// GetJellyfinClient returns the Jellyfin client instance (may be nil if disabled).
func (e *SyncEngine) GetJellyfinClient() *clients.JellyfinClient {
	return e.jellyfinClient
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
func (e *SyncEngine) applyRetentionRules(ctx context.Context) {
	e.mediaLibraryLock.Lock()
	defer e.mediaLibraryLock.Unlock()

	for id, media := range e.mediaLibrary {
		verdict := e.rules.Evaluate(ctx, &media)

		// Update media with deletion date and human-readable reason from verdict
		media.DeleteAfter = verdict.DeleteAfter
		if !verdict.DeleteAfter.IsZero() {
			media.DaysUntilDue = int(time.Until(verdict.DeleteAfter).Hours() / 24)
			media.DeletionReason = FormatDeletionReason(verdict, &media)
		} else {
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
	e.applyRetentionRules(context.Background())
	// Manual leaving-soon overrides must be re-applied AFTER the retention
	// rules: applyRetentionRules overwrites DeleteAfter for every item, which
	// would otherwise wipe the fixed dates of manually-flagged items until the
	// next full sync. Mirrors the FullSync ordering (applyRetentionRules then
	// applyManualLeavingSoon).
	e.applyManualLeavingSoon()
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

// applyManualLeavingSoon applies manual leaving soon flags to all media items.
// Runs after applyRetentionRules — overrides DeleteAfter with the stored fixed date.
// Excluded items are never marked as manual leaving soon (exclusion wins).
func (e *SyncEngine) applyManualLeavingSoon() {
	if e.manualLeavingSoon == nil {
		return
	}

	e.mediaLibraryLock.Lock()
	defer e.mediaLibraryLock.Unlock()

	flaggedCount := 0
	for id, media := range e.mediaLibrary {
		// Exclusion takes priority — never apply manual flag to excluded items
		if media.IsExcluded {
			if media.IsManualLeavingSoon {
				media.IsManualLeavingSoon = false
				e.mediaLibrary[id] = media
			}
			continue
		}

		isFlagged := e.manualLeavingSoon.IsFlagged(id)
		if isFlagged {
			item, _ := e.manualLeavingSoon.Get(id)
			media.IsManualLeavingSoon = true
			media.DeleteAfter = item.DeleteAfter
			media.DaysUntilDue = int(time.Until(item.DeleteAfter).Hours() / 24)
			media.DeletionReason = "Manual leaving soon"
			e.mediaLibrary[id] = media
			flaggedCount++
		} else if media.IsManualLeavingSoon {
			// Flag was removed — clear it
			media.IsManualLeavingSoon = false
			e.mediaLibrary[id] = media
		}
	}

	log.Debug().
		Int("media_count", len(e.mediaLibrary)).
		Int("flagged_count", flaggedCount).
		Msg("Applied manual leaving soon flags to media")
}

// AddManualLeavingSoon flags a media item for leaving soon with a fixed DeleteAfter date.
// Returns 409-style error if the item is currently excluded.
func (e *SyncEngine) AddManualLeavingSoon(ctx context.Context, mediaID string) error {
	media, found := e.GetMediaByID(mediaID)
	if !found {
		return fmt.Errorf("media not found: %s", mediaID)
	}

	if media.IsExcluded {
		return fmt.Errorf("conflict: item is protected. Remove protection first")
	}

	leavingSoonDays := e.config.App.LeavingSoonDays
	if leavingSoonDays <= 0 {
		leavingSoonDays = 14
	}
	deleteAfter := time.Now().AddDate(0, 0, leavingSoonDays)

	item := storage.ManualLeavingSoonItem{
		ExternalID:   mediaID,
		ExternalType: "unknown",
		MediaType:    string(media.Type),
		Title:        media.Title,
		DeleteAfter:  deleteAfter,
		FlaggedAt:    time.Now(),
		FlaggedBy:    "api",
	}

	if media.RadarrID > 0 {
		item.ExternalID = fmt.Sprintf("radarr-%d", media.RadarrID)
		item.ExternalType = "radarr"
	} else if media.SonarrID > 0 {
		item.ExternalID = fmt.Sprintf("sonarr-%d", media.SonarrID)
		item.ExternalType = "sonarr"
	}

	if err := e.manualLeavingSoon.Add(item); err != nil {
		return fmt.Errorf("adding manual leaving soon flag: %w", err)
	}

	// Update media library immediately
	e.mediaLibraryLock.Lock()
	media.IsManualLeavingSoon = true
	media.DeleteAfter = deleteAfter
	media.DaysUntilDue = int(time.Until(deleteAfter).Hours() / 24)
	media.DeletionReason = "Manual leaving soon"
	e.mediaLibrary[mediaID] = media
	e.mediaLibraryLock.Unlock()

	log.Info().
		Str("media_id", mediaID).
		Str("title", media.Title).
		Time("delete_after", deleteAfter).
		Msg("Media manually flagged as leaving soon")

	return nil
}

// RemoveManualLeavingSoon removes the manual leaving soon flag from a media item.
func (e *SyncEngine) RemoveManualLeavingSoon(ctx context.Context, mediaID string) error {
	media, found := e.GetMediaByID(mediaID)
	if !found {
		return fmt.Errorf("media not found: %s", mediaID)
	}

	externalID := mediaID
	if media.RadarrID > 0 {
		externalID = fmt.Sprintf("radarr-%d", media.RadarrID)
	} else if media.SonarrID > 0 {
		externalID = fmt.Sprintf("sonarr-%d", media.SonarrID)
	}

	if err := e.manualLeavingSoon.Remove(externalID); err != nil {
		return fmt.Errorf("removing manual leaving soon flag: %w", err)
	}

	// Update media library immediately - clear all manual leaving soon fields
	e.mediaLibraryLock.Lock()
	media.IsManualLeavingSoon = false
	media.DeleteAfter = time.Time{}
	media.DaysUntilDue = 0
	media.DeletionReason = ""
	e.mediaLibrary[mediaID] = media
	e.mediaLibraryLock.Unlock()

	log.Info().
		Str("media_id", mediaID).
		Str("title", media.Title).
		Msg("Manual leaving soon flag removed from media")

	return nil
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
				"jellyfin_id":  media.JellyfinID,
				"title":        media.Title,
				"year":         media.Year,
				"type":         media.Type,
				"file_size":    media.FileSize,
				"delete_after": media.DeleteAfter,
				"days_overdue": daysOverdue,
				"reason":       media.DeletionReason,
				"last_watched": media.LastWatched,
				"has_poster":   media.HasPoster,
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

// ExecuteDeletions performs actual deletion of overdue media items.
// Before each whole-item deletion, a pre-deletion safety check refreshes the watch state
// from the active stats provider to catch any watch activity that occurred after the last evaluation.
// If the item was watched since evaluation, deletion is skipped (fail-safe).
// Episode-level deletions skip the safety check — count/age-based cleanup is not
// affected by recent show-level watch activity.
//
// Returns (deletedCount, episodeItemsProcessed, episodeFilesDeleted, protectedCount, failedCount, deletedItems).
//   - deletedCount: whole-item deletions completed successfully
//   - episodeItemsProcessed: candidates handled via episode-level deletion (not whole-item)
//   - episodeFilesDeleted: individual episode files removed
//   - protectedCount: candidates skipped by the pre-deletion watch-state safety check
//     (fresh watch activity extended their retention). Not a failure.
//   - failedCount: whole-item candidates that failed to delete, plus episode candidates
//     where at least one episode-file deletion failed. An episode candidate can therefore
//     contribute to both episodeItemsProcessed and failedCount.
func (e *SyncEngine) ExecuteDeletions(ctx context.Context, candidates []map[string]interface{}) (int, int, int, int, int, []map[string]interface{}) {
	deletedCount := 0
	episodeItemsProcessed := 0 // candidates handled via episode-level deletion (not whole-item)
	episodeFilesDeleted := 0
	protectedCount := 0
	failedCount := 0
	deletedItems := make([]map[string]interface{}, 0)

	log.Info().
		Int("candidates", len(candidates)).
		Msg("Executing deletions for overdue items")

	// Pre-fetch watch history once for all candidates to avoid O(candidates × pages) HTTP calls.
	var watchStateMap map[string]time.Time
	if e.statsClient != nil {
		// Collect Jellyfin IDs from candidates so Streamystats can query per-item.
		// Jellystat ignores this list and fetches all history in bulk.
		var jellyfinIDs []string
		for _, candidate := range candidates {
			if id, ok := candidate["jellyfin_id"].(string); ok && id != "" {
				jellyfinIDs = append(jellyfinIDs, id)
			}
		}
		var err error
		watchStateMap, err = e.buildWatchStateMap(ctx, jellyfinIDs)
		if err != nil {
			log.Warn().
				Err(err).
				Msg("Pre-deletion safety check failed — skipping all deletions for safety")
			return 0, 0, 0, 0, len(candidates), deletedItems
		}
	}

	for _, candidate := range candidates {
		mediaID, ok := candidate["id"].(string)
		if !ok {
			failedCount++
			log.Warn().Interface("candidate", candidate).Msg("Invalid media ID in deletion candidate")
			continue
		}

		media, found := e.GetMediaByID(mediaID)
		if !found {
			failedCount++
			log.Warn().Str("media_id", mediaID).Msg("Media not found in library, skipping deletion")
			continue
		}

		// Re-evaluate to get the current verdict (including episode file IDs).
		verdict := e.rules.Evaluate(ctx, &media)

		if verdict.HasEpisodeDeletions() {
			// Episode-level deletion — skip watch-state safety check.
			// Recent show-level watch activity should not protect old episodes
			// from rolling-window or age-based cleanup.
			episodeFailures := 0
			for _, episodeFileID := range verdict.EpisodeFileIDs {
				if e.sonarrClient == nil {
					log.Warn().Msg("Sonarr client not available for episode file deletion")
					episodeFailures++
					break
				}
				if err := e.sonarrClient.DeleteEpisodeFile(ctx, episodeFileID); err != nil {
					episodeFailures++
					log.Error().Err(err).
						Int("episode_file_id", episodeFileID).
						Str("show", media.Title).
						Msg("Failed to delete episode file")
					continue
				}
				episodeFilesDeleted++
				log.Info().
					Int("episode_file_id", episodeFileID).
					Str("show", media.Title).
					Msg("Episode file deleted")
			}
			if episodeFailures > 0 {
				failedCount++
			}
			// The candidate itself is counted as processed (its episode files
			// were handled), but failedCount above still reflects any file-level
			// deletion failures.
			episodeItemsProcessed++
			continue
		}

		// Standard whole-item deletion with watch-state safety check.
		// Pre-deletion safety check: refresh watch state from the active stats
		// provider to catch any watch activity that occurred between evaluation and deletion.
		if watchStateMap != nil && media.JellyfinID != "" {
			latestWatched := watchStateMap[media.JellyfinID]

			if latestWatched.After(media.LastWatched) {
				// Watch activity detected since last evaluation — re-evaluate with fresh data.
				updatedMedia := media
				updatedMedia.LastWatched = latestWatched
				if updatedMedia.WatchCount == 0 {
					updatedMedia.WatchCount = 1
				}

				freshVerdict := e.rules.Evaluate(ctx, &updatedMedia)
				if freshVerdict.IsProtected || freshVerdict.DeleteAfter.After(time.Now()) {
					protectedCount++
					log.Info().
						Str("media_id", mediaID).
						Str("title", media.Title).
						Time("new_last_watched", latestWatched).
						Msg("Watch activity extended retention — skipping deletion")
					continue
				}
			}
		}

		// Attempt whole-item deletion
		if err := e.DeleteMedia(ctx, mediaID, false); err != nil {
			failedCount++
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
		Int("episode_files_deleted", episodeFilesDeleted).
		Int("protected", protectedCount).
		Int("failed", failedCount).
		Msg("Deletion execution completed")

	return deletedCount, episodeItemsProcessed, episodeFilesDeleted, protectedCount, failedCount, deletedItems
}

// ErrSyncInProgress is returned by ExecuteDeletionsLocked when a full or
// incremental sync currently holds the serialization lock. Callers should
// surface it as a transient busy state (e.g. HTTP 409) instead of blocking,
// since waiting for the lock and then running with an already-expired request
// context would turn every candidate into a spurious failure.
var ErrSyncInProgress = errors.New("a sync is currently in progress")

// ExecuteDeletionsLocked runs ExecuteDeletions while holding syncRunMu so a
// manual deletion pass (POST /api/deletions/execute) cannot overlap a running
// full or incremental sync. FullSync calls ExecuteDeletions directly while it
// already holds the lock, so only external callers should use this wrapper.
//
// It does not block for the lock: if a sync is running it returns
// ErrSyncInProgress immediately so the request can be rejected with a retryable
// status rather than hanging and then executing against a cancelled context.
func (e *SyncEngine) ExecuteDeletionsLocked(ctx context.Context, candidates []map[string]interface{}) (int, int, int, int, int, []map[string]interface{}, error) {
	if !e.syncRunMu.TryLock() {
		return 0, 0, 0, 0, 0, nil, ErrSyncInProgress
	}
	defer e.syncRunMu.Unlock()
	deletedCount, episodeItemsProcessed, episodeFilesDeleted, protectedCount, failedCount, deletedItems := e.ExecuteDeletions(ctx, candidates)
	return deletedCount, episodeItemsProcessed, episodeFilesDeleted, protectedCount, failedCount, deletedItems, nil
}

// buildWatchStateMap fetches watch history from the configured stats provider once and returns
// a map of jellyfinID → latest watch timestamp. This avoids repeated full-history
// fetches when checking multiple deletion candidates.
// jellyfinIDs is passed to support per-item providers (e.g. Streamystats); bulk providers (e.g. Jellystat, Tracearr) ignore it.
func (e *SyncEngine) buildWatchStateMap(ctx context.Context, jellyfinIDs []string) (map[string]time.Time, error) {
	history, err := e.statsClient.GetHistory(ctx, jellyfinIDs)
	if err != nil {
		return nil, fmt.Errorf("fetching watch history from stats provider: %w", err)
	}

	watchMap := make(map[string]time.Time, len(history))
	for _, item := range history {
		if item.WatchedAt.After(watchMap[item.JellyfinItemID]) {
			watchMap[item.JellyfinItemID] = item.WatchedAt
		}
	}

	log.Debug().
		Int("unique_items", len(watchMap)).
		Int("history_entries", len(history)).
		Msg("Built watch state map for pre-deletion safety check")

	return watchMap, nil
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
	InProgress    bool      `json:"in_progress"`
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

	// A sync (full/incremental) or deletion pass is in progress when the
	// serialization lock is held. Snapshot without blocking: TryLock fails if a
	// sync currently owns the lock.
	syncInProgress := !e.syncRunMu.TryLock()
	if !syncInProgress {
		e.syncRunMu.Unlock()
	}

	e.mediaLibraryLock.RLock()
	defer e.mediaLibraryLock.RUnlock()

	status := SyncStatus{
		Running:      running,
		InProgress:   syncInProgress,
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
