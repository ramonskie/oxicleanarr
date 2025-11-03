package services

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/ramonskie/prunarr/internal/cache"
	"github.com/ramonskie/prunarr/internal/clients"
	"github.com/ramonskie/prunarr/internal/config"
	"github.com/ramonskie/prunarr/internal/models"
	"github.com/ramonskie/prunarr/internal/storage"
	"github.com/rs/zerolog/log"
)

// SyncEngine handles media synchronization and cleanup operations
type SyncEngine struct {
	config     *config.Config
	cache      *cache.Cache
	jobs       *storage.JobsFile
	exclusions *storage.ExclusionsFile
	rules      *RulesEngine

	jellyfinClient   *clients.JellyfinClient
	radarrClient     *clients.RadarrClient
	sonarrClient     *clients.SonarrClient
	jellyseerrClient *clients.JellyseerrClient

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

	// Start full sync ticker
	fullInterval := time.Duration(e.config.Sync.FullInterval) * time.Second
	e.fullSyncTicker = time.NewTicker(fullInterval)

	// Start incremental sync ticker
	incrInterval := time.Duration(e.config.Sync.IncrementalInterval) * time.Second
	e.incrSyncTicker = time.NewTicker(incrInterval)

	// Run initial full sync if auto-start enabled
	if e.config.Sync.AutoStart {
		go func() {
			ctx := context.Background()
			if err := e.FullSync(ctx); err != nil {
				log.Error().Err(err).Msg("Initial full sync failed")
			}
		}()
	}

	// Start ticker goroutines
	go e.runFullSyncLoop()
	go e.runIncrementalSyncLoop()

	log.Info().
		Dur("full_interval", fullInterval).
		Dur("incr_interval", incrInterval).
		Msg("Sync engine started")

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

	// Sync requested items from Jellyseerr
	if e.jellyseerrClient != nil {
		if err := e.syncJellyseerr(ctx); err != nil {
			lastErr = err
			log.Error().Err(err).Msg("Failed to sync Jellyseerr")
		}
	}

	// Apply exclusions from file
	e.applyExclusions()

	// Apply retention rules to all media
	e.applyRetentionRules()

	// Calculate scheduled deletions and dry-run preview
	scheduledCount, wouldDelete := e.calculateDeletionInfo()

	// Update job
	completedAt := time.Now()
	duration := completedAt.Sub(startTime)

	job.CompletedAt = &completedAt
	job.DurationMs = duration.Milliseconds()
	job.Summary["movies"] = movieCount
	job.Summary["tv_shows"] = tvShowCount
	job.Summary["total_media"] = e.GetMediaCount()
	job.Summary["scheduled_deletions"] = scheduledCount
	job.Summary["dry_run"] = e.config.App.DryRun

	// Add deletion preview for dry-run mode
	if e.config.App.DryRun && len(wouldDelete) > 0 {
		job.Summary["would_delete"] = wouldDelete
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
		Bool("dry_run", e.config.App.DryRun).
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
			FilePath: rm.Path,
			FileSize: rm.SizeOnDisk,
			RadarrID: rm.ID,
			TMDBID:   rm.TmdbId,
		}

		if rm.MovieFile != nil {
			media.QualityTag = rm.MovieFile.Quality.Quality.Name
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

	// Update watch data for movies
	for _, jm := range jellyfinMovies {
		// Try to find corresponding media by TMDB ID
		tmdbID := jm.ProviderIds["Tmdb"]
		if tmdbID == "" {
			continue
		}

		for id, media := range e.mediaLibrary {
			if media.Type == models.MediaTypeMovie && strconv.Itoa(media.TMDBID) == tmdbID {
				media.JellyfinID = jm.ID
				media.WatchCount = jm.UserData.PlayCount
				if !jm.UserData.LastPlayedDate.IsZero() {
					media.LastWatched = jm.UserData.LastPlayedDate
				}
				e.mediaLibrary[id] = media
				break
			}
		}
	}

	// Get TV shows
	jellyfinShows, err := e.jellyfinClient.GetTVShows(ctx)
	if err != nil {
		return fmt.Errorf("fetching TV shows: %w", err)
	}

	// Update watch data for TV shows
	for _, js := range jellyfinShows {
		tvdbID := js.ProviderIds["Tvdb"]
		if tvdbID == "" {
			continue
		}

		for id, media := range e.mediaLibrary {
			if media.Type == models.MediaTypeTVShow && strconv.Itoa(media.TVDBID) == tvdbID {
				media.JellyfinID = js.ID
				media.WatchCount = js.UserData.PlayCount
				if !js.UserData.LastPlayedDate.IsZero() {
					media.LastWatched = js.UserData.LastPlayedDate
				}
				e.mediaLibrary[id] = media
				break
			}
		}
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
				if req.RequestedBy.Username != "" {
					media.RequestedByUsername = &req.RequestedBy.Username
				}
				if req.RequestedBy.Email != "" {
					media.RequestedByEmail = &req.RequestedBy.Email
				}
				e.mediaLibrary[id] = media
			}
		}
	}

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
		_, deleteAfter, _ := e.rules.EvaluateMedia(&media)

		// Update media with deletion date
		media.DeleteAfter = deleteAfter
		if !deleteAfter.IsZero() {
			daysUntilDue := int(time.Until(deleteAfter).Hours() / 24)
			media.DaysUntilDue = daysUntilDue

			// Set deletion reason for all items with future deletion dates
			if daysUntilDue > 0 {
				media.DeletionReason = e.rules.GenerateDeletionReason(&media, deleteAfter)
			}
		}

		e.mediaLibrary[id] = media
	}

	log.Debug().Int("media_count", len(e.mediaLibrary)).Msg("Applied retention rules to media")
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

// calculateDeletionInfo calculates scheduled deletions and returns dry-run preview
func (e *SyncEngine) calculateDeletionInfo() (int, []map[string]interface{}) {
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

		// Skip requested items
		if media.IsRequested {
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
			}
			wouldDelete = append(wouldDelete, candidate)
		}
	}

	return scheduledCount, wouldDelete
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

	// Delete from appropriate service
	if media.RadarrID > 0 && e.radarrClient != nil {
		if err := e.radarrClient.DeleteMovie(ctx, media.RadarrID, true); err != nil {
			return fmt.Errorf("deleting from Radarr: %w", err)
		}
	}

	if media.SonarrID > 0 && e.sonarrClient != nil {
		if err := e.sonarrClient.DeleteSeries(ctx, media.SonarrID, true); err != nil {
			return fmt.Errorf("deleting from Sonarr: %w", err)
		}
	}

	if media.JellyfinID != "" && e.jellyfinClient != nil {
		if err := e.jellyfinClient.DeleteItem(ctx, media.JellyfinID); err != nil {
			return fmt.Errorf("deleting from Jellyfin: %w", err)
		}
	}

	// Remove from library
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
	FullInterval  int       `json:"full_interval_seconds"`
	IncrInterval  int       `json:"incr_interval_seconds"`
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
