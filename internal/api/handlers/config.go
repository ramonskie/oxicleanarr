package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"

	"github.com/ramonskie/prunarr/internal/config"
	"github.com/ramonskie/prunarr/internal/services"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

// ConfigHandler handles configuration management requests
type ConfigHandler struct {
	syncEngine *services.SyncEngine
}

// NewConfigHandler creates a new ConfigHandler
func NewConfigHandler(syncEngine *services.SyncEngine) *ConfigHandler {
	return &ConfigHandler{
		syncEngine: syncEngine,
	}
}

// SanitizedConfig represents a sanitized version of the config (without passwords)
type SanitizedConfig struct {
	Admin         SanitizedAdminConfig        `json:"admin"`
	App           config.AppConfig            `json:"app"`
	Sync          config.SyncConfig           `json:"sync"`
	Rules         config.RulesConfig          `json:"rules"`
	Server        config.ServerConfig         `json:"server"`
	Integrations  SanitizedIntegrationsConfig `json:"integrations"`
	AdvancedRules []config.AdvancedRule       `json:"advanced_rules"`
}

// SanitizedAdminConfig holds admin config without password
type SanitizedAdminConfig struct {
	Username    string `json:"username"`
	DisableAuth bool   `json:"disable_auth"`
}

// SanitizedIntegrationsConfig holds sanitized integration configs
type SanitizedIntegrationsConfig struct {
	Jellyfin   SanitizedJellyfinConfig        `json:"jellyfin"`
	Radarr     SanitizedBaseIntegrationConfig `json:"radarr"`
	Sonarr     SanitizedBaseIntegrationConfig `json:"sonarr"`
	Jellyseerr SanitizedBaseIntegrationConfig `json:"jellyseerr"`
	Jellystat  SanitizedBaseIntegrationConfig `json:"jellystat"`
}

// SanitizedBaseIntegrationConfig holds sanitized base integration config
type SanitizedBaseIntegrationConfig struct {
	Enabled   bool   `json:"enabled"`
	URL       string `json:"url"`
	HasAPIKey bool   `json:"has_api_key"`
	Timeout   string `json:"timeout"`
}

// SanitizedJellyfinConfig holds sanitized Jellyfin config
type SanitizedJellyfinConfig struct {
	Enabled         bool                     `json:"enabled"`
	URL             string                   `json:"url"`
	HasAPIKey       bool                     `json:"has_api_key"`
	Timeout         string                   `json:"timeout"`
	Username        string                   `json:"username"`
	HasPassword     bool                     `json:"has_password"`
	LeavingSoonType string                   `json:"leaving_soon_type"`
	Collections     config.CollectionsConfig `json:"collections"`
}

