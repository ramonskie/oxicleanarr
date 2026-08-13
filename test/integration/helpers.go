package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

const (
	OxiCleanarrURL = "http://localhost:9709"
	JellyfinURL    = "http://localhost:8096"

	// OxiCleanarr admin credentials (configured in test config.yaml)
	AdminUsername = "admin"
	AdminPassword = "adminpassword"

	// Jellyfin admin credentials (Jellyfin defaults for initial setup)
	JellyfinAdminUser = "admin"
	JellyfinAdminPass = "adminpassword"

	MaxSyncWait  = 60 * time.Second
	PollInterval = 1 * time.Second
)

// TestClient wraps HTTP operations for integration tests
type TestClient struct {
	baseURL string
	token   string
	client  *http.Client
	t       *testing.T
}

// NewTestClient creates a new test client
func NewTestClient(t *testing.T, baseURL string) *TestClient {
	return &TestClient{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		t: t,
	}
}

// Authenticate gets a JWT token from OxiCleanarr
func (tc *TestClient) Authenticate(username, password string) {
	reqBody := map[string]string{
		"username": username,
		"password": password,
	}

	body, err := json.Marshal(reqBody)
	require.NoError(tc.t, err)

	resp, err := tc.client.Post(
		tc.baseURL+"/api/auth/login",
		"application/json",
		bytes.NewReader(body),
	)
	require.NoError(tc.t, err)
	defer resp.Body.Close()

	require.Equal(tc.t, http.StatusOK, resp.StatusCode, "Authentication failed")

	var result struct {
		Token string `json:"token"`
	}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(tc.t, err)
	require.NotEmpty(tc.t, result.Token, "Token should not be empty")

	tc.token = result.Token
	tc.t.Logf("Authenticated successfully")
}

// Get performs a GET request with auth
func (tc *TestClient) Get(path string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, tc.baseURL+path, nil)
	if err != nil {
		return nil, err
	}

	if tc.token != "" {
		req.Header.Set("Authorization", "Bearer "+tc.token)
	}

	return tc.client.Do(req)
}

// Post performs a POST request with auth
func (tc *TestClient) Post(path string, body interface{}) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequest(http.MethodPost, tc.baseURL+path, bodyReader)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	if tc.token != "" {
		req.Header.Set("Authorization", "Bearer "+tc.token)
	}

	return tc.client.Do(req)
}

// TriggerSync starts a full sync and waits for completion
func (tc *TestClient) TriggerSync() {
	tc.t.Logf("Triggering full sync...")

	resp, err := tc.Post("/api/sync/full", nil)
	require.NoError(tc.t, err)
	defer resp.Body.Close()

	require.Equal(tc.t, http.StatusAccepted, resp.StatusCode, "Failed to start sync")

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(tc.t, err)

	message, ok := result["message"].(string)
	require.True(tc.t, ok && strings.Contains(message, "started"), "Sync did not start")

	tc.t.Logf("Sync started, waiting for completion...")

	// Wait for sync to complete
	ctx, cancel := context.WithTimeout(context.Background(), MaxSyncWait)
	defer cancel()

	ticker := time.NewTicker(PollInterval)
	defer ticker.Stop()

	startTime := time.Now()
	lastLog := time.Now()

	for {
		select {
		case <-ctx.Done():
			require.Failf(tc.t, "Sync timed out", "Sync timed out after %v", MaxSyncWait)
		case <-ticker.C:
			resp, err := tc.Get("/api/sync/status")
			require.NoError(tc.t, err)

			var status struct {
				Running     bool `json:"running"`
				InProgress  bool `json:"in_progress"`
			}
			err = json.NewDecoder(resp.Body).Decode(&status)
			resp.Body.Close()
			require.NoError(tc.t, err)

			// Wait for sync operation to complete (not just scheduler state)
			if !status.InProgress {
				elapsed := time.Since(startTime)
				tc.t.Logf("Sync completed in %v", elapsed.Round(100*time.Millisecond))
				return
			}

			// Log progress every 10 seconds
			if time.Since(lastLog) >= 10*time.Second {
				elapsed := time.Since(startTime)
				tc.t.Logf("Still syncing... (%v elapsed)", elapsed.Round(time.Second))
				lastLog = time.Now()
			}
		}
	}
}

// GetMovieCount returns the number of movies in the library
func (tc *TestClient) GetMovieCount() int {
	resp, err := tc.Get("/api/media/movies")
	require.NoError(tc.t, err)
	defer resp.Body.Close()

	require.Equal(tc.t, http.StatusOK, resp.StatusCode)

	var result struct {
		Items []interface{} `json:"items"`
	}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(tc.t, err)

	return len(result.Items)
}

// GetTVShowCount returns the number of TV shows in the library
func (tc *TestClient) GetTVShowCount() int {
	resp, err := tc.Get("/api/media/shows")
	require.NoError(tc.t, err)
	defer resp.Body.Close()

	require.Equal(tc.t, http.StatusOK, resp.StatusCode)

	var result struct {
		Items []interface{} `json:"items"`
	}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(tc.t, err)

	return len(result.Items)
}

// GetScheduledCount returns the number of items scheduled for deletion
func (tc *TestClient) GetScheduledCount() int {
	resp, err := tc.Get("/api/media/leaving-soon")
	require.NoError(tc.t, err)
	defer resp.Body.Close()

	require.Equal(tc.t, http.StatusOK, resp.StatusCode)

	var result struct {
		Items []interface{} `json:"items"`
	}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(tc.t, err)

	return len(result.Items)
}

// TriggerSyncAndGetSummary triggers a full sync, waits for it to complete,
// then fetches the latest job and returns its summary map.
// This is the correct way to count overdue items (scheduled_deletions) and
// inspect would_delete candidates, since /api/media/leaving-soon only shows
// items within the leaving_soon_days window (not yet overdue).
func (tc *TestClient) TriggerSyncAndGetSummary() map[string]interface{} {
	tc.t.Helper()
	tc.TriggerSync()

	job, err := tc.WaitForJobCompletion(30 * time.Second)
	require.NoError(tc.t, err, "Failed to wait for job completion")

	summary, ok := job["summary"].(map[string]interface{})
	require.True(tc.t, ok, "Job summary not found in latest job")
	return summary
}

// GetJobScheduledCount triggers a sync and returns the scheduled_deletions count
// from the job summary. Unlike GetScheduledCount (which uses /api/media/leaving-soon
// and only shows items within the leaving_soon_days window), this counts ALL items
// whose deletion date has passed — including overdue items.
func (tc *TestClient) GetJobScheduledCount() int {
	tc.t.Helper()
	summary := tc.TriggerSyncAndGetSummary()

	scheduledDeletions, ok := summary["scheduled_deletions"].(float64)
	if !ok {
		return 0
	}
	return int(scheduledDeletions)
}

// GetJobWouldDelete triggers a sync and returns the would_delete list from the
// job summary. Each entry is a map with fields: id, title, year, reason, etc.
// This includes ALL items past their deletion date, not just leaving-soon items.
func (tc *TestClient) GetJobWouldDelete() []map[string]interface{} {
	tc.t.Helper()
	summary := tc.TriggerSyncAndGetSummary()

	raw, ok := summary["would_delete"].([]interface{})
	if !ok {
		return nil
	}

	items := make([]map[string]interface{}, 0, len(raw))
	for _, v := range raw {
		if item, ok := v.(map[string]interface{}); ok {
			items = append(items, item)
		}
	}
	return items
}

