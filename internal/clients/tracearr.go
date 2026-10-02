package clients

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/ramonskie/oxicleanarr/internal/config"
	"github.com/rs/zerolog/log"
)

// tracearrHistoryPageSize is the maximum page size accepted by the Tracearr v2
// public history endpoint (pageSize max 100).
const tracearrHistoryPageSize = 100

// tracearrMaxPages bounds cursor pagination so a server that returns a
// non-advancing (or self-referencing) cursor cannot loop forever.
const tracearrMaxPages = 1000

// TracearrClient handles communication with the Tracearr v2 Public API.
// Auth: Authorization: Bearer trr_pub_<base64url>.
// History: GET /api/v2/public/history?pageSize=100&cursor=<opaque>&serverId=<uuid>
type TracearrClient struct {
	baseURL  string
	apiKey   string
	serverID string
	client   *http.Client
}

// Compile-time assertion that TracearrClient satisfies the shared StatsProvider
// contract. The interface itself is intentionally left unchanged.
var _ StatsProvider = (*TracearrClient)(nil)

// NewTracearrClient creates a new Tracearr client.
func NewTracearrClient(cfg config.TracearrConfig) *TracearrClient {
	timeout := 30 * time.Second
	if cfg.Timeout != "" {
		if d, err := time.ParseDuration(cfg.Timeout); err == nil {
			timeout = d
		}
	}

	return &TracearrClient{
		baseURL:  cfg.URL,
		apiKey:   cfg.APIKey,
		serverID: cfg.ServerID,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

// tracearrHistoryRecord is a single HistoryRecord returned by the v2 API.
// rating_key is nullable; server_type distinguishes the originating media server.
type tracearrHistoryRecord struct {
	RatingKey  *string   `json:"rating_key"`
	ServerType string    `json:"server_type"`
	StartedAt  time.Time `json:"started_at"`
	DurationMS int       `json:"duration_ms"`
}

// tracearrHistoryMeta carries cursor pagination metadata. NextCursor is a
// pointer so a JSON null (or an absent field) both terminate pagination.
type tracearrHistoryMeta struct {
	NextCursor *string `json:"nextCursor"`
}

// tracearrHistoryResponse is the envelope returned by the history endpoint.
type tracearrHistoryResponse struct {
	Data []tracearrHistoryRecord `json:"data"`
	Meta tracearrHistoryMeta     `json:"meta"`
}

// GetHistory fetches the complete watch history from Tracearr, following
// meta.nextCursor until it is null/absent. itemIDs is accepted for interface
// compatibility but ignored — Tracearr returns bulk cursor-paginated history.
//
// Mapping: JellyfinItemID=rating_key, WatchedAt=started_at,
// PlaybackSeconds=duration_ms/1000. Rows with an empty/null rating_key or a
// server_type other than "jellyfin" are skipped.
func (c *TracearrClient) GetHistory(ctx context.Context, _ []string) ([]StatsHistoryItem, error) {
	// server_id is required when Tracearr is enabled (enforced by config
	// validation). Guarding here as well prevents history from multiple
	// Jellyfin servers being merged into one unscoped result if a config
	// bypasses validation (e.g. programmatic/hot-reload paths).
	if c.serverID == "" {
		return nil, fmt.Errorf("tracearr: server_id is required")
	}

	log.Debug().Str("url", c.baseURL).Msg("Fetching watch history from Tracearr")

	var items []StatsHistoryItem
	cursor := ""
	pages := 0

	for {
		if pages >= tracearrMaxPages {
			// Return the history gathered so far rather than discarding it —
			// a partial result is still usable, and callers should not lose
			// watch state because one page boundary was hit.
			log.Warn().
				Int("max_pages", tracearrMaxPages).
				Int("items_so_far", len(items)).
				Msg("Tracearr: reached max history pages; returning partial history")
			break
		}

		records, nextCursor, err := c.fetchHistoryPage(ctx, cursor)
		if err != nil {
			return nil, err
		}

		for _, rec := range records {
			// rating_key is the Jellyfin item GUID — without it the record is unusable.
			if rec.RatingKey == nil || *rec.RatingKey == "" {
				continue
			}
			// Defensive: only Jellyfin items are relevant to OxiCleanarr.
			if rec.ServerType != "jellyfin" {
				continue
			}
			// A JSON null/absent started_at decodes to the zero time; treat it
			// as invalid rather than recording an ancient watch date that would
			// make the item look long-unwatched and eligible for deletion.
			if rec.StartedAt.IsZero() {
				continue
			}

			items = append(items, StatsHistoryItem{
				JellyfinItemID:  *rec.RatingKey,
				WatchedAt:       rec.StartedAt,
				PlaybackSeconds: rec.DurationMS / 1000,
			})
		}

		pages++
		log.Debug().
			Int("page", pages).
			Int("records_on_page", len(records)).
			Msg("Fetched Tracearr history page")

		if nextCursor == "" {
			break
		}
		// Guard against a server echoing the same cursor: without this the
		// loop would issue up to tracearrMaxPages identical requests.
		if nextCursor == cursor {
			log.Warn().
				Int("page", pages).
				Msg("Tracearr: cursor did not advance; stopping pagination")
			break
		}
		cursor = nextCursor
	}

	log.Debug().
		Int("total_history_items", len(items)).
		Msg("Fetched all watch history from Tracearr")

	return items, nil
}

// fetchHistoryPage requests a single page of history and returns its records
// along with the next cursor (empty when pagination is exhausted).
func (c *TracearrClient) fetchHistoryPage(ctx context.Context, cursor string) ([]tracearrHistoryRecord, string, error) {
	endpoint, err := url.Parse(fmt.Sprintf("%s/api/v2/public/history", c.baseURL))
	if err != nil {
		return nil, "", fmt.Errorf("tracearr: parsing history URL: %w", err)
	}

	q := endpoint.Query()
	q.Set("pageSize", strconv.Itoa(tracearrHistoryPageSize))
	if cursor != "" {
		q.Set("cursor", cursor)
	}
	if c.serverID != "" {
		q.Set("serverId", c.serverID)
	}
	endpoint.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, "", fmt.Errorf("tracearr: creating request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("tracearr: making request to %s: %w", c.baseURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("tracearr: unexpected status code: %d", resp.StatusCode)
	}

	var result tracearrHistoryResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, "", fmt.Errorf("tracearr: decoding response: %w", err)
	}

	next := ""
	if result.Meta.NextCursor != nil {
		next = *result.Meta.NextCursor
	}

	return result.Data, next, nil
}

// Ping checks whether Tracearr is reachable by calling the public health
// endpoint. Only HTTP 200 is considered healthy.
func (c *TracearrClient) Ping(ctx context.Context) error {
	endpoint := fmt.Sprintf("%s/api/v2/public/health", c.baseURL)

	log.Debug().Str("url", c.baseURL).Msg("Pinging Tracearr")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("tracearr: creating request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("tracearr: making request to %s: %w", c.baseURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("tracearr: unexpected status code: %d", resp.StatusCode)
	}

	log.Debug().Msg("Tracearr ping successful")
	return nil
}
