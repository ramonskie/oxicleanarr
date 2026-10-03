package clients

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ramonskie/oxicleanarr/internal/config"
	"github.com/rs/zerolog/log"
)

// JellyfinClient handles communication with Jellyfin API
type JellyfinClient struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

// ErrImageNotFound is returned by GetItemImage when the item has no image of
// the requested type (Jellyfin responds 404). Callers can treat it as "nothing
// to draw on" rather than an infrastructure failure.
var ErrImageNotFound = errors.New("image not found")

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

// setAuth applies Jellyfin's supported authentication scheme to a request.
// The MediaBrowser Authorization header is the only form honored across
// Jellyfin 10.9–12.x. Legacy headers (X-Emby-Token, X-MediaBrowser-Token,
// X-Emby-Authorization) are ignored once EnableLegacyAuthorization is off,
// which is the default in Jellyfin 12, and surface as HTTP 401.
func (c *JellyfinClient) setAuth(req *http.Request) {
	req.Header.Set("Authorization", `MediaBrowser Token="`+c.apiKey+`"`)
}

// GetMovies fetches all movies from Jellyfin
func (c *JellyfinClient) GetMovies(ctx context.Context) ([]JellyfinItem, error) {
	return c.getItems(ctx, "Movie")
}

// GetTVShows fetches all TV shows from Jellyfin
func (c *JellyfinClient) GetTVShows(ctx context.Context) ([]JellyfinItem, error) {
	return c.getItems(ctx, "Series")
}

