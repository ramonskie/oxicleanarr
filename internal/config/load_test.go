package config

import (
	"os"
	"testing"
)

func TestLoad_SimplifiedUserRules(t *testing.T) {
	// Create a temporary config file with simplified user rules
	configContent := `
admin:
  username: admin
  password: changeme

app:
  dry_run: true

rules:
  movie_retention: 90d
  tv_retention: 120d

integrations:
  jellyfin:
    enabled: true
    url: http://localhost:8096
    api_key: test-key

advanced_rules:
  - name: Simple User Rule - ID Only
    type: user
    enabled: true
    users:
      - user_id: 1
        retention: 30d
  
  - name: Simple User Rule - Email Only
    type: user
    enabled: true
    users:
      - email: guest@example.com
        retention: 7d
  
  - name: Simple User Rule - Username Only
    type: user
    enabled: true
    users:
      - username: trial_user
        retention: 14d
        require_watched: true
`

	// Write to temporary file
	tmpfile, err := os.CreateTemp("", "prunarr-test-config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(configContent)); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	// Load and validate config
	cfg, err := Load(tmpfile.Name())
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify advanced rules loaded correctly
	if len(cfg.AdvancedRules) != 3 {
		t.Fatalf("Expected 3 advanced rules, got %d", len(cfg.AdvancedRules))
	}

	// Test Rule 1: User ID only
	rule1 := cfg.AdvancedRules[0]
	if rule1.Name != "Simple User Rule - ID Only" {
		t.Errorf("Expected rule name 'Simple User Rule - ID Only', got %s", rule1.Name)
	}
	if len(rule1.Users) != 1 {
		t.Fatalf("Expected 1 user in rule 1, got %d", len(rule1.Users))
	}
	user1 := rule1.Users[0]
	if user1.UserID == nil || *user1.UserID != 1 {
		t.Errorf("Expected user_id 1, got %v", user1.UserID)
	}
	if user1.Username != "" {
		t.Errorf("Expected empty username, got %s", user1.Username)
	}
	if user1.Email != "" {
		t.Errorf("Expected empty email, got %s", user1.Email)
	}
	if user1.Retention != "30d" {
		t.Errorf("Expected retention '30d', got %s", user1.Retention)
	}

	// Test Rule 2: Email only
	rule2 := cfg.AdvancedRules[1]
	if rule2.Name != "Simple User Rule - Email Only" {
		t.Errorf("Expected rule name 'Simple User Rule - Email Only', got %s", rule2.Name)
	}
	if len(rule2.Users) != 1 {
		t.Fatalf("Expected 1 user in rule 2, got %d", len(rule2.Users))
	}
	user2 := rule2.Users[0]
	if user2.UserID != nil {
		t.Errorf("Expected nil user_id, got %v", *user2.UserID)
	}
	if user2.Username != "" {
		t.Errorf("Expected empty username, got %s", user2.Username)
	}
	if user2.Email != "guest@example.com" {
		t.Errorf("Expected email 'guest@example.com', got %s", user2.Email)
	}
	if user2.Retention != "7d" {
		t.Errorf("Expected retention '7d', got %s", user2.Retention)
	}

	// Test Rule 3: Username only with require_watched
	rule3 := cfg.AdvancedRules[2]
	if rule3.Name != "Simple User Rule - Username Only" {
		t.Errorf("Expected rule name 'Simple User Rule - Username Only', got %s", rule3.Name)
	}
	if len(rule3.Users) != 1 {
		t.Fatalf("Expected 1 user in rule 3, got %d", len(rule3.Users))
	}
	user3 := rule3.Users[0]
	if user3.UserID != nil {
		t.Errorf("Expected nil user_id, got %v", *user3.UserID)
	}
	if user3.Username != "trial_user" {
		t.Errorf("Expected username 'trial_user', got %s", user3.Username)
	}
	if user3.Email != "" {
		t.Errorf("Expected empty email, got %s", user3.Email)
	}
	if user3.Retention != "14d" {
		t.Errorf("Expected retention '14d', got %s", user3.Retention)
	}
	if user3.RequireWatched == nil || !*user3.RequireWatched {
		t.Errorf("Expected require_watched to be true, got %v", user3.RequireWatched)
	}

	t.Log("✅ All simplified user rules loaded correctly!")
}
