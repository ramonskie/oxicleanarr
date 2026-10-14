package overlay

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/ramonskie/oxicleanarr/internal/clients"
	"github.com/ramonskie/oxicleanarr/internal/config"
	"github.com/ramonskie/oxicleanarr/internal/models"
	"github.com/ramonskie/oxicleanarr/internal/storage"
	"github.com/rs/zerolog/log"
)

// MediaProvider is the subset of the sync engine the overlay service reads
// scheduled-deletion media from. Satisfied by *services.SyncEngine.
type MediaProvider interface {
	GetMediaList() []models.Media
}

// ImageClient is the subset of the Jellyfin client the overlay service needs,
// so tests can substitute a fake. Satisfied by *clients.JellyfinClient.
type ImageClient interface {
	GetItemImage(ctx context.Context, itemID, imageType string) ([]byte, string, error)
	SetItemImage(ctx context.Context, itemID, imageType string, data []byte, contentType string) error
	// ItemExists confirms whether an item still exists before state+backup are
	// dropped, so an overloaded 404 (bad baseURL/proxy) cannot destroy backups.
	ItemExists(ctx context.Context, itemID string) (bool, error)
}

// ErrAlreadyRunning is returned when a run/reset is requested while another is
// in progress (surfaced as HTTP 409).
var ErrAlreadyRunning = errors.New("overlay pass is already running")

// ErrOverlayDisabled is returned when a manual run is requested while the
// overlay feature is disabled in config.
var ErrOverlayDisabled = errors.New("overlay feature is disabled")

// ErrJellyfinUnavailable is returned when the Jellyfin integration is not
// enabled, so no overlay can be drawn.
var ErrJellyfinUnavailable = errors.New("Jellyfin integration is not enabled")

// errNoPrimaryImage marks an item with no artwork to draw on; it is counted as
// skipped (noise-free) rather than as an error.
var errNoPrimaryImage = errors.New("no primary image")

// errBannerCurrent marks an item whose server-side poster already equals the
// freshly rendered banner; it is counted as skipped with no upload. It is what
// makes drift detection cheap: equality is verified before re-uploading, so an
// unchanged banner costs a GET but no write.
var errBannerCurrent = errors.New("banner already current")

// errRemovedAtSource marks a scheduled target whose Jellyfin item no longer
// exists: an upload came back with clients.ErrItemNotFound (HTTP 404). The
// item's overlay state and backup are dropped and it is counted as skipped,
// never an error, so a deleted item cannot wedge the pass with a permanent
// failure.
var errRemovedAtSource = errors.New("item removed at source")

// errBackupMissing marks an item whose overlay state exists but whose saved
// original poster is gone. Nothing can be reverted, so the state is dropped and
// the item is counted as skipped (never an error). The server's current Primary
// is deliberately NOT adopted as a new original: it may already be the banner,
// which would permanently record the banner as the original.
var errBackupMissing = errors.New("overlay backup missing")

// Result summarizes one overlay pass.
type Result struct {
	Processed int `json:"processed"`
	Reverted  int `json:"reverted"`
	Skipped   int `json:"skipped"`
	Errors    int `json:"errors"`
	// Reason explains a pass that intentionally did no work, e.g. the media
	// library was still empty because the first sync had not populated it. It is
	// empty on a normal pass; the status endpoint surfaces it so an apparently
	// idle overlay can be diagnosed without reading logs.
	Reason string `json:"reason,omitempty"`
}

// Status is the current overlay service state, exposed by GET /api/overlay/status.
type Status struct {
	Enabled    bool      `json:"enabled"`
	Running    bool      `json:"running"`
	LastRun    time.Time `json:"last_run,omitempty"`
	LastResult *Result   `json:"last_result,omitempty"`
	Tracked    int       `json:"tracked"`
	Targets    int       `json:"targets"`
}

// target is one item that should carry a deletion banner.
type target struct {
	ItemID      string
	DeleteAfter time.Time
	DaysLeft    int
}

