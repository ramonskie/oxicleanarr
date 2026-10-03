package clients

import (
	"context"
	"time"
)

// StatsHistoryItem is the normalised watch history record shared by all stats providers.
type StatsHistoryItem struct {
	JellyfinItemID  string
	WatchedAt       time.Time
	PlaybackSeconds int
	// PlayID is a stable identity for the play this record belongs to. When a
	// provider groups resume chains (e.g. Tracearr's reference_id) this is the
	// chain key; otherwise it is a per-session id or empty. Empty means each
	// record is treated as a distinct play.
	PlayID string
	// SeriesID, when non-empty, is the Jellyfin id of the show this record
	// belongs to. Providers that emit episode-level rows (e.g. Tracearr, which
	// exposes grandparent_rating_key) set this so episode plays roll up to the
	// series during sync. It is empty for movies and for providers whose rows
	// are already series-scoped: Jellystat stores the SeriesId as
	// NowPlayingItemId, and Streamystats is queried per series id.
	SeriesID string
}

// StatsProvider is the common interface for watch-history providers (Jellystat, Streamystats).
// GetHistory accepts a list of Jellyfin item IDs so that item-scoped providers
// (e.g. Streamystats) can query only the items of interest.
// Bulk providers (e.g. Jellystat) may ignore itemIDs and return their full history.
type StatsProvider interface {
	// GetHistory returns normalised watch history for the given Jellyfin item IDs.
	GetHistory(ctx context.Context, itemIDs []string) ([]StatsHistoryItem, error)
	// Ping checks reachability of the remote service.
	Ping(ctx context.Context) error
}
