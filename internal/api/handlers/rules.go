package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/ramonskie/oxicleanarr/internal/config"
	"github.com/rs/zerolog/log"
)

// RulesHandler handles advanced rules management requests
type RulesHandler struct{}

// NewRulesHandler creates a new RulesHandler
func NewRulesHandler() *RulesHandler {
	return &RulesHandler{}
}

// ListRules handles GET /api/rules
func (h *RulesHandler) ListRules(w http.ResponseWriter, r *http.Request) {
	cfg := config.Get()
	if cfg == nil {
		log.Error().Msg("Config not initialized")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "Config not initialized"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"rules": cfg.AdvancedRules,
	})
}

// CreateRuleRequest represents a request to create a new rule
type CreateRuleRequest struct {
	Name           string            `json:"name"`
	Type           string            `json:"type"`
	Enabled        bool              `json:"enabled"`
	Tag            string            `json:"tag,omitempty"`
	Retention      string            `json:"retention,omitempty"`
	MaxEpisodes    int               `json:"max_episodes,omitempty"`
	MaxAge         string            `json:"max_age,omitempty"`
	RequireWatched bool              `json:"require_watched,omitempty"`
	Users          []config.UserRule `json:"users,omitempty"`
}

// CreateRule handles POST /api/rules
func (h *RulesHandler) CreateRule(w http.ResponseWriter, r *http.Request) {
	var req CreateRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Error().Err(err).Msg("Failed to decode create rule request")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "Invalid request body"})
		return
	}

	// Validate rule
	rule := config.AdvancedRule{
		Name:           req.Name,
		Type:           req.Type,
		Enabled:        req.Enabled,
		Tag:            req.Tag,
		Retention:      req.Retention,
		MaxEpisodes:    req.MaxEpisodes,
		MaxAge:         req.MaxAge,
		RequireWatched: req.RequireWatched,
		Users:          req.Users,
	}

	if err := validateRule(&rule); err != nil {
		log.Error().Err(err).Msg("Rule validation failed")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{Error: err.Error()})
		return
	}

	cfg := config.Get()
	if cfg == nil {
		log.Error().Msg("Config not initialized")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "Config not initialized"})
		return
	}

	// Check for duplicate rule name
	for _, existingRule := range cfg.AdvancedRules {
		if existingRule.Name == rule.Name {
			log.Error().Str("name", rule.Name).Msg("Rule with this name already exists")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(ErrorResponse{Error: "Rule with this name already exists"})
			return
		}
	}

	// Add rule to config
	newCfg := *cfg
	newCfg.AdvancedRules = append(newCfg.AdvancedRules, rule)

	// Validate the entire config
	if err := config.Validate(&newCfg); err != nil {
		log.Error().Err(err).Msg("Config validation failed")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{Error: err.Error()})
		return
	}

	// Write to config file
	if err := writeConfigToFile(&newCfg); err != nil {
		log.Error().Err(err).Msg("Failed to write config to file")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "Failed to save rule"})
		return
	}

	// Reload config
	if err := config.Reload(); err != nil {
		log.Error().Err(err).Msg("Failed to reload config")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "Failed to reload configuration"})
		return
	}

	log.Info().Str("name", rule.Name).Msg("Rule created successfully")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(rule)
}

// UpdateRule handles PUT /api/rules/{name}
func (h *RulesHandler) UpdateRule(w http.ResponseWriter, r *http.Request) {
	ruleName := chi.URLParam(r, "name")
	if ruleName == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "Rule name is required"})
		return
	}

	var req CreateRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Error().Err(err).Msg("Failed to decode update rule request")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "Invalid request body"})
		return
	}

	cfg := config.Get()
	if cfg == nil {
		log.Error().Msg("Config not initialized")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "Config not initialized"})
		return
	}

	// Find the rule to update
	ruleIndex := -1
	for i, rule := range cfg.AdvancedRules {
		if rule.Name == ruleName {
			ruleIndex = i
			break
		}
	}

	if ruleIndex == -1 {
		log.Error().Str("name", ruleName).Msg("Rule not found")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "Rule not found"})
		return
	}

	// Create updated rule
	updatedRule := config.AdvancedRule{
		Name:           req.Name,
		Type:           req.Type,
		Enabled:        req.Enabled,
		Tag:            req.Tag,
		Retention:      req.Retention,
		MaxEpisodes:    req.MaxEpisodes,
		MaxAge:         req.MaxAge,
		RequireWatched: req.RequireWatched,
		Users:          req.Users,
	}

	// Validate rule
	if err := validateRule(&updatedRule); err != nil {
		log.Error().Err(err).Msg("Rule validation failed")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{Error: err.Error()})
		return
	}

	// Update rule in config
	newCfg := *cfg
	newCfg.AdvancedRules[ruleIndex] = updatedRule

	// Validate the entire config
	if err := config.Validate(&newCfg); err != nil {
		log.Error().Err(err).Msg("Config validation failed")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{Error: err.Error()})
		return
	}

	// Write to config file
	if err := writeConfigToFile(&newCfg); err != nil {
		log.Error().Err(err).Msg("Failed to write config to file")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "Failed to save rule"})
		return
	}

	// Reload config
	if err := config.Reload(); err != nil {
		log.Error().Err(err).Msg("Failed to reload config")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "Failed to reload configuration"})
		return
	}

	log.Info().Str("name", updatedRule.Name).Msg("Rule updated successfully")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(updatedRule)
}

