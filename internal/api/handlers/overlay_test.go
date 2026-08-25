package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/ramonskie/oxicleanarr/internal/config"
	"github.com/ramonskie/oxicleanarr/internal/models"
	"github.com/ramonskie/oxicleanarr/internal/services/overlay"
	"github.com/ramonskie/oxicleanarr/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// overlayTestClient implements overlay.ImageClient for handler tests.
type overlayTestClient struct {
	mu      sync.Mutex
	images  map[string][]byte
	uploads int
}

func (c *overlayTestClient) GetItemImage(_ context.Context, itemID, _ string) ([]byte, string, error) {
	img, ok := c.images[itemID]
	if !ok {
		return nil, "", fmt.Errorf("image not found for %s", itemID)
	}
	return img, "image/jpeg", nil
}

func (c *overlayTestClient) SetItemImage(_ context.Context, _, _ string, _ []byte, _ string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.uploads++
	return nil
}

// overlayTestProvider implements overlay.MediaProvider for handler tests.
type overlayTestProvider struct {
	media []models.Media
}

func (p *overlayTestProvider) GetMediaList() []models.Media {
	return append([]models.Media(nil), p.media...)
}

func newTestOverlayHandler(t *testing.T, enabled bool, media []models.Media) (*OverlayHandler, *overlayTestClient) {
	t.Helper()

	cfg := &config.Config{
		Overlay: config.OverlayConfig{
			Enabled:             enabled,
			IntervalHours:       24,
			TextTemplate:        "in {days} days",
			FontSizePercent:     5,
			FontColor:           "#ffffff",
			BackgroundColor:     "rgba(0,0,0,0.75)",
			PaddingPercent:      2,
			CornerRadiusPercent: 50,
		},
	}
	pinTestConfig(t, cfg)

	stateFile, err := storage.NewOverlayStateFile(t.TempDir())
	require.NoError(t, err)

	client := &overlayTestClient{images: make(map[string][]byte)}
	provider := &overlayTestProvider{media: media}

	svc := overlay.NewService(cfg, stateFile, client, provider, t.TempDir())
	return NewOverlayHandler(svc), client
}

// pinTestConfig pins the global config for the duration of a test (the overlay
// service reads config.Get() live) and restores the prior value on cleanup.
func pinTestConfig(t *testing.T, cfg *config.Config) {
	t.Helper()
	prev := config.Get()
	config.SetTestConfig(cfg)
	t.Cleanup(func() {
		if prev != nil {
			config.SetTestConfig(prev)
		} else {
			config.SetTestConfig(nil)
		}
	})
}

func overlayTestPoster(t *testing.T) []byte {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, 200, 300))
	for i := range img.Pix {
		img.Pix[i] = 0x2a
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("encode test poster: %v", err)
	}
	return buf.Bytes()
}

func overlayScheduledItem(itemID string, deleteAfter time.Time) models.Media {
	return models.Media{
		ID:          "media-" + itemID,
		Type:        models.MediaTypeMovie,
		Title:       "Movie " + itemID,
		JellyfinID:  itemID,
		DeleteAfter: deleteAfter,
	}
}

func TestOverlayHandler_Status(t *testing.T) {
	h, _ := newTestOverlayHandler(t, true, nil)
	w := httptest.NewRecorder()

	h.Status(w, httptest.NewRequest(http.MethodGet, "/api/overlay/status", nil))

	assert.Equal(t, http.StatusOK, w.Code)
	var status overlay.Status
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &status))
	assert.True(t, status.Enabled)
	assert.False(t, status.Running)
}

func TestOverlayHandler_RunDisabled(t *testing.T) {
	h, _ := newTestOverlayHandler(t, false, nil)
	w := httptest.NewRecorder()

	h.Run(w, httptest.NewRequest(http.MethodPost, "/api/overlay/run", nil))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestOverlayHandler_RunApplies(t *testing.T) {
	h, client := newTestOverlayHandler(t, true, []models.Media{
		overlayScheduledItem("jf-1", time.Now().Add(72*time.Hour)),
	})
	client.images["jf-1"] = overlayTestPoster(t)
	w := httptest.NewRecorder()

	h.Run(w, httptest.NewRequest(http.MethodPost, "/api/overlay/run", nil))

	assert.Equal(t, http.StatusOK, w.Code)
	var res overlay.Result
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &res))
	assert.Equal(t, 1, res.Processed)
	assert.Zero(t, res.Errors)
	assert.Equal(t, 1, client.uploads)
}

func TestOverlayHandler_Reset(t *testing.T) {
	h, _ := newTestOverlayHandler(t, true, nil)
	w := httptest.NewRecorder()

	h.Reset(w, httptest.NewRequest(http.MethodPost, "/api/overlay/reset", nil))

	assert.Equal(t, http.StatusOK, w.Code)
	var res overlay.Result
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &res))
	assert.Zero(t, res.Reverted)
}

func TestOverlayHandler_ResetWhenJellyfinMissing(t *testing.T) {
	cfg := &config.Config{Overlay: config.OverlayConfig{Enabled: true}}
	stateFile, err := storage.NewOverlayStateFile(t.TempDir())
	require.NoError(t, err)

	svc := overlay.NewService(cfg, stateFile, nil, &overlayTestProvider{}, t.TempDir())
	h := NewOverlayHandler(svc)

	w := httptest.NewRecorder()
	h.Reset(w, httptest.NewRequest(http.MethodPost, "/api/overlay/reset", nil))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