// Service draws "X days until deletion" banners onto the Jellyfin posters of
// scheduled-deletion media (Maintainerr-style) and reverts them when items
// leave the scheduled set.
type Service struct {
	cfg     *config.Config
	state   *storage.OverlayStateFile
	jf      ImageClient
	media   MediaProvider
	dataDir string

	// mu guards the status fields and scheduler lifecycle.
	mu         sync.Mutex
	running    bool
	lastRun    time.Time
	lastResult *Result

	// gateMu serializes pass ownership and the pending handoff. runningPass is
	// true while a pass holds the service; pendingPass records a pass requested
	// during that window so the holder runs once more. Holding both under gateMu
	// makes "acquire or coalesce" atomic, so a concurrent request can never be
	// erased by a holder clearing the flag.
	gateMu      sync.Mutex
	runningPass bool
	pendingPass bool

	ticker   *time.Ticker
	stopChan chan struct{}
	stopOnce sync.Once
}

// NewService creates the overlay service. jf may be nil when Jellyfin is not
// configured (runs then fail with ErrJellyfinUnavailable). dataDir is the
// application data directory (originals are stored under <dataDir>/overlays).
func NewService(
	cfg *config.Config,
	state *storage.OverlayStateFile,
	jf ImageClient,
	media MediaProvider,
	dataDir string,
) *Service {
	return &Service{
		cfg:      cfg,
		state:    state,
		jf:       jf,
		media:    media,
		dataDir:  dataDir,
		stopChan: make(chan struct{}),
	}
}

// Run executes one overlay pass: apply banners to scheduled-deletion items whose
// day count changed, verify unchanged ones against the server's current poster,
// and revert items that left the set. A concurrent Run/Reset is rejected with
// ErrAlreadyRunning.
func (s *Service) Run(ctx context.Context) (Result, error) {
	return s.runExclusive(ctx, false, false)
}

// RunForce executes one overlay pass with an explicit force flag. When force is
// true every scheduled target's banner is re-uploaded even when the day count,
// style, and the server's current Primary are unchanged, so an externally
// reverted or overwritten banner is unconditionally redrawn. When force is false
// it behaves exactly like Run. A concurrent Run/Reset is rejected with
// ErrAlreadyRunning.
func (s *Service) RunForce(ctx context.Context, force bool) (Result, error) {
	return s.runExclusive(ctx, false, force)
}

// runExclusive runs one pass, returning ErrAlreadyRunning when another pass
// holds the service. When it frees the slot it starts a background pass if one
// was requested while it was held, so a sync that completed during a manual
// run/reset is not missed. force is forwarded to runLocked and bypasses the
// per-item drift comparison.
func (s *Service) runExclusive(ctx context.Context, reset, force bool) (Result, error) {
	if !s.acquirePass() {
		return Result{}, ErrAlreadyRunning
	}
	defer s.finishPass()

	if reset {
		return s.resetLocked(ctx)
	}
	return s.runLocked(ctx, force)
}

// acquirePass claims the single pass slot, coalescing the request when a pass is
// already running. It reports false when the slot was busy.
func (s *Service) acquirePass() bool {
	s.gateMu.Lock()
	defer s.gateMu.Unlock()
	if s.runningPass {
		s.pendingPass = true
		return false
	}
	s.runningPass = true
	return true
}

// releasePass frees the pass slot and reports whether a pass was requested while
// it was held.
func (s *Service) releasePass() (rerun bool) {
	s.gateMu.Lock()
	defer s.gateMu.Unlock()
	s.runningPass = false
	if s.pendingPass {
		s.pendingPass = false
		return true
	}
	return false
}

// finishPass frees the slot and, if a pass was requested while it was held,
// starts one in the background.
func (s *Service) finishPass() {
	if s.releasePass() {
		go s.runPass()
	}
}

// hasPendingPass reports whether a pass is queued behind the current one.
func (s *Service) hasPendingPass() bool {
	s.gateMu.Lock()
	defer s.gateMu.Unlock()
	return s.pendingPass
}

