package services

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ramonskie/prunarr/internal/clients"
	"github.com/ramonskie/prunarr/internal/config"
	"github.com/ramonskie/prunarr/internal/models"
	"github.com/rs/zerolog/log"
)

// JellyfinVirtualFolderClient defines the interface for Virtual Folder operations
type JellyfinVirtualFolderClient interface {
	GetVirtualFolders(ctx context.Context) ([]clients.JellyfinVirtualFolder, error)
	CreateVirtualFolder(ctx context.Context, name, collectionType string, paths []string, dryRun bool) error
	DeleteVirtualFolder(ctx context.Context, name string, dryRun bool) error
	AddPathToVirtualFolder(ctx context.Context, name, path string, dryRun bool) error
	RefreshLibrary(ctx context.Context, dryRun bool) error
}

// SymlinkLibraryManager manages Jellyfin symlink-based libraries for "Leaving Soon" items
type SymlinkLibraryManager struct {
	jellyfinClient JellyfinVirtualFolderClient
	config         *config.Config
	dryRun         bool
}

// NewSymlinkLibraryManager creates a new symlink library manager
func NewSymlinkLibraryManager(jellyfinClient JellyfinVirtualFolderClient, cfg *config.Config) *SymlinkLibraryManager {
	return &SymlinkLibraryManager{
		jellyfinClient: jellyfinClient,
		config:         cfg,
		dryRun:         cfg.App.DryRun,
	}
}

// SyncLibraries synchronizes symlink-based Jellyfin libraries with scheduled deletions
func (m *SymlinkLibraryManager) SyncLibraries(ctx context.Context, mediaLibrary map[string]models.Media) error {
	cfg := config.Get()
	if cfg == nil {
		log.Debug().Msg("Config not available, using stored config")
		cfg = m.config
	}

	// Check if symlink libraries are enabled
	if !cfg.Integrations.Jellyfin.SymlinkLibrary.Enabled {
		log.Debug().Msg("Symlink libraries disabled, skipping")
		return nil
	}

	symlinkCfg := cfg.Integrations.Jellyfin.SymlinkLibrary

	// Apply defaults for library names if not specified
	moviesLibraryName := symlinkCfg.MoviesLibraryName
	if moviesLibraryName == "" {
		moviesLibraryName = "Leaving Soon - Movies"
	}
	tvLibraryName := symlinkCfg.TVLibraryName
	if tvLibraryName == "" {
		tvLibraryName = "Leaving Soon - TV Shows"
	}

	log.Info().
		Bool("dry_run", cfg.App.DryRun).
		Str("base_path", symlinkCfg.BasePath).
		Str("movies_library", moviesLibraryName).
		Str("tv_library", tvLibraryName).
		Msg("Starting symlink library sync")

	// Separate media into movies and TV shows that are scheduled for deletion
	movies, tvShows := m.filterScheduledMedia(mediaLibrary)

	log.Debug().
		Int("movies", len(movies)).
		Int("tv_shows", len(tvShows)).
		Msg("Filtered scheduled media")

	// Sync movie library
	moviePath := filepath.Join(symlinkCfg.BasePath, "movies")
	if err := m.syncLibrary(ctx, moviesLibraryName, "movies", moviePath, movies); err != nil {
		log.Error().Err(err).Str("type", "movies").Msg("Failed to sync movie library")
		return err
	}

	// Sync TV show library
	tvPath := filepath.Join(symlinkCfg.BasePath, "tv")
	if err := m.syncLibrary(ctx, tvLibraryName, "tvshows", tvPath, tvShows); err != nil {
		log.Error().Err(err).Str("type", "tv_shows").Msg("Failed to sync TV show library")
		return err
	}

	log.Info().Msg("Symlink library sync completed")
	return nil
}

