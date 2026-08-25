package storage

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOverlayStateFile_EmptyWhenMissing(t *testing.T) {
	sf, err := NewOverlayStateFile(t.TempDir())
	require.NoError(t, err)

	assert.Empty(t, sf.GetAll())
	_, found := sf.Get("item-1")
	assert.False(t, found)
}

func TestOverlayStateFile_SetGetRemove(t *testing.T) {
	sf, err := NewOverlayStateFile(t.TempDir())
	require.NoError(t, err)

	st := OverlayItemState{
		ItemID:        "item-1",
		OriginalPath:  "/data/overlays/originals/item-1.jpg",
		DaysLeftShown: 3,
		AppliedAt:     time.Now(),
	}

	require.NoError(t, sf.Set(st))

	got, found := sf.Get("item-1")
	assert.True(t, found)
	assert.Equal(t, st, got)
	assert.Len(t, sf.GetAll(), 1)

	require.NoError(t, sf.Remove("item-1"))
	_, found = sf.Get("item-1")
	assert.False(t, found)
	assert.Empty(t, sf.GetAll())
}

func TestOverlayStateFile_PersistsToDisk(t *testing.T) {
	dir := t.TempDir()
	sf, err := NewOverlayStateFile(dir)
	require.NoError(t, err)

	require.NoError(t, sf.Set(OverlayItemState{
		ItemID:        "item-1",
		OriginalPath:  "/data/overlays/originals/item-1.jpg",
		DaysLeftShown: 7,
		AppliedAt:     time.Now().UTC(),
	}))

	// A fresh instance must reload what was persisted.
	reloaded, err := NewOverlayStateFile(dir)
	require.NoError(t, err)

	got, found := reloaded.Get("item-1")
	require.True(t, found)
	assert.Equal(t, "item-1", got.ItemID)
	assert.Equal(t, 7, got.DaysLeftShown)

	// And the file lives at <data>/overlays/state.json.
	_, err = os.Stat(filepath.Join(dir, "overlays", "state.json"))
	assert.NoError(t, err)
}

func TestOverlayStateFile_RemovePersists(t *testing.T) {
	dir := t.TempDir()
	sf, err := NewOverlayStateFile(dir)
	require.NoError(t, err)

	require.NoError(t, sf.Set(OverlayItemState{ItemID: "a", OriginalPath: "p", DaysLeftShown: 1}))
	require.NoError(t, sf.Remove("a"))

	reloaded, err := NewOverlayStateFile(dir)
	require.NoError(t, err)
	assert.Empty(t, reloaded.GetAll())
}

func TestOverlayStateFile_InMemoryOnly(t *testing.T) {
	sf := &OverlayStateFile{
		Version: "1.0",
		Items:   make(map[string]OverlayItemState),
	}

	require.NoError(t, sf.Set(OverlayItemState{ItemID: "x", DaysLeftShown: 2}))
	got, found := sf.Get("x")
	assert.True(t, found)
	assert.Equal(t, 2, got.DaysLeftShown)
}
