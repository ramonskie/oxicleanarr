package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
	"golang.org/x/crypto/bcrypt"
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
	v.SetEnvPrefix("PRUNARR")
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

	// Auto-hash plain-text password
	if err := hashPasswordIfNeeded(cfg); err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
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
	if path := os.Getenv("PRUNARR_CONFIG_PATH"); path != "" {
		return path
	}

	// Default paths to check
	paths := []string{
		"./config/prunarr.yaml",
		"/app/config/prunarr.yaml",
		"./prunarr.yaml",
	}

	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	// Return first default if none exist
	return paths[0]
}

// hashPasswordIfNeeded checks if the password is plain-text and hashes it
func hashPasswordIfNeeded(cfg *Config) error {
	// Check if password looks like a bcrypt hash
	if strings.HasPrefix(cfg.Admin.Password, "$2a$") || strings.HasPrefix(cfg.Admin.Password, "$2b$") {
		// Already hashed
		return nil
	}

	// Hash the plain-text password
	log.Warn().Msg("Plain-text password detected and auto-hashed")
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(cfg.Admin.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	cfg.Admin.Password = string(hashedPassword)

	// Write back to config file
	if err := writePasswordToConfig(configPath, string(hashedPassword)); err != nil {
		log.Warn().Err(err).Msg("Failed to write hashed password back to config file")
		// Not a fatal error - the in-memory config is updated
	}

	return nil
}

// writePasswordToConfig writes the hashed password back to the config file
func writePasswordToConfig(path string, hashedPassword string) error {
	// Create config directory if it doesn't exist
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Read the config file
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Create a minimal config file
			minimalConfig := fmt.Sprintf(`admin:
  username: %s
  password: %s
`, globalConfig.Admin.Username, hashedPassword)
			return os.WriteFile(path, []byte(minimalConfig), 0600)
		}
		return err
	}

	// Simple replacement of the password line
	// This is a basic implementation - for production, consider using a YAML library
	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "password:") {
			// Preserve indentation
			indent := ""
			for _, ch := range line {
				if ch == ' ' || ch == '\t' {
					indent += string(ch)
				} else {
					break
				}
			}
			lines[i] = fmt.Sprintf("%spassword: %s", indent, hashedPassword)
			break
		}
	}

	// Write back to file
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0600)
}
