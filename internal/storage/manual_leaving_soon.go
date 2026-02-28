package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// ManualLeavingSoonItem represents a media item manually flagged for leaving soon
type ManualLeavingSoonItem struct {
	ExternalID   string    `json:"external_id"`
	ExternalType string    `json:"external_type"` // "radarr" | "sonarr"
	MediaType    string    `json:"media_type"`    // "movie" | "tv_show"
	Title        string    `json:"title"`
	DeleteAfter  time.Time `json:"delete_after"`
	FlaggedAt    time.Time `json:"flagged_at"`
	FlaggedBy    string    `json:"flagged_by"` // "api"
}

// ManualLeavingSoonFile represents the manual_leaving_soon.json structure
type ManualLeavingSoonFile struct {
	Version   string                           `json:"version"`
	UpdatedAt time.Time                        `json:"updated_at"`
	Items     map[string]ManualLeavingSoonItem `json:"items"`
	mu        sync.RWMutex                     `json:"-"`
	filePath  string                           `json:"-"`
}

// NewManualLeavingSoonFile creates or loads a manual leaving soon file
func NewManualLeavingSoonFile(dataPath string) (*ManualLeavingSoonFile, error) {
	filePath := filepath.Join(dataPath, "manual_leaving_soon.json")

	if err := os.MkdirAll(dataPath, 0755); err != nil {
		return nil, err
	}

	f := &ManualLeavingSoonFile{
		Version:  "1.0",
		Items:    make(map[string]ManualLeavingSoonItem),
		filePath: filePath,
	}

	if _, err := os.Stat(filePath); err == nil {
		if err := f.load(); err != nil {
			log.Warn().Err(err).Msg("Failed to load manual leaving soon file, starting fresh")
		}
	}

	return f, nil
}

// Add adds a manual leaving soon flag to the file
func (f *ManualLeavingSoonFile) Add(item ManualLeavingSoonItem) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.Items[item.ExternalID] = item
	f.UpdatedAt = time.Now()

	return f.save()
}

// Remove removes a manual leaving soon flag from the file
func (f *ManualLeavingSoonFile) Remove(externalID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	delete(f.Items, externalID)
	f.UpdatedAt = time.Now()

	return f.save()
}

// Get retrieves a manual leaving soon flag by external ID
func (f *ManualLeavingSoonFile) Get(externalID string) (ManualLeavingSoonItem, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	item, exists := f.Items[externalID]
	return item, exists
}

// GetAll returns all manual leaving soon flags
func (f *ManualLeavingSoonFile) GetAll() []ManualLeavingSoonItem {
	f.mu.RLock()
	defer f.mu.RUnlock()

	items := make([]ManualLeavingSoonItem, 0, len(f.Items))
	for _, item := range f.Items {
		items = append(items, item)
	}
	return items
}

// IsFlagged checks if an external ID has a manual leaving soon flag
func (f *ManualLeavingSoonFile) IsFlagged(externalID string) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()

	_, exists := f.Items[externalID]
	return exists
}

// load reads the manual leaving soon file from disk
func (f *ManualLeavingSoonFile) load() error {
	data, err := os.ReadFile(f.filePath)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(data, f); err != nil {
		return err
	}

	log.Info().Int("count", len(f.Items)).Msg("Loaded manual leaving soon flags from file")
	return nil
}

// save writes the manual leaving soon file to disk
func (f *ManualLeavingSoonFile) save() error {
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(f.filePath, data, 0644); err != nil {
		return err
	}

	log.Debug().Int("count", len(f.Items)).Msg("Saved manual leaving soon flags to file")
	return nil
}
