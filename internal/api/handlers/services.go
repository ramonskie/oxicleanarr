package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/ramonskie/oxicleanarr/internal/clients"
	"github.com/ramonskie/oxicleanarr/internal/config"
)

// ServiceStatusHandler handles checking the status of connected services
type ServiceStatusHandler struct {
	config *config.Config
}

// NewServiceStatusHandler creates a new ServiceStatusHandler
func NewServiceStatusHandler(cfg *config.Config) *ServiceStatusHandler {
	return &ServiceStatusHandler{
		config: cfg,
	}
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
	resultsChan := make(chan ServiceStatus, 5)

	// Helper to check service
	checkService := func(name string, enabled bool, pinger func(context.Context) error) {
		defer wg.Done()
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

	// Jellyfin
	wg.Add(1)
	go func() {
		client := clients.NewJellyfinClient(cfg.Integrations.Jellyfin)
		checkService("Jellyfin", cfg.Integrations.Jellyfin.Enabled, client.Ping)
	}()

	// Radarr
	wg.Add(1)
	go func() {
		client := clients.NewRadarrClient(cfg.Integrations.Radarr)
		checkService("Radarr", cfg.Integrations.Radarr.Enabled, client.Ping)
	}()

	// Sonarr
	wg.Add(1)
	go func() {
		client := clients.NewSonarrClient(cfg.Integrations.Sonarr)
		checkService("Sonarr", cfg.Integrations.Sonarr.Enabled, client.Ping)
	}()

	// Jellyseerr
	wg.Add(1)
	go func() {
		client := clients.NewJellyseerrClient(cfg.Integrations.Jellyseerr)
		checkService("Jellyseerr", cfg.Integrations.Jellyseerr.Enabled, client.Ping)
	}()

	// Jellystat
	wg.Add(1)
	go func() {
		client := clients.NewJellystatClient(cfg.Integrations.Jellystat)
		checkService("Jellystat", cfg.Integrations.Jellystat.Enabled, client.Ping)
	}()

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
