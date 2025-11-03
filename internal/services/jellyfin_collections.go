package services

import (
	"context"
	"time"

	"github.com/ramonskie/prunarr/internal/clients"
	"github.com/ramonskie/prunarr/internal/config"
	"github.com/ramonskie/prunarr/internal/models"
	"github.com/rs/zerolog/log"
)

// JellyfinCollectionClient defines the methods needed for collection management
type JellyfinCollectionClient interface {
	GetCollectionByName(ctx context.Context, name string) (*clients.JellyfinCollection, error)
	CreateCollection(ctx context.Context, name string, itemIDs []string, dryRun bool) (string, error)
	AddItemsToCollection(ctx context.Context, collectionID string, itemIDs []string, dryRun bool) error
	DeleteCollection(ctx context.Context, collectionID string, dryRun bool) error
}

// JellyfinCollectionManager manages "Leaving Soon" collections in Jellyfin
type JellyfinCollectionManager struct {
	client JellyfinCollectionClient
	config *config.CollectionsConfig
	dryRun bool
}

// NewJellyfinCollectionManager creates a new collection manager
func NewJellyfinCollectionManager(client JellyfinCollectionClient, cfg *config.CollectionsConfig, dryRun bool) *JellyfinCollectionManager {
	return &JellyfinCollectionManager{
		client: client,
		config: cfg,
		dryRun: dryRun,
	}
}

// SyncCollections updates Jellyfin collections with items scheduled for deletion
func (m *JellyfinCollectionManager) SyncCollections(ctx context.Context, mediaLibrary map[string]models.Media) error {
	if !m.config.Enabled {
		log.Debug().Msg("Collections sync skipped (disabled)")
		return nil
	}

	log.Debug().Msg("Starting Jellyfin collections sync")

	// Separate media by type and filter for items scheduled for deletion
	movieIDs := make([]string, 0)
	tvShowIDs := make([]string, 0)
	now := time.Now()

	skippedNoID := 0
	for _, media := range mediaLibrary {
		// Skip excluded items
		if media.IsExcluded {
			continue
		}

		// Only include items with deletion date in the future (not overdue)
		if !media.DeleteAfter.IsZero() && media.DeleteAfter.After(now) {
			if media.JellyfinID != "" {
				if media.Type == models.MediaTypeMovie {
					movieIDs = append(movieIDs, media.JellyfinID)
				} else if media.Type == models.MediaTypeTVShow {
					tvShowIDs = append(tvShowIDs, media.JellyfinID)
				}
			} else {
				skippedNoID++
			}
		}
	}

	if skippedNoID > 0 {
		log.Warn().
			Int("skipped_count", skippedNoID).
			Msg("Skipped items without Jellyfin IDs for collections")
	}

	log.Debug().
		Int("movies", len(movieIDs)).
		Int("tv_shows", len(tvShowIDs)).
		Strs("movie_ids_sample", func() []string {
			if len(movieIDs) > 3 {
				return movieIDs[:3]
			}
			return movieIDs
		}()).
		Msg("Found items scheduled for deletion")

	// Sync movie collection
	if err := m.syncCollection(ctx, m.config.Movies.Name, movieIDs, m.config.Movies.HideWhenEmpty); err != nil {
		log.Error().Err(err).Str("collection", m.config.Movies.Name).Msg("Failed to sync movie collection")
		return err
	}

	// Sync TV show collection
	if err := m.syncCollection(ctx, m.config.TVShows.Name, tvShowIDs, m.config.TVShows.HideWhenEmpty); err != nil {
		log.Error().Err(err).Str("collection", m.config.TVShows.Name).Msg("Failed to sync TV show collection")
		return err
	}

	log.Info().
		Int("movies", len(movieIDs)).
		Int("tv_shows", len(tvShowIDs)).
		Bool("dry_run", m.dryRun).
		Msg("Jellyfin collections synced successfully")

	return nil
}

// syncCollection manages a single collection (create/update/delete)
func (m *JellyfinCollectionManager) syncCollection(ctx context.Context, name string, itemIDs []string, hideWhenEmpty bool) error {
	// Find existing collection
	existing, err := m.client.GetCollectionByName(ctx, name)
	if err != nil {
		return err
	}

	// If no items and hide_when_empty is enabled, delete the collection
	if len(itemIDs) == 0 && hideWhenEmpty {
		if existing != nil {
			if err := m.client.DeleteCollection(ctx, existing.ID, m.dryRun); err != nil {
				return err
			}
			log.Info().
				Str("collection", name).
				Bool("dry_run", m.dryRun).
				Msg("Deleted empty collection")
		}
		return nil
	}

	// If no items but hide_when_empty is disabled, just skip
	if len(itemIDs) == 0 {
		log.Debug().Str("collection", name).Msg("No items for collection, skipping")
		return nil
	}

	// Create collection if it doesn't exist
	if existing == nil {
		collectionID, err := m.client.CreateCollection(ctx, name, itemIDs, m.dryRun)
		if err != nil {
			return err
		}
		log.Info().
			Str("collection", name).
			Str("collection_id", collectionID).
			Int("item_count", len(itemIDs)).
			Bool("dry_run", m.dryRun).
			Msg("Created collection")
		return nil
	}

	// Collection exists - update it by removing all items and re-adding
	// (Jellyfin doesn't have a "set items" API, only add/remove)

	// For simplicity, we'll delete and recreate if items changed
	// In production, you might want to diff and only add/remove changed items
	if m.dryRun {
		log.Info().
			Str("collection", name).
			Str("collection_id", existing.ID).
			Int("item_count", len(itemIDs)).
			Bool("dry_run", true).
			Msg("[DRY-RUN] Would update collection")
		return nil
	}

	// Add items to existing collection
	// Note: This is additive. Jellyfin doesn't provide a "replace all" operation
	// We rely on the collection being managed only by Prunarr
	if err := m.client.AddItemsToCollection(ctx, existing.ID, itemIDs, m.dryRun); err != nil {
		return err
	}

	log.Info().
		Str("collection", name).
		Str("collection_id", existing.ID).
		Int("item_count", len(itemIDs)).
		Msg("Updated collection")

	return nil
}
