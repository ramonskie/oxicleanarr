package clients

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ramonskie/oxicleanarr/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestJellyfinIntegration runs integration tests against a real Jellyfin instance
// Set OXICLEANARR_INTEGRATION_TEST=1 to enable these tests
func TestJellyfinIntegration(t *testing.T) {
	if os.Getenv("OXICLEANARR_INTEGRATION_TEST") != "1" {
		t.Skip("Skipping integration test. Set OXICLEANARR_INTEGRATION_TEST=1 to run.")
	}

	// Load test configuration
	cfg, err := config.Load("../../config/prunarr.test.yaml")
	require.NoError(t, err, "Failed to load test config")
	require.True(t, cfg.Integrations.Jellyfin.Enabled, "Jellyfin must be enabled in test config")

	client := NewJellyfinClient(cfg.Integrations.Jellyfin)
	ctx := context.Background()

	t.Run("Ping", func(t *testing.T) {
		err := client.Ping(ctx)
		assert.NoError(t, err, "Should be able to ping Jellyfin")
	})

	t.Run("GetMovies", func(t *testing.T) {
		movies, err := client.GetMovies(ctx)
		require.NoError(t, err, "Should be able to fetch movies")

		t.Logf("Found %d movies in Jellyfin", len(movies))

		if len(movies) == 0 {
			t.Log("Warning: No movies found in Jellyfin")
			return
		}

		// Validate first movie structure
		movie := movies[0]
		assert.NotEmpty(t, movie.ID, "Movie should have an ID")
		assert.NotEmpty(t, movie.Name, "Movie should have a name")
		assert.Equal(t, "Movie", movie.Type, "Type should be Movie")
		assert.False(t, movie.DateCreated.IsZero(), "Movie should have a created date")

		t.Logf("Sample movie: %s (%d)", movie.Name, movie.ProductionYear)
		t.Logf("  ID: %s", movie.ID)
		t.Logf("  Created: %s", movie.DateCreated.Format(time.RFC3339))
		t.Logf("  Path: %s", movie.Path)

		// Check user data
		if movie.UserData.PlayCount > 0 {
			t.Logf("  Play count: %d", movie.UserData.PlayCount)
			t.Logf("  Last played: %s", movie.UserData.LastPlayedDate.Format(time.RFC3339))
		}

		// Check provider IDs
		if len(movie.ProviderIds) > 0 {
			t.Logf("  Provider IDs: %v", movie.ProviderIds)
		}
	})

	t.Run("GetTVShows", func(t *testing.T) {
		shows, err := client.GetTVShows(ctx)
		require.NoError(t, err, "Should be able to fetch TV shows")

		t.Logf("Found %d TV shows in Jellyfin", len(shows))

		if len(shows) == 0 {
			t.Log("Warning: No TV shows found in Jellyfin")
			return
		}

		// Validate first show structure
		show := shows[0]
		assert.NotEmpty(t, show.ID, "Show should have an ID")
		assert.NotEmpty(t, show.Name, "Show should have a name")
		assert.Equal(t, "Series", show.Type, "Type should be Series")
		assert.False(t, show.DateCreated.IsZero(), "Show should have a created date")

		t.Logf("Sample TV show: %s (%d)", show.Name, show.ProductionYear)
		t.Logf("  ID: %s", show.ID)
		t.Logf("  Created: %s", show.DateCreated.Format(time.RFC3339))
		t.Logf("  Path: %s", show.Path)

		// Check user data
		if show.UserData.PlayCount > 0 {
			t.Logf("  Play count: %d", show.UserData.PlayCount)
			t.Logf("  Last played: %s", show.UserData.LastPlayedDate.Format(time.RFC3339))
		}
	})

	t.Run("MediaDataValidation", func(t *testing.T) {
		movies, err := client.GetMovies(ctx)
		require.NoError(t, err, "Should be able to fetch movies")

		shows, err := client.GetTVShows(ctx)
		require.NoError(t, err, "Should be able to fetch TV shows")

		t.Logf("Media library statistics:")
		t.Logf("  Total movies: %d", len(movies))
		t.Logf("  Total TV shows: %d", len(shows))
		t.Logf("  Total media items: %d", len(movies)+len(shows))

		// Count watched vs unwatched
		var (
			watchedMovies   int
			unwatchedMovies int
			watchedShows    int
			unwatchedShows  int
		)

		for _, movie := range movies {
			if movie.UserData.Played {
				watchedMovies++
			} else {
				unwatchedMovies++
			}
		}

		for _, show := range shows {
			if show.UserData.Played {
				watchedShows++
			} else {
				unwatchedShows++
			}
		}

		t.Logf("  Watched movies: %d (%.1f%%)",
			watchedMovies,
			float64(watchedMovies)/float64(len(movies))*100)
		t.Logf("  Unwatched movies: %d (%.1f%%)",
			unwatchedMovies,
			float64(unwatchedMovies)/float64(len(movies))*100)

		if len(shows) > 0 {
			t.Logf("  Watched shows: %d (%.1f%%)",
				watchedShows,
				float64(watchedShows)/float64(len(shows))*100)
			t.Logf("  Unwatched shows: %d (%.1f%%)",
				unwatchedShows,
				float64(unwatchedShows)/float64(len(shows))*100)
		}
	})

	t.Run("ProviderIDsValidation", func(t *testing.T) {
		movies, err := client.GetMovies(ctx)
		require.NoError(t, err, "Should be able to fetch movies")

		if len(movies) == 0 {
			t.Skip("No movies available")
		}

		// Count movies with various provider IDs
		var (
			withTmdb int
			withImdb int
			withTvdb int
		)

		for _, movie := range movies {
			if _, ok := movie.ProviderIds["Tmdb"]; ok {
				withTmdb++
			}
			if _, ok := movie.ProviderIds["Imdb"]; ok {
				withImdb++
			}
			if _, ok := movie.ProviderIds["Tvdb"]; ok {
				withTvdb++
			}
		}

		t.Logf("Provider ID coverage:")
		t.Logf("  Movies with TMDB ID: %d (%.1f%%)",
			withTmdb,
			float64(withTmdb)/float64(len(movies))*100)
		t.Logf("  Movies with IMDB ID: %d (%.1f%%)",
			withImdb,
			float64(withImdb)/float64(len(movies))*100)
		t.Logf("  Movies with TVDB ID: %d (%.1f%%)",
			withTvdb,
			float64(withTvdb)/float64(len(movies))*100)
	})

	t.Run("WatchHistoryValidation", func(t *testing.T) {
		movies, err := client.GetMovies(ctx)
		require.NoError(t, err, "Should be able to fetch movies")

		if len(movies) == 0 {
			t.Skip("No movies available")
		}

		// Find movies with watch history
		var recentlyWatched []JellyfinItem
		for _, movie := range movies {
			if movie.UserData.PlayCount > 0 && !movie.UserData.LastPlayedDate.IsZero() {
				recentlyWatched = append(recentlyWatched, movie)
			}
		}

		t.Logf("Watch history:")
		t.Logf("  Movies with watch history: %d", len(recentlyWatched))

		if len(recentlyWatched) > 0 {
			// Show top 5 most watched
			limit := 5
			if len(recentlyWatched) < limit {
				limit = len(recentlyWatched)
			}

			t.Logf("  Sample watched movies (up to %d):", limit)
			for i := 0; i < limit; i++ {
				movie := recentlyWatched[i]
				t.Logf("    - %s: played %d times, last on %s",
					movie.Name,
					movie.UserData.PlayCount,
					movie.UserData.LastPlayedDate.Format("2006-01-02"))
			}
		}
	})

	t.Run("DeleteItem_ReadOnly", func(t *testing.T) {
		// This test should NOT actually delete anything
		// We're just validating the method signature and ensuring dry_run is enforced
		t.Skip("Skipping delete test - read-only mode enforced")

		// If we were to test this (in a controlled environment):
		// err := client.DeleteItem(ctx, someID)
		// This should never be run in integration tests with real data
	})

	t.Run("ConcurrentRequests", func(t *testing.T) {
		// Test multiple concurrent requests
		results := make(chan error, 2)

		go func() {
			_, err := client.GetMovies(ctx)
			results <- err
		}()

		go func() {
			_, err := client.GetTVShows(ctx)
			results <- err
		}()

		// Collect results
		for i := 0; i < 2; i++ {
			err := <-results
			assert.NoError(t, err, "Concurrent request should succeed")
		}
	})
}

