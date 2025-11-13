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
	DryRun          bool `mapstructure:"dry_run" yaml:"dry_run" json:"dry_run"`
	EnableDeletion  bool `mapstructure:"enable_deletion" yaml:"enable_deletion" json:"enable_deletion"`
	LeavingSoonDays int  `mapstructure:"leaving_soon_days" yaml:"leaving_soon_days" json:"leaving_soon_days"`
}

// SyncConfig holds sync scheduler settings
type SyncConfig struct {
	FullInterval        int  `mapstructure:"full_interval" yaml:"full_interval" json:"full_interval"`
	IncrementalInterval int  `mapstructure:"incremental_interval" yaml:"incremental_interval" json:"incremental_interval"`
	AutoStart           bool `mapstructure:"auto_start" yaml:"auto_start" json:"auto_start"`
}

// RulesConfig holds simple retention rules
type RulesConfig struct {
	MovieRetention string `mapstructure:"movie_retention" yaml:"movie_retention" json:"movie_retention"`
	TVRetention    string `mapstructure:"tv_retention" yaml:"tv_retention" json:"tv_retention"`
}

// ServerConfig holds HTTP server settings
type ServerConfig struct {
	Host string `mapstructure:"host" yaml:"host" json:"host"`
	Port int    `mapstructure:"port" yaml:"port" json:"port"`
}

// IntegrationsConfig holds all integration settings
type IntegrationsConfig struct {
	Jellyfin   JellyfinConfig   `mapstructure:"jellyfin" yaml:"jellyfin" json:"jellyfin"`
	Radarr     RadarrConfig     `mapstructure:"radarr" yaml:"radarr" json:"radarr"`
	Sonarr     SonarrConfig     `mapstructure:"sonarr" yaml:"sonarr" json:"sonarr"`
	Jellyseerr JellyseerrConfig `mapstructure:"jellyseerr" yaml:"jellyseerr" json:"jellyseerr"`
	Jellystat  JellystatConfig  `mapstructure:"jellystat" yaml:"jellystat" json:"jellystat"`
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

// AdvancedRule represents tag-based, episode, or user-based rules
type AdvancedRule struct {
	Name           string     `mapstructure:"name" yaml:"name" json:"name"`
	Type           string     `mapstructure:"type" yaml:"type" json:"type"`
	Enabled        bool       `mapstructure:"enabled" yaml:"enabled" json:"enabled"`
	Tag            string     `mapstructure:"tag" yaml:"tag,omitempty" json:"tag,omitempty"`
	Retention      string     `mapstructure:"retention" yaml:"retention,omitempty" json:"retention,omitempty"`
	MaxEpisodes    int        `mapstructure:"max_episodes" yaml:"max_episodes,omitempty" json:"max_episodes,omitempty"`
	MaxAge         string     `mapstructure:"max_age" yaml:"max_age,omitempty" json:"max_age,omitempty"`
	RequireWatched bool       `mapstructure:"require_watched" yaml:"require_watched,omitempty" json:"require_watched,omitempty"`
	Users          []UserRule `mapstructure:"users" yaml:"users,omitempty" json:"users,omitempty"`
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