// filterScheduledMedia separates media into movies and TV shows scheduled for deletion
func (m *SymlinkLibraryManager) filterScheduledMedia(mediaLibrary map[string]models.Media) ([]models.Media, []models.Media) {
	movies := make([]models.Media, 0)
	tvShows := make([]models.Media, 0)
	now := time.Now()

	for _, media := range mediaLibrary {
		// Skip excluded items
		if media.IsExcluded {
			continue
		}

		// Skip items without deletion dates
		if media.DeleteAfter.IsZero() {
			continue
		}

		// Skip items without Jellyfin ID (can't create symlinks without mapping)
		if media.JellyfinID == "" {
			continue
		}

		// Only include future deletions (leaving soon items)
		if media.DeleteAfter.After(now) {
			switch media.Type {
			case models.MediaTypeMovie:
				movies = append(movies, media)
			case models.MediaTypeTVShow:
				tvShows = append(tvShows, media)
			}
		}
	}

	return movies, tvShows
}

// syncLibrary syncs a single symlink library (movies or TV shows)
func (m *SymlinkLibraryManager) syncLibrary(ctx context.Context, libraryName, collectionType, symlinkDir string, items []models.Media) error {
	cfg := config.Get()
	if cfg == nil {
		cfg = m.config
	}
	dryRun := cfg.App.DryRun

	log.Info().
		Str("library", libraryName).
		Str("path", symlinkDir).
		Int("item_count", len(items)).
		Bool("dry_run", dryRun).
		Msg("Syncing symlink library")

	// Check if library should be hidden when empty
	if len(items) == 0 && cfg.Integrations.Jellyfin.SymlinkLibrary.HideWhenEmpty {
		log.Info().
			Str("library", libraryName).
			Bool("dry_run", dryRun).
			Msg("Library is empty and hide_when_empty is true, removing library from Jellyfin")

		// Check if virtual folder exists before attempting deletion
		log.Info().Str("library", libraryName).Msg("Fetching virtual folders from Jellyfin")
		folders, err := m.jellyfinClient.GetVirtualFolders(ctx)
		if err != nil {
			log.Warn().
				Err(err).
				Str("library", libraryName).
				Msg("Failed to check for existing virtual folder, skipping deletion")
			return nil // Don't fail entire sync
		}

		log.Info().
			Str("library", libraryName).
			Int("folder_count", len(folders)).
			Msg("Retrieved virtual folders from Jellyfin")

		// Log all folder names for debugging
		for _, f := range folders {
			log.Info().
				Str("folder_name", f.Name).
				Str("searching_for", libraryName).
				Bool("matches", f.Name == libraryName).
				Msg("Checking virtual folder")
		}

		// Delete the virtual folder if it exists
		for _, folder := range folders {
			if folder.Name == libraryName {
				log.Info().
					Str("library", libraryName).
					Msg("Found matching virtual folder, proceeding with deletion")
				if err := m.jellyfinClient.DeleteVirtualFolder(ctx, libraryName, dryRun); err != nil {
					log.Warn().
						Err(err).
						Str("library", libraryName).
						Msg("Failed to delete empty virtual folder")
					return nil // Don't fail entire sync
				}
				log.Info().
					Str("library", libraryName).
					Bool("dry_run", dryRun).
					Msg("Empty library removed from Jellyfin")
				return nil
			}
		}

		// Library doesn't exist, nothing to do
		log.Debug().
			Str("library", libraryName).
			Msg("Library doesn't exist in Jellyfin, nothing to delete")
		return nil
	}

	// Step 1: Ensure virtual folder exists in Jellyfin
	if err := m.ensureVirtualFolder(ctx, libraryName, collectionType, symlinkDir, dryRun); err != nil {
		return fmt.Errorf("failed to ensure virtual folder: %w", err)
	}

	// Step 2: Create symlink directory if needed
	if err := m.ensureDirectory(symlinkDir, dryRun); err != nil {
		return fmt.Errorf("failed to create symlink directory: %w", err)
	}

	// Step 3: Create/update symlinks for scheduled items
	currentSymlinks, err := m.createSymlinks(symlinkDir, items, dryRun)
	if err != nil {
		return fmt.Errorf("failed to create symlinks: %w", err)
	}

	// Step 4: Clean up stale symlinks
	if err := m.cleanupSymlinks(symlinkDir, currentSymlinks, dryRun); err != nil {
		return fmt.Errorf("failed to cleanup symlinks: %w", err)
	}

	// Step 5: Trigger Jellyfin library scan to discover new content
	if len(items) > 0 || len(currentSymlinks) > 0 {
		log.Info().
			Str("library", libraryName).
			Int("symlinks", len(currentSymlinks)).
			Msg("Triggering Jellyfin library refresh to scan new content")

		if err := m.jellyfinClient.RefreshLibrary(ctx, dryRun); err != nil {
			// Log warning but don't fail entire sync - library will scan eventually
			log.Warn().
				Err(err).
				Str("library", libraryName).
				Msg("Failed to trigger library refresh, content may not appear immediately")
		}
	}

	log.Info().
		Str("library", libraryName).
		Int("symlinks_created", len(currentSymlinks)).
		Bool("dry_run", dryRun).
		Msg("Symlink library sync completed")

	return nil
}

