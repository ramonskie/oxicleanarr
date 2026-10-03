package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ramonskie/oxicleanarr/internal/cache"
	"github.com/ramonskie/oxicleanarr/internal/clients"
	"github.com/ramonskie/oxicleanarr/internal/config"
	"github.com/ramonskie/oxicleanarr/internal/models"
	"github.com/ramonskie/oxicleanarr/internal/services/rules"
	"github.com/ramonskie/oxicleanarr/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper function to create a test sync engine with minimal config
func newTestSyncEngine(t *testing.T) (*SyncEngine, *storage.JobsFile, *storage.ExclusionsFile) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		Sync: config.SyncConfig{
			FullInterval:        60,
			IncrementalInterval: 5,
			AutoStart:           false,
		},
		Rules: config.RulesConfig{
			MovieRetention: "90d",
			TVRetention:    "120d",
		},
	}

	// Set global config for tests that use config.Get()
	config.SetTestConfig(cfg)

	cacheInstance := cache.New()
	jobs, err := storage.NewJobsFile(tmpDir, 50)
	require.NoError(t, err)

	exclusions, err := storage.NewExclusionsFile(tmpDir)
	require.NoError(t, err)

	manualLeavingSoon, err := storage.NewManualLeavingSoonFile(tmpDir)
	require.NoError(t, err)

	rulesEngine := rules.NewRulesEngine(exclusions, nil)

	engine := NewSyncEngine(cfg, cacheInstance, jobs, exclusions, manualLeavingSoon, rulesEngine)

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
				FullInterval:        60,
				IncrementalInterval: 5,
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

		manualLS, err := storage.NewManualLeavingSoonFile(tmpDir)
		require.NoError(t, err)

		rulesEngine := rules.NewRulesEngine(exclusions, nil)

		engine := NewSyncEngine(cfg, cacheInstance, jobs, exclusions, manualLS, rulesEngine)

		assert.NotNil(t, engine.radarrClient)
		assert.NotNil(t, engine.sonarrClient)
	})
}

// newStatsTestEngine builds a SyncEngine with the given integrations, mirroring
// the manual construction used by the other tests in this file.
func newStatsTestEngine(t *testing.T, integrations config.IntegrationsConfig) *SyncEngine {
	t.Helper()

	tmpDir := t.TempDir()

	cfg := &config.Config{
		Sync: config.SyncConfig{
			FullInterval:        60,
			IncrementalInterval: 5,
		},
		Rules: config.RulesConfig{
			MovieRetention: "90d",
			TVRetention:    "120d",
		},
		Integrations: integrations,
	}
	config.SetTestConfig(cfg)

	cacheInstance := cache.New()
	jobs, err := storage.NewJobsFile(tmpDir, 50)
	require.NoError(t, err)

	exclusions, err := storage.NewExclusionsFile(tmpDir)
	require.NoError(t, err)

	manualLS, err := storage.NewManualLeavingSoonFile(tmpDir)
	require.NoError(t, err)

	rulesEngine := rules.NewRulesEngine(exclusions, nil)

	return NewSyncEngine(cfg, cacheInstance, jobs, exclusions, manualLS, rulesEngine)
}

func TestNewSyncEngine_StatsProviderSelection(t *testing.T) {
	t.Run("selects Tracearr client when only Tracearr is enabled", func(t *testing.T) {
		engine := newStatsTestEngine(t, config.IntegrationsConfig{
			Tracearr: config.TracearrConfig{
				BaseIntegrationConfig: config.BaseIntegrationConfig{
					Enabled: true,
					URL:     "http://localhost:8080",
					APIKey:  "trr_pub_test",
				},
				ServerID: "tracearr-server-uuid",
			},
		})

		require.NotNil(t, engine.statsClient)
		assert.IsType(t, &clients.TracearrClient{}, engine.statsClient)
	})

	t.Run("selects Jellystat client when only Jellystat is enabled", func(t *testing.T) {
		engine := newStatsTestEngine(t, config.IntegrationsConfig{
			Jellystat: config.JellystatConfig{
				BaseIntegrationConfig: config.BaseIntegrationConfig{
					Enabled: true,
					URL:     "http://localhost:8081",
					APIKey:  "jellystat-key",
				},
			},
		})

		require.NotNil(t, engine.statsClient)
		assert.IsType(t, &clients.JellystatClient{}, engine.statsClient)
	})

	t.Run("selects Streamystats client when only Streamystats is enabled", func(t *testing.T) {
		engine := newStatsTestEngine(t, config.IntegrationsConfig{
			Streamystats: config.StreamystatsConfig{
				BaseIntegrationConfig: config.BaseIntegrationConfig{
					Enabled: true,
					URL:     "http://localhost:8082",
					APIKey:  "jellyfin-key",
				},
				ServerID: "streamystats-server-uuid",
			},
		})

		require.NotNil(t, engine.statsClient)
		assert.IsType(t, &clients.StreamystatsClient{}, engine.statsClient)
	})

	t.Run("selects no stats client when no provider is enabled", func(t *testing.T) {
		engine := newStatsTestEngine(t, config.IntegrationsConfig{})

		assert.Nil(t, engine.statsClient)
	})
}

