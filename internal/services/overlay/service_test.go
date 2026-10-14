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
	mu           sync.Mutex
	images       map[string][]byte // itemID -> original poster
	uploads      []fakeUpload      // successful SetItemImage calls
	setAttempts  []string          // itemID per SetItemImage call, including failures
	getErr       error
	setErr       error
	setErrByItem map[string]error // itemID -> error SetItemImage returns for it

	// itemExistsByItem overrides ItemExists per item; unlisted items exist.
	// itemExistsErr, when set, makes every ItemExists probe fail.
	itemExistsByItem map[string]bool
	itemExistsErr    error
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
	f.mu.Lock()
	defer f.mu.Unlock()
	// Record every attempt, including failures, so a test can prove a missing
	// item is not retried on a later pass.
	f.setAttempts = append(f.setAttempts, itemID)
	if f.setErr != nil {
		return f.setErr
	}
	if err, ok := f.setErrByItem[itemID]; ok {
		return err
	}
	stored := append([]byte(nil), data...)
	f.uploads = append(f.uploads, fakeUpload{itemID: itemID, imageType: imageType, data: stored, contentType: contentType})
	// Persist the uploaded bytes as the item's current poster so a later pass's
	// drift comparison (GetItemImage vs the freshly rendered banner) sees what
	// was actually written, mirroring the real server.
	if f.images == nil {
		f.images = make(map[string][]byte)
	}
	f.images[itemID] = stored
	return nil
}

// ItemExists reports whether an item is still on the server. Items default to
// existing (true, nil) so a test that does not opt in keeps the pre-404-confirm
// behavior; set itemExistsByItem or itemExistsErr to exercise the drop guard.
func (f *fakeImageClient) ItemExists(_ context.Context, itemID string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.itemExistsErr != nil {
		return false, f.itemExistsErr
	}
	if exists, ok := f.itemExistsByItem[itemID]; ok {
		return exists, nil
	}
	return true, nil
}

