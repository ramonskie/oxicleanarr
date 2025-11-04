package clients

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/ramonskie/prunarr/internal/config"
	"github.com/rs/zerolog/log"
)

// SonarrClient handles communication with Sonarr API
type SonarrClient struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

// NewSonarrClient creates a new Sonarr client
func NewSonarrClient(cfg config.SonarrConfig) *SonarrClient {
	timeout := 30 * time.Second
	if cfg.Timeout != "" {
		if d, err := time.ParseDuration(cfg.Timeout); err == nil {
			timeout = d
		}
	}

	return &SonarrClient{
		baseURL: cfg.URL,
		apiKey:  cfg.APIKey,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

// GetSeries fetches all TV series from Sonarr
func (c *SonarrClient) GetSeries(ctx context.Context) ([]SonarrSeries, error) {
	url := fmt.Sprintf("%s/api/v3/series", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("X-Api-Key", c.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var series []SonarrSeries
	if err := json.NewDecoder(resp.Body).Decode(&series); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	log.Debug().Int("count", len(series)).Msg("Fetched series from Sonarr")
	return series, nil
}

// GetSeriesByID fetches a single series by ID
func (c *SonarrClient) GetSeriesByID(ctx context.Context, id int) (*SonarrSeries, error) {
	url := fmt.Sprintf("%s/api/v3/series/%d", c.baseURL, id)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("X-Api-Key", c.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var series SonarrSeries
	if err := json.NewDecoder(resp.Body).Decode(&series); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return &series, nil
}

// DeleteSeries deletes a series from Sonarr and optionally removes files
func (c *SonarrClient) DeleteSeries(ctx context.Context, id int, deleteFiles bool) error {
	url := fmt.Sprintf("%s/api/v3/series/%d?deleteFiles=%t", c.baseURL, id, deleteFiles)

	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("X-Api-Key", c.apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	log.Info().Int("series_id", id).Bool("delete_files", deleteFiles).Msg("Deleted series from Sonarr")
	return nil
}

// GetEpisodes fetches episodes for a series
func (c *SonarrClient) GetEpisodes(ctx context.Context, seriesID int) ([]SonarrEpisode, error) {
	url := fmt.Sprintf("%s/api/v3/episode?seriesId=%d", c.baseURL, seriesID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("X-Api-Key", c.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var episodes []SonarrEpisode
	if err := json.NewDecoder(resp.Body).Decode(&episodes); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return episodes, nil
}

// GetTags fetches all tags from Sonarr
func (c *SonarrClient) GetTags(ctx context.Context) ([]SonarrTag, error) {
	url := fmt.Sprintf("%s/api/v3/tag", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("X-Api-Key", c.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var tags []SonarrTag
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	log.Debug().Int("count", len(tags)).Msg("Fetched tags from Sonarr")
	return tags, nil
}

// Ping checks if Sonarr is reachable
func (c *SonarrClient) Ping(ctx context.Context) error {
	url := fmt.Sprintf("%s/api/v3/system/status", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("X-Api-Key", c.apiKey)

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