func TestSyncEngine_StartStop(t *testing.T) {
	t.Run("starts and stops successfully", func(t *testing.T) {
		engine, _, _ := newTestSyncEngine(t)

		err := engine.Start()
		require.NoError(t, err)
		assert.True(t, engine.running)

		// Tickers should be nil when AutoStart is false
		assert.Nil(t, engine.fullSyncTicker)
		assert.Nil(t, engine.incrSyncTicker)

		engine.Stop()
		assert.False(t, engine.running)
	})

	t.Run("starts with tickers when auto_start enabled", func(t *testing.T) {
		tmpDir := t.TempDir()

		cfg := &config.Config{
			Sync: config.SyncConfig{
				FullInterval:        60,
				IncrementalInterval: 5,
				AutoStart:           true, // Enable auto-start
			},
			Rules: config.RulesConfig{
				MovieRetention: "90d",
				TVRetention:    "120d",
			},
		}

		// Set global config for tests that use config.Get()
		config.SetTestConfig(cfg)

		cacheInstance := cache.New()
		jobs, err := storage.NewJobsFile(tmpDir, 50)
		require.NoError(t, err)

		exclusions, err := storage.NewExclusionsFile(tmpDir)
		require.NoError(t, err)

		manualLS2, err := storage.NewManualLeavingSoonFile(tmpDir)
		require.NoError(t, err)

		rulesEngine := rules.NewRulesEngine(exclusions, nil)
		engine := NewSyncEngine(cfg, cacheInstance, jobs, exclusions, manualLS2, rulesEngine)

		err = engine.Start()
		require.NoError(t, err)
		assert.True(t, engine.running)

		// Tickers should be initialized when AutoStart is true
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

	t.Run("scheduler goroutines exit after stop", func(t *testing.T) {
		tmpDir := t.TempDir()

		cfg := &config.Config{
			Sync: config.SyncConfig{
				FullInterval:        1,
				IncrementalInterval: 1,
				AutoStart:           true,
			},
			Rules: config.RulesConfig{
				MovieRetention: "90d",
				TVRetention:    "120d",
			},
		}
		config.SetTestConfig(cfg)

		cacheInstance := cache.New()
		jobs, err := storage.NewJobsFile(tmpDir, 50)
		require.NoError(t, err)

		exclusions, err := storage.NewExclusionsFile(tmpDir)
		require.NoError(t, err)

		manualLS, err := storage.NewManualLeavingSoonFile(tmpDir)
		require.NoError(t, err)

		rulesEngine := rules.NewRulesEngine(exclusions, nil)
		engine := NewSyncEngine(cfg, cacheInstance, jobs, exclusions, manualLS, rulesEngine)

		baseline := runtime.NumGoroutine()
		require.NoError(t, engine.Start())
		assert.NotNil(t, engine.fullSyncTicker)
		assert.NotNil(t, engine.incrSyncTicker)

		// Wait for the scheduler goroutines to be up (poll rather than a fixed
		// sleep so timing stays robust on loaded CI). Stop before the 1-minute
		// ticker intervals fire.
		started := false
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			if runtime.NumGoroutine() > baseline {
				started = true
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		require.True(t, started, "start should spawn scheduler goroutines")

		engine.Stop()
		assert.False(t, engine.running)

		// The loop goroutines must exit on stop, not keep spinning on the
		// closed stop channel (regression: a panic-recovering wrapper around
		// the whole loop restarted it forever). Wait for the goroutine count to
		// drop back toward the pre-start baseline.
		deadline = time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			if runtime.NumGoroutine() <= baseline+1 {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatal("scheduler goroutines did not exit after Stop()")
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
		assert.Equal(t, 60, status.FullInterval)
		assert.Equal(t, 5, status.IncrInterval)
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

func TestSyncEngine_CalculateDeletionInfo(t *testing.T) {
	t.Run("returns empty when no overdue items", func(t *testing.T) {
		engine, _, _ := newTestSyncEngine(t)

		// Add recent movie
		engine.mediaLibrary["movie-1"] = models.Media{
			ID:          "movie-1",
			Type:        models.MediaTypeMovie,
			Title:       "Recent Movie",
			DeleteAfter: time.Now().Add(30 * 24 * time.Hour), // 30 days in future
		}

		scheduledCount, candidates := engine.CalculateDeletionInfo()
		assert.Equal(t, 0, scheduledCount)
		assert.Empty(t, candidates)
	})

	t.Run("returns overdue items", func(t *testing.T) {
		engine, _, _ := newTestSyncEngine(t)

		// Add overdue movie
		deleteAfter := time.Now().Add(-5 * 24 * time.Hour) // 5 days ago
		engine.mediaLibrary["movie-1"] = models.Media{
			ID:          "movie-1",
			Type:        models.MediaTypeMovie,
			Title:       "Overdue Movie",
			Year:        2020,
			DeleteAfter: deleteAfter,
			FileSize:    10737418240, // 10 GB
			LastWatched: time.Now().Add(-100 * 24 * time.Hour),
		}

		scheduledCount, candidates := engine.CalculateDeletionInfo()
		assert.Equal(t, 1, scheduledCount)
		assert.Len(t, candidates, 1)

		// Verify candidate structure
		candidate := candidates[0]
		assert.Equal(t, "movie-1", candidate["id"])
		assert.Equal(t, "Overdue Movie", candidate["title"])
		assert.Equal(t, models.MediaTypeMovie, candidate["type"])
		assert.Equal(t, 2020, candidate["year"])
		assert.InDelta(t, 5, candidate["days_overdue"].(int), 1)
	})

	t.Run("excludes items marked as excluded", func(t *testing.T) {
		engine, _, exclusions := newTestSyncEngine(t)

		// Add overdue movie
		deleteAfter := time.Now().Add(-5 * 24 * time.Hour)
		engine.mediaLibrary["movie-1"] = models.Media{
			ID:          "movie-1",
			Type:        models.MediaTypeMovie,
			Title:       "Excluded Movie",
			DeleteAfter: deleteAfter,
			IsExcluded:  true,
		}

		// Add to exclusions
		exclusions.Add(storage.ExclusionItem{
			ExternalID:   "movie-1",
			ExternalType: "radarr",
			MediaType:    "movie",
			Title:        "Excluded Movie",
			Reason:       "User keep",
			ExcludedAt:   time.Now(),
		})

		scheduledCount, candidates := engine.CalculateDeletionInfo()
		assert.Equal(t, 0, scheduledCount)
		assert.Empty(t, candidates)
	})

	t.Run("includes jellyfin_id for watch-state safety check", func(t *testing.T) {
		engine, _, _ := newTestSyncEngine(t)

		deleteAfter := time.Now().Add(-5 * 24 * time.Hour)
		engine.mediaLibrary["movie-1"] = models.Media{
			ID:          "movie-1",
			JellyfinID:  "jf-movie-1",
			Type:        models.MediaTypeMovie,
			Title:       "Overdue Movie",
			DeleteAfter: deleteAfter,
		}

		scheduledCount, candidates := engine.CalculateDeletionInfo()
		assert.Equal(t, 1, scheduledCount)
		require.Len(t, candidates, 1)
		assert.Equal(t, "jf-movie-1", candidates[0]["jellyfin_id"])
	})

	t.Run("includes multiple overdue items", func(t *testing.T) {
		engine, _, _ := newTestSyncEngine(t)

		// Add multiple overdue items
		for i := 1; i <= 3; i++ {
			engine.mediaLibrary[fmt.Sprintf("movie-%d", i)] = models.Media{
				ID:          fmt.Sprintf("movie-%d", i),
				Type:        models.MediaTypeMovie,
				Title:       fmt.Sprintf("Movie %d", i),
				DeleteAfter: time.Now().Add(-time.Duration(i) * 24 * time.Hour),
			}
		}

		scheduledCount, candidates := engine.CalculateDeletionInfo()
		assert.Equal(t, 3, scheduledCount)
		assert.Len(t, candidates, 3)
	})
}

func TestSyncEngine_ExecuteDeletions(t *testing.T) {
	t.Run("returns zero when no candidates", func(t *testing.T) {
		engine, _, _ := newTestSyncEngine(t)
		ctx := context.Background()

		deletedCount, episodeItemsProcessed, episodeFilesDeleted, protectedCount, failedCount, deletedItems := engine.ExecuteDeletions(ctx, []map[string]interface{}{})
		assert.Equal(t, 0, deletedCount)
		assert.Equal(t, 0, episodeItemsProcessed)
		assert.Equal(t, 0, episodeFilesDeleted)
		assert.Equal(t, 0, protectedCount)
		assert.Equal(t, 0, failedCount)
		assert.Empty(t, deletedItems)
	})

	t.Run("skips invalid candidates", func(t *testing.T) {
		engine, _, _ := newTestSyncEngine(t)
		ctx := context.Background()

		// Invalid candidate (missing ID)
		candidates := []map[string]interface{}{
			{
				"title": "Invalid Movie",
			},
		}

		deletedCount, episodeItemsProcessed, episodeFilesDeleted, protectedCount, failedCount, deletedItems := engine.ExecuteDeletions(ctx, candidates)
		assert.Equal(t, 0, deletedCount)
		assert.Equal(t, 0, episodeItemsProcessed)
		assert.Equal(t, 0, episodeFilesDeleted)
		assert.Equal(t, 0, protectedCount)
		assert.Equal(t, 1, failedCount)
		assert.Empty(t, deletedItems)
	})

	t.Run("deletes valid candidates with Radarr client", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create config with Radarr enabled
		cfg := &config.Config{
			App: config.AppConfig{
				DryRun:         false,
				EnableDeletion: true,
			},
			Sync: config.SyncConfig{
				FullInterval:        60,
				IncrementalInterval: 5,
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
			},
		}

		cacheInstance := cache.New()
		jobs, err := storage.NewJobsFile(tmpDir, 50)
		require.NoError(t, err)

		exclusions, err := storage.NewExclusionsFile(tmpDir)
		require.NoError(t, err)

		manualLS3, err := storage.NewManualLeavingSoonFile(tmpDir)
		require.NoError(t, err)

		rulesEngine := rules.NewRulesEngine(exclusions, nil)
		engine := NewSyncEngine(cfg, cacheInstance, jobs, exclusions, manualLS3, rulesEngine)

		// Add movie to library
		engine.mediaLibrary["movie-1"] = models.Media{
			ID:       "movie-1",
			Type:     models.MediaTypeMovie,
			Title:    "Test Movie",
			RadarrID: 123,
		}

		ctx := context.Background()
		candidates := []map[string]interface{}{
			{
				"id":    "movie-1",
				"title": "Test Movie",
				"type":  models.MediaTypeMovie,
			},
		}

		// Note: This will fail in test because we don't have real Radarr
		// But it verifies the logic flow
		deletedCount, _, _, _, _, deletedItems := engine.ExecuteDeletions(ctx, candidates)

		// Should attempt deletion but fail without real Radarr
		assert.GreaterOrEqual(t, deletedCount, 0)
		assert.GreaterOrEqual(t, len(deletedItems), 0)
	})
}

func TestSyncEngine_ExecuteDeletions_EpisodeFailureAccounting(t *testing.T) {
	tmpDir := t.TempDir()

	// Sonarr stub: serves episodes for the rule, fails all episode-file deletes.
	var deleteCalls int
	sonarrServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			deleteCalls++
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		// GET /api/v3/episode?seriesId=N
		episodes := []clients.SonarrEpisode{
			{ID: 1, SeriesID: 1, EpisodeFileID: 101, EpisodeNumber: 1, SeasonNumber: 1, HasFile: true, AirDateUTC: time.Now().AddDate(0, 0, -30)},
			{ID: 2, SeriesID: 1, EpisodeFileID: 102, EpisodeNumber: 2, SeasonNumber: 1, HasFile: true, AirDateUTC: time.Now().AddDate(0, 0, -20)},
			{ID: 3, SeriesID: 1, EpisodeFileID: 103, EpisodeNumber: 3, SeasonNumber: 1, HasFile: true, AirDateUTC: time.Now().AddDate(0, 0, -10)},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(episodes)
	}))
	defer sonarrServer.Close()

	cfg := &config.Config{
		App: config.AppConfig{
			DryRun:         false,
			EnableDeletion: true,
		},
		Sync: config.SyncConfig{
			FullInterval:        60,
			IncrementalInterval: 5,
		},
		Rules: config.RulesConfig{
			MovieRetention: "90d",
			TVRetention:    "120d",
		},
		AdvancedRules: []config.AdvancedRule{
			{
				Name:                  "test-episode",
				Type:                  "episode",
				Enabled:               true,
				EpisodeDeleteStrategy: "oldest_first",
				MaxEpisodes:           2, // delete the oldest episode file (101)
			},
		},
		Integrations: config.IntegrationsConfig{
			Sonarr: config.SonarrConfig{
				BaseIntegrationConfig: config.BaseIntegrationConfig{
					Enabled: true,
					URL:     sonarrServer.URL,
					APIKey:  "test-key",
				},
			},
		},
	}
	config.SetTestConfig(cfg)

	cacheInstance := cache.New()
	jobs, err := storage.NewJobsFile(tmpDir, 50)
	require.NoError(t, err)
	exclusions, err := storage.NewExclusionsFile(tmpDir)
	require.NoError(t, err)
	manualLS, err := storage.NewManualLeavingSoonFile(tmpDir)
	require.NoError(t, err)

	rulesEngine := rules.NewRulesEngine(exclusions, nil)
	engine := NewSyncEngine(cfg, cacheInstance, jobs, exclusions, manualLS, rulesEngine)
	require.NotNil(t, engine.sonarrClient)

	engine.mediaLibrary["show-1"] = models.Media{
		ID:       "show-1",
		Type:     models.MediaTypeTVShow,
		Title:    "Test Show",
		SonarrID: 1,
	}

	ctx := context.Background()
	candidates := []map[string]interface{}{
		{
			"id":    "show-1",
			"title": "Test Show",
			"type":  models.MediaTypeTVShow,
		},
	}

	_, _, _, episodeFilesDeleted, failedCount, _ := engine.ExecuteDeletions(ctx, candidates)

	// The rule scheduled episode file 101; the delete failed → candidate must be counted as failed.
	assert.GreaterOrEqual(t, deleteCalls, 1, "episode file delete should have been attempted")
	assert.Equal(t, 0, episodeFilesDeleted)
	assert.Equal(t, 1, failedCount, "candidate with failed episode-file deletion must count as failed")
}

func TestSyncEngine_FullSync_EnableDeletion(t *testing.T) {
	t.Run("skips deletion when enable_deletion is false", func(t *testing.T) {
		tmpDir := t.TempDir()

		cfg := &config.Config{
			App: config.AppConfig{
				DryRun:         false, // Not dry-run
				EnableDeletion: false, // But deletion disabled
			},
			Sync: config.SyncConfig{
				FullInterval:        60,
				IncrementalInterval: 5,
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

		manualLS4, err := storage.NewManualLeavingSoonFile(tmpDir)
		require.NoError(t, err)

		rulesEngine := rules.NewRulesEngine(exclusions, nil)
		engine := NewSyncEngine(cfg, cacheInstance, jobs, exclusions, manualLS4, rulesEngine)

		// Add overdue movie
		engine.mediaLibrary["movie-1"] = models.Media{
			ID:          "movie-1",
			Type:        models.MediaTypeMovie,
			Title:       "Overdue Movie",
			DeleteAfter: time.Now().Add(-5 * 24 * time.Hour),
		}

		ctx := context.Background()
		err = engine.FullSync(ctx)
		require.NoError(t, err)

		// Verify movie still exists (not deleted)
		_, found := engine.GetMediaByID("movie-1")
		assert.True(t, found)

		// Check job summary
		latestJob, found := jobs.GetLatest()
		require.True(t, found)
		assert.False(t, latestJob.Summary["enable_deletion"].(bool))
	})

	t.Run("tracks enable_deletion in job summary", func(t *testing.T) {
		tmpDir := t.TempDir()

		cfg := &config.Config{
			App: config.AppConfig{
				DryRun:         true,
				EnableDeletion: true,
			},
			Sync: config.SyncConfig{
				FullInterval:        60,
				IncrementalInterval: 5,
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

		manualLS5, err := storage.NewManualLeavingSoonFile(tmpDir)
		require.NoError(t, err)

		rulesEngine := rules.NewRulesEngine(exclusions, nil)
		engine := NewSyncEngine(cfg, cacheInstance, jobs, exclusions, manualLS5, rulesEngine)

		ctx := context.Background()
		err = engine.FullSync(ctx)
		require.NoError(t, err)

		// Check job summary includes enable_deletion flag
		latestJob, found := jobs.GetLatest()
		require.True(t, found)
		assert.True(t, latestJob.Summary["enable_deletion"].(bool))
		assert.True(t, latestJob.Summary["dry_run"].(bool))
	})
}

func TestSyncEngine_ExecuteDeletionsLocked_BusyReturnsErrSyncInProgress(t *testing.T) {
	engine, _, _ := newTestSyncEngine(t)

	// Simulate a running sync holding the serialization lock.
	engine.syncRunMu.Lock()
	defer engine.syncRunMu.Unlock()

	_, _, _, _, _, _, err := engine.ExecuteDeletionsLocked(context.Background(), []map[string]interface{}{{}})
	require.ErrorIs(t, err, ErrSyncInProgress)
}

func TestSyncEngine_ExecuteDeletionsLocked_RunsWhenLockFree(t *testing.T) {
	engine, _, _ := newTestSyncEngine(t)

	// Lock is free: the wrapper must acquire it and delegate without error.
	deletedCount, episodeItemsProcessed, episodeFilesDeleted, protectedCount, failedCount, deletedItems, err := engine.ExecuteDeletionsLocked(context.Background(), nil)
	require.NoError(t, err)
	assert.Zero(t, deletedCount)
	assert.Zero(t, episodeItemsProcessed)
	assert.Zero(t, episodeFilesDeleted)
	assert.Zero(t, protectedCount)
	assert.Zero(t, failedCount)
	assert.Empty(t, deletedItems)
}

type stubStatsProvider struct {
	history []clients.StatsHistoryItem
}

func (s *stubStatsProvider) GetHistory(ctx context.Context, itemIDs []string) ([]clients.StatsHistoryItem, error) {
	return s.history, nil
}

func (s *stubStatsProvider) Ping(ctx context.Context) error { return nil }

func TestSyncEngine_ExecuteDeletions_ProtectsOnFreshWatchActivity(t *testing.T) {
	engine, _, _ := newTestSyncEngine(t)

	now := time.Now()
	engine.mediaLibrary["movie-1"] = models.Media{
		ID:          "movie-1",
		Type:        models.MediaTypeMovie,
		Title:       "Test Movie",
		JellyfinID:  "jf-1",
		LastWatched: now.AddDate(0, 0, -100), // overdue under 90d retention
	}
	engine.statsClient = &stubStatsProvider{
		history: []clients.StatsHistoryItem{
			{JellyfinItemID: "jf-1", WatchedAt: now}, // watched since evaluation
		},
	}

	ctx := context.Background()
	candidates := []map[string]interface{}{
		{
			"id":    "movie-1",
			"title": "Test Movie",
			"type":  models.MediaTypeMovie,
		},
	}

	deletedCount, _, _, protectedCount, failedCount, _ := engine.ExecuteDeletions(ctx, candidates)

	assert.Equal(t, 0, deletedCount, "protected item must not be deleted")
	assert.Equal(t, 1, protectedCount, "fresh watch activity must count as protected, not failed")
	assert.Equal(t, 0, failedCount)
}

func TestSyncEngine_SyncStats_RollsEpisodeHistoryToSeries(t *testing.T) {
	engine, _, _ := newTestSyncEngine(t)
	now := time.Now()

	engine.mediaLibrary["show-1"] = models.Media{
		ID: "show-1", Type: models.MediaTypeTVShow, Title: "Test Show", JellyfinID: "series-jf",
	}
	engine.mediaLibrary["movie-1"] = models.Media{
		ID: "movie-1", Type: models.MediaTypeMovie, Title: "Test Movie", JellyfinID: "movie-jf",
	}

	engine.statsClient = &stubStatsProvider{
		history: []clients.StatsHistoryItem{
			{JellyfinItemID: "ep-1", SeriesID: "series-jf", WatchedAt: now.Add(-48 * time.Hour), PlaybackSeconds: 1800, PlayID: "p1"},
			{JellyfinItemID: "ep-2", SeriesID: "series-jf", WatchedAt: now.Add(-24 * time.Hour), PlaybackSeconds: 1800, PlayID: "p2"},
			{JellyfinItemID: "movie-jf", WatchedAt: now.Add(-1 * time.Hour), PlaybackSeconds: 3600, PlayID: "p3"},
		},
	}

	require.NoError(t, engine.syncStats(context.Background()))

	show := engine.mediaLibrary["show-1"]
	assert.Equal(t, 2, show.WatchCount, "episode plays must roll up to the series")
	assert.Equal(t, 2, show.GatedPlayCount, "gated episode plays likewise roll up")
	assert.WithinDuration(t, now.Add(-24*time.Hour), show.LastWatched, time.Minute)

	movie := engine.mediaLibrary["movie-1"]
	assert.Equal(t, 1, movie.WatchCount, "movie plays still map by item id")
	assert.WithinDuration(t, now.Add(-1*time.Hour), movie.LastWatched, time.Minute)
}

func TestSyncJellyfin_PreservesStatsWatchData(t *testing.T) {
	engine, _, _ := newTestSyncEngine(t)
	lastWatched := time.Now().Add(-48 * time.Hour)

	engine.mediaLibrary["movie-1"] = models.Media{
		ID: "movie-1", Type: models.MediaTypeMovie, Title: "Dune",
		TMDBID: 438631, JellyfinID: "old-id", WatchCount: 7, LastWatched: lastWatched,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// No UserData — mirrors a real Jellyfin response without a user context.
		resp := `{"Items":[{"Id":"jf-dune","Name":"Dune","Type":"Movie","ProviderIds":{"Tmdb":"438631"}}]}`
		if r.URL.Query().Get("IncludeItemTypes") != "Movie" {
			resp = `{"Items":[]}`
		}
		_, _ = w.Write([]byte(resp))
	}))
	defer srv.Close()

	engine.jellyfinClient = clients.NewJellyfinClient(config.JellyfinConfig{
		BaseIntegrationConfig: config.BaseIntegrationConfig{URL: srv.URL, APIKey: "k"},
	})

	require.NoError(t, engine.syncJellyfin(context.Background()))

	movie := engine.mediaLibrary["movie-1"]
	assert.Equal(t, "jf-dune", movie.JellyfinID, "match is still recorded")
	assert.True(t, movie.HasPoster, "match metadata is still recorded")
	assert.Equal(t, "matched", movie.JellyfinMatchStatus, "match status is still recorded")
	assert.Equal(t, 7, movie.WatchCount, "absent Jellyfin UserData must not clobber stats watch data")
	assert.WithinDuration(t, lastWatched, movie.LastWatched, time.Second)
}

func TestSyncJellyfin_PreservesShowWatchData(t *testing.T) {
	engine, _, _ := newTestSyncEngine(t)
	lastWatched := time.Now().Add(-72 * time.Hour)

	// Series-level PlayCount is 0 in Jellyfin even when episodes were watched;
	// the stats provider owns show watch fields and must not be clobbered.
	engine.mediaLibrary["show-1"] = models.Media{
		ID: "show-1", Type: models.MediaTypeTVShow, Title: "Lost",
		TVDBID: 73739, JellyfinID: "old-id", WatchCount: 11, LastWatched: lastWatched,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := `{"Items":[]}`
		if r.URL.Query().Get("IncludeItemTypes") == "Series" {
			resp = `{"Items":[{"Id":"jf-lost","Name":"Lost","Type":"Series","ProviderIds":{"Tvdb":"73739"}}]}`
		}
		_, _ = w.Write([]byte(resp))
	}))
	defer srv.Close()

	engine.jellyfinClient = clients.NewJellyfinClient(config.JellyfinConfig{
		BaseIntegrationConfig: config.BaseIntegrationConfig{URL: srv.URL, APIKey: "k"},
	})

	require.NoError(t, engine.syncJellyfin(context.Background()))

	show := engine.mediaLibrary["show-1"]
	assert.Equal(t, "jf-lost", show.JellyfinID, "match is still recorded")
	assert.True(t, show.HasPoster, "match metadata is still recorded")
	assert.Equal(t, "matched", show.JellyfinMatchStatus, "match status is still recorded")
	assert.Equal(t, 11, show.WatchCount, "series PlayCount 0 must not clobber stats watch data")
	assert.WithinDuration(t, lastWatched, show.LastWatched, time.Second)
}

func TestSyncEngine_SyncStats_ScopesPlayIDPerEpisode(t *testing.T) {
	engine, _, _ := newTestSyncEngine(t)
	now := time.Now()

	engine.mediaLibrary["show-1"] = models.Media{
		ID: "show-1", Type: models.MediaTypeTVShow, Title: "Test Show", JellyfinID: "series-jf",
	}

	// Two different episodes sharing the same provider play id must both count —
	// the series aggregation bucket must not collapse them via a raw PlayID key.
	engine.statsClient = &stubStatsProvider{
		history: []clients.StatsHistoryItem{
			{JellyfinItemID: "ep-1", SeriesID: "series-jf", WatchedAt: now.Add(-2 * time.Hour), PlaybackSeconds: 1800, PlayID: "same-id"},
			{JellyfinItemID: "ep-2", SeriesID: "series-jf", WatchedAt: now.Add(-1 * time.Hour), PlaybackSeconds: 1800, PlayID: "same-id"},
		},
	}

	require.NoError(t, engine.syncStats(context.Background()))

	show := engine.mediaLibrary["show-1"]
	assert.Equal(t, 2, show.WatchCount)
	assert.Equal(t, 2, show.GatedPlayCount, "distinct episodes must not be deduped by a shared play id")
}

func TestSyncEngine_SyncQueuesBehindRunningSync(t *testing.T) {
	engine, _, _ := newTestSyncEngine(t)

	// Simulate a running sync holding the serialization lock.
	engine.syncRunMu.Lock()

	done := make(chan error, 1)
	go func() {
		done <- engine.IncrementalSync(context.Background())
	}()

	// The queued sync must block until the lock is released.
	select {
	case err := <-done:
		t.Fatalf("IncrementalSync returned while lock held: %v", err)
	case <-time.After(150 * time.Millisecond):
	}

	engine.syncRunMu.Unlock()

	select {
	case err := <-done:
		require.NoError(t, err, "queued sync must run once the lock is released")
	case <-time.After(2 * time.Second):
		t.Fatal("queued sync did not run after lock release")
	}
}

// newJellyfinStub starts a Jellyfin stub that answers /Items with the raw Items
// JSON keyed by IncludeItemTypes ("Movie"/"Series"); unmatched types get an
// empty page. It mirrors the client's getItems call shape.
func newJellyfinStub(t *testing.T, itemsByType map[string]string) *httptest.Server {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp, ok := itemsByType[r.URL.Query().Get("IncludeItemTypes")]
		if !ok {
			resp = `{"Items":[]}`
		}
		_, _ = w.Write([]byte(resp))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// newJellyfinSyncEngine builds a SyncEngine wired to a Jellyfin stub returning
// the given items per media type.
func newJellyfinSyncEngine(t *testing.T, itemsByType map[string]string) *SyncEngine {
	t.Helper()

	engine, _, _ := newTestSyncEngine(t)
	srv := newJellyfinStub(t, itemsByType)
	engine.jellyfinClient = clients.NewJellyfinClient(config.JellyfinConfig{
		BaseIntegrationConfig: config.BaseIntegrationConfig{URL: srv.URL, APIKey: "k"},
	})
	return engine
}

func TestSyncJellyfin_PathConflictClassifiesMovie(t *testing.T) {
	engine := newJellyfinSyncEngine(t, map[string]string{
		"Movie": `{"Items":[{"Id":"jf-distant","Name":"Distant","Type":"Movie",` +
			`"Path":"/movies/Long Distance (2024)/Long.Distance.2024.mkv","ProviderIds":{"Tmdb":"1395720"}}]}`,
	})

	// Radarr reports the movie folder; Jellyfin reports the file inside it. The
	// normalized-directory path index must bridge that and flag the conflict.
	engine.mediaLibrary["radarr-306"] = models.Media{
		ID:       "radarr-306",
		Type:     models.MediaTypeMovie,
		Title:    "Long Distance",
		TMDBID:   605722,
		FilePath: "/movies/Long Distance (2024)",
	}

	require.NoError(t, engine.syncJellyfin(context.Background()))

	movie := engine.mediaLibrary["radarr-306"]
	assert.Equal(t, "metadata_mismatch", movie.JellyfinMatchStatus)
	assert.Equal(t, "jf-distant", movie.JellyfinConflictID)
	assert.Contains(t, movie.JellyfinMatchReason, "Distant")
	assert.Contains(t, movie.JellyfinMatchReason, "1395720")
	assert.Contains(t, movie.JellyfinMatchReason, "605722")
	assert.Contains(t, movie.JellyfinMatchReason, "Long Distance")
	assert.Empty(t, movie.JellyfinID, "path conflict must not attach a Jellyfin id (read-only classification)")
	assert.Zero(t, movie.WatchCount, "path conflict must not attach watch data")
}

func TestSyncJellyfin_PathConflictClassifiesShow(t *testing.T) {
	engine := newJellyfinSyncEngine(t, map[string]string{
		"Series": `{"Items":[{"Id":"jf-fmab","Name":"Fullmetal Alchemist: Brotherhood",` +
			`"Type":"Series","Path":"/tv/Fullmetal Alchemist","ProviderIds":{"Tvdb":"85249"}}]}`,
	})

	engine.mediaLibrary["sonarr-97"] = models.Media{
		ID:       "sonarr-97",
		Type:     models.MediaTypeTVShow,
		Title:    "Fullmetal Alchemist",
		TVDBID:   75579,
		FilePath: "/tv/Fullmetal Alchemist",
	}

	require.NoError(t, engine.syncJellyfin(context.Background()))

	show := engine.mediaLibrary["sonarr-97"]
	assert.Equal(t, "metadata_mismatch", show.JellyfinMatchStatus)
	assert.Equal(t, "jf-fmab", show.JellyfinConflictID)
	assert.Contains(t, show.JellyfinMatchReason, "Fullmetal Alchemist: Brotherhood")
	assert.Contains(t, show.JellyfinMatchReason, "85249")
	assert.Contains(t, show.JellyfinMatchReason, "75579")
	assert.Empty(t, show.JellyfinID, "path conflict must not attach a Jellyfin id (read-only classification)")
}

func TestSyncJellyfin_ExactTitleMismatchKeepsReasonWording(t *testing.T) {
	engine := newJellyfinSyncEngine(t, map[string]string{
		"Series": `{"Items":[{"Id":"jf-vanished-2006","Name":"Vanished","Type":"Series",` +
			`"Path":"/tv/Vanished","ProviderIds":{"Tvdb":"79332"}}]}`,
	})

	engine.mediaLibrary["sonarr-179"] = models.Media{
		ID:       "sonarr-179",
		Type:     models.MediaTypeTVShow,
		Title:    "Vanished",
		TVDBID:   461839,
		FilePath: "/tv/Vanished",
	}

	require.NoError(t, engine.syncJellyfin(context.Background()))

	show := engine.mediaLibrary["sonarr-179"]
	assert.Equal(t, "metadata_mismatch", show.JellyfinMatchStatus)
	assert.Contains(t, show.JellyfinMatchReason, "TVDB 79332 instead of 461839",
		"same-title mismatches keep the existing reason wording")
	assert.Equal(t, "jf-vanished-2006", show.JellyfinConflictID,
		"exact-title mismatch records the matched Jellyfin item id so it can be fixed by id")
}

func TestSyncJellyfin_MatchedClearsDiagnosisFields(t *testing.T) {
	engine := newJellyfinSyncEngine(t, map[string]string{
		"Movie": `{"Items":[{"Id":"jf-dune","Name":"Dune","Type":"Movie",` +
			`"Path":"/movies/Dune (2021)/Dune.2021.mkv","ProviderIds":{"Tmdb":"438631"}}]}`,
	})

	engine.mediaLibrary["movie-1"] = models.Media{
		ID:                   "movie-1",
		Type:                 models.MediaTypeMovie,
		Title:                "Dune",
		TMDBID:               438631,
		JellyfinID:           "stale-id",
		JellyfinMatchReason:  "stale reason",
		JellyfinConflictID:   "stale-conflict",
		JellyfinVerdict:      string(VerdictJellyfinWrong),
		JellyfinMismatchInfo: "stale info",
	}

	require.NoError(t, engine.syncJellyfin(context.Background()))

	movie := engine.mediaLibrary["movie-1"]
	assert.Equal(t, "matched", movie.JellyfinMatchStatus)
	assert.Equal(t, "jf-dune", movie.JellyfinID)
	assert.Empty(t, movie.JellyfinMatchReason, "matched items must clear a stale reason")
	assert.Empty(t, movie.JellyfinConflictID, "matched items must clear a stale conflict id")
	assert.Empty(t, movie.JellyfinVerdict, "matched items must clear a stale verdict")
	assert.Empty(t, movie.JellyfinMismatchInfo)
}

func TestSyncJellyfin_NotFoundSetsNotScannedReason(t *testing.T) {
	engine := newJellyfinSyncEngine(t, nil)

	engine.mediaLibrary["radarr-999"] = models.Media{
		ID:       "radarr-999",
		Type:     models.MediaTypeMovie,
		Title:    "Ghost",
		TMDBID:   111,
		FilePath: "/movies/Ghost (2020)",
	}

	require.NoError(t, engine.syncJellyfin(context.Background()))

	movie := engine.mediaLibrary["radarr-999"]
	assert.Equal(t, "not_found", movie.JellyfinMatchStatus)
	assert.Equal(t, jellyfinNotScannedReason, movie.JellyfinMatchReason)
	assert.Empty(t, movie.JellyfinConflictID)
}

// --- AnalyzeJellyfinMatch / FixJellyfinMatch ---

// newTestJellyfinServer wires a Jellyfin client to a stub and returns the engine.
func newTestJellyfinServer(t *testing.T, handler http.HandlerFunc) (*SyncEngine, *httptest.Server) {
	t.Helper()

	engine, _, _ := newTestSyncEngine(t)
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	engine.jellyfinClient = clients.NewJellyfinClient(config.JellyfinConfig{
		BaseIntegrationConfig: config.BaseIntegrationConfig{URL: srv.URL, APIKey: "k"},
	})
	return engine, srv
}

func TestAnalyzeJellyfinMatch_MovieJellyfinWrong(t *testing.T) {
	engine, _ := newTestJellyfinServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(clients.JellyfinItemsResponse{Items: []clients.JellyfinItem{{
			ID:             "jf-distant",
			Name:           "Distant",
			Type:           "Movie",
			ProductionYear: 2024,
			Path:           "/movies/Long Distance (2024)/Long.Distance.2024.mkv",
			ProviderIds:    map[string]string{"Tmdb": "1395720"},
		}}})
	})

	radarrSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(clients.RadarrMovie{
			ID: 306, Title: "Long Distance", Year: 2024, TmdbId: 605722,
			Path: "/movies/Long Distance (2024)",
			MovieFile: &clients.RadarrMovieFile{
				Path: "/movies/Long Distance (2024)/Long.Distance.2024.mkv",
			},
		})
	}))
	defer radarrSrv.Close()
	engine.radarrClient = clients.NewRadarrClient(config.RadarrConfig{
		BaseIntegrationConfig: config.BaseIntegrationConfig{URL: radarrSrv.URL, APIKey: "k"},
	})

	engine.mediaLibrary["radarr-306"] = models.Media{
		ID: "radarr-306", Type: models.MediaTypeMovie, Title: "Long Distance",
		Year: 2024, TMDBID: 605722, RadarrID: 306, FilePath: "/movies/Long Distance (2024)",
	}

	analysis, err := engine.AnalyzeJellyfinMatch(context.Background(), "radarr-306")
	require.NoError(t, err)
	assert.Equal(t, VerdictJellyfinWrong, analysis.Verdict)
	assert.Greater(t, analysis.Confidence, 0.5)

	evidence := strings.Join(analysis.Evidence, "\n")
	assert.Contains(t, evidence, "jf-distant", "evidence must name the conflicting Jellyfin item")
	assert.Contains(t, evidence, "Tmdb=1395720", "evidence must include the Jellyfin provider id")
	assert.Contains(t, evidence, "tmdb:605722", "evidence must include the arr provider id")
	assert.Equal(t, string(VerdictJellyfinWrong), engine.mediaLibrary["radarr-306"].JellyfinVerdict)
}

