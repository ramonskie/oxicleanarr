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
