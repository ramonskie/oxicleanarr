package overlay

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/ramonskie/oxicleanarr/internal/clients"
	"github.com/ramonskie/oxicleanarr/internal/config"
	"github.com/ramonskie/oxicleanarr/internal/models"
	"github.com/ramonskie/oxicleanarr/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeImageClient records image calls; GetItemImage serves a fixed poster.
type fakeImageClient struct {
	mu      sync.Mutex
	images  map[string][]byte // itemID -> original poster
	uploads []fakeUpload
	getErr  error
	setErr  error
}

type fakeUpload struct {
	itemID      string
	imageType   string
	data        []byte
	contentType string
}

func (f *fakeImageClient) GetItemImage(_ context.Context, itemID, imageType string) ([]byte, string, error) {
	if f.getErr != nil {
		return nil, "", f.getErr
	}
	img, ok := f.images[itemID]
	if !ok {
		return nil, "", clients.ErrImageNotFound
	}
	return img, "image/jpeg", nil
}

func (f *fakeImageClient) SetItemImage(_ context.Context, itemID, imageType string, data []byte, contentType string) error {
	if f.setErr != nil {
		return f.setErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.uploads = append(f.uploads, fakeUpload{itemID: itemID, imageType: imageType, data: append([]byte(nil), data...), contentType: contentType})
	return nil
}

func (f *fakeImageClient) uploadCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.uploads)
}

// fakeMediaProvider returns a fixed media list.
type fakeMediaProvider struct {
	mu    sync.Mutex
	media []models.Media
}

func (f *fakeMediaProvider) GetMediaList() []models.Media {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]models.Media(nil), f.media...)
}

func (f *fakeMediaProvider) setMedia(media []models.Media) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.media = media
}

func scheduledMedia(itemID string, deleteAfter time.Time) models.Media {
	return models.Media{
		ID:          "media-" + itemID,
		Type:        models.MediaTypeMovie,
		Title:       "Movie " + itemID,
		JellyfinID:  itemID,
		DeleteAfter: deleteAfter,
	}
}

// useTestConfig pins the global config for the duration of a test so the
// service's live-config lookup (config.Get()) sees the test's settings, then
// restores whatever was there before.
func useTestConfig(t *testing.T, cfg *config.Config) {
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

func newTestService(t *testing.T, enabled bool, media []models.Media) (*Service, *fakeImageClient, *fakeMediaProvider, string) {
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
	useTestConfig(t, cfg)

	dir := t.TempDir()
	stateFile, err := storage.NewOverlayStateFile(dir)
	require.NoError(t, err)

	fake := &fakeImageClient{images: make(map[string][]byte)}
	provider := &fakeMediaProvider{}
	provider.setMedia(media)

	svc := NewService(cfg, stateFile, fake, provider, dir)
	return svc, fake, provider, dir
}

func TestScheduledTargets(t *testing.T) {
	now := time.Now()
	media := []models.Media{
		scheduledMedia("a", now.Add(72*time.Hour)),
		func() models.Media { m := scheduledMedia("b", now.Add(24*time.Hour)); m.IsExcluded = true; return m }(),
		func() models.Media { m := scheduledMedia("c", now.Add(24*time.Hour)); m.JellyfinID = ""; return m }(),
		{ID: "d", Type: models.MediaTypeMovie, Title: "No date"}, // DeleteAfter zero
		scheduledMedia("e", now.Add(48*time.Hour)),
	}

	targets := scheduledTargets(media, now)
	require.Len(t, targets, 2)
	assert.Equal(t, "a", targets[0].ItemID) // sorted
	assert.Equal(t, "e", targets[1].ItemID)
	assert.Equal(t, 3, targets[0].DaysLeft)
	assert.Equal(t, 2, targets[1].DaysLeft)
}

func TestDaysLeft(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name        string
		deleteAfter time.Time
		want        int
	}{
		{name: "past due", deleteAfter: now.Add(-time.Hour), want: 0},
		{name: "now", deleteAfter: now, want: 0},
		{name: "later today", deleteAfter: now.Add(time.Hour), want: 1},
		{name: "tomorrow", deleteAfter: now.Add(24 * time.Hour), want: 1},
		{name: "two days", deleteAfter: now.Add(50 * time.Hour), want: 3},
		{name: "three days", deleteAfter: now.Add(72 * time.Hour), want: 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, daysLeft(tt.deleteAfter, now))
		})
	}
}

