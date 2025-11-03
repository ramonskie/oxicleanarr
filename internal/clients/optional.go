package clients

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/ramonskie/prunarr/internal/config"
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

	for {
		url := fmt.Sprintf("%s/api/v1/request?take=50&skip=%d", c.baseURL, (page-1)*50)

		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("creating request: %w", err)
		}

		req.Header.Set("X-Api-Key", c.apiKey)
		req.Header.Set("Accept", "application/json")

		resp, err := c.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("making request: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
		}

		var result JellyseerrResponse
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("decoding response: %w", err)
		}
		resp.Body.Close()

		allRequests = append(allRequests, result.Results...)

		// Check if we've fetched all pages
		if page >= result.PageInfo.Pages || len(result.Results) == 0 {
			break
		}

		page++
	}

	return allRequests, nil
}

// Ping checks if Jellyseerr is reachable
func (c *JellyseerrClient) Ping(ctx context.Context) error {
	url := fmt.Sprintf("%s/api/v1/status", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("X-Api-Key", c.apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

// JellystatClient handles communication with Jellystat API
type JellystatClient struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

// NewJellystatClient creates a new Jellystat client
func NewJellystatClient(cfg config.JellystatConfig) *JellystatClient {
	timeout := 30 * time.Second
	if cfg.Timeout != "" {
		if d, err := time.ParseDuration(cfg.Timeout); err == nil {
			timeout = d
		}
	}

	return &JellystatClient{
		baseURL: cfg.URL,
		apiKey:  cfg.APIKey,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

// GetActivity fetches watch activity from Jellystat
func (c *JellystatClient) GetActivity(ctx context.Context, itemID string) (*JellystatActivity, error) {
	url := fmt.Sprintf("%s/api/activity/%s", c.baseURL, itemID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var activity JellystatActivity
	if err := json.NewDecoder(resp.Body).Decode(&activity); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return &activity, nil
}

// Ping checks if Jellystat is reachable
func (c *JellystatClient) Ping(ctx context.Context) error {
	url := fmt.Sprintf("%s/api/health", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}