// TestJellyfinClient_Unit runs unit tests that don't require a real Jellyfin instance
func TestJellyfinClient_Unit(t *testing.T) {
	t.Run("NewJellyfinClient", func(t *testing.T) {
		cfg := config.JellyfinConfig{
			BaseIntegrationConfig: config.BaseIntegrationConfig{
				URL:     "http://localhost:8096",
				APIKey:  "test-api-key",
				Timeout: "30s",
			},
		}

		client := NewJellyfinClient(cfg)

		assert.NotNil(t, client, "Client should not be nil")
		assert.Equal(t, cfg.URL, client.baseURL, "Base URL should match config")
		assert.Equal(t, cfg.APIKey, client.apiKey, "API key should match config")
		assert.NotNil(t, client.client, "HTTP client should be initialized")
	})

	t.Run("NewJellyfinClient_DefaultTimeout", func(t *testing.T) {
		cfg := config.JellyfinConfig{
			BaseIntegrationConfig: config.BaseIntegrationConfig{
				URL:    "http://localhost:8096",
				APIKey: "test-api-key",
				// No timeout specified
			},
		}

		client := NewJellyfinClient(cfg)

		assert.NotNil(t, client, "Client should not be nil")
		assert.Equal(t, 30*time.Second, client.client.Timeout, "Should use default 30s timeout")
	})

	t.Run("NewJellyfinClient_InvalidTimeout", func(t *testing.T) {
		cfg := config.JellyfinConfig{
			BaseIntegrationConfig: config.BaseIntegrationConfig{
				URL:     "http://localhost:8096",
				APIKey:  "test-api-key",
				Timeout: "invalid",
			},
		}

		client := NewJellyfinClient(cfg)

		// Should fall back to default timeout
		assert.Equal(t, 30*time.Second, client.client.Timeout, "Should use default timeout for invalid value")
	})

	t.Run("NewJellyfinClient_CustomTimeout", func(t *testing.T) {
		cfg := config.JellyfinConfig{
			BaseIntegrationConfig: config.BaseIntegrationConfig{
				URL:     "http://localhost:8096",
				APIKey:  "test-api-key",
				Timeout: "45s",
			},
		}

		client := NewJellyfinClient(cfg)

		assert.Equal(t, 45*time.Second, client.client.Timeout, "Should use custom timeout")
	})
}