func TestRun_Disabled(t *testing.T) {
	svc, fake, provider, _ := newTestService(t, false, []models.Media{scheduledMedia("a", time.Now().Add(72*time.Hour))})

	_, err := svc.Run(context.Background())
	assert.ErrorIs(t, err, ErrOverlayDisabled)
	assert.Zero(t, fake.uploadCount())

	// The disabled scheduler must not start either.
	provider.setMedia(nil)
	require.NoError(t, svc.Start())
}

func TestRun_JellyfinUnavailable(t *testing.T) {
	cfg := &config.Config{Overlay: config.OverlayConfig{Enabled: true, TextTemplate: "in {days} days"}}
	useTestConfig(t, cfg)
	stateFile, err := storage.NewOverlayStateFile(t.TempDir())
	require.NoError(t, err)

	svc := NewService(cfg, stateFile, nil, &fakeMediaProvider{}, t.TempDir())
	_, err = svc.Run(context.Background())
	assert.ErrorIs(t, err, ErrJellyfinUnavailable)
}

func TestRun_AppliesOverlays(t *testing.T) {
	svc, fake, _, _ := newTestService(t, true, []models.Media{
		scheduledMedia("a", time.Now().Add(72*time.Hour)),
		scheduledMedia("b", time.Now().Add(24*time.Hour)),
	})
	fake.images["a"] = testPoster(t, 300, 450)
	fake.images["b"] = testPoster(t, 300, 450)

	res, err := svc.Run(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 2, res.Processed)
	assert.Zero(t, res.Reverted)
	assert.Zero(t, res.Errors)

	assert.Equal(t, 2, fake.uploadCount())

	stA, found := svc.state.Get("a")
	require.True(t, found)
	assert.Equal(t, 3, stA.DaysLeftShown)
	assert.NotEmpty(t, stA.OriginalPath)

	// The original poster must be backed up before any upload.
	original := readFile(stA.OriginalPath)
	require.NotNil(t, original)
	assert.Equal(t, fake.images["a"], original)
}

func TestRun_SkipsUnchanged(t *testing.T) {
	svc, fake, _, _ := newTestService(t, true, []models.Media{
		scheduledMedia("a", time.Now().Add(72*time.Hour)),
	})
	fake.images["a"] = testPoster(t, 300, 450)

	res, err := svc.Run(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, res.Processed)
	assert.Equal(t, 1, fake.uploadCount())

	res, err = svc.Run(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, res.Skipped)
	assert.Zero(t, res.Processed)
	assert.Equal(t, 1, fake.uploadCount(), "unchanged banners must not re-upload")
}

func TestRun_UpdatesWhenDaysChange(t *testing.T) {
	svc, fake, provider, _ := newTestService(t, true, []models.Media{
		scheduledMedia("a", time.Now().Add(time.Hour)),
	})
	fake.images["a"] = testPoster(t, 300, 450)

	res, err := svc.Run(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, res.Processed)

	st, _ := svc.state.Get("a")
	assert.Equal(t, 1, st.DaysLeftShown)

	// The day count moves -> banner must be re-rendered.
	provider.setMedia([]models.Media{scheduledMedia("a", time.Now().Add(50*time.Hour))})
	res, err = svc.Run(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, res.Processed)
	assert.Equal(t, 2, fake.uploadCount())

	st, _ = svc.state.Get("a")
	assert.Equal(t, 3, st.DaysLeftShown)
}