// GetMovies returns all movies from the API
func (tc *TestClient) GetMovies() []map[string]interface{} {
	resp, err := tc.Get("/api/media/movies")
	require.NoError(tc.t, err)
	defer resp.Body.Close()

	require.Equal(tc.t, http.StatusOK, resp.StatusCode)

	var result struct {
		Items []map[string]interface{} `json:"items"`
	}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(tc.t, err)

	return result.Items
}

// UpdateRetentionPolicy modifies the retention value in the config file using YAML parsing
func UpdateRetentionPolicy(t *testing.T, configPath, retention string) {
	t.Helper()
	t.Logf("Updating retention policy to: %s", retention)

	// Read config file
	content, err := os.ReadFile(configPath)
	require.NoError(t, err)

	// Parse YAML
	var config map[string]interface{}
	err = yaml.Unmarshal(content, &config)
	require.NoError(t, err, "Failed to parse YAML config")

	// Update movie_retention
	rules, ok := config["rules"].(map[string]interface{})
	require.True(t, ok, "rules section not found in config")

	oldValue := rules["movie_retention"]
	rules["movie_retention"] = retention
	t.Logf("Updated movie_retention from %v to %s", oldValue, retention)

	// Marshal back to YAML
	newContent, err := yaml.Marshal(config)
	require.NoError(t, err, "Failed to marshal YAML config")

	// Write updated config
	err = os.WriteFile(configPath, newContent, 0644)
	require.NoError(t, err)

	t.Logf("Retention policy updated successfully")
}

// UpdateRetentionPolicyBoth updates both movie and TV retention in the config file.
func UpdateRetentionPolicyBoth(t *testing.T, configPath, movieRetention, tvRetention string) {
	t.Helper()
	t.Logf("Updating retention policies to: movie=%s tv=%s", movieRetention, tvRetention)

	content, err := os.ReadFile(configPath)
	require.NoError(t, err)

	var config map[string]interface{}
	err = yaml.Unmarshal(content, &config)
	require.NoError(t, err, "Failed to parse YAML config")

	rules, ok := config["rules"].(map[string]interface{})
	require.True(t, ok, "rules section not found in config")

	oldMovie := rules["movie_retention"]
	oldTV := rules["tv_retention"]
	rules["movie_retention"] = movieRetention
	rules["tv_retention"] = tvRetention
	t.Logf("Updated movie_retention %v -> %s, tv_retention %v -> %s", oldMovie, movieRetention, oldTV, tvRetention)

	newContent, err := yaml.Marshal(config)
	require.NoError(t, err, "Failed to marshal YAML config")

	err = os.WriteFile(configPath, newContent, 0644)
	require.NoError(t, err)

	t.Logf("Retention policies updated successfully")
}

// UpdateDryRun modifies the dry_run value in the config file using YAML parsing
func UpdateDryRun(t *testing.T, configPath string, dryRun bool) {
	t.Helper()
	t.Logf("Updating dry_run to: %v", dryRun)

	// Read config file
	content, err := os.ReadFile(configPath)
	require.NoError(t, err)

	// Parse YAML
	var config map[string]interface{}
	err = yaml.Unmarshal(content, &config)
	require.NoError(t, err, "Failed to parse YAML config")

	// Update dry_run
	app, ok := config["app"].(map[string]interface{})
	require.True(t, ok, "app section not found in config")

	oldValue := app["dry_run"]
	app["dry_run"] = dryRun
	t.Logf("Updated dry_run from %v to %v", oldValue, dryRun)

	// Marshal back to YAML
	newContent, err := yaml.Marshal(config)
	require.NoError(t, err, "Failed to marshal YAML config")

	// Write updated config
	err = os.WriteFile(configPath, newContent, 0644)
	require.NoError(t, err)

	t.Logf("dry_run updated successfully")
}

// UpdateConfigAPIKeys updates Jellyfin and Radarr API keys in the config file
func UpdateConfigAPIKeys(t *testing.T, configPath, jellyfinKey, radarrKey string) {
	UpdateConfigAPIKeysWithExtras(t, configPath, jellyfinKey, radarrKey, "", "", "")
}

