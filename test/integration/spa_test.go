package integration

import (
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testSPATraversalBlocked verifies that path traversal attempts against the SPA
// handler are rejected instead of reaching the file system (security fix).
// Percent-encoded paths are used so the HTTP client does not normalize the
// "../" segments away before the request is sent.
func testSPATraversalBlocked(t *testing.T) {
	paths := []string{
		"/..%2f..%2fetc%2fpasswd",
		"/.%2e/.%2e/etc/passwd",
		"/..%2f..%2f..%2fetc%2fpasswd",
	}

	for _, p := range paths {
		p := p
		t.Run(p, func(t *testing.T) {
			resp, err := http.Get(OxiCleanarrURL + p)
			require.NoError(t, err)
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			assert.NotEqual(t, http.StatusOK, resp.StatusCode,
				"traversal path must not be served with a 200")
			assert.NotContains(t, string(body), "root:",
				"traversal must not expose /etc/passwd content")
		})
	}
}
