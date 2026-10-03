package models

import (
	"encoding/json"
	"time"
)

// MediaType represents the type of media
type MediaType string

const (
	MediaTypeMovie  MediaType = "movie"
	MediaTypeTVShow MediaType = "tv_show"
)

// Media represents a media item (movie or TV show)
type Media struct {
	ID          string    `json:"id"`
	Type        MediaType `json:"type"`
	Title       string    `json:"title"`
	Year        int       `json:"year,omitempty"`
	AddedAt     time.Time `json:"added_at"`
	LastWatched time.Time `json:"last_watched,omitempty"`
	WatchCount  int       `json:"watch_count"`
	// GatedPlayCount counts distinct plays that met the minimum engagement
	// threshold (>= 120s), deduplicated by provider play identity. Kept
	// separate from WatchCount so existing rule semantics are unchanged.
	GatedPlayCount int `json:"gated_play_count"`
	// TotalWatchSeconds is the summed playback time of gated plays, used for
	// watch-hours-per-GB ROI.
	TotalWatchSeconds   int64     `json:"total_watch_seconds"`
	FilePath            string    `json:"file_path,omitempty"`
	FileSize            int64     `json:"file_size,omitempty"`
	QualityTag          string    `json:"quality_tag,omitempty"`
	Tags                []string  `json:"tags,omitempty"`
	IsExcluded          bool      `json:"excluded"`
	IsManualLeavingSoon bool      `json:"manual_leaving_soon"`
	IsRequested         bool      `json:"is_requested"`
	DeleteAfter         time.Time `json:"deletion_date,omitempty"`
	DaysUntilDue        int       `json:"days_until_deletion,omitempty"`
	DeletionReason      string    `json:"deletion_reason,omitempty"`

	// User-based cleanup fields
	RequestedByUserID   *int    `json:"requested_by_user_id,omitempty"`
	RequestedByUsername *string `json:"requested_by_username,omitempty"`
	RequestedByEmail    *string `json:"requested_by_email,omitempty"`
	WatchedByUsers      []int   `json:"watched_by_users,omitempty"`

	// Episode-level cleanup (TV shows only)
	// Non-empty = EpisodeRule targets these specific Sonarr episode file IDs for deletion.
	// Empty = whole-item deletion (standard behavior).
	EpisodeFileIDs []int `json:"episode_file_ids,omitempty"`

	// Source system IDs
	JellyfinID string `json:"jellyfin_id,omitempty"`
	RadarrID   int    `json:"radarr_id,omitempty"`
	SonarrID   int    `json:"sonarr_id,omitempty"`
	TMDBID     int    `json:"tmdb_id,omitempty"`
	TVDBID     int    `json:"tvdb_id,omitempty"`

	// Image availability (populated from Jellyfin during sync)
	HasPoster bool `json:"has_poster,omitempty"` // true when Jellyfin has a primary image for this item

	// Jellyfin matching status
	JellyfinMatchStatus  string `json:"jellyfin_match_status,omitempty"`  // "matched", "not_found", "metadata_mismatch"
	JellyfinMismatchInfo string `json:"jellyfin_mismatch_info,omitempty"` // Details about the mismatch

	// Jellyfin match diagnosis. Populated lazily by the adjudicator when the
	// user asks (Analyze / Fix Match); the scheduled sync only classifies the
	// item — status, a cheap human-readable reason, and a conflict id.
	// JellyfinMatchReason is a human-readable explanation of why the item is
	// unmatched against Jellyfin, JellyfinConflictID is the Jellyfin item id
	// involved in a conflict (empty when none), and JellyfinVerdict is the
	// adjudicator's conclusion ("jellyfin_wrong", "arr_wrong", or "ambiguous").
	JellyfinMatchReason string `json:"jellyfin_match_reason,omitempty"`
	JellyfinConflictID  string `json:"jellyfin_conflict_id,omitempty"`
	JellyfinVerdict     string `json:"jellyfin_verdict,omitempty"`
}

// MediaList represents a list of media items with metadata
type MediaList struct {
	Items      []Media `json:"items"`
	Total      int     `json:"total"`
	Page       int     `json:"page"`
	PageSize   int     `json:"page_size"`
	TotalPages int     `json:"total_pages"`
}

// WatchHistory represents watch history for a media item
type WatchHistory struct {
	MediaID   string    `json:"media_id"`
	UserID    string    `json:"user_id"`
	WatchedAt time.Time `json:"watched_at"`
	Completed bool      `json:"completed"`
	PlayCount int       `json:"play_count"`
}

// DeletionCandidate represents a media item ready for deletion
type DeletionCandidate struct {
	Media        Media     `json:"media"`
	Reason       string    `json:"reason"`
	RetentionDue time.Time `json:"retention_due"`
	DaysOverdue  int       `json:"days_overdue"`
	SizeBytes    int64     `json:"size_bytes"`
}

// DeletionTimeline represents the deletion schedule
type DeletionTimeline struct {
	TotalItems     int                `json:"total_items"`
	TotalSizeBytes int64              `json:"total_size_bytes"`
	ByDate         map[string][]Media `json:"by_date"`
	LeavingSoon    []Media            `json:"leaving_soon"`
}

// LeavingSoonItem is the normalized leaving-soon contract shared with external
// consumers (e.g. jellyfin-plugin-leaving-soon). Every provider emits the same
// shape so the plugin has one consumer path.
type LeavingSoonItem struct {
	// MediaServerID is the id Jellyfin knows for the item (a Jellyfin item GUID).
	// Empty when OxiCleanarr has no Jellyfin match yet - consumers skip those.
	MediaServerID string `json:"mediaServerId"`
	// Type is "movie" or "show" (already normalized, unlike models.Media.Type).
	Type string `json:"type"`
	// Title is informational only; consumers may prefer Jellyfin's own title.
	Title string `json:"title,omitempty"`
	// DeletionDate is when the item is scheduled for deletion (RFC3339).
	DeletionDate *time.Time `json:"deletionDate,omitempty"`
	// SourcePath is optional; consumers resolve paths from Jellyfin themselves.
	SourcePath string `json:"sourcePath,omitempty"`
}

// LeavingSoonResponse is the envelope for GET /api/media/leaving-soon.
type LeavingSoonResponse struct {
	// Version lets consumers detect contract changes without parsing fields.
	Version int               `json:"version"`
	Items   []LeavingSoonItem `json:"items"`
}

// MarshalJSON customizes the JSON output for Media to match frontend expectations
func (m Media) MarshalJSON() ([]byte, error) {
	// Convert type for frontend compatibility
	mediaType := string(m.Type)
	if mediaType == "tv_show" {
		mediaType = "show"
	}

	type Alias Media
	return json.Marshal(&struct {
		Type string `json:"type"`
		*Alias
	}{
		Type:  mediaType,
		Alias: (*Alias)(&m),
	})
}
