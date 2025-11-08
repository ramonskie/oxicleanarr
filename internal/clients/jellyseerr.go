package clients

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/ramonskie/oxicleanarr/internal/config"
	"github.com/rs/zerolog/log"
)

// JellyseerrClient handles communication with Jellyseerr API
type JellyseerrClient struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

// NewJellyseerrClient creates a new Jellyseerr client
func NewJellyseerrClient(cfg config.JellyseerrConfig) *JellyseerrClient {
	timeout := 30 * time.Second
	if cfg.Timeout != "" {
		if d, err := time.ParseDuration(cfg.Timeout); err == nil {
			timeout = d
		}
	}

	return &JellyseerrClient{
		baseURL: cfg.URL,
		apiKey:  cfg.APIKey,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

// GetRequests fetches all requests from Jellyseerr (handles pagination)
func (c *JellyseerrClient) GetRequests(ctx context.Context) ([]JellyseerrRequest, error) {
	var allRequests []JellyseerrRequest
	page := 1

	log.Debug().Msg("Fetching requests from Jellyseerr")

	for {
		url := fmt.Sprintf("%s/api/v1/request?take=50&skip=%d", c.baseURL, (page-1)*50)

		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			log.Error().Err(err).Msg("Failed to create Jellyseerr request")
			return nil, fmt.Errorf("creating request: %w", err)
		}

		req.Header.Set("X-Api-Key", c.apiKey)
		req.Header.Set("Accept", "application/json")

		resp, err := c.client.Do(req)
		if err != nil {
			log.Error().Err(err).Str("url", c.baseURL).Msg("Failed to connect to Jellyseerr")
			return nil, fmt.Errorf("making request: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			log.Error().Int("status_code", resp.StatusCode).Msg("Jellyseerr returned unexpected status code")
			return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
		}

		var result JellyseerrResponse
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			resp.Body.Close()
			log.Error().Err(err).Msg("Failed to decode Jellyseerr response")
			return nil, fmt.Errorf("decoding response: %w", err)
		}
		resp.Body.Close()

		log.Debug().
			Int("page", page).
			Int("total_pages", result.PageInfo.Pages).
			Int("results_on_page", len(result.Results)).
			Msg("Fetched Jellyseerr requests page")

		allRequests = append(allRequests, result.Results...)

		// Check if we've fetched all pages
		if page >= result.PageInfo.Pages || len(result.Results) == 0 {
			break
		}

		page++
	}

	log.Debug().
		Int("total_requests", len(allRequests)).
		Msg("Fetched all requests from Jellyseerr")

	return allRequests, nil
}

// Ping checks if Jellyseerr is reachable
func (c *JellyseerrClient) Ping(ctx context.Context) error {
	url := fmt.Sprintf("%s/api/v1/status", c.baseURL)

	log.Debug().Str("url", c.baseURL).Msg("Pinging Jellyseerr")

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create Jellyseerr ping request")
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("X-Api-Key", c.apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		log.Error().Err(err).Str("url", c.baseURL).Msg("Failed to ping Jellyseerr")
		return fmt.Errorf("making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Error().Int("status_code", resp.StatusCode).Msg("Jellyseerr ping returned unexpected status code")
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	log.Debug().Msg("Jellyseerr ping successful")
	return nil
}
