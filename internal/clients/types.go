package clients

import (
	"time"
)

// JellyfinItem represents a Jellyfin library item
type JellyfinItem struct {
	ID             string            `json:"Id"`
	Name           string            `json:"Name"`
	Type           string            `json:"Type"`
	ProductionYear int               `json:"ProductionYear"`
	DateCreated    time.Time         `json:"DateCreated"`
	Path           string            `json:"Path"`
	UserData       JellyfinUserData  `json:"UserData"`
	ProviderIds    map[string]string `json:"ProviderIds"`
}

// JellyfinVirtualFolder represents a virtual folder (library) in Jellyfin
type JellyfinVirtualFolder struct {
	Name           string                       `json:"Name"`
	Locations      []string                     `json:"Locations"`
	CollectionType string                       `json:"CollectionType"` // "movies", "tvshows", etc.
	LibraryOptions JellyfinVirtualFolderOptions `json:"LibraryOptions,omitempty"`
	ItemId         string                       `json:"ItemId,omitempty"`
}

// JellyfinVirtualFolderOptions represents library options
type JellyfinVirtualFolderOptions struct {
	EnablePhotos                          bool               `json:"EnablePhotos,omitempty"`
	EnableRealtimeMonitor                 bool               `json:"EnableRealtimeMonitor,omitempty"`
	EnableChapterImageExtraction          bool               `json:"EnableChapterImageExtraction,omitempty"`
	ExtractChapterImagesDuringLibraryScan bool               `json:"ExtractChapterImagesDuringLibraryScan,omitempty"`
	PathInfos                             []JellyfinPathInfo `json:"PathInfos,omitempty"`
}

// JellyfinPathInfo represents path information for a library
type JellyfinPathInfo struct {
	Path string `json:"Path"`
}

// JellyfinUserData represents user-specific data for a Jellyfin item
type JellyfinUserData struct {
	PlayCount      int       `json:"PlayCount"`
	LastPlayedDate time.Time `json:"LastPlayedDate"`
	Played         bool      `json:"Played"`
}

// JellyfinItemsResponse represents the response from Jellyfin items endpoint
type JellyfinItemsResponse struct {
	Items            []JellyfinItem `json:"Items"`
	TotalRecordCount int            `json:"TotalRecordCount"`
}

// RadarrMovie represents a movie in Radarr
type RadarrMovie struct {
	ID               int              `json:"id"`
	Title            string           `json:"title"`
	Year             int              `json:"year"`
	Added            time.Time        `json:"added"`
	Path             string           `json:"path"`
	SizeOnDisk       int64            `json:"sizeOnDisk"`
	HasFile          bool             `json:"hasFile"`
	QualityProfileId int              `json:"qualityProfileId"`
	TmdbId           int              `json:"tmdbId"`
	Tags             []int            `json:"tags"`
	MovieFile        *RadarrMovieFile `json:"movieFile,omitempty"`
}

// RadarrTag represents a tag in Radarr
type RadarrTag struct {
	ID    int    `json:"id"`
	Label string `json:"label"`
}

// RadarrMovieFile represents a movie file in Radarr
type RadarrMovieFile struct {
	ID           int           `json:"id"`
	RelativePath string        `json:"relativePath"`
	Path         string        `json:"path"`
	Size         int64         `json:"size"`
	DateAdded    time.Time     `json:"dateAdded"`
	Quality      RadarrQuality `json:"quality"`
}

// RadarrQuality represents quality information
type RadarrQuality struct {
	Quality RadarrQualityDef `json:"quality"`
}

// RadarrQualityDef represents quality definition
type RadarrQualityDef struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// RadarrHistory represents Radarr history entry
type RadarrHistory struct {
	MovieID   int       `json:"movieId"`
	EventType string    `json:"eventType"`
	Date      time.Time `json:"date"`
}

