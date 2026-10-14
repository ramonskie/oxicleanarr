package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/ramonskie/oxicleanarr/internal/services/overlay"
)

// OverlayHandler serves the deletion-overlay (poster banner) endpoints.
type OverlayHandler struct {
	svc *overlay.Service
}

// NewOverlayHandler creates a new OverlayHandler.
func NewOverlayHandler(svc *overlay.Service) *OverlayHandler {
	return &OverlayHandler{svc: svc}
}

// Run handles POST /api/overlay/run - executes one overlay pass. The optional
// force query parameter (?force=true|1) re-uploads every scheduled banner even
// when it already matches the server, so an externally reverted or overwritten
// overlay is redrawn; without it the pass keeps its normal verify/skip behavior.
func (h *OverlayHandler) Run(w http.ResponseWriter, r *http.Request) {
	res, err := h.svc.RunForce(r.Context(), forceRequested(r))
	if err != nil {
		writeOverlayError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(res)
}

// forceRequested reports whether the request asked for a forced overlay pass.
// Parsing is intentionally lenient: "true" and "1" (case-insensitive, trimmed)
// force the pass; an absent, empty, "false", or unrecognized value uses the
// normal pass.
func forceRequested(r *http.Request) bool {
	switch strings.ToLower(strings.TrimSpace(r.URL.Query().Get("force"))) {
	case "true", "1":
		return true
	default:
		return false
	}
}

// Reset handles POST /api/overlay/reset - restores every original poster.
func (h *OverlayHandler) Reset(w http.ResponseWriter, r *http.Request) {
	res, err := h.svc.Reset(r.Context())
	if err != nil {
		writeOverlayError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(res)
}

// Status handles GET /api/overlay/status.
func (h *OverlayHandler) Status(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(h.svc.Status())
}

// writeOverlayError maps overlay service errors onto HTTP status codes.
func writeOverlayError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, overlay.ErrAlreadyRunning):
		http.Error(w, "Overlay pass is already running", http.StatusConflict)
	case errors.Is(err, overlay.ErrOverlayDisabled):
		http.Error(w, "Overlay feature is disabled", http.StatusBadRequest)
	case errors.Is(err, overlay.ErrJellyfinUnavailable):
		http.Error(w, "Jellyfin integration is not enabled", http.StatusBadRequest)
	default:
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
