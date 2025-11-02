package clients

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/ramonskie/prunarr/internal/config"
	"github.com/rs/zerolog/log"
)

// JellyfinClient handles communication with Jellyfin API
type JellyfinClient struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

// NewJellyfinClient creates a new Jellyfin client
func NewJellyfinClient(cfg config.JellyfinConfig) *JellyfinClient {
	timeout := 30 * time.Second
	if cfg.Timeout != "" {
		if d, err := time.ParseDuration(cfg.Timeout); err == nil {
			timeout = d
		}
	}

	return &JellyfinClient{
		baseURL: cfg.URL,
		apiKey:  cfg.APIKey,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

// GetMovies fetches all movies from Jellyfin
func (c *JellyfinClient) GetMovies(ctx context.Context) ([]JellyfinItem, error) {
	return c.getItems(ctx, "Movie")
}

// GetTVShows fetches all TV shows from Jellyfin
func (c *JellyfinClient) GetTVShows(ctx context.Context) ([]JellyfinItem, error) {
	return c.getItems(ctx, "Series")
}

// getItems fetches items of a specific type
func (c *JellyfinClient) getItems(ctx context.Context, itemType string) ([]JellyfinItem, error) {
	url := fmt.Sprintf("%s/Items?IncludeItemTypes=%s&Recursive=true&Fields=Path,DateCreated,ProviderIds&api_key=%s",
		c.baseURL, itemType, c.apiKey)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result JellyfinItemsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	log.Debug().
		Str("type", itemType).
		Int("count", len(result.Items)).
		Msg("Fetched items from Jellyfin")

	return result.Items, nil
}

// GetUserData fetches user-specific data for an item
func (c *JellyfinClient) GetUserData(ctx context.Context, userID, itemID string) (*JellyfinUserData, error) {
	url := fmt.Sprintf("%s/Users/%s/Items/%s?api_key=%s",
		c.baseURL, userID, itemID, c.apiKey)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var item JellyfinItem
	if err := json.NewDecoder(resp.Body).Decode(&item); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return &item.UserData, nil
}

// DeleteItem deletes an item from Jellyfin
func (c *JellyfinClient) DeleteItem(ctx context.Context, itemID string) error {
	url := fmt.Sprintf("%s/Items/%s?api_key=%s", c.baseURL, itemID, c.apiKey)

	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	log.Info().Str("item_id", itemID).Msg("Deleted item from Jellyfin")
	return nil
}

// Ping checks if Jellyfin is reachable
func (c *JellyfinClient) Ping(ctx context.Context) error {
	url := fmt.Sprintf("%s/System/Info?api_key=%s", c.baseURL, c.apiKey)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

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
