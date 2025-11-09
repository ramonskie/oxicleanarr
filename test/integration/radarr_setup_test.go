package integration

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const (
	RadarrURL = "http://localhost:7878"
	// API key read from config.xml at runtime (auto-generated on first start)
)

// RadarrMovie represents a movie in Radarr's API
type RadarrMovie struct {
	ID               int                      `json:"id,omitempty"`
	Title            string                   `json:"title"`
	TitleSlug        string                   `json:"titleSlug"`
	QualityProfileID int                      `json:"qualityProfileId"`
	TmdbID           int                      `json:"tmdbId"`
	Year             int                      `json:"year"`
	RootFolderPath   string                   `json:"rootFolderPath"`
	Monitored        bool                     `json:"monitored"`
	HasFile          bool                     `json:"hasFile"`
	Images           []map[string]interface{} `json:"images,omitempty"`
	AddOptions       *RadarrAddOptions        `json:"addOptions,omitempty"`
}

// RadarrAddOptions represents add options for a movie
type RadarrAddOptions struct {
	SearchForMovie bool `json:"searchForMovie"`
}

// RadarrQualityProfile represents a quality profile
type RadarrQualityProfile struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// RadarrRootFolder represents a root folder
type RadarrRootFolder struct {
	ID   int    `json:"id"`
	Path string `json:"path"`
}

// TestMovies contains TMDB IDs and titles for test movies
var TestMovies = []struct {
	TmdbID int
	Title  string
}{
	{550, "Fight Club (1999)"},
	{13, "Forrest Gump (1994)"},
	{157336, "Interstellar (2014)"},
	{27205, "Inception (2010)"},
	{155, "The Dark Knight (2008)"},
	{680, "Pulp Fiction (1994)"},
	{424, "Schindler's List (1993)"},
}

// RadarrConfig represents the Radarr config.xml structure
type RadarrConfig struct {
	XMLName  xml.Name `xml:"Config"`
	ApiKey   string   `xml:"ApiKey"`
	Port     int      `xml:"Port"`
	LogLevel string   `xml:"LogLevel"`
}

// GetRadarrAPIKeyFromConfig reads the API key from Radarr's config.xml
// Deprecated: Use GetRadarrAPIKeyFromContainer for Docker-based tests
func GetRadarrAPIKeyFromConfig(configPath string) (string, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return "", fmt.Errorf("failed to read config file: %w", err)
	}

	var config RadarrConfig
	if err := xml.Unmarshal(data, &config); err != nil {
		return "", fmt.Errorf("failed to parse config XML: %w", err)
	}

	if config.ApiKey == "" {
		return "", fmt.Errorf("API key not found in config")
	}

	return config.ApiKey, nil
}

// GetRadarrAPIKeyFromContainer reads the API key from Radarr's config.xml inside a Docker container
func GetRadarrAPIKeyFromContainer(t *testing.T, containerName string) (string, error) {
	t.Helper()

	// Execute docker exec to read config.xml from container
	cmd := fmt.Sprintf("docker exec %s cat /config/config.xml", containerName)

	// Run command and capture output
	var out bytes.Buffer
	var stderr bytes.Buffer

	// Use sh -c to execute the command
	process := exec.Command("sh", "-c", cmd)
	process.Stdout = &out
	process.Stderr = &stderr

	err := process.Run()
	if err != nil {
		return "", fmt.Errorf("failed to execute docker exec: %w (stderr: %s)", err, stderr.String())
	}

	// Parse XML from stdout
	var config RadarrConfig
	if err := xml.Unmarshal(out.Bytes(), &config); err != nil {
		return "", fmt.Errorf("failed to parse config XML: %w", err)
	}

	if config.ApiKey == "" {
		return "", fmt.Errorf("API key not found in config")
	}

	return config.ApiKey, nil
}

