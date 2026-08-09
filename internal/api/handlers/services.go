package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/ramonskie/oxicleanarr/internal/clients"
	"github.com/ramonskie/oxicleanarr/internal/config"
	"github.com/rs/zerolog/log"
)

// ServiceStatusHandler handles checking the status of connected services
type ServiceStatusHandler struct{}

// NewServiceStatusHandler creates a new ServiceStatusHandler
func NewServiceStatusHandler() *ServiceStatusHandler {
	return &ServiceStatusHandler{}
}

// ServiceStatus represents the status of a service
type ServiceStatus struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
	Online  bool   `json:"online"`
	Error   string `json:"error,omitempty"`
	Latency string `json:"latency,omitempty"`
}

// ServiceStatusResponse represents the response for service status check
type ServiceStatusResponse struct {
	Services []ServiceStatus `json:"services"`
}

// CheckStatus handles GET /api/system/services
func (h *ServiceStatusHandler) CheckStatus(w http.ResponseWriter, r *http.Request) {
	// Always get fresh config to reflect current settings
	cfg := config.Get()

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	results := make([]ServiceStatus, 0)
	resultsChan := make(chan ServiceStatus, 6)

	// Helper to check service
	checkService := func(name string, enabled bool, pinger func(context.Context) error) {
		status := ServiceStatus{
			Name:    name,
			Enabled: enabled,
		}

		if !enabled {
			resultsChan <- status
			return
		}

		start := time.Now()
		if err := pinger(ctx); err != nil {
			status.Online = false
			status.Error = err.Error()
		} else {
			status.Online = true
			status.Latency = time.Since(start).String()
		}
		resultsChan <- status
	}

	// launch runs checkService in a goroutine, releasing wg even if it panics so
	// the handler can never hang on wg.Wait().
	launch := func(name string, enabled bool, pinger func(context.Context) error) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if recovered := recover(); recovered != nil {
					log.Error().Str("service", name).Interface("panic", recovered).Msg("Service check panicked")
				}
			}()
			checkService(name, enabled, pinger)
		}()
	}

	// Jellyfin
	launch("Jellyfin", cfg.Integrations.Jellyfin.Enabled, clients.NewJellyfinClient(cfg.Integrations.Jellyfin).Ping)

	// Radarr
	launch("Radarr", cfg.Integrations.Radarr.Enabled, clients.NewRadarrClient(cfg.Integrations.Radarr).Ping)

	// Sonarr
	launch("Sonarr", cfg.Integrations.Sonarr.Enabled, clients.NewSonarrClient(cfg.Integrations.Sonarr).Ping)

	// Jellyseerr
	launch("Jellyseerr", cfg.Integrations.Jellyseerr.Enabled, clients.NewJellyseerrClient(cfg.Integrations.Jellyseerr).Ping)

	// Jellystat
	launch("Jellystat", cfg.Integrations.Jellystat.Enabled, clients.NewJellystatClient(cfg.Integrations.Jellystat).Ping)

	// Streamystats
	launch("Streamystats", cfg.Integrations.Streamystats.Enabled, clients.NewStreamystatsClient(cfg.Integrations.Streamystats).Ping)

	// Wait for all checks to complete
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// Collect results
	for status := range resultsChan {
		results = append(results, status)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ServiceStatusResponse{Services: results})
}
