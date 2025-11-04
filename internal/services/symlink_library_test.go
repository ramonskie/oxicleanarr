package services

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ramonskie/prunarr/internal/clients"
	"github.com/ramonskie/prunarr/internal/config"
	"github.com/ramonskie/prunarr/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock Jellyfin client for testing
type mockJellyfinClientForSymlink struct {
	virtualFolders    []clients.JellyfinVirtualFolder
	createCalled      int
	deleteCalled      int
	addPathCalled     int
	lastCreatedFolder *clients.JellyfinVirtualFolder
}

func (m *mockJellyfinClientForSymlink) GetVirtualFolders(ctx context.Context) ([]clients.JellyfinVirtualFolder, error) {
	return m.virtualFolders, nil
}

func (m *mockJellyfinClientForSymlink) CreateVirtualFolder(ctx context.Context, name, collectionType string, paths []string, dryRun bool) error {
	m.createCalled++
	if !dryRun {
		m.lastCreatedFolder = &clients.JellyfinVirtualFolder{
			Name:           name,
			CollectionType: collectionType,
			Locations:      paths,
		}
		m.virtualFolders = append(m.virtualFolders, *m.lastCreatedFolder)
	}
	return nil
}

func (m *mockJellyfinClientForSymlink) DeleteVirtualFolder(ctx context.Context, name string, dryRun bool) error {
	m.deleteCalled++
	if !dryRun {
		for i, vf := range m.virtualFolders {
			if vf.Name == name {
				m.virtualFolders = append(m.virtualFolders[:i], m.virtualFolders[i+1:]...)
				break
			}
		}
	}
	return nil
}

func (m *mockJellyfinClientForSymlink) AddPathToVirtualFolder(ctx context.Context, name, path string, dryRun bool) error {
	m.addPathCalled++
	if !dryRun {
		for i, vf := range m.virtualFolders {
			if vf.Name == name {
				m.virtualFolders[i].Locations = append(m.virtualFolders[i].Locations, path)
				break
			}
		}
	}
	return nil
}

// Ping is required by JellyfinClient interface but not used in tests
func (m *mockJellyfinClientForSymlink) Ping(ctx context.Context) error {
	return nil
}

func (m *mockJellyfinClientForSymlink) GetMovies(ctx context.Context) ([]clients.JellyfinItem, error) {
	return nil, nil
}

func (m *mockJellyfinClientForSymlink) GetTVShows(ctx context.Context) ([]clients.JellyfinItem, error) {
	return nil, nil
}

// Helper function to create test media items
func createTestMediaItemForSymlink(id, title string, mediaType models.MediaType, deletionDate time.Time, excluded bool) models.Media {
	return models.Media{
		ID:           id,
		Type:         mediaType,
		Title:        title,
		Year:         2023,
		FilePath:     fmt.Sprintf("/media/%s/%s.mkv", mediaType, title),
		JellyfinID:   fmt.Sprintf("jellyfin-%s", id),
		DeleteAfter:  deletionDate,
		IsExcluded:   excluded,
		DaysUntilDue: int(time.Until(deletionDate).Hours() / 24),
	}
}

func TestNewSymlinkLibraryManager(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		App: config.AppConfig{
			DryRun: true,
		},
		Integrations: config.IntegrationsConfig{
			Jellyfin: config.JellyfinConfig{
				SymlinkLibrary: config.SymlinkLibraryConfig{
					Enabled:         true,
					SymlinkBasePath: filepath.Join(tmpDir, "symlinks"),
					Movies: config.LibraryItemConfig{
						Name:           "Movies - Leaving Soon",
						CollectionType: "movies",
					},
					TVShows: config.LibraryItemConfig{
						Name:           "TV Shows - Leaving Soon",
						CollectionType: "tvshows",
					},
				},
			},
		},
	}

	mockClient := &mockJellyfinClientForSymlink{}
	manager := NewSymlinkLibraryManager(mockClient, cfg)

	assert.NotNil(t, manager)
	assert.Equal(t, cfg, manager.config)
}