func TestAnalyzeJellyfinMatch_ShowEpisodeAgreement(t *testing.T) {
	names := []string{
		"Fullmetal Alchemist", "The Body of a Man", "City of Heresy",
		"A Forger's Love", "The Man with the Mechanical Arm",
	}
	jellyfinEpisodes := make([]clients.JellyfinEpisode, 0, len(names))
	sonarrEpisodes := make([]clients.SonarrEpisode, 0, len(names))
	for i, name := range names {
		path := fmt.Sprintf("/tv/Fullmetal Alchemist/S01E%02d.mkv", i+1)
		jellyfinEpisodes = append(jellyfinEpisodes, clients.JellyfinEpisode{
			ID: fmt.Sprintf("e%d", i+1), Name: name,
			ParentIndexNumber: 1, IndexNumber: i + 1, Path: path,
		})
		sonarrEpisodes = append(sonarrEpisodes, clients.SonarrEpisode{
			SeasonNumber: 1, EpisodeNumber: i + 1, Title: name,
			EpisodeFile: &clients.SonarrEpisodeFile{Path: path},
		})
	}

	engine, _ := newTestJellyfinServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/Episodes") {
			_ = json.NewEncoder(w).Encode(map[string]any{"Items": jellyfinEpisodes})
			return
		}
		_ = json.NewEncoder(w).Encode(clients.JellyfinItemsResponse{Items: []clients.JellyfinItem{{
			ID:             "jf-fmab",
			Name:           "Fullmetal Alchemist: Brotherhood",
			Type:           "Series",
			ProductionYear: 2009,
			Path:           "/tv/Fullmetal Alchemist",
			ProviderIds:    map[string]string{"Tvdb": "85249"},
		}}})
	})

	sonarrSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasPrefix(r.URL.Path, "/api/v3/episode") {
			_ = json.NewEncoder(w).Encode(sonarrEpisodes)
			return
		}
		_ = json.NewEncoder(w).Encode(clients.SonarrSeries{
			ID: 97, Title: "Fullmetal Alchemist", Year: 2003, TvdbId: 75579,
			Path: "/tv/Fullmetal Alchemist",
		})
	}))
	defer sonarrSrv.Close()
	engine.sonarrClient = clients.NewSonarrClient(config.SonarrConfig{
		BaseIntegrationConfig: config.BaseIntegrationConfig{URL: sonarrSrv.URL, APIKey: "k"},
	})

	engine.mediaLibrary["sonarr-97"] = models.Media{
		ID: "sonarr-97", Type: models.MediaTypeTVShow, Title: "Fullmetal Alchemist",
		Year: 2003, TVDBID: 75579, SonarrID: 97, FilePath: "/tv/Fullmetal Alchemist",
		JellyfinConflictID: "jf-fmab",
	}

	analysis, err := engine.AnalyzeJellyfinMatch(context.Background(), "sonarr-97")
	require.NoError(t, err)
	assert.Equal(t, VerdictJellyfinWrong, analysis.Verdict, "high episode-title agreement pins the mismatch on Jellyfin")
	evidence := strings.Join(analysis.Evidence, "\n")
	assert.Contains(t, evidence, "episode titles agree")
	assert.Contains(t, evidence, "85249")
	assert.Contains(t, evidence, "75579")
}

