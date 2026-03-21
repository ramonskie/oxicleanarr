package config

// Config represents the complete application configuration
type Config struct {
	Admin         AdminConfig        `mapstructure:"admin" yaml:"admin" json:"admin"`
	App           AppConfig          `mapstructure:"app" yaml:"app" json:"app"`
	Sync          SyncConfig         `mapstructure:"sync" yaml:"sync" json:"sync"`
	Rules         RulesConfig        `mapstructure:"rules" yaml:"rules" json:"rules"`
	Server        ServerConfig       `mapstructure:"server" yaml:"server" json:"server"`
	Integrations  IntegrationsConfig `mapstructure:"integrations" yaml:"integrations" json:"integrations"`
	AdvancedRules []AdvancedRule     `mapstructure:"advanced_rules" yaml:"advanced_rules,omitempty" json:"advanced_rules,omitempty"`
}

// AdminConfig holds admin user credentials
type AdminConfig struct {
	Username    string `mapstructure:"username" yaml:"username" json:"username"`
	Password    string `mapstructure:"password" yaml:"password" json:"password"`
	DisableAuth bool   `mapstructure:"disable_auth" yaml:"disable_auth" json:"disable_auth"`
}

// AppConfig holds general application settings
type AppConfig struct {
	DryRun          bool                `mapstructure:"dry_run" yaml:"dry_run" json:"dry_run"`
	EnableDeletion  bool                `mapstructure:"enable_deletion" yaml:"enable_deletion" json:"enable_deletion"`
	LeavingSoonDays int                 `mapstructure:"leaving_soon_days" yaml:"leaving_soon_days" json:"leaving_soon_days"`
	DiskThreshold   DiskThresholdConfig `mapstructure:"disk_threshold" yaml:"disk_threshold,omitempty" json:"disk_threshold,omitempty"`
}

// DiskThresholdConfig holds disk-space threshold settings for conditional rule activation
type DiskThresholdConfig struct {
	Enabled     bool   `mapstructure:"enabled" yaml:"enabled" json:"enabled"`
	FreeSpaceGB int    `mapstructure:"free_space_gb" yaml:"free_space_gb" json:"free_space_gb"`
	CheckSource string `mapstructure:"check_source" yaml:"check_source,omitempty" json:"check_source,omitempty"` // "radarr" (default), "sonarr", "lowest"
}

// SyncConfig holds sync scheduler settings
type SyncConfig struct {
	FullInterval        int  `mapstructure:"full_interval" yaml:"full_interval" json:"full_interval"`
	IncrementalInterval int  `mapstructure:"incremental_interval" yaml:"incremental_interval" json:"incremental_interval"`
	AutoStart           bool `mapstructure:"auto_start" yaml:"auto_start" json:"auto_start"`
}

// RulesConfig holds simple retention rules
type RulesConfig struct {
	MovieRetention     string `mapstructure:"movie_retention" yaml:"movie_retention" json:"movie_retention"`
	TVRetention        string `mapstructure:"tv_retention" yaml:"tv_retention" json:"tv_retention"`
	RetentionBase      string `mapstructure:"retention_base" yaml:"retention_base,omitempty" json:"retention_base,omitempty"`                // "last_watched_or_added" (default), "last_watched", "added"
	UnwatchedBehavior  string `mapstructure:"unwatched_behavior" yaml:"unwatched_behavior,omitempty" json:"unwatched_behavior,omitempty"`    // "added" (default), "never"
	UnwatchedRetention string `mapstructure:"unwatched_retention" yaml:"unwatched_retention,omitempty" json:"unwatched_retention,omitempty"` // separate retention for unwatched items (only when retention_base=last_watched AND unwatched_behavior=added)
}

// ServerConfig holds HTTP server settings
type ServerConfig struct {
	Host string `mapstructure:"host" yaml:"host" json:"host"`
	Port int    `mapstructure:"port" yaml:"port" json:"port"`
}

// IntegrationsConfig holds all integration settings
type IntegrationsConfig struct {
	Jellyfin     JellyfinConfig     `mapstructure:"jellyfin" yaml:"jellyfin" json:"jellyfin"`
	Radarr       RadarrConfig       `mapstructure:"radarr" yaml:"radarr" json:"radarr"`
	Sonarr       SonarrConfig       `mapstructure:"sonarr" yaml:"sonarr" json:"sonarr"`
	Jellyseerr   JellyseerrConfig   `mapstructure:"jellyseerr" yaml:"jellyseerr" json:"jellyseerr"`
	Jellystat    JellystatConfig    `mapstructure:"jellystat" yaml:"jellystat" json:"jellystat"`
	Streamystats StreamystatsConfig `mapstructure:"streamystats" yaml:"streamystats" json:"streamystats"`
}

// BaseIntegrationConfig holds common integration settings
type BaseIntegrationConfig struct {
	Enabled bool   `mapstructure:"enabled" yaml:"enabled" json:"enabled"`
	URL     string `mapstructure:"url" yaml:"url" json:"url"`
	APIKey  string `mapstructure:"api_key" yaml:"api_key" json:"api_key"`
	Timeout string `mapstructure:"timeout" yaml:"timeout" json:"timeout"`
}

// JellyfinConfig holds Jellyfin integration settings
type JellyfinConfig struct {
	BaseIntegrationConfig `mapstructure:",squash" yaml:",inline" json:",inline"`
	SymlinkLibrary        SymlinkLibraryConfig `mapstructure:"symlink_library" yaml:"symlink_library" json:"symlink_library"`
}

