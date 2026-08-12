package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/ramonskie/oxicleanarr/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// loadTestConfig creates a temp config file, loads it into the global config,
// and returns its path. The config is valid so the rule/config handlers can
// re-validate the written config and persist via writeConfigToFile + Reload.
func loadTestConfig(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := `admin:
  username: admin
  password: adminpassword
integrations:
  jellyfin:
    enabled: true
    url: http://jellyfin:8096
    api_key: test-key
rules:
  movie_retention: 90d
  tv_retention: 120d
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0600))
	_, err := config.Load(path)
	require.NoError(t, err)
	return path
}

// withRuleName attaches a chi route context so URLParam("name") resolves.
func withRuleName(r *http.Request, name string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("name", name)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

// seedRules writes the given rules to the config file and reloads it, so rule
// handlers operate against a known non-empty rule set.
func seedRules(t *testing.T, rules []config.AdvancedRule) {
	t.Helper()
	cfg := config.Get()
	require.NotNil(t, cfg, "config must be loaded before seeding rules")
	newCfg := cloneConfigWithRules(cfg)
	newCfg.AdvancedRules = rules
	require.NoError(t, writeConfigToFile(newCfg))
	require.NoError(t, config.Reload())
}

func TestRulesHandler_ListRules(t *testing.T) {
	loadTestConfig(t)
	seedRules(t, []config.AdvancedRule{
		{Name: "rule-a", Type: "tag", Enabled: true, Tag: "ta", Retention: "30d"},
	})

	handler := NewRulesHandler()
	req := httptest.NewRequest(http.MethodGet, "/api/rules", nil)
	rec := httptest.NewRecorder()

	handler.ListRules(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		Rules []config.AdvancedRule `json:"rules"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Len(t, body.Rules, 1)
	assert.Equal(t, "rule-a", body.Rules[0].Name)
}