func TestFilterScheduledMedia(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		App: config.AppConfig{DryRun: true},
		Integrations: config.IntegrationsConfig{
			Jellyfin: config.JellyfinConfig{
				SymlinkLibrary: config.SymlinkLibraryConfig{
					Enabled:         true,
					SymlinkBasePath: tmpDir,
					Movies:          config.LibraryItemConfig{Name: "Movies", CollectionType: "movies"},
					TVShows:         config.LibraryItemConfig{Name: "TV Shows", CollectionType: "tvshows"},
				},
			},
		},
	}

	mockClient := &mockJellyfinClientForSymlink{}
	manager := NewSymlinkLibraryManager(mockClient, cfg)

	now := time.Now()
	futureDate := now.Add(7 * 24 * time.Hour)
	pastDate := now.Add(-7 * 24 * time.Hour)
	zeroDate := time.Time{}

	tests := []struct {
		name           string
		mediaLibrary   map[string]models.Media
		expectedMovies int
		expectedTV     int
	}{
		{
			name: "separates movies and TV shows",
			mediaLibrary: map[string]models.Media{
				"m1": createTestMediaItemForSymlink("m1", "Movie 1", models.MediaTypeMovie, futureDate, false),
				"m2": createTestMediaItemForSymlink("m2", "Movie 2", models.MediaTypeMovie, futureDate, false),
				"t1": createTestMediaItemForSymlink("t1", "Show 1", models.MediaTypeTVShow, futureDate, false),
				"t2": createTestMediaItemForSymlink("t2", "Show 2", models.MediaTypeTVShow, futureDate, false),
			},
			expectedMovies: 2,
			expectedTV:     2,
		},
		{
			name: "filters out excluded items",
			mediaLibrary: map[string]models.Media{
				"m1": createTestMediaItemForSymlink("m1", "Movie 1", models.MediaTypeMovie, futureDate, false),
				"m2": createTestMediaItemForSymlink("m2", "Movie 2", models.MediaTypeMovie, futureDate, true), // excluded
			},
			expectedMovies: 1,
			expectedTV:     0,
		},
		{
			name: "filters out past deletion dates",
			mediaLibrary: map[string]models.Media{
				"m1": createTestMediaItemForSymlink("m1", "Movie 1", models.MediaTypeMovie, futureDate, false),
				"m2": createTestMediaItemForSymlink("m2", "Movie 2", models.MediaTypeMovie, pastDate, false), // past date
			},
			expectedMovies: 1,
			expectedTV:     0,
		},
		{
			name: "filters out zero deletion dates",
			mediaLibrary: map[string]models.Media{
				"m1": createTestMediaItemForSymlink("m1", "Movie 1", models.MediaTypeMovie, futureDate, false),
				"m2": createTestMediaItemForSymlink("m2", "Movie 2", models.MediaTypeMovie, zeroDate, false), // zero date
			},
			expectedMovies: 1,
			expectedTV:     0,
		},
		{
			name: "filters out missing Jellyfin IDs",
			mediaLibrary: map[string]models.Media{
				"m1": createTestMediaItemForSymlink("m1", "Movie 1", models.MediaTypeMovie, futureDate, false),
				"m2": func() models.Media {
					m := createTestMediaItemForSymlink("m2", "Movie 2", models.MediaTypeMovie, futureDate, false)
					m.JellyfinID = "" // missing ID
					return m
				}(),
			},
			expectedMovies: 1,
			expectedTV:     0,
		},
		{
			name:           "empty library",
			mediaLibrary:   map[string]models.Media{},
			expectedMovies: 0,
			expectedTV:     0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			movies, tvShows := manager.filterScheduledMedia(tt.mediaLibrary)
			assert.Len(t, movies, tt.expectedMovies, "unexpected movie count")
			assert.Len(t, tvShows, tt.expectedTV, "unexpected TV show count")

			// Verify all returned items are valid
			for _, movie := range movies {
				assert.Equal(t, models.MediaTypeMovie, movie.Type)
				assert.False(t, movie.IsExcluded)
				assert.True(t, movie.DeleteAfter.After(time.Now()))
				assert.NotEmpty(t, movie.JellyfinID)
			}

			for _, show := range tvShows {
				assert.Equal(t, models.MediaTypeTVShow, show.Type)
				assert.False(t, show.IsExcluded)
				assert.True(t, show.DeleteAfter.After(time.Now()))
				assert.NotEmpty(t, show.JellyfinID)
			}
		})
	}
}