// GetConfig handles GET /api/config
func (h *ConfigHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	cfg := config.Get()
	if cfg == nil {
		log.Error().Msg("Config not initialized")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "Config not initialized"})
		return
	}

	// Sanitize config (remove passwords and API keys)
	sanitized := SanitizedConfig{
		Admin: SanitizedAdminConfig{
			Username:    cfg.Admin.Username,
			DisableAuth: cfg.Admin.DisableAuth,
		},
		App:           cfg.App,
		Sync:          cfg.Sync,
		Rules:         cfg.Rules,
		Server:        cfg.Server,
		AdvancedRules: cfg.AdvancedRules,
		Integrations: SanitizedIntegrationsConfig{
			Jellyfin: SanitizedJellyfinConfig{
				Enabled:         cfg.Integrations.Jellyfin.Enabled,
				URL:             cfg.Integrations.Jellyfin.URL,
				HasAPIKey:       cfg.Integrations.Jellyfin.APIKey != "",
				Timeout:         cfg.Integrations.Jellyfin.Timeout,
				Username:        cfg.Integrations.Jellyfin.Username,
				HasPassword:     cfg.Integrations.Jellyfin.Password != "",
				LeavingSoonType: cfg.Integrations.Jellyfin.LeavingSoonType,
				Collections:     cfg.Integrations.Jellyfin.Collections,
			},
			Radarr: SanitizedBaseIntegrationConfig{
				Enabled:   cfg.Integrations.Radarr.Enabled,
				URL:       cfg.Integrations.Radarr.URL,
				HasAPIKey: cfg.Integrations.Radarr.APIKey != "",
				Timeout:   cfg.Integrations.Radarr.Timeout,
			},
			Sonarr: SanitizedBaseIntegrationConfig{
				Enabled:   cfg.Integrations.Sonarr.Enabled,
				URL:       cfg.Integrations.Sonarr.URL,
				HasAPIKey: cfg.Integrations.Sonarr.APIKey != "",
				Timeout:   cfg.Integrations.Sonarr.Timeout,
			},
			Jellyseerr: SanitizedBaseIntegrationConfig{
				Enabled:   cfg.Integrations.Jellyseerr.Enabled,
				URL:       cfg.Integrations.Jellyseerr.URL,
				HasAPIKey: cfg.Integrations.Jellyseerr.APIKey != "",
				Timeout:   cfg.Integrations.Jellyseerr.Timeout,
			},
			Jellystat: SanitizedBaseIntegrationConfig{
				Enabled:   cfg.Integrations.Jellystat.Enabled,
				URL:       cfg.Integrations.Jellystat.URL,
				HasAPIKey: cfg.Integrations.Jellystat.APIKey != "",
				Timeout:   cfg.Integrations.Jellystat.Timeout,
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(sanitized)
}

// UpdateConfigRequest represents a config update request
type UpdateConfigRequest struct {
	Admin         *UpdateAdminConfig        `json:"admin,omitempty"`
	App           *config.AppConfig         `json:"app,omitempty"`
	Sync          *config.SyncConfig        `json:"sync,omitempty"`
	Rules         *config.RulesConfig       `json:"rules,omitempty"`
	Server        *config.ServerConfig      `json:"server,omitempty"`
	Integrations  *UpdateIntegrationsConfig `json:"integrations,omitempty"`
	AdvancedRules *[]config.AdvancedRule    `json:"advanced_rules,omitempty"`
}

// UpdateAdminConfig holds updatable admin config
type UpdateAdminConfig struct {
	Username    *string `json:"username,omitempty"`
	Password    *string `json:"password,omitempty"`
	DisableAuth *bool   `json:"disable_auth,omitempty"`
}

// UpdateIntegrationsConfig holds updatable integration configs
type UpdateIntegrationsConfig struct {
	Jellyfin   *UpdateJellyfinConfig        `json:"jellyfin,omitempty"`
	Radarr     *UpdateBaseIntegrationConfig `json:"radarr,omitempty"`
	Sonarr     *UpdateBaseIntegrationConfig `json:"sonarr,omitempty"`
	Jellyseerr *UpdateBaseIntegrationConfig `json:"jellyseerr,omitempty"`
	Jellystat  *UpdateBaseIntegrationConfig `json:"jellystat,omitempty"`
}

// UpdateBaseIntegrationConfig holds updatable base integration config
type UpdateBaseIntegrationConfig struct {
	Enabled *bool   `json:"enabled,omitempty"`
	URL     *string `json:"url,omitempty"`
	APIKey  *string `json:"api_key,omitempty"`
	Timeout *string `json:"timeout,omitempty"`
}

// UpdateJellyfinConfig holds updatable Jellyfin config
type UpdateJellyfinConfig struct {
	Enabled         *bool                     `json:"enabled,omitempty"`
	URL             *string                   `json:"url,omitempty"`
	APIKey          *string                   `json:"api_key,omitempty"`
	Timeout         *string                   `json:"timeout,omitempty"`
	Username        *string                   `json:"username,omitempty"`
	Password        *string                   `json:"password,omitempty"`
	LeavingSoonType *string                   `json:"leaving_soon_type,omitempty"`
	Collections     *config.CollectionsConfig `json:"collections,omitempty"`
}

// UpdateConfig handles PUT /api/config
func (h *ConfigHandler) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	var req UpdateConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Error().Err(err).Msg("Failed to decode update config request")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "Invalid request body"})
		return
	}

	// Get current config
	cfg := config.Get()
	if cfg == nil {
		log.Error().Msg("Config not initialized")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "Config not initialized"})
		return
	}

	// Create a new config with updated values
	newCfg := *cfg

	// Update fields if provided
	if req.Admin != nil {
		if req.Admin.Username != nil {
			newCfg.Admin.Username = *req.Admin.Username
		}
		if req.Admin.Password != nil {
			newCfg.Admin.Password = *req.Admin.Password
		}
		if req.Admin.DisableAuth != nil {
			newCfg.Admin.DisableAuth = *req.Admin.DisableAuth
		}
	}

	if req.App != nil {
		newCfg.App = *req.App
	}

	if req.Sync != nil {
		newCfg.Sync = *req.Sync
	}

	if req.Rules != nil {
		newCfg.Rules = *req.Rules
	}

	if req.Server != nil {
		newCfg.Server = *req.Server
	}

	if req.Integrations != nil {
		if req.Integrations.Jellyfin != nil {
			if req.Integrations.Jellyfin.Enabled != nil {
				newCfg.Integrations.Jellyfin.Enabled = *req.Integrations.Jellyfin.Enabled
			}
			if req.Integrations.Jellyfin.URL != nil {
				newCfg.Integrations.Jellyfin.URL = *req.Integrations.Jellyfin.URL
			}
			if req.Integrations.Jellyfin.APIKey != nil {
				newCfg.Integrations.Jellyfin.APIKey = *req.Integrations.Jellyfin.APIKey
			}
			if req.Integrations.Jellyfin.Timeout != nil {
				newCfg.Integrations.Jellyfin.Timeout = *req.Integrations.Jellyfin.Timeout
			}
			if req.Integrations.Jellyfin.Username != nil {
				newCfg.Integrations.Jellyfin.Username = *req.Integrations.Jellyfin.Username
			}
			if req.Integrations.Jellyfin.Password != nil {
				newCfg.Integrations.Jellyfin.Password = *req.Integrations.Jellyfin.Password
			}
			if req.Integrations.Jellyfin.LeavingSoonType != nil {
				newCfg.Integrations.Jellyfin.LeavingSoonType = *req.Integrations.Jellyfin.LeavingSoonType
			}
			if req.Integrations.Jellyfin.Collections != nil {
				newCfg.Integrations.Jellyfin.Collections = *req.Integrations.Jellyfin.Collections
			}
		}

		if req.Integrations.Radarr != nil {
			if req.Integrations.Radarr.Enabled != nil {
				newCfg.Integrations.Radarr.Enabled = *req.Integrations.Radarr.Enabled
			}
			if req.Integrations.Radarr.URL != nil {
				newCfg.Integrations.Radarr.URL = *req.Integrations.Radarr.URL
			}
			if req.Integrations.Radarr.APIKey != nil {
				newCfg.Integrations.Radarr.APIKey = *req.Integrations.Radarr.APIKey
			}
			if req.Integrations.Radarr.Timeout != nil {
				newCfg.Integrations.Radarr.Timeout = *req.Integrations.Radarr.Timeout
			}
		}

		if req.Integrations.Sonarr != nil {
			if req.Integrations.Sonarr.Enabled != nil {
				newCfg.Integrations.Sonarr.Enabled = *req.Integrations.Sonarr.Enabled
			}
			if req.Integrations.Sonarr.URL != nil {
				newCfg.Integrations.Sonarr.URL = *req.Integrations.Sonarr.URL
			}
			if req.Integrations.Sonarr.APIKey != nil {
				newCfg.Integrations.Sonarr.APIKey = *req.Integrations.Sonarr.APIKey
			}
			if req.Integrations.Sonarr.Timeout != nil {
				newCfg.Integrations.Sonarr.Timeout = *req.Integrations.Sonarr.Timeout
			}
		}

		if req.Integrations.Jellyseerr != nil {
			if req.Integrations.Jellyseerr.Enabled != nil {
				newCfg.Integrations.Jellyseerr.Enabled = *req.Integrations.Jellyseerr.Enabled
			}
			if req.Integrations.Jellyseerr.URL != nil {
				newCfg.Integrations.Jellyseerr.URL = *req.Integrations.Jellyseerr.URL
			}
			if req.Integrations.Jellyseerr.APIKey != nil {
				newCfg.Integrations.Jellyseerr.APIKey = *req.Integrations.Jellyseerr.APIKey
			}
			if req.Integrations.Jellyseerr.Timeout != nil {
				newCfg.Integrations.Jellyseerr.Timeout = *req.Integrations.Jellyseerr.Timeout
			}
		}

		if req.Integrations.Jellystat != nil {
			if req.Integrations.Jellystat.Enabled != nil {
				newCfg.Integrations.Jellystat.Enabled = *req.Integrations.Jellystat.Enabled
			}
			if req.Integrations.Jellystat.URL != nil {
				newCfg.Integrations.Jellystat.URL = *req.Integrations.Jellystat.URL
			}
			if req.Integrations.Jellystat.APIKey != nil {
				newCfg.Integrations.Jellystat.APIKey = *req.Integrations.Jellystat.APIKey
			}
			if req.Integrations.Jellystat.Timeout != nil {
				newCfg.Integrations.Jellystat.Timeout = *req.Integrations.Jellystat.Timeout
			}
		}
	}

	if req.AdvancedRules != nil {
		newCfg.AdvancedRules = *req.AdvancedRules
	}

	// Validate the new config
	if err := config.Validate(&newCfg); err != nil {
		log.Error().Err(err).Msg("Config validation failed")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{Error: err.Error()})
		return
	}

	log.Info().Int("leaving_soon_days", newCfg.App.LeavingSoonDays).Msg("About to write config to file")

	// Write to config file
	if err := writeConfigToFile(&newCfg); err != nil {
		log.Error().Err(err).Msg("Failed to write config to file")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "Failed to save configuration"})
		return
	}

	log.Info().Msg("Write config completed successfully")

	// Track if retention rules changed (requires full sync to re-evaluate)
	retentionChanged := false
	if req.Rules != nil {
		oldCfg := config.Get()
		if oldCfg != nil {
			if req.Rules.MovieRetention != oldCfg.Rules.MovieRetention ||
				req.Rules.TVRetention != oldCfg.Rules.TVRetention {
				retentionChanged = true
				log.Info().Msg("Retention rules changed, will trigger full sync to re-evaluate media")
			}
		}
	}
	if req.AdvancedRules != nil {
		retentionChanged = true
		log.Info().Msg("Advanced rules changed, will trigger full sync to re-evaluate media")
	}

	// Reload config to apply changes
	if err := config.Reload(); err != nil {
		log.Error().Err(err).Msg("Failed to reload config")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "Failed to reload configuration"})
		return
	}

	// Trigger full sync if retention rules changed
	if retentionChanged && h.syncEngine != nil {
		log.Info().Msg("Triggering full sync to re-apply retention rules")
		go func() {
			ctx := context.Background()
			if err := h.syncEngine.FullSync(ctx); err != nil {
				log.Error().Err(err).Msg("Failed to trigger full sync after config update")
			} else {
				log.Info().Msg("Full sync completed after config update")
			}
		}()
	}

	log.Info().Msg("Configuration updated successfully")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Configuration updated successfully"})
}