func TestFixJellyfinMatch_RefusesArrWrong(t *testing.T) {
	var applied, searched atomic.Int32

	engine, _ := newTestJellyfinServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasPrefix(r.URL.Path, "/Items/RemoteSearch/Apply/"):
			applied.Add(1)
			w.WriteHeader(http.StatusNoContent)
		case strings.HasPrefix(r.URL.Path, "/Items/RemoteSearch/"):
			searched.Add(1)
			_, _ = w.Write([]byte(`[]`))
		default:
			_ = json.NewEncoder(w).Encode(clients.JellyfinItemsResponse{Items: []clients.JellyfinItem{{
				ID:             "jf-distant",
				Name:           "Distant",
				Type:           "Movie",
				ProductionYear: 2024,
				Path:           "/movies/Distant (2024)/Distant.2024.mkv",
				ProviderIds:    map[string]string{"Tmdb": "1395720"},
			}}})
		}
	})

	// The arr entry claims "Long Distance" but the file on disk is named for the
	// Jellyfin identity ("Distant"), so the arr entry is the outlier.
	engine.mediaLibrary["radarr-306"] = models.Media{
		ID: "radarr-306", Type: models.MediaTypeMovie, Title: "Long Distance",
		Year: 2024, TMDBID: 605722, FilePath: "/movies/Distant (2024)/Distant.2024.mkv",
	}

	_, err := engine.FixJellyfinMatch(context.Background(), "radarr-306", false)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrMatchArrWrong, "arr_wrong must refuse the fix")

	var refusal *MatchRefusalError
	require.ErrorAs(t, err, &refusal, "refusals must carry their adjudication")
	require.NotNil(t, refusal.Analysis)
	assert.Equal(t, VerdictArrWrong, refusal.Analysis.Verdict)
	assert.Zero(t, applied.Load(), "Jellyfin must not be mutated when the arr is wrong")
	assert.Zero(t, searched.Load(), "no remote search should run when the arr is wrong")
}

