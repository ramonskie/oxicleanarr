package integration

import (
	"archive/zip"
	"bytes"
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
)

// JellyfinSetup handles automated Jellyfin setup via Startup Wizard API
type JellyfinSetup struct {
	baseURL  string
	username string
	password string
	client   *http.Client
	t        *testing.T
}

// NewJellyfinSetup creates a new Jellyfin setup helper
func NewJellyfinSetup(t *testing.T, baseURL, username, password string) *JellyfinSetup {
	return &JellyfinSetup{
		baseURL:  baseURL,
		username: username,
		password: password,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		t: t,
	}
}

// WaitForReady waits for Jellyfin to be accessible
func (js *JellyfinSetup) WaitForReady(maxRetries int, retryDelay time.Duration) error {
	js.t.Logf("Waiting for Jellyfin to be ready at %s...", js.baseURL)

	for i := 0; i < maxRetries; i++ {
		// Try health endpoint first
		if resp, err := js.client.Get(js.baseURL + "/health"); err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				js.t.Logf("Jellyfin is ready!")
				return nil
			}
		}

		// Fallback to public info endpoint
		if resp, err := js.client.Get(js.baseURL + "/System/Info/Public"); err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				js.t.Logf("Jellyfin is ready!")
				return nil
			}
		}

		if (i+1)%10 == 0 {
			js.t.Logf("Still waiting... (%d/%d)", i+1, maxRetries)
		}
		time.Sleep(retryDelay)
	}

	return fmt.Errorf("jellyfin failed to start after %v", time.Duration(maxRetries)*retryDelay)
}

// CheckSetupStatus returns true if setup wizard needs to be completed
func (js *JellyfinSetup) CheckSetupStatus() (bool, error) {
	js.t.Logf("Checking if setup wizard is needed...")

	// Try to get startup configuration
	resp, err := js.client.Get(js.baseURL + "/Startup/Configuration")
	if err == nil {
		resp.Body.Close()
		// If we get a response, check if User endpoint exists (means wizard not completed)
		if resp.StatusCode == http.StatusOK {
			userResp, err := js.client.Get(js.baseURL + "/Startup/User")
			if err == nil {
				userResp.Body.Close()
				if userResp.StatusCode == http.StatusOK {
					js.t.Logf("Setup wizard needs to be completed")
					return true, nil
				}
			}
		}
	}

	// Check if we can get public system info without auth (means setup complete)
	resp, err = js.client.Get(js.baseURL + "/System/Info/Public")
	if err == nil {
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			js.t.Logf("Setup wizard already completed")
			return false, nil
		}
	}

	// Default: assume setup needed
	js.t.Logf("Setup wizard needs to be completed")
	return true, nil
}

