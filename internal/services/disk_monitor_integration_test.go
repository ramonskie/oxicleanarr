package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

// ── Helpers ───────────────────────────────────────────────────────────────────

// buildDiskSpaceServerAtomic returns a test server whose response can be swapped
// at runtime via the returned setter. The first call to the setter configures the
// initial volumes; subsequent calls replace them atomically.
func buildDiskSpaceServerAtomic(t *testing.T) (*httptest.Server, func([]clients.DiskSpace)) {
	t.Helper()
	var current atomic.Value
	current.Store([]clients.DiskSpace{})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v3/diskspace":
			json.NewEncoder(w).Encode(current.Load().([]clients.DiskSpace))
		case "/api/v3/movie":
			// Return empty movie list so FullSync can complete without errors.
			json.NewEncoder(w).Encode([]interface{}{})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	set := func(volumes []clients.DiskSpace) { current.Store(volumes) }
	return srv, set
}

// newDiskThresholdEngine wires a real DiskMonitor (pointing at the given Radarr
// test server) into a RulesEngine and returns both, ready for end-to-end testing.
// The global config is set to cfg for the duration of the test.
func newDiskThresholdEngine(
	t *testing.T,
	cfg *config.Config,
	radarrURL string,
) (*DiskMonitor, *rules.RulesEngine) {
	t.Helper()
	config.SetTestConfig(cfg)
	t.Cleanup(func() { config.SetTestConfig(nil) })

	tmpDir := t.TempDir()
	exclusions, err := storage.NewExclusionsFile(tmpDir)
	require.NoError(t, err)

	radarrClient := clients.NewRadarrClient(config.RadarrConfig{
		BaseIntegrationConfig: config.BaseIntegrationConfig{URL: radarrURL, APIKey: "test"},
	})

	monitor := NewDiskMonitor(radarrClient, nil)
	engine := rules.NewRulesEngine(exclusions, monitor)
	return monitor, engine
}

// newDiskThresholdSyncEngine builds a full SyncEngine with a real DiskMonitor
// wired in, backed by the given Radarr test server URL.
//
// The Radarr integration is enabled in the config so that NewSyncEngine creates
// a RadarrClient pointing at radarrURL. NewSyncEngine then creates the DiskMonitor
// and injects it into the RulesEngine via SetDiskMonitor — no manual wiring needed.
func newDiskThresholdSyncEngine(
	t *testing.T,
	cfg *config.Config,
	radarrURL string,
) *SyncEngine {
	t.Helper()

	// Enable Radarr integration so NewSyncEngine creates a RadarrClient for the
	// DiskMonitor. The URL points at the test server.
	cfg.Integrations.Radarr = config.RadarrConfig{
		BaseIntegrationConfig: config.BaseIntegrationConfig{
			Enabled: true,
			URL:     radarrURL,
			APIKey:  "test",
		},
	}

	config.SetTestConfig(cfg)
	t.Cleanup(func() { config.SetTestConfig(nil) })

	tmpDir := t.TempDir()
	jobs, err := storage.NewJobsFile(tmpDir, 50)
	require.NoError(t, err)
	exclusions, err := storage.NewExclusionsFile(tmpDir)
	require.NoError(t, err)

	// Pass nil diskMonitor — NewSyncEngine will create it and inject it into the engine.
	engine := rules.NewRulesEngine(exclusions, nil)
	return NewSyncEngine(cfg, cache.New(), jobs, exclusions, engine)
}