// ensureVirtualFolder ensures the Jellyfin virtual folder exists
func (m *SymlinkLibraryManager) ensureVirtualFolder(ctx context.Context, name, collectionType, path string, dryRun bool) error {
	// Check if virtual folder already exists
	folders, err := m.jellyfinClient.GetVirtualFolders(ctx)
	if err != nil {
		return fmt.Errorf("failed to get virtual folders: %w", err)
	}

	// Check if folder already exists
	for _, folder := range folders {
		if folder.Name == name {
			log.Debug().
				Str("name", name).
				Str("collection_type", folder.CollectionType).
				Msg("Virtual folder already exists")

			// Check if path is already added
			hasPath := false
			for _, loc := range folder.Locations {
				if loc == path {
					hasPath = true
					break
				}
			}

			// Add path if missing
			if !hasPath {
				log.Info().
					Str("name", name).
					Str("path", path).
					Bool("dry_run", dryRun).
					Msg("Adding path to existing virtual folder")

				if err := m.jellyfinClient.AddPathToVirtualFolder(ctx, name, path, dryRun); err != nil {
					return fmt.Errorf("failed to add path to virtual folder: %w", err)
				}
			}

			return nil
		}
	}

	// Create new virtual folder
	log.Info().
		Str("name", name).
		Str("collection_type", collectionType).
		Str("path", path).
		Bool("dry_run", dryRun).
		Msg("Creating new virtual folder")

	if err := m.jellyfinClient.CreateVirtualFolder(ctx, name, collectionType, []string{path}, dryRun); err != nil {
		return fmt.Errorf("failed to create virtual folder: %w", err)
	}

	return nil
}

// ensureDirectory creates the directory if it doesn't exist
func (m *SymlinkLibraryManager) ensureDirectory(path string, dryRun bool) error {
	// Check if directory exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		log.Info().
			Str("path", path).
			Bool("dry_run", dryRun).
			Msg("Creating symlink directory")

		if !dryRun {
			if err := os.MkdirAll(path, 0755); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}
		}
	} else {
		log.Debug().Str("path", path).Msg("Directory already exists")
	}

	return nil
}

