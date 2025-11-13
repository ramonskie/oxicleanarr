package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
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
		// Simulate successful creation for all items in dry-run
		createdPaths := make([]string, len(items))
		for i, item := range items {
			createdPaths[i] = filepath.Join(item.TargetDirectory, filepath.Base(item.SourcePath))
		}
		return &PluginAddSymlinksResponse{
			Success:         true,
			CreatedSymlinks: createdPaths,
			Errors:          []string{},
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

	// Read response body for better error reporting
	bodyBytes, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, fmt.Errorf("reading response body: %w", readErr)
	}

	// Log full response for debugging
	log.Debug().
		Int("status_code", resp.StatusCode).
		Str("response_body", string(bodyBytes)).
		Msg("Plugin AddSymlinks response")

	// Check HTTP status code
	if resp.StatusCode != http.StatusOK {
		log.Error().
			Int("status_code", resp.StatusCode).
			Str("response_body", string(bodyBytes)).
			Msg("Plugin returned non-200 status code")
		return nil, fmt.Errorf("plugin returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var addResp PluginAddSymlinksResponse
	if err := json.Unmarshal(bodyBytes, &addResp); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	// HTTP 200 + valid JSON = success
	created := len(addResp.CreatedSymlinks)
	failed := len(addResp.Errors)

	log.Info().
		Bool("success", addResp.Success).
		Int("created", created).
		Int("failed", failed).
		Msg("Symlinks created via plugin")

	// Log individual errors if any
	if failed > 0 {
		for _, errMsg := range addResp.Errors {
			log.Warn().
				Str("error", errMsg).
				Msg("Symlink creation failed")
		}
	}

	return &addResp, nil
}

// RemoveSymlinks removes symlinks via the OxiCleanarr Bridge Plugin
func (c *JellyfinClient) RemoveSymlinks(ctx context.Context, paths []string, dryRun bool) (*PluginRemoveSymlinksResponse, error) {
	if dryRun {
		log.Info().
			Int("count", len(paths)).
			Msg("[DRY-RUN] Would remove symlinks via plugin")
		return &PluginRemoveSymlinksResponse{
			Success:         true,
			RemovedSymlinks: paths,
			Errors:          []string{},
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

	log.Debug().
		Str("request_body", string(bodyBytes)).
		Msg("RemoveSymlinks request body")

	log.Info().
		Int("count", len(paths)).
		Msg("Removing symlinks via OxiCleanarr Bridge Plugin")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("making request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body for better error reporting
	respBody, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, fmt.Errorf("reading response body: %w", readErr)
	}

	// Log response for debugging
	log.Debug().
		Int("status_code", resp.StatusCode).
		Str("response_body", string(respBody)).
		Msg("Plugin RemoveSymlinks response")

	// Check HTTP status code
	if resp.StatusCode != http.StatusOK {
		log.Error().
			Int("status_code", resp.StatusCode).
			Str("response_body", string(respBody)).
			Msg("Plugin returned non-200 status code")
		return nil, fmt.Errorf("plugin returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var removeResp PluginRemoveSymlinksResponse
	if err := json.Unmarshal(respBody, &removeResp); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	// HTTP 200 + valid JSON = success
	removed := len(removeResp.RemovedSymlinks)
	failed := len(removeResp.Errors)

	log.Info().
		Bool("success", removeResp.Success).
		Int("removed", removed).
		Int("failed", failed).
		Msg("Symlinks removed via plugin")

	// Log individual errors if any
	if failed > 0 {
		for _, errMsg := range removeResp.Errors {
			log.Warn().
				Str("error", errMsg).
				Msg("Symlink removal failed")
		}
	}

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

	// Read response body for better error reporting
	respBody, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, fmt.Errorf("reading response body: %w", readErr)
	}

	// Log response for debugging
	log.Debug().
		Int("status_code", resp.StatusCode).
		Str("response_body", string(respBody)).
		Msg("Plugin ListSymlinks response")

	// Check HTTP status code
	if resp.StatusCode != http.StatusOK {
		log.Error().
			Int("status_code", resp.StatusCode).
			Str("response_body", string(respBody)).
			Msg("Plugin returned non-200 status code")
		return nil, fmt.Errorf("plugin returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var listResp PluginListSymlinksResponse
	if err := json.Unmarshal(respBody, &listResp); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	// HTTP 200 + valid JSON = success. Plugin may omit Success field.

	log.Debug().
		Int("count", len(listResp.Symlinks)).
		Str("directory", directory).
		Msg("Symlinks listed successfully via plugin")

	return &listResp, nil
}

// CreateDirectory creates a directory via the OxiCleanarr plugin
func (c *JellyfinClient) CreateDirectory(ctx context.Context, path string, dryRun bool) (*PluginCreateDirectoryResponse, error) {
	log := log.With().Str("client", "jellyfin").Str("operation", "create_directory").Logger()

	if dryRun {
		log.Info().Str("path", path).Msg("DRY RUN: Would create directory via plugin")
		return &PluginCreateDirectoryResponse{
			Success:   true,
			Directory: path,
			Created:   false,
			Message:   "dry-run mode",
		}, nil
	}

	// Prepare request
	reqBody := PluginCreateDirectoryRequest{
		Directory: path,
	}

	reqBytes, err := json.Marshal(reqBody)
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal create directory request")
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// Build URL
	pluginURL := fmt.Sprintf("%s/api/oxicleanarr/directories/create", c.baseURL)

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", pluginURL, bytes.NewReader(reqBytes))
	if err != nil {
		log.Error().Err(err).Msg("Failed to create directory request")
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Emby-Token", c.apiKey)

	log.Debug().Str("url", pluginURL).Str("path", path).Msg("Creating directory via plugin")

	// Execute request
	resp, err := c.client.Do(req)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create directory via plugin")
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Error().Err(err).Msg("Failed to read create directory response")
		return nil, fmt.Errorf("read response: %w", err)
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		log.Error().
			Int("status_code", resp.StatusCode).
			Str("response", string(body)).
			Msg("Plugin returned error status for directory creation")
		return nil, fmt.Errorf("plugin returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var createResp PluginCreateDirectoryResponse
	if err := json.Unmarshal(body, &createResp); err != nil {
		log.Error().Err(err).Str("body", string(body)).Msg("Failed to parse create directory response")
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	// HTTP 200 + valid JSON = success. Plugin may omit Success field.

	log.Info().
		Str("path", path).
		Bool("created", createResp.Created).
		Str("message", createResp.Message).
		Msg("Directory created successfully via plugin")

	return &createResp, nil
}

// DeleteDirectory deletes a directory via the OxiCleanarr plugin
func (c *JellyfinClient) DeleteDirectory(ctx context.Context, path string, force bool, dryRun bool) (*PluginDeleteDirectoryResponse, error) {
	log := log.With().Str("client", "jellyfin").Str("operation", "delete_directory").Logger()

	if dryRun {
		log.Info().Str("path", path).Bool("force", force).Msg("DRY RUN: Would delete directory via plugin")
		return &PluginDeleteDirectoryResponse{
			Success:   true,
			Directory: path,
			Deleted:   false,
			Message:   "dry-run mode",
		}, nil
	}

	// Prepare request
	reqBody := PluginDeleteDirectoryRequest{
		Directory: path,
		Force:     force,
	}

	reqBytes, err := json.Marshal(reqBody)
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal delete directory request")
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// Build URL
	pluginURL := fmt.Sprintf("%s/api/oxicleanarr/directories/remove", c.baseURL)

	// Create request
	req, err := http.NewRequestWithContext(ctx, "DELETE", pluginURL, bytes.NewReader(reqBytes))
	if err != nil {
		log.Error().Err(err).Msg("Failed to create delete directory request")
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Emby-Token", c.apiKey)

	log.Debug().Str("url", pluginURL).Str("path", path).Bool("force", force).Msg("Deleting directory via plugin")

	// Execute request
	resp, err := c.client.Do(req)
	if err != nil {
		log.Error().Err(err).Msg("Failed to delete directory via plugin")
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Error().Err(err).Msg("Failed to read delete directory response")
		return nil, fmt.Errorf("read response: %w", err)
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		log.Error().
			Int("status_code", resp.StatusCode).
			Str("response", string(body)).
			Msg("Plugin returned error status for directory deletion")
		return nil, fmt.Errorf("plugin returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var deleteResp PluginDeleteDirectoryResponse
	if err := json.Unmarshal(body, &deleteResp); err != nil {
		log.Error().Err(err).Str("body", string(body)).Msg("Failed to parse delete directory response")
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	// HTTP 200 + valid JSON = success. Plugin may omit Success field.

	log.Info().
		Str("path", path).
		Bool("deleted", deleteResp.Deleted).
		Str("message", deleteResp.Message).
		Msg("Directory deleted successfully via plugin")

	return &deleteResp, nil
}