// DeleteRule handles DELETE /api/rules/{name}
func (h *RulesHandler) DeleteRule(w http.ResponseWriter, r *http.Request) {
	ruleName := chi.URLParam(r, "name")
	if ruleName == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "Rule name is required"})
		return
	}

	cfg := config.Get()
	if cfg == nil {
		log.Error().Msg("Config not initialized")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "Config not initialized"})
		return
	}

	// Find the rule to delete
	ruleIndex := -1
	for i, rule := range cfg.AdvancedRules {
		if rule.Name == ruleName {
			ruleIndex = i
			break
		}
	}

	if ruleIndex == -1 {
		log.Error().Str("name", ruleName).Msg("Rule not found")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "Rule not found"})
		return
	}

	// Remove rule from config
	newCfg := *cfg
	newCfg.AdvancedRules = append(newCfg.AdvancedRules[:ruleIndex], newCfg.AdvancedRules[ruleIndex+1:]...)

	// Write to config file
	if err := writeConfigToFile(&newCfg); err != nil {
		log.Error().Err(err).Msg("Failed to write config to file")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "Failed to delete rule"})
		return
	}

	// Reload config
	if err := config.Reload(); err != nil {
		log.Error().Err(err).Msg("Failed to reload config")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "Failed to reload configuration"})
		return
	}

	log.Info().Str("name", ruleName).Msg("Rule deleted successfully")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Rule deleted successfully"})
}

// ToggleRuleRequest represents a request to toggle a rule's enabled state
type ToggleRuleRequest struct {
	Enabled bool `json:"enabled"`
}

// ToggleRule handles PATCH /api/rules/{name}/toggle
func (h *RulesHandler) ToggleRule(w http.ResponseWriter, r *http.Request) {
	ruleName := chi.URLParam(r, "name")
	if ruleName == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "Rule name is required"})
		return
	}

	var req ToggleRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Error().Err(err).Msg("Failed to decode toggle rule request")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "Invalid request body"})
		return
	}

	cfg := config.Get()
	if cfg == nil {
		log.Error().Msg("Config not initialized")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "Config not initialized"})
		return
	}

	// Find the rule to toggle
	ruleIndex := -1
	for i, rule := range cfg.AdvancedRules {
		if rule.Name == ruleName {
			ruleIndex = i
			break
		}
	}

	if ruleIndex == -1 {
		log.Error().Str("name", ruleName).Msg("Rule not found")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "Rule not found"})
		return
	}

	// Update rule enabled state
	newCfg := *cfg
	newCfg.AdvancedRules[ruleIndex].Enabled = req.Enabled

	// Write to config file
	if err := writeConfigToFile(&newCfg); err != nil {
		log.Error().Err(err).Msg("Failed to write config to file")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "Failed to toggle rule"})
		return
	}

	// Reload config
	if err := config.Reload(); err != nil {
		log.Error().Err(err).Msg("Failed to reload config")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "Failed to reload configuration"})
		return
	}

	log.Info().Str("name", ruleName).Bool("enabled", req.Enabled).Msg("Rule toggled successfully")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(newCfg.AdvancedRules[ruleIndex])
}

// validateRule validates a rule configuration
func validateRule(rule *config.AdvancedRule) error {
	if rule.Name == "" {
		return ErrInvalidInput{Field: "name", Message: "Rule name is required"}
	}

	validTypes := map[string]bool{
		"tag":     true,
		"episode": true,
		"user":    true,
	}

	if !validTypes[rule.Type] {
		return ErrInvalidInput{Field: "type", Message: "Rule type must be 'tag', 'episode', or 'user'"}
	}

	// Type-specific validation
	switch rule.Type {
	case "tag":
		if rule.Tag == "" {
			return ErrInvalidInput{Field: "tag", Message: "Tag is required for tag-based rules"}
		}
		if rule.Retention == "" {
			return ErrInvalidInput{Field: "retention", Message: "Retention is required for tag-based rules"}
		}
	case "episode":
		if rule.MaxEpisodes <= 0 && rule.MaxAge == "" {
			return ErrInvalidInput{Field: "max_episodes", Message: "Either max_episodes or max_age is required for episode rules"}
		}
	case "user":
		if len(rule.Users) == 0 {
			return ErrInvalidInput{Field: "users", Message: "At least one user is required for user-based rules"}
		}
		for i, user := range rule.Users {
			if user.UserID == nil && user.Username == "" && user.Email == "" {
				return ErrInvalidInput{
					Field:   "users",
					Message: "Each user must have at least one identifier (user_id, username, or email)",
					Index:   &i,
				}
			}
			if user.Retention == "" {
				return ErrInvalidInput{
					Field:   "users",
					Message: "Retention is required for each user",
					Index:   &i,
				}
			}
		}
	}

	return nil
}

// ErrInvalidInput represents an invalid input error
type ErrInvalidInput struct {
	Field   string
	Message string
	Index   *int
}

func (e ErrInvalidInput) Error() string {
	if e.Index != nil {
		return e.Field + "[" + string(rune(*e.Index)) + "]: " + e.Message
	}
	return e.Field + ": " + e.Message
}
