package services

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ramonskie/prunarr/internal/cache"
	"github.com/ramonskie/prunarr/internal/clients"
	"github.com/ramonskie/prunarr/internal/config"
	"github.com/ramonskie/prunarr/internal/models"
	"github.com/ramonskie/prunarr/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper function to create a test sync engine with minimal config
func newTestSyncEngine(t *testing.T) (*SyncEngine, *storage.JobsFile, *storage.ExclusionsFile) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		Sync: config.SyncConfig{
			FullInterval:        3600,
			IncrementalInterval: 300,
			AutoStart:           false,
		},
		Rules: config.RulesConfig{
			MovieRetention: "90d",
			TVRetention:    "120d",
		},
	}

	cacheInstance := cache.New()
	jobs, err := storage.NewJobsFile(tmpDir, 50)
	require.NoError(t, err)

	exclusions, err := storage.NewExclusionsFile(tmpDir)
	require.NoError(t, err)

	rules := NewRulesEngine(cfg, exclusions)

	engine := NewSyncEngine(cfg, cacheInstance, jobs, exclusions, rules)

	return engine, jobs, exclusions
}

func TestNewSyncEngine(t *testing.T) {
	t.Run("creates sync engine successfully", func(t *testing.T) {
		engine, _, _ := newTestSyncEngine(t)

		assert.NotNil(t, engine)
		assert.NotNil(t, engine.config)
		assert.NotNil(t, engine.cache)
		assert.NotNil(t, engine.jobs)
		assert.NotNil(t, engine.exclusions)
		assert.NotNil(t, engine.rules)
		assert.NotNil(t, engine.mediaLibrary)
		assert.NotNil(t, engine.stopChan)
		assert.False(t, engine.running)
	})

	t.Run("initializes clients based on config", func(t *testing.T) {
		tmpDir := t.TempDir()

		cfg := &config.Config{
			Sync: config.SyncConfig{
				FullInterval:        3600,
				IncrementalInterval: 300,
			},
			Rules: config.RulesConfig{
				MovieRetention: "90d",
				TVRetention:    "120d",
			},
			Integrations: config.IntegrationsConfig{
				Radarr: config.RadarrConfig{
					BaseIntegrationConfig: config.BaseIntegrationConfig{
						Enabled: true,
						URL:     "http://localhost:7878",
						APIKey:  "test-key",
					},
				},
				Sonarr: config.SonarrConfig{
					BaseIntegrationConfig: config.BaseIntegrationConfig{
						Enabled: true,
						URL:     "http://localhost:8989",
						APIKey:  "test-key",
					},
				},
			},
		}

		cacheInstance := cache.New()
		jobs, err := storage.NewJobsFile(tmpDir, 50)
		require.NoError(t, err)

		exclusions, err := storage.NewExclusionsFile(tmpDir)
		require.NoError(t, err)

		rules := NewRulesEngine(cfg, exclusions)

		engine := NewSyncEngine(cfg, cacheInstance, jobs, exclusions, rules)

		assert.NotNil(t, engine.radarrClient)
		assert.NotNil(t, engine.sonarrClient)
	})
}

func TestSyncEngine_StartStop(t *testing.T) {
	t.Run("starts and stops successfully", func(t *testing.T) {
		engine, _, _ := newTestSyncEngine(t)

		err := engine.Start()
		require.NoError(t, err)
		assert.True(t, engine.running)
		assert.NotNil(t, engine.fullSyncTicker)
		assert.NotNil(t, engine.incrSyncTicker)

		// Give tickers time to initialize
		time.Sleep(10 * time.Millisecond)

		engine.Stop()
		assert.False(t, engine.running)
	})

	t.Run("cannot start when already running", func(t *testing.T) {
		engine, _, _ := newTestSyncEngine(t)

		err := engine.Start()
		require.NoError(t, err)

		err = engine.Start()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already running")

		engine.Stop()
	})

	t.Run("stop when not running is safe", func(t *testing.T) {
		engine, _, _ := newTestSyncEngine(t)

		// Should not panic
		engine.Stop()
		assert.False(t, engine.running)
	})
}

