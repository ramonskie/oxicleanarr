package config

import (
	"strings"
	"testing"
)

func TestValidate_UserRules_RequiresAtLeastOneIdentifier(t *testing.T) {
	userID := 123

	tests := []struct {
		name        string
		userRule    UserRule
		shouldError bool
	}{
		{
			name: "valid with user_id only",
			userRule: UserRule{
				UserID:    &userID,
				Retention: "30d",
			},
			shouldError: false,
		},
		{
			name: "valid with username only",
			userRule: UserRule{
				Username:  "john_doe",
				Retention: "30d",
			},
			shouldError: false,
		},
		{
			name: "valid with email only",
			userRule: UserRule{
				Email:     "user@example.com",
				Retention: "30d",
			},
			shouldError: false,
		},
		{
			name: "valid with multiple identifiers",
			userRule: UserRule{
				UserID:    &userID,
				Email:     "user@example.com",
				Retention: "30d",
			},
			shouldError: false,
		},
		{
			name: "invalid - no identifiers",
			userRule: UserRule{
				Retention: "30d",
			},
			shouldError: true,
		},
		{
			name: "invalid - no retention",
			userRule: UserRule{
				UserID: &userID,
			},
			shouldError: true,
		},
		{
			name: "invalid - bad retention format",
			userRule: UserRule{
				UserID:    &userID,
				Retention: "bad",
			},
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Admin: AdminConfig{
					Username: "admin",
					Password: "pass",
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
						BaseIntegrationConfig: BaseIntegrationConfig{
							Enabled: true,
							URL:     "http://jellyfin:8096",
							APIKey:  "test-key",
						},
					},
				},
				AdvancedRules: []AdvancedRule{
					{
						Name:    "Test Rule",
						Type:    "user",
						Enabled: true,
						Users:   []UserRule{tt.userRule},
					},
				},
			}

			err := Validate(cfg)
			if tt.shouldError && err == nil {
				t.Errorf("expected validation error but got none")
			}
			if !tt.shouldError && err != nil {
				t.Errorf("unexpected validation error: %v", err)
			}

			// Check for specific error message when no identifiers provided
			if tt.name == "invalid - no identifiers" && err != nil {
				if !strings.Contains(err.Error(), "at least one identifier") {
					t.Errorf("expected 'at least one identifier' error, got: %v", err)
				}
			}
		})
	}
}

func TestValidate_UserRules_RetentionFormat(t *testing.T) {
	userID := 123

	validFormats := []string{"1d", "7d", "30d", "90d", "1h", "24h", "30m", "60m"}
	invalidFormats := []string{"1", "d", "30days", "1week", "bad", ""}

	for _, format := range validFormats {
		t.Run("valid_"+format, func(t *testing.T) {
			cfg := &Config{
				Admin: AdminConfig{
					Username: "admin",
					Password: "pass",
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
						BaseIntegrationConfig: BaseIntegrationConfig{
							Enabled: true,
							URL:     "http://jellyfin:8096",
							APIKey:  "test-key",
						},
					},
				},
				AdvancedRules: []AdvancedRule{
					{
						Name:    "Test Rule",
						Type:    "user",
						Enabled: true,
						Users: []UserRule{
							{
								UserID:    &userID,
								Retention: format,
							},
						},
					},
				},
			}

			err := Validate(cfg)
			if err != nil {
				t.Errorf("valid format %q should not error, got: %v", format, err)
			}
		})
	}

	for _, format := range invalidFormats {
		t.Run("invalid_"+format, func(t *testing.T) {
			cfg := &Config{
				Admin: AdminConfig{
					Username: "admin",
					Password: "pass",
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
						BaseIntegrationConfig: BaseIntegrationConfig{
							Enabled: true,
							URL:     "http://jellyfin:8096",
							APIKey:  "test-key",
						},
					},
				},
				AdvancedRules: []AdvancedRule{
					{
						Name:    "Test Rule",
						Type:    "user",
						Enabled: true,
						Users: []UserRule{
							{
								UserID:    &userID,
								Retention: format,
							},
						},
					},
				},
			}

			err := Validate(cfg)
			if err == nil {
				t.Errorf("invalid format %q should error", format)
			}
		})
	}
}

