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

// GetHistory fetches the complete watch history from Jellystat (handles pagination).
// itemIDs is accepted for interface compatibility but is ignored — Jellystat returns
// bulk history for all items and filtering happens in the sync layer.
func (c *JellystatClient) GetHistory(ctx context.Context, _ []string) ([]StatsHistoryItem, error) {
	var rawHistory []JellystatHistoryItem
	page := 1
	pageSize := 100

	log.Debug().Msg("Fetching watch history from Jellystat")

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
			return nil, fmt.Errorf("making request to %s: %w", c.baseURL, err)
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

		log.Debug().
			Int("page", page).
			Int("total_pages", result.Pages).
			Int("results_on_page", len(result.Results)).
			Msg("Fetched Jellystat history page")

		rawHistory = append(rawHistory, result.Results...)

		// Check if we've fetched all pages
		if page >= result.Pages || len(result.Results) == 0 {
			break
		}

		page++
	}

	// Normalise to the shared StatsHistoryItem type
	items := make([]StatsHistoryItem, 0, len(rawHistory))
	for _, h := range rawHistory {
		items = append(items, StatsHistoryItem{
			JellyfinItemID:  h.NowPlayingItemID,
			WatchedAt:       h.ActivityDateInserted,
			PlaybackSeconds: h.PlaybackDuration,
		})
	}

	log.Debug().
		Int("total_history_items", len(items)).
		Msg("Fetched all watch history from Jellystat")

	return items, nil
}

// Ping checks if Jellystat is reachable
func (c *JellystatClient) Ping(ctx context.Context) error {
	url := fmt.Sprintf("%s/api/getLibraries", c.baseURL)

	log.Debug().Str("url", c.baseURL).Msg("Pinging Jellystat")

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("x-api-token", c.apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("making request to %s: %w", c.baseURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	log.Debug().Msg("Jellystat ping successful")
	return nil
}