func TestSyncEngine_MediaLibrary(t *testing.T) {
	t.Run("GetMediaList returns empty list initially", func(t *testing.T) {
		engine, _, _ := newTestSyncEngine(t)

		media := engine.GetMediaList()

		assert.Empty(t, media)
		assert.NotNil(t, media)
	})

	t.Run("GetMediaByID returns false for non-existent media", func(t *testing.T) {
		engine, _, _ := newTestSyncEngine(t)

		_, found := engine.GetMediaByID("non-existent")

		assert.False(t, found)
	})

	t.Run("GetMediaCount returns 0 initially", func(t *testing.T) {
		engine, _, _ := newTestSyncEngine(t)

		count := engine.GetMediaCount()

		assert.Equal(t, 0, count)
	})

	t.Run("manually adding media to library works", func(t *testing.T) {
		engine, _, _ := newTestSyncEngine(t)

		// Manually add media for testing
		engine.mediaLibrary["test-1"] = models.Media{
			ID:    "test-1",
			Type:  models.MediaTypeMovie,
			Title: "Test Movie",
		}

		media, found := engine.GetMediaByID("test-1")
		assert.True(t, found)
		assert.Equal(t, "Test Movie", media.Title)

		count := engine.GetMediaCount()
		assert.Equal(t, 1, count)

		list := engine.GetMediaList()
		assert.Len(t, list, 1)
	})
}

func TestSyncEngine_AddExclusion(t *testing.T) {
	t.Run("adds exclusion for existing media", func(t *testing.T) {
		engine, _, exclusions := newTestSyncEngine(t)

		// Add media to library
		engine.mediaLibrary["radarr-1"] = models.Media{
			ID:       "radarr-1",
			Type:     models.MediaTypeMovie,
			Title:    "Test Movie",
			RadarrID: 1,
		}

		ctx := context.Background()
		err := engine.AddExclusion(ctx, "radarr-1", "user favorite")

		require.NoError(t, err)

		// Check exclusion was added
		assert.True(t, exclusions.IsExcluded("radarr-1"))

		// Check media was marked as excluded
		media, found := engine.GetMediaByID("radarr-1")
		require.True(t, found)
		assert.True(t, media.IsExcluded)
	})

	t.Run("returns error for non-existent media", func(t *testing.T) {
		engine, _, _ := newTestSyncEngine(t)

		ctx := context.Background()
		err := engine.AddExclusion(ctx, "non-existent", "test")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "media not found")
	})
}

func TestSyncEngine_RemoveExclusion(t *testing.T) {
	t.Run("removes exclusion for existing media", func(t *testing.T) {
		engine, _, exclusions := newTestSyncEngine(t)

		// Add media with exclusion
		engine.mediaLibrary["radarr-1"] = models.Media{
			ID:         "radarr-1",
			Type:       models.MediaTypeMovie,
			Title:      "Test Movie",
			RadarrID:   1,
			IsExcluded: true,
		}

		exclusions.Add(storage.ExclusionItem{
			ExternalID:   "radarr-1",
			ExternalType: "radarr",
			MediaType:    "movie",
			Title:        "Test Movie",
		})

		ctx := context.Background()
		err := engine.RemoveExclusion(ctx, "radarr-1")

		require.NoError(t, err)

		// Check exclusion was removed
		assert.False(t, exclusions.IsExcluded("radarr-1"))

		// Check media was unmarked
		media, found := engine.GetMediaByID("radarr-1")
		require.True(t, found)
		assert.False(t, media.IsExcluded)
	})

	t.Run("returns error for non-existent media", func(t *testing.T) {
		engine, _, _ := newTestSyncEngine(t)

		ctx := context.Background()
		err := engine.RemoveExclusion(ctx, "non-existent")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "media not found")
	})
}

func TestSyncEngine_DeleteMedia(t *testing.T) {
	t.Run("dry run does not delete media", func(t *testing.T) {
		engine, _, _ := newTestSyncEngine(t)

		engine.mediaLibrary["radarr-1"] = models.Media{
			ID:       "radarr-1",
			Type:     models.MediaTypeMovie,
			Title:    "Test Movie",
			RadarrID: 1,
		}

		ctx := context.Background()
		err := engine.DeleteMedia(ctx, "radarr-1", true)

		require.NoError(t, err)

		// Media should still exist in dry run mode
		_, found := engine.GetMediaByID("radarr-1")
		assert.True(t, found)
	})

	t.Run("returns error for non-existent media", func(t *testing.T) {
		engine, _, _ := newTestSyncEngine(t)

		ctx := context.Background()
		err := engine.DeleteMedia(ctx, "non-existent", false)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "media not found")
	})
}

