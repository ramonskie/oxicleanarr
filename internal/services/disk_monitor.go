package services

import (
	"context"
	"fmt"
	"sync"

	"github.com/ramonskie/oxicleanarr/internal/clients"
	"github.com/ramonskie/oxicleanarr/internal/config"
	"github.com/ramonskie/oxicleanarr/internal/services/rules"
	"github.com/rs/zerolog/log"
)

// DiskMonitor fetches and caches disk space from Radarr/Sonarr.
// It satisfies the rules.DiskMonitor interface.
type DiskMonitor struct {
	radarr *clients.RadarrClient
	sonarr *clients.SonarrClient

	mu              sync.RWMutex
	freeSpaceGB     int
	totalSpaceGB    int
	thresholdActive bool
	initialized     bool
}

// Ensure DiskMonitor satisfies the rules.DiskMonitor interface.
var _ rules.DiskMonitor = (*DiskMonitor)(nil)

// NewDiskMonitor creates a new DiskMonitor.
// Either radarr or sonarr (or both) may be nil; the monitor will use whichever are available.
func NewDiskMonitor(radarr *clients.RadarrClient, sonarr *clients.SonarrClient) *DiskMonitor {
	return &DiskMonitor{
		radarr: radarr,
		sonarr: sonarr,
	}
}

// Update fetches the latest disk space and updates cached state.
// Called once at the start of each FullSync. Failures are non-fatal —
// the last known state is retained and a warning is logged.
func (m *DiskMonitor) Update(ctx context.Context) error {
	cfg := config.Get()
	if !cfg.App.DiskThreshold.Enabled {
		return nil
	}

	source := cfg.App.DiskThreshold.CheckSource
	if source == "" {
		source = "radarr"
	}

	freeBytes, totalBytes, err := m.fetchDiskSpace(ctx, source)
	if err != nil {
		return fmt.Errorf("fetching disk space (%s): %w", source, err)
	}

	freeGB := int(freeBytes / (1024 * 1024 * 1024))
	totalGB := int(totalBytes / (1024 * 1024 * 1024))
	breached := freeGB < cfg.App.DiskThreshold.FreeSpaceGB

	m.mu.Lock()
	prevBreached := m.thresholdActive
	prevInitialized := m.initialized
	m.freeSpaceGB = freeGB
	m.totalSpaceGB = totalGB
	m.thresholdActive = breached
	m.initialized = true
	m.mu.Unlock()

	// Log state transitions
	if !prevInitialized {
		if breached {
			log.Warn().
				Int("free_gb", freeGB).
				Int("threshold_gb", cfg.App.DiskThreshold.FreeSpaceGB).
				Str("source", source).
				Msg("Disk threshold BREACHED — rules now ACTIVE")
		} else {
			log.Info().
				Int("free_gb", freeGB).
				Int("total_gb", totalGB).
				Int("threshold_gb", cfg.App.DiskThreshold.FreeSpaceGB).
				Str("source", source).
				Msg("Disk status initialised — threshold not breached, rules dormant")
		}
	} else if breached && !prevBreached {
		log.Warn().
			Int("free_gb", freeGB).
			Int("threshold_gb", cfg.App.DiskThreshold.FreeSpaceGB).
			Str("source", source).
			Msg("Disk threshold BREACHED — rules now ACTIVE")
	} else if !breached && prevBreached {
		log.Info().
			Int("free_gb", freeGB).
			Int("threshold_gb", cfg.App.DiskThreshold.FreeSpaceGB).
			Str("source", source).
			Msg("Disk threshold RECOVERED — rules now DORMANT")
	} else {
		log.Debug().
			Int("free_gb", freeGB).
			Int("total_gb", totalGB).
			Int("threshold_gb", cfg.App.DiskThreshold.FreeSpaceGB).
			Bool("breached", breached).
			Str("source", source).
			Msg("Disk status updated")
	}

	return nil
}

// GetStatus returns a snapshot of the current disk status for use in EvalContext.
// Returns nil if the disk threshold feature is disabled.
func (m *DiskMonitor) GetStatus() *rules.DiskStatus {
	cfg := config.Get()
	if !cfg.App.DiskThreshold.Enabled {
		return nil
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	source := cfg.App.DiskThreshold.CheckSource
	if source == "" {
		source = "radarr"
	}

	return &rules.DiskStatus{
		Enabled:           true,
		FreeSpaceGB:       m.freeSpaceGB,
		TotalSpaceGB:      m.totalSpaceGB,
		ThresholdGB:       cfg.App.DiskThreshold.FreeSpaceGB,
		ThresholdBreached: m.thresholdActive,
		CheckSource:       source,
	}
}

// fetchDiskSpace retrieves free and total bytes from the configured source.
func (m *DiskMonitor) fetchDiskSpace(ctx context.Context, source string) (freeBytes, totalBytes int64, err error) {
	switch source {
	case "sonarr":
		return m.fetchFromSonarr(ctx)
	case "lowest":
		return m.fetchLowest(ctx)
	default: // "radarr" or empty
		return m.fetchFromRadarr(ctx)
	}
}

func (m *DiskMonitor) fetchFromRadarr(ctx context.Context) (int64, int64, error) {
	if m.radarr == nil {
		return 0, 0, fmt.Errorf("radarr client not available")
	}
	volumes, err := m.radarr.GetDiskSpace(ctx)
	if err != nil {
		return 0, 0, err
	}
	return aggregateVolumes(volumes)
}

func (m *DiskMonitor) fetchFromSonarr(ctx context.Context) (int64, int64, error) {
	if m.sonarr == nil {
		return 0, 0, fmt.Errorf("sonarr client not available")
	}
	volumes, err := m.sonarr.GetDiskSpace(ctx)
	if err != nil {
		return 0, 0, err
	}
	return aggregateVolumes(volumes)
}

// fetchLowest returns the single lowest free-space volume across Radarr and Sonarr.
func (m *DiskMonitor) fetchLowest(ctx context.Context) (int64, int64, error) {
	var allVolumes []clients.DiskSpace

	if m.radarr != nil {
		volumes, err := m.radarr.GetDiskSpace(ctx)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to fetch disk space from Radarr for lowest check")
		} else {
			allVolumes = append(allVolumes, volumes...)
		}
	}

	if m.sonarr != nil {
		volumes, err := m.sonarr.GetDiskSpace(ctx)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to fetch disk space from Sonarr for lowest check")
		} else {
			allVolumes = append(allVolumes, volumes...)
		}
	}

	if len(allVolumes) == 0 {
		return 0, 0, fmt.Errorf("no disk space data available from any source")
	}

	// Find the volume with the lowest free space
	lowest := allVolumes[0]
	for _, v := range allVolumes[1:] {
		if v.FreeSpace < lowest.FreeSpace {
			lowest = v
		}
	}

	return lowest.FreeSpace, lowest.TotalSpace, nil
}

// aggregateVolumes sums free and total space across all volumes.
func aggregateVolumes(volumes []clients.DiskSpace) (freeBytes, totalBytes int64, err error) {
	if len(volumes) == 0 {
		return 0, 0, fmt.Errorf("no disk volumes returned")
	}
	for _, v := range volumes {
		freeBytes += v.FreeSpace
		totalBytes += v.TotalSpace
	}
	return freeBytes, totalBytes, nil
}
