package clients

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/ramonskie/oxicleanarr/internal/config"
	"github.com/rs/zerolog/log"
)

// StreamystatsClient handles communication with the Streamystats API.
// Auth: Authorization: Bearer <jellyfin-api-key> (validated live against Jellyfin).
// History: GET /api/get-item-details/{jellyfinItemId}?serverId={serverId}
// Returns up to 50 sessions per item — no server-side pagination.
type StreamystatsClient struct {
	baseURL  string
	apiKey   string
	serverID string
	client   *http.Client
}

// NewStreamystatsClient creates a new Streamystats client.
func NewStreamystatsClient(cfg config.StreamystatsConfig) *StreamystatsClient {
	timeout := 30 * time.Second
	if cfg.Timeout != "" {
		if d, err := time.ParseDuration(cfg.Timeout); err == nil {
			timeout = d
		}
	}

	return &StreamystatsClient{
		baseURL:  cfg.URL,
		apiKey:   cfg.APIKey,
		serverID: cfg.ServerID,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

// streamystatsItemDetailsResponse is the response from /api/get-item-details/{id}.
type streamystatsItemDetailsResponse struct {
	WatchHistory []streamystatsSession `json:"watchHistory"`
}

// streamystatsSession represents a single play session returned by Streamystats.
type streamystatsSession struct {
	UserID           string    `json:"userId"`
	LastActivityDate time.Time `json:"lastActivityDate"`
	PlayDuration     int       `json:"playDuration"` // seconds
	Completed        bool      `json:"completed"`
}

// GetHistory fetches watch history for each provided Jellyfin item ID concurrently.
// Each item is queried against GET /api/get-item-details/{id}?serverId={serverID}.
// Results are normalised to []StatsHistoryItem (one entry per session).
func (c *StreamystatsClient) GetHistory(ctx context.Context, itemIDs []string) ([]StatsHistoryItem, error) {
	if len(itemIDs) == 0 {
		return nil, nil
	}

	log.Debug().
		Int("item_count", len(itemIDs)).
		Msg("Fetching watch history from Streamystats")

	type result struct {
		items []StatsHistoryItem
		err   error
	}

	resultCh := make(chan result, len(itemIDs))

	var wg sync.WaitGroup
	for _, id := range itemIDs {
		wg.Add(1)
		go func(jellyfinID string) {
			defer wg.Done()
			items, err := c.fetchItemHistory(ctx, jellyfinID)
			resultCh <- result{items: items, err: err}
		}(id)
	}

	// Close channel once all goroutines finish.
	go func() {
		wg.Wait()
		close(resultCh)
	}()

	var allItems []StatsHistoryItem
	for r := range resultCh {
		if r.err != nil {
			// Log non-fatal per-item errors and continue — a single missing item
			// should not abort the entire history fetch.
			log.Warn().Err(r.err).Msg("Streamystats: failed to fetch item history, skipping")
			continue
		}
		allItems = append(allItems, r.items...)
	}

	log.Debug().
		Int("total_sessions", len(allItems)).
		Msg("Fetched watch history from Streamystats")

	return allItems, nil
}

// fetchItemHistory queries /api/get-item-details/{id}?serverId=... for one item.
func (c *StreamystatsClient) fetchItemHistory(ctx context.Context, jellyfinID string) ([]StatsHistoryItem, error) {
	url := fmt.Sprintf("%s/api/get-item-details/%s?serverId=%s", c.baseURL, jellyfinID, c.serverID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request for item %s: %w", jellyfinID, err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("making request for item %s: %w", jellyfinID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		// Item not tracked in Streamystats yet — not an error.
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d for item %s", resp.StatusCode, jellyfinID)
	}

	var details streamystatsItemDetailsResponse
	if err := json.NewDecoder(resp.Body).Decode(&details); err != nil {
		return nil, fmt.Errorf("decoding response for item %s: %w", jellyfinID, err)
	}

	items := make([]StatsHistoryItem, 0, len(details.WatchHistory))
	for _, s := range details.WatchHistory {
		items = append(items, StatsHistoryItem{
			JellyfinItemID:  jellyfinID,
			WatchedAt:       s.LastActivityDate,
			PlaybackSeconds: s.PlayDuration,
		})
	}

	return items, nil
}

// Ping checks whether Streamystats is reachable by requesting any item-details
// endpoint with a well-known placeholder and accepting a 404 as "online".
func (c *StreamystatsClient) Ping(ctx context.Context) error {
	url := fmt.Sprintf("%s/api/get-item-details/ping-check?serverId=%s", c.baseURL, c.serverID)

	log.Debug().Str("url", c.baseURL).Msg("Pinging Streamystats")

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("making request to %s: %w", c.baseURL, err)
	}
	defer resp.Body.Close()

	// 200 (found) or 404 (not found) both prove the server is reachable.
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	log.Debug().Msg("Streamystats ping successful")
	return nil
}