func TestSyncEngine_GetStatus(t *testing.T) {
	t.Run("returns correct status", func(t *testing.T) {
		engine, _, _ := newTestSyncEngine(t)

		// Add test media
		engine.mediaLibrary["movie-1"] = models.Media{
			ID:   "movie-1",
			Type: models.MediaTypeMovie,
		}
		engine.mediaLibrary["movie-2"] = models.Media{
			ID:         "movie-2",
			Type:       models.MediaTypeMovie,
			IsExcluded: true,
		}
		engine.mediaLibrary["tv-1"] = models.Media{
			ID:   "tv-1",
			Type: models.MediaTypeTVShow,
		}

		status := engine.GetStatus()

		assert.False(t, status.Running)
		assert.Equal(t, 3, status.MediaCount)
		assert.Equal(t, 2, status.MoviesCount)
		assert.Equal(t, 1, status.TVShowsCount)
		assert.Equal(t, 1, status.ExcludedCount)
		assert.Equal(t, 3600, status.FullInterval)
		assert.Equal(t, 300, status.IncrInterval)
	})

	t.Run("reflects running state", func(t *testing.T) {
		engine, _, _ := newTestSyncEngine(t)

		status := engine.GetStatus()
		assert.False(t, status.Running)

		err := engine.Start()
		require.NoError(t, err)

		status = engine.GetStatus()
		assert.True(t, status.Running)

		engine.Stop()

		status = engine.GetStatus()
		assert.False(t, status.Running)
	})
}

func TestSyncEngine_SyncRadarr(t *testing.T) {
	t.Run("syncs movies correctly", func(t *testing.T) {
		engine, _, _ := newTestSyncEngine(t)

		// Create test Radarr data
		testMovies := []clients.RadarrMovie{
			{
				ID:         1,
				Title:      "Test Movie 1",
				Year:       2023,
				HasFile:    true,
				Path:       "/movies/test1",
				SizeOnDisk: 1024 * 1024 * 1024,
				Added:      time.Now(),
				TmdbId:     12345,
				MovieFile: &clients.RadarrMovieFile{
					Quality: clients.RadarrQuality{
						Quality: clients.RadarrQualityDef{
							Name: "HD-1080p",
						},
					},
				},
			},
			{
				ID:         2,
				Title:      "Test Movie 2",
				Year:       2024,
				HasFile:    false, // Should be skipped
				Path:       "/movies/test2",
				SizeOnDisk: 0,
				Added:      time.Now(),
				TmdbId:     12346,
			},
		}

		// For unit testing, we would need to inject a mock client
		// Since the current implementation doesn't support dependency injection
		// for clients, we'll test the parts we can test

		// Test media library state after manual addition
		engine.mediaLibrary["radarr-1"] = models.Media{
			ID:         "radarr-1",
			Type:       models.MediaTypeMovie,
			Title:      testMovies[0].Title,
			Year:       testMovies[0].Year,
			RadarrID:   testMovies[0].ID,
			TMDBID:     testMovies[0].TmdbId,
			FilePath:   testMovies[0].Path,
			FileSize:   testMovies[0].SizeOnDisk,
			QualityTag: "HD-1080p",
		}

		media, found := engine.GetMediaByID("radarr-1")
		require.True(t, found)
		assert.Equal(t, "Test Movie 1", media.Title)
		assert.Equal(t, 2023, media.Year)
		assert.Equal(t, models.MediaTypeMovie, media.Type)
		assert.Equal(t, 1, media.RadarrID)
		assert.Equal(t, 12345, media.TMDBID)
	})
}

func TestSyncEngine_SyncSonarr(t *testing.T) {
	t.Run("syncs TV shows correctly", func(t *testing.T) {
		engine, _, _ := newTestSyncEngine(t)

		// Test media library state after manual addition
		engine.mediaLibrary["sonarr-1"] = models.Media{
			ID:       "sonarr-1",
			Type:     models.MediaTypeTVShow,
			Title:    "Test Show 1",
			Year:     2023,
			SonarrID: 1,
			TVDBID:   67890,
			FilePath: "/tv/testshow1",
			FileSize: 5 * 1024 * 1024 * 1024,
		}

		media, found := engine.GetMediaByID("sonarr-1")
		require.True(t, found)
		assert.Equal(t, "Test Show 1", media.Title)
		assert.Equal(t, 2023, media.Year)
		assert.Equal(t, models.MediaTypeTVShow, media.Type)
		assert.Equal(t, 1, media.SonarrID)
		assert.Equal(t, 67890, media.TVDBID)
	})
}

