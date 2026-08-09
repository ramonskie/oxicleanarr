package handlers

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSPAHandler_PathTraversalRejected(t *testing.T) {
	// Build a fake dist directory
	distPath := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(distPath, "index.html"), []byte("<html>index</html>"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(distPath, "app.js"), []byte("console.log(1)"), 0644))

	handler, err := NewSPAHandler(distPath)
	require.NoError(t, err)

	// Create a secret file outside dist to prove traversal is blocked
	secretFile := filepath.Join(filepath.Dir(distPath), "secret.txt")
	require.NoError(t, os.WriteFile(secretFile, []byte("topsecret"), 0644))

	tests := []struct {
		name   string
		target string
		want   int
	}{
		{
			name:   "normal asset served",
			target: "/app.js",
			want:   http.StatusOK,
		},
		{
			name:   "root serves index",
			target: "/",
			want:   http.StatusOK,
		},
		{
			name:   "dotdot traversal",
			target: "/../../secret.txt",
			want:   http.StatusNotFound,
		},
		{
			name:   "encoded traversal",
			target: "/%2e%2e/%2e%2e/secret.txt",
			want:   http.StatusNotFound,
		},
		{
			name:   "deep traversal",
			target: "/../../../etc/passwd",
			want:   http.StatusNotFound,
		},
		{
			name:   "dotdot past real path segment",
			target: "/app/../../../../secret.txt",
			want:   http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.target, nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			assert.Equal(t, tt.want, rec.Code, "unexpected status for %q", tt.target)
			// Security invariant: never serve content outside dist, never leak the secret.
			assert.NotContains(t, rec.Body.String(), "topsecret", "must not leak secret file contents")
		})
	}
}
