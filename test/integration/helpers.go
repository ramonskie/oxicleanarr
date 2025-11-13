package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const (
	OxiCleanarrURL = "http://localhost:8080"
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
				Running        bool `json:"running"`
				SyncInProgress bool `json:"sync_in_progress"`
			}
			err = json.NewDecoder(resp.Body).Decode(&status)
			resp.Body.Close()
			require.NoError(tc.t, err)

			// Wait for sync operation to complete (not just scheduler state)
			if !status.SyncInProgress {
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

// UpdateRetentionPolicy modifies the retention value in the config file
func UpdateRetentionPolicy(t *testing.T, configPath, retention string) {
	t.Logf("Updating retention policy to: %s", retention)
	t.Logf("Config file path: %s", configPath)

	// Read config file
	content, err := os.ReadFile(configPath)
	require.NoError(t, err)
	t.Logf("Read config file successfully (%d bytes)", len(content))

	// Replace movie_retention value
	oldLine := ""
	newLine := fmt.Sprintf("  movie_retention: %s", retention)

	lines := strings.Split(string(content), "\n")
	for i, line := range lines {
		if strings.Contains(line, "movie_retention:") {
			oldLine = line
			lines[i] = newLine
			t.Logf("Found movie_retention at line %d: %q", i+1, oldLine)
			t.Logf("Replacing with: %q", newLine)
			break
		}
	}

	require.NotEmpty(t, oldLine, "movie_retention not found in config")

	// Write updated config
	newContent := strings.Join(lines, "\n")
	t.Logf("Writing updated config (%d bytes)", len(newContent))
	err = os.WriteFile(configPath, []byte(newContent), 0644)
	require.NoError(t, err)

	// Verify the write by reading back
	verifyContent, err := os.ReadFile(configPath)
	require.NoError(t, err)
	verifyLines := strings.Split(string(verifyContent), "\n")

	found := false
	for _, line := range verifyLines {
		if strings.Contains(line, "movie_retention:") {
			t.Logf("Verified file now contains: %q", line)
			found = true
			break
		}
	}
	require.True(t, found, "movie_retention not found after write")

	t.Logf("Retention policy updated and verified successfully")
}

// UpdateDryRun modifies the dry_run value in the config file
func UpdateDryRun(t *testing.T, configPath string, dryRun bool) {
	t.Logf("Updating dry_run to: %v", dryRun)
	t.Logf("Config file path: %s", configPath)

	// Read config file
	content, err := os.ReadFile(configPath)
	require.NoError(t, err)
	t.Logf("Read config file successfully (%d bytes)", len(content))

	// Replace dry_run value
	oldLine := ""
	newLine := fmt.Sprintf("  dry_run: %v", dryRun)

	lines := strings.Split(string(content), "\n")
	for i, line := range lines {
		if strings.Contains(line, "dry_run:") {
			oldLine = line
			lines[i] = newLine
			t.Logf("Found dry_run at line %d: %q", i+1, oldLine)
			t.Logf("Replacing with: %q", newLine)
			break
		}
	}

	require.NotEmpty(t, oldLine, "dry_run not found in config")

	// Write updated config
	newContent := strings.Join(lines, "\n")
	t.Logf("Writing updated config (%d bytes)", len(newContent))
	err = os.WriteFile(configPath, []byte(newContent), 0644)
	require.NoError(t, err)

	// Verify the write by reading back
	verifyContent, err := os.ReadFile(configPath)
	require.NoError(t, err)
	verifyLines := strings.Split(string(verifyContent), "\n")

	found := false
	for _, line := range verifyLines {
		if strings.Contains(line, "dry_run:") {
			t.Logf("Verified file now contains: %q", line)
			found = true
			break
		}
	}
	require.True(t, found, "dry_run not found after write")

	t.Logf("dry_run updated and verified successfully")
}

// UpdateConfigAPIKeys updates Jellyfin and Radarr API keys in the config file
func UpdateConfigAPIKeys(t *testing.T, configPath, jellyfinKey, radarrKey string) {
	t.Logf("Updating config API keys...")

	// Read config file
	content, err := os.ReadFile(configPath)
	require.NoError(t, err)

	lines := strings.Split(string(content), "\n")
	inJellyfinSection := false
	inRadarrSection := false
	jellyfinUpdated := false
	radarrUpdated := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Track which integration section we're in
		if strings.HasPrefix(trimmed, "jellyfin:") {
			inJellyfinSection = true
			inRadarrSection = false
			// Also enable Jellyfin integration
			continue
		} else if strings.HasPrefix(trimmed, "radarr:") {
			inRadarrSection = true
			inJellyfinSection = false
			continue
		} else if strings.HasPrefix(trimmed, "sonarr:") ||
			strings.HasPrefix(trimmed, "jellyseerr:") ||
			strings.HasPrefix(trimmed, "jellystat:") {
			// Exit both sections when hitting other integrations (siblings)
			inJellyfinSection = false
			inRadarrSection = false
			continue
		} else if strings.HasSuffix(trimmed, ":") && !strings.HasPrefix(line, " ") {
			// New top-level section
			inJellyfinSection = false
			inRadarrSection = false
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
			lines[i] = fmt.Sprintf("%s%sapi_key: \"%s\"", strings.Repeat(" ", indent), "", jellyfinKey)
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
	}

	require.True(t, jellyfinUpdated, "Failed to update Jellyfin API key")
	require.True(t, radarrUpdated, "Failed to update Radarr API key")

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

// CheckSymlinks verifies the symlink directory state via Jellyfin plugin API
func CheckSymlinks(t *testing.T, jellyfinAPIKey string, symlinkDir string, expectedCount int) {
	movieDir := filepath.Join(symlinkDir, "movies")

	// Query Jellyfin plugin API to list symlinks
	reqURL := fmt.Sprintf("%s/api/oxicleanarr/symlinks/list?directory=%s&api_key=%s",
		JellyfinURL, url.QueryEscape(movieDir), jellyfinAPIKey)

	resp, err := http.Get(reqURL)
	require.NoError(t, err, "Failed to query Jellyfin plugin API")
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "Failed to read plugin response")

	// Response structure based on plugin API spec
	type SymlinkInfo struct {
		Path   string `json:"Path"`
		Target string `json:"Target"`
		Name   string `json:"Name"`
	}

	var listResp struct {
		Success      bool          `json:"Success"`
		Symlinks     []SymlinkInfo `json:"Symlinks"`
		Count        int           `json:"Count"`
		SymlinkNames []string      `json:"SymlinkNames"`
		Message      string        `json:"Message"`
		ErrorMessage string        `json:"ErrorMessage"`
	}

	err = json.Unmarshal(body, &listResp)
	require.NoError(t, err, "Failed to parse plugin response")

	actualCount := listResp.Count

	if actualCount == expectedCount {
		t.Logf("Symlink count correct: %d (expected: %d)", actualCount, expectedCount)
		if actualCount > 0 {
			t.Logf("Symlinks found:")
			for _, link := range listResp.SymlinkNames {
				t.Logf("  %s", link)
			}
		}
		return
	}

	if actualCount > 0 {
		t.Logf("Symlinks found:")
		for _, link := range listResp.SymlinkNames {
			t.Logf("  %s", link)
		}
	}

	require.Failf(t, "Symlink count mismatch", "Symlink count mismatch: %d (expected: %d)", actualCount, expectedCount)
}

// CheckJellyfinLibrary verifies Jellyfin virtual folder state
func CheckJellyfinLibrary(t *testing.T, apiKey string, expectedExists bool) {
	libraryName := "Leaving Soon - Movies"
	t.Logf("Checking Jellyfin virtual folders...")

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
			// Extract value between quotes
			parts := strings.Split(trimmed, "\"")
			if len(parts) >= 2 {
				return parts[1]
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

// GetHideWhenEmpty reads the hide_when_empty config value
func GetHideWhenEmpty(t *testing.T, configPath string) bool {
	content, err := os.ReadFile(configPath)
	require.NoError(t, err)

	lines := strings.Split(string(content), "\n")
	inSymlinkSection := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "symlink_library:") {
			inSymlinkSection = true
			continue
		}

		if inSymlinkSection && strings.HasPrefix(trimmed, "hide_when_empty:") {
			parts := strings.Fields(trimmed)
			if len(parts) >= 2 {
				return parts[1] == "true"
			}
		}
	}

	// Default to true if not specified
	return true
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

// CleanupTestEnvironment removes all test data to ensure fresh start
func CleanupTestEnvironment(t *testing.T, composeFile string) error {
	t.Helper()
	t.Logf("=== Cleaning up test environment ===")

	// Step 1: Stop all containers
	t.Logf("Stopping all containers...")
	cmd := exec.Command("docker", "compose", "-f", composeFile, "down")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("Warning: Failed to stop containers: %v\nOutput: %s", err, string(output))
	}
	time.Sleep(2 * time.Second)

	// Step 2: Clean Jellyfin directories completely (config, cache)
	// These directories are created by Docker containers as root, so we need sudo
	t.Logf("Cleaning Jellyfin directories...")
	jellyfinConfigDir := "../../integration-test-OLD/jellyfin-config"
	jellyfinCacheDir := "../../integration-test-OLD/jellyfin-cache"

	// Try normal removal first, fall back to sudo if permission denied
	if err := os.RemoveAll(jellyfinConfigDir); err != nil && !os.IsNotExist(err) {
		t.Logf("Normal removal failed, trying with sudo: %v", err)
		cmd := exec.Command("sudo", "rm", "-rf", jellyfinConfigDir)
		if output, sudoErr := cmd.CombinedOutput(); sudoErr != nil {
			t.Logf("Warning: Failed to remove Jellyfin config directory: %v\nOutput: %s", sudoErr, string(output))
		}
	}
	if err := os.RemoveAll(jellyfinCacheDir); err != nil && !os.IsNotExist(err) {
		t.Logf("Normal removal failed, trying with sudo: %v", err)
		cmd := exec.Command("sudo", "rm", "-rf", jellyfinCacheDir)
		if output, sudoErr := cmd.CombinedOutput(); sudoErr != nil {
			t.Logf("Warning: Failed to remove Jellyfin cache directory: %v\nOutput: %s", sudoErr, string(output))
		}
	}

	// Step 3: Clean OxiCleanarr data files
	t.Logf("Cleaning OxiCleanarr data files...")
	oxiDataDir := "../../integration-test-OLD/oxicleanarr-data"
	oxiFiles, _ := filepath.Glob(filepath.Join(oxiDataDir, "*.json"))
	for _, file := range oxiFiles {
		if err := os.Remove(file); err != nil {
			t.Logf("Warning: Failed to remove %s: %v", file, err)
		}
	}

	// Step 4: Clean Radarr config directory completely
	t.Logf("Cleaning Radarr config directory...")
	radarrConfigDir := "../../integration-test-OLD/radarr-config"

	if err := os.RemoveAll(radarrConfigDir); err != nil && !os.IsNotExist(err) {
		t.Logf("Warning: Failed to remove Radarr config directory: %v", err)
	}

	// Step 5: Clean symlink directory (may be owned by root)
	t.Logf("Cleaning symlink directory...")
	symlinkDir := "../../integration-test-OLD/leaving-soon"
	symlinkMoviesDir := filepath.Join(symlinkDir, "movies")
	if err := os.RemoveAll(symlinkMoviesDir); err != nil && !os.IsNotExist(err) {
		t.Logf("Normal removal failed, trying with sudo: %v", err)
		cmd := exec.Command("sudo", "rm", "-rf", symlinkMoviesDir)
		if output, sudoErr := cmd.CombinedOutput(); sudoErr != nil {
			t.Logf("Warning: Failed to clean symlink directory: %v\nOutput: %s", sudoErr, string(output))
		}
	}

	// Step 6: Restart containers
	t.Logf("Starting containers with clean state...")
	cmd = exec.Command("docker", "compose", "-f", composeFile, "up", "-d")
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to start containers: %w\nOutput: %s", err, string(output))
	}

	t.Logf("=== Cleanup complete, containers restarting ===")
	return nil
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
	req, err := http.NewRequest(http.MethodGet, jellyfinURL+"/Library/VirtualFolders", nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("X-MediaBrowser-Token", apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to query virtual folders: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to get virtual folders (HTTP %d)", resp.StatusCode)
	}

	var folders []struct {
		Name   string `json:"Name"`
		ItemId string `json:"ItemId"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&folders); err != nil {
		return "", fmt.Errorf("failed to decode virtual folders: %w", err)
	}

	for _, folder := range folders {
		if folder.Name == libraryName {
			return folder.ItemId, nil
		}
	}

	return "", fmt.Errorf("library '%s' not found", libraryName)
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