// runLocked executes one overlay pass. Callers must hold the pass slot
// (acquirePass); it is not released here so a panic still unwinds through the
// caller's deferred release. force bypasses the per-item drift comparison and
// re-uploads every target.
func (s *Service) runLocked(ctx context.Context, force bool) (Result, error) {
	s.markRunning()
	defer s.markIdle()

	oc := s.currentOverlayConfig()
	if !oc.Enabled {
		return Result{}, ErrOverlayDisabled
	}
	if s.jf == nil {
		return Result{}, ErrJellyfinUnavailable
	}

	// An empty media library almost always means the initial sync has not
	// populated anything yet: Start() launches the first full sync in a
	// goroutine, so the very first overlay pass runs against an empty list.
	// Treating that as "nothing scheduled" would revert every persisted banner
	// on every restart. Defer the whole pass until the library is populated.
	media := s.media.GetMediaList()
	if len(media) == 0 {
		// Nothing to look at yet: the first sync has not populated the library.
		// Reverting persisted banners here would be wrong, so defer the whole
		// pass. Record why (surfaced via Status) and log at Info so an apparently
		// idle overlay is diagnosable without enabling debug logging.
		res := Result{Reason: "media library empty; waiting for sync to populate it"}
		s.mu.Lock()
		s.lastResult = &res
		s.mu.Unlock()
		log.Info().Msg("Overlay pass deferred: media library empty (sync not yet complete)")
		return res, nil
	}

	targets := scheduledTargets(media, time.Now())
	targetSet := make(map[string]target, len(targets))
	for _, t := range targets {
		targetSet[t.ItemID] = t
	}

	res := Result{}
	wantStyle := styleHash(oc)

	// Revert items that are no longer scheduled for deletion.
	for itemID, st := range s.state.GetAll() {
		if _, keep := targetSet[itemID]; keep {
			continue
		}
		if err := s.revertItem(ctx, itemID, st.OriginalPath); err != nil {
			res.Errors++
			log.Warn().Err(err).Str("item_id", itemID).Msg("Failed to revert overlay")
			continue
		}
		res.Reverted++
	}

	// Apply or update banners. Every target is verified against the server's
	// current Primary so an externally reverted or overwritten banner is
	// redrawn: applyTarget reports errBannerCurrent when the poster already
	// matches the freshly rendered banner, and force skips that comparison.
	for _, t := range targets {
		err := s.applyTarget(ctx, t, wantStyle, force)
		switch {
		case err == nil:
			res.Processed++
		case errors.Is(err, errBannerCurrent):
			res.Skipped++
		case errors.Is(err, errNoPrimaryImage):
			res.Skipped++
			log.Debug().Str("item_id", t.ItemID).Msg("No primary image for item, skipping overlay")
		case errors.Is(err, errRemovedAtSource):
			res.Skipped++
		case errors.Is(err, errBackupMissing):
			res.Skipped++
		default:
			res.Errors++
			log.Warn().Err(err).Str("item_id", t.ItemID).Int("days_left", t.DaysLeft).Msg("Failed to apply overlay")
		}
	}

	s.mu.Lock()
	s.lastResult = &res
	s.mu.Unlock()

	log.Info().
		Int("processed", res.Processed).
		Int("reverted", res.Reverted).
		Int("skipped", res.Skipped).
		Int("errors", res.Errors).
		Msg("Overlay pass complete")

	return res, nil
}

// Reset restores every original poster and drops all overlay state.
// A concurrent Run/Reset is rejected with ErrAlreadyRunning.
func (s *Service) Reset(ctx context.Context) (Result, error) {
	return s.runExclusive(ctx, true, false)
}

// resetLocked restores every original poster and drops all overlay state.
// Callers must hold the pass slot (acquirePass); it is released by the caller.
func (s *Service) resetLocked(ctx context.Context) (Result, error) {
	s.markRunning()
	defer s.markIdle()

	if s.jf == nil {
		return Result{}, ErrJellyfinUnavailable
	}

	res := Result{}
	for itemID, st := range s.state.GetAll() {
		if err := s.revertItem(ctx, itemID, st.OriginalPath); err != nil {
			res.Errors++
			log.Warn().Err(err).Str("item_id", itemID).Msg("Failed to revert overlay during reset")
			continue
		}
		res.Reverted++
	}

	s.mu.Lock()
	s.lastResult = &res
	s.mu.Unlock()

	log.Info().Int("reverted", res.Reverted).Int("errors", res.Errors).Msg("Overlay reset complete")
	return res, nil
}