func TestValidate_DisabledRetention_MovieAndTV(t *testing.T) {
	tests := []struct {
		name           string
		movieRetention string
		tvRetention    string
		shouldError    bool
		errorContains  string
	}{
		{
			name:           "both retention disabled with 'never'",
			movieRetention: "never",
			tvRetention:    "never",
			shouldError:    false,
		},
		{
			name:           "both retention disabled with '0d'",
			movieRetention: "0d",
			tvRetention:    "0d",
			shouldError:    false,
		},
		{
			name:           "movie retention disabled with 'never', TV normal",
			movieRetention: "never",
			tvRetention:    "120d",
			shouldError:    false,
		},
		{
			name:           "movie retention disabled with '0d', TV normal",
			movieRetention: "0d",
			tvRetention:    "120d",
			shouldError:    false,
		},
		{
			name:           "TV retention disabled with 'never', movie normal",
			movieRetention: "90d",
			tvRetention:    "never",
			shouldError:    false,
		},
		{
			name:           "TV retention disabled with '0d', movie normal",
			movieRetention: "90d",
			tvRetention:    "0d",
			shouldError:    false,
		},
		{
			name:           "mixed - 'never' and '0d'",
			movieRetention: "never",
			tvRetention:    "0d",
			shouldError:    false,
		},
		{
			name:           "mixed - '0d' and 'never'",
			movieRetention: "0d",
			tvRetention:    "never",
			shouldError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Admin: AdminConfig{
					Username: "admin",
					Password: "pass",
				},
				Rules: RulesConfig{
					MovieRetention: tt.movieRetention,
					TVRetention:    tt.tvRetention,
				},
				Server: ServerConfig{
					Host: "0.0.0.0",
					Port: 8080,
				},
				Integrations: IntegrationsConfig{
					Jellyfin: JellyfinConfig{
						BaseIntegrationConfig: BaseIntegrationConfig{
							Enabled: true,
							URL:     "http://jellyfin:8096",
							APIKey:  "test-key",
						},
					},
				},
			}

			err := Validate(cfg)
			if tt.shouldError && err == nil {
				t.Errorf("expected validation error but got none")
			}
			if !tt.shouldError && err != nil {
				t.Errorf("unexpected validation error: %v", err)
			}
			if tt.shouldError && tt.errorContains != "" && err != nil {
				if !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("expected error containing %q, got: %v", tt.errorContains, err)
				}
			}
		})
	}
}

func TestValidate_DisabledRetention_AdvancedRules(t *testing.T) {
	userID := 123

	tests := []struct {
		name          string
		ruleRetention string
		shouldError   bool
	}{
		{
			name:          "advanced rule with 'never' retention",
			ruleRetention: "never",
			shouldError:   false,
		},
		{
			name:          "advanced rule with '0d' retention",
			ruleRetention: "0d",
			shouldError:   false,
		},
		{
			name:          "user rule with 'never' retention",
			ruleRetention: "never",
			shouldError:   false,
		},
		{
			name:          "user rule with '0d' retention",
			ruleRetention: "0d",
			shouldError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Admin: AdminConfig{
					Username: "admin",
					Password: "pass",
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
						BaseIntegrationConfig: BaseIntegrationConfig{
							Enabled: true,
							URL:     "http://jellyfin:8096",
							APIKey:  "test-key",
						},
					},
				},
				AdvancedRules: []AdvancedRule{
					{
						Name:    "Test Rule",
						Type:    "user",
						Enabled: true,
						Users: []UserRule{
							{
								UserID:    &userID,
								Retention: tt.ruleRetention,
							},
						},
					},
				},
			}

			err := Validate(cfg)
			if tt.shouldError && err == nil {
				t.Errorf("expected validation error but got none")
			}
			if !tt.shouldError && err != nil {
				t.Errorf("unexpected validation error: %v", err)
			}
		})
	}
}
