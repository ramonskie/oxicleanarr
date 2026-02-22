package integration

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const (
	SonarrURL = "http://localhost:8989"
)

// SonarrSeries represents a TV series in Sonarr's API
type SonarrSeries struct {
	ID               int                      `json:"id,omitempty"`
	Title            string                   `json:"title"`
	TitleSlug        string                   `json:"titleSlug"`
	QualityProfileID int                      `json:"qualityProfileId"`
	TvdbID           int                      `json:"tvdbId"`
	Year             int                      `json:"year"`
	RootFolderPath   string                   `json:"rootFolderPath"`
	Monitored        bool                     `json:"monitored"`
	SeasonFolder     bool                     `json:"seasonFolder"`
	SeriesType       string                   `json:"seriesType"`
	Images           []map[string]interface{} `json:"images,omitempty"`
	Seasons          []SonarrSeason           `json:"seasons,omitempty"`
	AddOptions       *SonarrAddOptions        `json:"addOptions,omitempty"`
	Statistics       *SonarrSeriesStatistics  `json:"statistics,omitempty"`
	Status           string                   `json:"status,omitempty"`
}

// SonarrSeason represents a season within a series
type SonarrSeason struct {
	SeasonNumber int  `json:"seasonNumber"`
	Monitored    bool `json:"monitored"`
}

// SonarrAddOptions represents add options for a series
type SonarrAddOptions struct {
	SearchForMissingEpisodes bool `json:"searchForMissingEpisodes"`
}

// SonarrSeriesStatistics holds file counts for a series
type SonarrSeriesStatistics struct {
	EpisodeFileCount int `json:"episodeFileCount"`
	EpisodeCount     int `json:"episodeCount"`
}

// SonarrQualityProfile represents a quality profile
type SonarrQualityProfile struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// SonarrRootFolder represents a root folder
type SonarrRootFolder struct {
	ID   int    `json:"id"`
	Path string `json:"path"`
}

// SonarrConfig represents the Sonarr config.xml structure
type SonarrConfig struct {
	XMLName xml.Name `xml:"Config"`
	ApiKey  string   `xml:"ApiKey"`
	Port    int      `xml:"Port"`
}

// TestTVShows contains TVDB IDs and titles for test TV shows.
// These match the assets in test/assets/test-media/tvshows/.
var TestTVShows = []struct {
	TvdbID int
	Title  string
}{
	{81189, "Breaking Bad"},
	{71256, "The Daily Show"},
	{121361, "Game of Thrones"},
}

// GetSonarrAPIKeyFromContainer reads the Sonarr API key from config.xml inside the Docker container
func GetSonarrAPIKeyFromContainer(t *testing.T, containerName string) (string, error) {
	t.Helper()

	cmd := exec.Command("docker", "exec", containerName, "cat", "/config/config.xml")
	configXML, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to read Sonarr config.xml: %w", err)
	}

	var config SonarrConfig
	if err := xml.Unmarshal(configXML, &config); err != nil {
		return "", fmt.Errorf("failed to parse Sonarr config XML: %w", err)
	}

	if config.ApiKey == "" {
		return "", fmt.Errorf("Sonarr API key is empty after extraction")
	}

	return config.ApiKey, nil
}