// TestJellyfinClientAuthHeader verifies every Jellyfin request authenticates
// with the MediaBrowser Authorization scheme. Jellyfin 12.x ignores the legacy
// X-Emby-Token/X-MediaBrowser-Token headers when EnableLegacyAuthorization is
// off (the 12.x default), which surfaces as HTTP 401.
func TestJellyfinClientAuthHeader(t *testing.T) {
	const apiKey = "test-api-key-123"

	type receivedHeaders struct {
		auth      string
		xEmby     string
		xMediaBro string
	}

	var got receivedHeaders

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = receivedHeaders{
			auth:      r.Header.Get("Authorization"),
			xEmby:     r.Header.Get("X-Emby-Token"),
			xMediaBro: r.Header.Get("X-MediaBrowser-Token"),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"Items":[]}`))
	}))
	defer srv.Close()

	client := NewJellyfinClient(config.JellyfinConfig{
		BaseIntegrationConfig: config.BaseIntegrationConfig{
			URL:    srv.URL,
			APIKey: apiKey,
		},
	})
	ctx := context.Background()

	calls := []struct {
		name string
		call func() error
	}{
		{"Ping", func() error { return client.Ping(ctx) }},
		{"GetMovies", func() error { _, err := client.GetMovies(ctx); return err }},
		{"GetTVShows", func() error { _, err := client.GetTVShows(ctx); return err }},
		{"GetUserData", func() error { _, err := client.GetUserData(ctx, "user-1", "item-1"); return err }},
		{"DeleteItem", func() error { return client.DeleteItem(ctx, "item-1") }},
		{"RefreshLibrary", func() error { return client.RefreshLibrary(ctx, false) }},
	}

	for _, tc := range calls {
		t.Run(tc.name, func(t *testing.T) {
			// Reset captured headers so each subtest proves its own request.
			got = receivedHeaders{}

			require.NoError(t, tc.call())

			assert.Equal(t, `MediaBrowser Token="`+apiKey+`"`, got.auth)
			assert.Empty(t, got.xEmby, "must not send legacy X-Emby-Token")
			assert.Empty(t, got.xMediaBro, "must not send legacy X-MediaBrowser-Token")
		})
	}
}

// TestJellyfinClientGetItemsNoBoxSetCollapse verifies GetMovies/GetTVShows
// request the library with CollapseBoxSetItems=false. Jellyfin defaults this to
// true for movie queries when unset, hiding every movie that belongs to a
// collection/box set; those items then fail to match and show as unmatched.
// The test also pins the rest of the query shape so a dropped field is caught.
func TestJellyfinClientGetItemsNoBoxSetCollapse(t *testing.T) {
	type request struct {
		path  string
		query url.Values
	}
	var requests []request

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, request{path: r.URL.Path, query: r.URL.Query()})
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"Items":[]}`))
	}))
	defer srv.Close()

	client := NewJellyfinClient(config.JellyfinConfig{
		BaseIntegrationConfig: config.BaseIntegrationConfig{
			URL:    srv.URL,
			APIKey: "test-api-key",
		},
	})
	ctx := context.Background()

	_, err := client.GetMovies(ctx)
	require.NoError(t, err)
	_, err = client.GetTVShows(ctx)
	require.NoError(t, err)

	require.Len(t, requests, 2, "expected one request per media type")
	assert.Equal(t, "/Items", requests[0].path)
	assert.Equal(t, "/Items", requests[1].path)

	wantTypes := []string{"Movie", "Series"}
	for i, req := range requests {
		assert.Equal(t, wantTypes[i], req.query.Get("IncludeItemTypes"))
		assert.Equal(t, "true", req.query.Get("Recursive"))
		assert.Equal(t, "false", req.query.Get("CollapseBoxSetItems"),
			"library query must not collapse box-set members: %s", req.query.Encode())
		assert.ElementsMatch(t, []string{"Path", "DateCreated", "ProviderIds"},
			strings.Split(req.query.Get("Fields"), ","))
	}
}

