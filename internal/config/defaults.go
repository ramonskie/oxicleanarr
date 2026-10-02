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
		Overlay: OverlayConfig{
			Enabled:             false,
			IntervalHours:       24,
			TextTemplate:        "in {days} days",
			FontSizePercent:     5,
			FontColor:           "#ffffff",
			BackgroundColor:     "rgba(0,0,0,0.75)",
			PaddingPercent:      2,
			CornerRadiusPercent: 50,
		},
		Rules: RulesConfig{
			MovieRetention: "90d",
			TVRetention:    "120d",
		},
		Analytics: AnalyticsConfig{
			Enabled:             true,
			StaleDays:           90,
			ROIPeriodDays:       90,
			IncludeAgeDecay:     true,
			SuggestDeletionDays: 180,
			ValueThresholds: ValueThresholdsConfig{
				Movie:   ValueThreshold{Low: 0.1, High: 0.5},
				Episode: ValueThreshold{Low: 0.5, High: 2.0},
				Show:    ValueThreshold{Low: 0.3, High: 1.0},
			},
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

	// Overlay defaults
	if cfg.Overlay.IntervalHours == 0 {
		cfg.Overlay.IntervalHours = defaults.Overlay.IntervalHours
	}
	if cfg.Overlay.TextTemplate == "" {
		cfg.Overlay.TextTemplate = defaults.Overlay.TextTemplate
	}
	if cfg.Overlay.FontSizePercent == 0 {
		cfg.Overlay.FontSizePercent = defaults.Overlay.FontSizePercent
	}
	if cfg.Overlay.FontColor == "" {
		cfg.Overlay.FontColor = defaults.Overlay.FontColor
	}
	if cfg.Overlay.BackgroundColor == "" {
		cfg.Overlay.BackgroundColor = defaults.Overlay.BackgroundColor
	}
	if cfg.Overlay.PaddingPercent == 0 {
		cfg.Overlay.PaddingPercent = defaults.Overlay.PaddingPercent
	}
	if cfg.Overlay.CornerRadiusPercent == 0 {
		cfg.Overlay.CornerRadiusPercent = defaults.Overlay.CornerRadiusPercent
	}

	// Rules defaults
	if cfg.Rules.MovieRetention == "" {
		cfg.Rules.MovieRetention = defaults.Rules.MovieRetention
	}
	if cfg.Rules.TVRetention == "" {
		cfg.Rules.TVRetention = defaults.Rules.TVRetention
	}

	// Analytics defaults
	if cfg.Analytics.StaleDays == 0 {
		cfg.Analytics.StaleDays = defaults.Analytics.StaleDays
	}
	if cfg.Analytics.ROIPeriodDays == 0 {
		cfg.Analytics.ROIPeriodDays = defaults.Analytics.ROIPeriodDays
	}
	if cfg.Analytics.SuggestDeletionDays == 0 {
		cfg.Analytics.SuggestDeletionDays = defaults.Analytics.SuggestDeletionDays
	}
	if cfg.Analytics.ValueThresholds.Movie == (ValueThreshold{}) {
		cfg.Analytics.ValueThresholds.Movie = defaults.Analytics.ValueThresholds.Movie
	}
	if cfg.Analytics.ValueThresholds.Episode == (ValueThreshold{}) {
		cfg.Analytics.ValueThresholds.Episode = defaults.Analytics.ValueThresholds.Episode
	}
	if cfg.Analytics.ValueThresholds.Show == (ValueThreshold{}) {
		cfg.Analytics.ValueThresholds.Show = defaults.Analytics.ValueThresholds.Show
	}

	// Server defaults
	if cfg.Server.Host == "" {
		cfg.Server.Host = defaults.Server.Host
	}
	if cfg.Server.Port == 0 {
		cfg.Server.Port = defaults.Server.Port
	}
}