// SetLanguage sets the preferred language (optional step)
func (js *JellyfinSetup) SetLanguage(language string) error {
	js.t.Logf("Setting preferred language to %s...", language)

	reqBody := map[string]string{
		"UICulture":                 language,
		"MetadataCountryCode":       "US",
		"PreferredMetadataLanguage": "en",
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	resp, err := js.client.Post(
		js.baseURL+"/Startup/Configuration",
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		js.t.Logf("Warning: Failed to set language (non-critical)")
		return nil // Don't fail on this
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent {
		js.t.Logf("Language set successfully")
	} else {
		js.t.Logf("Warning: Language setting returned status %d (non-critical)", resp.StatusCode)
	}
	return nil
}

// CreateAdminUser creates the admin user via startup wizard
func (js *JellyfinSetup) CreateAdminUser() error {
	js.t.Logf("Creating admin user: %s", js.username)

	reqBody := map[string]string{
		"Name":     js.username,
		"Password": js.password,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	resp, err := js.client.Post(
		js.baseURL+"/Startup/User",
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		return fmt.Errorf("failed to create admin user: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create admin user (HTTP %d): %s", resp.StatusCode, string(bodyBytes))
	}

	js.t.Logf("Admin user created successfully")
	return nil
}

// CompleteWizard completes the startup wizard
func (js *JellyfinSetup) CompleteWizard() error {
	js.t.Logf("Completing startup wizard...")

	resp, err := js.client.Post(
		js.baseURL+"/Startup/Complete",
		"application/json",
		bytes.NewReader([]byte("{}")),
	)
	if err != nil {
		return fmt.Errorf("failed to complete wizard: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("failed to complete wizard (HTTP %d)", resp.StatusCode)
	}

	js.t.Logf("Startup wizard completed")
	return nil
}

// Authenticate logs in and returns user ID and access token
func (js *JellyfinSetup) Authenticate() (string, string, error) {
	js.t.Logf("Authenticating as %s...", js.username)

	reqBody := map[string]string{
		"Username": js.username,
		"Pw":       js.password,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", "", err
	}

	req, err := http.NewRequest(http.MethodPost, js.baseURL+"/Users/AuthenticateByName", bytes.NewReader(body))
	if err != nil {
		return "", "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Emby-Authorization", `MediaBrowser Client="OxiCleanarr-Setup", Device="IntegrationTest", DeviceId="setup-test", Version="1.0.0"`)

	resp, err := js.client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("authentication request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", "", fmt.Errorf("authentication failed (HTTP %d): %s", resp.StatusCode, string(bodyBytes))
	}

	var authResp struct {
		User struct {
			ID string `json:"Id"`
		} `json:"User"`
		AccessToken string `json:"AccessToken"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
		return "", "", fmt.Errorf("failed to decode auth response: %w", err)
	}

	if authResp.AccessToken == "" {
		return "", "", fmt.Errorf("no access token in response")
	}

	js.t.Logf("Authentication successful")
	return authResp.User.ID, authResp.AccessToken, nil
}

// CreateAPIKey creates an API key for OxiCleanarr
func (js *JellyfinSetup) CreateAPIKey(accessToken string) (string, error) {
	js.t.Logf("Creating API key for OxiCleanarr...")

	// Check if API key already exists
	req, err := http.NewRequest(http.MethodGet, js.baseURL+"/Auth/Keys", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("X-Emby-Token", accessToken)

	resp, err := js.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to list API keys: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		var keysResp struct {
			Items []struct {
				AppName     string `json:"AppName"`
				AccessToken string `json:"AccessToken"`
			} `json:"Items"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&keysResp); err == nil {
			for _, key := range keysResp.Items {
				if key.AppName == "OxiCleanarr" {
					js.t.Logf("API key already exists (reusing)")
					return key.AccessToken, nil
				}
			}
		}
	}

	// Create new API key
	req, err = http.NewRequest(http.MethodPost, js.baseURL+"/Auth/Keys?app=OxiCleanarr", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("X-Emby-Token", accessToken)

	resp, err = js.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to create API key: %w", err)
	}
	resp.Body.Close()

	// Wait a moment for key creation
	time.Sleep(1 * time.Second)

	// Query to get the newly created key
	req, err = http.NewRequest(http.MethodGet, js.baseURL+"/Auth/Keys", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("X-Emby-Token", accessToken)

	resp, err = js.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to retrieve API keys: %w", err)
	}
	defer resp.Body.Close()

	var keysResp struct {
		Items []struct {
			AppName     string `json:"AppName"`
			AccessToken string `json:"AccessToken"`
		} `json:"Items"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&keysResp); err != nil {
		return "", fmt.Errorf("failed to decode API keys: %w", err)
	}

	for _, key := range keysResp.Items {
		if key.AppName == "OxiCleanarr" {
			js.t.Logf("API key created successfully")
			return key.AccessToken, nil
		}
	}

	return "", fmt.Errorf("API key not found after creation")
}

// AddMediaLibrary creates a Jellyfin media library
func (js *JellyfinSetup) AddMediaLibrary(accessToken, name, path, contentType string) error {
	js.t.Logf("Adding media library: %s (%s)", name, path)

	// URL encode the library name (can contain spaces, e.g. "TV Shows")
	encodedName := url.QueryEscape(name)

	reqBody := map[string]interface{}{
		"LibraryOptions": map[string]interface{}{
			"EnablePhotos":                          true,
			"EnableRealtimeMonitor":                 false,
			"EnableChapterImageExtraction":          false,
			"ExtractChapterImagesDuringLibraryScan": false,
			"PathInfos": []map[string]string{
				{"Path": path},
			},
			"SaveLocalMetadata":             false,
			"EnableAutomaticSeriesGrouping": false,
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/Library/VirtualFolders?collectionType=%s&name=%s&refreshLibrary=true",
		js.baseURL, contentType, encodedName)

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Emby-Token", accessToken)

	resp, err := js.client.Do(req)
	if err != nil {
		js.t.Logf("Warning: Failed to create media library (non-critical)")
		return nil // Don't fail if library creation fails
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent {
		js.t.Logf("Media library '%s' created", name)
	} else {
		js.t.Logf("Warning: Media library creation returned status %d (non-critical)", resp.StatusCode)
	}
	return nil
}

// SetupJellyfinForTest runs the complete Jellyfin setup workflow
// Returns: (userID, apiKey, error)
func SetupJellyfinForTest(t *testing.T, jellyfinURL, username, password, composeFile string) (string, string, error) {
	setup := NewJellyfinSetup(t, jellyfinURL, username, password)

	// Wait for Jellyfin to be ready
	if err := setup.WaitForReady(60, 2*time.Second); err != nil {
		return "", "", err
	}

	// Check if setup is needed
	needsSetup, err := setup.CheckSetupStatus()
	if err != nil {
		return "", "", err
	}

	var userID, accessToken, apiKey string

	if !needsSetup {
		// Already setup - just authenticate and get/create API key
		t.Logf("Jellyfin already configured, authenticating...")
		userID, accessToken, err = setup.Authenticate()
		if err != nil {
			return "", "", fmt.Errorf("authentication failed: %w", err)
		}

		apiKey, err = setup.CreateAPIKey(accessToken)
		if err != nil {
			return "", "", fmt.Errorf("API key creation failed: %w", err)
		}

		t.Logf("Successfully authenticated with existing setup")

		// Install the Leaving Soon plugin into the Jellyfin container (mirrors the
		// old OxiCleanarr-bridge installer, which ran during infrastructure setup).
		if err := InstallLeavingSoonPluginToContainer(t, composeFile, apiKey); err != nil {
			return "", "", err
		}

		return userID, apiKey, nil
	}

	// Run setup wizard
	t.Logf("Running automated Jellyfin setup wizard...")

	// Step 1: Set language (optional)
	_ = setup.SetLanguage("en-US")

	// Step 2: Create admin user (required)
	if err := setup.CreateAdminUser(); err != nil {
		return "", "", err
	}

	// Step 3: Complete wizard (required)
	if err := setup.CompleteWizard(); err != nil {
		return "", "", err
	}

	// Give Jellyfin a moment to finish setup
	time.Sleep(2 * time.Second)

	// Step 4: Authenticate (required for API key)
	userID, accessToken, err = setup.Authenticate()
	if err != nil {
		return "", "", err
	}

	// Step 5: Create API key
	apiKey, err = setup.CreateAPIKey(accessToken)
	if err != nil {
		return "", "", err
	}

	t.Logf("Jellyfin setup completed successfully")
	t.Logf("  User ID: %s", userID)
	t.Logf("  API Key: %s...", apiKey[:8])

	// Install the Leaving Soon plugin into the Jellyfin container (mirrors the old
	// OxiCleanarr-bridge installer, which ran during infrastructure setup).
	if err := InstallLeavingSoonPluginToContainer(t, composeFile, apiKey); err != nil {
		return "", "", err
	}

	return userID, apiKey, nil
}

// EnsureJellyfinLibrary ensures a media library exists in Jellyfin
func EnsureJellyfinLibrary(t *testing.T, jellyfinURL, apiKey, name, path, contentType string) error {
	setup := NewJellyfinSetup(t, jellyfinURL, "", "")

	// First check if library already exists
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest(http.MethodGet, jellyfinURL+"/Library/VirtualFolders", nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-MediaBrowser-Token", apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to list virtual folders: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		var folders []struct {
			Name string `json:"Name"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&folders); err == nil {
			for _, folder := range folders {
				if folder.Name == name {
					t.Logf("Library '%s' already exists", name)
					return nil
				}
			}
		}
	}

	// Create library if it doesn't exist
	return setup.AddMediaLibrary(apiKey, name, path, contentType)
}

// isWithinDir reports whether path stays inside the given base directory.
// Used to guard against zip-slip and path traversal during extraction.
func isWithinDir(base, path string) bool {
	rel, err := filepath.Rel(base, path)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// InstallLeavingSoonPluginToContainer downloads and installs the Leaving Soon
// plugin into a Docker container. This is the container-aware version that uses
// docker cp to install into Jellyfin's /config/plugins/ directory.
//
// The release zip only contains the DLL. Jellyfin also needs a meta.json manifest
// and a config.xml, so those are written into the extracted folder before the
// docker cp. The config.xml points the plugin's OxiCleanarr provider at the
// container URL so it can poll /api/media/leaving-soon.
func InstallLeavingSoonPluginToContainer(t *testing.T, composeFile, apiKey string) error {
	t.Logf("Installing Leaving Soon plugin to container: %s", "oxicleanarr-test-jellyfin")

	// GitHub API endpoint for latest release
	releaseURL := "https://api.github.com/repos/ramonskie/jellyfin-plugin-leaving-soon/releases/latest"

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest(http.MethodGet, releaseURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// GitHub API requires User-Agent header
	req.Header.Set("User-Agent", "OxiCleanarr-IntegrationTest")

	// Use GitHub token if available to avoid rate limiting
	if githubToken := os.Getenv("GITHUB_TOKEN"); githubToken != "" {
		req.Header.Set("Authorization", "Bearer "+githubToken)
		t.Logf("Using GITHUB_TOKEN for authenticated API request")
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch release info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("GitHub API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var release struct {
		TagName string `json:"tag_name"`
		Assets  []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return fmt.Errorf("failed to decode release info: %w", err)
	}

	// Find the plugin zip file
	var downloadURL string
	for _, asset := range release.Assets {
		if strings.HasSuffix(asset.Name, ".zip") && strings.Contains(asset.Name, "LeavingSoon") {
			downloadURL = asset.BrowserDownloadURL
			break
		}
	}

	if downloadURL == "" {
		return fmt.Errorf("plugin zip not found in release %s", release.TagName)
	}

	t.Logf("Downloading plugin version %s from: %s", release.TagName, downloadURL)

	// Download zip file
	resp, err = client.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("failed to download plugin: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	// Read zip file into memory
	zipData, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read zip data: %w", err)
	}

	t.Logf("Downloaded %d bytes, extracting to temp directory", len(zipData))

	// Extract to temp directory
	tempDir, err := os.MkdirTemp("", "leaving-soon-plugin-")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	tempPluginDir := filepath.Join(tempDir, LeavingSoonPluginFolder)
	if err := os.MkdirAll(tempPluginDir, 0o755); err != nil {
		return fmt.Errorf("failed to create temp plugin directory: %w", err)
	}

	// Extract zip file to temp directory
	zipReader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return fmt.Errorf("failed to open zip: %w", err)
	}

	for _, file := range zipReader.File {
		// Open file in zip
		rc, err := file.Open()
		if err != nil {
			return fmt.Errorf("failed to open zip entry %s: %w", file.Name, err)
		}

		// Create destination file in temp directory
		destPath := filepath.Join(tempPluginDir, file.Name)

		// Guard against zip-slip: entry must not escape tempPluginDir
		if !isWithinDir(tempPluginDir, destPath) {
			rc.Close()
			return fmt.Errorf("zip entry escapes extraction directory: %s", file.Name)
		}

		if file.FileInfo().IsDir() {
			os.MkdirAll(destPath, file.Mode())
			rc.Close()
			continue
		}

		// Create parent directory if needed
		if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
			rc.Close()
			return fmt.Errorf("failed to create directory for %s: %w", destPath, err)
		}

		// Write file
		outFile, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
		if err != nil {
			rc.Close()
			return fmt.Errorf("failed to create file %s: %w", destPath, err)
		}

		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()

		if err != nil {
			return fmt.Errorf("failed to write file %s: %w", destPath, err)
		}

		t.Logf("  Extracted: %s", file.Name)
	}

	// Write the manifest and config.xml Jellyfin expects.
	if err := writeLeavingSoonMetaAndConfig(tempPluginDir); err != nil {
		return err
	}

	// Use docker cp to copy plugin into container's /config/plugins/ directory
	t.Logf("Copying plugin files to container %s:%s", "oxicleanarr-test-jellyfin", "/config/plugins/"+LeavingSoonPluginFolder)

	// First, ensure the plugins directory exists in the container
	mkdirCmd := exec.Command("docker", "exec", "oxicleanarr-test-jellyfin", "mkdir", "-p", "/config/plugins/"+LeavingSoonPluginFolder)
	if output, err := mkdirCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create plugins directory in container: %w\nOutput: %s", err, string(output))
	}

	// Copy plugin files into container
	// Note: docker cp requires source to end with "/." to copy contents (not the directory itself)
	cpCmd := exec.Command("docker", "cp", tempPluginDir+"/.", "oxicleanarr-test-jellyfin:/config/plugins/"+LeavingSoonPluginFolder+"/")
	if output, err := cpCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to copy plugin files to container: %w\nOutput: %s", err, string(output))
	}

	t.Logf("Leaving Soon plugin installed successfully to container (version %s)", release.TagName)

	// Restart Jellyfin to load the plugin.
	t.Logf("Restarting Jellyfin container to load plugin...")
	restartCmd := exec.Command("docker", "compose", "-f", composeFile, "restart", "jellyfin")
	if output, err := restartCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to restart Jellyfin: %w\nOutput: %s", err, string(output))
	}

	setup := NewJellyfinSetup(t, JellyfinURL, "", "")
	if err := setup.WaitForReady(60, 2*time.Second); err != nil {
		return fmt.Errorf("Jellyfin not ready after plugin install: %w", err)
	}
	t.Logf("Jellyfin restarted and ready with Leaving Soon plugin loaded")

	// Configure the plugin through Jellyfin's plugin config API (DB-backed, the
	// canonical mechanism in Jellyfin 10.11). The config.xml written during install
	// is a fallback for older versions; the API works regardless.
	if err := ConfigureLeavingSoonPlugin(t, apiKey); err != nil {
		return err
	}

	// Verify the provider is actually configured (ProviderCount >= 1). This turns a
	// silent "plugin loaded but no provider" failure into an explicit one.
	if err := WaitForLeavingSoonProviderCount(t, apiKey, 60*time.Second); err != nil {
		return err
	}

	t.Logf("Leaving Soon plugin configured with OxiCleanarr provider")
	return nil
}