// Status reports the current overlay service state.
func (s *Service) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()

	oc := s.currentOverlayConfig()
	return Status{
		Enabled:    oc.Enabled,
		Running:    s.running,
		LastRun:    s.lastRun,
		LastResult: s.lastResult,
		Tracked:    len(s.state.GetAll()),
		Targets:    len(scheduledTargets(s.media.GetMediaList(), time.Now())),
	}
}

// markRunning flags the service as busy for Status.
func (s *Service) markRunning() {
	s.mu.Lock()
	s.running = true
	s.mu.Unlock()
}

// markIdle clears the busy flag and records the pass end time.
func (s *Service) markIdle() {
	s.mu.Lock()
	s.running = false
	s.lastRun = time.Now()
	s.mu.Unlock()
}

// Start begins the scheduled overlay pass (only when the feature is enabled).
// Mirrors the sync engine's ticker pattern; config is read live so an
// interval change applies on the next tick.
func (s *Service) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.ticker != nil {
		return nil
	}

	oc := s.currentOverlayConfig()
	if !oc.Enabled {
		log.Info().Msg("Overlay feature is disabled, scheduler not started")
		return nil
	}

	interval := time.Duration(oc.IntervalHours) * time.Hour
	if interval <= 0 {
		interval = 24 * time.Hour
	}

	// Recreate so Start() can be called again after Stop().
	s.stopChan = make(chan struct{})
	s.stopOnce = sync.Once{}
	s.ticker = time.NewTicker(interval)
	go s.runLoop(s.ticker, interval)

	log.Info().
		Bool("enabled", oc.Enabled).
		Dur("interval", interval).
		Msg("Overlay scheduler started")

	return nil
}

// Stop stops the scheduled overlay pass.
func (s *Service) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.ticker != nil {
		s.ticker.Stop()
		s.ticker = nil
	}
	s.stopOnce.Do(func() { close(s.stopChan) })
	log.Info().Msg("Overlay scheduler stopped")
}

// runLoop ticks on the configured interval and also runs once immediately.
// The ticker is captured locally so a concurrent Stop() (which nils the field)
// cannot race the Reset below.
func (s *Service) runLoop(ticker *time.Ticker, interval time.Duration) {
	defer func() {
		if r := recover(); r != nil {
			log.Error().Interface("panic", r).Msg("Overlay scheduler panicked")
		}
	}()

	s.runPass()

	for {
		select {
		case <-ticker.C:
			// Pick up config changes (interval, enabled) live.
			oc := s.currentOverlayConfig()
			if !oc.Enabled {
				continue
			}
			want := time.Duration(oc.IntervalHours) * time.Hour
			if want <= 0 {
				want = 24 * time.Hour
			}
			if want != interval {
				interval = want
				ticker.Reset(interval)
			}
			s.runPass()
		case <-s.stopChan:
			return
		}
	}
}

// runPass executes overlay passes with the scheduler's timeout, serialization
// and error handling. Panics are recovered (and the slot always released) so a
// background trigger can never crash the application or wedge the service. A
// disabled feature or an unavailable Jellyfin client is not worth a warning.
//
// If another pass already holds the slot, this one coalesces into pendingPass
// and returns without blocking; the holder drains it and runs once more so a
// sync that repopulated the library mid-pass is not missed.
func (s *Service) runPass() {
	for {
		if !s.acquirePass() {
			return
		}

		// Fresh deadline per pass so a coalesced follow-up does not inherit the
		// time already spent on the previous one.
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)

		var rerun bool
		var err error
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Error().Interface("panic", r).Msg("Overlay pass panicked")
				}
				rerun = s.releasePass()
			}()
			_, err = s.runLocked(ctx, false)
		}()
		cancel()

		if err != nil &&
			!errors.Is(err, ErrOverlayDisabled) &&
			!errors.Is(err, ErrJellyfinUnavailable) &&
			!errors.Is(err, ErrAlreadyRunning) {
			log.Warn().Err(err).Msg("Overlay pass failed")
		}

		if !rerun {
			return
		}
	}
}

