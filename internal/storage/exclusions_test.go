package storage

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewExclusionsFile(t *testing.T) {
	t.Run("creates new exclusions file", func(t *testing.T) {
		tmpDir := t.TempDir()

		ef, err := NewExclusionsFile(tmpDir)

		require.NoError(t, err)
		assert.NotNil(t, ef)
		assert.Equal(t, "1.0", ef.Version)
		assert.NotNil(t, ef.Items)
		assert.Empty(t, ef.Items)

		// File is lazily created, so it shouldn't exist yet
		filePath := filepath.Join(tmpDir, "exclusions.json")
		_, err = os.Stat(filePath)
		assert.Error(t, err)
		assert.True(t, os.IsNotExist(err))

		// But after adding an item, it should exist
		ef.Add(ExclusionItem{ExternalID: "test-1", Title: "Test"})
		_, err = os.Stat(filePath)
		assert.NoError(t, err)
	})

	t.Run("loads existing exclusions file", func(t *testing.T) {
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "exclusions.json")

		// Create existing file with data
		testData := `{
  "version": "1.0",
  "updated_at": "2024-01-01T00:00:00Z",
  "items": {
    "movie-1": {
      "external_id": "movie-1",
      "external_type": "radarr",
      "media_type": "movie",
      "title": "Test Movie",
      "excluded_at": "2024-01-01T00:00:00Z",
      "excluded_by": "admin",
      "reason": "classic"
    }
  }
}`
		err := os.WriteFile(filePath, []byte(testData), 0644)
		require.NoError(t, err)

		// Load the file
		ef, err := NewExclusionsFile(tmpDir)

		require.NoError(t, err)
		assert.Len(t, ef.Items, 1)
		assert.Contains(t, ef.Items, "movie-1")
		assert.Equal(t, "Test Movie", ef.Items["movie-1"].Title)
	})

	t.Run("handles corrupted file gracefully", func(t *testing.T) {
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "exclusions.json")

		// Write invalid JSON
		err := os.WriteFile(filePath, []byte("invalid json"), 0644)
		require.NoError(t, err)

		// Should still create file successfully, but with empty items
		ef, err := NewExclusionsFile(tmpDir)

		require.NoError(t, err)
		assert.Empty(t, ef.Items)
	})
}

func TestExclusionsFile_Add(t *testing.T) {
	t.Run("adds exclusion successfully", func(t *testing.T) {
		tmpDir := t.TempDir()
		ef, err := NewExclusionsFile(tmpDir)
		require.NoError(t, err)

		item := ExclusionItem{
			ExternalID:   "movie-1",
			ExternalType: "radarr",
			MediaType:    "movie",
			Title:        "Test Movie",
			ExcludedAt:   time.Now(),
			ExcludedBy:   "admin",
			Reason:       "user favorite",
		}

		err = ef.Add(item)

		require.NoError(t, err)
		assert.Len(t, ef.Items, 1)
		assert.Contains(t, ef.Items, "movie-1")
		assert.Equal(t, "Test Movie", ef.Items["movie-1"].Title)
		assert.False(t, ef.UpdatedAt.IsZero())
	})

	t.Run("overwrites existing exclusion", func(t *testing.T) {
		tmpDir := t.TempDir()
		ef, err := NewExclusionsFile(tmpDir)
		require.NoError(t, err)

		item1 := ExclusionItem{
			ExternalID: "movie-1",
			Title:      "Original Title",
			Reason:     "original reason",
		}

		err = ef.Add(item1)
		require.NoError(t, err)

		item2 := ExclusionItem{
			ExternalID: "movie-1",
			Title:      "Updated Title",
			Reason:     "updated reason",
		}

		err = ef.Add(item2)

		require.NoError(t, err)
		assert.Len(t, ef.Items, 1)
		assert.Equal(t, "Updated Title", ef.Items["movie-1"].Title)
		assert.Equal(t, "updated reason", ef.Items["movie-1"].Reason)
	})

	t.Run("persists to disk", func(t *testing.T) {
		tmpDir := t.TempDir()
		ef, err := NewExclusionsFile(tmpDir)
		require.NoError(t, err)

		item := ExclusionItem{
			ExternalID: "movie-1",
			Title:      "Test Movie",
		}

		err = ef.Add(item)
		require.NoError(t, err)

		// Load a new instance and verify persistence
		ef2, err := NewExclusionsFile(tmpDir)
		require.NoError(t, err)

		assert.Len(t, ef2.Items, 1)
		assert.Contains(t, ef2.Items, "movie-1")
	})
}