func TestGenerateSymlinkName(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		App: config.AppConfig{DryRun: true},
		Integrations: config.IntegrationsConfig{
			Jellyfin: config.JellyfinConfig{
				SymlinkLibrary: config.SymlinkLibraryConfig{
					Enabled:         true,
					SymlinkBasePath: tmpDir,
				},
			},
		},
	}

	mockClient := &mockJellyfinClientForSymlink{}
	manager := NewSymlinkLibraryManager(mockClient, cfg)

	tests := []struct {
		name     string
		media    models.Media
		expected string
	}{
		{
			name: "uses original filename",
			media: models.Media{
				Title:    "The Matrix",
				Year:     1999,
				Type:     models.MediaTypeMovie,
				FilePath: "/media/movies/The Matrix (1999).mkv",
			},
			expected: "The Matrix (1999).mkv",
		},
		{
			name: "generates from title when no extension",
			media: models.Media{
				Title:    "Breaking Bad",
				Year:     2008,
				Type:     models.MediaTypeTVShow,
				FilePath: "/media/tv/breaking-bad",
			},
			expected: "Breaking Bad (2008)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := manager.generateSymlinkName(tt.media)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCreateSymlinks(t *testing.T) {
	tmpDir := t.TempDir()
	symlinkDir := filepath.Join(tmpDir, "symlinks")
	mediaDir := filepath.Join(tmpDir, "media")

	// Create real media files for testing
	require.NoError(t, os.MkdirAll(mediaDir, 0755))
	moviePath := filepath.Join(mediaDir, "test-movie.mkv")
	require.NoError(t, os.WriteFile(moviePath, []byte("test"), 0644))

	cfg := &config.Config{
		App: config.AppConfig{DryRun: false},
		Integrations: config.IntegrationsConfig{
			Jellyfin: config.JellyfinConfig{
				SymlinkLibrary: config.SymlinkLibraryConfig{
					Enabled:         true,
					SymlinkBasePath: symlinkDir,
				},
			},
		},
	}

	mockClient := &mockJellyfinClientForSymlink{}
	manager := NewSymlinkLibraryManager(mockClient, cfg)

	t.Run("creates symlinks successfully", func(t *testing.T) {
		require.NoError(t, os.MkdirAll(symlinkDir, 0755))

		futureDate := time.Now().Add(7 * 24 * time.Hour)
		media := []models.Media{
			{
				ID:          "m1",
				Title:       "Test Movie",
				Year:        2023,
				Type:        models.MediaTypeMovie,
				FilePath:    moviePath,
				DeleteAfter: futureDate,
			},
		}

		created, err := manager.createSymlinks(symlinkDir, media, false)
		require.NoError(t, err)
		assert.Len(t, created, 1)

		// Verify symlink was created (using base filename from moviePath)
		expectedSymlink := filepath.Join(symlinkDir, "test-movie.mkv")
		info, err := os.Lstat(expectedSymlink)
		require.NoError(t, err)
		assert.True(t, info.Mode()&os.ModeSymlink != 0, "should be a symlink")

		// Verify symlink points to correct target
		target, err := os.Readlink(expectedSymlink)
		require.NoError(t, err)
		assert.Equal(t, moviePath, target)
	})

	t.Run("dry-run mode does not create symlinks", func(t *testing.T) {
		dryRunDir := filepath.Join(tmpDir, "dryrun-symlinks")

		futureDate := time.Now().Add(7 * 24 * time.Hour)
		media := []models.Media{
			{
				ID:          "m2",
				Title:       "Dry Run Movie",
				Year:        2023,
				Type:        models.MediaTypeMovie,
				FilePath:    moviePath,
				DeleteAfter: futureDate,
			},
		}

		created, err := manager.createSymlinks(dryRunDir, media, true)
		require.NoError(t, err)
		assert.Len(t, created, 1) // Still tracked in map

		// Verify directory wasn't created
		_, err = os.Stat(dryRunDir)
		assert.True(t, os.IsNotExist(err), "directory should not exist in dry-run mode")
	})

	t.Run("skips missing source files", func(t *testing.T) {
		skipDir := filepath.Join(tmpDir, "skip-symlinks")
		require.NoError(t, os.MkdirAll(skipDir, 0755))

		futureDate := time.Now().Add(7 * 24 * time.Hour)
		media := []models.Media{
			{
				ID:          "m3",
				Title:       "Missing File",
				Year:        2023,
				Type:        models.MediaTypeMovie,
				FilePath:    "/nonexistent/path.mkv",
				DeleteAfter: futureDate,
			},
		}

		// Should not error, just skip the missing file
		created, err := manager.createSymlinks(skipDir, media, false)
		require.NoError(t, err)
		assert.Empty(t, created)
	})
}

func TestCleanupSymlinks(t *testing.T) {
	tmpDir := t.TempDir()
	symlinkDir := filepath.Join(tmpDir, "cleanup-test")
	require.NoError(t, os.MkdirAll(symlinkDir, 0755))

	// Create some test symlinks
	targetFile := filepath.Join(tmpDir, "target.mkv")
	require.NoError(t, os.WriteFile(targetFile, []byte("test"), 0644))

	symlink1 := filepath.Join(symlinkDir, "Keep This (2023).mkv")
	symlink2 := filepath.Join(symlinkDir, "Remove This (2022).mkv")
	require.NoError(t, os.Symlink(targetFile, symlink1))
	require.NoError(t, os.Symlink(targetFile, symlink2))

	cfg := &config.Config{
		App: config.AppConfig{DryRun: false},
		Integrations: config.IntegrationsConfig{
			Jellyfin: config.JellyfinConfig{
				SymlinkLibrary: config.SymlinkLibraryConfig{
					Enabled:         true,
					SymlinkBasePath: symlinkDir,
				},
			},
		},
	}

	mockClient := &mockJellyfinClientForSymlink{}
	manager := NewSymlinkLibraryManager(mockClient, cfg)

	t.Run("removes stale symlinks", func(t *testing.T) {
		expectedSymlinks := map[string]bool{
			"Keep This (2023).mkv": true,
		}

		err := manager.cleanupSymlinks(symlinkDir, expectedSymlinks, false)
		require.NoError(t, err)

		// symlink1 should still exist
		_, err = os.Lstat(symlink1)
		assert.NoError(t, err, "kept symlink should exist")

		// symlink2 should be removed
		_, err = os.Lstat(symlink2)
		assert.True(t, os.IsNotExist(err), "stale symlink should be removed")
	})

	t.Run("dry-run mode does not remove symlinks", func(t *testing.T) {
		// Recreate symlink2 for this test
		require.NoError(t, os.Symlink(targetFile, symlink2))

		err := manager.cleanupSymlinks(symlinkDir, map[string]bool{}, true)
		require.NoError(t, err)

		// Both symlinks should still exist in dry-run
		_, err = os.Lstat(symlink1)
		assert.NoError(t, err)
		_, err = os.Lstat(symlink2)
		assert.NoError(t, err)
	})
}

func TestEnsureVirtualFolder(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		App: config.AppConfig{DryRun: false},
		Integrations: config.IntegrationsConfig{
			Jellyfin: config.JellyfinConfig{
				SymlinkLibrary: config.SymlinkLibraryConfig{
					Enabled:         true,
					SymlinkBasePath: tmpDir,
				},
			},
		},
	}

	t.Run("creates new virtual folder", func(t *testing.T) {
		mockClient := &mockJellyfinClientForSymlink{
			virtualFolders: []clients.JellyfinVirtualFolder{},
		}
		manager := NewSymlinkLibraryManager(mockClient, cfg)

		libraryPath := filepath.Join(tmpDir, "movies")
		err := manager.ensureVirtualFolder(context.Background(), "Movies Leaving", "movies", libraryPath, false)
		require.NoError(t, err)

		assert.Equal(t, 1, mockClient.createCalled)
		assert.Equal(t, 0, mockClient.deleteCalled)
		require.Len(t, mockClient.virtualFolders, 1)
		assert.Equal(t, "Movies Leaving", mockClient.virtualFolders[0].Name)
		assert.Equal(t, "movies", mockClient.virtualFolders[0].CollectionType)
	})

	t.Run("adds path to existing virtual folder with different path", func(t *testing.T) {
		existingPath := filepath.Join(tmpDir, "old-path")
		mockClient := &mockJellyfinClientForSymlink{
			virtualFolders: []clients.JellyfinVirtualFolder{
				{
					Name:           "Movies Leaving",
					CollectionType: "movies",
					Locations:      []string{existingPath},
				},
			},
		}
		manager := NewSymlinkLibraryManager(mockClient, cfg)

		newPath := filepath.Join(tmpDir, "new-path")
		err := manager.ensureVirtualFolder(context.Background(), "Movies Leaving", "movies", newPath, false)
		require.NoError(t, err)

		// Should add path instead of deleting
		assert.Equal(t, 0, mockClient.deleteCalled)
		assert.Equal(t, 1, mockClient.addPathCalled)
	})

	t.Run("skips when folder exists with correct path", func(t *testing.T) {
		libraryPath := filepath.Join(tmpDir, "movies")
		mockClient := &mockJellyfinClientForSymlink{
			virtualFolders: []clients.JellyfinVirtualFolder{
				{
					Name:           "Movies Leaving",
					CollectionType: "movies",
					Locations:      []string{libraryPath},
				},
			},
		}
		manager := NewSymlinkLibraryManager(mockClient, cfg)

		err := manager.ensureVirtualFolder(context.Background(), "Movies Leaving", "movies", libraryPath, false)
		require.NoError(t, err)

		// Should not call create, delete, or add path
		assert.Equal(t, 0, mockClient.createCalled)
		assert.Equal(t, 0, mockClient.deleteCalled)
		assert.Equal(t, 0, mockClient.addPathCalled)
	})

	t.Run("dry-run mode prevents modifications", func(t *testing.T) {
		mockClient := &mockJellyfinClientForSymlink{
			virtualFolders: []clients.JellyfinVirtualFolder{},
		}
		manager := NewSymlinkLibraryManager(mockClient, cfg)

		libraryPath := filepath.Join(tmpDir, "movies-dryrun")
		err := manager.ensureVirtualFolder(context.Background(), "Movies Leaving", "movies", libraryPath, true)
		require.NoError(t, err)

		// Should call create but not actually create (dry-run)
		assert.Equal(t, 1, mockClient.createCalled)
		assert.Empty(t, mockClient.virtualFolders, "should not create folder in dry-run")
	})
}

