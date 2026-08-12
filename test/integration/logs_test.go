package integration

import (
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testLogStreaming verifies that GET /api/logs?stream=true emits SSE log events.
func testLogStreaming(t *testing.T) {
	client := NewTestClient(t, OxiCleanarrURL)
	client.Authenticate(AdminUsername, AdminPassword)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		OxiCleanarrURL+"/api/logs?file=backend&stream=true", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+clientToken(client))

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Contains(t, resp.Header.Get("Content-Type"), "text/event-stream")

	// Read until the request context expires (stream stays open), then
	// assert the initial snapshot of log events arrived.
	data, _ := io.ReadAll(resp.Body)
	body := string(data)
	assert.Contains(t, body, "event: log", "SSE stream must emit log events")
	assert.Contains(t, body, `"level"`, "log events must be JSON log lines")
}
