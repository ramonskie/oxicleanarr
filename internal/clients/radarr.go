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

// RadarrClient handles communication with Radarr API
type RadarrClient struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

// NewRadarrClient creates a new Radarr client
func NewRadarrClient(cfg config.RadarrConfig) *RadarrClient {
	timeout := 30 * time.Second
	if cfg.Timeout != "" {
		if d, err := time.ParseDuration(cfg.Timeout); err == nil {
			timeout = d
		}
	}

	return &RadarrClient{
		baseURL: cfg.URL,
		apiKey:  cfg.APIKey,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

// GetMovies fetches all movies from Radarr
func (c *RadarrClient) GetMovies(ctx context.Context) ([]RadarrMovie, error) {
	url := fmt.Sprintf("%s/api/v3/movie", c.baseURL)

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

	var movies []RadarrMovie
	if err := json.NewDecoder(resp.Body).Decode(&movies); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	log.Debug().Int("count", len(movies)).Msg("Fetched movies from Radarr")
	return movies, nil
}

// GetMovie fetches a single movie by ID
func (c *RadarrClient) GetMovie(ctx context.Context, id int) (*RadarrMovie, error) {
	url := fmt.Sprintf("%s/api/v3/movie/%d", c.baseURL, id)

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

	var movie RadarrMovie
	if err := json.NewDecoder(resp.Body).Decode(&movie); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return &movie, nil
}

// DeleteMovie deletes a movie from Radarr and optionally removes files
func (c *RadarrClient) DeleteMovie(ctx context.Context, id int, deleteFiles bool) error {
	url := fmt.Sprintf("%s/api/v3/movie/%d?deleteFiles=%t", c.baseURL, id, deleteFiles)

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

	log.Info().Int("movie_id", id).Bool("delete_files", deleteFiles).Msg("Deleted movie from Radarr")
	return nil
}

// GetHistory fetches history for a movie
func (c *RadarrClient) GetHistory(ctx context.Context, movieID int) ([]RadarrHistory, error) {
	url := fmt.Sprintf("%s/api/v3/history/movie?movieId=%d", c.baseURL, movieID)

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

	var history []RadarrHistory
	if err := json.NewDecoder(resp.Body).Decode(&history); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return history, nil
}

// Ping checks if Radarr is reachable
func (c *RadarrClient) Ping(ctx context.Context) error {
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