// newTestJellyfinClient builds a client pointed at a test server URL.
func newTestJellyfinClient(url string) *JellyfinClient {
	return NewJellyfinClient(config.JellyfinConfig{
		BaseIntegrationConfig: config.BaseIntegrationConfig{
			URL:    url,
			APIKey: "test-api-key",
		},
	})
}

func TestJellyfinClientRemoteSearchMovie(t *testing.T) {
	var (
		gotMethod string
		gotPath   string
		gotAuth   string
		gotBody   []byte
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[{"Name":"Fullmetal Alchemist","ProviderIds":{"Tvdb":"75579"},"ProductionYear":2003}]`))
	}))
	defer srv.Close()

	client := newTestJellyfinClient(srv.URL)

	results, err := client.RemoteSearchMovie(context.Background(), "Fullmetal Alchemist", 2003, map[string]string{"Tvdb": "75579"})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "Fullmetal Alchemist", results[0].Name)
	assert.Equal(t, "75579", results[0].ProviderIds["Tvdb"])
	assert.Equal(t, 2003, results[0].ProductionYear)

	assert.Equal(t, http.MethodPost, gotMethod)
	assert.Equal(t, "/Items/RemoteSearch/Movie", gotPath)
	assert.Equal(t, `MediaBrowser Token="test-api-key"`, gotAuth)

	var body map[string]any
	require.NoError(t, json.Unmarshal(gotBody, &body))
	searchInfo, ok := body["SearchInfo"].(map[string]any)
	require.True(t, ok, "request must wrap criteria in SearchInfo: %s", string(gotBody))
	assert.Equal(t, "Fullmetal Alchemist", searchInfo["Name"])
	assert.Equal(t, float64(2003), searchInfo["Year"])
	providerIDs, ok := searchInfo["ProviderIds"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "75579", providerIDs["Tvdb"])
}

func TestJellyfinClientRemoteSearchSeries(t *testing.T) {
	var (
		gotPath string
		gotBody []byte
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[{"Name":"Fullmetal Alchemist","ProviderIds":{"Tvdb":"75579"},"ProductionYear":2003},{"Name":"Fullmetal Alchemist: Brotherhood","ProviderIds":{"Tvdb":"85249"},"ProductionYear":2009}]`))
	}))
	defer srv.Close()

	client := newTestJellyfinClient(srv.URL)

	results, err := client.RemoteSearchSeries(context.Background(), "Fullmetal Alchemist", 2003, nil)
	require.NoError(t, err)
	require.Len(t, results, 2)
	assert.Equal(t, "85249", results[1].ProviderIds["Tvdb"])

	assert.Equal(t, "/Items/RemoteSearch/Series", gotPath)

	var body map[string]any
	require.NoError(t, json.Unmarshal(gotBody, &body))
	searchInfo, ok := body["SearchInfo"].(map[string]any)
	require.True(t, ok, "request must wrap criteria in SearchInfo: %s", string(gotBody))
	assert.Equal(t, "Fullmetal Alchemist", searchInfo["Name"])
}

