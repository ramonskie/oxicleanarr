package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ramonskie/oxicleanarr/internal/clients"
	"github.com/ramonskie/oxicleanarr/internal/config"
)

// buildDiskSpaceServer returns a test HTTP server that serves disk space JSON.
func buildDiskSpaceServer(t *testing.T, volumes []clients.DiskSpace) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/diskspace" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(volumes)
	}))
}

// setDiskThresholdConfig configures disk threshold in the global config for the duration of a test.
func setDiskThresholdConfig(t *testing.T, enabled bool, freeSpaceGB int, source string) {
	t.Helper()
	base := config.Get()
	if base == nil {
		base = config.DefaultConfig()
	}
	prev := base.App.DiskThreshold
	base.App.DiskThreshold = config.DiskThresholdConfig{
		Enabled:     enabled,
		FreeSpaceGB: freeSpaceGB,
		CheckSource: source,
	}
	config.SetTestConfig(base)
	t.Cleanup(func() {
		cfg := config.Get()
		cfg.App.DiskThreshold = prev
		config.SetTestConfig(cfg)
	})
}

func TestDiskMonitor_GetStatus_FeatureDisabled_ReturnsNil(t *testing.T) {
	setDiskThresholdConfig(t, false, 0, "")

	m := NewDiskMonitor(nil, nil)
	status := m.GetStatus()

	if status != nil {
		t.Errorf("expected nil status when feature disabled, got %+v", status)
	}
}

func TestDiskMonitor_Update_FeatureDisabled_IsNoop(t *testing.T) {
	setDiskThresholdConfig(t, false, 500, "radarr")

	m := NewDiskMonitor(nil, nil)
	if err := m.Update(context.Background()); err != nil {
		t.Errorf("expected no error when feature disabled, got %v", err)
	}
}