// ConfigureLeavingSoonPlugin sets the plugin configuration via Jellyfin's
// POST /Plugins/{id}/Configuration endpoint, which stores config in the Jellyfin
// database. Jellyfin's plugin config API uses PascalCase JSON keys and the plugin
// id without dashes (verified against a live 10.11 instance).
func ConfigureLeavingSoonPlugin(t *testing.T, apiKey string) error {
	t.Helper()
	configJSON := fmt.Sprintf(`{
  "BasePath": %q,
  "MoviesLibraryName": %q,
  "TvLibraryName": %q,
  "HideWhenEmpty": true,
  "SyncIntervalMinutes": 1,
  "ForceEmptyAfterFailureCount": 3,
  "Providers": [
    {
      "Type": "oxicleanarr",
      "Name": "oxicleanarr",
      "Enabled": true,
      "Url": %q,
      "ApiKey": %q,
      "IncludeCollections": ""
    }
  ]
}`, LeavingSoonBasePath, LeavingSoonMoviesLibrary, LeavingSoonTVLibrary,
		LeavingSoonOxiCleanarrURL, OxiCleanarrTestAPIKey)

	req, err := http.NewRequest(http.MethodPost, JellyfinURL+"/Plugins/"+LeavingSoonPluginAPIID+"/Configuration",
		strings.NewReader(configJSON))
	if err != nil {
		return fmt.Errorf("failed to create plugin config request: %w", err)
	}
	req.Header.Set("X-Emby-Token", apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to configure Leaving Soon plugin: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("configuring Leaving Soon plugin returned %d: %s", resp.StatusCode, string(body))
	}

	t.Logf("Leaving Soon plugin configuration submitted")
	return nil
}

