package storage

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewManualLeavingSoonFile(t *testing.T) {
	t.Run("creates new file with empty items", func(t *testing.T) {
		tmpDir := t.TempDir()

		f, err := NewManualLeavingSoonFile(tmpDir)

		require.NoError(t, err)
		assert.NotNil(t, f)
		assert.Equal(t, "1.0", f.Version)
		assert.NotNil(t, f.Items)
		assert.Empty(t, f.Items)

		// File is lazily created — should not exist yet
		filePath := filepath.Join(tmpDir, "manual_leaving_soon.json")
		_, err = os.Stat(filePath)
		assert.True(t, os.IsNotExist(err), "file should not exist before first write")

		// After adding an item the file should be created
		_ = f.Add(ManualLeavingSoonItem{ExternalID: "radarr-1", Title: "Test"})
		_, err = os.Stat(filePath)
		assert.NoError(t, err, "file should exist after first write")
	})

	t.Run("loads existing file", func(t *testing.T) {
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "manual_leaving_soon.json")

		testData := `{
  "version": "1.0",
  "updated_at": "2024-01-01T00:00:00Z",
  "items": {
    "radarr-1": {
      "external_id": "radarr-1",
      "external_type": "radarr",
      "media_type": "movie",
      "title": "Fight Club",
      "delete_after": "2024-02-01T00:00:00Z",
      "flagged_at": "2024-01-01T00:00:00Z",
      "flagged_by": "api"
    }
  }
}`
		err := os.WriteFile(filePath, []byte(testData), 0644)
		require.NoError(t, err)

		f, err := NewManualLeavingSoonFile(tmpDir)

		require.NoError(t, err)
		assert.Len(t, f.Items, 1)
		assert.Contains(t, f.Items, "radarr-1")
		assert.Equal(t, "Fight Club", f.Items["radarr-1"].Title)
		assert.Equal(t, "radarr", f.Items["radarr-1"].ExternalType)
		assert.Equal(t, "movie", f.Items["radarr-1"].MediaType)
	})

	t.Run("handles corrupted file gracefully", func(t *testing.T) {
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "manual_leaving_soon.json")

		err := os.WriteFile(filePath, []byte("invalid json {{"), 0644)
		require.NoError(t, err)

		f, err := NewManualLeavingSoonFile(tmpDir)

		require.NoError(t, err)
		assert.Empty(t, f.Items)
	})
}

func TestManualLeavingSoonFile_Add(t *testing.T) {
	t.Run("adds flag successfully", func(t *testing.T) {
		tmpDir := t.TempDir()
		f, err := NewManualLeavingSoonFile(tmpDir)
		require.NoError(t, err)

		item := ManualLeavingSoonItem{
			ExternalID:   "radarr-1",
			ExternalType: "radarr",
			MediaType:    "movie",
			Title:        "Fight Club",
			DeleteAfter:  time.Now().Add(7 * 24 * time.Hour),
			FlaggedAt:    time.Now(),
			FlaggedBy:    "api",
		}

		err = f.Add(item)

		require.NoError(t, err)
		assert.Len(t, f.Items, 1)
		assert.Contains(t, f.Items, "radarr-1")
		assert.Equal(t, "Fight Club", f.Items["radarr-1"].Title)
		assert.False(t, f.UpdatedAt.IsZero())
	})

	t.Run("overwrites existing flag", func(t *testing.T) {
		tmpDir := t.TempDir()
		f, err := NewManualLeavingSoonFile(tmpDir)
		require.NoError(t, err)

		_ = f.Add(ManualLeavingSoonItem{ExternalID: "radarr-1", Title: "Original Title"})
		_ = f.Add(ManualLeavingSoonItem{ExternalID: "radarr-1", Title: "Updated Title"})

		assert.Len(t, f.Items, 1)
		assert.Equal(t, "Updated Title", f.Items["radarr-1"].Title)
	})

	t.Run("persists to disk", func(t *testing.T) {
		tmpDir := t.TempDir()
		f, err := NewManualLeavingSoonFile(tmpDir)
		require.NoError(t, err)

		_ = f.Add(ManualLeavingSoonItem{ExternalID: "radarr-1", Title: "Fight Club"})

		// Load a fresh instance and verify persistence
		f2, err := NewManualLeavingSoonFile(tmpDir)
		require.NoError(t, err)

		assert.Len(t, f2.Items, 1)
		assert.Contains(t, f2.Items, "radarr-1")
		assert.Equal(t, "Fight Club", f2.Items["radarr-1"].Title)
	})

	t.Run("multiple items with different IDs", func(t *testing.T) {
		tmpDir := t.TempDir()
		f, err := NewManualLeavingSoonFile(tmpDir)
		require.NoError(t, err)

		_ = f.Add(ManualLeavingSoonItem{ExternalID: "radarr-1", Title: "Movie 1"})
		_ = f.Add(ManualLeavingSoonItem{ExternalID: "radarr-2", Title: "Movie 2"})
		_ = f.Add(ManualLeavingSoonItem{ExternalID: "sonarr-10", Title: "Show 1"})

		assert.Len(t, f.Items, 3)
	})
}

