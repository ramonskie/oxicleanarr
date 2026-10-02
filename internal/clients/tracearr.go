package clients

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
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

// TracearrClient handles communication with the Tracearr public API.
// Auth: Authorization: Bearer trr_pub_<base64url>.
// Health/discovery: GET /api/v1/public/health
// History: GET /api/v2/public/history?pageSize=100&cursor=<opaque>&serverId=<uuid>
//
// serverID may be left empty (zero-config). On first GetHistory the client
// auto-detects the sole Jellyfin media server from the health endpoint and
// caches the resolved UUID for its lifetime. An explicit serverID skips
// discovery entirely.
type TracearrClient struct {
	baseURL  string
	apiKey   string
	serverID string
	client   *http.Client

	mu               sync.Mutex // guards resolvedServerID
	resolvedServerID string     // cached auto-detected media-server UUID
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
		serverID: strings.TrimSpace(cfg.ServerID),
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

// resolveServerID returns the media-server UUID to scope history to. A
// configured server_id is used as-is and never triggers discovery. Otherwise
// the v1 health endpoint is queried once, the sole Jellyfin server is selected,
// cached, and reused on later calls. The mutex serialises concurrent first
// calls so discovery happens at most once.
func (c *TracearrClient) resolveServerID(ctx context.Context) (string, error) {
	if c.serverID != "" {
		return c.serverID, nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.resolvedServerID != "" {
		return c.resolvedServerID, nil
	}

	discovered, err := c.discoverJellyfinServer(ctx)
	if err != nil {
		return "", err
	}
	c.resolvedServerID = discovered
	return discovered, nil
}

// discoverJellyfinServer queries the v1 health endpoint and returns the UUID of
// the single Jellyfin server. Zero or multiple Jellyfin servers are an error
// because either would make watch-history scoping ambiguous.
func (c *TracearrClient) discoverJellyfinServer(ctx context.Context) (string, error) {
	servers, err := c.fetchHealthServers(ctx)
	if err != nil {
		return "", err
	}

	jellyfin := make([]tracearrHealthServer, 0, len(servers))
	for _, s := range servers {
		if strings.EqualFold(strings.TrimSpace(s.Type), "jellyfin") {
			jellyfin = append(jellyfin, s)
		}
	}

	switch len(jellyfin) {
	case 0:
		return "", fmt.Errorf("tracearr: no Jellyfin server found; configure one in Tracearr or set server_id explicitly")
	case 1:
		id := strings.TrimSpace(jellyfin[0].ID)
		if id == "" {
			return "", fmt.Errorf("tracearr: discovered Jellyfin server has no id; set server_id explicitly")
		}
		log.Debug().Str("server_id", id).Msg("Tracearr: auto-detected Jellyfin server")
		return id, nil
	default:
		names := make([]string, 0, len(jellyfin))
		for _, s := range jellyfin {
			names = append(names, fmt.Sprintf("%s (%s)", s.Name, strings.TrimSpace(s.ID)))
		}
		return "", fmt.Errorf("tracearr: multiple Jellyfin servers found (%s); set server_id explicitly", strings.Join(names, ", "))
	}
}

// fetchHealthServers calls GET /api/v1/public/health and returns its servers.
func (c *TracearrClient) fetchHealthServers(ctx context.Context) ([]tracearrHealthServer, error) {
	endpoint := fmt.Sprintf("%s/api/v1/public/health", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("tracearr: creating request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tracearr: making request to %s: %w", c.baseURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tracearr: unexpected status code: %d", resp.StatusCode)
	}

	var result tracearrHealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("tracearr: decoding response: %w", err)
	}

	return result.Servers, nil
}

// tracearrHistoryRecord is a single HistoryRecord returned by the v2 API.
// rating_key is nullable; server_type distinguishes the originating media server.
type tracearrHistoryRecord struct {
	RatingKey   *string   `json:"rating_key"`
	ServerID    string    `json:"server_id"`
	ServerType  string    `json:"server_type"`
	StartedAt   time.Time `json:"started_at"`
	DurationMS  int       `json:"duration_ms"`
	ReferenceID string    `json:"reference_id"` // resume-chain / play key
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

// tracearrHealthServer is one media-server entry from the v1 health endpoint.
// ID is the media-server UUID used as the history serverId/server_id.
type tracearrHealthServer struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Type          string `json:"type"`
	Online        bool   `json:"online"`
	Historical    bool   `json:"historical"`
	ActiveStreams int    `json:"activeStreams"`
}

// tracearrHealthResponse is the GET /api/v1/public/health envelope. The v2
// public API has no health endpoint; health is v1-only.
type tracearrHealthResponse struct {
	Status    string                 `json:"status"`
	Version   string                 `json:"version"`
	Timestamp string                 `json:"timestamp"`
	Servers   []tracearrHealthServer `json:"servers"`
}

// GetHistory fetches the complete watch history from Tracearr, following
// meta.nextCursor until it is null/absent. itemIDs is accepted for interface
// compatibility but ignored — Tracearr returns bulk cursor-paginated history.
//
// Mapping: JellyfinItemID=rating_key, WatchedAt=started_at,
// PlaybackSeconds=duration_ms/1000. Rows with an empty/null rating_key or a
// server_type other than "jellyfin" are skipped.
func (c *TracearrClient) GetHistory(ctx context.Context, _ []string) ([]StatsHistoryItem, error) {
	// Resolve the media server to scope history to. An explicit server_id wins;
	// otherwise the sole Jellyfin server is auto-detected from health and
	// cached so history from multiple servers is never merged unscoped.
	serverID, err := c.resolveServerID(ctx)
	if err != nil {
		return nil, err
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

		records, nextCursor, err := c.fetchHistoryPage(ctx, serverID, cursor)
		if err != nil {
			return nil, err
		}

		for _, rec := range records {
			// rating_key is the Jellyfin item GUID — without it the record is unusable.
			if rec.RatingKey == nil || *rec.RatingKey == "" {
				continue
			}
			// Defensive: only Jellyfin items are relevant to OxiCleanarr. Match
			// case-insensitively and ignore surrounding whitespace so a
			// differently-cased provider value is not silently dropped.
			if !strings.EqualFold(strings.TrimSpace(rec.ServerType), "jellyfin") {
				continue
			}
			// Record-level scope defence: if the API ignores the serverId query
			// and returns rows from other media servers, drop any row that names
			// a different server. An empty server_id is tolerated (older rows).
			if recServerID := strings.TrimSpace(rec.ServerID); recServerID != "" && recServerID != serverID {
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
				PlayID:          rec.ReferenceID,
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
// along with the next cursor (empty when pagination is exhausted). serverID is
// always non-empty — GetHistory resolves it (explicit or auto-detected) first.
func (c *TracearrClient) fetchHistoryPage(ctx context.Context, serverID, cursor string) ([]tracearrHistoryRecord, string, error) {
	endpoint, err := url.Parse(fmt.Sprintf("%s/api/v2/public/history", c.baseURL))
	if err != nil {
		return nil, "", fmt.Errorf("tracearr: parsing history URL: %w", err)
	}

	q := endpoint.Query()
	q.Set("pageSize", strconv.Itoa(tracearrHistoryPageSize))
	if cursor != "" {
		q.Set("cursor", cursor)
	}
	q.Set("serverId", serverID)
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

// Ping checks whether Tracearr is reachable by calling the v1 public health
// endpoint (the v2 public API has no health endpoint). Only HTTP 200 is
// considered healthy.
func (c *TracearrClient) Ping(ctx context.Context) error {
	endpoint := fmt.Sprintf("%s/api/v1/public/health", c.baseURL)

	log.Debug().Str("url", c.baseURL).Msg("Pinging Tracearr")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("tracearr: creating request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")

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