func TestDiskMonitor_Update_FromRadarr(t *testing.T) {
	volumes := []clients.DiskSpace{
		{Path: "/data", FreeSpace: 600 * 1024 * 1024 * 1024, TotalSpace: 4000 * 1024 * 1024 * 1024},
	}
	srv := buildDiskSpaceServer(t, volumes)
	defer srv.Close()

	setDiskThresholdConfig(t, true, 500, "radarr")

	radarrClient := clients.NewRadarrClient(config.RadarrConfig{
		BaseIntegrationConfig: config.BaseIntegrationConfig{URL: srv.URL, APIKey: "test"},
	})
	m := NewDiskMonitor(radarrClient, nil)

	if err := m.Update(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	status := m.GetStatus()
	if status == nil {
		t.Fatal("expected non-nil status")
	}
	if status.FreeSpaceGB != 600 {
		t.Errorf("expected FreeSpaceGB=600, got %d", status.FreeSpaceGB)
	}
	if status.ThresholdBreached {
		t.Error("expected threshold not breached (600GB > 500GB)")
	}
}

func TestDiskMonitor_Update_FromSonarr(t *testing.T) {
	volumes := []clients.DiskSpace{
		{Path: "/tv", FreeSpace: 400 * 1024 * 1024 * 1024, TotalSpace: 4000 * 1024 * 1024 * 1024},
	}
	srv := buildDiskSpaceServer(t, volumes)
	defer srv.Close()

	setDiskThresholdConfig(t, true, 500, "sonarr")

	sonarrClient := clients.NewSonarrClient(config.SonarrConfig{
		BaseIntegrationConfig: config.BaseIntegrationConfig{URL: srv.URL, APIKey: "test"},
	})
	m := NewDiskMonitor(nil, sonarrClient)

	if err := m.Update(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	status := m.GetStatus()
	if status == nil {
		t.Fatal("expected non-nil status")
	}
	if status.FreeSpaceGB != 400 {
		t.Errorf("expected FreeSpaceGB=400, got %d", status.FreeSpaceGB)
	}
	if !status.ThresholdBreached {
		t.Error("expected threshold breached (400GB < 500GB)")
	}
}

func TestDiskMonitor_Update_Lowest(t *testing.T) {
	radarrVolumes := []clients.DiskSpace{
		{Path: "/movies", FreeSpace: 800 * 1024 * 1024 * 1024, TotalSpace: 4000 * 1024 * 1024 * 1024},
	}
	sonarrVolumes := []clients.DiskSpace{
		{Path: "/tv", FreeSpace: 300 * 1024 * 1024 * 1024, TotalSpace: 4000 * 1024 * 1024 * 1024},
	}

	radarrSrv := buildDiskSpaceServer(t, radarrVolumes)
	defer radarrSrv.Close()
	sonarrSrv := buildDiskSpaceServer(t, sonarrVolumes)
	defer sonarrSrv.Close()

	setDiskThresholdConfig(t, true, 500, "lowest")

	radarrClient := clients.NewRadarrClient(config.RadarrConfig{
		BaseIntegrationConfig: config.BaseIntegrationConfig{URL: radarrSrv.URL, APIKey: "test"},
	})
	sonarrClient := clients.NewSonarrClient(config.SonarrConfig{
		BaseIntegrationConfig: config.BaseIntegrationConfig{URL: sonarrSrv.URL, APIKey: "test"},
	})
	m := NewDiskMonitor(radarrClient, sonarrClient)

	if err := m.Update(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	status := m.GetStatus()
	if status == nil {
		t.Fatal("expected non-nil status")
	}
	// lowest volume is 300GB (Sonarr)
	if status.FreeSpaceGB != 300 {
		t.Errorf("expected FreeSpaceGB=300 (lowest), got %d", status.FreeSpaceGB)
	}
	if !status.ThresholdBreached {
		t.Error("expected threshold breached (300GB < 500GB)")
	}
}

func TestDiskMonitor_Update_APIFailure_UsesLastKnown(t *testing.T) {
	// First call succeeds with 600GB
	volumes := []clients.DiskSpace{
		{Path: "/data", FreeSpace: 600 * 1024 * 1024 * 1024, TotalSpace: 4000 * 1024 * 1024 * 1024},
	}
	srv := buildDiskSpaceServer(t, volumes)

	setDiskThresholdConfig(t, true, 500, "radarr")

	radarrClient := clients.NewRadarrClient(config.RadarrConfig{
		BaseIntegrationConfig: config.BaseIntegrationConfig{URL: srv.URL, APIKey: "test"},
	})
	m := NewDiskMonitor(radarrClient, nil)

	if err := m.Update(context.Background()); err != nil {
		t.Fatalf("unexpected error on first update: %v", err)
	}

	// Close the server to simulate API failure
	srv.Close()

	// Second call fails — last known state should be retained
	updateErr := m.Update(context.Background())
	if updateErr == nil {
		t.Error("expected error after server closed")
	}

	// State unchanged from successful first update
	status := m.GetStatus()
	if status == nil {
		t.Fatal("expected non-nil status (last known state)")
	}
	if status.FreeSpaceGB != 600 {
		t.Errorf("expected last known FreeSpaceGB=600, got %d", status.FreeSpaceGB)
	}
}

func TestDiskMonitor_StateChange_LogsBreach(t *testing.T) {
	// Start above threshold, then drop below
	volumesAbove := []clients.DiskSpace{
		{Path: "/data", FreeSpace: 600 * 1024 * 1024 * 1024, TotalSpace: 4000 * 1024 * 1024 * 1024},
	}
	volumesBelow := []clients.DiskSpace{
		{Path: "/data", FreeSpace: 400 * 1024 * 1024 * 1024, TotalSpace: 4000 * 1024 * 1024 * 1024},
	}

	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if callCount == 0 {
			json.NewEncoder(w).Encode(volumesAbove)
		} else {
			json.NewEncoder(w).Encode(volumesBelow)
		}
		callCount++
	}))
	defer srv.Close()

	setDiskThresholdConfig(t, true, 500, "radarr")

	radarrClient := clients.NewRadarrClient(config.RadarrConfig{
		BaseIntegrationConfig: config.BaseIntegrationConfig{URL: srv.URL, APIKey: "test"},
	})
	m := NewDiskMonitor(radarrClient, nil)

	// First update — above threshold
	if err := m.Update(context.Background()); err != nil {
		t.Fatalf("first update error: %v", err)
	}
	if m.GetStatus().ThresholdBreached {
		t.Error("expected threshold not breached after first update")
	}

	// Second update — drops below threshold (breach logged internally)
	if err := m.Update(context.Background()); err != nil {
		t.Fatalf("second update error: %v", err)
	}
	if !m.GetStatus().ThresholdBreached {
		t.Error("expected threshold breached after second update")
	}
}

func TestDiskMonitor_StateChange_LogsRecovery(t *testing.T) {
	// Start below threshold, then recover
	volumesBelow := []clients.DiskSpace{
		{Path: "/data", FreeSpace: 400 * 1024 * 1024 * 1024, TotalSpace: 4000 * 1024 * 1024 * 1024},
	}
	volumesAbove := []clients.DiskSpace{
		{Path: "/data", FreeSpace: 600 * 1024 * 1024 * 1024, TotalSpace: 4000 * 1024 * 1024 * 1024},
	}

	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if callCount == 0 {
			json.NewEncoder(w).Encode(volumesBelow)
		} else {
			json.NewEncoder(w).Encode(volumesAbove)
		}
		callCount++
	}))
	defer srv.Close()

	setDiskThresholdConfig(t, true, 500, "radarr")

	radarrClient := clients.NewRadarrClient(config.RadarrConfig{
		BaseIntegrationConfig: config.BaseIntegrationConfig{URL: srv.URL, APIKey: "test"},
	})
	m := NewDiskMonitor(radarrClient, nil)

	// First update — below threshold (breach)
	if err := m.Update(context.Background()); err != nil {
		t.Fatalf("first update error: %v", err)
	}
	if !m.GetStatus().ThresholdBreached {
		t.Error("expected threshold breached after first update")
	}

	// Second update — recovers above threshold
	if err := m.Update(context.Background()); err != nil {
		t.Fatalf("second update error: %v", err)
	}
	if m.GetStatus().ThresholdBreached {
		t.Error("expected threshold not breached after recovery")
	}
}
