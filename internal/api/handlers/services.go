package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/ramonskie/oxicleanarr/internal/clients"
	"github.com/ramonskie/oxicleanarr/internal/config"
	"github.com/rs/zerolog/log"
)

// ServiceStatusHandler handles checking the status of connected services
type ServiceStatusHandler struct {
	mu      sync.Mutex
	lastCfg *config.Config
	checks  []serviceCheck
}

// serviceCheck bundles the metadata and pinger for one service.
type serviceCheck struct {
	name    string
	enabled bool
	pinger  func(context.Context) error
}

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

// buildChecks derives the pinger list from the current config.
// Client instances are cheap to build (config struct + http.Client), but
// rebuilding them on every request still churns allocations, so CheckStatus
// caches them and only rebuilds when the config pointer changes (hot-reload).
func (h *ServiceStatusHandler) buildChecks(cfg *config.Config) []serviceCheck {
	return []serviceCheck{
		{name: "Jellyfin", enabled: cfg.Integrations.Jellyfin.Enabled, pinger: clients.NewJellyfinClient(cfg.Integrations.Jellyfin).Ping},
		{name: "Radarr", enabled: cfg.Integrations.Radarr.Enabled, pinger: clients.NewRadarrClient(cfg.Integrations.Radarr).Ping},
		{name: "Sonarr", enabled: cfg.Integrations.Sonarr.Enabled, pinger: clients.NewSonarrClient(cfg.Integrations.Sonarr).Ping},
		{name: "Jellyseerr", enabled: cfg.Integrations.Jellyseerr.Enabled, pinger: clients.NewJellyseerrClient(cfg.Integrations.Jellyseerr).Ping},
		{name: "Jellystat", enabled: cfg.Integrations.Jellystat.Enabled, pinger: clients.NewJellystatClient(cfg.Integrations.Jellystat).Ping},
		{name: "Streamystats", enabled: cfg.Integrations.Streamystats.Enabled, pinger: clients.NewStreamystatsClient(cfg.Integrations.Streamystats).Ping},
	}
}

// CheckStatus handles GET /api/system/services
func (h *ServiceStatusHandler) CheckStatus(w http.ResponseWriter, r *http.Request) {
	// Always get fresh config to reflect current settings
	cfg := config.Get()
	if cfg == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "Configuration not available"})
		return
	}

	// Reuse cached client instances; rebuild only when config was reloaded.
	// buildChecks runs outside the lock so a panic there (e.g. nil config)
	// cannot leave h.mu held forever and wedge every later request.
	h.mu.Lock()
	rebuild := h.lastCfg != cfg
	checks := h.checks
	h.mu.Unlock()

	if rebuild {
		checks = h.buildChecks(cfg)
		h.mu.Lock()
		h.lastCfg = cfg
		h.checks = checks
		h.mu.Unlock()
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	results := make([]ServiceStatus, 0)
	resultsChan := make(chan ServiceStatus, len(checks))

	// Helper to check service
	checkService := func(check serviceCheck) {
		status := ServiceStatus{
			Name:    check.name,
			Enabled: check.enabled,
		}

		if !check.enabled {
			resultsChan <- status
			return
		}

		start := time.Now()
		if err := check.pinger(ctx); err != nil {
			status.Online = false
			// Log the raw error server-side before sanitizing, so operators can
			// still see which host/port/URL failed even though the API response
			// strips that detail.
			log.Warn().Str("service", check.name).Err(err).Msg("Service check failed")
			status.Error = sanitizePingError(err)
		} else {
			status.Online = true
			status.Latency = time.Since(start).String()
		}
		resultsChan <- status
	}

	// launch runs checkService in a goroutine, releasing wg even if it panics so
	// the handler can never hang on wg.Wait().
	launch := func(check serviceCheck) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if recovered := recover(); recovered != nil {
					log.Error().Str("service", check.name).Interface("panic", recovered).Msg("Service check panicked")
				}
			}()
			checkService(check)
		}()
	}

	for _, check := range checks {
		launch(check)
	}

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

