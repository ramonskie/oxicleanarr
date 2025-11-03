package services

import (
	"context"
	"testing"
	"time"

	"github.com/ramonskie/prunarr/internal/clients"
	"github.com/ramonskie/prunarr/internal/config"
	"github.com/ramonskie/prunarr/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock Jellyfin client for testing collections
type mockJellyfinClientForCollections struct {
	collections      map[string]*clients.JellyfinCollection
	getError         error
	createError      error
	addItemsError    error
	removeItemsError error
	deleteError      error
	createCallCount  int
	addCallCount     int
	deleteCallCount  int
}

func newMockJellyfinClientForCollections() *mockJellyfinClientForCollections {
	return &mockJellyfinClientForCollections{
		collections: make(map[string]*clients.JellyfinCollection),
	}
}

func (m *mockJellyfinClientForCollections) GetCollectionByName(ctx context.Context, name string) (*clients.JellyfinCollection, error) {
	if m.getError != nil {
		return nil, m.getError
	}
	collection, exists := m.collections[name]
	if !exists {
		return nil, nil
	}
	return collection, nil
}

func (m *mockJellyfinClientForCollections) CreateCollection(ctx context.Context, name string, itemIDs []string, dryRun bool) (string, error) {
	if m.createError != nil {
		return "", m.createError
	}
	m.createCallCount++
	if dryRun {
		return "dry-run-id", nil
	}
	id := "collection-" + name
	m.collections[name] = &clients.JellyfinCollection{
		ID:   id,
		Name: name,
	}
	return id, nil
}

func (m *mockJellyfinClientForCollections) AddItemsToCollection(ctx context.Context, collectionID string, itemIDs []string, dryRun bool) error {
	if m.addItemsError != nil {
		return m.addItemsError
	}
	m.addCallCount++
	return nil
}

func (m *mockJellyfinClientForCollections) RemoveItemsFromCollection(ctx context.Context, collectionID string, itemIDs []string, dryRun bool) error {
	if m.removeItemsError != nil {
		return m.removeItemsError
	}
	return nil
}

func (m *mockJellyfinClientForCollections) DeleteCollection(ctx context.Context, collectionID string, dryRun bool) error {
	if m.deleteError != nil {
		return m.deleteError
	}
	m.deleteCallCount++
	if dryRun {
		return nil
	}
	// Find and remove collection by ID
	for name, coll := range m.collections {
		if coll.ID == collectionID {
			delete(m.collections, name)
			break
		}
	}
	return nil
}

// Mock now only needs to implement JellyfinCollectionClient interface (no extra methods needed)

func TestNewJellyfinCollectionManager(t *testing.T) {
	client := newMockJellyfinClientForCollections()
	cfg := &config.CollectionsConfig{
		Enabled: true,
		Movies: config.CollectionItemConfig{
			Name:          "Test Movies",
			HideWhenEmpty: true,
		},
		TVShows: config.CollectionItemConfig{
			Name:          "Test TV Shows",
			HideWhenEmpty: false,
		},
	}

	manager := NewJellyfinCollectionManager(client, cfg, true)

	assert.NotNil(t, manager)
	assert.True(t, manager.dryRun)
}

func TestSyncCollections_Disabled(t *testing.T) {
	client := newMockJellyfinClientForCollections()
	cfg := &config.CollectionsConfig{
		Enabled: false,
	}
	manager := NewJellyfinCollectionManager(client, cfg, false)

	mediaLibrary := map[string]models.Media{
		"movie-1": createMockMediaWithJellyfinID("movie-1", models.MediaTypeMovie, "jf-movie-1", 60, -1, false, false),
	}

	err := manager.SyncCollections(context.Background(), mediaLibrary)

	assert.NoError(t, err)
	assert.Equal(t, 0, client.createCallCount, "Should not create collections when disabled")
}

func TestSyncCollections_CreateMovieCollection(t *testing.T) {
	client := newMockJellyfinClientForCollections()
	cfg := &config.CollectionsConfig{
		Enabled: true,
		Movies: config.CollectionItemConfig{
			Name:          "Movies Leaving Soon",
			HideWhenEmpty: true,
		},
		TVShows: config.CollectionItemConfig{
			Name:          "TV Shows Leaving Soon",
			HideWhenEmpty: true,
		},
	}
	manager := NewJellyfinCollectionManager(client, cfg, false)

	// Create media with future deletion dates
	now := time.Now()
	mediaLibrary := map[string]models.Media{
		"movie-1": createMockMediaWithDeleteAfter("movie-1", models.MediaTypeMovie, "jf-movie-1", now.Add(10*24*time.Hour)),
		"movie-2": createMockMediaWithDeleteAfter("movie-2", models.MediaTypeMovie, "jf-movie-2", now.Add(20*24*time.Hour)),
	}

	err := manager.SyncCollections(context.Background(), mediaLibrary)

	assert.NoError(t, err)
	assert.Equal(t, 1, client.createCallCount, "Should create movie collection")
	assert.Contains(t, client.collections, "Movies Leaving Soon")
}

func TestSyncCollections_CreateTVShowCollection(t *testing.T) {
	client := newMockJellyfinClientForCollections()
	cfg := &config.CollectionsConfig{
		Enabled: true,
		Movies: config.CollectionItemConfig{
			Name:          "Movies Leaving Soon",
			HideWhenEmpty: true,
		},
		TVShows: config.CollectionItemConfig{
			Name:          "TV Shows Leaving Soon",
			HideWhenEmpty: true,
		},
	}
	manager := NewJellyfinCollectionManager(client, cfg, false)

	now := time.Now()
	mediaLibrary := map[string]models.Media{
		"show-1": createMockMediaWithDeleteAfter("show-1", models.MediaTypeTVShow, "jf-show-1", now.Add(10*24*time.Hour)),
		"show-2": createMockMediaWithDeleteAfter("show-2", models.MediaTypeTVShow, "jf-show-2", now.Add(20*24*time.Hour)),
	}

	err := manager.SyncCollections(context.Background(), mediaLibrary)

	assert.NoError(t, err)
	assert.Equal(t, 1, client.createCallCount, "Should create TV show collection")
	assert.Contains(t, client.collections, "TV Shows Leaving Soon")
}

func TestSyncCollections_SeparatesByType(t *testing.T) {
	client := newMockJellyfinClientForCollections()
	cfg := &config.CollectionsConfig{
		Enabled: true,
		Movies: config.CollectionItemConfig{
			Name:          "Movies Leaving Soon",
			HideWhenEmpty: true,
		},
		TVShows: config.CollectionItemConfig{
			Name:          "TV Shows Leaving Soon",
			HideWhenEmpty: true,
		},
	}
	manager := NewJellyfinCollectionManager(client, cfg, false)

	now := time.Now()
	mediaLibrary := map[string]models.Media{
		"movie-1": createMockMediaWithDeleteAfter("movie-1", models.MediaTypeMovie, "jf-movie-1", now.Add(10*24*time.Hour)),
		"show-1":  createMockMediaWithDeleteAfter("show-1", models.MediaTypeTVShow, "jf-show-1", now.Add(10*24*time.Hour)),
	}

	err := manager.SyncCollections(context.Background(), mediaLibrary)

	assert.NoError(t, err)
	assert.Equal(t, 2, client.createCallCount, "Should create both movie and TV collections")
	assert.Contains(t, client.collections, "Movies Leaving Soon")
	assert.Contains(t, client.collections, "TV Shows Leaving Soon")
}

func TestSyncCollections_SkipsExcludedItems(t *testing.T) {
	client := newMockJellyfinClientForCollections()
	cfg := &config.CollectionsConfig{
		Enabled: true,
		Movies: config.CollectionItemConfig{
			Name:          "Movies Leaving Soon",
			HideWhenEmpty: true,
		},
		TVShows: config.CollectionItemConfig{
			Name:          "TV Shows Leaving Soon",
			HideWhenEmpty: true,
		},
	}
	manager := NewJellyfinCollectionManager(client, cfg, false)

	now := time.Now()
	mediaLibrary := map[string]models.Media{
		"movie-1": createMockMediaWithDeleteAfterAndExcluded("movie-1", models.MediaTypeMovie, "jf-movie-1", now.Add(10*24*time.Hour), true),
		"movie-2": createMockMediaWithDeleteAfterAndExcluded("movie-2", models.MediaTypeMovie, "jf-movie-2", now.Add(10*24*time.Hour), false),
	}

	err := manager.SyncCollections(context.Background(), mediaLibrary)

	assert.NoError(t, err)
	// Should still create collection with 1 non-excluded movie
	assert.Equal(t, 1, client.createCallCount)
}

func TestSyncCollections_SkipsItemsWithoutJellyfinID(t *testing.T) {
	client := newMockJellyfinClientForCollections()
	cfg := &config.CollectionsConfig{
		Enabled: true,
		Movies: config.CollectionItemConfig{
			Name:          "Movies Leaving Soon",
			HideWhenEmpty: true,
		},
		TVShows: config.CollectionItemConfig{
			Name:          "TV Shows Leaving Soon",
			HideWhenEmpty: true,
		},
	}
	manager := NewJellyfinCollectionManager(client, cfg, false)

	now := time.Now()
	mediaLibrary := map[string]models.Media{
		"movie-1": createMockMediaWithDeleteAfter("movie-1", models.MediaTypeMovie, "", now.Add(10*24*time.Hour)), // No JellyfinID
		"movie-2": createMockMediaWithDeleteAfter("movie-2", models.MediaTypeMovie, "jf-movie-2", now.Add(10*24*time.Hour)),
	}

	err := manager.SyncCollections(context.Background(), mediaLibrary)

	assert.NoError(t, err)
	// Should create collection with 1 movie that has JellyfinID
	assert.Equal(t, 1, client.createCallCount)
}

func TestSyncCollections_SkipsPastDeletionDates(t *testing.T) {
	client := newMockJellyfinClientForCollections()
	cfg := &config.CollectionsConfig{
		Enabled: true,
		Movies: config.CollectionItemConfig{
			Name:          "Movies Leaving Soon",
			HideWhenEmpty: true,
		},
		TVShows: config.CollectionItemConfig{
			Name:          "TV Shows Leaving Soon",
			HideWhenEmpty: true,
		},
	}
	manager := NewJellyfinCollectionManager(client, cfg, false)

	now := time.Now()
	mediaLibrary := map[string]models.Media{
		"movie-1": createMockMediaWithDeleteAfter("movie-1", models.MediaTypeMovie, "jf-movie-1", now.Add(-10*24*time.Hour)), // Past deletion date
		"movie-2": createMockMediaWithDeleteAfter("movie-2", models.MediaTypeMovie, "jf-movie-2", now.Add(10*24*time.Hour)),  // Future deletion date
	}

	err := manager.SyncCollections(context.Background(), mediaLibrary)

	assert.NoError(t, err)
	// Should create collection with only future deletion date
	assert.Equal(t, 1, client.createCallCount)
}

func TestSyncCollections_DeletesEmptyCollectionWithHideWhenEmpty(t *testing.T) {
	client := newMockJellyfinClientForCollections()
	cfg := &config.CollectionsConfig{
		Enabled: true,
		Movies: config.CollectionItemConfig{
			Name:          "Movies Leaving Soon",
			HideWhenEmpty: true,
		},
		TVShows: config.CollectionItemConfig{
			Name:          "TV Shows Leaving Soon",
			HideWhenEmpty: true,
		},
	}

	// Pre-create a collection
	client.collections["Movies Leaving Soon"] = &clients.JellyfinCollection{
		ID:   "existing-collection-id",
		Name: "Movies Leaving Soon",
	}

	manager := NewJellyfinCollectionManager(client, cfg, false)

	// Empty media library - no items scheduled for deletion
	mediaLibrary := map[string]models.Media{}

	err := manager.SyncCollections(context.Background(), mediaLibrary)

	assert.NoError(t, err)
	assert.Equal(t, 1, client.deleteCallCount, "Should delete empty collection when hide_when_empty is true")
	assert.NotContains(t, client.collections, "Movies Leaving Soon", "Collection should be removed")
}

func TestSyncCollections_KeepsEmptyCollectionWithoutHideWhenEmpty(t *testing.T) {
	client := newMockJellyfinClientForCollections()
	cfg := &config.CollectionsConfig{
		Enabled: true,
		Movies: config.CollectionItemConfig{
			Name:          "Movies Leaving Soon",
			HideWhenEmpty: false, // Keep even when empty
		},
		TVShows: config.CollectionItemConfig{
			Name:          "TV Shows Leaving Soon",
			HideWhenEmpty: false,
		},
	}

	// Pre-create a collection
	client.collections["Movies Leaving Soon"] = &clients.JellyfinCollection{
		ID:   "existing-collection-id",
		Name: "Movies Leaving Soon",
	}

	manager := NewJellyfinCollectionManager(client, cfg, false)

	// Empty media library
	mediaLibrary := map[string]models.Media{}

	err := manager.SyncCollections(context.Background(), mediaLibrary)

	assert.NoError(t, err)
	assert.Equal(t, 0, client.deleteCallCount, "Should NOT delete collection when hide_when_empty is false")
	assert.Contains(t, client.collections, "Movies Leaving Soon", "Collection should remain")
}

func TestSyncCollections_UpdatesExistingCollection(t *testing.T) {
	client := newMockJellyfinClientForCollections()
	cfg := &config.CollectionsConfig{
		Enabled: true,
		Movies: config.CollectionItemConfig{
			Name:          "Movies Leaving Soon",
			HideWhenEmpty: true,
		},
		TVShows: config.CollectionItemConfig{
			Name:          "TV Shows Leaving Soon",
			HideWhenEmpty: true,
		},
	}

	// Pre-create a collection
	client.collections["Movies Leaving Soon"] = &clients.JellyfinCollection{
		ID:   "existing-collection-id",
		Name: "Movies Leaving Soon",
	}

	manager := NewJellyfinCollectionManager(client, cfg, false)

	now := time.Now()
	mediaLibrary := map[string]models.Media{
		"movie-1": createMockMediaWithDeleteAfter("movie-1", models.MediaTypeMovie, "jf-movie-1", now.Add(10*24*time.Hour)),
	}

	err := manager.SyncCollections(context.Background(), mediaLibrary)

	assert.NoError(t, err)
	assert.Equal(t, 0, client.createCallCount, "Should not create new collection")
	assert.Equal(t, 1, client.addCallCount, "Should update existing collection")
	assert.Contains(t, client.collections, "Movies Leaving Soon")
}

func TestSyncCollections_DryRunMode(t *testing.T) {
	client := newMockJellyfinClientForCollections()
	cfg := &config.CollectionsConfig{
		Enabled: true,
		Movies: config.CollectionItemConfig{
			Name:          "Movies Leaving Soon",
			HideWhenEmpty: true,
		},
		TVShows: config.CollectionItemConfig{
			Name:          "TV Shows Leaving Soon",
			HideWhenEmpty: true,
		},
	}
	manager := NewJellyfinCollectionManager(client, cfg, true) // Dry run enabled

	now := time.Now()
	mediaLibrary := map[string]models.Media{
		"movie-1": createMockMediaWithDeleteAfter("movie-1", models.MediaTypeMovie, "jf-movie-1", now.Add(10*24*time.Hour)),
	}

	err := manager.SyncCollections(context.Background(), mediaLibrary)

	assert.NoError(t, err)
	assert.Equal(t, 1, client.createCallCount, "Should call create in dry-run")
	// In dry-run, actual collection isn't created in mock
}

// Helper functions for collection tests

func createMockMediaWithJellyfinID(id string, mediaType models.MediaType, jellyfinID string, addedDaysAgo, lastWatchedDaysAgo int, isRequested, isExcluded bool) models.Media {
	media := createMockMedia(id, mediaType, addedDaysAgo, lastWatchedDaysAgo, isRequested, isExcluded)
	media.JellyfinID = jellyfinID
	return media
}

func createMockMediaWithDeleteAfter(id string, mediaType models.MediaType, jellyfinID string, deleteAfter time.Time) models.Media {
	now := time.Now()
	media := models.Media{
		ID:          id,
		Type:        mediaType,
		Title:       "Test Media " + id,
		Year:        2024,
		AddedAt:     now.AddDate(0, 0, -60),
		JellyfinID:  jellyfinID,
		DeleteAfter: deleteAfter,
		IsExcluded:  false,
		FileSize:    1024 * 1024 * 1024,
	}
	return media
}

func createMockMediaWithDeleteAfterAndExcluded(id string, mediaType models.MediaType, jellyfinID string, deleteAfter time.Time, isExcluded bool) models.Media {
	media := createMockMediaWithDeleteAfter(id, mediaType, jellyfinID, deleteAfter)
	media.IsExcluded = isExcluded
	return media
}

func TestSyncCollection_EmptyName(t *testing.T) {
	client := newMockJellyfinClientForCollections()
	cfg := &config.CollectionsConfig{
		Enabled: true,
		Movies: config.CollectionItemConfig{
			Name:          "",
			HideWhenEmpty: true,
		},
	}
	manager := NewJellyfinCollectionManager(client, cfg, false)

	err := manager.syncCollection(context.Background(), "", []string{"item-1"}, true)

	// Should handle empty name gracefully (skip or create with default name)
	require.NoError(t, err)
}