// diskThresholdCfg returns a config with disk threshold enabled at the given
// freeSpaceGB threshold, sourced from Radarr.
func diskThresholdCfg(freeSpaceGB int) *config.Config {
	return &config.Config{
		App: config.AppConfig{
			DryRun:          false,
			EnableDeletion:  false,
			LeavingSoonDays: 14,
			DiskThreshold: config.DiskThresholdConfig{
				Enabled:     true,
				FreeSpaceGB: freeSpaceGB,
				CheckSource: "radarr",
			},
		},
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
}

// ── DiskMonitor → RulesEngine end-to-end ─────────────────────────────────────

// TestDiskThreshold_EndToEnd_DiskOK_BlocksDeletion verifies that when the real
// DiskMonitor reports free space above the threshold, the RulesEngine protects
// all media regardless of how overdue they are.
func TestDiskThreshold_EndToEnd_DiskOK_BlocksDeletion(t *testing.T) {
	srv, setVolumes := buildDiskSpaceServerAtomic(t)
	// 600 GB free — above 500 GB threshold
	setVolumes([]clients.DiskSpace{
		{Path: "/data", FreeSpace: 600 * 1024 * 1024 * 1024, TotalSpace: 4000 * 1024 * 1024 * 1024},
	})

	cfg := diskThresholdCfg(500)
	monitor, engine := newDiskThresholdEngine(t, cfg, srv.URL)

	// Fetch fresh disk state
	require.NoError(t, monitor.Update(context.Background()))

	// Movie 120 days old — past 90d retention, but disk is OK
	media := models.Media{
		ID:      "movie-1",
		Type:    models.MediaTypeMovie,
		Title:   "Old Movie",
		AddedAt: timeAgo(120),
	}
	v := engine.Evaluate(context.Background(), &media)

	assert.True(t, v.IsProtected, "disk OK should protect overdue media")
	assert.Equal(t, rules.ProtectedDiskOK, v.ProtectionReason)
	assert.Equal(t, "disk_threshold", v.ProtectingRule)
	assert.True(t, v.DeleteAfter.IsZero())
}

// TestDiskThreshold_EndToEnd_DiskBreached_AllowsDeletion verifies that when the
// real DiskMonitor reports free space below the threshold, the gate opens and
// standard retention rules schedule the media for deletion.
func TestDiskThreshold_EndToEnd_DiskBreached_AllowsDeletion(t *testing.T) {
	srv, setVolumes := buildDiskSpaceServerAtomic(t)
	// 400 GB free — below 500 GB threshold
	setVolumes([]clients.DiskSpace{
		{Path: "/data", FreeSpace: 400 * 1024 * 1024 * 1024, TotalSpace: 4000 * 1024 * 1024 * 1024},
	})

	cfg := diskThresholdCfg(500)
	monitor, engine := newDiskThresholdEngine(t, cfg, srv.URL)

	require.NoError(t, monitor.Update(context.Background()))

	// Movie 120 days old — past 90d retention, disk is breached → should delete
	media := models.Media{
		ID:      "movie-1",
		Type:    models.MediaTypeMovie,
		Title:   "Old Movie",
		AddedAt: timeAgo(120),
	}
	v := engine.Evaluate(context.Background(), &media)

	assert.False(t, v.IsProtected, "breached threshold should allow deletion")
	assert.True(t, v.ShouldDelete(), "overdue media should be scheduled for deletion")
	assert.Equal(t, rules.SourceStandardRetention, v.ScheduleSource)
}

// TestDiskThreshold_EndToEnd_StateTransition verifies that the gate correctly
// transitions from open (breached) to closed (OK) as disk space changes between
// Update() calls.
func TestDiskThreshold_EndToEnd_StateTransition(t *testing.T) {
	srv, setVolumes := buildDiskSpaceServerAtomic(t)

	cfg := diskThresholdCfg(500)
	monitor, engine := newDiskThresholdEngine(t, cfg, srv.URL)

	media := models.Media{
		ID:      "movie-1",
		Type:    models.MediaTypeMovie,
		Title:   "Old Movie",
		AddedAt: timeAgo(120),
	}

	// Step 1: disk breached → gate open → deletion scheduled
	setVolumes([]clients.DiskSpace{
		{Path: "/data", FreeSpace: 400 * 1024 * 1024 * 1024, TotalSpace: 4000 * 1024 * 1024 * 1024},
	})
	require.NoError(t, monitor.Update(context.Background()))

	v1 := engine.Evaluate(context.Background(), &media)
	assert.False(t, v1.IsProtected, "step 1: breached → gate open")
	assert.True(t, v1.ShouldDelete())

	// Step 2: disk recovers → gate closes → media protected
	setVolumes([]clients.DiskSpace{
		{Path: "/data", FreeSpace: 700 * 1024 * 1024 * 1024, TotalSpace: 4000 * 1024 * 1024 * 1024},
	})
	require.NoError(t, monitor.Update(context.Background()))

	v2 := engine.Evaluate(context.Background(), &media)
	assert.True(t, v2.IsProtected, "step 2: recovered → gate closed")
	assert.Equal(t, rules.ProtectedDiskOK, v2.ProtectionReason)
}

// TestDiskThreshold_EndToEnd_APIFailure_RetainsLastKnown verifies that when the
// disk space API fails after a successful first call, the last known state is
// retained and the engine continues to behave correctly.
func TestDiskThreshold_EndToEnd_APIFailure_RetainsLastKnown(t *testing.T) {
	srv, setVolumes := buildDiskSpaceServerAtomic(t)
	// First call: disk OK
	setVolumes([]clients.DiskSpace{
		{Path: "/data", FreeSpace: 600 * 1024 * 1024 * 1024, TotalSpace: 4000 * 1024 * 1024 * 1024},
	})

	cfg := diskThresholdCfg(500)
	monitor, engine := newDiskThresholdEngine(t, cfg, srv.URL)

	require.NoError(t, monitor.Update(context.Background()))

	// Verify gate is closed after first successful update
	media := models.Media{
		ID:      "movie-1",
		Type:    models.MediaTypeMovie,
		Title:   "Old Movie",
		AddedAt: timeAgo(120),
	}
	v1 := engine.Evaluate(context.Background(), &media)
	assert.True(t, v1.IsProtected, "disk OK: gate should be closed")

	// Simulate API failure by closing the server
	srv.Close()

	// Second update fails — last known state (OK) should be retained
	updateErr := monitor.Update(context.Background())
	assert.Error(t, updateErr, "expected error after server closed")

	// Gate should still be closed (last known: disk OK)
	v2 := engine.Evaluate(context.Background(), &media)
	assert.True(t, v2.IsProtected, "after API failure, last known state (OK) should be retained")
	assert.Equal(t, rules.ProtectedDiskOK, v2.ProtectionReason)
}

// TestDiskThreshold_EndToEnd_PreviewIgnoresGate verifies that EvaluateForPreview
// always bypasses the disk threshold gate, even when disk is healthy.
func TestDiskThreshold_EndToEnd_PreviewIgnoresGate(t *testing.T) {
	srv, setVolumes := buildDiskSpaceServerAtomic(t)
	// Disk is healthy — gate would block in normal Evaluate()
	setVolumes([]clients.DiskSpace{
		{Path: "/data", FreeSpace: 800 * 1024 * 1024 * 1024, TotalSpace: 4000 * 1024 * 1024 * 1024},
	})

	cfg := diskThresholdCfg(500)
	monitor, engine := newDiskThresholdEngine(t, cfg, srv.URL)

	require.NoError(t, monitor.Update(context.Background()))

	media := models.Media{
		ID:      "movie-1",
		Type:    models.MediaTypeMovie,
		Title:   "Old Movie",
		AddedAt: timeAgo(120),
	}

	// Normal path: gate blocks
	vNormal := engine.Evaluate(context.Background(), &media)
	assert.True(t, vNormal.IsProtected, "Evaluate: disk OK gate should block")

	// Preview path: gate bypassed
	vPreview := engine.EvaluateForPreview(context.Background(), &media)
	assert.False(t, vPreview.IsProtected, "EvaluateForPreview must bypass disk gate")
	assert.False(t, vPreview.DeleteAfter.IsZero(), "preview must show deletion date")
	assert.Equal(t, rules.SourceStandardRetention, vPreview.ScheduleSource)
}

// ── DiskMonitor → SyncEngine.applyRetentionRules end-to-end ──────────────────

// TestDiskThreshold_SyncEngine_DiskOK_MediaNotScheduled verifies that after a
// FullSync with disk above threshold, media in the library has no DeleteAfter set.
func TestDiskThreshold_SyncEngine_DiskOK_MediaNotScheduled(t *testing.T) {
	srv, setVolumes := buildDiskSpaceServerAtomic(t)
	setVolumes([]clients.DiskSpace{
		{Path: "/data", FreeSpace: 600 * 1024 * 1024 * 1024, TotalSpace: 4000 * 1024 * 1024 * 1024},
	})

	cfg := diskThresholdCfg(500)
	syncEngine := newDiskThresholdSyncEngine(t, cfg, srv.URL)

	// Seed an overdue movie directly into the library
	syncEngine.mediaLibrary["movie-1"] = models.Media{
		ID:      "movie-1",
		Type:    models.MediaTypeMovie,
		Title:   "Old Movie",
		AddedAt: timeAgo(120),
	}

	ctx := context.Background()
	require.NoError(t, syncEngine.FullSync(ctx))

	media, found := syncEngine.GetMediaByID("movie-1")
	require.True(t, found)
	assert.True(t, media.DeleteAfter.IsZero(),
		"disk OK: overdue movie should not be scheduled for deletion")
}

// TestDiskThreshold_SyncEngine_DiskBreached_MediaScheduled verifies that after a
// FullSync with disk below threshold, overdue media gets a DeleteAfter date set.
func TestDiskThreshold_SyncEngine_DiskBreached_MediaScheduled(t *testing.T) {
	srv, setVolumes := buildDiskSpaceServerAtomic(t)
	setVolumes([]clients.DiskSpace{
		{Path: "/data", FreeSpace: 400 * 1024 * 1024 * 1024, TotalSpace: 4000 * 1024 * 1024 * 1024},
	})

	cfg := diskThresholdCfg(500)
	syncEngine := newDiskThresholdSyncEngine(t, cfg, srv.URL)

	syncEngine.mediaLibrary["movie-1"] = models.Media{
		ID:      "movie-1",
		Type:    models.MediaTypeMovie,
		Title:   "Old Movie",
		AddedAt: timeAgo(120),
	}

	ctx := context.Background()
	require.NoError(t, syncEngine.FullSync(ctx))

	media, found := syncEngine.GetMediaByID("movie-1")
	require.True(t, found)
	assert.False(t, media.DeleteAfter.IsZero(),
		"disk breached: overdue movie should be scheduled for deletion")
	assert.True(t, media.DeleteAfter.Before(timeAgo(0)),
		"deletion date should be in the past (overdue)")
}

// TestDiskThreshold_SyncEngine_FeatureDisabled_StandardRetentionApplies verifies
// that when disk threshold is disabled, standard retention applies normally and
// overdue media is scheduled regardless of disk space.
func TestDiskThreshold_SyncEngine_FeatureDisabled_StandardRetentionApplies(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		App: config.AppConfig{
			DryRun:          false,
			EnableDeletion:  false,
			LeavingSoonDays: 14,
			DiskThreshold:   config.DiskThresholdConfig{Enabled: false},
		},
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
	config.SetTestConfig(cfg)
	t.Cleanup(func() { config.SetTestConfig(nil) })

	jobs, err := storage.NewJobsFile(tmpDir, 50)
	require.NoError(t, err)
	exclusions, err := storage.NewExclusionsFile(tmpDir)
	require.NoError(t, err)

	// No disk monitor — feature disabled
	engine := rules.NewRulesEngine(exclusions, nil)
	syncEngine := NewSyncEngine(cfg, cache.New(), jobs, exclusions, engine)

	syncEngine.mediaLibrary["movie-1"] = models.Media{
		ID:      "movie-1",
		Type:    models.MediaTypeMovie,
		Title:   "Old Movie",
		AddedAt: timeAgo(120),
	}

	ctx := context.Background()
	require.NoError(t, syncEngine.FullSync(ctx))

	media, found := syncEngine.GetMediaByID("movie-1")
	require.True(t, found)
	assert.False(t, media.DeleteAfter.IsZero(),
		"feature disabled: standard retention should schedule overdue media")
}

// TestDiskThreshold_SyncEngine_GetDiskMonitor verifies the GetDiskMonitor accessor.
func TestDiskThreshold_SyncEngine_GetDiskMonitor(t *testing.T) {
	srv, setVolumes := buildDiskSpaceServerAtomic(t)
	setVolumes([]clients.DiskSpace{
		{Path: "/data", FreeSpace: 600 * 1024 * 1024 * 1024, TotalSpace: 4000 * 1024 * 1024 * 1024},
	})

	cfg := diskThresholdCfg(500)
	syncEngine := newDiskThresholdSyncEngine(t, cfg, srv.URL)

	dm := syncEngine.GetDiskMonitor()
	require.NotNil(t, dm, "GetDiskMonitor should return non-nil when feature enabled")

	require.NoError(t, dm.Update(context.Background()))
	status := dm.GetStatus()
	require.NotNil(t, status)
	assert.True(t, status.Enabled)
	assert.Equal(t, 600, status.FreeSpaceGB)
	assert.False(t, status.ThresholdBreached)
}

// TestDiskThreshold_SyncEngine_GetDiskMonitor_Nil verifies that GetDiskMonitor
// returns nil when the feature is disabled.
func TestDiskThreshold_SyncEngine_GetDiskMonitor_Nil(t *testing.T) {
	engine, _, _ := newTestSyncEngine(t)
	assert.Nil(t, engine.GetDiskMonitor(), "GetDiskMonitor should be nil when feature disabled")
}

// ── helpers ───────────────────────────────────────────────────────────────────

// timeAgo returns a time.Time that is daysAgo days in the past.
func timeAgo(daysAgo int) time.Time {
	return time.Now().AddDate(0, 0, -daysAgo)
}
