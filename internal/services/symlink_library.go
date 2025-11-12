package services

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/ramonskie/oxicleanarr/internal/clients"
	"github.com/ramonskie/oxicleanarr/internal/config"
	"github.com/ramonskie/oxicleanarr/internal/models"
	"github.com/rs/zerolog/log"
)

// JellyfinVirtualFolderClient defines the interface for Virtual Folder and Plugin operations
type JellyfinVirtualFolderClient interface {
	GetVirtualFolders(ctx context.Context) ([]clients.JellyfinVirtualFolder, error)
	CreateVirtualFolder(ctx context.Context, name, collectionType string, paths []string, dryRun bool) error
	DeleteVirtualFolder(ctx context.Context, name string, dryRun bool) error
	AddPathToVirtualFolder(ctx context.Context, name, path string, dryRun bool) error
	RefreshLibrary(ctx context.Context, dryRun bool) error

	// Plugin methods for symlink management
	CheckPluginStatus(ctx context.Context) (*clients.PluginStatusResponse, error)
	AddSymlinks(ctx context.Context, items []clients.PluginSymlinkItem, dryRun bool) (*clients.PluginAddSymlinksResponse, error)
	RemoveSymlinks(ctx context.Context, paths []string, dryRun bool) (*clients.PluginRemoveSymlinksResponse, error)
	ListSymlinks(ctx context.Context, directory string) (*clients.PluginListSymlinksResponse, error)
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

	skippedExcluded := 0
	skippedNoDeleteDate := 0
	skippedPastDeletion := 0
	skippedNoJellyfinID := 0

	for _, media := range mediaLibrary {
		// Skip excluded items
		if media.IsExcluded {
			skippedExcluded++
			continue
		}

		// Skip items without deletion dates
		if media.DeleteAfter.IsZero() {
			skippedNoDeleteDate++
			continue
		}

		// Only include future deletions (leaving soon items)
		if !media.DeleteAfter.After(now) {
			skippedPastDeletion++
			continue
		}

		// Skip items without Jellyfin ID (can't create symlinks without mapping)
		if media.JellyfinID == "" {
			log.Debug().
				Str("title", media.Title).
				Str("media_id", media.ID).
				Str("match_status", media.JellyfinMatchStatus).
				Msg("Skipping media without Jellyfin ID - cannot create symlink without Jellyfin mapping")
			skippedNoJellyfinID++
			continue
		}

		// Add to appropriate list based on media type
		switch media.Type {
		case models.MediaTypeMovie:
			movies = append(movies, media)
		case models.MediaTypeTVShow:
			tvShows = append(tvShows, media)
		}
	}

	log.Info().
		Int("movies", len(movies)).
		Int("tv_shows", len(tvShows)).
		Int("skipped_excluded", skippedExcluded).
		Int("skipped_no_delete_date", skippedNoDeleteDate).
		Int("skipped_past_deletion", skippedPastDeletion).
		Int("skipped_no_jellyfin_id", skippedNoJellyfinID).
		Msg("Filtered scheduled media for symlink library")

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

		// Step 1: Clean up any existing symlinks first
		log.Info().
			Str("library", libraryName).
			Str("path", symlinkDir).
			Msg("Cleaning up symlinks before removing empty library")

		emptySymlinks := make(map[string]bool) // Empty map = remove all symlinks
		if err := m.cleanupSymlinks(ctx, symlinkDir, emptySymlinks, dryRun); err != nil {
			log.Warn().
				Err(err).
				Str("library", libraryName).
				Msg("Failed to cleanup symlinks for empty library")
			// Continue with library deletion anyway
		}

		// Step 2: Check if virtual folder exists before attempting deletion
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

		// Step 3: Delete the virtual folder if it exists
		libraryDeleted := false
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
				libraryDeleted = true
				log.Info().
					Str("library", libraryName).
					Bool("dry_run", dryRun).
					Msg("Empty library removed from Jellyfin")
				break
			}
		}

		// Step 4: Trigger library scan to refresh Jellyfin's dashboard/UI
		if libraryDeleted {
			log.Info().
				Str("library", libraryName).
				Msg("Triggering Jellyfin library refresh to update dashboard after deletion")

			if err := m.jellyfinClient.RefreshLibrary(ctx, dryRun); err != nil {
				log.Warn().
					Err(err).
					Str("library", libraryName).
					Msg("Failed to trigger library refresh after deletion, dashboard may not update immediately")
			}
		} else {
			log.Debug().
				Str("library", libraryName).
				Msg("Library doesn't exist in Jellyfin, nothing to delete")
		}

		return nil
	}

	// Step 1: Ensure virtual folder exists in Jellyfin
	if err := m.ensureVirtualFolder(ctx, libraryName, collectionType, symlinkDir, dryRun); err != nil {
		return fmt.Errorf("failed to ensure virtual folder: %w", err)
	}

	// Step 2: Create/update symlinks for scheduled items (plugin handles directory creation)
	currentSymlinks, err := m.createSymlinks(symlinkDir, items, dryRun)
	if err != nil {
		return fmt.Errorf("failed to create symlinks: %w", err)
	}

	// Step 3: Clean up stale symlinks
	if err := m.cleanupSymlinks(ctx, symlinkDir, currentSymlinks, dryRun); err != nil {
		return fmt.Errorf("failed to cleanup symlinks: %w", err)
	}

	// Step 4: Trigger Jellyfin library scan to discover new content
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
	// Step 1: Ensure the directory exists via plugin before creating Virtual Folder
	// This is required because Jellyfin's CreateVirtualFolder returns 400 Bad Request if the path doesn't exist
	log.Debug().
		Str("path", path).
		Bool("dry_run", dryRun).
		Msg("Ensuring directory exists before creating Virtual Folder")

	// Cast to get access to CreateDirectory method
	type directoryClient interface {
		CreateDirectory(ctx context.Context, path string, dryRun bool) (*clients.PluginCreateDirectoryResponse, error)
	}

	if dirClient, ok := m.jellyfinClient.(directoryClient); ok {
		dirResp, err := dirClient.CreateDirectory(ctx, path, dryRun)
		if err != nil {
			log.Warn().
				Err(err).
				Str("path", path).
				Msg("Failed to create directory via plugin, continuing anyway")
			// Continue - directory might already exist or plugin might not be available
		} else {
			log.Info().
				Str("path", path).
				Bool("created", dirResp.Created).
				Str("message", dirResp.Message).
				Msg("Directory ensured via plugin")
		}
	} else {
		log.Debug().Msg("Client doesn't support CreateDirectory, skipping directory creation")
	}

	// Step 2: Check if virtual folder already exists
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

	// Step 3: Create new virtual folder (directory already exists from Step 1)
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