func TestRun_RevertsStale(t *testing.T) {
	svc, fake, provider, _ := newTestService(t, true, []models.Media{
		scheduledMedia("a", time.Now().Add(72*time.Hour)),
	})
	fake.images["a"] = testPoster(t, 300, 450)

	res, err := svc.Run(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, res.Processed)

	// The item leaves the scheduled set (still present, but no deletion date)
	// -> original poster must be restored.
	provider.setMedia([]models.Media{
		{ID: "media-a", Type: models.MediaTypeMovie, Title: "Movie a", JellyfinID: "a"},
	})
	res, err = svc.Run(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, res.Reverted)
	assert.Zero(t, res.Errors)

	assert.Equal(t, 2, fake.uploadCount())
	last := fake.uploads[1]
	assert.Equal(t, "a", last.itemID)
	assert.Equal(t, fake.images["a"], last.data, "revert must upload the original poster")

	_, found := svc.state.Get("a")
	assert.False(t, found)
}

func TestRun_EmptyLibraryDefersPass(t *testing.T) {
	// State persisted from a previous session + an empty media list (the state
	// right after startup, before the first sync completes). The pass must not
	// revert the persisted banners.
	svc, fake, provider, _ := newTestService(t, true, nil)
	fake.images["a"] = testPoster(t, 300, 450)

	// Simulate a previous session that applied a banner (with its backup file).
	backupPath := filepath.Join(t.TempDir(), "overlays", "originals", "a.jpg")
	require.NoError(t, os.MkdirAll(filepath.Dir(backupPath), 0o755))
	require.NoError(t, os.WriteFile(backupPath, fake.images["a"], 0o644))
	require.NoError(t, svc.state.Set(storage.OverlayItemState{
		ItemID:        "a",
		OriginalPath:  backupPath,
		DaysLeftShown: 3,
		StyleHash:     "prev",
		AppliedAt:     time.Now(),
	}))

	res, err := svc.Run(context.Background())
	require.NoError(t, err)
	assert.Zero(t, res.Processed)
	assert.Zero(t, res.Reverted)
	assert.Zero(t, res.Errors)
	assert.Zero(t, fake.uploadCount(), "no media library yet: nothing may be uploaded or reverted")

	// Once the library populates (even with zero scheduled items), stale state reverts.
	provider.setMedia([]models.Media{
		{ID: "media-a", Type: models.MediaTypeMovie, Title: "Movie a", JellyfinID: "a"},
	})
	res, err = svc.Run(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, res.Reverted)
	assert.Equal(t, 1, fake.uploadCount())
}

func TestRun_ReappliesWhenStyleChanges(t *testing.T) {
	svc, fake, _, _ := newTestService(t, true, []models.Media{
		scheduledMedia("a", time.Now().Add(72*time.Hour)),
	})
	fake.images["a"] = testPoster(t, 300, 450)

	res, err := svc.Run(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, res.Processed)
	assert.Equal(t, 1, fake.uploadCount())

	// Same day count, different style -> must re-render.
	svc.cfg.Overlay.TextTemplate = "Leaving in {days} days"
	res, err = svc.Run(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, res.Processed, "style change must re-apply even with an unchanged day count")
	assert.Equal(t, 2, fake.uploadCount())

	st, _ := svc.state.Get("a")
	assert.NotEqual(t, "prev", st.StyleHash)
	assert.NotEmpty(t, st.StyleHash)
}

func TestRun_PosterLessItemIsSkippedNotError(t *testing.T) {
	svc, fake, _, _ := newTestService(t, true, []models.Media{
		scheduledMedia("a", time.Now().Add(72*time.Hour)),
	})
	// No image registered for "a" -> GetItemImage fails with a 404-like error.

	res, err := svc.Run(context.Background())
	require.NoError(t, err)
	assert.Zero(t, res.Processed)
	assert.Zero(t, res.Errors, "missing poster is a skip, not an error")
	assert.Equal(t, 1, res.Skipped)
	assert.Zero(t, fake.uploadCount())
}