// SonarrSeries represents a TV series in Sonarr
type SonarrSeries struct {
	ID         int         `json:"id"`
	Title      string      `json:"title"`
	Year       int         `json:"year"`
	Added      time.Time   `json:"added"`
	Path       string      `json:"path"`
	Statistics SonarrStats `json:"statistics"`
	Tags       []int       `json:"tags"`
	TvdbId     int         `json:"tvdbId"`
}

// SonarrTag represents a tag in Sonarr
type SonarrTag struct {
	ID    int    `json:"id"`
	Label string `json:"label"`
}

// SonarrStats represents Sonarr series statistics
type SonarrStats struct {
	EpisodeFileCount  int   `json:"episodeFileCount"`
	EpisodeCount      int   `json:"episodeCount"`
	TotalEpisodeCount int   `json:"totalEpisodeCount"`
	SizeOnDisk        int64 `json:"sizeOnDisk"`
}

// SonarrEpisode represents a TV episode in Sonarr
type SonarrEpisode struct {
	ID            int                `json:"id"`
	SeriesID      int                `json:"seriesId"`
	EpisodeNumber int                `json:"episodeNumber"`
	SeasonNumber  int                `json:"seasonNumber"`
	Title         string             `json:"title"`
	HasFile       bool               `json:"hasFile"`
	EpisodeFile   *SonarrEpisodeFile `json:"episodeFile,omitempty"`
}

// SonarrEpisodeFile represents an episode file
type SonarrEpisodeFile struct {
	ID        int       `json:"id"`
	Path      string    `json:"path"`
	Size      int64     `json:"size"`
	DateAdded time.Time `json:"dateAdded"`
}

// SonarrHistory represents Sonarr history entry
type SonarrHistory struct {
	SeriesID  int       `json:"seriesId"`
	EpisodeID int       `json:"episodeId"`
	EventType string    `json:"eventType"`
	Date      time.Time `json:"date"`
}

// JellyseerrRequest represents a request in Jellyseerr
type JellyseerrRequest struct {
	ID          int             `json:"id"`
	Type        string          `json:"type"`
	Status      int             `json:"status"`
	Media       JellyseerrMedia `json:"media"`
	RequestedBy JellyseerrUser  `json:"requestedBy"`
	CreatedAt   time.Time       `json:"createdAt"`
}

// JellyseerrMedia represents media in a Jellyseerr request
type JellyseerrMedia struct {
	TmdbId int `json:"tmdbId"`
	TvdbId int `json:"tvdbId"`
}

// JellyseerrUser represents a user in Jellyseerr (Jellyfin-focused)
type JellyseerrUser struct {
	ID               int    `json:"id"`
	Email            string `json:"email"`
	Username         string `json:"username"`         // Usually empty
	JellyfinUsername string `json:"jellyfinUsername"` // Actual Jellyfin username
	DisplayName      string `json:"displayName"`      // Display name (preferred)
}

// JellyseerrResponse represents paginated response
type JellyseerrResponse struct {
	Results  []JellyseerrRequest `json:"results"`
	PageInfo JellyseerrPageInfo  `json:"pageInfo"`
}

// JellyseerrPageInfo represents pagination info
type JellyseerrPageInfo struct {
	Pages   int `json:"pages"`
	Results int `json:"results"`
}

// JellystatHistoryResponse represents paginated history response from Jellystat
type JellystatHistoryResponse struct {
	CurrentPage int                    `json:"current_page"`
	Pages       int                    `json:"pages"`
	Size        int                    `json:"size"`
	Results     []JellystatHistoryItem `json:"results"`
}

// JellystatHistoryItem represents a single watch history entry
type JellystatHistoryItem struct {
	ID                   string    `json:"Id"`
	UserID               string    `json:"UserId"`
	UserName             string    `json:"UserName"`
	NowPlayingItemID     string    `json:"NowPlayingItemId"`
	NowPlayingItemName   string    `json:"NowPlayingItemName"`
	SeriesName           string    `json:"SeriesName"` // null for movies, series name for TV
	EpisodeID            string    `json:"EpisodeId"`
	SeasonID             string    `json:"SeasonId"`
	PlaybackDuration     int       `json:"PlaybackDuration"`     // Duration in seconds
	ActivityDateInserted time.Time `json:"ActivityDateInserted"` // Last watched timestamp
}