// Trigger requests an immediate overlay pass in the background, independent of
// the configured interval. It is called after a sync has populated the media
// library so banners are applied without waiting for the next scheduled tick.
// It never blocks the caller, and passes are serialized (a trigger that arrives
// during a running pass is coalesced into one extra pass after it finishes).
func (s *Service) Trigger() {
	if !s.overlayActive() {
		return
	}
	go s.runPass()
}

// overlayActive reports whether a pass could actually do work: the feature is
// enabled and a Jellyfin client is configured. It lets Trigger avoid spawning a
// goroutine that would only no-op.
func (s *Service) overlayActive() bool {
	if s.jf == nil {
		return false
	}
	return s.currentOverlayConfig().Enabled
}

// currentOverlayConfig reads the live config so hot-reloads apply without a
// restart (falls back to the injected pointer when the global is unavailable).
func (s *Service) currentOverlayConfig() *config.OverlayConfig {
	if cur := config.Get(); cur != nil {
		return &cur.Overlay
	}
	return &s.cfg.Overlay
}

// scheduledTargets returns the items that should carry a banner: scheduled for
// deletion (DeleteAfter set), not excluded, and matched to Jellyfin.
func scheduledTargets(media []models.Media, now time.Time) []target {
	targets := make([]target, 0)
	for _, m := range media {
		if m.IsExcluded || m.JellyfinID == "" || m.DeleteAfter.IsZero() {
			continue
		}
		targets = append(targets, target{
			ItemID:      m.JellyfinID,
			DeleteAfter: m.DeleteAfter,
			DaysLeft:    daysLeft(m.DeleteAfter, now),
		})
	}
	// Deterministic order for stable logs and tests.
	sort.Slice(targets, func(i, j int) bool { return targets[i].ItemID < targets[j].ItemID })
	return targets
}

// daysLeft returns the whole days until deleteAfter, clamped at 0 (past due
// renders "today").
func daysLeft(deleteAfter, now time.Time) int {
	diff := deleteAfter.Sub(now)
	if diff <= 0 {
		return 0
	}
	return int(math.Ceil(diff.Hours() / 24))
}