// setAttemptsFor counts SetItemImage attempts (successful or not) for an item.
func (f *fakeImageClient) setAttemptsFor(itemID string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, id := range f.setAttempts {
		if id == itemID {
			n++
		}
	}
	return n
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
	poster := testPoster(t, 300, 450)
	fake.images["a"] = poster
	fake.images["b"] = poster

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

	// The original poster must be backed up before any upload (the fake now
	// replaces fake.images["a"] with the uploaded banner, so compare against
	// the captured poster).
	original := readFile(stA.OriginalPath)
	require.NotNil(t, original)
	assert.Equal(t, poster, original)
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

func TestRun_ReuploadsOnDrift(t *testing.T) {
	poster := testPoster(t, 300, 450)
	svc, fake, _, _ := newTestService(t, true, []models.Media{
		scheduledMedia("a", time.Now().Add(72*time.Hour)),
	})
	fake.images["a"] = poster

	res, err := svc.Run(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, res.Processed)
	assert.Equal(t, 1, fake.uploadCount())

	// An external writer (or an image replace-all refresh) reverts the poster to
	// the original while our state still claims the banner is applied. DaysLeft
	// and StyleHash are unchanged, so only the byte comparison can detect it.
	fake.images["a"] = poster

	res, err = svc.Run(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, res.Processed, "a reverted banner must be redrawn even with unchanged day count/style")
	assert.Zero(t, res.Skipped)
	assert.Equal(t, 2, fake.uploadCount(), "drift must trigger a re-upload")

	// Once repaired, the banner matches again and the next pass skips.
	res, err = svc.Run(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, res.Skipped)
	assert.Zero(t, res.Processed)
	assert.Equal(t, 2, fake.uploadCount())
}

func TestRun_ForceBypassesSkip(t *testing.T) {
	svc, fake, _, _ := newTestService(t, true, []models.Media{
		scheduledMedia("a", time.Now().Add(72*time.Hour)),
	})
	fake.images["a"] = testPoster(t, 300, 450)

	res, err := svc.Run(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, res.Processed)
	assert.Equal(t, 1, fake.uploadCount())

	// Non-forced re-run: the banner is current -> skip, no upload.
	res, err = svc.Run(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, res.Skipped)
	assert.Equal(t, 1, fake.uploadCount())

	// Forced re-run: bypasses the drift comparison and always re-uploads.
	res, err = svc.RunForce(context.Background(), true)
	require.NoError(t, err)
	assert.Equal(t, 1, res.Processed)
	assert.Equal(t, 2, fake.uploadCount(), "force must re-upload even when the banner is current")

	// force=false behaves exactly like Run.
	res, err = svc.RunForce(context.Background(), false)
	require.NoError(t, err)
	assert.Equal(t, 1, res.Skipped)
	assert.Equal(t, 2, fake.uploadCount())
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

// TestRun_MissingItemClearsState covers the P4 durability rule: when a tracked
// item is deleted from Jellyfin, restoring its banner returns clients.ErrItemNotFound.
// The pass must drop the item's overlay state and its saved original (so no
// orphan backup lingers), count it as reverted rather than an error, and a
// subsequent pass must perform no further work for it — no perpetual 404 retry.
func TestRun_MissingItemClearsState(t *testing.T) {
	svc, fake, provider, _ := newTestService(t, true, []models.Media{
		scheduledMedia("a", time.Now().Add(72*time.Hour)),
		scheduledMedia("b", time.Now().Add(72*time.Hour)),
	})
	poster := testPoster(t, 300, 450)
	fake.images["a"] = poster
	fake.images["b"] = poster

	// Seed overlay state + backups for both items with a normal pass.
	res, err := svc.Run(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 2, res.Processed)
	assert.Zero(t, res.Errors)

	stA, found := svc.state.Get("a")
	require.True(t, found)
	backupA := stA.OriginalPath
	_, statErr := os.Stat(backupA)
	require.NoError(t, statErr, "the applied banner must have a saved original backup")

	// "a" is deleted from Jellyfin: any upload to it now 404s, and it leaves the
	// scheduled set, so the pass reverts it and the restore upload hits the
	// typed 404 rather than failing forever. Deletion is confirmed through the
	// realistic path (ItemExists=false), not assumed from the bare 404. "b"
	// stays scheduled and unchanged.
	fake.setErrByItem = map[string]error{"a": clients.ErrItemNotFound}
	fake.itemExistsByItem = map[string]bool{"a": false}
	provider.setMedia([]models.Media{
		scheduledMedia("b", time.Now().Add(72*time.Hour)),
		{ID: "media-a", Type: models.MediaTypeMovie, Title: "Movie a", JellyfinID: "a"},
	})

	res, err = svc.Run(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, res.Reverted, "the item removed at source is counted as reverted, not errored")
	assert.Zero(t, res.Errors, "a 404 on revert must not be counted as a failure")

	_, found = svc.state.Get("a")
	assert.False(t, found, "overlay state for the removed item must be dropped")
	_, statErr = os.Stat(backupA)
	assert.True(t, os.IsNotExist(statErr), "the removed item's backup must be deleted")

	_, found = svc.state.Get("b")
	assert.True(t, found, "the surviving scheduled item keeps its state")

	// A second pass must not retry the missing item: its state is gone, no new
	// upload attempt is made, and it is never counted again.
	attemptsAfterFirst := fake.setAttemptsFor("a")
	uploadsAfterFirst := fake.uploadCount()

	res, err = svc.Run(context.Background())
	require.NoError(t, err)
	assert.Zero(t, res.Reverted)
	assert.Zero(t, res.Errors)
	assert.Equal(t, attemptsAfterFirst, fake.setAttemptsFor("a"), "a deleted item must not be retried")
	assert.Equal(t, uploadsAfterFirst, fake.uploadCount(), "a deleted item must not trigger further uploads")
}

// TestRun_MissingItemStillScheduledDropsState covers the apply path for a
// tracked item that disappears while it is STILL scheduled: the drift guard
// detects the poster changed, the re-upload 404s, and ItemExists confirms the
// item is gone, so state+backup are dropped and the pass counts a skip with no
// error rather than wedging forever on a 404.
func TestRun_MissingItemStillScheduledDropsState(t *testing.T) {
	svc, fake, _, _ := newTestService(t, true, []models.Media{
		scheduledMedia("a", time.Now().Add(72*time.Hour)),
	})
	poster := testPoster(t, 300, 450)
	fake.images["a"] = poster

	res, err := svc.Run(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, res.Processed)

	stA, found := svc.state.Get("a")
	require.True(t, found)
	backupA := stA.OriginalPath
	_, statErr := os.Stat(backupA)
	require.NoError(t, statErr)

	// The item is deleted server-side but is still in the scheduled set. Force
	// the upload path (the poster drifted back to the original), make the upload
	// 404, and confirm deletion via ItemExists=false.
	fake.setErrByItem = map[string]error{"a": clients.ErrItemNotFound}
	fake.itemExistsByItem = map[string]bool{"a": false}
	fake.images["a"] = poster // drift from the applied banner -> re-upload

	res, err = svc.Run(context.Background())
	require.NoError(t, err)
	assert.Zero(t, res.Processed)
	assert.Zero(t, res.Errors, "a confirmed-removed item must not be counted as an error")
	assert.Equal(t, 1, res.Skipped)

	_, found = svc.state.Get("a")
	assert.False(t, found, "state for the confirmed-removed item must be dropped")
	_, statErr = os.Stat(backupA)
	assert.True(t, os.IsNotExist(statErr), "backup for the confirmed-removed item must be deleted")
}

// TestRun_ItemNotFoundNotConfirmedKeepsState covers the safety guard: a 404 from
// SetItemImage is NOT enough to drop state+backup. When ItemExists reports the
// item still exists (or the probe errors), the pass must keep both and surface a
// normal error to retry, so a misconfigured baseURL/proxy cannot destroy the
// only recovery artifact.
func TestRun_ItemNotFoundNotConfirmedKeepsState(t *testing.T) {
	run := func(t *testing.T, mutate func(*fakeImageClient)) {
		t.Helper()
		svc, fake, _, _ := newTestService(t, true, []models.Media{
			scheduledMedia("a", time.Now().Add(72*time.Hour)),
		})
		poster := testPoster(t, 300, 450)
		fake.images["a"] = poster

		res, err := svc.Run(context.Background())
		require.NoError(t, err)
		require.Equal(t, 1, res.Processed)

		stA, found := svc.state.Get("a")
		require.True(t, found)
		backupA := stA.OriginalPath

		fake.setErrByItem = map[string]error{"a": clients.ErrItemNotFound}
		fake.images["a"] = poster // drift -> upload path
		mutate(fake)

		res, err = svc.Run(context.Background())
		require.NoError(t, err)
		assert.Equal(t, 1, res.Errors, "an unconfirmed 404 must be a normal retryable error")
		assert.Zero(t, res.Processed)

		_, found = svc.state.Get("a")
		assert.True(t, found, "state must be kept when removal is not confirmed")
		_, statErr := os.Stat(backupA)
		assert.NoError(t, statErr, "backup must be kept when removal is not confirmed")
	}

	t.Run("item still exists", func(t *testing.T) {
		run(t, func(f *fakeImageClient) { f.itemExistsByItem = map[string]bool{"a": true} })
	})

	t.Run("confirmation probe fails", func(t *testing.T) {
		run(t, func(f *fakeImageClient) { f.itemExistsErr = errors.New("probe unavailable") })
	})
}

// TestRun_BackupLostDropsStateNoRebaseline covers the durability guard: when
// overlay state exists but its saved original is gone, the pass must NOT adopt
// the server's current Primary as a new "original" (it may already be the
// banner). It drops the state, uploads nothing, and counts a skip, not an error.
func TestRun_BackupLostDropsStateNoRebaseline(t *testing.T) {
	svc, fake, _, _ := newTestService(t, true, []models.Media{
		scheduledMedia("a", time.Now().Add(72*time.Hour)),
	})
	poster := testPoster(t, 300, 450)
	fake.images["a"] = poster

	res, err := svc.Run(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, res.Processed)
	require.Equal(t, 1, fake.uploadCount())

	stA, found := svc.state.Get("a")
	require.True(t, found)
	backupA := stA.OriginalPath
	require.NoError(t, os.Remove(backupA), "simulate a lost backup file")

	// The server currently serves the applied banner (fake.images["a"] is the
	// uploaded bytes). Re-baselining would permanently record it as the original.
	res, err = svc.Run(context.Background())
	require.NoError(t, err)
	assert.Zero(t, res.Processed)
	assert.Zero(t, res.Errors, "a lost backup is a skip, not an error")
	assert.Equal(t, 1, res.Skipped)
	assert.Equal(t, 1, fake.uploadCount(), "a lost backup must not trigger a re-baselined upload")

	_, found = svc.state.Get("a")
	assert.False(t, found, "state for an un-revertable item must be dropped")
	_, statErr := os.Stat(backupA)
	assert.True(t, os.IsNotExist(statErr), "no re-baselined backup may be written")
}

// TestRun_DriftPosterGoneDropsState covers the drift-guard 404: while overlay
// state exists, the verify GET returns ErrImageNotFound (the poster/item is
// gone), so state+backup are dropped and the item is skipped rather than left
// as a state entry that can never revert.
func TestRun_DriftPosterGoneDropsState(t *testing.T) {
	svc, fake, _, _ := newTestService(t, true, []models.Media{
		scheduledMedia("a", time.Now().Add(72*time.Hour)),
	})
	fake.images["a"] = testPoster(t, 300, 450)

	res, err := svc.Run(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, res.Processed)

	stA, found := svc.state.Get("a")
	require.True(t, found)
	backupA := stA.OriginalPath

	// The poster vanishes server-side; the drift verify GET 404s while our
	// backup is still present (so this is not the backup-lost path). ItemExists
	// confirms the item itself is gone, so the drop is justified.
	delete(fake.images, "a")
	fake.itemExistsByItem = map[string]bool{"a": false}

	res, err = svc.Run(context.Background())
	require.NoError(t, err)
	assert.Zero(t, res.Processed)
	assert.Zero(t, res.Errors)
	assert.Equal(t, 1, res.Skipped)

	_, found = svc.state.Get("a")
	assert.False(t, found, "state must be dropped when the poster is gone")
	_, statErr := os.Stat(backupA)
	assert.True(t, os.IsNotExist(statErr), "backup must be dropped when the poster is gone")
}

// TestRun_DriftPosterGoneNotConfirmedKeepsState is the safety guard on the
// drift-guard 404: a verify GET returning ErrImageNotFound is NOT enough to drop
// state+backup. When ItemExists reports the item still exists (it merely lacks a
// Primary) the banner is re-uploaded; when the probe itself errors both are kept
// and the pass surfaces a retryable error. A misconfigured baseURL/proxy that
// 404s every request must not destroy the only recovery artifact.
func TestRun_DriftPosterGoneNotConfirmedKeepsState(t *testing.T) {
	t.Run("item still exists re-uploads banner", func(t *testing.T) {
		svc, fake, _, _ := newTestService(t, true, []models.Media{
			scheduledMedia("a", time.Now().Add(72*time.Hour)),
		})
		fake.images["a"] = testPoster(t, 300, 450)

		res, err := svc.Run(context.Background())
		require.NoError(t, err)
		require.Equal(t, 1, res.Processed)

		stA, found := svc.state.Get("a")
		require.True(t, found)
		backupA := stA.OriginalPath
		uploadsBefore := fake.uploadCount()

		// The server's Primary vanishes so the drift verify GET 404s, but the
		// item itself still exists: re-upload instead of dropping.
		delete(fake.images, "a")

		res, err = svc.Run(context.Background())
		require.NoError(t, err)
		assert.Equal(t, 1, res.Processed, "the banner is re-uploaded, not skipped")
		assert.Zero(t, res.Errors)
		assert.Equal(t, uploadsBefore+1, fake.uploadCount(), "the banner must be re-uploaded")

		_, found = svc.state.Get("a")
		assert.True(t, found, "state must be kept when the item still exists")
		_, statErr := os.Stat(backupA)
		assert.NoError(t, statErr, "backup must be kept when the item still exists")
	})

	t.Run("confirmation probe fails keeps state", func(t *testing.T) {
		svc, fake, _, _ := newTestService(t, true, []models.Media{
			scheduledMedia("a", time.Now().Add(72*time.Hour)),
		})
		fake.images["a"] = testPoster(t, 300, 450)

		res, err := svc.Run(context.Background())
		require.NoError(t, err)
		require.Equal(t, 1, res.Processed)

		stA, found := svc.state.Get("a")
		require.True(t, found)
		backupA := stA.OriginalPath
		uploadsBefore := fake.uploadCount()

		// Drift verify GET 404s and the existence probe fails: keep state+backup
		// and report a retryable error, never a drop.
		delete(fake.images, "a")
		fake.itemExistsErr = errors.New("probe unavailable")

		res, err = svc.Run(context.Background())
		require.NoError(t, err)
		assert.Equal(t, 1, res.Errors, "an unconfirmed drift 404 must be a retryable error")
		assert.Zero(t, res.Processed)
		assert.Equal(t, uploadsBefore, fake.uploadCount(), "an unconfirmed 404 must not re-upload")

		_, found = svc.state.Get("a")
		assert.True(t, found, "state must be kept when removal is not confirmed")
		_, statErr := os.Stat(backupA)
		assert.NoError(t, statErr, "backup must be kept when removal is not confirmed")
	})
}

// TestRun_RevertItemNotFoundNotConfirmedKeepsState is the revert-side mirror of
// the upload guard: a restore 404 that ItemExists cannot confirm must keep
// state+backup and retry, not destroy them.
func TestRun_RevertItemNotFoundNotConfirmedKeepsState(t *testing.T) {
	run := func(t *testing.T, mutate func(*fakeImageClient)) {
		t.Helper()
		svc, fake, provider, _ := newTestService(t, true, []models.Media{
			scheduledMedia("a", time.Now().Add(72*time.Hour)),
		})
		fake.images["a"] = testPoster(t, 300, 450)

		res, err := svc.Run(context.Background())
		require.NoError(t, err)
		require.Equal(t, 1, res.Processed)

		stA, found := svc.state.Get("a")
		require.True(t, found)
		backupA := stA.OriginalPath

		// The item leaves the scheduled set -> the revert path runs. Its restore
		// upload 404s, but removal is not confirmed. (The library stays
		// non-empty so the pass is not deferred as an unpopulated library.)
		provider.setMedia([]models.Media{
			{ID: "media-a", Type: models.MediaTypeMovie, Title: "Movie a", JellyfinID: "a"},
		})
		fake.setErrByItem = map[string]error{"a": clients.ErrItemNotFound}
		mutate(fake)

		res, err = svc.Run(context.Background())
		require.NoError(t, err)
		assert.Equal(t, 1, res.Errors, "an unconfirmed restore 404 must be a retryable error")
		assert.Zero(t, res.Reverted)

		_, found = svc.state.Get("a")
		assert.True(t, found, "state must be kept when removal is not confirmed")
		_, statErr := os.Stat(backupA)
		assert.NoError(t, statErr, "backup must be kept when removal is not confirmed")
	}

	t.Run("item still exists", func(t *testing.T) {
		run(t, func(f *fakeImageClient) { f.itemExistsByItem = map[string]bool{"a": true} })
	})

	t.Run("confirmation probe fails", func(t *testing.T) {
		run(t, func(f *fakeImageClient) { f.itemExistsErr = errors.New("probe unavailable") })
	})
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
	assert.NotEmpty(t, res.Reason, "a deferred pass must explain why it did no work")

	status := svc.Status()
	require.NotNil(t, status.LastResult, "a deferral must be recorded as the last result")
	assert.NotEmpty(t, status.LastResult.Reason, "status must expose why the pass deferred")

	// Once the library populates (even with zero scheduled items), stale state reverts.
	provider.setMedia([]models.Media{
		{ID: "media-a", Type: models.MediaTypeMovie, Title: "Movie a", JellyfinID: "a"},
	})
	res, err = svc.Run(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, res.Reverted)
	assert.Equal(t, 1, fake.uploadCount())
}

func TestTrigger_RunsPassInBackground(t *testing.T) {
	svc, fake, _, _ := newTestService(t, true, []models.Media{
		scheduledMedia("a", time.Now().Add(72*time.Hour)),
	})
	fake.images["a"] = testPoster(t, 300, 450)

	svc.Trigger()

	require.Eventually(t, func() bool {
		_, found := svc.state.Get("a")
		return found
	}, 3*time.Second, 20*time.Millisecond, "Trigger must run a pass without waiting for the interval")
	assert.Equal(t, 1, fake.uploadCount())
}

func TestTrigger_DisabledIsNoOp(t *testing.T) {
	svc, fake, _, _ := newTestService(t, false, []models.Media{
		scheduledMedia("a", time.Now().Add(72*time.Hour)),
	})
	fake.images["a"] = testPoster(t, 300, 450)

	svc.Trigger()

	assert.False(t, svc.overlayActive(), "a disabled overlay must report inactive")
	assert.False(t, svc.hasPendingPass(), "Trigger must not arm a pass while inactive")
	// A spawned pass would set LastRun via markIdle even though it uploads
	// nothing, so LastRun is the deterministic signal that Trigger was gated.
	require.Never(t, func() bool { return !svc.Status().LastRun.IsZero() },
		200*time.Millisecond, 20*time.Millisecond, "a disabled overlay must not spawn a pass")
	require.Never(t, func() bool { return fake.uploadCount() > 0 },
		200*time.Millisecond, 20*time.Millisecond, "a disabled overlay must not upload")
}

// sequencedMediaProvider returns an extra scheduled item from its second
// GetMediaList call, so a coalesced follow-up pass has observable work.
type sequencedMediaProvider struct {
	mu    sync.Mutex
	calls int
	base  []models.Media
	extra models.Media
}

func (p *sequencedMediaProvider) GetMediaList() []models.Media {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls++
	out := append([]models.Media(nil), p.base...)
	if p.calls >= 2 {
		out = append(out, p.extra)
	}
	return out
}

func TestTrigger_DrainsPendingAfterRunningPass(t *testing.T) {
	block := &blockingImageClient{
		started: make(chan struct{}),
		release: make(chan struct{}),
		img:     testPoster(t, 300, 450),
	}
	cfg := &config.Config{Overlay: config.OverlayConfig{
		Enabled: true, IntervalHours: 24, TextTemplate: "in {days} days",
		FontSizePercent: 5, FontColor: "#ffffff", BackgroundColor: "rgba(0,0,0,0.75)",
		PaddingPercent: 2, CornerRadiusPercent: 50,
	}}
	useTestConfig(t, cfg)
	stateFile, err := storage.NewOverlayStateFile(t.TempDir())
	require.NoError(t, err)

	provider := &sequencedMediaProvider{
		base:  []models.Media{scheduledMedia("a", time.Now().Add(72*time.Hour))},
		extra: scheduledMedia("b", time.Now().Add(48*time.Hour)),
	}
	svc := NewService(cfg, stateFile, block, provider, t.TempDir())

	svc.Trigger() // first pass blocks downloading "a"
	<-block.started
	svc.Trigger() // arrives mid-pass -> must be coalesced, not dropped
	close(block.release)

	require.Eventually(t, func() bool {
		_, found := svc.state.Get("b")
		return found
	}, 3*time.Second, 20*time.Millisecond, "a trigger during a running pass must not be dropped")

	assert.False(t, svc.hasPendingPass(), "the pending trigger must be drained")
}

func TestTrigger_NonBlockingWhilePassRunning(t *testing.T) {
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
	<-block.started // a pass now holds the pass slot

	// Trigger must return promptly even though a pass is running: it records a
	// pending pass and returns instead of blocking.
	returned := make(chan struct{})
	go func() {
		svc.Trigger()
		close(returned)
	}()
	select {
	case <-returned:
	case <-time.After(time.Second):
		t.Fatal("Trigger must not block while a pass is running")
	}

	close(block.release)
	<-done
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

// TestRun_UploadErrorDoesNotRecordState covers a first-time apply whose upload
// fails: the item had no prior overlay state, so a failed upload must leave none
// recorded. (The "prior state survives a failed re-upload" case is covered
// separately by TestRun_UploadErrorKeepsPriorState.)
func TestRun_UploadErrorDoesNotRecordState(t *testing.T) {
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

// TestRun_UploadErrorKeepsPriorState covers a re-upload that fails with a
// non-404 error for an item that already carries overlay state: the prior state
// and its saved original must both survive so a later pass can retry, and the
// failed attempt must not be counted as an upload.
func TestRun_UploadErrorKeepsPriorState(t *testing.T) {
	svc, fake, _, _ := newTestService(t, true, []models.Media{
		scheduledMedia("a", time.Now().Add(72*time.Hour)),
	})
	poster := testPoster(t, 300, 450)
	fake.images["a"] = poster

	// Seed overlay state + backup with a normal first pass.
	res, err := svc.Run(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, res.Processed)

	stA, found := svc.state.Get("a")
	require.True(t, found)
	backupA := stA.OriginalPath
	uploadsBefore := fake.uploadCount()

	// The server's poster drifts back to the original, so the drift guard
	// re-applies; the re-upload then fails with a non-404 error.
	fake.images["a"] = poster
	fake.setErr = errors.New("jellyfin refused")

	res, err = svc.Run(context.Background())
	require.NoError(t, err)
	assert.Zero(t, res.Processed)
	assert.Equal(t, 1, res.Errors)
	assert.Equal(t, uploadsBefore, fake.uploadCount(), "a failed re-upload must not record an upload")

	_, found = svc.state.Get("a")
	assert.True(t, found, "a failed re-upload must keep prior state")
	_, statErr := os.Stat(backupA)
	assert.NoError(t, statErr, "a failed re-upload must keep the prior backup")
}

// TestRun_DriftVerifyErrorKeepsState proves a non-404 error from the drift
// guard's "verify current poster" GET (not the initial download) is counted as
// an error and keeps both the item's state and its saved original: a transient
// verify failure must not drop the only recovery artifact nor re-upload blindly.
func TestRun_DriftVerifyErrorKeepsState(t *testing.T) {
	svc, fake, _, _ := newTestService(t, true, []models.Media{
		scheduledMedia("a", time.Now().Add(72*time.Hour)),
	})
	fake.images["a"] = testPoster(t, 300, 450)

	// Seed overlay state + backup with a normal first pass.
	res, err := svc.Run(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, res.Processed)

	stA, found := svc.state.Get("a")
	require.True(t, found)
	backupA := stA.OriginalPath
	uploadsBefore := fake.uploadCount()

	// Once state exists the download path is skipped, so the only GetItemImage
	// call is the drift guard's verify GET. A non-404 error there must not be
	// mistaken for a reverted or missing poster.
	fake.getErr = errors.New("verify unavailable")

	res, err = svc.Run(context.Background())
	require.NoError(t, err)
	assert.Zero(t, res.Processed)
	assert.Equal(t, 1, res.Errors, "a non-404 drift verify error must be a retryable error")
	assert.Equal(t, uploadsBefore, fake.uploadCount(), "a failed verify must not re-upload")

	_, found = svc.state.Get("a")
	assert.True(t, found, "state must be kept when the drift verify fails")
	_, statErr := os.Stat(backupA)
	assert.NoError(t, statErr, "backup must be kept when the drift verify fails")
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
// hold a Run in progress deterministically. The first call signals started and
// blocks; later calls return immediately, so a coalesced follow-up pass cannot
// double-close the channel.
type blockingImageClient struct {
	started   chan struct{}
	release   chan struct{}
	startOnce sync.Once
	img       []byte
}

func (b *blockingImageClient) GetItemImage(_ context.Context, _, _ string) ([]byte, string, error) {
	b.startOnce.Do(func() {
		close(b.started)
		<-b.release
	})
	return b.img, "image/jpeg", nil
}

func (b *blockingImageClient) SetItemImage(_ context.Context, _, _ string, _ []byte, _ string) error {
	return nil
}

// ItemExists satisfies ImageClient; the blocking fake only exercises the pass
// serialization path, which never reaches the 404 drop guard.
func (b *blockingImageClient) ItemExists(_ context.Context, _ string) (bool, error) {
	return true, nil
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