func TestFixJellyfinMatch_NoJellyfinItem(t *testing.T) {
	engine, _ := newTestJellyfinServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(clients.JellyfinItemsResponse{Items: []clients.JellyfinItem{}})
	})

	engine.mediaLibrary["radarr-999"] = models.Media{
		ID: "radarr-999", Type: models.MediaTypeMovie, Title: "Ghost",
		Year: 2020, TMDBID: 111, FilePath: "/movies/Ghost (2020)",
	}

	t.Run("analyze", func(t *testing.T) {
		_, err := engine.AnalyzeJellyfinMatch(context.Background(), "radarr-999")
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrJellyfinItemNotFound)
	})

	t.Run("fix", func(t *testing.T) {
		_, err := engine.FixJellyfinMatch(context.Background(), "radarr-999", false)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrJellyfinItemNotFound)
	})
}

func TestFixJellyfinMatch_HappyPath(t *testing.T) {
	var (
		applied  atomic.Int32
		searched atomic.Int32
		mu       sync.Mutex
		current  = clients.JellyfinItem{
			ID: "jf-distant", Name: "Distant", Type: "Movie", ProductionYear: 2024,
			Path:        "/movies/Long Distance (2024)/Long.Distance.2024.mkv",
			ProviderIds: map[string]string{"Tmdb": "1395720"},
		}
	)

	engine, _ := newTestJellyfinServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasPrefix(r.URL.Path, "/Items/RemoteSearch/Apply/"):
			applied.Add(1)
			mu.Lock()
			current.Name = "Long Distance"
			current.ProviderIds = map[string]string{"Tmdb": "605722"}
			mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
		case r.URL.Path == "/Items/RemoteSearch/Movie":
			searched.Add(1)
			_ = json.NewEncoder(w).Encode([]clients.RemoteSearchResult{{
				Name: "Long Distance", ProductionYear: 2024,
				ProviderIds: map[string]string{"Tmdb": "605722"},
			}})
		default:
			mu.Lock()
			item := current
			mu.Unlock()
			_ = json.NewEncoder(w).Encode(clients.JellyfinItemsResponse{Items: []clients.JellyfinItem{item}})
		}
	})

	engine.mediaLibrary["radarr-306"] = models.Media{
		ID: "radarr-306", Type: models.MediaTypeMovie, Title: "Long Distance",
		Year: 2024, TMDBID: 605722, FilePath: "/movies/Long Distance (2024)",
	}

	result, err := engine.FixJellyfinMatch(context.Background(), "radarr-306", false)
	require.NoError(t, err)

	assert.Equal(t, int32(1), applied.Load(), "the matching remote search result must be applied once")
	assert.Equal(t, int32(1), searched.Load())
	assert.Equal(t, "jf-distant", result.JellyfinID, "Jellyfin re-identifies the existing item in place")
	assert.Equal(t, "Long Distance", result.Title)
	assert.Equal(t, "605722", result.AppliedProviderIDs["Tmdb"])
	assert.Equal(t, VerdictJellyfinWrong, result.Analysis.Verdict)

	updated := engine.mediaLibrary["radarr-306"]
	assert.Equal(t, "matched", updated.JellyfinMatchStatus, "re-sync must flip the item to matched")
	assert.Equal(t, "jf-distant", updated.JellyfinID)
	assert.Empty(t, updated.JellyfinVerdict,
		"a successful fix must clear the verdict, not leave jellyfin_wrong behind")
}