// applyTarget draws the banner onto one item's poster and records state. When
// the item already carries overlay state it fetches the server's CURRENT Primary
// and compares it to the freshly rendered banner: an external writer (or a
// Jellyfin refresh) can revert or overwrite the poster without touching our
// state, so the day-count/style dedupe alone would never notice. force skips
// that comparison and always re-uploads.
func (s *Service) applyTarget(ctx context.Context, t target, styleHash string, force bool) error {
	originalPath := s.originalPath(t.ItemID)

	// Reuse the saved original poster when present; otherwise download and back
	// it up before any mutation.
	var original []byte
	st, has := s.state.Get(t.ItemID)
	if has {
		original = readFile(st.OriginalPath)
		if original == nil {
			// The state says a banner was applied but the saved original is gone:
			// nothing can be reverted. Downloading the server's current Primary
			// here would permanently record the banner (which the server may
			// currently serve) as the "original", so drop the state instead and
			// skip the item with a loud warning.
			s.dropStateAndBackup(t.ItemID, originalPath)
			log.Warn().Str("item_id", t.ItemID).
				Msg("Backup missing; dropping overlay state, cannot revert")
			return errBackupMissing
		}
	}

	backedUpThisPass := false
	if !has {
		data, _, err := s.jf.GetItemImage(ctx, t.ItemID, "Primary")
		if err != nil {
			if errors.Is(err, clients.ErrImageNotFound) {
				// Nothing to draw on; callers count this as a skip, not an error.
				return errNoPrimaryImage
			}
			return fmt.Errorf("download poster: %w", err)
		}
		if len(data) == 0 {
			return errNoPrimaryImage
		}
		original = data
		if err := s.saveOriginal(originalPath, data); err != nil {
			return fmt.Errorf("back up poster: %w", err)
		}
		backedUpThisPass = true
	}

	style := bannerStyleFromConfig(s.currentOverlayConfig(), t.DaysLeft)
	rendered, err := RenderBanner(original, style)
	if err != nil {
		// Nothing was uploaded; drop a backup taken this pass so a broken
		// render cannot leave an unclaimed original behind.
		if backedUpThisPass {
			_ = os.Remove(originalPath)
		}
		return fmt.Errorf("render banner: %w", err)
	}

	// Drift guard: when we already applied a banner, verify the server still
	// holds it before deciding the item is up to date. A mismatch means the
	// poster was reverted (e.g. an image replace-all refresh) or overwritten,
	// so re-upload even though DaysLeftShown/StyleHash are unchanged. force
	// bypasses this and always re-uploads.
	if has && !force {
		current, _, err := s.jf.GetItemImage(ctx, t.ItemID, "Primary")
		switch {
		case err == nil:
			if bytes.Equal(current, rendered) {
				return errBannerCurrent
			}
			log.Debug().Str("item_id", t.ItemID).Msg("Overlay drifted from state, re-applying")
		case errors.Is(err, clients.ErrImageNotFound):
			// A 404 here is overloaded (a bad baseURL/proxy would 404 every
			// request), so confirm the item is really gone before destroying the
			// only recovery artifact. If it still exists — or the probe fails —
			// keep state+backup and retry on a later pass.
			exists, probeErr := s.jf.ItemExists(ctx, t.ItemID)
			if probeErr != nil {
				return fmt.Errorf("confirm item exists after poster 404: %w", probeErr)
			}
			if !exists {
				// The item is gone while we still hold overlay state: it is no
				// longer recoverable, so drop state+backup and count it as
				// removed at source (skipped) instead of a perpetual skip of a
				// state entry that can never revert.
				s.dropStateAndBackup(t.ItemID, originalPath)
				log.Info().Str("item_id", t.ItemID).
					Msg("Item removed at source (poster gone), dropping overlay state")
				return errRemovedAtSource
			}
			// The item still exists but currently serves no Primary (the poster
			// was removed without deleting the item). Do not drop state+backup:
			// fall through to re-upload the rendered banner, skipping the byte
			// comparison since there is nothing to compare against.
			log.Debug().Str("item_id", t.ItemID).
				Msg("Poster 404 but item still exists, re-applying")
		default:
			return fmt.Errorf("verify current poster: %w", err)
		}
	}

	if err := s.jf.SetItemImage(ctx, t.ItemID, "Primary", rendered, "image/jpeg"); err != nil {
		if errors.Is(err, clients.ErrItemNotFound) {
			// A 404 is overloaded (a bad baseURL/proxy would 404 everything), so
			// confirm the item is really gone before destroying the only recovery
			// artifact. If it still exists — or the probe fails — keep the state
			// and backup and retry on a later pass instead of dropping them.
			exists, probeErr := s.jf.ItemExists(ctx, t.ItemID)
			if probeErr != nil {
				return fmt.Errorf("confirm item exists after upload 404: %w", probeErr)
			}
			if exists {
				return fmt.Errorf("upload overlay: item reported missing but still exists: %w", err)
			}
			s.dropStateAndBackup(t.ItemID, originalPath)
			log.Info().Str("item_id", t.ItemID).Msg("Item removed at source, dropping overlay state")
			return errRemovedAtSource
		}
		return fmt.Errorf("upload overlay: %w", err)
	}

	if err := s.state.Set(storage.OverlayItemState{
		ItemID:        t.ItemID,
		OriginalPath:  originalPath,
		DaysLeftShown: t.DaysLeft,
		StyleHash:     styleHash,
		AppliedAt:     time.Now(),
	}); err != nil {
		return fmt.Errorf("record state: %w", err)
	}

	log.Info().Str("item_id", t.ItemID).Int("days_left", t.DaysLeft).Msg("Overlay applied")
	return nil
}

