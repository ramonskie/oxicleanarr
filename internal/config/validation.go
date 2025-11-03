package config

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

var durationRegex = regexp.MustCompile(`^\d+[dhms]$`)

// ValidationError represents a configuration validation error
type ValidationError struct {
	Field   string
	Message string
}

// ValidationErrors is a collection of validation errors
type ValidationErrors []ValidationError

func (v ValidationErrors) Error() string {
	if len(v) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("Configuration validation failed:\n")
	for _, err := range v {
		sb.WriteString(fmt.Sprintf("  - %s: %s\n", err.Field, err.Message))
	}
	return sb.String()
}

// Validate validates the configuration and returns all errors found
func Validate(cfg *Config) error {
	var errors ValidationErrors

	// Validate admin credentials
	if cfg.Admin.Username == "" {
		errors = append(errors, ValidationError{
			Field:   "admin.username",
			Message: "required",
		})
	}
	if cfg.Admin.Password == "" {
		errors = append(errors, ValidationError{
			Field:   "admin.password",
			Message: "required",
		})
	}

	// Validate at least one integration enabled
	hasIntegration := cfg.Integrations.Jellyfin.Enabled ||
		cfg.Integrations.Radarr.Enabled ||
		cfg.Integrations.Sonarr.Enabled ||
		cfg.Integrations.Jellyseerr.Enabled ||
		cfg.Integrations.Jellystat.Enabled

	if !hasIntegration {
		errors = append(errors, ValidationError{
			Field:   "integrations",
			Message: "at least one integration must be enabled",
		})
	}

	// Validate Jellyfin
	if cfg.Integrations.Jellyfin.Enabled {
		errors = validateIntegration(errors, "integrations.jellyfin", cfg.Integrations.Jellyfin.URL, cfg.Integrations.Jellyfin.APIKey)

		// Validate collections config
		if cfg.Integrations.Jellyfin.Collections.Enabled {
			if cfg.Integrations.Jellyfin.Collections.Movies.Name == "" {
				errors = append(errors, ValidationError{
					Field:   "integrations.jellyfin.collections.movies.name",
					Message: "required when collections.enabled=true",
				})
			}
			if cfg.Integrations.Jellyfin.Collections.TVShows.Name == "" {
				errors = append(errors, ValidationError{
					Field:   "integrations.jellyfin.collections.tv_shows.name",
					Message: "required when collections.enabled=true",
				})
			}
		}
	}

	// Validate Radarr
	if cfg.Integrations.Radarr.Enabled {
		errors = validateIntegration(errors, "integrations.radarr", cfg.Integrations.Radarr.URL, cfg.Integrations.Radarr.APIKey)
	}

	// Validate Sonarr
	if cfg.Integrations.Sonarr.Enabled {
		errors = validateIntegration(errors, "integrations.sonarr", cfg.Integrations.Sonarr.URL, cfg.Integrations.Sonarr.APIKey)
	}

	// Validate Jellyseerr
	if cfg.Integrations.Jellyseerr.Enabled {
		errors = validateIntegration(errors, "integrations.jellyseerr", cfg.Integrations.Jellyseerr.URL, cfg.Integrations.Jellyseerr.APIKey)
	}

	// Validate Jellystat
	if cfg.Integrations.Jellystat.Enabled {
		errors = validateIntegration(errors, "integrations.jellystat", cfg.Integrations.Jellystat.URL, cfg.Integrations.Jellystat.APIKey)
	}

	// Validate duration formats
	if !isValidDuration(cfg.Rules.MovieRetention) {
		errors = append(errors, ValidationError{
			Field:   "rules.movie_retention",
			Message: fmt.Sprintf("invalid duration format %q (use formats like '30d', '1h', '90d')", cfg.Rules.MovieRetention),
		})
	}
	if !isValidDuration(cfg.Rules.TVRetention) {
		errors = append(errors, ValidationError{
			Field:   "rules.tv_retention",
			Message: fmt.Sprintf("invalid duration format %q (use formats like '30d', '1h', '120d')", cfg.Rules.TVRetention),
		})
	}

	// Validate advanced rules
	for i, rule := range cfg.AdvancedRules {
		if rule.Enabled {
			prefix := fmt.Sprintf("advanced_rules[%d]", i)
			if rule.Retention != "" && !isValidDuration(rule.Retention) {
				errors = append(errors, ValidationError{
					Field:   fmt.Sprintf("%s.retention", prefix),
					Message: fmt.Sprintf("invalid duration format %q", rule.Retention),
				})
			}
			if rule.MaxAge != "" && !isValidDuration(rule.MaxAge) {
				errors = append(errors, ValidationError{
					Field:   fmt.Sprintf("%s.max_age", prefix),
					Message: fmt.Sprintf("invalid duration format %q", rule.MaxAge),
				})
			}

			// Validate user rules
			if rule.Type == "user" {
				for j, user := range rule.Users {
					userPrefix := fmt.Sprintf("%s.users[%d]", prefix, j)
					if user.UserID == nil && user.Username == "" && user.Email == "" {
						errors = append(errors, ValidationError{
							Field:   userPrefix,
							Message: "at least one identifier (user_id, username, or email) required",
						})
					}
					if user.Retention == "" {
						errors = append(errors, ValidationError{
							Field:   fmt.Sprintf("%s.retention", userPrefix),
							Message: "required",
						})
					} else if !isValidDuration(user.Retention) {
						errors = append(errors, ValidationError{
							Field:   fmt.Sprintf("%s.retention", userPrefix),
							Message: fmt.Sprintf("invalid duration format %q", user.Retention),
						})
					}
				}

				// Validate require_watched setting
				if rule.RequireWatched && !cfg.Integrations.Jellystat.Enabled {
					errors = append(errors, ValidationError{
						Field:   fmt.Sprintf("%s.require_watched", prefix),
						Message: "require_watched=true requires Jellystat integration to be enabled",
					})
				}
			}
		}
	}

	// Validate port range
	if cfg.Server.Port < 1 || cfg.Server.Port > 65535 {
		errors = append(errors, ValidationError{
			Field:   "server.port",
			Message: fmt.Sprintf("must be between 1 and 65535 (got %d)", cfg.Server.Port),
		})
	}

	if len(errors) > 0 {
		return errors
	}
	return nil
}

// validateIntegration validates URL and API key for an integration
func validateIntegration(errors ValidationErrors, prefix, urlStr, apiKey string) ValidationErrors {
	if urlStr == "" {
		errors = append(errors, ValidationError{
			Field:   fmt.Sprintf("%s.url", prefix),
			Message: "required when enabled=true",
		})
	} else {
		_, err := url.Parse(urlStr)
		if err != nil {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("%s.url", prefix),
				Message: fmt.Sprintf("must be a valid URL (got: %q)", urlStr),
			})
		}
	}

	if apiKey == "" {
		errors = append(errors, ValidationError{
			Field:   fmt.Sprintf("%s.api_key", prefix),
			Message: "required when enabled=true",
		})
	}

	return errors
}

// isValidDuration checks if a duration string is valid (e.g., "30d", "1h", "90d")
// Special values "never" and "0d" are allowed to disable retention rules
func isValidDuration(duration string) bool {
	// Allow special values for disabling retention
	if duration == "never" || duration == "0d" {
		return true
	}
	return durationRegex.MatchString(duration)
}