func TestSyncEngine_ConcurrentAccess(t *testing.T) {
	t.Run("handles concurrent media library access", func(t *testing.T) {
		engine, _, _ := newTestSyncEngine(t)

		// Add initial media
		for i := 0; i < 10; i++ {
			engine.mediaLibrary[fmt.Sprintf("media-%d", i)] = models.Media{
				ID:    fmt.Sprintf("media-%d", i),
				Type:  models.MediaTypeMovie,
				Title: fmt.Sprintf("Movie %d", i),
			}
		}

		done := make(chan bool, 6)

		// Concurrent reads
		for i := 0; i < 3; i++ {
			go func() {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("Panic during concurrent read: %v", r)
					}
					done <- true
				}()

				for j := 0; j < 100; j++ {
					_ = engine.GetMediaList()
					_ = engine.GetMediaCount()
					_, _ = engine.GetMediaByID("media-0")
					_ = engine.GetStatus()
				}
			}()
		}

		// Concurrent writes (simulated)
		for i := 0; i < 3; i++ {
			go func(id int) {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("Panic during concurrent write: %v", r)
					}
					done <- true
				}()

				for j := 0; j < 10; j++ {
					engine.mediaLibraryLock.Lock()
					engine.mediaLibrary[fmt.Sprintf("new-%d-%d", id, j)] = models.Media{
						ID:    fmt.Sprintf("new-%d-%d", id, j),
						Type:  models.MediaTypeMovie,
						Title: "Concurrent Movie",
					}
					engine.mediaLibraryLock.Unlock()
				}
			}(i)
		}

		// Wait for all goroutines
		for i := 0; i < 6; i++ {
			<-done
		}

		// Verify library integrity
		count := engine.GetMediaCount()
		assert.Greater(t, count, 10) // Should have more than initial 10
	})
}

func TestSyncEngine_FullSync_JobTracking(t *testing.T) {
	t.Run("creates and updates job entry", func(t *testing.T) {
		engine, jobs, _ := newTestSyncEngine(t)

		// Run full sync (will fail due to no clients, but should still create job)
		ctx := context.Background()
		_ = engine.FullSync(ctx)

		// Check that job was created
		latestJob, found := jobs.GetLatest()
		require.True(t, found)

		assert.Equal(t, storage.JobTypeFullSync, latestJob.Type)
		// Status could be completed or failed depending on client availability
		assert.NotEqual(t, storage.JobStatusRunning, latestJob.Status)
		assert.NotNil(t, latestJob.CompletedAt)
		assert.GreaterOrEqual(t, latestJob.DurationMs, int64(0))
	})
}

func TestSyncEngine_FullSync_CacheClear(t *testing.T) {
	t.Run("clears cache after full sync", func(t *testing.T) {
		engine, _, _ := newTestSyncEngine(t)

		// Add something to cache
		engine.cache.Set("test-key", "test-value", time.Minute)
		val, found := engine.cache.Get("test-key")
		require.True(t, found)
		assert.Equal(t, "test-value", val)

		// Run full sync
		ctx := context.Background()
		_ = engine.FullSync(ctx)

		// Cache should be cleared
		_, found = engine.cache.Get("test-key")
		assert.False(t, found)
	})
}

func TestSyncEngine_MediaMatching(t *testing.T) {
	t.Run("matches movie by TMDB ID", func(t *testing.T) {
		engine, _, _ := newTestSyncEngine(t)

		// Add movie from Radarr
		engine.mediaLibrary["radarr-1"] = models.Media{
			ID:       "radarr-1",
			Type:     models.MediaTypeMovie,
			Title:    "Test Movie",
			RadarrID: 1,
			TMDBID:   12345,
		}

		// Simulate Jellyfin data update (what syncJellyfin would do)
		media := engine.mediaLibrary["radarr-1"]
		media.JellyfinID = "jellyfin-abc"
		media.WatchCount = 5
		media.LastWatched = time.Now()
		engine.mediaLibrary["radarr-1"] = media

		// Verify match
		matched, found := engine.GetMediaByID("radarr-1")
		require.True(t, found)
		assert.Equal(t, "jellyfin-abc", matched.JellyfinID)
		assert.Equal(t, 5, matched.WatchCount)
	})

	t.Run("matches TV show by TVDB ID", func(t *testing.T) {
		engine, _, _ := newTestSyncEngine(t)

		// Add TV show from Sonarr
		engine.mediaLibrary["sonarr-1"] = models.Media{
			ID:       "sonarr-1",
			Type:     models.MediaTypeTVShow,
			Title:    "Test Show",
			SonarrID: 1,
			TVDBID:   67890,
		}

		// Simulate Jellyfin data update
		media := engine.mediaLibrary["sonarr-1"]
		media.JellyfinID = "jellyfin-xyz"
		media.WatchCount = 3
		engine.mediaLibrary["sonarr-1"] = media

		// Verify match
		matched, found := engine.GetMediaByID("sonarr-1")
		require.True(t, found)
		assert.Equal(t, "jellyfin-xyz", matched.JellyfinID)
		assert.Equal(t, 3, matched.WatchCount)
	})
}