// WaitForLeavingSoonProviderCount polls the plugin status endpoint until at least
// minProviders are configured (or the timeout elapses). The plugin status endpoint
// serializes with PascalCase keys.
func WaitForLeavingSoonProviderCount(t *testing.T, apiKey string, maxWait time.Duration) error {
	t.Helper()
	deadline := time.Now().Add(maxWait)
	for {
		req, err := http.NewRequest(http.MethodGet, JellyfinURL+"/api/leaving-soon/status", nil)
		if err != nil {
			return err
		}
		req.Header.Set("X-Emby-Token", apiKey)
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			var status struct {
				ProviderCount int `json:"ProviderCount"`
			}
			decodeErr := json.NewDecoder(resp.Body).Decode(&status)
			resp.Body.Close()
			if decodeErr == nil && status.ProviderCount >= 1 {
				return nil
			}
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("Leaving Soon plugin has no providers configured after %v", maxWait)
		}
		time.Sleep(2 * time.Second)
	}
}

// writeLeavingSoonMetaAndConfig writes the meta.json manifest and config.xml
// (pointing at OxiCleanarr) into the staged plugin folder.
func writeLeavingSoonMetaAndConfig(stageDir string) error {
	// meta.json manifest. Jellyfin loads every DLL in the folder when Assemblies
	// is empty, but providing the manifest keeps the plugin name/guid explicit.
	manifest := fmt.Sprintf(`{
  "Category": "General",
  "Changelog": "Integration test build",
  "Description": "Surfaces scheduled-deletion media as symlink-backed leaving-soon libraries in Jellyfin.",
  "Id": "%s",
  "Name": "Leaving Soon",
  "Overview": "Polls provider apps for scheduled-deletion media.",
  "Owner": "ramonskie",
  "TargetAbi": "10.11.0.0",
  "Timestamp": "%s",
  "Version": "1.0.0.0",
  "Status": "Active",
  "AutoUpdate": false,
  "ImagePath": "",
  "Assemblies": ["Jellyfin.Plugin.LeavingSoon.dll"]
}`, LeavingSoonPluginGUID, time.Now().UTC().Format(time.RFC3339))

	if err := os.WriteFile(filepath.Join(stageDir, "meta.json"), []byte(manifest), 0o644); err != nil {
		return fmt.Errorf("write meta.json: %w", err)
	}

	// config.xml uses Jellyfin's XmlSerializer: root element is the config type,
	// properties are elements, List<ProviderConfig> serializes as repeated elements.
	configXML := fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<PluginConfiguration xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:xsd="http://www.w3.org/2001/XMLSchema">
  <BasePath>%s</BasePath>
  <MoviesLibraryName>%s</MoviesLibraryName>
  <TvLibraryName>%s</TvLibraryName>
  <HideWhenEmpty>true</HideWhenEmpty>
  <SyncIntervalMinutes>1</SyncIntervalMinutes>
  <ForceEmptyAfterFailureCount>3</ForceEmptyAfterFailureCount>
  <Providers>
    <ProviderConfig>
      <Type>oxicleanarr</Type>
      <Name>oxicleanarr</Name>
      <Enabled>true</Enabled>
      <Url>%s</Url>
      <ApiKey>%s</ApiKey>
      <IncludeCollections></IncludeCollections>
    </ProviderConfig>
  </Providers>
</PluginConfiguration>
`, LeavingSoonBasePath, LeavingSoonMoviesLibrary, LeavingSoonTVLibrary, LeavingSoonOxiCleanarrURL, OxiCleanarrTestAPIKey)

	if err := os.WriteFile(filepath.Join(stageDir, "config.xml"), []byte(configXML), 0o644); err != nil {
		return fmt.Errorf("write config.xml: %w", err)
	}

	return nil
}
