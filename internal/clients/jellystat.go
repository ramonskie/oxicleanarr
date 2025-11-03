package clients

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/ramonskie/prunarr/internal/config"
)

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

// GetHistory fetches watch history from Jellystat (handles pagination)
func (c *JellystatClient) GetHistory(ctx context.Context) ([]JellystatHistoryItem, error) {
	var allHistory []JellystatHistoryItem
	page := 1
	pageSize := 100

	for {
		url := fmt.Sprintf("%s/api/getHistory?page=%d&size=%d", c.baseURL, page, pageSize)

		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("creating request: %w", err)
		}

		req.Header.Set("x-api-token", c.apiKey)
		req.Header.Set("Accept", "application/json")

		resp, err := c.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("making request: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
		}

		var result JellystatHistoryResponse
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("decoding response: %w", err)
		}
		resp.Body.Close()

		allHistory = append(allHistory, result.Results...)

		// Check if we've fetched all pages
		if page >= result.Pages || len(result.Results) == 0 {
			break
		}

		page++
	}

	return allHistory, nil
}

// Ping checks if Jellystat is reachable
func (c *JellystatClient) Ping(ctx context.Context) error {
	url := fmt.Sprintf("%s/api/getLibraries", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("x-api-token", c.apiKey)

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