// OxiCleanarr Bridge Plugin Types
// These types are used for communicating with the Jellyfin OxiCleanarr Bridge Plugin

// PluginSymlinkItem represents a symlink to be created by the plugin
type PluginSymlinkItem struct {
	Path            string `json:"path"`             // Full path where symlink should be created
	TargetDirectory string `json:"target_directory"` // Directory containing the actual media file
}

// PluginAddSymlinksRequest represents the request to add symlinks
type PluginAddSymlinksRequest struct {
	Items  []PluginSymlinkItem `json:"items"`
	DryRun bool                `json:"dry_run,omitempty"`
}

// PluginAddSymlinksResponse represents the response from adding symlinks
type PluginAddSymlinksResponse struct {
	Success      bool     `json:"success"`
	Created      int      `json:"created"`
	Skipped      int      `json:"skipped"`
	Failed       int      `json:"failed"`
	ErrorMessage string   `json:"error_message,omitempty"`
	Details      []string `json:"details,omitempty"`
}

// PluginRemoveSymlinksRequest represents the request to remove symlinks
type PluginRemoveSymlinksRequest struct {
	Paths  []string `json:"paths"`
	DryRun bool     `json:"dry_run,omitempty"`
}

// PluginRemoveSymlinksResponse represents the response from removing symlinks
type PluginRemoveSymlinksResponse struct {
	Success      bool     `json:"success"`
	Removed      int      `json:"removed"`
	Failed       int      `json:"failed"`
	ErrorMessage string   `json:"error_message,omitempty"`
	Details      []string `json:"details,omitempty"`
}

// PluginSymlinkInfo represents information about a symlink
type PluginSymlinkInfo struct {
	Path   string `json:"path"`
	Target string `json:"target"`
	Valid  bool   `json:"valid"`
	Name   string `json:"name"` // Added in plugin API update
}

// PluginListSymlinksResponse represents the response from listing symlinks
type PluginListSymlinksResponse struct {
	Success      bool                `json:"success"`
	Symlinks     []PluginSymlinkInfo `json:"symlinks"`
	Count        int                 `json:"count"`        // Total number of symlinks
	SymlinkNames []string            `json:"symlinkNames"` // Array of just filenames
	Message      string              `json:"message"`      // Human-readable status
	ErrorMessage string              `json:"error_message,omitempty"`
}

// PluginStatusResponse represents the health check response from the plugin
type PluginStatusResponse struct {
	Success bool   `json:"success"`
	Version string `json:"version,omitempty"`
	Message string `json:"message,omitempty"`
}

// PluginCreateDirectoryRequest represents the request to create a directory
type PluginCreateDirectoryRequest struct {
	Directory string `json:"directory"`
}

// PluginCreateDirectoryResponse represents the response from directory creation
type PluginCreateDirectoryResponse struct {
	Success   bool   `json:"success"`
	Directory string `json:"directory,omitempty"`
	Created   bool   `json:"created,omitempty"`
	Message   string `json:"message,omitempty"`
}

// PluginDeleteDirectoryRequest represents the request to delete a directory
type PluginDeleteDirectoryRequest struct {
	Directory string `json:"directory"`
	Force     bool   `json:"force,omitempty"`
}

// PluginDeleteDirectoryResponse represents the response from directory deletion
type PluginDeleteDirectoryResponse struct {
	Success   bool   `json:"success"`
	Directory string `json:"directory,omitempty"`
	Deleted   bool   `json:"deleted,omitempty"`
	Message   string `json:"message,omitempty"`
}