func TestSyncLibraries_Integration(t *testing.T) {
	tmpDir := t.TempDir()
	symlinkBase := filepath.Join(tmpDir, "symlinks")
	mediaDir := filepath.Join(tmpDir, "media")

	// Create real media files
	require.NoError(t, os.MkdirAll(mediaDir, 0755))
	moviePath := filepath.Join(mediaDir, "movie.mkv")
	tvPath := filepath.Join(mediaDir, "show.mkv")
	require.NoError(t, os.WriteFile(moviePath, []byte("movie"), 0644))
	require.NoError(t, os.WriteFile(tvPath, []byte("tv"), 0644))

	cfg := &config.Config{
		App: config.AppConfig{DryRun: false},
		Integrations: config.IntegrationsConfig{
			Jellyfin: config.JellyfinConfig{
				SymlinkLibrary: config.SymlinkLibraryConfig{
					Enabled:         true,
					SymlinkBasePath: symlinkBase,
					Movies: config.LibraryItemConfig{
						Name:           "Movies - Leaving Soon",
						CollectionType: "movies",
					},
					TVShows: config.LibraryItemConfig{
						Name:           "TV Shows - Leaving Soon",
						CollectionType: "tvshows",
					},
				},
			},
		},
	}

	mockClient := &mockJellyfinClientForSymlink{
		virtualFolders: []clients.JellyfinVirtualFolder{},
	}
	manager := NewSymlinkLibraryManager(mockClient, cfg)

	futureDate := time.Now().Add(7 * 24 * time.Hour)
	mediaLibrary := map[string]models.Media{
		"m1": {
			ID:          "m1",
			Title:       "Test Movie",
			Year:        2023,
			Type:        models.MediaTypeMovie,
			FilePath:    moviePath,
			JellyfinID:  "jf-m1",
			DeleteAfter: futureDate,
			IsExcluded:  false,
		},
		"t1": {
			ID:          "t1",
			Title:       "Test Show",
			Year:        2023,
			Type:        models.MediaTypeTVShow,
			FilePath:    tvPath,
			JellyfinID:  "jf-t1",
			DeleteAfter: futureDate,
			IsExcluded:  false,
		},
	}

	err := manager.SyncLibraries(context.Background(), mediaLibrary)
	require.NoError(t, err)

	// Verify symlinks were created
	movieSymlink := filepath.Join(symlinkBase, "movies", "movie.mkv")
	tvSymlink := filepath.Join(symlinkBase, "tv", "show.mkv")

	_, err = os.Lstat(movieSymlink)
	assert.NoError(t, err, "movie symlink should exist")

	_, err = os.Lstat(tvSymlink)
	assert.NoError(t, err, "TV symlink should exist")

	// Verify virtual folders were created
	assert.Equal(t, 2, mockClient.createCalled)
	assert.Len(t, mockClient.virtualFolders, 2)
}