// createSymlinks creates symlinks for media items and returns a map of created symlink names
func (m *SymlinkLibraryManager) createSymlinks(symlinkDir string, items []models.Media, dryRun bool) (map[string]bool, error) {
	currentSymlinks := make(map[string]bool)

	for _, media := range items {
		// Skip items without file paths
		if media.FilePath == "" {
			log.Warn().
				Str("title", media.Title).
				Str("media_id", media.ID).
				Msg("Media has no file path, skipping symlink creation")
			continue
		}

		// Check if source file exists (skip if missing)
		if _, err := os.Stat(media.FilePath); os.IsNotExist(err) {
			log.Warn().
				Str("title", media.Title).
				Str("file_path", media.FilePath).
				Msg("Source file does not exist, skipping symlink creation")
			continue
		}

		// Generate safe symlink name
		symlinkName := m.generateSymlinkName(media)
		symlinkPath := filepath.Join(symlinkDir, symlinkName)

		// Check if symlink already exists and points to correct target
		if existingTarget, err := os.Readlink(symlinkPath); err == nil {
			if existingTarget == media.FilePath {
				log.Debug().
					Str("symlink", symlinkName).
					Str("target", media.FilePath).
					Msg("Symlink already exists and is correct")
				currentSymlinks[symlinkName] = true
				continue
			} else {
				// Symlink exists but points to wrong target - remove it
				log.Info().
					Str("symlink", symlinkName).
					Str("old_target", existingTarget).
					Str("new_target", media.FilePath).
					Bool("dry_run", dryRun).
					Msg("Removing stale symlink")

				if !dryRun {
					if err := os.Remove(symlinkPath); err != nil {
						log.Error().Err(err).Str("path", symlinkPath).Msg("Failed to remove stale symlink")
						continue
					}
				}
			}
		}

		// Create symlink
		log.Info().
			Str("symlink", symlinkName).
			Str("target", media.FilePath).
			Bool("dry_run", dryRun).
			Msg("Creating symlink")

		if !dryRun {
			if err := os.Symlink(media.FilePath, symlinkPath); err != nil {
				log.Error().
					Err(err).
					Str("symlink", symlinkPath).
					Str("target", media.FilePath).
					Msg("Failed to create symlink")
				continue
			}
		}

		// Track successfully created symlink
		currentSymlinks[symlinkName] = true
	}

	return currentSymlinks, nil
}

// cleanupSymlinks removes symlinks that are no longer needed
func (m *SymlinkLibraryManager) cleanupSymlinks(symlinkDir string, currentSymlinks map[string]bool, dryRun bool) error {
	// Read directory contents
	entries, err := os.ReadDir(symlinkDir)
	if err != nil {
		if os.IsNotExist(err) {
			log.Debug().Str("path", symlinkDir).Msg("Symlink directory does not exist, nothing to clean up")
			return nil
		}
		return fmt.Errorf("failed to read symlink directory: %w", err)
	}

	// Remove symlinks not in current set
	for _, entry := range entries {
		if !currentSymlinks[entry.Name()] {
			symlinkPath := filepath.Join(symlinkDir, entry.Name())

			// Only remove if it's actually a symlink
			info, err := os.Lstat(symlinkPath)
			if err != nil {
				log.Warn().Err(err).Str("path", symlinkPath).Msg("Failed to stat file")
				continue
			}

			if info.Mode()&os.ModeSymlink != 0 {
				log.Info().
					Str("symlink", entry.Name()).
					Bool("dry_run", dryRun).
					Msg("Removing stale symlink")

				if !dryRun {
					if err := os.Remove(symlinkPath); err != nil {
						log.Error().Err(err).Str("path", symlinkPath).Msg("Failed to remove symlink")
					}
				}
			}
		}
	}

	return nil
}

// generateSymlinkName creates a safe filename for the symlink
func (m *SymlinkLibraryManager) generateSymlinkName(media models.Media) string {
	// Use original filename if available
	if media.FilePath != "" {
		ext := filepath.Ext(media.FilePath)
		base := filepath.Base(media.FilePath)

		// For files, use the original filename
		if ext != "" {
			return base
		}
	}

	// Fallback: generate name from title and year
	name := media.Title
	if media.Year > 0 {
		name = fmt.Sprintf("%s (%d)", name, media.Year)
	}

	// Sanitize filename (remove unsafe characters)
	name = strings.ReplaceAll(name, "/", "-")
	name = strings.ReplaceAll(name, "\\", "-")
	name = strings.ReplaceAll(name, ":", "-")
	name = strings.ReplaceAll(name, "*", "-")
	name = strings.ReplaceAll(name, "?", "-")
	name = strings.ReplaceAll(name, "\"", "-")
	name = strings.ReplaceAll(name, "<", "-")
	name = strings.ReplaceAll(name, ">", "-")
	name = strings.ReplaceAll(name, "|", "-")

	return name
}
