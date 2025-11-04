package handlers

import (
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog/log"
)

// SPAHandler serves the Single Page Application frontend
type SPAHandler struct {
	staticFS   http.FileSystem
	indexPath  string
	staticPath string
}

// NewSPAHandler creates a new SPA handler
// distPath should be the path to the web/dist directory (e.g., "./web/dist")
func NewSPAHandler(distPath string) (*SPAHandler, error) {
	// Check if dist directory exists
	if _, err := os.Stat(distPath); os.IsNotExist(err) {
		log.Warn().
			Str("path", distPath).
			Msg("Frontend dist directory not found, SPA handler disabled")
		return nil, err
	}

	indexPath := filepath.Join(distPath, "index.html")
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		log.Warn().
			Str("path", indexPath).
			Msg("Frontend index.html not found, SPA handler disabled")
		return nil, err
	}

	log.Info().
		Str("dist_path", distPath).
		Msg("SPA handler initialized successfully")

	return &SPAHandler{
		staticFS:   http.Dir(distPath),
		indexPath:  indexPath,
		staticPath: distPath,
	}, nil
}

// ServeHTTP handles HTTP requests for the SPA
func (h *SPAHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Clean the path to prevent directory traversal attacks
	path := filepath.Clean(r.URL.Path)

	// Remove leading slash for file system operations
	if len(path) > 0 && path[0] == '/' {
		path = path[1:]
	}

	// If empty path, serve index.html
	if path == "" {
		path = "index.html"
	}

	// Full path to the requested file
	fullPath := filepath.Join(h.staticPath, path)

	// Check if file exists
	info, err := os.Stat(fullPath)
	if err != nil {
		// File doesn't exist, serve index.html for client-side routing
		if os.IsNotExist(err) {
			log.Debug().
				Str("path", r.URL.Path).
				Msg("SPA: File not found, serving index.html for client-side routing")
			http.ServeFile(w, r, h.indexPath)
			return
		}
		// Other error, return 500
		log.Error().Err(err).Str("path", fullPath).Msg("SPA: Error checking file")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// If it's a directory, serve index.html
	if info.IsDir() {
		log.Debug().
			Str("path", r.URL.Path).
			Msg("SPA: Directory requested, serving index.html")
		http.ServeFile(w, r, h.indexPath)
		return
	}

	// File exists, serve it with appropriate content type
	log.Debug().
		Str("path", r.URL.Path).
		Str("file", fullPath).
		Msg("SPA: Serving static file")

	// Set content type based on file extension
	contentType := getContentType(fullPath)
	if contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}

	http.ServeFile(w, r, fullPath)
}

// getContentType returns the content type for common file extensions
func getContentType(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".html":
		return "text/html; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".js":
		return "application/javascript; charset=utf-8"
	case ".json":
		return "application/json; charset=utf-8"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".svg":
		return "image/svg+xml"
	case ".ico":
		return "image/x-icon"
	case ".woff":
		return "font/woff"
	case ".woff2":
		return "font/woff2"
	case ".ttf":
		return "font/ttf"
	case ".eot":
		return "application/vnd.ms-fontobject"
	default:
		return ""
	}
}

// NewSPAHandlerFromFS creates a new SPA handler from an embedded filesystem
// This is useful for embedding the frontend in the binary
func NewSPAHandlerFromFS(fsys fs.FS) (*SPAHandler, error) {
	// Check if index.html exists
	if _, err := fs.Stat(fsys, "index.html"); err != nil {
		log.Warn().Msg("Frontend index.html not found in embedded FS, SPA handler disabled")
		return nil, err
	}

	log.Info().Msg("SPA handler initialized from embedded filesystem")

	return &SPAHandler{
		staticFS: http.FS(fsys),
		// For embedded FS, we'll handle index serving differently
	}, nil
}