// getItems fetches items of a specific type.
//
// CollapseBoxSetItems=false is required: Jellyfin defaults this to true for
// movie queries when unset, which hides every movie that belongs to a
// collection/box set. Those items then fail to match and are reported as
// unmatched (observed on the live Jellyfin 12.1.0 server: 314 vs 372 movies).
func (c *JellyfinClient) getItems(ctx context.Context, itemType string) ([]JellyfinItem, error) {
	url := fmt.Sprintf("%s/Items?IncludeItemTypes=%s&Recursive=true&CollapseBoxSetItems=false&Fields=Path,DateCreated,ProviderIds,RunTimeTicks",
		c.baseURL, itemType)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	c.setAuth(req)
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
	reqURL := fmt.Sprintf("%s/Users/%s/Items/%s",
		c.baseURL, url.PathEscape(userID), url.PathEscape(itemID))

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	c.setAuth(req)
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
	reqURL := fmt.Sprintf("%s/Items/%s", c.baseURL, url.PathEscape(itemID))

	req, err := http.NewRequestWithContext(ctx, "DELETE", reqURL, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	c.setAuth(req)

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
	url := fmt.Sprintf("%s/System/Info", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	c.setAuth(req)

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

// RefreshLibrary triggers a library scan in Jellyfin to discover new content.
// Called after deletions so Jellyfin picks up removed files.
func (c *JellyfinClient) RefreshLibrary(ctx context.Context, dryRun bool) error {
	if dryRun {
		log.Info().
			Msg("[DRY-RUN] Would trigger library refresh in Jellyfin")
		return nil
	}

	reqURL := fmt.Sprintf("%s/Library/Refresh", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, "POST", reqURL, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	c.setAuth(req)

	log.Debug().Msg("Triggering library refresh in Jellyfin")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	log.Debug().Msg("Library refresh triggered successfully in Jellyfin")

	return nil
}

// Image Proxy Methods

// ProxyImage fetches an image from Jellyfin and returns the response body and content type.
// The caller is responsible for closing the returned ReadCloser.
// maxWidth and quality are optional sizing hints (pass 0 to omit).
func (c *JellyfinClient) ProxyImage(ctx context.Context, itemID, imageType string, maxWidth, quality int) (io.ReadCloser, string, error) {
	imgURL := fmt.Sprintf("%s/Items/%s/Images/%s", c.baseURL, url.PathEscape(itemID), url.PathEscape(imageType))

	if maxWidth > 0 {
		imgURL += fmt.Sprintf("?maxWidth=%d", maxWidth)
	}
	if quality > 0 {
		if maxWidth > 0 {
			imgURL += fmt.Sprintf("&quality=%d", quality)
		} else {
			imgURL += fmt.Sprintf("?quality=%d", quality)
		}
	}

	req, err := http.NewRequestWithContext(ctx, "GET", imgURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("creating image request: %w", err)
	}

	c.setAuth(req)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("fetching image: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, "", fmt.Errorf("image not found (status %d)", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "image/jpeg"
	}

	// Limit response body to 10MB to prevent unbounded reads
	const maxImageSize = 10 << 20 // 10MB
	limitedBody := struct {
		io.Reader
		io.Closer
	}{
		Reader: io.LimitReader(resp.Body, maxImageSize),
		Closer: resp.Body,
	}

	return limitedBody, contentType, nil
}

// GetItemImage downloads the full bytes of an item image (e.g. "Primary").
// Requests the JPG format so callers can rely on a known content type; returns
// the image bytes and the response content type. Errors when the image cannot
// be fetched (404 included).
func (c *JellyfinClient) GetItemImage(ctx context.Context, itemID, imageType string) ([]byte, string, error) {
	imgURL := fmt.Sprintf("%s/Items/%s/Images/%s?format=Jpg", c.baseURL, url.PathEscape(itemID), url.PathEscape(imageType))

	req, err := http.NewRequestWithContext(ctx, "GET", imgURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("creating image request: %w", err)
	}

	c.setAuth(req)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("fetching image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, "", ErrImageNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("image not found (status %d)", resp.StatusCode)
	}

	const maxImageSize = 10 << 20 // 10MB
	if resp.ContentLength > maxImageSize {
		return nil, "", fmt.Errorf("image too large (%d bytes, limit %d)", resp.ContentLength, maxImageSize)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxImageSize))
	if err != nil {
		return nil, "", fmt.Errorf("reading image body: %w", err)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "image/jpeg"
	}

	return data, contentType, nil
}

// SetItemImage uploads a new image of the given type for an item.
//
// The body is sent base64-encoded: the Jellyfin server rejects raw binary
// payloads on this endpoint with a 500, despite the OpenAPI description hinting
// at image/* binary. Base64 is the empirically-verified working path (see
// jellyfin/jellyfin#12447), used by Maintainerr's overlay feature.
func (c *JellyfinClient) SetItemImage(ctx context.Context, itemID, imageType string, data []byte, contentType string) error {
	imgURL := fmt.Sprintf("%s/Items/%s/Images/%s", c.baseURL, url.PathEscape(itemID), url.PathEscape(imageType))

	body := base64.StdEncoding.EncodeToString(data)

	req, err := http.NewRequestWithContext(ctx, "POST", imgURL, strings.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating image request: %w", err)
	}

	c.setAuth(req)
	req.Header.Set("Content-Type", contentType)

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("uploading image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("upload failed (status %d)", resp.StatusCode)
	}

	log.Info().Str("item_id", itemID).Str("image_type", imageType).Msg("Set item image")
	return nil
}

// RemoteSearchInfo is the search criteria Jellyfin accepts inside a
// remote-search query. ProviderIds, when set, pins an exact external identity
// (e.g. {"Tvdb": "75579"}) and yields the strongest candidates.
type RemoteSearchInfo struct {
	Name        string            `json:"Name"`
	Year        int               `json:"Year,omitempty"`
	ProviderIds map[string]string `json:"ProviderIds,omitempty"`
}

// RemoteSearchQuery is the request body for the RemoteSearch endpoints. It
// mirrors Jellyfin's MovieInfoRemoteSearchQuery / SeriesInfoRemoteSearchQuery
// shapes for the fields this client needs.
type RemoteSearchQuery struct {
	SearchInfo RemoteSearchInfo `json:"SearchInfo"`
}

// RemoteSearchResult is a single candidate returned by a RemoteSearch call.
type RemoteSearchResult struct {
	Name               string            `json:"Name"`
	ProviderIds        map[string]string `json:"ProviderIds"`
	ProductionYear     int               `json:"ProductionYear"`
	PremiereDate       string            `json:"PremiereDate,omitempty"`
	ImageURL           string            `json:"ImageUrl,omitempty"`
	SearchProviderName string            `json:"SearchProviderName,omitempty"`
}

// RemoteSearchMovie looks up movie candidates for the given name/year/provider
// IDs using Jellyfin's configured metadata providers. A successful call always
// returns 200 with a (possibly empty) candidate list.
func (c *JellyfinClient) RemoteSearchMovie(ctx context.Context, name string, year int, providerIDs map[string]string) ([]RemoteSearchResult, error) {
	return c.remoteSearch(ctx, "Movie", name, year, providerIDs)
}

// RemoteSearchSeries looks up series candidates for the given name/year/
// provider IDs using Jellyfin's configured metadata providers.
func (c *JellyfinClient) RemoteSearchSeries(ctx context.Context, name string, year int, providerIDs map[string]string) ([]RemoteSearchResult, error) {
	return c.remoteSearch(ctx, "Series", name, year, providerIDs)
}

// canonicalProviderIDKey returns a provider id key in the casing Jellyfin's
// ItemLookupInfo.ProviderIds expects. Jellyfin's provider-id pin is
// case-sensitive and its keys are PascalCase, so a caller-supplied lowercase
// key (the shape the sync service builds from arr refs, e.g. {"tmdb": "..."})
// is otherwise silently ignored server-side and the pin does not bind.
// Unknown providers fall back to first-letter capitalization.
func canonicalProviderIDKey(key string) string {
	switch strings.ToLower(key) {
	case "tmdb":
		return "Tmdb"
	case "tvdb":
		return "Tvdb"
	case "imdb":
		return "Imdb"
	}
	if key == "" {
		return key
	}
	return strings.ToUpper(key[:1]) + key[1:]
}

// canonicalProviderIDs returns a copy of ids with each key canonicalized to
// Jellyfin's expected casing. Empty input yields nil so the omitempty tag still
// drops ProviderIds from the request. A duplicate after canonicalization keeps
// one value (map iteration order is not significant to Jellyfin).
func canonicalProviderIDs(ids map[string]string) map[string]string {
	if len(ids) == 0 {
		return nil
	}
	out := make(map[string]string, len(ids))
	for key, value := range ids {
		out[canonicalProviderIDKey(key)] = value
	}
	return out
}

// remoteSearch posts a remote-search query and decodes the candidate list.
func (c *JellyfinClient) remoteSearch(ctx context.Context, kind, name string, year int, providerIDs map[string]string) ([]RemoteSearchResult, error) {
	reqURL := fmt.Sprintf("%s/Items/RemoteSearch/%s", c.baseURL, kind)

	payload, err := json.Marshal(RemoteSearchQuery{
		SearchInfo: RemoteSearchInfo{
			Name:        name,
			Year:        year,
			ProviderIds: canonicalProviderIDs(providerIDs),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("encoding remote search request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", reqURL, strings.NewReader(string(payload)))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	c.setAuth(req)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("remote search %s: unexpected status code: %d", kind, resp.StatusCode)
	}

	var results []RemoteSearchResult
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	log.Debug().
		Str("type", kind).
		Str("name", name).
		Int("year", year).
		Int("count", len(results)).
		Msg("Remote search returned candidates")

	return results, nil
}

// ApplyRemoteSearch applies a chosen RemoteSearchResult to an existing library
// item, re-identifying it with the result's metadata. Jellyfin responds 204 No
// Content on success and refreshes the item's metadata as a side effect.
//
// replaceAllImages controls whether existing images are replaced by the
// match's remote artwork; pass false to preserve current images.
func (c *JellyfinClient) ApplyRemoteSearch(ctx context.Context, itemID string, result RemoteSearchResult, replaceAllImages bool) error {
	reqURL := fmt.Sprintf("%s/Items/RemoteSearch/Apply/%s?replaceAllImages=%t",
		c.baseURL, url.PathEscape(itemID), replaceAllImages)

	payload, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("encoding remote search result: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", reqURL, strings.NewReader(string(payload)))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	c.setAuth(req)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("apply remote search: unexpected status code: %d", resp.StatusCode)
	}

	log.Info().
		Str("item_id", itemID).
		Bool("replace_all_images", replaceAllImages).
		Msg("Applied remote search match to Jellyfin item")

	return nil
}

// JellyfinEpisode is a normalized episode row returned by GetEpisodes. Jellyfin
// encodes the season in ParentIndexNumber and the episode number in IndexNumber.
type JellyfinEpisode struct {
	ID                string            `json:"Id"`
	Name              string            `json:"Name"`
	SeriesID          string            `json:"SeriesId"`
	Path              string            `json:"Path"`
	ParentIndexNumber int               `json:"ParentIndexNumber"`
	IndexNumber       int               `json:"IndexNumber"`
	ProviderIds       map[string]string `json:"ProviderIds"`
}

// GetEpisodes fetches every episode of a Jellyfin series. It is read-only and
// exists so the match adjudicator can compare episode titles by (season, episode)
// against the arr side. Provider IDs and paths are requested for evidence.
func (c *JellyfinClient) GetEpisodes(ctx context.Context, seriesID string) ([]JellyfinEpisode, error) {
	reqURL := fmt.Sprintf("%s/Shows/%s/Episodes?Fields=Path,ProviderIds", c.baseURL, url.PathEscape(seriesID))

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	c.setAuth(req)
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result struct {
		Items []JellyfinEpisode `json:"Items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	log.Debug().
		Str("series_id", seriesID).
		Int("count", len(result.Items)).
		Msg("Fetched episodes from Jellyfin")

	return result.Items, nil
}
