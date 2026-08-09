package clients

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ramonskie/oxicleanarr/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPluginResponseCasing decodes live-verified PascalCase plugin responses
// to lock the wire contract. Confirmed against plugin v3.2.4:
// GET /api/oxicleanarr/status -> {"Status":"ok"}.
func TestPluginResponseCasing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/oxicleanarr/status":
			w.Write([]byte(`{"Status":"ok"}`))
		case "/api/oxicleanarr/symlinks/add":
			w.Write([]byte(`{"Success":true,"CreatedSymlinks":["/leaving/movie.mkv"],"Errors":[]}`))
		case "/api/oxicleanarr/symlinks/remove":
			w.Write([]byte(`{"Success":true,"RemovedSymlinks":["/leaving/movie.mkv"],"Errors":[]}`))
		case "/api/oxicleanarr/symlinks/list":
			w.Write([]byte(`{"Symlinks":[{"Path":"/leaving/movie.mkv","Target":"/media/movie.mkv","Name":"movie.mkv"}],"Count":1,"SymlinkNames":["movie.mkv"],"Message":"Found 1 symlink(s)"}`))
		case "/api/oxicleanarr/directories/create":
			w.Write([]byte(`{"Success":true,"Directory":"/leaving","Created":true,"Message":"Directory created successfully"}`))
		case "/api/oxicleanarr/directories/remove":
			w.Write([]byte(`{"Success":true,"Directory":"/leaving","Message":"Directory removed successfully"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewJellyfinClient(config.JellyfinConfig{
		BaseIntegrationConfig: config.BaseIntegrationConfig{
			URL:    server.URL,
			APIKey: "test-key",
		},
	})
	ctx := context.Background()

	t.Run("status", func(t *testing.T) {
		resp, err := client.CheckPluginStatus(ctx)
		require.NoError(t, err)
		assert.Equal(t, "ok", resp.Status, "PluginStatusResponse.Status must decode PascalCase 'Status'")
	})

	t.Run("add symlinks", func(t *testing.T) {
		resp, err := client.AddSymlinks(ctx, []PluginSymlinkItem{
			{SourcePath: "/media/movie.mkv", TargetDirectory: "/leaving"},
		}, false)
		require.NoError(t, err)
		assert.True(t, resp.Success)
		assert.Equal(t, []string{"/leaving/movie.mkv"}, resp.CreatedSymlinks)
	})

	t.Run("remove symlinks", func(t *testing.T) {
		resp, err := client.RemoveSymlinks(ctx, []string{"/leaving/movie.mkv"}, false)
		require.NoError(t, err)
		assert.True(t, resp.Success)
		assert.Equal(t, []string{"/leaving/movie.mkv"}, resp.RemovedSymlinks)
	})

	t.Run("list symlinks", func(t *testing.T) {
		resp, err := client.ListSymlinks(ctx, "/leaving")
		require.NoError(t, err)
		require.Len(t, resp.Symlinks, 1)
		assert.Equal(t, "/leaving/movie.mkv", resp.Symlinks[0].Path)
		assert.Equal(t, "/media/movie.mkv", resp.Symlinks[0].Target)
		assert.Equal(t, "movie.mkv", resp.Symlinks[0].Name)
		assert.Equal(t, 1, resp.Count)
		assert.Equal(t, []string{"movie.mkv"}, resp.SymlinkNames)
	})

	t.Run("create directory", func(t *testing.T) {
		resp, err := client.CreateDirectory(ctx, "/leaving", false)
		require.NoError(t, err)
		assert.True(t, resp.Success)
		assert.True(t, resp.Created)
		assert.Equal(t, "/leaving", resp.Directory)
	})

	t.Run("delete directory", func(t *testing.T) {
		resp, err := client.DeleteDirectory(ctx, "/leaving", false, false)
		require.NoError(t, err)
		assert.True(t, resp.Success)
		assert.Equal(t, "/leaving", resp.Directory)
	})
}

func TestPluginResponsesSerializePascalCase(t *testing.T) {
	// Round-trip check: marshaled plugin responses use PascalCase keys.
	payload, err := json.Marshal(PluginStatusResponse{Status: "ok"})
	require.NoError(t, err)
	assert.JSONEq(t, `{"Status":"ok"}`, string(payload))
}

func TestAddPathToVirtualFolder_SendsBody(t *testing.T) {
	var receivedBody string
	var receivedName string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 1024)
		n, _ := r.Body.Read(buf)
		receivedBody = string(buf[:n])
		receivedName = r.URL.Query().Get("name")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewJellyfinClient(config.JellyfinConfig{
		BaseIntegrationConfig: config.BaseIntegrationConfig{
			URL:    server.URL,
			APIKey: "test-key",
		},
	})

	err := client.AddPathToVirtualFolder(context.Background(), "Leaving Soon", "/media/movies", false)
	require.NoError(t, err)
	assert.JSONEq(t, `{"Path":"/media/movies"}`, receivedBody, "POST body must contain the path")
	assert.Equal(t, "Leaving Soon", receivedName)
}
