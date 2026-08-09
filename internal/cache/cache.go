package cache

import (
	"fmt"
	"sync"
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

	inflightMu sync.Mutex
	inflight   map[string]*inflightCall
}

type inflightCall struct {
	done chan struct{}
	val  any
	err  error
}

// New creates a new Cache instance
func New() *Cache {
	return &Cache{
		store:    gocache.New(5*time.Minute, 10*time.Minute),
		inflight: make(map[string]*inflightCall),
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

// GetOrSet retrieves a value from cache, or sets it if not found.
// Concurrent callers for the same key share a single factory invocation
// (single-flight), so fn never runs twice for the same key at once.
//
// If fn panics, the panic is recovered into a returned error so waiters are
// released instead of hanging. Note: if fn exits via runtime.Goexit (rather
// than returning or panicking), waiters are still released but receive a zero
// value with a nil error; callers must not rely on recovering from that.
func (c *Cache) GetOrSet(key string, ttl time.Duration, fn func() (any, error)) (val any, err error) {
	if val, found := c.Get(key); found {
		return val, nil
	}

	// Claim or join an in-flight computation for this key.
	c.inflightMu.Lock()
	if call, ok := c.inflight[key]; ok {
		c.inflightMu.Unlock()
		<-call.done
		return call.val, call.err
	}

	call := &inflightCall{done: make(chan struct{})}
	c.inflight[key] = call
	c.inflightMu.Unlock()

	// If fn panics, recover it into call.err so waiters are released and the
	// panic becomes a returned error instead of crashing the calling goroutine
	// (or stranding concurrent/future callers on <-call.done).
	defer func() {
		if recovered := recover(); recovered != nil {
			call.err = fmt.Errorf("cache GetOrSet factory panicked: %v", recovered)
			err = call.err
		}
		close(call.done)
		c.inflightMu.Lock()
		delete(c.inflight, key)
		c.inflightMu.Unlock()
	}()

	val, err = fn()

	call.val = val
	call.err = err
	if err == nil {
		c.Set(key, val, ttl)
	}

	return val, err
}
