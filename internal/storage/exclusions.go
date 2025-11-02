package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// ExclusionItem represents a media item excluded from deletion
type ExclusionItem struct {
	ExternalID   string    `json:"external_id"`
	ExternalType string    `json:"external_type"`
	MediaType    string    `json:"media_type"`
	Title        string    `json:"title"`
	ExcludedAt   time.Time `json:"excluded_at"`
	ExcludedBy   string    `json:"excluded_by"`
	Reason       string    `json:"reason"`
}

// ExclusionsFile represents the exclusions.json structure
type ExclusionsFile struct {
	Version   string                   `json:"version"`
	UpdatedAt time.Time                `json:"updated_at"`
	Items     map[string]ExclusionItem `json:"items"`
	mu        sync.RWMutex             `json:"-"`
	filePath  string                   `json:"-"`
}

// NewExclusionsFile creates or loads an exclusions file
func NewExclusionsFile(dataPath string) (*ExclusionsFile, error) {
	filePath := filepath.Join(dataPath, "exclusions.json")

	// Create data directory if it doesn't exist
	if err := os.MkdirAll(dataPath, 0755); err != nil {
		return nil, err
	}

	ef := &ExclusionsFile{
		Version:  "1.0",
		Items:    make(map[string]ExclusionItem),
		filePath: filePath,
	}

	// Try to load existing file
	if _, err := os.Stat(filePath); err == nil {
		if err := ef.load(); err != nil {
			log.Warn().Err(err).Msg("Failed to load exclusions file, starting fresh")
		}
	}

	return ef, nil
}

// Add adds an exclusion to the file
func (ef *ExclusionsFile) Add(item ExclusionItem) error {
	ef.mu.Lock()
	defer ef.mu.Unlock()

	ef.Items[item.ExternalID] = item
	ef.UpdatedAt = time.Now()

	return ef.save()
}

// Remove removes an exclusion from the file
func (ef *ExclusionsFile) Remove(externalID string) error {
	ef.mu.Lock()
	defer ef.mu.Unlock()

	delete(ef.Items, externalID)
	ef.UpdatedAt = time.Now()

	return ef.save()
}

// Get retrieves an exclusion by external ID
func (ef *ExclusionsFile) Get(externalID string) (ExclusionItem, bool) {
	ef.mu.RLock()
	defer ef.mu.RUnlock()

	item, exists := ef.Items[externalID]
	return item, exists
}

// GetAll returns all exclusions
func (ef *ExclusionsFile) GetAll() []ExclusionItem {
	ef.mu.RLock()
	defer ef.mu.RUnlock()

	items := make([]ExclusionItem, 0, len(ef.Items))
	for _, item := range ef.Items {
		items = append(items, item)
	}
	return items
}

// IsExcluded checks if an external ID is excluded
func (ef *ExclusionsFile) IsExcluded(externalID string) bool {
	ef.mu.RLock()
	defer ef.mu.RUnlock()

	_, exists := ef.Items[externalID]
	return exists
}

// load reads the exclusions file from disk
func (ef *ExclusionsFile) load() error {
	data, err := os.ReadFile(ef.filePath)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(data, ef); err != nil {
		return err
	}

	log.Info().Int("count", len(ef.Items)).Msg("Loaded exclusions from file")
	return nil
}

// save writes the exclusions file to disk
func (ef *ExclusionsFile) save() error {
	data, err := json.MarshalIndent(ef, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(ef.filePath, data, 0644); err != nil {
		return err
	}

	log.Debug().Int("count", len(ef.Items)).Msg("Saved exclusions to file")
	return nil
}
