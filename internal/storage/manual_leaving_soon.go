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
			// Preserve the corrupt file for manual recovery instead of
			// silently starting fresh and overwriting it on the next save.
			if backup, backupErr := backupCorruptFile(filePath); backupErr != nil {
				log.Error().Err(err).Err(backupErr).
					Msg("Failed to load manual leaving soon file; corrupt backup also failed, starting fresh")
			} else {
				log.Error().Err(err).Str("backup", backup).
					Msg("Failed to load manual leaving soon file; corrupt file preserved, starting fresh")
			}
		}
	}

	return f, nil
}

// Add adds a manual leaving soon flag to the file
func (f *ManualLeavingSoonFile) Add(item ManualLeavingSoonItem) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	next := make(map[string]ManualLeavingSoonItem, len(f.Items)+1)
	for id, existing := range f.Items {
		next[id] = existing
	}
	next[item.ExternalID] = item
	now := time.Now()

	if err := f.persist(next, now); err != nil {
		return err
	}

	f.Items = next
	f.UpdatedAt = now
	return nil
}

// Remove removes a manual leaving soon flag from the file
func (f *ManualLeavingSoonFile) Remove(externalID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, exists := f.Items[externalID]; !exists {
		return nil
	}

	next := make(map[string]ManualLeavingSoonItem, len(f.Items)-1)
	for id, existing := range f.Items {
		if id != externalID {
			next[id] = existing
		}
	}
	now := time.Now()

	if err := f.persist(next, now); err != nil {
		return err
	}

	f.Items = next
	f.UpdatedAt = now
	return nil
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

	var loaded ManualLeavingSoonFile
	if err := json.Unmarshal(data, &loaded); err != nil {
		return err
	}

	f.Version = loaded.Version
	f.UpdatedAt = loaded.UpdatedAt
	f.Items = loaded.Items
	if f.Items == nil {
		f.Items = make(map[string]ManualLeavingSoonItem)
	}

	log.Info().Int("count", len(f.Items)).Msg("Loaded manual leaving soon flags from file")
	return nil
}

// persist atomically writes the given state to disk. Callers hold f.mu.
// A struct constructed without a file path (e.g. in tests) is in-memory only.
func (f *ManualLeavingSoonFile) persist(items map[string]ManualLeavingSoonItem, updatedAt time.Time) error {
	if f.filePath == "" {
		return nil
	}

	data, err := json.MarshalIndent(&ManualLeavingSoonFile{
		Version:   f.Version,
		UpdatedAt: updatedAt,
		Items:     items,
	}, "", "  ")
	if err != nil {
		return err
	}

	if err := writeFileAtomic(f.filePath, data, 0644); err != nil {
		return err
	}

	log.Debug().Int("count", len(items)).Msg("Saved manual leaving soon flags to file")
	return nil
}