// UpdateConfigAPIKeysWithExtras updates all service API keys in the config file.
// Pass an empty string for any optional key (jellyseerrKey, jellystatKey, sonarrKey)
// to leave that integration unchanged.
func UpdateConfigAPIKeysWithExtras(t *testing.T, configPath, jellyfinKey, radarrKey, jellyseerrKey, jellystatKey, sonarrKey string) {
	t.Logf("Updating config API keys...")

	// Read config file
	content, err := os.ReadFile(configPath)
	require.NoError(t, err)

	lines := strings.Split(string(content), "\n")
	inJellyfinSection := false
	inRadarrSection := false
	inJellyseerrSection := false
	inJellystatSection := false
	inSonarrSection := false
	jellyfinUpdated := false
	radarrUpdated := false
	jellyseerrUpdated := false
	jellystatUpdated := false
	sonarrUpdated := false

	// Track which integration section we're in. The section headers are siblings
	// under `integrations:`; ANY other key at that indentation (or shallower, e.g.
	// another integration or a top-level section) ends the current section. Without
	// this, an indented key like `streamystats:` would keep the previous section's
	// tracker active and its keys would be misprocessed as that section's.
	integrationIndent := -1
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		indent := len(line) - len(strings.TrimLeft(line, " "))

		if strings.HasPrefix(trimmed, "jellyfin:") {
			integrationIndent = indent
			inJellyfinSection = true
			inRadarrSection = false
			inJellyseerrSection = false
			inJellystatSection = false
			inSonarrSection = false
			continue
		} else if strings.HasPrefix(trimmed, "radarr:") {
			integrationIndent = indent
			inJellyfinSection = false
			inRadarrSection = true
			inJellyseerrSection = false
			inJellystatSection = false
			inSonarrSection = false
			continue
		} else if strings.HasPrefix(trimmed, "jellyseerr:") {
			integrationIndent = indent
			inJellyfinSection = false
			inRadarrSection = false
			inJellyseerrSection = true
			inJellystatSection = false
			inSonarrSection = false
			continue
		} else if strings.HasPrefix(trimmed, "jellystat:") {
			integrationIndent = indent
			inJellyfinSection = false
			inRadarrSection = false
			inJellyseerrSection = false
			inJellystatSection = true
			inSonarrSection = false
			continue
		} else if strings.HasPrefix(trimmed, "sonarr:") {
			integrationIndent = indent
			inJellyfinSection = false
			inRadarrSection = false
			inJellyseerrSection = false
			inJellystatSection = false
			inSonarrSection = true
			continue
		} else if strings.HasSuffix(trimmed, ":") && integrationIndent >= 0 && indent <= integrationIndent {
			// New key at the integration level (or shallower) — ends the current
			// integration section.
			inJellyfinSection = false
			inRadarrSection = false
			inJellyseerrSection = false
			inJellystatSection = false
			inSonarrSection = false
			continue
		}

		// Update enabled: false to enabled: true for Jellyfin
		if inJellyfinSection && strings.Contains(trimmed, "enabled:") && strings.Contains(trimmed, "false") {
			lines[i] = strings.Replace(line, "false", "true", 1)
			t.Logf("Enabled Jellyfin integration")
			continue
		}

		// Update Jellyfin API key
		if inJellyfinSection && strings.HasPrefix(trimmed, "api_key:") {
			indent := len(line) - len(strings.TrimLeft(line, " "))
			lines[i] = fmt.Sprintf("%s%sapi_key: %s", strings.Repeat(" ", indent), "", jellyfinKey)
			jellyfinUpdated = true
			t.Logf("Updated Jellyfin API key")
			continue
		}

		// Update Radarr API key
		if inRadarrSection && strings.HasPrefix(trimmed, "api_key:") {
			indent := len(line) - len(strings.TrimLeft(line, " "))
			lines[i] = fmt.Sprintf("%s%sapi_key: %s", strings.Repeat(" ", indent), "", radarrKey)
			radarrUpdated = true
			t.Logf("Updated Radarr API key")
			continue
		}

		// Update Jellyseerr API key and URL (only if key provided)
		if inJellyseerrSection && jellyseerrKey != "" {
			if strings.Contains(trimmed, "enabled:") && strings.Contains(trimmed, "false") {
				lines[i] = strings.Replace(line, "false", "true", 1)
				t.Logf("Enabled Jellyseerr integration")
				continue
			}
			if strings.HasPrefix(trimmed, "api_key:") {
				indent := len(line) - len(strings.TrimLeft(line, " "))
				lines[i] = fmt.Sprintf("%s%sapi_key: %s", strings.Repeat(" ", indent), "", jellyseerrKey)
				jellyseerrUpdated = true
				t.Logf("Updated Jellyseerr API key")
				continue
			}
			if strings.HasPrefix(trimmed, "url:") {
				indent := len(line) - len(strings.TrimLeft(line, " "))
				lines[i] = fmt.Sprintf("%s%surl: \"http://jellyseerr:5055\"", strings.Repeat(" ", indent), "")
				t.Logf("Updated Jellyseerr URL")
				continue
			}
		}

		// Update Jellystat API key and URL (only if key provided)
		if inJellystatSection && jellystatKey != "" {
			if strings.Contains(trimmed, "enabled:") && strings.Contains(trimmed, "false") {
				lines[i] = strings.Replace(line, "false", "true", 1)
				t.Logf("Enabled Jellystat integration")
				continue
			}
			if strings.HasPrefix(trimmed, "api_key:") {
				indent := len(line) - len(strings.TrimLeft(line, " "))
				lines[i] = fmt.Sprintf("%s%sapi_key: %s", strings.Repeat(" ", indent), "", jellystatKey)
				jellystatUpdated = true
				t.Logf("Updated Jellystat API key")
				continue
			}
			if strings.HasPrefix(trimmed, "url:") {
				indent := len(line) - len(strings.TrimLeft(line, " "))
				lines[i] = fmt.Sprintf("%s%surl: \"http://jellystat:3000\"", strings.Repeat(" ", indent), "")
				t.Logf("Updated Jellystat URL")
				continue
			}
		}

		// Update Sonarr API key, URL, and enable it (only if key provided)
		if inSonarrSection && sonarrKey != "" {
			if strings.Contains(trimmed, "enabled:") && strings.Contains(trimmed, "false") {
				lines[i] = strings.Replace(line, "false", "true", 1)
				t.Logf("Enabled Sonarr integration")
				continue
			}
			if strings.HasPrefix(trimmed, "api_key:") {
				indent := len(line) - len(strings.TrimLeft(line, " "))
				lines[i] = fmt.Sprintf("%s%sapi_key: %s", strings.Repeat(" ", indent), "", sonarrKey)
				sonarrUpdated = true
				t.Logf("Updated Sonarr API key")
				continue
			}
			if strings.HasPrefix(trimmed, "url:") {
				indent := len(line) - len(strings.TrimLeft(line, " "))
				lines[i] = fmt.Sprintf("%s%surl: \"http://oxicleanarr-test-sonarr:8989\"", strings.Repeat(" ", indent), "")
				t.Logf("Updated Sonarr URL")
				continue
			}
		}
	}

	require.True(t, jellyfinUpdated, "Failed to update Jellyfin API key")
	require.True(t, radarrUpdated, "Failed to update Radarr API key")

	// Only check these if keys were provided
	if jellyseerrKey != "" {
		require.True(t, jellyseerrUpdated, "Failed to update Jellyseerr API key")
	}
	if jellystatKey != "" {
		require.True(t, jellystatUpdated, "Failed to update Jellystat API key")
	}
	if sonarrKey != "" {
		require.True(t, sonarrUpdated, "Failed to update Sonarr API key")
	}

	// Write updated config
	newContent := strings.Join(lines, "\n")
	err = os.WriteFile(configPath, []byte(newContent), 0644)
	require.NoError(t, err)

	t.Logf("API keys updated successfully")
}

// RestartOxiCleanarr restarts the container and waits for health check
func RestartOxiCleanarr(t *testing.T, composeFile string) {
	t.Logf("Restarting OxiCleanarr to reload config...")

	cmd := fmt.Sprintf("docker compose -f %s restart oxicleanarr", composeFile)
	err := execCommand(cmd)
	require.NoError(t, err)

	// Wait for health check
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := &http.Client{Timeout: 2 * time.Second}
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			require.Failf(t, "Restart timed out", "OxiCleanarr failed to restart within 30 seconds")
		case <-ticker.C:
			resp, err := client.Get(OxiCleanarrURL + "/health")
			if err == nil && resp.StatusCode == http.StatusOK {
				resp.Body.Close()
				t.Logf("OxiCleanarr restarted successfully")
				time.Sleep(2 * time.Second) // Extra delay for full initialization
				return
			}
			if resp != nil {
				resp.Body.Close()
			}
		}
	}
}

// WaitForConfigValue waits for a specific config value to be loaded via hot-reload
func WaitForConfigValue(t *testing.T, client *TestClient, fieldPath string, expectedValue string) {
	t.Logf("Waiting for config field '%s' to have value '%s'...", fieldPath, expectedValue)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			require.Failf(t, "Config value timeout", "Config field '%s' did not reach expected value '%s' within 10 seconds", fieldPath, expectedValue)
		case <-ticker.C:
			// Query config endpoint
			resp, err := client.Get("/api/config")
			if err != nil {
				t.Logf("Error fetching config: %v", err)
				continue
			}

			var config map[string]interface{}
			err = json.NewDecoder(resp.Body).Decode(&config)
			resp.Body.Close()
			if err != nil {
				t.Logf("Error decoding config: %v", err)
				continue
			}

			// Parse field path (e.g., "movie_retention" or "app.dry_run")
			value := getNestedValue(config, fieldPath)
			if value == expectedValue {
				t.Logf("Config field '%s' has expected value '%s'", fieldPath, expectedValue)
				return
			}
			t.Logf("Current value: %v (waiting for: %s)", value, expectedValue)
		}
	}
}

