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
	ID             string    `json:"id"`
	Type           MediaType `json:"type"`
	Title          string    `json:"title"`
	Year           int       `json:"year,omitempty"`
	AddedAt        time.Time `json:"added_at"`
	LastWatched    time.Time `json:"last_watched,omitempty"`
	WatchCount     int       `json:"watch_count"`
	FilePath       string    `json:"file_path,omitempty"`
	FileSize       int64     `json:"file_size,omitempty"`
	QualityTag     string    `json:"quality_tag,omitempty"`
	Tags           []string  `json:"tags,omitempty"`
	IsExcluded     bool      `json:"excluded"`
	IsRequested    bool      `json:"is_requested"`
	DeleteAfter    time.Time `json:"deletion_date,omitempty"`
	DaysUntilDue   int       `json:"days_until_deletion,omitempty"`
	DeletionReason string    `json:"deletion_reason,omitempty"`

	// User-based cleanup fields
	RequestedByUserID   *int    `json:"requested_by_user_id,omitempty"`
	RequestedByUsername *string `json:"requested_by_username,omitempty"`
	RequestedByEmail    *string `json:"requested_by_email,omitempty"`
	WatchedByUsers      []int   `json:"watched_by_users,omitempty"`

	// Source system IDs
	JellyfinID string `json:"jellyfin_id,omitempty"`
	RadarrID   int    `json:"radarr_id,omitempty"`
	SonarrID   int    `json:"sonarr_id,omitempty"`
	TMDBID     int    `json:"tmdb_id,omitempty"`
	TVDBID     int    `json:"tvdb_id,omitempty"`

	// Jellyfin matching status
	JellyfinMatchStatus  string `json:"jellyfin_match_status,omitempty"`  // "matched", "not_found", "metadata_mismatch"
	JellyfinMismatchInfo string `json:"jellyfin_mismatch_info,omitempty"` // Details about the mismatch
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