// createSymlinks creates symlinks for media items via plugin API and returns a map of created symlink names
func (m *SymlinkLibraryManager) createSymlinks(symlinkDir string, items []models.Media, dryRun bool) (map[string]bool, error) {
	ctx := context.Background()
	currentSymlinks := make(map[string]bool)

	log.Info().
		Str("directory", symlinkDir).
		Int("items_to_process", len(items)).
		Bool("dry_run", dryRun).
		Msg("Starting symlink creation")

	// Get existing symlinks from plugin to check what needs updating
	listResp, err := m.jellyfinClient.ListSymlinks(ctx, symlinkDir)
	if err != nil {
		log.Warn().
			Err(err).
			Str("directory", symlinkDir).
			Msg("Failed to list existing symlinks, will create all")
		listResp = &clients.PluginListSymlinksResponse{Symlinks: []clients.PluginSymlinkInfo{}}
	} else {
		log.Info().
			Int("existing_count", len(listResp.Symlinks)).
			Str("directory", symlinkDir).
			Msg("Found existing symlinks")
	}

	// Build map of existing symlinks for quick lookup
	existingSymlinks := make(map[string]string) // path -> target
	for _, symlink := range listResp.Symlinks {
		existingSymlinks[symlink.Path] = symlink.Target
	}

	// Build list of symlinks to create/update
	var symlinkItems []clients.PluginSymlinkItem
	var staleSymlinks []string
	pendingSymlinks := make(map[string]string) // symlinkPath -> symlinkName (for tracking after creation)

	for _, media := range items {
		// Skip items without file paths
		if media.FilePath == "" {
			log.Warn().
				Str("title", media.Title).
				Str("media_id", media.ID).
				Msg("Media has no file path, skipping symlink creation")
			continue
		}

		// Generate safe symlink name
		symlinkName := m.generateSymlinkName(media)
		symlinkPath := filepath.Join(symlinkDir, symlinkName)

		// Check if symlink already exists and points to correct target
		if existingTarget, exists := existingSymlinks[symlinkPath]; exists {
			if existingTarget == media.FilePath {
				log.Debug().
					Str("symlink", symlinkName).
					Str("target", media.FilePath).
					Msg("Symlink already exists and is correct")
				currentSymlinks[symlinkName] = true
				continue
			} else {
				// Symlink exists but points to wrong target - mark for removal
				log.Info().
					Str("symlink", symlinkName).
					Str("old_target", existingTarget).
					Str("new_target", media.FilePath).
					Bool("dry_run", dryRun).
					Msg("Marking stale symlink for removal")
				staleSymlinks = append(staleSymlinks, symlinkPath)
			}
		}

		// Add to creation list
		log.Info().
			Str("symlink", symlinkName).
			Str("target", media.FilePath).
			Bool("dry_run", dryRun).
			Msg("Preparing symlink for creation")

		symlinkItems = append(symlinkItems, clients.PluginSymlinkItem{
			SourcePath:      media.FilePath,
			TargetDirectory: symlinkDir,
		})

		// Store mapping for later tracking (after successful creation)
		// Map from symlinkPath to symlinkName for tracking
		pendingSymlinks[symlinkPath] = symlinkName
	}

	// Log summary of what we're about to do
	log.Info().
		Int("symlinks_to_create", len(symlinkItems)).
		Int("stale_to_remove", len(staleSymlinks)).
		Int("already_correct", len(currentSymlinks)).
		Msg("Symlink creation summary")

	// Remove stale symlinks first (if any)
	if len(staleSymlinks) > 0 {
		log.Info().
			Int("count", len(staleSymlinks)).
			Bool("dry_run", dryRun).
			Msg("Removing stale symlinks before creating new ones")

		removeResp, err := m.jellyfinClient.RemoveSymlinks(ctx, staleSymlinks, dryRun)
		if err != nil {
			log.Error().
				Err(err).
				Int("count", len(staleSymlinks)).
				Msg("Failed to remove stale symlinks via plugin")
			// Continue anyway - we'll try to create new ones
		} else {
			log.Info().
				Int("removed", removeResp.Removed).
				Int("failed", removeResp.Failed).
				Msg("Stale symlinks removed")
		}
	}

	// Create new/updated symlinks via plugin API
	if len(symlinkItems) > 0 {
		log.Info().
			Int("count", len(symlinkItems)).
			Bool("dry_run", dryRun).
			Msg("Creating symlinks via plugin")

		addResp, err := m.jellyfinClient.AddSymlinks(ctx, symlinkItems, dryRun)
		if err != nil {
			return nil, fmt.Errorf("failed to create symlinks via plugin: %w", err)
		}

		log.Info().
			Int("created", addResp.Created).
			Int("skipped", addResp.Skipped).
			Int("failed", addResp.Failed).
			Msg("Symlinks created via plugin")

		// Log individual failures if any
		if addResp.Failed > 0 && len(addResp.Details) > 0 {
			for _, detail := range addResp.Details {
				log.Warn().
					Str("detail", detail).
					Msg("Symlink creation issue")
			}
		}

		// Track symlinks based on mode
		if dryRun {
			// In dry-run mode, track everything we would have created
			for _, name := range pendingSymlinks {
				currentSymlinks[name] = true
			}
		} else if addResp.Created > 0 {
			// In live mode, verify which symlinks actually exist
			// We need to verify since plugin only returns counts, not which items succeeded
			verifyResp, err := m.jellyfinClient.ListSymlinks(ctx, symlinkDir)
			if err != nil {
				log.Warn().
					Err(err).
					Msg("Failed to verify created symlinks, tracking all attempted")
				// Fall back to tracking all pending symlinks
				for _, name := range pendingSymlinks {
					currentSymlinks[name] = true
				}
			} else {
				// Build set of actually created symlink paths
				createdPaths := make(map[string]bool)
				for _, symlink := range verifyResp.Symlinks {
					createdPaths[symlink.Path] = true
				}
				// Only track symlinks that actually exist
				for path, name := range pendingSymlinks {
					if createdPaths[path] {
						currentSymlinks[name] = true
					}
				}
			}
		}
	} else {
		log.Info().
			Str("directory", symlinkDir).
			Int("items_processed", len(items)).
			Msg("No symlinks need to be created (all already exist or no items with file paths)")
	}

	log.Info().
		Str("directory", symlinkDir).
		Int("final_count", len(currentSymlinks)).
		Msg("Symlink creation completed")

	return currentSymlinks, nil
}

