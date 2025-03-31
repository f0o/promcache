package cache

import (
	"log/slog"
	"sync"
	"time"
)

// Item represents a cached item with expiration
type Item struct {
	Value      []byte
	Expiration int64
}

// Cache is a simple TTL cache for Prometheus query results
type Cache struct {
	mu    sync.RWMutex
	items map[string]Item
	ttl   time.Duration
	log   *slog.Logger
}

// New creates a new cache with the specified TTL
func New(ttl time.Duration, log *slog.Logger) *Cache {
	c := &Cache{
		items: make(map[string]Item),
		ttl:   ttl,
		log:   log,
	}

	// Start background cleanup
	go c.startCleanup()

	return c
}

// Get retrieves an item from the cache if it exists and has not expired
func (c *Cache) Get(key string) ([]byte, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	c.log.Debug("Looking up cache key", "key", key)

	item, found := c.items[key]
	if !found {
		c.log.Debug("Cache key not found", "key", key)
		return nil, false
	}

	// Check if the item has expired
	if time.Now().UnixNano() > item.Expiration {
		c.log.Debug("Cache item expired", "key", key)
		return nil, false
	}

	c.log.Debug("Cache hit", "key", key)
	return item.Value, true
}

// Set adds an item to the cache with the default TTL
func (c *Cache) Set(key string, value []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.log.Debug("Caching response", "key", key)
	c.items[key] = Item{
		Value:      value,
		Expiration: time.Now().Add(c.ttl).UnixNano(),
	}
}

// Delete removes an item from the cache
func (c *Cache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.items, key)
}

// startCleanup periodically removes expired items from the cache
func (c *Cache) startCleanup() {
	ticker := time.NewTicker(c.ttl / 2)
	defer ticker.Stop()

	for range ticker.C {
		c.cleanup()
	}
}

// cleanup removes expired items from the cache
func (c *Cache) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now().UnixNano()
	for k, v := range c.items {
		if now > v.Expiration {
			c.log.Debug("Removing expired item", "key", k)
			delete(c.items, k)
		}
	}
}

// TTL returns the cache TTL duration
func (c *Cache) TTL() time.Duration {
	return c.ttl
}

// Keys returns all keys in the cache
func (c *Cache) Keys() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	keys := make([]string, 0, len(c.items))
	for k := range c.items {
		keys = append(keys, k)
	}
	return keys
}
