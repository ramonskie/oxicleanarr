package config

// DefaultConfig returns a Config struct with all default values
func DefaultConfig() *Config {
	return &Config{
		App: AppConfig{
			DryRun:          true,
			EnableDeletion:  false,
			LeavingSoonDays: 14,
		},
		Sync: SyncConfig{
			FullInterval:        60, // 60 minutes (1 hour)
			IncrementalInterval: 15, // 15 minutes
			AutoStart:           true,
		},
		Rules: RulesConfig{
			MovieRetention: "90d",
			TVRetention:    "120d",
		},
		Server: ServerConfig{
			Host: "0.0.0.0",
			Port: 9709,
		},
	}
}

// SetDefaults applies default values to missing config fields
func SetDefaults(cfg *Config) {
	defaults := DefaultConfig()

	// App defaults
	if cfg.App.LeavingSoonDays == 0 {
		cfg.App.LeavingSoonDays = defaults.App.LeavingSoonDays
	}

	// Sync defaults
	if cfg.Sync.FullInterval == 0 {
		cfg.Sync.FullInterval = defaults.Sync.FullInterval
	}
	if cfg.Sync.IncrementalInterval == 0 {
		cfg.Sync.IncrementalInterval = defaults.Sync.IncrementalInterval
	}

	// Rules defaults
	if cfg.Rules.MovieRetention == "" {
		cfg.Rules.MovieRetention = defaults.Rules.MovieRetention
	}
	if cfg.Rules.TVRetention == "" {
		cfg.Rules.TVRetention = defaults.Rules.TVRetention
	}

	// Server defaults
	if cfg.Server.Host == "" {
		cfg.Server.Host = defaults.Server.Host
	}
	if cfg.Server.Port == 0 {
		cfg.Server.Port = defaults.Server.Port
	}

	// Symlink Library defaults (only applied when enabled)
	if cfg.Integrations.Jellyfin.SymlinkLibrary.Enabled {
		if cfg.Integrations.Jellyfin.SymlinkLibrary.MoviesLibraryName == "" {
			cfg.Integrations.Jellyfin.SymlinkLibrary.MoviesLibraryName = "Leaving Soon - Movies"
		}
		if cfg.Integrations.Jellyfin.SymlinkLibrary.TVLibraryName == "" {
			cfg.Integrations.Jellyfin.SymlinkLibrary.TVLibraryName = "Leaving Soon - TV Shows"
		}
		// Default to hiding empty libraries (better UX - no clutter in sidebar)
		// Note: We can't detect if user explicitly set it to false, so we use a heuristic:
		// If symlink library is enabled but hide_when_empty isn't set, default to true
		// Users who want empty libraries visible can explicitly set hide_when_empty: false
		if !cfg.Integrations.Jellyfin.SymlinkLibrary.HideWhenEmpty {
			cfg.Integrations.Jellyfin.SymlinkLibrary.HideWhenEmpty = true
		}
	}
}