// TestJellyfinClientRemoteSearchCanonicalizesProviderIDs pins the fix for the
// case-sensitive Jellyfin ItemLookupInfo.ProviderIds: the sync service builds
// lowercase keys ({"tmdb": ...}/{"tvdb": ...}), but Jellyfin only honors
// PascalCase keys, so the outbound body must carry "Tmdb"/"Tvdb".
func TestJellyfinClientRemoteSearchCanonicalizesProviderIDs(t *testing.T) {
	cases := []struct {
		name        string
		kind        string
		providerIDs map[string]string
		want        map[string]string
	}{
		{"movie lowercase tmdb", "Movie", map[string]string{"tmdb": "605722"}, map[string]string{"Tmdb": "605722"}},
		{"series lowercase tvdb", "Series", map[string]string{"tvdb": "75579"}, map[string]string{"Tvdb": "75579"}},
		{"lowercase imdb", "Movie", map[string]string{"imdb": "tt1234567"}, map[string]string{"Imdb": "tt1234567"}},
		{"unknown provider first-letter cap", "Movie", map[string]string{"other": "x"}, map[string]string{"Other": "x"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotBody []byte
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotBody, _ = io.ReadAll(r.Body)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`[]`))
			}))
			defer srv.Close()

			client := newTestJellyfinClient(srv.URL)
			ctx := context.Background()

			var err error
			if tc.kind == "Series" {
				_, err = client.RemoteSearchSeries(ctx, "Fullmetal Alchemist", 2003, tc.providerIDs)
			} else {
				_, err = client.RemoteSearchMovie(ctx, "Long Distance", 2024, tc.providerIDs)
			}
			require.NoError(t, err)

			var body struct {
				SearchInfo struct {
					ProviderIds map[string]string `json:"ProviderIds"`
				} `json:"SearchInfo"`
			}
			require.NoError(t, json.Unmarshal(gotBody, &body))
			for wantKey, wantValue := range tc.want {
				assert.Equal(t, wantValue, body.SearchInfo.ProviderIds[wantKey],
					"outbound ProviderIds must use Jellyfin's PascalCase key: %s", string(gotBody))
			}
			for lower := range tc.providerIDs {
				_, present := body.SearchInfo.ProviderIds[lower]
				assert.False(t, present, "lowercase key %q must not be sent: %s", lower, string(gotBody))
			}
		})
	}
}

func TestCanonicalProviderIDKey(t *testing.T) {
	cases := map[string]string{
		"tmdb":  "Tmdb",
		"TMDB":  "Tmdb",
		"Tmdb":  "Tmdb",
		"tvdb":  "Tvdb",
		"imdb":  "Imdb",
		"other": "Other",
		"":      "",
	}
	for in, want := range cases {
		assert.Equal(t, want, canonicalProviderIDKey(in), "canonicalProviderIDKey(%q)", in)
	}
}

func TestJellyfinClientRemoteSearchNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := newTestJellyfinClient(srv.URL)
	ctx := context.Background()

	t.Run("movie", func(t *testing.T) {
		results, err := client.RemoteSearchMovie(ctx, "x", 0, nil)
		require.Error(t, err)
		assert.Nil(t, results)
		assert.Contains(t, err.Error(), "unexpected status code: 500")
	})

	t.Run("series", func(t *testing.T) {
		results, err := client.RemoteSearchSeries(ctx, "x", 0, nil)
		require.Error(t, err)
		assert.Nil(t, results)
		assert.Contains(t, err.Error(), "unexpected status code: 500")
	})
}

func TestJellyfinClientApplyRemoteSearch(t *testing.T) {
	cases := []struct {
		name             string
		replaceAllImages bool
	}{
		{"keep images", false},
		{"replace all images", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var (
				gotPath  string
				gotQuery url.Values
				gotBody  []byte
			)

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				gotQuery = r.URL.Query()
				gotBody, _ = io.ReadAll(r.Body)
				w.WriteHeader(http.StatusNoContent)
			}))
			defer srv.Close()

			client := newTestJellyfinClient(srv.URL)
			result := RemoteSearchResult{
				Name:           "Fullmetal Alchemist",
				ProviderIds:    map[string]string{"Tvdb": "75579"},
				ProductionYear: 2003,
			}

			err := client.ApplyRemoteSearch(context.Background(), "item-123", result, tc.replaceAllImages)
			require.NoError(t, err)

			assert.Equal(t, "/Items/RemoteSearch/Apply/item-123", gotPath)

			wantQuery := "false"
			if tc.replaceAllImages {
				wantQuery = "true"
			}
			assert.Equal(t, wantQuery, gotQuery.Get("replaceAllImages"))

			var sent RemoteSearchResult
			require.NoError(t, json.Unmarshal(gotBody, &sent))
			assert.Equal(t, result, sent)
		})
	}
}

func TestJellyfinClientApplyRemoteSearchNon204(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	client := newTestJellyfinClient(srv.URL)

	err := client.ApplyRemoteSearch(context.Background(), "item-123", RemoteSearchResult{Name: "x"}, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected status code: 400")
}

func TestJellyfinClientGetEpisodes(t *testing.T) {
	t.Run("decodes episodes and requests path/provider fields", func(t *testing.T) {
		var (
			gotPath  string
			gotQuery url.Values
			gotAuth  string
		)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			gotQuery = r.URL.Query()
			gotAuth = r.Header.Get("Authorization")
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"Items":[{"Id":"e1","Name":"Fullmetal Alchemist",` +
				`"SeriesId":"series-1","ParentIndexNumber":1,"IndexNumber":1,` +
				`"Path":"/tv/x/S01E01.mkv","ProviderIds":{"Tvdb":"75579"}}]}`))
		}))
		defer srv.Close()

		client := newTestJellyfinClient(srv.URL)
		episodes, err := client.GetEpisodes(context.Background(), "series-1")
		require.NoError(t, err)
		require.Len(t, episodes, 1)
		assert.Equal(t, "Fullmetal Alchemist", episodes[0].Name)
		assert.Equal(t, 1, episodes[0].ParentIndexNumber)
		assert.Equal(t, 1, episodes[0].IndexNumber)
		assert.Equal(t, "/tv/x/S01E01.mkv", episodes[0].Path)
		assert.Equal(t, "75579", episodes[0].ProviderIds["Tvdb"])

		assert.Equal(t, "/Shows/series-1/Episodes", gotPath)
		assert.Equal(t, "Path,ProviderIds", gotQuery.Get("Fields"))
		assert.Equal(t, `MediaBrowser Token="test-api-key"`, gotAuth)
	})

	t.Run("non-200 is an error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()

		client := newTestJellyfinClient(srv.URL)
		episodes, err := client.GetEpisodes(context.Background(), "series-1")
		require.Error(t, err)
		assert.Nil(t, episodes)
		assert.Contains(t, err.Error(), "unexpected status code: 500")
	})
}