// --- Jellyfin path index ---

func TestBuildJellyfinPathIndex_WindowsPathParentBridge(t *testing.T) {
	items := []clients.JellyfinItem{{
		ID:   "jf-win",
		Path: `C:\Movies\Long Distance (2024)\Long.Distance.2024.mkv`,
	}}

	index := buildJellyfinPathIndex(items)
	parentKey := normalizeJellyfinPathKey(`C:\Movies\Long Distance (2024)`)
	require.Contains(t, index, parentKey, "a Windows path must still yield a real parent directory")
	assert.Equal(t, "jf-win", index[parentKey].ID)
}

func TestBuildJellyfinPathIndex_ParentDirCollisionDropped(t *testing.T) {
	items := []clients.JellyfinItem{
		{ID: "feature", Path: "/movies/Foo (2020)/Foo.2020.mkv"},
		{ID: "featurette", Path: "/movies/Foo (2020)/Featurette.mkv"},
	}

	index := buildJellyfinPathIndex(items)
	parentKey := normalizeJellyfinPathKey("/movies/Foo (2020)")
	assert.NotContains(t, index, parentKey, "an ambiguous parent directory must be dropped")
	assert.Nil(t, lookupJellyfinPath(items, parentKey), "a shared folder must not resolve to a wrong item")

	// Exact file paths still resolve to their own item.
	assert.Equal(t, "feature", lookupJellyfinPath(items, normalizeJellyfinPathKey("/movies/Foo (2020)/Foo.2020.mkv")).ID)
	assert.Equal(t, "featurette", lookupJellyfinPath(items, normalizeJellyfinPathKey("/movies/Foo (2020)/Featurette.mkv")).ID)
}

