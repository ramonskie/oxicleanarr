package integration

import (
	"context"
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// TestInfrastructureSetup validates that the Docker environment can start reliably
// Phase 1: Basic infrastructure validation (Jellyfin + Radarr + OxiCleanarr)
// Phase 2 (later): Symlink lifecycle tests
func TestInfrastructureSetup(t *testing.T) {
	if os.Getenv("OXICLEANARR_INTEGRATION_TEST") != "1" {
		t.Skip("Integration tests require OXICLEANARR_INTEGRATION_TEST=1")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Step 1: Stop any existing containers
	t.Log("Step 1: Stopping existing containers...")
	stopDockerEnvironment(t)

	// Step 2: Force cleanup directories
	t.Log("Step 2: Force cleanup test directories...")
	cleanupTestDirectoriesForced(t)

	// Step 3: Start docker-compose environment
	t.Log("Step 3: Starting docker-compose environment...")
	startDockerEnvironment(t)
	defer func() {
		t.Log("Cleanup: Stopping docker-compose environment...")
		stopDockerEnvironment(t)
	}()

	// Step 4: Wait for Jellyfin to be ready
	t.Log("Step 4: Waiting for Jellyfin to be ready...")
	jellyfinURL := "http://localhost:8096"
	if err := waitForDockerService(ctx, t, jellyfinURL+"/health", 60*time.Second); err != nil {
		t.Fatalf("Jellyfin failed to become ready: %v", err)
	}
	t.Log("✅ Jellyfin is ready")

	// Step 5: Wait for Radarr to be ready
	t.Log("Step 5: Waiting for Radarr to be ready...")
	radarrURL := "http://localhost:7878"
	if err := waitForDockerService(ctx, t, radarrURL+"/ping", 60*time.Second); err != nil {
		t.Fatalf("Radarr failed to become ready: %v", err)
	}
	t.Log("✅ Radarr is ready")

	// Step 6: Wait for OxiCleanarr to be ready
	t.Log("Step 6: Waiting for OxiCleanarr to be ready...")
	oxicleanURL := "http://localhost:8080"
	if err := waitForDockerService(ctx, t, oxicleanURL+"/health", 30*time.Second); err != nil {
		t.Fatalf("OxiCleanarr failed to become ready: %v", err)
	}
	t.Log("✅ OxiCleanarr is ready")

	// Step 7: Initialize Jellyfin
	t.Log("Step 7: Initializing Jellyfin...")
	composeFilePath := filepath.Join("..", "assets", "docker-compose.yml")
	absComposeFile, err := filepath.Abs(composeFilePath)
	if err != nil {
		t.Fatalf("Failed to resolve compose file path: %v", err)
	}
	jellyfinUserID, jellyfinAPIKey, err := SetupJellyfinForTest(t, jellyfinURL, "admin", "adminpassword", absComposeFile)
	if err != nil {
		t.Fatalf("Failed to initialize Jellyfin: %v", err)
	}
	t.Logf("✅ Jellyfin initialized (User ID: %s, API key: %s)", jellyfinUserID[:8]+"...", jellyfinAPIKey[:8]+"...")

	// Step 7a: Extract Radarr API key from Docker container
	t.Log("Step 7a: Extracting Radarr API key from container...")
	cmd := exec.Command("docker", "exec", "oxicleanarr-test-radarr", "cat", "/config/config.xml")
	configXML, err := cmd.Output()
	if err != nil {
		t.Fatalf("Failed to read Radarr config.xml: %v", err)
	}

	// Parse XML to extract API key
	var radarrConfig RadarrConfig
	if err := xml.Unmarshal(configXML, &radarrConfig); err != nil {
		t.Fatalf("Failed to parse Radarr config: %v", err)
	}
	radarrAPIKey := radarrConfig.ApiKey
	if radarrAPIKey == "" {
		t.Fatal("Radarr API key is empty after extraction")
	}
	t.Logf("✅ Radarr API key extracted: %s...", radarrAPIKey[:8])

	// Step 7b: Verify OxiCleanarr plugin installation
	t.Log("Step 7b: Verifying OxiCleanarr Bridge plugin status...")
	if err := VerifyOxiCleanarrPlugin(t, jellyfinURL, jellyfinAPIKey); err != nil {
		t.Fatalf("❌ Plugin verification failed: %v", err)
	}

	// Step 7c: Verify OxiCleanarr plugin API endpoint
	t.Log("Step 7c: Verifying OxiCleanarr plugin API endpoint...")
	if err := VerifyOxiCleanarrPluginAPI(t, jellyfinURL, jellyfinAPIKey); err != nil {
		t.Fatalf("❌ Plugin API verification failed: %v", err)
	}

	// Step 8: Initialize Radarr
	t.Log("Step 8: Initializing Radarr...")
	movieCount, err := SetupRadarrForTest(t, radarrURL, radarrAPIKey)
	if err != nil {
		t.Fatalf("Failed to initialize Radarr: %v", err)
	}
	t.Logf("✅ Radarr initialized with %d movies", movieCount)

	// Step 9: Import movies into Radarr
	t.Log("Step 9: Importing movies into Radarr...")
	if err := EnsureRadarrMoviesExist(t, radarrURL, radarrAPIKey); err != nil {
		t.Fatalf("Failed to import movies into Radarr: %v", err)
	}
	t.Log("✅ Radarr movies imported successfully")

	// TODO Phase 2: Jellyfin scan and count validation
	// Steps 10-11 skipped for Phase 1 (infrastructure validation only)
	// - Add ScanJellyfinLibrary() function
	// - Add GetJellyfinMovieCount() function
	// - Validate movie count matches between Radarr and Jellyfin

	// Step 10: Create Jellyfin movie library for testing
	t.Log("Step 10: Creating Jellyfin movie library...")
	if err := EnsureJellyfinLibrary(t, jellyfinURL, jellyfinAPIKey, "Movies", "/media/movies", "movies"); err != nil {
		t.Fatalf("Failed to create Jellyfin movie library: %v", err)
	}
	t.Log("✅ Jellyfin movie library created")

	// Step 11: Get Jellyfin library ID
	t.Log("Step 11: Getting Jellyfin library ID...")
	libraryID, err := GetJellyfinLibraryID(t, jellyfinURL, jellyfinAPIKey, "Movies")
	if err != nil {
		t.Fatalf("Failed to get Jellyfin library ID: %v", err)
	}
	t.Logf("✅ Jellyfin library ID: %s", libraryID)

	// Step 12: Trigger Jellyfin library scan to import movies
	t.Log("Step 12: Triggering Jellyfin library scan...")
	if err := TriggerJellyfinLibraryScan(t, jellyfinURL, jellyfinAPIKey, libraryID); err != nil {
		t.Fatalf("Failed to trigger Jellyfin library scan: %v", err)
	}
	t.Log("✅ Jellyfin library scan triggered")

	// Step 13: Wait for Jellyfin to match all movies
	t.Log("Step 13: Waiting for Jellyfin to match movies...")
	if err := WaitForJellyfinMovies(t, jellyfinURL, jellyfinAPIKey, movieCount, libraryID); err != nil {
		t.Fatalf("Failed to wait for Jellyfin movie matching: %v", err)
	}
	t.Logf("✅ Jellyfin matched %d movies", movieCount)

	// Step 14: Update OxiCleanarr config with real API keys
	t.Log("Step 14: Updating OxiCleanarr config with API keys...")
	configPath := filepath.Join("..", "assets", "config", "config.yaml")
	absConfigPath, err := filepath.Abs(configPath)
	if err != nil {
		t.Fatalf("Failed to resolve config path: %v", err)
	}
	UpdateConfigAPIKeys(t, absConfigPath, jellyfinAPIKey, radarrAPIKey)
	t.Log("✅ Config updated with API keys")

	// Step 15: Restart OxiCleanarr to reload config
	t.Log("Step 15: Restarting OxiCleanarr to reload config...")
	RestartOxiCleanarr(t, absComposeFile)
	t.Log("✅ OxiCleanarr restarted")

	// Step 16: Wait for OxiCleanarr to be ready after restart
	t.Log("Step 16: Waiting for OxiCleanarr to be ready after restart...")
	if err := waitForDockerService(ctx, t, oxicleanURL+"/health", 30*time.Second); err != nil {
		t.Fatalf("OxiCleanarr not ready after restart: %v", err)
	}
	t.Log("✅ OxiCleanarr ready after restart")

	// Step 17: Create OxiCleanarr test client and authenticate
	t.Log("Step 17: Authenticating with OxiCleanarr...")
	client := NewTestClient(t, oxicleanURL)
	client.Authenticate("admin", "adminpassword")
	t.Log("✅ Authenticated with OxiCleanarr")

	// Step 18: Trigger OxiCleanarr full sync
	t.Log("Step 18: Triggering OxiCleanarr full sync...")
	client.TriggerSync()
	t.Log("✅ Full sync triggered")

	// Step 19: Verify OxiCleanarr synced movies from Radarr
	t.Log("Step 19: Verifying OxiCleanarr synced movies...")
	syncedMovieCount := client.GetMovieCount()
	if syncedMovieCount != movieCount {
		t.Fatalf("OxiCleanarr movie count mismatch: expected %d, got %d", movieCount, syncedMovieCount)
	}
	t.Logf("✅ OxiCleanarr synced %d movies from Radarr", syncedMovieCount)

	// Step 20: Validate data consistency across all services
	t.Log("Step 20: Validating data consistency across services...")
	radarrCount, err := GetRadarrMovieCount(t, radarrURL, radarrAPIKey)
	if err != nil {
		t.Fatalf("Failed to get Radarr movie count: %v", err)
	}
	jellyfinCount, err := GetJellyfinMovieCount(t, jellyfinURL, jellyfinAPIKey, libraryID)
	if err != nil {
		t.Fatalf("Failed to get Jellyfin movie count: %v", err)
	}

	t.Logf("Data consistency check: Radarr=%d, Jellyfin=%d, OxiCleanarr=%d",
		radarrCount, jellyfinCount, syncedMovieCount)

	if radarrCount != movieCount {
		t.Fatalf("Radarr count mismatch: expected %d, got %d", movieCount, radarrCount)
	}
	if jellyfinCount != movieCount {
		t.Fatalf("Jellyfin count mismatch: expected %d, got %d", movieCount, jellyfinCount)
	}
	if syncedMovieCount != movieCount {
		t.Fatalf("OxiCleanarr count mismatch: expected %d, got %d", movieCount, syncedMovieCount)
	}
	t.Log("✅ Data consistency validated: All services report 7 movies")

	// Infrastructure Setup Complete
	t.Log("\n========================================")
	t.Log("✅ Infrastructure Setup Test PASSED")
	t.Log("========================================")
	t.Log("Summary:")
	t.Logf("  - Jellyfin: Ready with OxiCleanarr plugin and %d movies (%s)", jellyfinCount, jellyfinURL)
	t.Logf("  - Radarr: Ready with %d movies (%s)", radarrCount, radarrURL)
	t.Logf("  - OxiCleanarr: Ready and synced %d movies (%s)", syncedMovieCount, oxicleanURL)
	t.Logf("  - Data consistency: All 3 services validated with matching counts")
	t.Log("\nNext Steps:")
	t.Log("  - Run TestSymlinkLifecycle to test symlink creation/cleanup")
	t.Log("  - Run TestHideWhenEmpty to test empty library behavior")
}

// startDockerEnvironment starts the docker-compose stack
func startDockerEnvironment(t *testing.T) {
	t.Helper()
	assetsDir := filepath.Join("..", "assets")
	cmd := exec.Command("docker-compose", "up", "-d", "--build")
	cmd.Dir = assetsDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to start docker-compose: %v", err)
	}
}

// stopDockerEnvironment stops the docker-compose stack and removes volumes
func stopDockerEnvironment(t *testing.T) {
	t.Helper()
	assetsDir := filepath.Join("..", "assets")
	cmd := exec.Command("docker-compose", "down", "-v", "--remove-orphans")
	cmd.Dir = assetsDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Don't fail test if stop fails (might not be running)
	_ = cmd.Run()
}

// waitForDockerService polls a URL until it responds or timeout is reached
func waitForDockerService(ctx context.Context, t *testing.T, url string, timeout time.Duration) error {
	t.Helper()
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled")
		default:
		}

		cmd := exec.Command("curl", "-f", "-s", url)
		if err := cmd.Run(); err == nil {
			return nil
		}

		time.Sleep(2 * time.Second)
	}

	return fmt.Errorf("service at %s did not become ready within %v", url, timeout)
}

// cleanupTestDirectoriesForced removes all test data directories with sudo
func cleanupTestDirectoriesForced(t *testing.T) {
	t.Helper()
	// Docker volumes will be cleaned by docker-compose down -v
	// This function is a placeholder for future directory cleanup if needed
	t.Log("Test directories will be cleaned via docker-compose down -v")
}