// SetupSonarrForTest initialises Sonarr with the test TV shows.
// Returns the number of series added and any error.
func SetupSonarrForTest(t *testing.T, sonarrURL, apiKey string) (int, error) {
	t.Helper()

	client := &http.Client{Timeout: 30 * time.Second}

	// Get quality profile ID
	qualityProfileID, err := getSonarrQualityProfile(client, sonarrURL, apiKey)
	if err != nil {
		return 0, fmt.Errorf("failed to get Sonarr quality profile: %w", err)
	}
	t.Logf("Sonarr Quality Profile ID: %d", qualityProfileID)

	// Ensure root folder exists
	rootFolder, err := ensureSonarrRootFolder(client, sonarrURL, apiKey, "/media/tvshows")
	if err != nil {
		return 0, fmt.Errorf("failed to ensure Sonarr root folder: %w", err)
	}
	t.Logf("Sonarr Root Folder: %s", rootFolder)

	// Get existing series to avoid duplicates
	existingSeries, err := getSonarrSeries(client, sonarrURL, apiKey)
	if err != nil {
		return 0, fmt.Errorf("failed to get existing Sonarr series: %w", err)
	}
	existingTvdbIDs := make(map[int]bool)
	for _, s := range existingSeries {
		existingTvdbIDs[s.TvdbID] = true
	}
	t.Logf("Sonarr has %d existing series", len(existingSeries))

	// Add test TV shows
	addedCount := 0
	for _, show := range TestTVShows {
		if existingTvdbIDs[show.TvdbID] {
			t.Logf("Series already exists: %s (TVDB ID: %d), skipping...", show.Title, show.TvdbID)
			continue
		}

		t.Logf("Adding series: %s (TVDB ID: %d)", show.Title, show.TvdbID)

		seriesInfo, err := lookupSonarrSeriesByTvdb(client, sonarrURL, apiKey, show.TvdbID)
		if err != nil {
			t.Logf("Warning: Failed to lookup series %s: %v", show.Title, err)
			continue
		}

		seriesInfo.QualityProfileID = qualityProfileID
		seriesInfo.RootFolderPath = rootFolder
		seriesInfo.Monitored = true
		seriesInfo.SeasonFolder = true
		seriesInfo.SeriesType = "standard"
		seriesInfo.AddOptions = &SonarrAddOptions{SearchForMissingEpisodes: false}

		// Monitor all seasons
		for i := range seriesInfo.Seasons {
			seriesInfo.Seasons[i].Monitored = true
		}

		if err := addSonarrSeries(client, sonarrURL, apiKey, seriesInfo); err != nil {
			t.Logf("Warning: Failed to add series %s: %v", show.Title, err)
			continue
		}

		t.Logf("  Added successfully")
		addedCount++
	}

	finalSeries, err := getSonarrSeries(client, sonarrURL, apiKey)
	if err != nil {
		return addedCount, fmt.Errorf("failed to get final series count: %w", err)
	}

	t.Logf("Total series in Sonarr: %d (added %d new)", len(finalSeries), addedCount)
	return len(finalSeries), nil
}

// EnsureSonarrSeriesExist ensures Sonarr is set up with test TV shows and
// triggers a disk scan, waiting for episode files to be detected.
func EnsureSonarrSeriesExist(t *testing.T, sonarrURL, apiKey string) error {
	t.Helper()

	seriesCount, err := SetupSonarrForTest(t, sonarrURL, apiKey)
	if err != nil {
		return fmt.Errorf("Sonarr setup failed: %w", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}

	// Trigger bulk disk scan for the tvshows root folder
	t.Logf("Triggering DownloadedEpisodesScan for /media/tvshows...")
	if err := triggerSonarrDiskScan(client, sonarrURL, apiKey, "/media/tvshows"); err != nil {
		return fmt.Errorf("failed to trigger Sonarr disk scan: %w", err)
	}
	t.Logf("Sonarr disk scan command sent")

	// Poll until all series have at least one episode file detected
	t.Logf("Waiting for Sonarr to complete disk scan (%d series)...", seriesCount)
	maxPolls := 60 // 60 seconds max
	pollInterval := 1 * time.Second

	var seriesWithFiles int
	for i := 0; i < maxPolls; i++ {
		time.Sleep(pollInterval)

		allSeries, err := getSonarrSeries(client, sonarrURL, apiKey)
		if err != nil {
			return fmt.Errorf("failed to get series after scan: %w", err)
		}

		seriesWithFiles = 0
		for _, s := range allSeries {
			if s.Statistics != nil && s.Statistics.EpisodeFileCount > 0 {
				seriesWithFiles++
			}
		}

		t.Logf("Poll %d/%d: %d/%d series have episode files", i+1, maxPolls, seriesWithFiles, len(allSeries))

		if seriesWithFiles == len(allSeries) && len(allSeries) > 0 {
			t.Logf("All series scanned successfully!")
			break
		}
	}

	if seriesWithFiles == 0 {
		return fmt.Errorf("no series have episode files after %d seconds — Sonarr may need more time to scan", maxPolls)
	}

	t.Logf("Sonarr scan complete: %d/%d series ready", seriesWithFiles, seriesCount)
	return nil
}

// GetSonarrSeriesCount queries Sonarr and returns the total series count
func GetSonarrSeriesCount(t *testing.T, sonarrURL, apiKey string) (int, error) {
	t.Helper()

	client := &http.Client{Timeout: 10 * time.Second}
	series, err := getSonarrSeries(client, sonarrURL, apiKey)
	if err != nil {
		return 0, err
	}
	return len(series), nil
}

// ── internal helpers ──────────────────────────────────────────────────────────

func getSonarrQualityProfile(client *http.Client, sonarrURL, apiKey string) (int, error) {
	req, err := http.NewRequest(http.MethodGet, sonarrURL+"/api/v3/qualityprofile", nil)
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
		return 0, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var profiles []SonarrQualityProfile
	if err := json.NewDecoder(resp.Body).Decode(&profiles); err != nil {
		return 0, err
	}
	if len(profiles) == 0 {
		return 0, fmt.Errorf("no quality profiles found in Sonarr")
	}
	return profiles[0].ID, nil
}

func ensureSonarrRootFolder(client *http.Client, sonarrURL, apiKey, path string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, sonarrURL+"/api/v3/rootfolder", nil)
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
		return "", fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var folders []SonarrRootFolder
	if err := json.NewDecoder(resp.Body).Decode(&folders); err != nil {
		return "", err
	}

	for _, f := range folders {
		if f.Path == path {
			return f.Path, nil
		}
	}

	// Create the root folder
	createBody := map[string]interface{}{"path": path}
	bodyBytes, err := json.Marshal(createBody)
	if err != nil {
		return "", err
	}

	req, err = http.NewRequest(http.MethodPost, sonarrURL+"/api/v3/rootfolder", bytes.NewReader(bodyBytes))
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
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to create root folder (status %d): %s", resp.StatusCode, string(body))
	}

	var newFolder SonarrRootFolder
	if err := json.NewDecoder(resp.Body).Decode(&newFolder); err != nil {
		return "", fmt.Errorf("failed to decode new root folder: %w", err)
	}
	return newFolder.Path, nil
}

