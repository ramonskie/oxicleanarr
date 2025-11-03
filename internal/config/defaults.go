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
			FullInterval:        3600,
			IncrementalInterval: 900,
			AutoStart:           true,
		},
		Rules: RulesConfig{
			MovieRetention: "90d",
			TVRetention:    "120d",
		},
		Server: ServerConfig{
			Host: "0.0.0.0",
			Port: 8080,
		},
		Integrations: IntegrationsConfig{
			Jellyfin: JellyfinConfig{
				LeavingSoonType: "MOVIES_AND_TV",
			},
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

	// Jellyfin defaults
	if cfg.Integrations.Jellyfin.LeavingSoonType == "" {
		cfg.Integrations.Jellyfin.LeavingSoonType = defaults.Integrations.Jellyfin.LeavingSoonType
	}
}
