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
	ID             int              `json:"id"`
	Title          string           `json:"title"`
	Year           int              `json:"year"`
	Added          time.Time        `json:"added"`
	Path           string           `json:"path"`
	SizeOnDisk     int64            `json:"sizeOnDisk"`
	HasFile        bool             `json:"hasFile"`
	QualityProfile RadarrQuality    `json:"qualityProfileId"`
	TmdbId         int              `json:"tmdbId"`
	MovieFile      *RadarrMovieFile `json:"movieFile,omitempty"`
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
	TvdbId     int         `json:"tvdbId"`
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
	ID        int             `json:"id"`
	Type      string          `json:"type"`
	Status    int             `json:"status"`
	Media     JellyseerrMedia `json:"media"`
	CreatedAt time.Time       `json:"createdAt"`
}

// JellyseerrMedia represents media in a Jellyseerr request
type JellyseerrMedia struct {
	TmdbId int `json:"tmdbId"`
	TvdbId int `json:"tvdbId"`
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

// JellystatActivity represents watch activity from Jellystat
type JellystatActivity struct {
	ItemID     string    `json:"item_id"`
	UserID     string    `json:"user_id"`
	PlayCount  int       `json:"play_count"`
	LastPlayed time.Time `json:"last_played"`
}