// getNestedValue extracts a value from a nested map using dot notation
func getNestedValue(data map[string]interface{}, path string) string {
	parts := strings.Split(path, ".")
	current := data

	for i, part := range parts {
		if i == len(parts)-1 {
			// Last part - extract value
			if val, ok := current[part]; ok {
				return fmt.Sprintf("%v", val)
			}
			return ""
		}

		// Navigate deeper
		if next, ok := current[part].(map[string]interface{}); ok {
			current = next
		} else {
			return ""
		}
	}
	return ""
}

// CheckJellyfinLibrary verifies Jellyfin virtual folder state
func CheckJellyfinLibrary(t *testing.T, apiKey string, libraryName string, expectedExists bool) {
	t.Logf("Checking Jellyfin virtual folders for library: %s", libraryName)

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest(http.MethodGet, JellyfinURL+"/Library/VirtualFolders", nil)
	require.NoError(t, err)

	req.Header.Set("X-MediaBrowser-Token", apiKey)

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode, "Failed to query Jellyfin virtual folders")

	var folders []struct {
		Name   string `json:"Name"`
		ItemId string `json:"ItemId"`
	}
	err = json.NewDecoder(resp.Body).Decode(&folders)
	require.NoError(t, err)

	// Check if library exists
	var libraryExists bool
	var libraryID string
	for _, folder := range folders {
		if folder.Name == libraryName {
			libraryExists = true
			libraryID = folder.ItemId
			break
		}
	}

	if expectedExists {
		require.True(t, libraryExists, "Jellyfin library '%s' does not exist (expected to exist)", libraryName)
		t.Logf("Jellyfin library '%s' exists (expected)", libraryName)

		// Get item count
		if libraryID != "" {
			req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/Items?ParentId=%s&Recursive=true", JellyfinURL, libraryID), nil)
			require.NoError(t, err)
			req.Header.Set("X-MediaBrowser-Token", apiKey)

			resp, err := client.Do(req)
			if err == nil && resp.StatusCode == http.StatusOK {
				var items struct {
					TotalRecordCount int `json:"TotalRecordCount"`
				}
				json.NewDecoder(resp.Body).Decode(&items)
				resp.Body.Close()
				t.Logf("Library contains %d items", items.TotalRecordCount)
			}
			if resp != nil {
				resp.Body.Close()
			}
		}
	} else {
		require.False(t, libraryExists, "Jellyfin library '%s' still exists (expected to be deleted)", libraryName)
		t.Logf("Jellyfin library '%s' does not exist (expected)", libraryName)
	}
}

// GetJellyfinAPIKey extracts the API key from the config file
func GetJellyfinAPIKey(t *testing.T, configPath string) string {
	content, err := os.ReadFile(configPath)
	require.NoError(t, err)

	lines := strings.Split(string(content), "\n")
	inJellyfinSection := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "jellyfin:") {
			inJellyfinSection = true
			continue
		}

		if inJellyfinSection && strings.HasPrefix(trimmed, "api_key:") {
			// Extract value (handles both quoted and unquoted)
			value := strings.TrimPrefix(trimmed, "api_key:")
			value = strings.TrimSpace(value)

			// Remove quotes if present
			if strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") {
				value = strings.Trim(value, "\"")
			}

			if value != "" {
				return value
			}
		}

		// Exit section if we hit another top-level key
		if inJellyfinSection && strings.HasSuffix(trimmed, ":") && !strings.HasPrefix(trimmed, " ") {
			break
		}
	}

	require.Failf(t, "API key extraction failed", "Failed to extract Jellyfin API key from config")
	return ""
}

// GetRadarrAPIKeyFromYAMLConfig extracts the Radarr API key from the config file
func GetRadarrAPIKeyFromYAMLConfig(t *testing.T, configPath string) string {
	content, err := os.ReadFile(configPath)
	require.NoError(t, err)

	lines := strings.Split(string(content), "\n")
	inRadarrSection := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "radarr:") {
			inRadarrSection = true
			continue
		}

		if inRadarrSection && strings.HasPrefix(trimmed, "api_key:") {
			// Extract value (handles both quoted and unquoted)
			value := strings.TrimPrefix(trimmed, "api_key:")
			value = strings.TrimSpace(value)

			// Remove quotes if present
			if strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") {
				value = strings.Trim(value, "\"")
			}

			if value != "" {
				return value
			}
		}

		// Exit section if we hit another top-level key
		if inRadarrSection && strings.HasSuffix(trimmed, ":") && !strings.HasPrefix(trimmed, " ") {
			break
		}
	}

	require.Failf(t, "API key extraction failed", "Failed to extract Radarr API key from config")
	return ""
}

// execCommand executes a shell command
func execCommand(cmd string) error {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return fmt.Errorf("empty command")
	}

	// For docker compose commands, handle specially
	if parts[0] == "docker" && len(parts) > 1 && parts[1] == "compose" {
		// docker compose -f file.yml restart service
		args := parts[1:] // Skip "docker", keep "compose" onwards

		// Use exec package (need to import)
		cmdExec := exec.Command("docker", args...)
		output, err := cmdExec.CombinedOutput()
		if err != nil {
			return fmt.Errorf("command failed: %s, output: %s", err, string(output))
		}
		return nil
	}

	return fmt.Errorf("unsupported command: %s", cmd)
}

// TriggerJellyfinLibraryScan triggers a library scan for a specific library
func TriggerJellyfinLibraryScan(t *testing.T, jellyfinURL, apiKey, libraryID string) error {
	t.Helper()
	t.Logf("Triggering Jellyfin library scan for library ID: %s", libraryID)

	client := &http.Client{Timeout: 10 * time.Second}
	url := fmt.Sprintf("%s/Library/Refresh?api_key=%s", jellyfinURL, apiKey)

	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create scan request: %w", err)
	}

	req.Header.Set("X-MediaBrowser-Token", apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to trigger library scan: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("library scan request failed (HTTP %d): %s", resp.StatusCode, string(body))
	}

	t.Logf("Library scan triggered successfully")
	return nil
}