// WaitForRadarr waits for Radarr to be ready and initialized
func WaitForRadarr(t *testing.T, radarrURL, apiKey string) error {
	t.Helper()
	t.Logf("Waiting for Radarr to be ready at %s...", radarrURL)

	client := &http.Client{Timeout: 5 * time.Second}
	maxRetries := 60 // 5 minutes max
	retryDelay := 5 * time.Second

	// Wait for API to be ready (Radarr auto-initializes on first run)
	t.Logf("Waiting for API to respond with provided API key...")
	for i := 0; i < maxRetries; i++ {
		req, err := http.NewRequest("GET", radarrURL+"/api/v3/system/status", nil)
		if err != nil {
			time.Sleep(retryDelay)
			continue
		}
		req.Header.Set("X-Api-Key", apiKey)

		resp, err := client.Do(req)
		if err != nil {
			time.Sleep(retryDelay)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode == 200 {
			t.Logf("Radarr API is ready!")
			// Additional wait for full initialization
			time.Sleep(5 * time.Second)
			return nil
		}

		time.Sleep(retryDelay)
	}

	return fmt.Errorf("Radarr did not become ready within timeout")
}

// SetupRadarrForTest initializes Radarr with test movies
// Returns the number of movies added and any error
func SetupRadarrForTest(t *testing.T, radarrURL, apiKey string) (int, error) {
	t.Helper()

	client := &http.Client{Timeout: 30 * time.Second}

	// Get quality profile ID
	qualityProfileID, err := getRadarrQualityProfile(client, radarrURL, apiKey)
	if err != nil {
		return 0, fmt.Errorf("failed to get quality profile: %w", err)
	}
	t.Logf("Radarr Quality Profile ID: %d", qualityProfileID)

	// Ensure root folder exists (create if needed)
	// Use /media/movies to match actual test media location
	rootFolder, err := ensureRadarrRootFolder(client, radarrURL, apiKey, "/media/movies")
	if err != nil {
		return 0, fmt.Errorf("failed to ensure root folder: %w", err)
	}
	t.Logf("Radarr Root Folder: %s", rootFolder)

	// Get existing movies
	existingMovies, err := getRadarrMovies(client, radarrURL, apiKey)
	if err != nil {
		return 0, fmt.Errorf("failed to get existing movies: %w", err)
	}
	existingTmdbIDs := make(map[int]bool)
	for _, movie := range existingMovies {
		existingTmdbIDs[movie.TmdbID] = true
	}
	t.Logf("Radarr has %d existing movies", len(existingMovies))

	// Add test movies
	addedCount := 0
	for _, testMovie := range TestMovies {
		if existingTmdbIDs[testMovie.TmdbID] {
			t.Logf("Movie already exists: %s (TMDB ID: %d), skipping...", testMovie.Title, testMovie.TmdbID)
			continue
		}

		t.Logf("Adding movie: %s (TMDB ID: %d)", testMovie.Title, testMovie.TmdbID)

		// Lookup movie by TMDB ID
		movieInfo, err := lookupRadarrMovieByTmdb(client, radarrURL, apiKey, testMovie.TmdbID)
		if err != nil {
			t.Logf("Warning: Failed to lookup movie %s: %v", testMovie.Title, err)
			continue
		}

		// Populate required fields
		movieInfo.QualityProfileID = qualityProfileID
		movieInfo.RootFolderPath = rootFolder
		movieInfo.Monitored = true
		movieInfo.AddOptions = &RadarrAddOptions{SearchForMovie: false}

		// Add the movie
		err = addRadarrMovie(client, radarrURL, apiKey, movieInfo)
		if err != nil {
			t.Logf("Warning: Failed to add movie %s: %v", testMovie.Title, err)
			continue
		}

		t.Logf("  Added successfully")
		addedCount++
	}

	// Get final movie count (bulk scan will be triggered in EnsureRadarrMoviesExist)
	finalMovies, err := getRadarrMovies(client, radarrURL, apiKey)
	if err != nil {
		return addedCount, fmt.Errorf("failed to get final movie count: %w", err)
	}

	t.Logf("Total movies in Radarr: %d (added %d new movies)", len(finalMovies), addedCount)

	return addedCount, nil
}

// getRadarrQualityProfile gets the first quality profile ID
func getRadarrQualityProfile(client *http.Client, radarrURL, apiKey string) (int, error) {
	req, err := http.NewRequest("GET", radarrURL+"/api/v3/qualityprofile", nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("X-Api-Key", apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var profiles []RadarrQualityProfile
	if err := json.NewDecoder(resp.Body).Decode(&profiles); err != nil {
		return 0, err
	}

	if len(profiles) == 0 {
		return 0, fmt.Errorf("no quality profiles found")
	}

	return profiles[0].ID, nil
}

// getRadarrRootFolder gets the first root folder path
func getRadarrRootFolder(client *http.Client, radarrURL, apiKey string) (string, error) {
	req, err := http.NewRequest("GET", radarrURL+"/api/v3/rootfolder", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("X-Api-Key", apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var folders []RadarrRootFolder
	if err := json.NewDecoder(resp.Body).Decode(&folders); err != nil {
		return "", err
	}

	if len(folders) == 0 {
		return "", fmt.Errorf("no root folders found")
	}

	return folders[0].Path, nil
}

// ensureRadarrRootFolder ensures a root folder exists, creating it if necessary
func ensureRadarrRootFolder(client *http.Client, radarrURL, apiKey, path string) (string, error) {
	// Check if root folders exist
	req, err := http.NewRequest("GET", radarrURL+"/api/v3/rootfolder", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("X-Api-Key", apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var folders []RadarrRootFolder
	if err := json.NewDecoder(resp.Body).Decode(&folders); err != nil {
		return "", err
	}

	// Check if the specific root folder path exists
	for _, folder := range folders {
		if folder.Path == path {
			return folder.Path, nil
		}
	}

	// No matching root folder found, create one
	createBody := map[string]interface{}{
		"path": path,
	}
	bodyBytes, err := json.Marshal(createBody)
	if err != nil {
		return "", err
	}

	req, err = http.NewRequest("POST", radarrURL+"/api/v3/rootfolder", bytes.NewReader(bodyBytes))
	if err != nil {
		return "", err
	}
	req.Header.Set("X-Api-Key", apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err = client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to create root folder (status %d): %s", resp.StatusCode, string(respBody))
	}

	var newFolder RadarrRootFolder
	if err := json.NewDecoder(resp.Body).Decode(&newFolder); err != nil {
		return "", fmt.Errorf("failed to decode new root folder response: %w", err)
	}

	return newFolder.Path, nil
}

// getRadarrMovies gets all movies from Radarr
func getRadarrMovies(client *http.Client, radarrURL, apiKey string) ([]RadarrMovie, error) {
	req, err := http.NewRequest("GET", radarrURL+"/api/v3/movie", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Api-Key", apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var movies []RadarrMovie
	if err := json.NewDecoder(resp.Body).Decode(&movies); err != nil {
		return nil, err
	}

	return movies, nil
}

// lookupRadarrMovieByTmdb looks up a movie by TMDB ID
func lookupRadarrMovieByTmdb(client *http.Client, radarrURL, apiKey string, tmdbID int) (*RadarrMovie, error) {
	url := fmt.Sprintf("%s/api/v3/movie/lookup/tmdb?tmdbId=%d", radarrURL, tmdbID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Api-Key", apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var movie RadarrMovie
	if err := json.NewDecoder(resp.Body).Decode(&movie); err != nil {
		return nil, err
	}

	return &movie, nil
}

// addRadarrMovie adds a movie to Radarr
func addRadarrMovie(client *http.Client, radarrURL, apiKey string, movie *RadarrMovie) error {
	body, err := json.Marshal(movie)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", radarrURL+"/api/v3/movie", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("X-Api-Key", apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// refreshRadarrMovie triggers Radarr to scan disk for a movie's files
func refreshRadarrMovie(client *http.Client, radarrURL, apiKey string, movieID int) error {
	command := map[string]interface{}{
		"name":    "RefreshMovie",
		"movieId": movieID,
	}

	body, err := json.Marshal(command)
	if err != nil {
		return fmt.Errorf("marshaling command: %w", err)
	}

	req, err := http.NewRequest("POST", radarrURL+"/api/v3/command", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("X-Api-Key", apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// triggerDownloadedMoviesScan triggers Radarr to scan the entire root folder for movies
// This is more efficient than refreshing each movie individually
func triggerDownloadedMoviesScan(client *http.Client, radarrURL, apiKey, folderPath string) error {
	command := map[string]interface{}{
		"name": "DownloadedMoviesScan",
		"path": folderPath,
	}

	body, err := json.Marshal(command)
	if err != nil {
		return fmt.Errorf("marshaling command: %w", err)
	}

	req, err := http.NewRequest("POST", radarrURL+"/api/v3/command", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("X-Api-Key", apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// EnsureRadarrMoviesExist is a wrapper function that can be called from tests
// It ensures Radarr is set up with test movies and returns an error if setup fails
func EnsureRadarrMoviesExist(t *testing.T, radarrURL, apiKey string) error {
	t.Helper()

	addedCount, err := SetupRadarrForTest(t, radarrURL, apiKey)
	if err != nil {
		return fmt.Errorf("Radarr setup failed: %w", err)
	}

	// Verify at least some movies exist
	client := &http.Client{Timeout: 30 * time.Second}
	movies, err := getRadarrMovies(client, radarrURL, apiKey)
	if err != nil {
		return fmt.Errorf("failed to verify Radarr movies: %w", err)
	}

	if len(movies) == 0 {
		return fmt.Errorf("no movies found in Radarr after setup")
	}

	t.Logf("Radarr setup verified: %d total movies (%d newly added)", len(movies), addedCount)

	// Trigger bulk scan of entire root folder (more efficient than per-movie refresh)
	t.Logf("Triggering DownloadedMoviesScan for /media/movies to detect all movie files...")
	if err := triggerDownloadedMoviesScan(client, radarrURL, apiKey, "/media/movies"); err != nil {
		return fmt.Errorf("failed to trigger bulk scan: %w", err)
	}
	t.Logf("Bulk scan command sent successfully")

	// Wait for bulk scan to complete
	// Poll for movies to have hasFile: true (Radarr scans asynchronously)
	t.Logf("Waiting for Radarr to complete bulk scan...")
	maxPolls := 30 // 30 seconds max (1 second per poll)
	pollInterval := 1 * time.Second

	var hasFileCount int
	for i := 0; i < maxPolls; i++ {
		time.Sleep(pollInterval)

		moviesAfterRefresh, err := getRadarrMovies(client, radarrURL, apiKey)
		if err != nil {
			return fmt.Errorf("failed to get movies after refresh: %w", err)
		}

		hasFileCount = 0
		for _, movie := range moviesAfterRefresh {
			if movie.HasFile {
				hasFileCount++
			}
		}

		t.Logf("Poll %d/%d: %d/%d movies have files", i+1, maxPolls, hasFileCount, len(moviesAfterRefresh))

		// If all movies have files, we're done
		if hasFileCount == len(moviesAfterRefresh) {
			t.Logf("All movies scanned successfully!")
			break
		}
	}

	if hasFileCount == 0 {
		return fmt.Errorf("no movies have hasFile=true after %d seconds - Radarr may need more time to scan", maxPolls)
	}

	t.Logf("Radarr refresh complete: %d/%d movies ready for sync", hasFileCount, len(movies))
	return nil
}

// GetRadarrAPIKey returns the Radarr API key from environment variable
func GetRadarrAPIKey() string {
	return os.Getenv("RADARR_API_KEY")
}

// TestRadarrSetup is a standalone test to verify Radarr setup works
func TestRadarrSetup(t *testing.T) {
	apiKey := GetRadarrAPIKey()
	require.NotEmpty(t, apiKey, "RADARR_API_KEY must be set")

	err := EnsureRadarrMoviesExist(t, RadarrURL, apiKey)
	require.NoError(t, err, "Radarr setup should succeed")
}