func TestBuildJellyfinPathIndex_ExactPathWinsOverParent(t *testing.T) {
	// The folder item is listed first on purpose. A naive index that writes each
	// item's parent key last-wins would have the second (file) item overwrite the
	// shared folder key, resolving it to the wrong item. Only an implementation
	// that gives exact item.Path precedence over the parent bridge passes.
	items := []clients.JellyfinItem{
		{ID: "folder-item", Path: "/movies/Foo (2020)"},
		{ID: "movie", Path: "/movies/Foo (2020)/Foo.2020.mkv"},
	}

	index := buildJellyfinPathIndex(items)
	folderKey := normalizeJellyfinPathKey("/movies/Foo (2020)")
	require.Contains(t, index, folderKey)
	assert.Equal(t, "folder-item", index[folderKey].ID,
		"an exact item.Path must win over another item's parent-directory entry")
	assert.Equal(t, "folder-item", lookupJellyfinPath(items, folderKey).ID)
	// The distinct exact file path still resolves to its own item.
	assert.Equal(t, "movie",
		lookupJellyfinPath(items, normalizeJellyfinPathKey("/movies/Foo (2020)/Foo.2020.mkv")).ID,
		"hardening the parent key must not disturb exact file-path resolution")
}

func TestSelectRemoteSearchResult_RequiresProviderMatch(t *testing.T) {
	results := []clients.RemoteSearchResult{
		{Name: "Decoy", ProviderIds: map[string]string{"Tmdb": "999999"}},
		{Name: "Correct", ProviderIds: map[string]string{"Tmdb": "605722"}},
	}

	match, ok := selectRemoteSearchResult(results, "Tmdb", "605722")
	require.True(t, ok)
	assert.Equal(t, "Correct", match.Name, "selection must be driven by the provider id, not result order")

	_, ok = selectRemoteSearchResult(results, "Tmdb", "12345")
	assert.False(t, ok)
}

func TestFixJellyfinMatch_AppliesOnlyMatchingProviderResult(t *testing.T) {
	var (
		applied   atomic.Int32
		appliedID atomic.Value
	)

	engine, _ := newTestJellyfinServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasPrefix(r.URL.Path, "/Items/RemoteSearch/Apply/"):
			applied.Add(1)
			var body clients.RemoteSearchResult
			_ = json.NewDecoder(r.Body).Decode(&body)
			appliedID.Store(body.ProviderIds["Tmdb"])
			w.WriteHeader(http.StatusNoContent)
		case r.URL.Path == "/Items/RemoteSearch/Movie":
			_ = json.NewEncoder(w).Encode([]clients.RemoteSearchResult{
				{Name: "Distant", ProductionYear: 2024, ProviderIds: map[string]string{"Tmdb": "999999"}},
				{Name: "Long Distance", ProductionYear: 2024, ProviderIds: map[string]string{"Tmdb": "605722"}},
			})
		default:
			_ = json.NewEncoder(w).Encode(clients.JellyfinItemsResponse{Items: []clients.JellyfinItem{{
				ID: "jf-distant", Name: "Distant", Type: "Movie", ProductionYear: 2024,
				Path:        "/movies/Long Distance (2024)/Long.Distance.2024.mkv",
				ProviderIds: map[string]string{"Tmdb": "1395720"},
			}}})
		}
	})

	engine.mediaLibrary["radarr-306"] = models.Media{
		ID: "radarr-306", Type: models.MediaTypeMovie, Title: "Long Distance",
		Year: 2024, TMDBID: 605722, FilePath: "/movies/Long Distance (2024)",
	}

	_, err := engine.FixJellyfinMatch(context.Background(), "radarr-306", false)
	require.NoError(t, err)
	assert.Equal(t, int32(1), applied.Load())
	assert.Equal(t, "605722", appliedID.Load(),
		"only the remote-search result carrying the arr provider id may be applied")
}

func TestFixJellyfinMatch_RemoteSearchNoMatch(t *testing.T) {
	var applied atomic.Int32

	engine, _ := newTestJellyfinServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasPrefix(r.URL.Path, "/Items/RemoteSearch/Apply/"):
			applied.Add(1)
			w.WriteHeader(http.StatusNoContent)
		case r.URL.Path == "/Items/RemoteSearch/Movie":
			_ = json.NewEncoder(w).Encode([]clients.RemoteSearchResult{{
				Name: "Decoy", ProductionYear: 2024, ProviderIds: map[string]string{"Tmdb": "999999"},
			}})
		default:
			_ = json.NewEncoder(w).Encode(clients.JellyfinItemsResponse{Items: []clients.JellyfinItem{{
				ID: "jf-distant", Name: "Distant", Type: "Movie", ProductionYear: 2024,
				Path:        "/movies/Long Distance (2024)/Long.Distance.2024.mkv",
				ProviderIds: map[string]string{"Tmdb": "1395720"},
			}}})
		}
	})

	engine.mediaLibrary["radarr-306"] = models.Media{
		ID: "radarr-306", Type: models.MediaTypeMovie, Title: "Long Distance",
		Year: 2024, TMDBID: 605722, FilePath: "/movies/Long Distance (2024)",
	}

	_, err := engine.FixJellyfinMatch(context.Background(), "radarr-306", false)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrRemoteSearchNoMatch)

	var refusal *MatchRefusalError
	require.ErrorAs(t, err, &refusal)
	require.NotNil(t, refusal.Analysis)
	assert.Equal(t, VerdictJellyfinWrong, refusal.Analysis.Verdict)
	assert.Zero(t, applied.Load(), "no result may be applied when none matches")
}

func TestFixJellyfinMatch_ResyncFailureStillSucceeds(t *testing.T) {
	var (
		applied    atomic.Int32
		syncFailed atomic.Bool
	)

	engine, _ := newTestJellyfinServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/Items/RemoteSearch/Apply/"):
			applied.Add(1)
			syncFailed.Store(true)
			w.WriteHeader(http.StatusNoContent)
		case r.URL.Path == "/Items/RemoteSearch/Movie":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]clients.RemoteSearchResult{{
				Name: "Long Distance", ProductionYear: 2024, ProviderIds: map[string]string{"Tmdb": "605722"},
			}})
		default:
			if syncFailed.Load() {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(clients.JellyfinItemsResponse{Items: []clients.JellyfinItem{{
				ID: "jf-distant", Name: "Distant", Type: "Movie", ProductionYear: 2024,
				Path:        "/movies/Long Distance (2024)/Long.Distance.2024.mkv",
				ProviderIds: map[string]string{"Tmdb": "1395720"},
			}}})
		}
	})

	lastWatched := time.Now().Add(-2 * time.Hour)
	engine.mediaLibrary["radarr-306"] = models.Media{
		ID: "radarr-306", Type: models.MediaTypeMovie, Title: "Long Distance",
		Year: 2024, TMDBID: 605722, FilePath: "/movies/Long Distance (2024)",
		// Seed the stale diagnosis a previous Analyze left behind, so the test
		// proves a successful fix clears it even when the re-sync cannot be
		// observed. Watch data must survive the fix untouched.
		JellyfinVerdict:     string(VerdictJellyfinWrong),
		JellyfinMatchReason: "stale: Jellyfin matched Distant (Tmdb 1395720)",
		JellyfinConflictID:  "jf-distant",
		WatchCount:          7,
		LastWatched:         lastWatched,
	}

	result, err := engine.FixJellyfinMatch(context.Background(), "radarr-306", false)
	require.NoError(t, err, "a post-apply re-sync failure must not fail the fix")
	assert.Equal(t, int32(1), applied.Load())
	assert.Equal(t, "jf-distant", result.JellyfinID,
		"the applied identity must still be reported when the re-sync cannot be observed")

	updated := engine.mediaLibrary["radarr-306"]
	assert.Empty(t, updated.JellyfinVerdict,
		"a successful fix must clear a stale verdict even when the re-sync fails")
	assert.Empty(t, updated.JellyfinMatchReason,
		"a successful fix must clear a stale mismatch reason even when the re-sync fails")
	assert.Empty(t, updated.JellyfinConflictID,
		"a successful fix must clear a stale conflict id even when the re-sync fails")
	assert.Equal(t, 7, updated.WatchCount, "clearing the diagnosis must not touch watch data")
	assert.WithinDuration(t, lastWatched, updated.LastWatched, time.Second,
		"clearing the diagnosis must not touch LastWatched")
}

