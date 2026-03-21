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