var (
	urlPattern      = regexp.MustCompile(`https?://[^\s"'<>]+`)
	ipPortPattern   = regexp.MustCompile(`\d{1,3}(?:\.\d{1,3}){3}(?::\d{1,5})?`)
	ipv6Pattern     = regexp.MustCompile(`\[[0-9a-fA-F:]+(?:%[0-9a-zA-Z.-]+)?\](?::\d{1,5})?`)
	hostnamePattern = regexp.MustCompile(`[a-zA-Z0-9](?:[a-zA-Z0-9-]*[a-zA-Z0-9])?(?:\.[a-zA-Z0-9](?:[a-zA-Z0-9-]*[a-zA-Z0-9])?)+(?::\d{1,5})?`)
	// Single-label hostnames (no dot) only leak in the fallback when they carry
	// a port, e.g. "dial tcp jellyfin:8096". A bare label without a port is too
	// ambiguous to scrub safely.
	bareHostPortPattern = regexp.MustCompile(`[a-zA-Z0-9](?:[a-zA-Z0-9-]*[a-zA-Z0-9])?:\d{1,5}`)
)

// sanitizePingError strips internal hostnames, IPs, ports, and URL fragments
// from a ping error before it is exposed through the public API. Transport
// errors from http.Client.Do arrive as *url.Error wrapping a net.OpError that
// embeds the dialed address, and several clients also embed baseURL directly.
func sanitizePingError(err error) string {
	if err == nil {
		return ""
	}

	var uerr *url.Error
	if errors.As(err, &uerr) && uerr.Err != nil {
		err = uerr.Err
	}

	msg := strings.ToLower(err.Error())

	// Status-code errors from the clients are already safe (no URL embedded).
	if strings.HasPrefix(msg, "unexpected status code") {
		return err.Error()
	}

	// Classify transport-level failures into safe categories. This catches DNS
	// failures ("lookup internal-host ...") and cert mismatches that regex
	// scrubbing would otherwise leak verbatim.
	switch {
	case strings.Contains(msg, "connection refused"):
		return "connection refused"
	case strings.Contains(msg, "no such host") || strings.Contains(msg, "lookup"):
		return "host could not be resolved"
	case strings.Contains(msg, "connection reset"):
		return "connection reset"
	case strings.Contains(msg, "timed out") || strings.Contains(msg, "timeout") || strings.Contains(msg, "context deadline exceeded"):
		return "request timed out"
	case strings.Contains(msg, "network is unreachable"):
		return "network is unreachable"
	case strings.Contains(msg, "no route to host"):
		return "network is unreachable"
	case strings.Contains(msg, "x509") || strings.Contains(msg, "certificate"):
		return "certificate verification failed"
	}

	// Fallback: scrub any residual URLs, IPs, and host:port fragments. The
	// hostname pattern also catches bare hosts in cert/dial errors that carry
	// no URL scheme (e.g. "dial tcp mynas.lan:7878: network is unreachable").
	scrubbed := err.Error()
	scrubbed = urlPattern.ReplaceAllString(scrubbed, "endpoint")
	scrubbed = ipv6Pattern.ReplaceAllString(scrubbed, "endpoint")
	scrubbed = ipPortPattern.ReplaceAllString(scrubbed, "endpoint")
	scrubbed = bareHostPortPattern.ReplaceAllString(scrubbed, "endpoint")
	scrubbed = hostnamePattern.ReplaceAllString(scrubbed, "endpoint")
	scrubbed = strings.TrimSuffix(strings.TrimSpace(scrubbed), ":")
	scrubbed = strings.TrimSpace(scrubbed)

	if scrubbed == "" || scrubbed == "endpoint" {
		return "connection failed"
	}
	return scrubbed
}
