package cache

import (
	"time"

	gocache "github.com/patrickmn/go-cache"
)

// Cache keys constants
const (
	CacheKeyJellyfinLibrary    = "jellyfin:library:%s"
	CacheKeyRadarrMovies       = "radarr:movies"
	CacheKeyRadarrHistory      = "radarr:history:%d"
	CacheKeySonarrShows        = "sonarr:shows"
	CacheKeySonarrHistory      = "sonarr:history:%d"
	CacheKeyJellyseerrRequests = "jellyseerr:requests"
	CacheKeyJellystatWatch     = "jellystat:watch:%s"
	CacheKeyRuleEvaluation     = "rule:eval:%s"
	CacheKeyDeletionTimeline   = "timeline:deletion"
	CacheKeyLeavingSoon        = "library:leaving_soon"
)

// Cache TTL constants
const (
	TTLJellyfinLibrary    = 1 * time.Hour
	TTLRadarrMovies       = 30 * time.Minute
	TTLRadarrHistory      = 15 * time.Minute
	TTLSonarrShows        = 30 * time.Minute
	TTLSonarrHistory      = 15 * time.Minute
	TTLJellyseerrRequests = 15 * time.Minute
	TTLJellystatWatch     = 5 * time.Minute
	TTLRuleEvaluation     = 0 // No expiration, cleared on sync
	TTLDeletionTimeline   = 5 * time.Minute
	TTLLeavingSoon        = 5 * time.Minute
)

// Cache is a wrapper around go-cache
type Cache struct {
	store *gocache.Cache
}

// New creates a new Cache instance
func New() *Cache {
	return &Cache{
		store: gocache.New(5*time.Minute, 10*time.Minute),
	}
}

// Set stores a value in the cache with the specified TTL
func (c *Cache) Set(key string, value any, ttl time.Duration) {
	c.store.Set(key, value, ttl)
}

// Get retrieves a value from the cache
func (c *Cache) Get(key string) (any, bool) {
	return c.store.Get(key)
}

// Delete removes a value from the cache
func (c *Cache) Delete(key string) {
	c.store.Delete(key)
}

// Clear removes all values from the cache
func (c *Cache) Clear() {
	c.store.Flush()
}

// DeletePattern removes all keys matching a pattern (prefix)
func (c *Cache) DeletePattern(pattern string) {
	items := c.store.Items()
	for key := range items {
		if len(key) >= len(pattern) && key[:len(pattern)] == pattern {
			c.store.Delete(key)
		}
	}
}

// GetOrSet retrieves a value from cache, or sets it if not found
func (c *Cache) GetOrSet(key string, ttl time.Duration, fn func() (any, error)) (any, error) {
	if val, found := c.Get(key); found {
		return val, nil
	}

	val, err := fn()
	if err != nil {
		return nil, err
	}

	c.Set(key, val, ttl)
	return val, nil
}
