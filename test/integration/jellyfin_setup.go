package integration

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
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

	// URL encode the library name
	encodedName := fmt.Sprintf("%s", name) // json.Marshal handles encoding

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

// InstallOxiCleanarrPlugin downloads and installs the OxiCleanarr Bridge plugin from GitHub releases
func InstallOxiCleanarrPlugin(t *testing.T, pluginsDir string) error {
	pluginSubdir := filepath.Join(pluginsDir, "OxiCleanarr")

	// Check if plugin already exists
	if _, err := os.Stat(pluginSubdir); err == nil {
		t.Logf("OxiCleanarr plugin already installed at: %s", pluginSubdir)
		return nil
	}

	t.Logf("Installing OxiCleanarr Bridge plugin from GitHub releases...")

	// GitHub API endpoint for latest release
	releaseURL := "https://api.github.com/repos/ramonskie/jellyfin-plugin-oxicleanarr/releases/latest"

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest(http.MethodGet, releaseURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// GitHub API requires User-Agent header
	req.Header.Set("User-Agent", "OxiCleanarr-IntegrationTest")

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
		if asset.Name == "jellyfin-plugin-oxicleanarr.zip" {
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

	t.Logf("Downloaded %d bytes, extracting to: %s", len(zipData), pluginSubdir)

	// Extract to temp directory first (to avoid permission issues)
	tempDir, err := os.MkdirTemp("", "oxicleanarr-plugin-")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	tempPluginDir := filepath.Join(tempDir, "OxiCleanarr")
	if err := os.MkdirAll(tempPluginDir, 0755); err != nil {
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

		if file.FileInfo().IsDir() {
			os.MkdirAll(destPath, file.Mode())
			rc.Close()
			continue
		}

		// Create parent directory if needed
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
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

	// Use sudo to copy from temp to final location (handles permission issues)
	t.Logf("Copying plugin files to Jellyfin plugins directory (may require sudo)...")
	cmd := exec.Command("sudo", "mkdir", "-p", pluginSubdir)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create plugin directory with sudo: %w\nOutput: %s", err, string(output))
	}

	cmd = exec.Command("sudo", "cp", "-r", tempPluginDir+"/.", pluginSubdir+"/")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to copy plugin files with sudo: %w\nOutput: %s", err, string(output))
	}

	t.Logf("OxiCleanarr plugin installed successfully (version %s)", release.TagName)
	return nil
}

// InstallOxiCleanarrPluginToContainer downloads and installs the OxiCleanarr Bridge plugin into a Docker container
// This is the container-aware version that uses docker cp to install into Jellyfin's /config/plugins/ directory
func InstallOxiCleanarrPluginToContainer(t *testing.T, containerName string) error {
	t.Logf("Installing OxiCleanarr Bridge plugin to container: %s", containerName)

	// GitHub API endpoint for latest release
	releaseURL := "https://api.github.com/repos/ramonskie/jellyfin-plugin-oxicleanarr/releases/latest"

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
		if asset.Name == "jellyfin-plugin-oxicleanarr.zip" {
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
	tempDir, err := os.MkdirTemp("", "oxicleanarr-plugin-")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	tempPluginDir := filepath.Join(tempDir, "OxiCleanarr")
	if err := os.MkdirAll(tempPluginDir, 0755); err != nil {
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

		if file.FileInfo().IsDir() {
			os.MkdirAll(destPath, file.Mode())
			rc.Close()
			continue
		}

		// Create parent directory if needed
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
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

	// Use docker cp to copy plugin into container's /config/plugins/ directory
	t.Logf("Copying plugin files to container %s:/config/plugins/OxiCleanarr/", containerName)

	// First, ensure the plugins directory exists in the container
	mkdirCmd := exec.Command("docker", "exec", containerName, "mkdir", "-p", "/config/plugins/OxiCleanarr")
	if output, err := mkdirCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create plugins directory in container: %w\nOutput: %s", err, string(output))
	}

	// Copy plugin files into container
	// Note: docker cp requires source to end with "/." to copy contents (not the directory itself)
	cpCmd := exec.Command("docker", "cp", tempPluginDir+"/.", containerName+":/config/plugins/OxiCleanarr/")
	if output, err := cpCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to copy plugin files to container: %w\nOutput: %s", err, string(output))
	}

	t.Logf("OxiCleanarr plugin installed successfully to container (version %s)", release.TagName)
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

	// Install OxiCleanarr Bridge plugin before setup wizard
	// This enables symlink management via Jellyfin plugin API
	containerName := "oxicleanarr-test-jellyfin"
	if err := InstallOxiCleanarrPluginToContainer(t, containerName); err != nil {
		t.Logf("Warning: Failed to install OxiCleanarr plugin: %v", err)
		t.Logf("Continuing without plugin - symlink tests will use fallback filesystem operations")
	} else {
		// Restart Jellyfin container to load the plugin
		t.Logf("Restarting Jellyfin container to load plugin...")
		restartCmd := exec.Command("docker", "compose", "-f", composeFile, "restart", "jellyfin")
		if output, err := restartCmd.CombinedOutput(); err != nil {
			return "", "", fmt.Errorf("failed to restart Jellyfin: %w\nOutput: %s", err, string(output))
		}

		// Wait for Jellyfin to be ready again after restart
		t.Logf("Waiting for Jellyfin to be ready after restart...")
		if err := setup.WaitForReady(60, 2*time.Second); err != nil {
			return "", "", fmt.Errorf("Jellyfin not ready after restart: %w", err)
		}
		t.Logf("Jellyfin restarted and ready with OxiCleanarr plugin loaded")
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

	return userID, apiKey, nil
}

// VerifyOxiCleanarrPlugin checks if the OxiCleanarr Bridge plugin is installed and active
// This is a non-fatal check - logs a warning if plugin is missing but doesn't fail tests
func VerifyOxiCleanarrPlugin(t *testing.T, jellyfinURL, apiKey string) error {
	t.Logf("Verifying OxiCleanarr Bridge plugin installation...")

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest(http.MethodGet, jellyfinURL+"/Plugins", nil)
	if err != nil {
		return fmt.Errorf("failed to create plugins request: %w", err)
	}
	req.Header.Set("X-MediaBrowser-Token", apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to query plugins: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("plugins endpoint returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var plugins []struct {
		Name    string `json:"Name"`
		Version string `json:"Version"`
		Status  string `json:"Status"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&plugins); err != nil {
		return fmt.Errorf("failed to decode plugins response: %w", err)
	}

	// Look for OxiCleanarr plugin (case-insensitive match)
	for _, plugin := range plugins {
		if plugin.Name == "OxiCleanarr" || plugin.Name == "OxiCleanarr Bridge" {
			t.Logf("✅ OxiCleanarr plugin found: version %s, status: %s", plugin.Version, plugin.Status)
			if plugin.Status != "Active" {
				return fmt.Errorf("plugin found but status is '%s' (expected 'Active')", plugin.Status)
			}
			return nil
		}
	}

	// Plugin not found - this is a critical failure for integration tests
	return fmt.Errorf("OxiCleanarr Bridge plugin not found in Jellyfin - required for symlink integration tests")
}

// VerifyOxiCleanarrPluginAPI verifies the OxiCleanarr plugin's custom API endpoint is functional
// This checks the /api/oxicleanarr/status endpoint that OxiCleanarr will actually call
func VerifyOxiCleanarrPluginAPI(t *testing.T, jellyfinURL, apiKey string) error {
	client := &http.Client{Timeout: 10 * time.Second}

	// Call the plugin's custom API endpoint
	req, err := http.NewRequest(http.MethodGet, jellyfinURL+"/api/oxicleanarr/status", nil)
	if err != nil {
		return fmt.Errorf("failed to create status request: %w", err)
	}
	req.Header.Set("X-MediaBrowser-Token", apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to call plugin API endpoint: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("plugin API returned status %d (expected 200): %s", resp.StatusCode, string(body))
	}

	// Read and parse response
	var statusResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&statusResp); err != nil {
		return fmt.Errorf("failed to decode plugin API response: %w", err)
	}

	t.Logf("✅ OxiCleanarr plugin API is functional: %+v", statusResp)
	return nil
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