// cleanupSymlinks removes symlinks that are no longer needed
func (m *SymlinkLibraryManager) cleanupSymlinks(ctx context.Context, symlinkDir string, currentSymlinks map[string]bool, dryRun bool) error {
	// List existing symlinks via plugin API
	listResp, err := m.jellyfinClient.ListSymlinks(ctx, symlinkDir)
	if err != nil {
		// If directory doesn't exist or plugin can't access it, that's fine
		log.Debug().
			Err(err).
			Str("path", symlinkDir).
			Msg("Cannot list symlinks (directory may not exist)")
		return nil
	}

	// Find symlinks not in current set
	var staleSymlinks []string
	for _, symlink := range listResp.Symlinks {
		symlinkName := filepath.Base(symlink.Path)
		if !currentSymlinks[symlinkName] {
			staleSymlinks = append(staleSymlinks, symlink.Path)
			log.Debug().
				Str("symlink", symlinkName).
				Str("path", symlink.Path).
				Msg("Found stale symlink")
		}
	}

	// Remove stale symlinks via plugin API
	if len(staleSymlinks) > 0 {
		log.Info().
			Int("count", len(staleSymlinks)).
			Bool("dry_run", dryRun).
			Msg("Removing stale symlinks via plugin")

		removeResp, err := m.jellyfinClient.RemoveSymlinks(ctx, staleSymlinks, dryRun)
		if err != nil {
			return fmt.Errorf("failed to remove stale symlinks via plugin: %w", err)
		}

		log.Info().
			Int("removed", removeResp.Removed).
			Int("failed", removeResp.Failed).
			Msg("Stale symlinks removed")

		// Log details if any failures
		if removeResp.Failed > 0 && len(removeResp.Details) > 0 {
			for _, detail := range removeResp.Details {
				log.Warn().
					Str("detail", detail).
					Msg("Symlink removal issue")
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