func TestRulesHandler_ListRules_ConfigNil(t *testing.T) {
	config.SetTestConfig(nil)

	handler := NewRulesHandler()
	req := httptest.NewRequest(http.MethodGet, "/api/rules", nil)
	rec := httptest.NewRecorder()

	handler.ListRules(rec, req)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestRulesHandler_CreateRule(t *testing.T) {
	loadTestConfig(t)
	handler := NewRulesHandler()

	t.Run("creates valid tag rule", func(t *testing.T) {
		body := `{"name":"test-tag","type":"tag","enabled":true,"tag":"mytag","retention":"30d"}`
		req := httptest.NewRequest(http.MethodPost, "/api/rules", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.CreateRule(rec, req)
		require.Equal(t, http.StatusCreated, rec.Code)

		// The rule must survive the write-to-file + reload round trip.
		rules := config.Get().AdvancedRules
		require.Len(t, rules, 1)
		assert.Equal(t, "test-tag", rules[0].Name)
		assert.Equal(t, "tag", rules[0].Type)
		assert.Equal(t, "mytag", rules[0].Tag)
		assert.True(t, rules[0].Enabled)
	})

	t.Run("rejects duplicate name", func(t *testing.T) {
		body := `{"name":"test-tag","type":"tag","tag":"other","retention":"30d"}`
		req := httptest.NewRequest(http.MethodPost, "/api/rules", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.CreateRule(rec, req)
		assert.Equal(t, http.StatusConflict, rec.Code)
	})

	t.Run("rejects missing name", func(t *testing.T) {
		body := `{"type":"tag","tag":"x","retention":"30d"}`
		req := httptest.NewRequest(http.MethodPost, "/api/rules", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.CreateRule(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), "name")
	})

	t.Run("rejects invalid type", func(t *testing.T) {
		body := `{"name":"bad","type":"movie","retention":"30d"}`
		req := httptest.NewRequest(http.MethodPost, "/api/rules", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.CreateRule(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), "type")
	})

	t.Run("rejects tag rule without tag", func(t *testing.T) {
		body := `{"name":"bad","type":"tag","retention":"30d"}`
		req := httptest.NewRequest(http.MethodPost, "/api/rules", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.CreateRule(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), "tag")
	})

	t.Run("rejects malformed body", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/rules", strings.NewReader(`{`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.CreateRule(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

func TestRulesHandler_UpdateRule(t *testing.T) {
	loadTestConfig(t)
	seedRules(t, []config.AdvancedRule{
		{Name: "rule-a", Type: "tag", Enabled: true, Tag: "ta", Retention: "30d"},
		{Name: "rule-b", Type: "tag", Enabled: true, Tag: "tb", Retention: "60d"},
	})
	handler := NewRulesHandler()

	t.Run("updates existing rule", func(t *testing.T) {
		body := `{"name":"rule-a","type":"tag","enabled":false,"tag":"newtag","retention":"10d"}`
		req := httptest.NewRequest(http.MethodPut, "/api/rules/rule-a", strings.NewReader(body))
		req = withRuleName(req, "rule-a")
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.UpdateRule(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)

		var updated config.AdvancedRule
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &updated))
		assert.Equal(t, "newtag", updated.Tag)
		assert.False(t, updated.Enabled)

		// Persisted after reload.
		rules := config.Get().AdvancedRules
		require.Len(t, rules, 2)
		assert.Equal(t, "newtag", rules[0].Tag)
		assert.False(t, rules[0].Enabled)
	})

	t.Run("rejects rename onto existing rule", func(t *testing.T) {
		// Rename rule-a -> rule-b, which already exists.
		body := `{"name":"rule-b","type":"tag","tag":"ta","retention":"30d"}`
		req := httptest.NewRequest(http.MethodPut, "/api/rules/rule-a", strings.NewReader(body))
		req = withRuleName(req, "rule-a")
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.UpdateRule(rec, req)
		assert.Equal(t, http.StatusConflict, rec.Code)
	})

	t.Run("returns 404 for missing rule", func(t *testing.T) {
		body := `{"name":"nope","type":"tag","tag":"t","retention":"30d"}`
		req := httptest.NewRequest(http.MethodPut, "/api/rules/nope", strings.NewReader(body))
		req = withRuleName(req, "nope")
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.UpdateRule(rec, req)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("rejects malformed body", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/api/rules/rule-a", strings.NewReader(`{`))
		req = withRuleName(req, "rule-a")
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.UpdateRule(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

func TestRulesHandler_DeleteRule(t *testing.T) {
	loadTestConfig(t)
	seedRules(t, []config.AdvancedRule{
		{Name: "rule-a", Type: "tag", Enabled: true, Tag: "ta", Retention: "30d"},
	})
	handler := NewRulesHandler()

	t.Run("deletes existing rule", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/rules/rule-a", nil)
		req = withRuleName(req, "rule-a")
		rec := httptest.NewRecorder()

		handler.DeleteRule(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)
		assert.Empty(t, config.Get().AdvancedRules, "rule must be removed after reload")
	})

	t.Run("returns 404 for missing rule", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/rules/rule-a", nil)
		req = withRuleName(req, "rule-a")
		rec := httptest.NewRecorder()

		handler.DeleteRule(rec, req)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}

func TestRulesHandler_ToggleRule(t *testing.T) {
	loadTestConfig(t)
	seedRules(t, []config.AdvancedRule{
		{Name: "rule-a", Type: "tag", Enabled: true, Tag: "ta", Retention: "30d"},
	})
	handler := NewRulesHandler()

	t.Run("toggles enabled state", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPatch, "/api/rules/rule-a/toggle", strings.NewReader(`{"enabled":false}`))
		req = withRuleName(req, "rule-a")
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.ToggleRule(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)
		assert.False(t, config.Get().AdvancedRules[0].Enabled, "rule must be disabled after reload")
	})

	t.Run("returns 404 for missing rule", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPatch, "/api/rules/nope/toggle", strings.NewReader(`{"enabled":true}`))
		req = withRuleName(req, "nope")
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.ToggleRule(rec, req)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("rejects malformed body", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPatch, "/api/rules/rule-a/toggle", strings.NewReader(`{`))
		req = withRuleName(req, "rule-a")
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.ToggleRule(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

func TestValidateRule(t *testing.T) {
	cases := []struct {
		name       string
		rule       config.AdvancedRule
		wantErr    bool
		wantSubstr string
	}{
		{"valid tag", config.AdvancedRule{Name: "r", Type: "tag", Tag: "t", Retention: "30d"}, false, ""},
		{"missing name", config.AdvancedRule{Type: "tag", Tag: "t", Retention: "30d"}, true, "name"},
		{"invalid type", config.AdvancedRule{Name: "r", Type: "movie"}, true, "type"},
		{"tag missing tag", config.AdvancedRule{Name: "r", Type: "tag", Retention: "30d"}, true, "tag"},
		{"tag missing retention", config.AdvancedRule{Name: "r", Type: "tag", Tag: "t"}, true, "retention"},
		{"episode missing max", config.AdvancedRule{Name: "r", Type: "episode"}, true, "max_episodes"},
		{"user missing users", config.AdvancedRule{Name: "r", Type: "user"}, true, "users"},
		{"user missing identifier", config.AdvancedRule{Name: "r", Type: "user", Users: []config.UserRule{{Retention: "30d"}}}, true, "users[0]"},
		{"user missing retention", config.AdvancedRule{Name: "r", Type: "user", Users: []config.UserRule{{Username: "bob"}}}, true, "Retention is required"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateRule(&tc.rule)
			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantSubstr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestErrInvalidInput_Error(t *testing.T) {
	// Regression: an index > 9 must format as a decimal number, not a control
	// character (the old string(rune(index)) produced garbage above index 9).
	i10 := 10
	err := ErrInvalidInput{Field: "users", Message: "Retention is required for each user", Index: &i10}
	assert.Equal(t, "users[10]: Retention is required for each user", err.Error())

	// Without an index the error is field: message.
	err = ErrInvalidInput{Field: "name", Message: "Rule name is required"}
	assert.Equal(t, "name: Rule name is required", err.Error())
}
