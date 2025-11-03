package config

// Config represents the complete application configuration
type Config struct {
	Admin         AdminConfig        `mapstructure:"admin"`
	App           AppConfig          `mapstructure:"app"`
	Sync          SyncConfig         `mapstructure:"sync"`
	Rules         RulesConfig        `mapstructure:"rules"`
	Server        ServerConfig       `mapstructure:"server"`
	Integrations  IntegrationsConfig `mapstructure:"integrations"`
	AdvancedRules []AdvancedRule     `mapstructure:"advanced_rules"`
}

// AdminConfig holds admin user credentials
type AdminConfig struct {
	Username    string `mapstructure:"username"`
	Password    string `mapstructure:"password"`
	DisableAuth bool   `mapstructure:"disable_auth"`
}

// AppConfig holds general application settings
type AppConfig struct {
	DryRun          bool `mapstructure:"dry_run"`
	LeavingSoonDays int  `mapstructure:"leaving_soon_days"`
}

// SyncConfig holds sync scheduler settings
type SyncConfig struct {
	FullInterval        int  `mapstructure:"full_interval"`
	IncrementalInterval int  `mapstructure:"incremental_interval"`
	AutoStart           bool `mapstructure:"auto_start"`
}

// RulesConfig holds simple retention rules
type RulesConfig struct {
	MovieRetention string `mapstructure:"movie_retention"`
	TVRetention    string `mapstructure:"tv_retention"`
}

// ServerConfig holds HTTP server settings
type ServerConfig struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
}

// IntegrationsConfig holds all integration settings
type IntegrationsConfig struct {
	Jellyfin   JellyfinConfig   `mapstructure:"jellyfin"`
	Radarr     RadarrConfig     `mapstructure:"radarr"`
	Sonarr     SonarrConfig     `mapstructure:"sonarr"`
	Jellyseerr JellyseerrConfig `mapstructure:"jellyseerr"`
	Jellystat  JellystatConfig  `mapstructure:"jellystat"`
}

// BaseIntegrationConfig holds common integration settings
type BaseIntegrationConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	URL     string `mapstructure:"url"`
	APIKey  string `mapstructure:"api_key"`
	Timeout string `mapstructure:"timeout"`
}

// JellyfinConfig holds Jellyfin integration settings
type JellyfinConfig struct {
	BaseIntegrationConfig `mapstructure:",squash"`
	Username              string            `mapstructure:"username"`
	Password              string            `mapstructure:"password"`
	LeavingSoonType       string            `mapstructure:"leaving_soon_type"`
	Collections           CollectionsConfig `mapstructure:"collections"`
}

// CollectionsConfig holds Jellyfin collection management settings
type CollectionsConfig struct {
	Enabled bool                 `mapstructure:"enabled"`
	Movies  CollectionItemConfig `mapstructure:"movies"`
	TVShows CollectionItemConfig `mapstructure:"tv_shows"`
}

// CollectionItemConfig holds settings for a specific collection
type CollectionItemConfig struct {
	Name          string `mapstructure:"name"`
	HideWhenEmpty bool   `mapstructure:"hide_when_empty"`
}

// RadarrConfig holds Radarr integration settings
type RadarrConfig struct {
	BaseIntegrationConfig `mapstructure:",squash"`
}

// SonarrConfig holds Sonarr integration settings
type SonarrConfig struct {
	BaseIntegrationConfig `mapstructure:",squash"`
}

// JellyseerrConfig holds Jellyseerr integration settings
type JellyseerrConfig struct {
	BaseIntegrationConfig `mapstructure:",squash"`
}

// JellystatConfig holds Jellystat integration settings
type JellystatConfig struct {
	BaseIntegrationConfig `mapstructure:",squash"`
}

// AdvancedRule represents tag-based, episode, or user-based rules
type AdvancedRule struct {
	Name           string     `mapstructure:"name"`
	Type           string     `mapstructure:"type"`
	Enabled        bool       `mapstructure:"enabled"`
	Tag            string     `mapstructure:"tag"`
	Retention      string     `mapstructure:"retention"`
	MaxEpisodes    int        `mapstructure:"max_episodes"`
	MaxAge         string     `mapstructure:"max_age"`
	RequireWatched bool       `mapstructure:"require_watched"`
	Users          []UserRule `mapstructure:"users"`
}

// UserRule represents a user-based cleanup rule
// Note: Only ONE identifier (UserID, Username, OR Email) is required per rule.
// Multiple identifiers can be provided for redundancy but are not necessary.
// Matching is case-insensitive for Username and Email.
type UserRule struct {
	UserID         *int   `mapstructure:"user_id"`         // Jellyseerr user ID (most reliable)
	Username       string `mapstructure:"username"`        // Jellyseerr username (case-insensitive)
	Email          string `mapstructure:"email"`           // User email address (case-insensitive)
	Retention      string `mapstructure:"retention"`       // Required: duration format (e.g., "7d", "30d")
	RequireWatched *bool  `mapstructure:"require_watched"` // Optional: only delete if user watched the content
}