func getSonarrSeries(client *http.Client, sonarrURL, apiKey string) ([]SonarrSeries, error) {
	req, err := http.NewRequest(http.MethodGet, sonarrURL+"/api/v3/series", nil)
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
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var series []SonarrSeries
	if err := json.NewDecoder(resp.Body).Decode(&series); err != nil {
		return nil, err
	}
	return series, nil
}

func lookupSonarrSeriesByTvdb(client *http.Client, sonarrURL, apiKey string, tvdbID int) (*SonarrSeries, error) {
	url := fmt.Sprintf("%s/api/v3/series/lookup?term=tvdb:%d", sonarrURL, tvdbID)
	req, err := http.NewRequest(http.MethodGet, url, nil)
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
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var results []SonarrSeries
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("no series found for TVDB ID %d", tvdbID)
	}
	return &results[0], nil
}

func addSonarrSeries(client *http.Client, sonarrURL, apiKey string, series *SonarrSeries) error {
	body, err := json.Marshal(series)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, sonarrURL+"/api/v3/series", bytes.NewReader(body))
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
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

func triggerSonarrDiskScan(client *http.Client, sonarrURL, apiKey, folderPath string) error {
	command := map[string]interface{}{
		"name": "DownloadedEpisodesScan",
		"path": folderPath,
	}

	body, err := json.Marshal(command)
	if err != nil {
		return fmt.Errorf("marshaling command: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, sonarrURL+"/api/v3/command", bytes.NewReader(body))
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
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// GetSonarrSeriesByTitle finds a series in Sonarr by title
func GetSonarrSeriesByTitle(t *testing.T, sonarrURL, apiKey, title string) *SonarrSeries {
	t.Helper()
	t.Logf("Searching for Sonarr series: %s", title)

	client := &http.Client{Timeout: 10 * time.Second}
	series, err := getSonarrSeries(client, sonarrURL, apiKey)
	require.NoError(t, err)

	for i, s := range series {
		if s.Title == title {
			t.Logf("Found Sonarr series ID %d: %s", s.ID, s.Title)
			return &series[i]
		}
	}

	require.Failf(t, "Series not found in Sonarr", "Title: %s", title)
	return nil
}
