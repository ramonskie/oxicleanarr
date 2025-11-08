package main

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog/log"
)

// SPAHandler returns a handler that serves the Single Page Application
// In production (when web/dist exists), serves static files
// Falls back to index.html for client-side routing
func SPAHandler() http.Handler {
	// Check if dist directory exists
	distPath := filepath.Join("web", "dist")
	if _, err := os.Stat(distPath); os.IsNotExist(err) {
		log.Info().Msg("Frontend dist directory not found, serving API only")
		return http.NotFoundHandler()
	}

	log.Info().Str("path", distPath).Msg("Serving frontend from filesystem")
	fileServer := http.FileServer(http.Dir(distPath))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Don't serve static files for API routes
		if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/health" {
			http.NotFound(w, r)
			return
		}

		// Check if the file exists
		path := filepath.Join(distPath, r.URL.Path)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			// File doesn't exist, serve index.html for SPA routing
			http.ServeFile(w, r, filepath.Join(distPath, "index.html"))
			return
		}

		fileServer.ServeHTTP(w, r)
	})
}