func TestManualLeavingSoonFile_Remove(t *testing.T) {
	t.Run("removes flag successfully", func(t *testing.T) {
		tmpDir := t.TempDir()
		f, err := NewManualLeavingSoonFile(tmpDir)
		require.NoError(t, err)

		_ = f.Add(ManualLeavingSoonItem{ExternalID: "radarr-1", Title: "Movie 1"})
		_ = f.Add(ManualLeavingSoonItem{ExternalID: "radarr-2", Title: "Movie 2"})

		err = f.Remove("radarr-1")

		require.NoError(t, err)
		assert.Len(t, f.Items, 1)
		assert.NotContains(t, f.Items, "radarr-1")
		assert.Contains(t, f.Items, "radarr-2")
	})

	t.Run("removing non-existent flag is safe", func(t *testing.T) {
		tmpDir := t.TempDir()
		f, err := NewManualLeavingSoonFile(tmpDir)
		require.NoError(t, err)

		err = f.Remove("non-existent")

		require.NoError(t, err)
		assert.Empty(t, f.Items)
	})

	t.Run("persists removal to disk", func(t *testing.T) {
		tmpDir := t.TempDir()
		f, err := NewManualLeavingSoonFile(tmpDir)
		require.NoError(t, err)

		_ = f.Add(ManualLeavingSoonItem{ExternalID: "radarr-1", Title: "Movie 1"})
		_ = f.Remove("radarr-1")

		f2, err := NewManualLeavingSoonFile(tmpDir)
		require.NoError(t, err)

		assert.Empty(t, f2.Items)
	})
}

func TestManualLeavingSoonFile_Get(t *testing.T) {
	t.Run("retrieves existing flag", func(t *testing.T) {
		tmpDir := t.TempDir()
		f, err := NewManualLeavingSoonFile(tmpDir)
		require.NoError(t, err)

		deleteAfter := time.Now().Add(7 * 24 * time.Hour)
		_ = f.Add(ManualLeavingSoonItem{
			ExternalID:  "radarr-1",
			Title:       "Fight Club",
			DeleteAfter: deleteAfter,
		})

		retrieved, found := f.Get("radarr-1")

		assert.True(t, found)
		assert.Equal(t, "Fight Club", retrieved.Title)
		assert.Equal(t, "radarr-1", retrieved.ExternalID)
	})

	t.Run("returns false for non-existent item", func(t *testing.T) {
		tmpDir := t.TempDir()
		f, err := NewManualLeavingSoonFile(tmpDir)
		require.NoError(t, err)

		_, found := f.Get("non-existent")

		assert.False(t, found)
	})
}

func TestManualLeavingSoonFile_GetAll(t *testing.T) {
	t.Run("returns all flags", func(t *testing.T) {
		tmpDir := t.TempDir()
		f, err := NewManualLeavingSoonFile(tmpDir)
		require.NoError(t, err)

		_ = f.Add(ManualLeavingSoonItem{ExternalID: "radarr-1", Title: "Movie 1"})
		_ = f.Add(ManualLeavingSoonItem{ExternalID: "radarr-2", Title: "Movie 2"})
		_ = f.Add(ManualLeavingSoonItem{ExternalID: "sonarr-10", Title: "Show 1"})

		items := f.GetAll()

		assert.Len(t, items, 3)
	})

	t.Run("returns empty slice when no flags", func(t *testing.T) {
		tmpDir := t.TempDir()
		f, err := NewManualLeavingSoonFile(tmpDir)
		require.NoError(t, err)

		items := f.GetAll()

		assert.Empty(t, items)
		assert.NotNil(t, items)
	})
}

func TestManualLeavingSoonFile_IsFlagged(t *testing.T) {
	t.Run("returns true for flagged item", func(t *testing.T) {
		tmpDir := t.TempDir()
		f, err := NewManualLeavingSoonFile(tmpDir)
		require.NoError(t, err)

		_ = f.Add(ManualLeavingSoonItem{ExternalID: "radarr-1"})

		assert.True(t, f.IsFlagged("radarr-1"))
	})

	t.Run("returns false for unflagged item", func(t *testing.T) {
		tmpDir := t.TempDir()
		f, err := NewManualLeavingSoonFile(tmpDir)
		require.NoError(t, err)

		assert.False(t, f.IsFlagged("radarr-1"))
	})

	t.Run("returns false after flag is removed", func(t *testing.T) {
		tmpDir := t.TempDir()
		f, err := NewManualLeavingSoonFile(tmpDir)
		require.NoError(t, err)

		_ = f.Add(ManualLeavingSoonItem{ExternalID: "radarr-1"})
		assert.True(t, f.IsFlagged("radarr-1"))

		_ = f.Remove("radarr-1")
		assert.False(t, f.IsFlagged("radarr-1"))
	})
}

func TestManualLeavingSoonFile_ConcurrentAccess(t *testing.T) {
	t.Run("handles concurrent reads and writes", func(t *testing.T) {
		tmpDir := t.TempDir()
		f, err := NewManualLeavingSoonFile(tmpDir)
		require.NoError(t, err)

		// Seed initial items
		for i := 0; i < 10; i++ {
			_ = f.Add(ManualLeavingSoonItem{
				ExternalID: string(rune('a' + i)),
				Title:      "Item " + string(rune('a'+i)),
			})
		}

		done := make(chan bool, 6)

		// Concurrent reads
		for i := 0; i < 3; i++ {
			go func() {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("Panic during concurrent read: %v", r)
					}
					done <- true
				}()

				for j := 0; j < 100; j++ {
					_ = f.GetAll()
					_ = f.IsFlagged("a")
					_, _ = f.Get("b")
				}
			}()
		}

		// Concurrent writes
		for i := 0; i < 3; i++ {
			go func(id int) {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("Panic during concurrent write: %v", r)
					}
					done <- true
				}()

				for j := 0; j < 10; j++ {
					_ = f.Add(ManualLeavingSoonItem{
						ExternalID: string(rune('k' + id)),
						Title:      "Concurrent Item",
					})
				}
			}(i)
		}

		for i := 0; i < 6; i++ {
			<-done
		}
	})
}
