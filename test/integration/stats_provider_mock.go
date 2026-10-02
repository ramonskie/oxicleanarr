package integration

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// StatsProviderMock abstracts a stats-provider mock server so the same
// watch-history lifecycle scenarios can be exercised against Jellystat and
// Tracearr. The adapters below embed the concrete mocks, inheriting their
// URL/Close/SetMovieIDs/SetWatchTimestamp behaviour; they only add provider
// identity and config-wiring operations.
type StatsProviderMock interface {
	Name() string      // "Jellystat" | "Tracearr"
	ConfigKey() string // "jellystat" | "tracearr"
	URL() string
	APIKey() string // "" for Jellystat (its mock has no auth), mock key for Tracearr
	Close()
	SetMovieIDs(map[string]string)
	SetWatchTimestamp(title string, watchedAt time.Time)
	Enable(cfg map[string]interface{})
	Disable(cfg map[string]interface{})
}

// tracearrTestServerID is a fixed media-server UUID written into the Tracearr
// integration config. Validation requires a non-empty server_id when the
// provider is enabled, and the Tracearr client fails fast without one.
const tracearrTestServerID = "00000000-0000-0000-0000-0000000000aa"

// statsProviderKeys lists every mutually-exclusive stats provider config key.
// At most one may be enabled at a time.
func statsProviderKeys() []string {
	return []string{"jellystat", "streamystats", "tracearr"}
}

// statsProviderFactory pairs a provider name with a constructor for its mock.
// Tests iterate factories and call NewMock INSIDE each subtest closure so a mock
// server is only started for subtests that actually run (e.g. honouring -run).
type statsProviderFactory struct {
	Name    string
	NewMock func() StatsProviderMock
}

// statsProviderFactories returns one factory per stats provider, in run order.
func statsProviderFactories() []statsProviderFactory {
	return []statsProviderFactory{
		{
			Name:    "Jellystat",
			NewMock: func() StatsProviderMock { return jellystatProviderMock{NewMockJellystatServer()} },
		},
		{
			Name:    "Tracearr",
			NewMock: func() StatsProviderMock { return tracearrProviderMock{NewMockTracearrServer()} },
		},
	}
}

// jellystatProviderMock adapts MockJellystatServer to StatsProviderMock.
type jellystatProviderMock struct {
	*MockJellystatServer
}

func (jellystatProviderMock) Name() string      { return "Jellystat" }
func (jellystatProviderMock) ConfigKey() string { return "jellystat" }

// APIKey is empty because the Jellystat mock does not authenticate requests.
// Enable therefore writes no api_key into the config; setup leaves Jellystat's
// api_key as the empty string.
func (jellystatProviderMock) APIKey() string { return "" }

func (j jellystatProviderMock) Enable(cfg map[string]interface{}) {
	enableStatsProvider(cfg, j.ConfigKey(), j.URL(), j.APIKey(), "")
}

func (j jellystatProviderMock) Disable(cfg map[string]interface{}) {
	disableStatsProvider(cfg, j.ConfigKey())
}

// tracearrProviderMock adapts MockTracearrServer to StatsProviderMock.
type tracearrProviderMock struct {
	*MockTracearrServer
}

func (tracearrProviderMock) Name() string      { return "Tracearr" }
func (tracearrProviderMock) ConfigKey() string { return "tracearr" }

// APIKey returns the bearer key the mock accepts; Enable writes it into the
// Tracearr integration config so OxiCleanarr authenticates successfully.
func (t tracearrProviderMock) APIKey() string { return t.MockTracearrServer.APIKey() }

func (t tracearrProviderMock) Enable(cfg map[string]interface{}) {
	enableStatsProvider(cfg, t.ConfigKey(), t.URL(), t.APIKey(), tracearrTestServerID)
}

func (t tracearrProviderMock) Disable(cfg map[string]interface{}) {
	disableStatsProvider(cfg, t.ConfigKey())
}

// enableStatsProvider enables exactly one stats provider in the integrations
// section: it writes key = {enabled:true, url:<docker-reachable>} plus api_key
// and server_id when non-empty, and disables the other stats providers so the
// exactly-one validation rule is satisfied.
func enableStatsProvider(cfg map[string]interface{}, key, rawURL, apiKey, serverID string) {
	integrations, ok := cfg["integrations"].(map[string]interface{})
	if !ok {
		integrations = make(map[string]interface{})
		cfg["integrations"] = integrations
	}

	for _, other := range statsProviderKeys() {
		if other == key {
			continue
		}
		if entry, ok := integrations[other].(map[string]interface{}); ok {
			entry["enabled"] = false
		}
	}

	entry, _ := integrations[key].(map[string]interface{})
	if entry == nil {
		entry = make(map[string]interface{})
		integrations[key] = entry
	}
	entry["enabled"] = true
	entry["url"] = convertMockURLForDocker(rawURL)
	if apiKey != "" {
		entry["api_key"] = apiKey
	}
	if serverID != "" {
		entry["server_id"] = serverID
	}
}

// disableStatsProvider sets integrations[key].enabled = false, leaving the
// section's other fields (url/api_key/server_id) untouched.
func disableStatsProvider(cfg map[string]interface{}, key string) {
	integrations, ok := cfg["integrations"].(map[string]interface{})
	if !ok {
		return
	}
	if entry, ok := integrations[key].(map[string]interface{}); ok {
		entry["enabled"] = false
	}
}

// RestoreStatsProviderConfig disables ALL stats-provider integrations in the
// config file, restoring the pre-test disabled state. Disabling every provider
// (not just the one under test) also re-disables any sibling provider that
// enableStatsProvider force-disabled to satisfy the exactly-one validation rule.
func RestoreStatsProviderConfig(t *testing.T, configPath string, provider StatsProviderMock) {
	t.Helper()
	t.Logf("Restoring %s config to disabled state...", provider.Name())

	content, err := os.ReadFile(configPath)
	require.NoError(t, err)

	var cfg map[string]interface{}
	err = yaml.Unmarshal(content, &cfg)
	require.NoError(t, err)

	for _, key := range statsProviderKeys() {
		disableStatsProvider(cfg, key)
	}

	newContent, err := yaml.Marshal(cfg)
	require.NoError(t, err)
	err = os.WriteFile(configPath, newContent, 0644)
	require.NoError(t, err)

	t.Logf("%s config restored", provider.Name())
}

// convertMockURLForDocker converts a localhost mock server URL to be reachable from Docker
func convertMockURLForDocker(mockURL string) string {
	// Convert http://127.0.0.1:PORT to http://host.docker.internal:PORT
	// This allows Docker containers to reach the host machine's mock servers
	return strings.Replace(mockURL, "127.0.0.1", "host.docker.internal", 1)
}