// revertItem restores an item's original poster and drops its state. A missing
// backup clears state without an upload (nothing to restore).
func (s *Service) revertItem(ctx context.Context, itemID, originalPath string) error {
	original := readFile(originalPath)
	if original == nil {
		log.Warn().Str("item_id", itemID).Msg("No saved original poster, dropping overlay state")
		return s.state.Remove(itemID)
	}

	if err := s.jf.SetItemImage(ctx, itemID, "Primary", original, "image/jpeg"); err != nil {
		if errors.Is(err, clients.ErrItemNotFound) {
			// Confirm the item is really gone before dropping state+backup: a
			// misconfigured baseURL/proxy 404s every request, and destroying the
			// backup would make the restore impossible once the config is fixed.
			exists, probeErr := s.jf.ItemExists(ctx, itemID)
			if probeErr != nil {
				return fmt.Errorf("confirm item exists after restore 404: %w", probeErr)
			}
			if exists {
				return fmt.Errorf("restore poster: item reported missing but still exists: %w", err)
			}
			// The item no longer exists on Jellyfin, so there is nothing to
			// restore. Drop the state and backup instead of retrying this upload
			// on every pass. Counted as reverted (removed at source), never an
			// error, so a subsequent pass performs no further work for it.
			s.dropStateAndBackup(itemID, originalPath)
			log.Info().Str("item_id", itemID).Msg("Item removed at source, dropped overlay state")
			return nil
		}
		// Keep backup and state so a later pass can retry the restore.
		return fmt.Errorf("restore poster: %w", err)
	}

	_ = os.Remove(originalPath)
	if err := s.state.Remove(itemID); err != nil {
		return fmt.Errorf("drop state: %w", err)
	}

	log.Info().Str("item_id", itemID).Msg("Overlay reverted")
	return nil
}

// dropStateAndBackup removes an item's overlay state and its saved original
// poster. It is used when the item no longer exists on Jellyfin (a typed 404
// upload): there is nothing left to restore and retrying would fail forever.
//
// Ordering matters: the state entry is removed first and the backup file is
// only deleted once that succeeded (or the entry was already absent). If the
// state write fails the backup is kept and a warning is logged, so a failed
// removal never destroys the only recovery artifact.
func (s *Service) dropStateAndBackup(itemID, originalPath string) {
	if err := s.state.Remove(itemID); err != nil {
		log.Warn().Err(err).Str("item_id", itemID).
			Msg("Failed to drop overlay state; keeping backup for recovery")
		return
	}
	if originalPath != "" {
		_ = os.Remove(originalPath)
	}
}

// bannerStyleFromConfig builds a BannerStyle for a day count.
func bannerStyleFromConfig(oc *config.OverlayConfig, daysLeft int) BannerStyle {
	return BannerStyle{
		Text:                bannerText(oc.TextTemplate, daysLeft),
		FontSizePercent:     oc.FontSizePercent,
		FontColor:           oc.FontColor,
		BackgroundColor:     oc.BackgroundColor,
		PaddingPercent:      oc.PaddingPercent,
		CornerRadiusPercent: oc.CornerRadiusPercent,
		FontPath:            oc.FontPath,
	}
}

// styleHash fingerprints the style settings a banner was rendered with, so a
// style change (template, colors, font, sizing) re-renders existing banners
// even when the day count is unchanged.
func styleHash(oc *config.OverlayConfig) string {
	h := sha256.New()
	for _, s := range []string{
		oc.TextTemplate,
		oc.FontColor,
		oc.BackgroundColor,
		oc.FontPath,
		fmt.Sprintf("%v", oc.FontSizePercent),
		fmt.Sprintf("%v", oc.PaddingPercent),
		fmt.Sprintf("%v", oc.CornerRadiusPercent),
	} {
		io.WriteString(h, s)
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

func (s *Service) originalPath(itemID string) string {
	return filepath.Join(s.dataDir, "overlays", "originals", itemID+".jpg")
}

// saveOriginal persists the original poster atomically: it writes to a temp
// file in the destination directory, fsyncs, then renames it into place (the
// same approach as internal/storage's atomic write). A crash mid-write leaves
// any previous backup intact instead of a truncated file that would be adopted
// as a bogus "original".
func (s *Service) saveOriginal(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer func() {
		if tmpName != "" {
			tmp.Close()
			os.Remove(tmpName)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("sync temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		return fmt.Errorf("chmod temp file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("rename temp file: %w", err)
	}
	tmpName = "" // renamed; nothing left to clean up
	return nil
}

func readFile(path string) []byte {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return data
}
