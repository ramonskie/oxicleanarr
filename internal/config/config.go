package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

var (
	globalConfig *Config
	configPath   string
)

// Load loads configuration from file and environment variables
func Load(path string) (*Config, error) {
	v := viper.New()

	// Set config file path
	if path == "" {
		path = getDefaultConfigPath()
	}
	configPath = path

	v.SetConfigFile(path)
	v.SetConfigType("yaml")

	// Environment variable support
	v.SetEnvPrefix("OXICLEANARR")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Read config file
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			log.Warn().Str("path", path).Msg("Config file not found, using defaults and environment variables")
		} else {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	// Start with defaults
	cfg := DefaultConfig()

	// Unmarshal into config struct
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Debug: Log admin config after unmarshaling
	log.Debug().
		Str("username", cfg.Admin.Username).
		Bool("has_password", cfg.Admin.Password != "").
		Bool("disable_auth", cfg.Admin.DisableAuth).
		Msg("Admin config after unmarshaling from YAML")

	// Apply defaults for any missing values
	SetDefaults(cfg)

	// Debug: Log admin config after applying defaults
	log.Debug().
		Str("username", cfg.Admin.Username).
		Bool("has_password", cfg.Admin.Password != "").
		Bool("disable_auth", cfg.Admin.DisableAuth).
		Msg("Admin config after applying defaults")

	// Validate configuration
	if err := Validate(cfg); err != nil {
		return nil, err
	}

	globalConfig = cfg
	return cfg, nil
}

// Get returns the global config instance
func Get() *Config {
	return globalConfig
}

// SetTestConfig sets a test config (for testing only - bypasses validation)
// This should only be used in test files
func SetTestConfig(cfg *Config) {
	globalConfig = cfg
}

// GetPath returns the current config file path
func GetPath() string {
	return configPath
}

// Reload reloads the configuration from disk
func Reload() error {
	log.Info().Msg("Reloading configuration from disk")
	cfg, err := Load(configPath)
	if err != nil {
		return err
	}
	globalConfig = cfg
	log.Info().
		Str("movie_retention", cfg.Rules.MovieRetention).
		Str("tv_retention", cfg.Rules.TVRetention).
		Bool("dry_run", cfg.App.DryRun).
		Msg("Configuration reloaded successfully")
	return nil
}

// getDefaultConfigPath returns the default config file path
func getDefaultConfigPath() string {
	// Check environment variable first
	if path := os.Getenv("OXICLEANARR_CONFIG_PATH"); path != "" {
		return path
	}

	// Default paths to check
	paths := []string{
		"./config/config.yaml",
		"/app/config/config.yaml",
		"./config.yaml",
	}

	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	// Return first default if none exist
	return paths[0]
}
