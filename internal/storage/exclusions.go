package storage

import (
	"encoding/json"
	"fmt"
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

	// Try to load existing file. A load failure must NOT be treated as an empty
	// exclusion set: excluded items would silently become deletable. Fail closed
	// so the operator can repair or remove the corrupt file first.
	if _, err := os.Stat(filePath); err == nil {
		if err := ef.load(); err != nil {
			backup, backupErr := backupCorruptFile(filePath)
			if backupErr != nil {
				return nil, fmt.Errorf("failed to load exclusions file: %w (and backing it up failed: %v)", err, backupErr)
			}
			return nil, fmt.Errorf("failed to load exclusions file %s: %w (corrupt file preserved at %s)", filePath, err, backup)
		}
	}

	return ef, nil
}

// Add adds an exclusion to the file
func (ef *ExclusionsFile) Add(item ExclusionItem) error {
	ef.mu.Lock()
	defer ef.mu.Unlock()

	next := make(map[string]ExclusionItem, len(ef.Items)+1)
	for id, existing := range ef.Items {
		next[id] = existing
	}
	next[item.ExternalID] = item
	now := time.Now()

	if err := ef.persist(next, now); err != nil {
		return err
	}

	ef.Items = next
	ef.UpdatedAt = now
	return nil
}

// Remove removes an exclusion from the file
func (ef *ExclusionsFile) Remove(externalID string) error {
	ef.mu.Lock()
	defer ef.mu.Unlock()

	if _, exists := ef.Items[externalID]; !exists {
		return nil
	}

	next := make(map[string]ExclusionItem, len(ef.Items)-1)
	for id, existing := range ef.Items {
		if id != externalID {
			next[id] = existing
		}
	}
	now := time.Now()

	if err := ef.persist(next, now); err != nil {
		return err
	}

	ef.Items = next
	ef.UpdatedAt = now
	return nil
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

	var loaded ExclusionsFile
	if err := json.Unmarshal(data, &loaded); err != nil {
		return err
	}

	ef.Version = loaded.Version
	ef.UpdatedAt = loaded.UpdatedAt
	ef.Items = loaded.Items
	if ef.Items == nil {
		ef.Items = make(map[string]ExclusionItem)
	}

	log.Info().Int("count", len(ef.Items)).Msg("Loaded exclusions from file")
	return nil
}

// persist atomically writes the given state to disk. Callers hold ef.mu.
// A struct constructed without a file path (e.g. in tests) is in-memory only.
func (ef *ExclusionsFile) persist(items map[string]ExclusionItem, updatedAt time.Time) error {
	if ef.filePath == "" {
		return nil
	}

	data, err := json.MarshalIndent(&ExclusionsFile{
		Version:   ef.Version,
		UpdatedAt: updatedAt,
		Items:     items,
	}, "", "  ")
	if err != nil {
		return err
	}

	if err := writeFileAtomic(ef.filePath, data, 0644); err != nil {
		return err
	}

	log.Debug().Int("count", len(items)).Msg("Saved exclusions to file")
	return nil
}