// writeConfigToFile writes the config to the YAML file
func writeConfigToFile(cfg *config.Config) error {
	// Get the config file path from the loaded config
	configPath := config.GetPath()
	if configPath == "" {
		// Fallback to default if not set (shouldn't happen)
		configPath = "./config/prunarr.yaml"
	}

	log.Info().Str("path", configPath).Msg("Writing config to file")

	// Marshal config to YAML
	data, err := yaml.Marshal(cfg)
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal config to YAML")
		return err
	}

	// Add a header comment
	header := "# Prunarr Configuration\n# Generated by Prunarr Web UI\n\n"
	content := header + string(data)

	// Write to file (preserve original permissions or use 0600 for new files)
	info, err := os.Stat(configPath)
	perm := os.FileMode(0600)
	if err == nil {
		perm = info.Mode()
	}

	// Ensure directory exists (extract directory from full path)
	dir := filepath.Dir(configPath)
	log.Info().Str("dir", dir).Msg("Ensuring directory exists")
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Error().Err(err).Str("dir", dir).Msg("Failed to create directory")
		return err
	}

	log.Info().Str("path", configPath).Int("bytes", len(content)).Msg("Writing file")
	if err := os.WriteFile(configPath, []byte(content), perm); err != nil {
		log.Error().Err(err).Str("path", configPath).Msg("Failed to write config file")
		return err
	}

	log.Info().Str("path", configPath).Msg("Config file written successfully")
	return nil
}