// SymlinkLibraryConfig holds symlink-based library management settings
type SymlinkLibraryConfig struct {
	Enabled           bool   `mapstructure:"enabled" yaml:"enabled" json:"enabled"`
	BasePath          string `mapstructure:"base_path" yaml:"base_path" json:"base_path"`                                         // Base directory for symlinks (e.g., /data/media/prunarr-leaving-soon)
	MoviesLibraryName string `mapstructure:"movies_library_name" yaml:"movies_library_name,omitempty" json:"movies_library_name"` // Jellyfin library name for movies (default: "Leaving Soon - Movies")
	TVLibraryName     string `mapstructure:"tv_library_name" yaml:"tv_library_name,omitempty" json:"tv_library_name"`             // Jellyfin library name for TV shows (default: "Leaving Soon - TV Shows")
	HideWhenEmpty     bool   `mapstructure:"hide_when_empty" yaml:"hide_when_empty" json:"hide_when_empty"`                       // Automatically delete libraries when no items are scheduled (prevents empty libraries in sidebar)
}

// RadarrConfig holds Radarr integration settings
type RadarrConfig struct {
	BaseIntegrationConfig `mapstructure:",squash" yaml:",inline" json:",inline"`
}

// SonarrConfig holds Sonarr integration settings
type SonarrConfig struct {
	BaseIntegrationConfig `mapstructure:",squash" yaml:",inline" json:",inline"`
}

// JellyseerrConfig holds Jellyseerr integration settings
type JellyseerrConfig struct {
	BaseIntegrationConfig `mapstructure:",squash" yaml:",inline" json:",inline"`
}

// JellystatConfig holds Jellystat integration settings
type JellystatConfig struct {
	BaseIntegrationConfig `mapstructure:",squash" yaml:",inline" json:",inline"`
}

// StreamystatsConfig holds Streamystats integration settings.
// ServerID is the Streamystats server UUID (required when enabled).
// APIKey should be set to the Jellyfin API key — Streamystats validates it
// live against the Jellyfin /System/Info endpoint.
type StreamystatsConfig struct {
	BaseIntegrationConfig `mapstructure:",squash" yaml:",inline" json:",inline"`
	ServerID              string `mapstructure:"server_id" yaml:"server_id" json:"server_id"`
}

// AdvancedRule represents tag-based, episode, or user-based rules
type AdvancedRule struct {
	Name              string     `mapstructure:"name" yaml:"name" json:"name"`
	Type              string     `mapstructure:"type" yaml:"type" json:"type"`
	Enabled           bool       `mapstructure:"enabled" yaml:"enabled" json:"enabled"`
	Tag               string     `mapstructure:"tag" yaml:"tag,omitempty" json:"tag,omitempty"`
	Retention         string     `mapstructure:"retention" yaml:"retention,omitempty" json:"retention,omitempty"`
	RetentionBase     string     `mapstructure:"retention_base" yaml:"retention_base,omitempty" json:"retention_base,omitempty"`             // per-rule override: "last_watched_or_added", "last_watched", "added"
	UnwatchedBehavior string     `mapstructure:"unwatched_behavior" yaml:"unwatched_behavior,omitempty" json:"unwatched_behavior,omitempty"` // per-rule override: "added", "never"
	MaxEpisodes       int        `mapstructure:"max_episodes" yaml:"max_episodes,omitempty" json:"max_episodes,omitempty"`
	MaxAge            string     `mapstructure:"max_age" yaml:"max_age,omitempty" json:"max_age,omitempty"`
	RequireWatched    bool       `mapstructure:"require_watched" yaml:"require_watched,omitempty" json:"require_watched,omitempty"`
	Users             []UserRule `mapstructure:"users" yaml:"users,omitempty" json:"users,omitempty"`

	// Episode-specific fields (only valid when Type="episode")
	SeasonNumbers           []int  `mapstructure:"season_numbers" yaml:"season_numbers,omitempty" json:"season_numbers,omitempty"`
	ExcludeContinuingSeries bool   `mapstructure:"exclude_continuing_series" yaml:"exclude_continuing_series,omitempty" json:"exclude_continuing_series,omitempty"`
	KeepLatestSeason        bool   `mapstructure:"keep_latest_season" yaml:"keep_latest_season,omitempty" json:"keep_latest_season,omitempty"`
	EpisodeDeleteStrategy   string `mapstructure:"episode_delete_strategy" yaml:"episode_delete_strategy,omitempty" json:"episode_delete_strategy,omitempty"` // "oldest_first", "by_age", "by_season_age"
}

// UserRule represents a user-based cleanup rule
// Note: Only ONE identifier (UserID, Username, OR Email) is required per rule.
// Multiple identifiers can be provided for redundancy but are not necessary.
// Matching is case-insensitive for Username and Email.
type UserRule struct {
	UserID         *int   `mapstructure:"user_id" yaml:"user_id,omitempty" json:"user_id,omitempty"`                         // Jellyseerr user ID (most reliable)
	Username       string `mapstructure:"username" yaml:"username,omitempty" json:"username,omitempty"`                      // Jellyseerr username (case-insensitive)
	Email          string `mapstructure:"email" yaml:"email,omitempty" json:"email,omitempty"`                               // User email address (case-insensitive)
	Retention      string `mapstructure:"retention" yaml:"retention" json:"retention"`                                       // Required: duration format (e.g., "7d", "30d")
	RequireWatched *bool  `mapstructure:"require_watched" yaml:"require_watched,omitempty" json:"require_watched,omitempty"` // Optional: only delete if user watched the content
}