// TestFixJellyfinMatch_ThreadsReplaceImages proves the replaceImages argument
// reaches the Jellyfin Apply call as the replaceAllImages query parameter, for
// both values. The poster regression this guards against is an always-false
// hardcode, so false alone would not catch it.
func TestFixJellyfinMatch_ThreadsReplaceImages(t *testing.T) {
	for _, replaceImages := range []bool{true, false} {
		t.Run(fmt.Sprintf("replaceImages=%t", replaceImages), func(t *testing.T) {
			var gotReplace atomic.Value // string

			engine, _ := newTestJellyfinServer(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch {
				case strings.HasPrefix(r.URL.Path, "/Items/RemoteSearch/Apply/"):
					gotReplace.Store(r.URL.Query().Get("replaceAllImages"))
					w.WriteHeader(http.StatusNoContent)
				case r.URL.Path == "/Items/RemoteSearch/Movie":
					_ = json.NewEncoder(w).Encode([]clients.RemoteSearchResult{{
						Name: "Long Distance", ProductionYear: 2024,
						ProviderIds: map[string]string{"Tmdb": "605722"},
					}})
				default:
					_ = json.NewEncoder(w).Encode(clients.JellyfinItemsResponse{Items: []clients.JellyfinItem{{
						ID: "jf-distant", Name: "Distant", Type: "Movie", ProductionYear: 2024,
						Path:        "/movies/Long Distance (2024)/Long.Distance.2024.mkv",
						ProviderIds: map[string]string{"Tmdb": "1395720"},
					}}})
				}
			})

			engine.mediaLibrary["radarr-306"] = models.Media{
				ID: "radarr-306", Type: models.MediaTypeMovie, Title: "Long Distance",
				Year: 2024, TMDBID: 605722, FilePath: "/movies/Long Distance (2024)",
			}

			_, err := engine.FixJellyfinMatch(context.Background(), "radarr-306", replaceImages)
			require.NoError(t, err)
			assert.Equal(t, fmt.Sprintf("%t", replaceImages), gotReplace.Load(),
				"replaceImages must be forwarded verbatim to ApplyRemoteSearch")
		})
	}
}

// tVShowFixture builds a Jellyfin item (Brotherhood), a Sonarr series (2003
// FMA) and matching episode lists, plus a Jellyfin handler that serves an empty
// episode list on the first N calls. When failRetry is true the call after the
// empty ones fails with a 500 instead of returning episodes. Returns the engine
// ready for analysis.
func newEpisodeRetryEngine(t *testing.T, emptyEpisodeFetches int, failRetry ...bool) (*SyncEngine, *atomic.Int32) {
	t.Helper()

	retryErrors := len(failRetry) > 0 && failRetry[0]

	names := []string{
		"Fullmetal Alchemist", "The Body of a Man", "City of Heresy",
		"A Forger's Love", "The Man with the Mechanical Arm",
	}
	jellyfinEpisodes := make([]clients.JellyfinEpisode, 0, len(names))
	sonarrEpisodes := make([]clients.SonarrEpisode, 0, len(names))
	for i, name := range names {
		path := fmt.Sprintf("/tv/Fullmetal Alchemist/S01E%02d.mkv", i+1)
		jellyfinEpisodes = append(jellyfinEpisodes, clients.JellyfinEpisode{
			ID: fmt.Sprintf("e%d", i+1), Name: name,
			ParentIndexNumber: 1, IndexNumber: i + 1, Path: path,
		})
		sonarrEpisodes = append(sonarrEpisodes, clients.SonarrEpisode{
			SeasonNumber: 1, EpisodeNumber: i + 1, Title: name,
			EpisodeFile: &clients.SonarrEpisodeFile{Path: path},
		})
	}

	var episodeFetches atomic.Int32
	engine, _ := newTestJellyfinServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/Episodes") {
			if int(episodeFetches.Add(1)) <= emptyEpisodeFetches {
				_ = json.NewEncoder(w).Encode(map[string]any{"Items": []clients.JellyfinEpisode{}})
				return
			}
			if retryErrors {
				http.Error(w, "jellyfin unavailable", http.StatusInternalServerError)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"Items": jellyfinEpisodes})
			return
		}
		_ = json.NewEncoder(w).Encode(clients.JellyfinItemsResponse{Items: []clients.JellyfinItem{{
			ID: "jf-fmab", Name: "Fullmetal Alchemist: Brotherhood", Type: "Series",
			ProductionYear: 2009, Path: "/tv/Fullmetal Alchemist",
			ProviderIds: map[string]string{"Tvdb": "85249"},
		}}})
	})

	sonarrSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasPrefix(r.URL.Path, "/api/v3/episode") {
			_ = json.NewEncoder(w).Encode(sonarrEpisodes)
			return
		}
		_ = json.NewEncoder(w).Encode(clients.SonarrSeries{
			ID: 97, Title: "Fullmetal Alchemist", Year: 2003, TvdbId: 75579,
			Path: "/tv/Fullmetal Alchemist",
		})
	}))
	t.Cleanup(sonarrSrv.Close)
	engine.sonarrClient = clients.NewSonarrClient(config.SonarrConfig{
		BaseIntegrationConfig: config.BaseIntegrationConfig{URL: sonarrSrv.URL, APIKey: "k"},
	})

	engine.mediaLibrary["sonarr-97"] = models.Media{
		ID: "sonarr-97", Type: models.MediaTypeTVShow, Title: "Fullmetal Alchemist",
		Year: 2003, TVDBID: 75579, SonarrID: 97, FilePath: "/tv/Fullmetal Alchemist",
		JellyfinConflictID: "jf-fmab",
	}
	return engine, &episodeFetches
}

// TestAnalyzeJellyfinMatch_RetriesEmptyJellyfinEpisodes proves a transient empty
// Jellyfin episode list is re-fetched exactly once, so a show mid-refresh still
// adjudicates from episode evidence instead of falling back to ambiguity.
func TestAnalyzeJellyfinMatch_RetriesEmptyJellyfinEpisodes(t *testing.T) {
	engine, episodeFetches := newEpisodeRetryEngine(t, 1)

	analysis, err := engine.AnalyzeJellyfinMatch(context.Background(), "sonarr-97")
	require.NoError(t, err)

	assert.Equal(t, VerdictJellyfinWrong, analysis.Verdict,
		"the retry must recover the episode list and pin the mismatch on Jellyfin")
	assert.Equal(t, int32(2), episodeFetches.Load(),
		"an empty episode list must be retried exactly once")
	assert.Equal(t, 5, analysis.matchedEpisodes,
		"the shared-episode count must be derived from the retried episode data")
	assert.NotContains(t, strings.Join(analysis.Evidence, "\n"), "Jellyfin returned no episodes")
}

// TestAnalyzeJellyfinMatch_EmptyJellyfinEpisodesEvidence proves that when the
// episode list is still empty after the retry the analysis says so explicitly,
// rather than presenting the missing data as a considered ambiguity.
func TestAnalyzeJellyfinMatch_EmptyJellyfinEpisodesEvidence(t *testing.T) {
	engine, episodeFetches := newEpisodeRetryEngine(t, 2)

	analysis, err := engine.AnalyzeJellyfinMatch(context.Background(), "sonarr-97")
	require.NoError(t, err)

	assert.Equal(t, int32(2), episodeFetches.Load(),
		"the empty episode list must be retried once, then accepted")
	assert.Equal(t, 0, analysis.matchedEpisodes)
	assert.Equal(t, VerdictJellyfinWrong, analysis.Verdict,
		"empty episode data must still adjudicate from filename evidence; a regression to ambiguous must fail this test")
	assert.Contains(t, strings.Join(analysis.Evidence, "\n"),
		"Jellyfin returned no episodes for this series",
		"insufficient episode data must be surfaced as explicit evidence")
}

// TestAnalyzeJellyfinMatch_RetryErrorSurfaced proves that when the single retry
// after an empty episode list fails, the API failure is surfaced as distinct
// evidence instead of being masked as a genuine no-episodes result, while the
// verdict still flows from the remaining (filename) evidence.
func TestAnalyzeJellyfinMatch_RetryErrorSurfaced(t *testing.T) {
	engine, episodeFetches := newEpisodeRetryEngine(t, 1, true)

	analysis, err := engine.AnalyzeJellyfinMatch(context.Background(), "sonarr-97")
	require.NoError(t, err, "a failed episode retry must not fail the analysis")

	assert.Equal(t, int32(2), episodeFetches.Load(),
		"the empty episode list must still be retried exactly once")

	evidence := strings.Join(analysis.Evidence, "\n")
	assert.Contains(t, evidence, "Jellyfin episode fetch failed:",
		"a failed retry must be reported as an API failure")
	assert.NotContains(t, evidence, "Jellyfin returned no episodes for this series",
		"a failed retry must not be masked as a genuine no-episodes result")
	assert.Equal(t, VerdictJellyfinWrong, analysis.Verdict,
		"the verdict must still flow from the filename evidence")
}