// GetJellyfinLibraryID gets the library ID for a given library name
func GetJellyfinLibraryID(t *testing.T, jellyfinURL, apiKey, libraryName string) (string, error) {
	t.Helper()

	client := &http.Client{Timeout: 10 * time.Second}

	// Retry up to 10 times with 1 second delay (Jellyfin may take a moment to populate ItemId after library creation)
	maxRetries := 10
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(1 * time.Second)
			t.Logf("Retrying GetJellyfinLibraryID (attempt %d/%d)...", attempt+1, maxRetries)
		}

		req, err := http.NewRequest(http.MethodGet, jellyfinURL+"/Library/VirtualFolders", nil)
		if err != nil {
			return "", err
		}

		req.Header.Set("X-MediaBrowser-Token", apiKey)

		resp, err := client.Do(req)
		if err != nil {
			return "", fmt.Errorf("failed to query virtual folders: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return "", fmt.Errorf("failed to get virtual folders (HTTP %d)", resp.StatusCode)
		}

		var folders []struct {
			Name   string `json:"Name"`
			ItemId string `json:"ItemId"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&folders); err != nil {
			resp.Body.Close()
			return "", fmt.Errorf("failed to decode virtual folders: %w", err)
		}
		resp.Body.Close()

		for _, folder := range folders {
			if folder.Name == libraryName {
				if folder.ItemId != "" {
					return folder.ItemId, nil
				}
				// Library found but ItemId not populated yet, retry
				t.Logf("Library '%s' found but ItemId not populated yet", libraryName)
				break
			}
		}
	}

	return "", fmt.Errorf("library '%s' not found or ItemId not populated after %d attempts", libraryName, maxRetries)
}

// WaitForJellyfinMovies waits until Jellyfin has matched the expected number of movies
func WaitForJellyfinMovies(t *testing.T, jellyfinURL, apiKey string, expectedCount int, libraryID string) error {
	t.Helper()
	t.Logf("Waiting for Jellyfin to match %d movies...", expectedCount)

	client := &http.Client{Timeout: 10 * time.Second}
	maxRetries := 60 // 5 minutes max
	retryDelay := 5 * time.Second

	for i := 0; i < maxRetries; i++ {
		url := fmt.Sprintf("%s/Items?ParentId=%s&Recursive=true&IncludeItemTypes=Movie", jellyfinURL, libraryID)
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			time.Sleep(retryDelay)
			continue
		}

		req.Header.Set("X-MediaBrowser-Token", apiKey)

		resp, err := client.Do(req)
		if err != nil {
			time.Sleep(retryDelay)
			continue
		}

		var result struct {
			TotalRecordCount int `json:"TotalRecordCount"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			resp.Body.Close()
			time.Sleep(retryDelay)
			continue
		}
		resp.Body.Close()

		t.Logf("Jellyfin has %d movies (expecting %d)", result.TotalRecordCount, expectedCount)

		if result.TotalRecordCount >= expectedCount {
			t.Logf("Jellyfin movie matching complete!")
			return nil
		}

		if (i+1)%6 == 0 {
			t.Logf("Still waiting for movie matching... (%d/%d movies, %d/%d attempts)",
				result.TotalRecordCount, expectedCount, i+1, maxRetries)
		}

		time.Sleep(retryDelay)
	}

	return fmt.Errorf("jellyfin failed to match %d movies within timeout", expectedCount)
}

// WaitForJellyfinShows waits until Jellyfin has matched the expected number of TV shows
func WaitForJellyfinShows(t *testing.T, jellyfinURL, apiKey string, expectedCount int, libraryID string) error {
	t.Helper()
	t.Logf("Waiting for Jellyfin to match %d TV shows...", expectedCount)

	client := &http.Client{Timeout: 10 * time.Second}
	maxRetries := 60 // 5 minutes max
	retryDelay := 5 * time.Second

	for i := 0; i < maxRetries; i++ {
		url := fmt.Sprintf("%s/Items?ParentId=%s&Recursive=true&IncludeItemTypes=Series", jellyfinURL, libraryID)
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			time.Sleep(retryDelay)
			continue
		}
		req.Header.Set("X-MediaBrowser-Token", apiKey)

		resp, err := client.Do(req)
		if err != nil {
			time.Sleep(retryDelay)
			continue
		}

		var result struct {
			TotalRecordCount int `json:"TotalRecordCount"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			resp.Body.Close()
			time.Sleep(retryDelay)
			continue
		}
		resp.Body.Close()

		t.Logf("Jellyfin has %d TV shows (expecting %d)", result.TotalRecordCount, expectedCount)
		if result.TotalRecordCount >= expectedCount {
			t.Logf("Jellyfin TV show matching complete!")
			return nil
		}

		time.Sleep(retryDelay)
	}

	return fmt.Errorf("jellyfin failed to match %d TV shows within timeout", expectedCount)
}

// GetRadarrMovieCount queries Radarr's /api/v3/movie endpoint and returns the movie count
func GetRadarrMovieCount(t *testing.T, radarrURL, apiKey string) (int, error) {
	t.Helper()

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest(http.MethodGet, radarrURL+"/api/v3/movie", nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-Api-Key", apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("failed to query Radarr movies: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("unexpected status code %d from Radarr", resp.StatusCode)
	}

	var movies []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&movies); err != nil {
		return 0, fmt.Errorf("failed to decode Radarr response: %w", err)
	}

	return len(movies), nil
}

// GetJellyfinMovieCount queries Jellyfin for the movie count in a specific library
func GetJellyfinMovieCount(t *testing.T, jellyfinURL, apiKey, libraryID string) (int, error) {
	t.Helper()

	client := &http.Client{Timeout: 10 * time.Second}
	url := fmt.Sprintf("%s/Items?ParentId=%s&Recursive=true&IncludeItemTypes=Movie", jellyfinURL, libraryID)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-MediaBrowser-Token", apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("failed to query Jellyfin movies: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("unexpected status code %d from Jellyfin", resp.StatusCode)
	}

	var result struct {
		TotalRecordCount int `json:"TotalRecordCount"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("failed to decode Jellyfin response: %w", err)
	}

	return result.TotalRecordCount, nil
}

// GetJellyfinUserID gets the first user ID from Jellyfin (used for user views queries)
func GetJellyfinUserID(t *testing.T, jellyfinURL, apiKey string) (string, error) {
	t.Helper()

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest(http.MethodGet, jellyfinURL+"/Users", nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-MediaBrowser-Token", apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to query Jellyfin users: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code %d from Jellyfin", resp.StatusCode)
	}

	var users []struct {
		Name string `json:"Name"`
		ID   string `json:"Id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&users); err != nil {
		return "", fmt.Errorf("failed to decode Jellyfin users: %w", err)
	}

	if len(users) == 0 {
		return "", fmt.Errorf("no users found in Jellyfin")
	}

	return users[0].ID, nil
}

// CheckJellyfinUserViews verifies that a library appears (or doesn't appear) in user views (dashboard)
// This is the critical test for the double-refresh fix - user views must update after library deletion
// Uses retry logic with timeout since Jellyfin may need time to update its user view cache
func CheckJellyfinUserViews(t *testing.T, jellyfinURL, apiKey, libraryName string, expectedExists bool) {
	t.Helper()
	t.Logf("Checking Jellyfin user views for library: %s (expected exists: %v)", libraryName, expectedExists)

	// Get user ID first
	userID, err := GetJellyfinUserID(t, jellyfinURL, apiKey)
	require.NoError(t, err, "Failed to get Jellyfin user ID")
	t.Logf("Using user ID: %s", userID)

	// Retry with timeout - Jellyfin needs time to update user view cache after library changes
	maxRetries := 10
	retryDelay := 1 * time.Second

	var libraryExists bool
	var lastItems []string

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			t.Logf("Retry %d/%d: Waiting %v for user view cache to update...", attempt, maxRetries-1, retryDelay)
			time.Sleep(retryDelay)
		}

		client := &http.Client{Timeout: 10 * time.Second}
		url := fmt.Sprintf("%s/Users/%s/Views", jellyfinURL, userID)
		req, err := http.NewRequest(http.MethodGet, url, nil)
		require.NoError(t, err)

		req.Header.Set("X-MediaBrowser-Token", apiKey)

		resp, err := client.Do(req)
		require.NoError(t, err)

		require.Equal(t, http.StatusOK, resp.StatusCode, "Failed to query Jellyfin user views")

		var result struct {
			Items []struct {
				Name           string `json:"Name"`
				CollectionType string `json:"CollectionType"`
				ID             string `json:"Id"`
			} `json:"Items"`
		}
		err = json.NewDecoder(resp.Body).Decode(&result)
		resp.Body.Close()
		require.NoError(t, err)

		// Check if library exists in user views
		libraryExists = false
		lastItems = make([]string, 0, len(result.Items))
		for _, item := range result.Items {
			lastItems = append(lastItems, item.Name)
			if item.Name == libraryName {
				libraryExists = true
			}
		}

		// If we got the expected result, success!
		if libraryExists == expectedExists {
			t.Logf("✅ User view cache state matches expectation after %d attempts", attempt+1)
			break
		}

		// Log current state for debugging
		if attempt == 0 {
			t.Logf("Initial user views: %v", lastItems)
		}
	}

	// Log final state
	t.Logf("Final user views: %v", lastItems)

	if expectedExists {
		require.True(t, libraryExists, "Library '%s' does not appear in user views (dashboard) after %d retries - expected to exist", libraryName, maxRetries)
		t.Logf("✅ Library '%s' appears in user views (expected)", libraryName)
	} else {
		require.False(t, libraryExists, "Library '%s' still appears in user views (dashboard) after %d retries - expected to be deleted", libraryName, maxRetries)
		t.Logf("✅ Library '%s' does not appear in user views (expected - deletion successful)", libraryName)
	}
}

// GetMediaByTitle searches for a media item by title and returns its ID
func (tc *TestClient) GetMediaByTitle(title string) (string, error) {
	tc.t.Helper()
	tc.t.Logf("Searching for media with title: %s", title)

	resp, err := tc.Get("/api/media/movies")
	if err != nil {
		return "", fmt.Errorf("failed to get movies: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code %d", resp.StatusCode)
	}

	var result struct {
		Items []struct {
			ID    string `json:"id"`
			Title string `json:"title"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	for _, item := range result.Items {
		if item.Title == title {
			tc.t.Logf("Found media ID: %s for title: %s", item.ID, title)
			return item.ID, nil
		}
	}

	return "", fmt.Errorf("media not found with title: %s", title)
}