func TestStyleHash(t *testing.T) {
	base := &config.OverlayConfig{
		TextTemplate:        "in {days} days",
		FontSizePercent:     5,
		FontColor:           "#ffffff",
		BackgroundColor:     "rgba(0,0,0,0.75)",
		PaddingPercent:      2,
		CornerRadiusPercent: 50,
	}

	same := *base
	if styleHash(base) != styleHash(&same) {
		t.Fatal("identical styles must hash identically")
	}

	changed := *base
	changed.TextTemplate = "Leaving in {days} days"
	if styleHash(base) == styleHash(&changed) {
		t.Fatal("changed style must hash differently")
	}
}

func TestRun_UploadErrorKeepsState(t *testing.T) {
	svc, fake, _, _ := newTestService(t, true, []models.Media{
		scheduledMedia("a", time.Now().Add(72*time.Hour)),
	})
	fake.images["a"] = testPoster(t, 300, 450)
	fake.setErr = errors.New("jellyfin refused")

	res, err := svc.Run(context.Background())
	require.NoError(t, err)
	assert.Zero(t, res.Processed)
	assert.Equal(t, 1, res.Errors)

	_, found := svc.state.Get("a")
	assert.False(t, found, "failed upload must not record state")
}

func TestReset(t *testing.T) {
	svc, fake, provider, _ := newTestService(t, true, []models.Media{
		scheduledMedia("a", time.Now().Add(72*time.Hour)),
		scheduledMedia("b", time.Now().Add(48*time.Hour)),
	})
	fake.images["a"] = testPoster(t, 300, 450)
	fake.images["b"] = testPoster(t, 300, 450)

	res, err := svc.Run(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 2, res.Processed)
	assert.Len(t, svc.state.GetAll(), 2)

	provider.setMedia(nil)
	res, err = svc.Reset(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 2, res.Reverted)
	assert.Empty(t, svc.state.GetAll())
}

func TestStatus(t *testing.T) {
	svc, fake, _, _ := newTestService(t, true, []models.Media{
		scheduledMedia("a", time.Now().Add(72*time.Hour)),
	})
	fake.images["a"] = testPoster(t, 300, 450)

	status := svc.Status()
	assert.True(t, status.Enabled)
	assert.False(t, status.Running)
	assert.Equal(t, 1, status.Targets)
	assert.Zero(t, status.Tracked)

	_, err := svc.Run(context.Background())
	require.NoError(t, err)

	status = svc.Status()
	assert.Equal(t, 1, status.Tracked)
	require.NotNil(t, status.LastResult)
	assert.Equal(t, 1, status.LastResult.Processed)
}

// blockingImageClient blocks inside GetItemImage until released, letting tests
// hold a Run in progress deterministically.
type blockingImageClient struct {
	started chan struct{}
	release chan struct{}
	img     []byte
}

func (b *blockingImageClient) GetItemImage(_ context.Context, _, _ string) ([]byte, string, error) {
	close(b.started)
	<-b.release
	return b.img, "image/jpeg", nil
}

func (b *blockingImageClient) SetItemImage(_ context.Context, _, _ string, _ []byte, _ string) error {
	return nil
}

func TestRun_RejectsConcurrentRun(t *testing.T) {
	block := &blockingImageClient{
		started: make(chan struct{}),
		release: make(chan struct{}),
		img:     testPoster(t, 300, 450),
	}

	cfg := &config.Config{Overlay: config.OverlayConfig{Enabled: true, TextTemplate: "in {days} days"}}
	useTestConfig(t, cfg)
	stateFile, err := storage.NewOverlayStateFile(t.TempDir())
	require.NoError(t, err)
	provider := &fakeMediaProvider{}
	provider.setMedia([]models.Media{scheduledMedia("a", time.Now().Add(72*time.Hour))})

	svc := NewService(cfg, stateFile, block, provider, t.TempDir())

	done := make(chan struct{})
	go func() {
		_, _ = svc.Run(context.Background())
		close(done)
	}()

	<-block.started // first run is in progress, holding the service lock

	_, err = svc.Run(context.Background())
	assert.ErrorIs(t, err, ErrAlreadyRunning)

	close(block.release)
	<-done
}
