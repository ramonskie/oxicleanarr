package clients

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
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
	url := fmt.Sprintf("%s/Items?IncludeItemTypes=%s&Recursive=true&Fields=Path,DateCreated,ProviderIds",
		c.baseURL, itemType)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("X-Emby-Token", c.apiKey)
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
	url := fmt.Sprintf("%s/Users/%s/Items/%s",
		c.baseURL, userID, itemID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("X-Emby-Token", c.apiKey)
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
	url := fmt.Sprintf("%s/Items/%s", c.baseURL, itemID)

	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("X-Emby-Token", c.apiKey)

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

	req.Header.Set("X-Emby-Token", c.apiKey)

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

	req.Header.Set("X-Emby-Token", c.apiKey)

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
	imgURL := fmt.Sprintf("%s/Items/%s/Images/%s", c.baseURL, itemID, imageType)

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

	req.Header.Set("X-Emby-Token", c.apiKey)

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
	imgURL := fmt.Sprintf("%s/Items/%s/Images/%s?format=Jpg", c.baseURL, itemID, imageType)

	req, err := http.NewRequestWithContext(ctx, "GET", imgURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("creating image request: %w", err)
	}

	req.Header.Set("X-Emby-Token", c.apiKey)

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
	imgURL := fmt.Sprintf("%s/Items/%s/Images/%s", c.baseURL, itemID, imageType)

	body := base64.StdEncoding.EncodeToString(data)

	req, err := http.NewRequestWithContext(ctx, "POST", imgURL, strings.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating image request: %w", err)
	}

	req.Header.Set("X-Emby-Token", c.apiKey)
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