// ExcludeMedia adds an exclusion for a media item
func (tc *TestClient) ExcludeMedia(mediaID, reason string) error {
	tc.t.Helper()
	tc.t.Logf("Excluding media ID: %s with reason: %s", mediaID, reason)

	body := map[string]string{
		"reason": reason,
	}

	resp, err := tc.Post(fmt.Sprintf("/api/media/%s/exclude", mediaID), body)
	if err != nil {
		return fmt.Errorf("failed to exclude media: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(bodyBytes))
	}

	tc.t.Logf("Successfully excluded media ID: %s", mediaID)
	return nil
}

// RemoveExclusion removes an exclusion for a media item
func (tc *TestClient) RemoveExclusion(mediaID string) error {
	tc.t.Helper()
	tc.t.Logf("Removing exclusion for media ID: %s", mediaID)

	req, err := http.NewRequest(http.MethodDelete, tc.baseURL+fmt.Sprintf("/api/media/%s/exclude", mediaID), nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	if tc.token != "" {
		req.Header.Set("Authorization", "Bearer "+tc.token)
	}

	resp, err := tc.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to remove exclusion: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(bodyBytes))
	}

	tc.t.Logf("Successfully removed exclusion for media ID: %s", mediaID)
	return nil
}

// GetMediaDetails retrieves details for a specific media item
func (tc *TestClient) GetMediaDetails(mediaID string) (map[string]interface{}, error) {
	tc.t.Helper()
	tc.t.Logf("Getting media details for ID: %s", mediaID)

	resp, err := tc.Get(fmt.Sprintf("/api/media/%s", mediaID))
	if err != nil {
		return nil, fmt.Errorf("failed to get media details: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var details map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&details); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	tc.t.Logf("Retrieved media details for ID: %s", mediaID)
	return details, nil
}

// GetLatestJob retrieves the latest job from the API
func (tc *TestClient) GetLatestJob() (map[string]interface{}, error) {
	tc.t.Helper()
	tc.t.Logf("Getting latest job from API...")

	resp, err := tc.Get("/api/jobs/latest")
	if err != nil {
		return nil, fmt.Errorf("failed to get latest job: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var job map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&job); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	tc.t.Logf("Retrieved latest job ID: %v", job["id"])
	return job, nil
}

// WaitForJobCompletion polls the latest job until it's no longer in "running" status
func (tc *TestClient) WaitForJobCompletion(maxWait time.Duration) (map[string]interface{}, error) {
	tc.t.Helper()
	tc.t.Logf("Waiting for job to complete (max wait: %v)...", maxWait)

	timeout := time.After(maxWait)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	var lastJob map[string]interface{}
	var lastJobID string

	for {
		select {
		case <-timeout:
			return nil, fmt.Errorf("timed out waiting for job to complete after %v (last status: %v)", maxWait, lastJob["status"])
		case <-ticker.C:
			job, err := tc.GetLatestJob()
			if err != nil {
				return nil, fmt.Errorf("failed to get latest job: %w", err)
			}

			lastJob = job
			jobID, _ := job["id"].(string)
			status, _ := job["status"].(string)

			// Track if we're looking at a new job
			if lastJobID == "" {
				lastJobID = jobID
				tc.t.Logf("Watching job %s (status: %s)", jobID, status)
			} else if jobID != lastJobID {
				tc.t.Logf("New job detected: %s (status: %s)", jobID, status)
				lastJobID = jobID
			}

			// Job is complete if it's not in "running" or "pending" status
			if status != "running" && status != "pending" {
				tc.t.Logf("Job %s completed with status: %s", jobID, status)
				return job, nil
			}
		}
	}
}

// RadarrMovieResponse represents a movie from Radarr API
type RadarrMovieResponse struct {
	ID         int    `json:"id"`
	Title      string `json:"title"`
	Year       int    `json:"year"`
	Tags       []int  `json:"tags"`
	HasFile    bool   `json:"hasFile"`
	Path       string `json:"path"`
	SizeOnDisk int64  `json:"sizeOnDisk"`
}

// RadarrTagResponse represents a tag from Radarr API
type RadarrTagResponse struct {
	ID    int    `json:"id"`
	Label string `json:"label"`
}

// CreateRadarrTag creates a tag in Radarr and returns the tag ID
func CreateRadarrTag(t *testing.T, radarrURL, apiKey, tagLabel string) int {
	t.Helper()
	t.Logf("Creating Radarr tag: %s", tagLabel)

	tagData := map[string]string{"label": tagLabel}
	jsonData, err := json.Marshal(tagData)
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, radarrURL+"/api/v3/tag", bytes.NewBuffer(jsonData))
	require.NoError(t, err)
	req.Header.Set("X-Api-Key", apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusCreated, resp.StatusCode, "Failed to create Radarr tag")

	var tag RadarrTagResponse
	err = json.NewDecoder(resp.Body).Decode(&tag)
	require.NoError(t, err)

	t.Logf("Created Radarr tag ID %d: %s", tag.ID, tag.Label)
	return tag.ID
}

// DeleteRadarrTag deletes a tag from Radarr by ID
func DeleteRadarrTag(t *testing.T, radarrURL, apiKey string, tagID int) {
	t.Helper()
	t.Logf("Deleting Radarr tag ID: %d", tagID)

	req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/api/v3/tag/%d", radarrURL, tagID), nil)
	require.NoError(t, err)
	req.Header.Set("X-Api-Key", apiKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.True(t, resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent, "Failed to delete Radarr tag")
	t.Logf("Deleted Radarr tag ID: %d", tagID)
}

// GetRadarrMovieByTitle finds a movie in Radarr by title
func GetRadarrMovieByTitle(t *testing.T, radarrURL, apiKey, title string) *RadarrMovieResponse {
	t.Helper()
	t.Logf("Searching for Radarr movie: %s", title)

	req, err := http.NewRequest(http.MethodGet, radarrURL+"/api/v3/movie", nil)
	require.NoError(t, err)
	req.Header.Set("X-Api-Key", apiKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var movies []RadarrMovieResponse
	err = json.NewDecoder(resp.Body).Decode(&movies)
	require.NoError(t, err)

	for _, movie := range movies {
		if movie.Title == title {
			t.Logf("Found Radarr movie ID %d: %s", movie.ID, movie.Title)
			return &movie
		}
	}

	require.Fail(t, "Movie not found in Radarr", "Title: %s", title)
	return nil
}

// TagRadarrMovie adds a tag to a movie in Radarr
func TagRadarrMovie(t *testing.T, radarrURL, apiKey string, movieID, tagID int) {
	t.Helper()
	t.Logf("Adding tag %d to Radarr movie ID %d", tagID, movieID)

	// Get current movie data
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/api/v3/movie/%d", radarrURL, movieID), nil)
	require.NoError(t, err)
	req.Header.Set("X-Api-Key", apiKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var movie map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&movie)
	require.NoError(t, err)

	// Add tag to tags array
	tags, ok := movie["tags"].([]interface{})
	if !ok {
		tags = []interface{}{}
	}
	tags = append(tags, float64(tagID))
	movie["tags"] = tags

	// Update movie
	jsonData, err := json.Marshal(movie)
	require.NoError(t, err)

	req, err = http.NewRequest(http.MethodPut, fmt.Sprintf("%s/api/v3/movie/%d", radarrURL, movieID), bytes.NewBuffer(jsonData))
	require.NoError(t, err)
	req.Header.Set("X-Api-Key", apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err = client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.True(t, resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusAccepted, "Failed to tag Radarr movie")
	t.Logf("Tagged Radarr movie ID %d with tag %d", movieID, tagID)
}

// VerifyMovieExistsInRadarr checks if movie exists in Radarr
func VerifyMovieExistsInRadarr(t *testing.T, radarrURL, apiKey, title string) bool {
	t.Helper()
	t.Logf("Verifying movie exists in Radarr: %s", title)

	req, err := http.NewRequest(http.MethodGet, radarrURL+"/api/v3/movie", nil)
	require.NoError(t, err)
	req.Header.Set("X-Api-Key", apiKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var movies []RadarrMovieResponse
	err = json.NewDecoder(resp.Body).Decode(&movies)
	require.NoError(t, err)

	for _, movie := range movies {
		if movie.Title == title {
			t.Logf("✅ Movie exists in Radarr: %s", title)
			return true
		}
	}

	t.Logf("❌ Movie NOT found in Radarr: %s", title)
	return false
}

// UpdateEnableDeletion updates the enable_deletion setting in config using YAML parsing
func UpdateEnableDeletion(t *testing.T, configPath string, enabled bool) {
	t.Helper()
	t.Logf("Updating enable_deletion to: %v", enabled)

	// Read config file
	content, err := os.ReadFile(configPath)
	require.NoError(t, err)

	// Parse YAML
	var config map[string]interface{}
	err = yaml.Unmarshal(content, &config)
	require.NoError(t, err, "Failed to parse YAML config")

	// Update enable_deletion
	app, ok := config["app"].(map[string]interface{})
	require.True(t, ok, "app section not found in config")

	oldValue := app["enable_deletion"]
	app["enable_deletion"] = enabled
	t.Logf("Updated enable_deletion from %v to %v", oldValue, enabled)

	// Marshal back to YAML
	newContent, err := yaml.Marshal(config)
	require.NoError(t, err, "Failed to marshal YAML config")

	// Write updated config
	err = os.WriteFile(configPath, newContent, 0644)
	require.NoError(t, err)

	t.Logf("Config updated: enable_deletion=%v", enabled)
}

// AdvancedRuleConfig represents a rule for config updates
type AdvancedRuleConfig struct {
	Name           string
	Type           string
	Enabled        bool
	Tag            string
	Retention      string
	RequireWatched bool
}

// AddAdvancedRule adds an advanced rule to the config file using YAML round-trip
// to ensure all field values are correctly typed (no raw string appending).
func AddAdvancedRule(t *testing.T, configPath string, rule AdvancedRuleConfig) {
	t.Helper()
	t.Logf("Adding advanced rule: %s (type: %s, tag: %s)", rule.Name, rule.Type, rule.Tag)

	content, err := os.ReadFile(configPath)
	require.NoError(t, err)

	var cfg map[string]interface{}
	err = yaml.Unmarshal(content, &cfg)
	require.NoError(t, err, "Failed to parse YAML config")

	ruleMap := map[string]interface{}{
		"name":            rule.Name,
		"type":            rule.Type,
		"enabled":         rule.Enabled,
		"require_watched": rule.RequireWatched,
	}
	if rule.Tag != "" {
		ruleMap["tag"] = rule.Tag
	}
	if rule.Retention != "" {
		ruleMap["retention"] = rule.Retention
	}

	cfg["advanced_rules"] = []interface{}{ruleMap}

	newContent, err := yaml.Marshal(cfg)
	require.NoError(t, err, "Failed to marshal YAML config")

	err = os.WriteFile(configPath, newContent, 0644)
	require.NoError(t, err)

	t.Logf("Advanced rule added to config")
}

// RemoveAdvancedRules removes all advanced rules from the config using YAML round-trip.
func RemoveAdvancedRules(t *testing.T, configPath string) {
	t.Helper()
	t.Logf("Removing advanced rules from config")

	content, err := os.ReadFile(configPath)
	require.NoError(t, err)

	var cfg map[string]interface{}
	err = yaml.Unmarshal(content, &cfg)
	require.NoError(t, err, "Failed to parse YAML config")

	delete(cfg, "advanced_rules")

	newContent, err := yaml.Marshal(cfg)
	require.NoError(t, err, "Failed to marshal YAML config")

	err = os.WriteFile(configPath, newContent, 0644)
	require.NoError(t, err)

	t.Logf("Advanced rules removed from config")
}

// UpdateDiskThreshold enables or disables the disk_threshold section in the config file.
// When enabled=true, it sets free_space_gb and check_source.
// When enabled=false, it removes the disk_threshold section entirely (or sets enabled: false).
func UpdateDiskThreshold(t *testing.T, configPath string, enabled bool, freeSpaceGB int, checkSource string) {
	t.Helper()
	t.Logf("Updating disk_threshold: enabled=%v, free_space_gb=%d, check_source=%s", enabled, freeSpaceGB, checkSource)

	content, err := os.ReadFile(configPath)
	require.NoError(t, err)

	var cfg map[string]interface{}
	err = yaml.Unmarshal(content, &cfg)
	require.NoError(t, err, "Failed to parse YAML config")

	app, ok := cfg["app"].(map[string]interface{})
	require.True(t, ok, "app section not found in config")

	if enabled {
		app["disk_threshold"] = map[string]interface{}{
			"enabled":       true,
			"free_space_gb": freeSpaceGB,
			"check_source":  checkSource,
		}
	} else {
		delete(app, "disk_threshold")
	}

	newContent, err := yaml.Marshal(cfg)
	require.NoError(t, err, "Failed to marshal YAML config")

	err = os.WriteFile(configPath, newContent, 0644)
	require.NoError(t, err)

	t.Logf("disk_threshold config updated")
}

// resetConfigAPIKeysToPlaceholders resets all service API keys in the config file back to
// safe placeholder values. This is called from TestMain (which has no *testing.T) so it
// uses log.Fatal on error rather than require/t.Fatal.
//
// It uses the same line-based parser as UpdateConfigAPIKeysWithExtras so that the file
// format is preserved exactly (no YAML round-trip reformatting).
func resetConfigAPIKeysToPlaceholders(configPath string) {
	content, err := os.ReadFile(configPath)
	if err != nil {
		fmt.Printf("WARN: resetConfigAPIKeysToPlaceholders: failed to read config: %v\n", err)
		return
	}

	lines := strings.Split(string(content), "\n")
	inJellyfinSection := false
	inRadarrSection := false
	inSonarrSection := false

	integrationIndent := -1
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		indentLevel := len(line) - len(strings.TrimLeft(line, " "))

		// Track which integration section we're in (mirrors UpdateConfigAPIKeysWithExtras logic).
		switch {
		case strings.HasPrefix(trimmed, "jellyfin:"):
			integrationIndent = indentLevel
			inJellyfinSection, inRadarrSection, inSonarrSection = true, false, false
			continue
		case strings.HasPrefix(trimmed, "radarr:"):
			integrationIndent = indentLevel
			inJellyfinSection, inRadarrSection, inSonarrSection = false, true, false
			continue
		case strings.HasPrefix(trimmed, "sonarr:"):
			integrationIndent = indentLevel
			inJellyfinSection, inRadarrSection, inSonarrSection = false, false, true
			continue
		case strings.HasSuffix(trimmed, ":") && integrationIndent >= 0 && indentLevel <= integrationIndent:
			// New key at the integration level (or shallower) — exit all integration sections.
			inJellyfinSection, inRadarrSection, inSonarrSection = false, false, false
		}

		indent := strings.Repeat(" ", len(line)-len(strings.TrimLeft(line, " ")))

		if inJellyfinSection && strings.HasPrefix(trimmed, "api_key:") {
			lines[i] = indent + "api_key: your-jellyfin-api-key-here"
		}
		if inRadarrSection && strings.HasPrefix(trimmed, "api_key:") {
			lines[i] = indent + "api_key: your-radarr-api-key-here"
		}
		if inSonarrSection {
			if strings.HasPrefix(trimmed, "api_key:") {
				lines[i] = indent + "api_key: your-sonarr-api-key-here"
			}
			// Sonarr is disabled in the template; restore that state.
			if strings.HasPrefix(trimmed, "enabled:") && strings.Contains(trimmed, "true") {
				lines[i] = strings.Replace(line, "true", "false", 1)
			}
		}
	}

	if err := os.WriteFile(configPath, []byte(strings.Join(lines, "\n")), 0644); err != nil {
		fmt.Printf("WARN: resetConfigAPIKeysToPlaceholders: failed to write config: %v\n", err)
		return
	}

	fmt.Println("✅ Config API keys reset to placeholder values")
}

// AddManualLeavingSoon flags a media item as manually leaving soon
func (tc *TestClient) AddManualLeavingSoon(mediaID string) error {
	tc.t.Helper()
	tc.t.Logf("Adding manual leaving soon flag for media ID: %s", mediaID)

	resp, err := tc.Post(fmt.Sprintf("/api/media/%s/manual-leaving-soon", mediaID), nil)
	if err != nil {
		return fmt.Errorf("failed to add manual leaving soon flag: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(bodyBytes))
	}

	tc.t.Logf("Successfully added manual leaving soon flag for media ID: %s", mediaID)
	return nil
}

// AddManualLeavingSoonRaw flags a media item as manually leaving soon and returns the raw response
// (used when the caller needs to inspect non-200 status codes such as 409 Conflict)
func (tc *TestClient) AddManualLeavingSoonRaw(mediaID string) (*http.Response, error) {
	tc.t.Helper()
	tc.t.Logf("Adding manual leaving soon flag (raw) for media ID: %s", mediaID)
	return tc.Post(fmt.Sprintf("/api/media/%s/manual-leaving-soon", mediaID), nil)
}

// RemoveManualLeavingSoon removes the manual leaving soon flag from a media item
func (tc *TestClient) RemoveManualLeavingSoon(mediaID string) error {
	tc.t.Helper()
	tc.t.Logf("Removing manual leaving soon flag for media ID: %s", mediaID)

	req, err := http.NewRequest(http.MethodDelete, tc.baseURL+fmt.Sprintf("/api/media/%s/manual-leaving-soon", mediaID), nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	if tc.token != "" {
		req.Header.Set("Authorization", "Bearer "+tc.token)
	}

	resp, err := tc.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to remove manual leaving soon flag: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(bodyBytes))
	}

	tc.t.Logf("Successfully removed manual leaving soon flag for media ID: %s", mediaID)
	return nil
}

// GetDiskStatus queries the OxiCleanarr /api/system/disk endpoint and returns the response body.
func (tc *TestClient) GetDiskStatus() (map[string]interface{}, error) {
	tc.t.Helper()

	resp, err := tc.Get("/api/system/disk")
	if err != nil {
		return nil, fmt.Errorf("failed to query disk status: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode disk status: %w", err)
	}

	return result, nil
}

// clientToken returns the JWT currently held by the test client.
func clientToken(client *TestClient) string {
	return client.token
}

// syncRunning reports whether a sync or deletion pass is currently executing.
func syncRunning(t *testing.T, client *TestClient) bool {
	resp, err := client.Get("/api/sync/status")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var status struct {
		InProgress bool `json:"in_progress"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&status))
	return status.InProgress
}

// loginToken authenticates via the API and returns the JWT token.
func loginToken(t *testing.T) string {
	t.Helper()
	resp, err := http.Post(OxiCleanarrURL+"/api/auth/login", "application/json",
		bytes.NewReader([]byte(`{"username":"admin","password":"adminpassword"}`)))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result struct {
		Token string `json:"token"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	require.NotEmpty(t, result.Token)
	return result.Token
}

// rawRequest performs an arbitrary HTTP request with optional token,
// content-type, and body, returning the response and its body.
func rawRequest(t *testing.T, method, url, token, contentType string, body []byte) (*http.Response, []byte) {
	t.Helper()
	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, url, bodyReader)
	require.NoError(t, err)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp, data
}
