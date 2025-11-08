package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
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

// GetVirtualFolders lists all virtual folders (libraries) in Jellyfin
func (c *JellyfinClient) GetVirtualFolders(ctx context.Context) ([]JellyfinVirtualFolder, error) {
	reqURL := fmt.Sprintf("%s/Library/VirtualFolders?api_key=%s", c.baseURL, c.apiKey)

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
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

	var folders []JellyfinVirtualFolder
	if err := json.NewDecoder(resp.Body).Decode(&folders); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	log.Debug().
		Int("count", len(folders)).
		Msg("Fetched virtual folders from Jellyfin")

	return folders, nil
}

// CreateVirtualFolder creates a new virtual folder (library) in Jellyfin
func (c *JellyfinClient) CreateVirtualFolder(ctx context.Context, name, collectionType string, paths []string, dryRun bool) error {
	if dryRun {
		log.Info().
			Str("library_name", name).
			Str("collection_type", collectionType).
			Strs("paths", paths).
			Msg("[DRY-RUN] Would create virtual folder in Jellyfin")
		return nil
	}

	// Build query parameters
	params := url.Values{}
	params.Set("name", name)
	params.Set("collectionType", collectionType)
	params.Set("refreshLibrary", "false") // Don't auto-scan, we'll manage content

	// Add paths to query
	for _, path := range paths {
		params.Add("paths", path)
	}

	reqURL := fmt.Sprintf("%s/Library/VirtualFolders?%s&api_key=%s", c.baseURL, params.Encode(), c.apiKey)

	req, err := http.NewRequestWithContext(ctx, "POST", reqURL, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	log.Info().
		Str("library_name", name).
		Str("collection_type", collectionType).
		Strs("paths", paths).
		Msg("Creating virtual folder in Jellyfin")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	log.Info().
		Str("library_name", name).
		Str("collection_type", collectionType).
		Msg("Created virtual folder in Jellyfin")

	return nil
}

// DeleteVirtualFolder deletes a virtual folder (library) by name
func (c *JellyfinClient) DeleteVirtualFolder(ctx context.Context, name string, dryRun bool) error {
	if dryRun {
		log.Info().
			Str("library_name", name).
			Msg("[DRY-RUN] Would delete virtual folder from Jellyfin")
		return nil
	}

	reqURL := fmt.Sprintf("%s/Library/VirtualFolders?name=%s&api_key=%s",
		c.baseURL, url.QueryEscape(name), c.apiKey)

	req, err := http.NewRequestWithContext(ctx, "DELETE", reqURL, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	log.Info().
		Str("library_name", name).
		Msg("Deleting virtual folder from Jellyfin")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	log.Info().
		Str("library_name", name).
		Msg("Deleted virtual folder from Jellyfin")

	return nil
}

// AddPathToVirtualFolder adds a path to an existing virtual folder
func (c *JellyfinClient) AddPathToVirtualFolder(ctx context.Context, name, path string, dryRun bool) error {
	if dryRun {
		log.Info().
			Str("library_name", name).
			Str("path", path).
			Msg("[DRY-RUN] Would add path to virtual folder in Jellyfin")
		return nil
	}

	// Build request body
	type addPathRequest struct {
		Path string `json:"Path"`
	}
	reqBody := addPathRequest{Path: path}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshaling request: %w", err)
	}

	reqURL := fmt.Sprintf("%s/Library/VirtualFolders/Paths?name=%s&api_key=%s",
		c.baseURL, url.QueryEscape(name), c.apiKey)

	req, err := http.NewRequestWithContext(ctx, "POST", reqURL, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	log.Info().
		Str("library_name", name).
		Str("path", path).
		Msg("Adding path to virtual folder in Jellyfin")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("making request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body for debugging
	_ = bodyBytes // Use the variable

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	log.Info().
		Str("library_name", name).
		Str("path", path).
		Msg("Added path to virtual folder in Jellyfin")

	return nil
}

// RefreshLibrary triggers a library scan in Jellyfin to discover new content
// This should be called after creating symlinks to make content visible
func (c *JellyfinClient) RefreshLibrary(ctx context.Context, dryRun bool) error {
	if dryRun {
		log.Info().
			Msg("[DRY-RUN] Would trigger library refresh in Jellyfin")
		return nil
	}

	reqURL := fmt.Sprintf("%s/Library/Refresh?api_key=%s", c.baseURL, c.apiKey)

	req, err := http.NewRequestWithContext(ctx, "POST", reqURL, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	log.Info().Msg("Triggering library refresh in Jellyfin")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	log.Info().Msg("Library refresh triggered successfully in Jellyfin")

	return nil
}

// OxiCleanarr Bridge Plugin Methods
// These methods communicate with the Jellyfin OxiCleanarr Bridge Plugin
// for managing symlinks without direct filesystem access

// CheckPluginStatus checks if the OxiCleanarr Bridge Plugin is installed and responsive
func (c *JellyfinClient) CheckPluginStatus(ctx context.Context) (*PluginStatusResponse, error) {
	reqURL := fmt.Sprintf("%s/api/oxicleanarr/status?api_key=%s", c.baseURL, c.apiKey)

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	log.Debug().Msg("Checking OxiCleanarr Bridge Plugin status")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("plugin not available (status %d)", resp.StatusCode)
	}

	var statusResp PluginStatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&statusResp); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	log.Info().
		Str("version", statusResp.Version).
		Msg("OxiCleanarr Bridge Plugin is available")

	return &statusResp, nil
}

// AddSymlinks creates symlinks via the OxiCleanarr Bridge Plugin
func (c *JellyfinClient) AddSymlinks(ctx context.Context, items []PluginSymlinkItem, dryRun bool) (*PluginAddSymlinksResponse, error) {
	if dryRun {
		log.Info().
			Int("count", len(items)).
			Msg("[DRY-RUN] Would create symlinks via plugin")
		return &PluginAddSymlinksResponse{
			Success: true,
			Created: len(items),
			Skipped: 0,
			Failed:  0,
		}, nil
	}

	reqBody := PluginAddSymlinksRequest{
		Items:  items,
		DryRun: false,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	reqURL := fmt.Sprintf("%s/api/oxicleanarr/symlinks/add?api_key=%s", c.baseURL, c.apiKey)

	req, err := http.NewRequestWithContext(ctx, "POST", reqURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	log.Info().
		Int("count", len(items)).
		Msg("Creating symlinks via OxiCleanarr Bridge Plugin")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("making request: %w", err)
	}
	defer resp.Body.Close()

	var addResp PluginAddSymlinksResponse
	if err := json.NewDecoder(resp.Body).Decode(&addResp); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	if !addResp.Success {
		return &addResp, fmt.Errorf("plugin error: %s", addResp.ErrorMessage)
	}

	log.Info().
		Int("created", addResp.Created).
		Int("skipped", addResp.Skipped).
		Int("failed", addResp.Failed).
		Msg("Symlinks created successfully via plugin")

	return &addResp, nil
}

// RemoveSymlinks removes symlinks via the OxiCleanarr Bridge Plugin
func (c *JellyfinClient) RemoveSymlinks(ctx context.Context, paths []string, dryRun bool) (*PluginRemoveSymlinksResponse, error) {
	if dryRun {
		log.Info().
			Int("count", len(paths)).
			Msg("[DRY-RUN] Would remove symlinks via plugin")
		return &PluginRemoveSymlinksResponse{
			Success: true,
			Removed: len(paths),
			Failed:  0,
		}, nil
	}

	reqBody := PluginRemoveSymlinksRequest{
		Paths:  paths,
		DryRun: false,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	reqURL := fmt.Sprintf("%s/api/oxicleanarr/symlinks/remove?api_key=%s", c.baseURL, c.apiKey)

	req, err := http.NewRequestWithContext(ctx, "POST", reqURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	log.Info().
		Int("count", len(paths)).
		Msg("Removing symlinks via OxiCleanarr Bridge Plugin")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("making request: %w", err)
	}
	defer resp.Body.Close()

	var removeResp PluginRemoveSymlinksResponse
	if err := json.NewDecoder(resp.Body).Decode(&removeResp); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	if !removeResp.Success {
		return &removeResp, fmt.Errorf("plugin error: %s", removeResp.ErrorMessage)
	}

	log.Info().
		Int("removed", removeResp.Removed).
		Int("failed", removeResp.Failed).
		Msg("Symlinks removed successfully via plugin")

	return &removeResp, nil
}

// ListSymlinks lists symlinks in a directory via the OxiCleanarr Bridge Plugin
func (c *JellyfinClient) ListSymlinks(ctx context.Context, directory string) (*PluginListSymlinksResponse, error) {
	reqURL := fmt.Sprintf("%s/api/oxicleanarr/symlinks/list?directory=%s&api_key=%s",
		c.baseURL, url.QueryEscape(directory), c.apiKey)

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	log.Debug().
		Str("directory", directory).
		Msg("Listing symlinks via OxiCleanarr Bridge Plugin")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("making request: %w", err)
	}
	defer resp.Body.Close()

	var listResp PluginListSymlinksResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	if !listResp.Success {
		return &listResp, fmt.Errorf("plugin error: %s", listResp.ErrorMessage)
	}

	log.Debug().
		Int("count", len(listResp.Symlinks)).
		Str("directory", directory).
		Msg("Symlinks listed successfully via plugin")

	return &listResp, nil
}
