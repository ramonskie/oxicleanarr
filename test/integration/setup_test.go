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

// TestMain manages the Docker environment lifecycle for all integration tests
func TestMain(m *testing.M) {
	// Start Docker environment once for all tests
	fmt.Println("========================================")
	fmt.Println("TestMain: Starting Docker environment...")
	fmt.Println("========================================")

	assetsDir := filepath.Join("..", "assets")

	// Stop any existing containers and cleanup
	stopCmd := exec.Command("docker-compose", "down", "-v", "--remove-orphans")
	stopCmd.Dir = assetsDir
	stopCmd.Stdout = os.Stdout
	stopCmd.Stderr = os.Stderr
	_ = stopCmd.Run() // Ignore errors if nothing to stop

	// Start fresh environment
	startCmd := exec.Command("docker-compose", "up", "-d", "--build")
	startCmd.Dir = assetsDir
	startCmd.Stdout = os.Stdout
	startCmd.Stderr = os.Stderr

	if err := startCmd.Run(); err != nil {
		fmt.Printf("FATAL: Failed to start docker-compose: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✅ Docker environment started")
	fmt.Println("Running all integration tests...")
	fmt.Println()

	// Run all tests
	exitCode := m.Run()

	// Cleanup after all tests (unless KEEP_TEST_ENV is set)
	if os.Getenv("KEEP_TEST_ENV") != "" {
		fmt.Println()
		fmt.Println("========================================")
		fmt.Println("TestMain: KEEP_TEST_ENV is set - skipping cleanup")
		fmt.Println("========================================")
		fmt.Println("⚠️  Docker environment is still running for debugging")
		fmt.Println("⚠️  To stop manually, run:")
		fmt.Printf("    cd %s /config/config.yml && docker-compose down -v --remove-orphans\n", assetsDir)
		fmt.Println()
	} else {
		fmt.Println()
		fmt.Println("========================================")
		fmt.Println("TestMain: Cleaning up Docker environment...")
		fmt.Println("========================================")

		cleanupCmd := exec.Command("docker-compose", "down", "-v", "--remove-orphans")
		cleanupCmd.Dir = assetsDir
		cleanupCmd.Stdout = os.Stdout
		cleanupCmd.Stderr = os.Stderr
		_ = cleanupCmd.Run() // Best effort cleanup

		fmt.Println("✅ Cleanup complete")
	}

	os.Exit(exitCode)
}

// infrastructureReady tracks if infrastructure has been set up in this test run
var infrastructureReady = false

// TestIntegrationSuite runs all integration tests in order with shared infrastructure
// This ensures TestSymlinkLifecycle has the required environment built by TestInfrastructureSetup
func TestIntegrationSuite(t *testing.T) {
	// Run infrastructure setup first (builds environment, imports 7 movies)
	t.Run("InfrastructureSetup", func(t *testing.T) {
		testInfrastructureSetup(t)
		infrastructureReady = true
	})

	// Run symlink lifecycle tests second (uses existing environment)
	t.Run("SymlinkLifecycle", func(t *testing.T) {
		// If infrastructure wasn't set up (due to -run filter), set it up now
		if !infrastructureReady {
			t.Log("⚠️  Infrastructure not ready (filtered by -run), setting up now...")
			testInfrastructureSetup(t)
			infrastructureReady = true
		}
		testSymlinkLifecycle(t)
	})

	// Run exclusion lifecycle tests third (uses existing environment)
	t.Run("ExclusionLifecycle", func(t *testing.T) {
		// If infrastructure wasn't set up (due to -run filter), set it up now
		if !infrastructureReady {
			t.Log("⚠️  Infrastructure not ready (filtered by -run), setting up now...")
			testInfrastructureSetup(t)
			infrastructureReady = true
		}
		testExclusionLifecycle(t)
	})
}

// testInfrastructureSetup validates that the Docker environment can start reliably
// This is the actual test implementation, called by TestIntegrationSuite or standalone
func testInfrastructureSetup(t *testing.T) {
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

	// Note: OxiCleanarr will start but crash due to missing API keys in config.
	// We'll check its health after config is populated and it's restarted (Step 16).
	oxicleanURL := "http://localhost:8080"

	// Step 6: Initialize Jellyfin
	t.Log("Step 6: Initializing Jellyfin...")
	composeFilePath := filepath.Join("..", "assets", "docker-compose.yml")
	absComposeFile, err := filepath.Abs(composeFilePath)
	if err != nil {
		t.Fatalf("Failed to resolve compose file path: %v", err)
	}
	jellyfinUserID, jellyfinAPIKey, err := SetupJellyfinForTest(t, jellyfinURL, JellyfinAdminUser, JellyfinAdminPass, absComposeFile)
	if err != nil {
		t.Fatalf("Failed to initialize Jellyfin: %v", err)
	}
	t.Logf("✅ Jellyfin initialized (User ID: %s, API key: %s)", jellyfinUserID[:8]+"...", jellyfinAPIKey[:8]+"...")

	// Step 6a: Extract Radarr API key from Docker container
	t.Log("Step 6a: Extracting Radarr API key from container...")
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

	// Step 6b: Verify OxiCleanarr plugin installation
	t.Log("Step 6b: Verifying OxiCleanarr Bridge plugin status...")
	if err := VerifyOxiCleanarrPlugin(t, jellyfinURL, jellyfinAPIKey); err != nil {
		t.Fatalf("❌ Plugin verification failed: %v", err)
	}

	// Step 6c: Verify OxiCleanarr plugin API endpoint
	t.Log("Step 6c: Verifying OxiCleanarr plugin API endpoint...")
	if err := VerifyOxiCleanarrPluginAPI(t, jellyfinURL, jellyfinAPIKey); err != nil {
		t.Fatalf("❌ Plugin API verification failed: %v", err)
	}

	// Step 7: Initialize Radarr
	t.Log("Step 7: Initializing Radarr...")
	movieCount, err := SetupRadarrForTest(t, radarrURL, radarrAPIKey)
	if err != nil {
		t.Fatalf("Failed to initialize Radarr: %v", err)
	}
	t.Logf("✅ Radarr initialized with %d movies", movieCount)

	// Step 8: Import movies into Radarr
	t.Log("Step 8: Importing movies into Radarr...")
	if err := EnsureRadarrMoviesExist(t, radarrURL, radarrAPIKey); err != nil {
		t.Fatalf("Failed to import movies into Radarr: %v", err)
	}
	t.Log("✅ Radarr movies imported successfully")

	// TODO Phase 2: Jellyfin scan and count validation
	// Steps 10-11 skipped for Phase 1 (infrastructure validation only)
	// - Add ScanJellyfinLibrary() function
	// - Add GetJellyfinMovieCount() function
	// - Validate movie count matches between Radarr and Jellyfin

	// Step 9: Create Jellyfin movie library for testing
	t.Log("Step 9: Creating Jellyfin movie library...")
	if err := EnsureJellyfinLibrary(t, jellyfinURL, jellyfinAPIKey, "Movies", "/media/movies", "movies"); err != nil {
		t.Fatalf("Failed to create Jellyfin movie library: %v", err)
	}
	t.Log("✅ Jellyfin movie library created")

	// Step 10: Get Jellyfin library ID
	t.Log("Step 10: Getting Jellyfin library ID...")
	libraryID, err := GetJellyfinLibraryID(t, jellyfinURL, jellyfinAPIKey, "Movies")
	if err != nil {
		t.Fatalf("Failed to get Jellyfin library ID: %v", err)
	}
	t.Logf("✅ Jellyfin library ID: %s", libraryID)

	// Step 11: Trigger Jellyfin library scan to import movies
	t.Log("Step 11: Triggering Jellyfin library scan...")
	if err := TriggerJellyfinLibraryScan(t, jellyfinURL, jellyfinAPIKey, libraryID); err != nil {
		t.Fatalf("Failed to trigger Jellyfin library scan: %v", err)
	}
	t.Log("✅ Jellyfin library scan triggered")

	// Step 12: Wait for Jellyfin to match all movies
	t.Log("Step 12: Waiting for Jellyfin to match movies...")
	if err := WaitForJellyfinMovies(t, jellyfinURL, jellyfinAPIKey, movieCount, libraryID); err != nil {
		t.Fatalf("Failed to wait for Jellyfin movie matching: %v", err)
	}
	t.Logf("✅ Jellyfin matched %d movies", movieCount)

	// Step 13: Update OxiCleanarr config with real API keys
	t.Log("Step 13: Updating OxiCleanarr config with API keys...")
	configPath := filepath.Join("..", "assets", "config", "config.yaml")
	absConfigPath, err := filepath.Abs(configPath)
	if err != nil {
		t.Fatalf("Failed to resolve config path: %v", err)
	}
	UpdateConfigAPIKeys(t, absConfigPath, jellyfinAPIKey, radarrAPIKey)
	t.Log("✅ Config updated with API keys")

	// Step 14: Restart OxiCleanarr to reload config
	t.Log("Step 14: Restarting OxiCleanarr to reload config...")
	RestartOxiCleanarr(t, absComposeFile)
	t.Log("✅ OxiCleanarr restarted")

	// Step 15: Wait for OxiCleanarr to be ready after restart
	t.Log("Step 15: Waiting for OxiCleanarr to be ready after restart...")
	if err := waitForDockerService(ctx, t, oxicleanURL+"/health", 30*time.Second); err != nil {
		t.Fatalf("OxiCleanarr not ready after restart: %v", err)
	}
	t.Log("✅ OxiCleanarr ready after restart")

	// Step 16: Validate data consistency BEFORE OxiCleanarr sync (which will create symlink libraries)
	t.Log("Step 16: Validating data consistency across services (before sync)...")
	radarrCount, err := GetRadarrMovieCount(t, radarrURL, radarrAPIKey)
	if err != nil {
		t.Fatalf("Failed to get Radarr movie count: %v", err)
	}
	jellyfinCount, err := GetJellyfinMovieCount(t, jellyfinURL, jellyfinAPIKey, libraryID)
	if err != nil {
		t.Fatalf("Failed to get Jellyfin movie count: %v", err)
	}

	t.Logf("Data consistency check (pre-sync): Radarr=%d, Jellyfin=%d", radarrCount, jellyfinCount)

	if radarrCount != movieCount {
		t.Fatalf("Radarr count mismatch: expected %d, got %d", movieCount, radarrCount)
	}
	if jellyfinCount != movieCount {
		t.Fatalf("Jellyfin count mismatch: expected %d, got %d", movieCount, jellyfinCount)
	}
	t.Log("✅ Data consistency validated: Radarr and Jellyfin both report 7 movies")

	// Step 17: Create OxiCleanarr test client and authenticate
	t.Log("Step 17: Authenticating with OxiCleanarr...")
	client := NewTestClient(t, oxicleanURL)
	client.Authenticate(AdminUsername, AdminPassword)
	t.Log("✅ Authenticated with OxiCleanarr")

	// Step 18: Trigger OxiCleanarr full sync (this will create "Leaving Soon" libraries)
	t.Log("Step 18: Triggering OxiCleanarr full sync...")
	client.TriggerSync()
	t.Log("✅ Full sync triggered")

	// Step 19: Verify OxiCleanarr synced movies from Radarr
	syncedMovieCount := client.GetMovieCount()
	t.Logf("✅ OxiCleanarr synced %d movies from Radarr", syncedMovieCount)

	if syncedMovieCount != movieCount {
		t.Fatalf("OxiCleanarr count mismatch: expected %d, got %d", movieCount, syncedMovieCount)
	}
	t.Log("✅ OxiCleanarr validation complete: 7 movies synced")

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