func TestExclusionsFile_Remove(t *testing.T) {
	t.Run("removes exclusion successfully", func(t *testing.T) {
		tmpDir := t.TempDir()
		ef, err := NewExclusionsFile(tmpDir)
		require.NoError(t, err)

		// Add two items
		ef.Add(ExclusionItem{ExternalID: "movie-1", Title: "Movie 1"})
		ef.Add(ExclusionItem{ExternalID: "movie-2", Title: "Movie 2"})

		// Remove one
		err = ef.Remove("movie-1")

		require.NoError(t, err)
		assert.Len(t, ef.Items, 1)
		assert.NotContains(t, ef.Items, "movie-1")
		assert.Contains(t, ef.Items, "movie-2")
	})

	t.Run("removing non-existent item is safe", func(t *testing.T) {
		tmpDir := t.TempDir()
		ef, err := NewExclusionsFile(tmpDir)
		require.NoError(t, err)

		err = ef.Remove("non-existent")

		require.NoError(t, err)
		assert.Empty(t, ef.Items)
	})

	t.Run("persists removal to disk", func(t *testing.T) {
		tmpDir := t.TempDir()
		ef, err := NewExclusionsFile(tmpDir)
		require.NoError(t, err)

		ef.Add(ExclusionItem{ExternalID: "movie-1", Title: "Movie 1"})
		ef.Remove("movie-1")

		// Load a new instance and verify removal was persisted
		ef2, err := NewExclusionsFile(tmpDir)
		require.NoError(t, err)

		assert.Empty(t, ef2.Items)
	})
}

func TestExclusionsFile_Get(t *testing.T) {
	t.Run("retrieves existing exclusion", func(t *testing.T) {
		tmpDir := t.TempDir()
		ef, err := NewExclusionsFile(tmpDir)
		require.NoError(t, err)

		item := ExclusionItem{
			ExternalID: "movie-1",
			Title:      "Test Movie",
			Reason:     "favorite",
		}
		ef.Add(item)

		retrieved, found := ef.Get("movie-1")

		assert.True(t, found)
		assert.Equal(t, "Test Movie", retrieved.Title)
		assert.Equal(t, "favorite", retrieved.Reason)
	})

	t.Run("returns false for non-existent item", func(t *testing.T) {
		tmpDir := t.TempDir()
		ef, err := NewExclusionsFile(tmpDir)
		require.NoError(t, err)

		_, found := ef.Get("non-existent")

		assert.False(t, found)
	})
}

func TestExclusionsFile_GetAll(t *testing.T) {
	t.Run("returns all exclusions", func(t *testing.T) {
		tmpDir := t.TempDir()
		ef, err := NewExclusionsFile(tmpDir)
		require.NoError(t, err)

		ef.Add(ExclusionItem{ExternalID: "movie-1", Title: "Movie 1"})
		ef.Add(ExclusionItem{ExternalID: "movie-2", Title: "Movie 2"})
		ef.Add(ExclusionItem{ExternalID: "tv-1", Title: "TV Show 1"})

		items := ef.GetAll()

		assert.Len(t, items, 3)
	})

	t.Run("returns empty slice when no exclusions", func(t *testing.T) {
		tmpDir := t.TempDir()
		ef, err := NewExclusionsFile(tmpDir)
		require.NoError(t, err)

		items := ef.GetAll()

		assert.Empty(t, items)
		assert.NotNil(t, items)
	})
}

func TestExclusionsFile_IsExcluded(t *testing.T) {
	t.Run("returns true for excluded item", func(t *testing.T) {
		tmpDir := t.TempDir()
		ef, err := NewExclusionsFile(tmpDir)
		require.NoError(t, err)

		ef.Add(ExclusionItem{ExternalID: "movie-1"})

		assert.True(t, ef.IsExcluded("movie-1"))
	})

	t.Run("returns false for non-excluded item", func(t *testing.T) {
		tmpDir := t.TempDir()
		ef, err := NewExclusionsFile(tmpDir)
		require.NoError(t, err)

		assert.False(t, ef.IsExcluded("movie-1"))
	})
}

func TestExclusionsFile_ConcurrentAccess(t *testing.T) {
	t.Run("handles concurrent reads and writes", func(t *testing.T) {
		tmpDir := t.TempDir()
		ef, err := NewExclusionsFile(tmpDir)
		require.NoError(t, err)

		// Add initial items
		for i := 0; i < 10; i++ {
			ef.Add(ExclusionItem{
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
					_ = ef.GetAll()
					_ = ef.IsExcluded("a")
					_, _ = ef.Get("b")
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
					_ = ef.Add(ExclusionItem{
						ExternalID: string(rune('k' + id)),
						Title:      "Concurrent Item",
					})
				}
			}(i)
		}

		// Wait for all goroutines
		for i := 0; i < 6; i++ {
			<-done
		}
	})
}
