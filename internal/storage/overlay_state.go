package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// OverlayItemState tracks one Jellyfin item whose poster carries a deletion
// banner. The original poster is preserved at OriginalPath so the banner can be
// reverted (restored) when the item leaves the scheduled-deletion set.
type OverlayItemState struct {
	ItemID        string `json:"item_id"`
	OriginalPath  string `json:"original_path"`
	DaysLeftShown int    `json:"days_left_shown"`
	// StyleHash fingerprints the banner style (template/colors/fonts) the
	// applied banner was rendered with, so a style change re-renders even when
	// the day count is unchanged. Empty for state written before this field.
	StyleHash string    `json:"style_hash,omitempty"`
	AppliedAt time.Time `json:"applied_at,omitempty"`
}

// OverlayStateFile represents the data/overlays/state.json structure.
type OverlayStateFile struct {
	Version  string                      `json:"version"`
	Items    map[string]OverlayItemState `json:"items"`
	mu       sync.RWMutex
	filePath string
}

// NewOverlayStateFile creates or loads the overlay state file under
// <dataPath>/overlays/state.json.
func NewOverlayStateFile(dataPath string) (*OverlayStateFile, error) {
	dir := filepath.Join(dataPath, "overlays")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}

	filePath := filepath.Join(dir, "state.json")

	sf := &OverlayStateFile{
		Version:  "1.0",
		Items:    make(map[string]OverlayItemState),
		filePath: filePath,
	}

	if _, err := os.Stat(filePath); err == nil {
		if err := sf.load(); err != nil {
			if backup, backupErr := backupCorruptFile(filePath); backupErr != nil {
				log.Error().Err(err).Err(backupErr).
					Msg("Failed to load overlay state file; corrupt backup also failed, starting fresh")
			} else {
				log.Error().Err(err).Str("backup", backup).
					Msg("Failed to load overlay state file; corrupt file preserved, starting fresh")
			}
		}
	}

	return sf, nil
}

// Get returns the state for an item, if any.
func (sf *OverlayStateFile) Get(itemID string) (OverlayItemState, bool) {
	sf.mu.RLock()
	defer sf.mu.RUnlock()

	st, found := sf.Items[itemID]
	return st, found
}

// Set records or replaces the state for an item.
func (sf *OverlayStateFile) Set(st OverlayItemState) error {
	sf.mu.Lock()
	defer sf.mu.Unlock()

	next := make(map[string]OverlayItemState, len(sf.Items)+1)
	for id, s := range sf.Items {
		next[id] = s
	}
	next[st.ItemID] = st

	if err := sf.persist(next); err != nil {
		return err
	}

	sf.Items = next
	return nil
}

// Remove deletes the state for an item (no-op when absent).
func (sf *OverlayStateFile) Remove(itemID string) error {
	sf.mu.Lock()
	defer sf.mu.Unlock()

	if _, found := sf.Items[itemID]; !found {
		return nil
	}

	next := make(map[string]OverlayItemState, len(sf.Items)-1)
	for id, s := range sf.Items {
		if id != itemID {
			next[id] = s
		}
	}

	if err := sf.persist(next); err != nil {
		return err
	}

	sf.Items = next
	return nil
}

// GetAll returns a snapshot of every tracked item state.
func (sf *OverlayStateFile) GetAll() map[string]OverlayItemState {
	sf.mu.RLock()
	defer sf.mu.RUnlock()

	items := make(map[string]OverlayItemState, len(sf.Items))
	for id, st := range sf.Items {
		items[id] = st
	}
	return items
}

// load reads the state file from disk. Callers do not hold sf.mu.
func (sf *OverlayStateFile) load() error {
	data, err := os.ReadFile(sf.filePath)
	if err != nil {
		return err
	}

	var temp struct {
		Version string                      `json:"version"`
		Items   map[string]OverlayItemState `json:"items"`
	}

	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	sf.Version = temp.Version
	sf.Items = temp.Items
	if sf.Items == nil {
		sf.Items = make(map[string]OverlayItemState)
	}

	log.Info().Int("count", len(sf.Items)).Msg("Loaded overlay state from file")
	return nil
}

// persist atomically writes the given state to disk. Callers hold sf.mu.
// A struct constructed without a file path (e.g. in tests) is in-memory only.
func (sf *OverlayStateFile) persist(items map[string]OverlayItemState) error {
	if sf.filePath == "" {
		return nil
	}

	data := struct {
		Version string                      `json:"version"`
		Items   map[string]OverlayItemState `json:"items"`
	}{
		Version: sf.Version,
		Items:   items,
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	if err := writeFileAtomic(sf.filePath, jsonData, 0o644); err != nil {
		return err
	}

	log.Debug().Int("count", len(items)).Msg("Saved overlay state to file")
	return nil
}
