package wazuh

import (
	"sync"
	"time"
)

// cacheEntry holds a cached value and its expiry.
type cacheEntry struct {
	value     any
	expiresAt time.Time
}

// clientCache is embedded in Client to provide TTL caching.
type clientCache struct {
	cacheMu sync.RWMutex
	cache   map[string]cacheEntry
}

// getCached returns the cached value for key if non-expired, otherwise calls
// fetch() and stores its result for ttl. fetch is called outside the lock.
// On fetch error, cache is not updated.
func getCached[T any](c *Client, key string, ttl time.Duration, fetch func() (T, error)) (T, error) {
	c.cacheMu.RLock()
	if e, ok := c.cache[key]; ok && time.Now().Before(e.expiresAt) {
		v := e.value.(T)
		c.cacheMu.RUnlock()
		return v, nil
	}
	c.cacheMu.RUnlock()

	v, err := fetch()
	if err != nil {
		var zero T
		return zero, err
	}
	c.cacheMu.Lock()
	c.cache[key] = cacheEntry{value: v, expiresAt: time.Now().Add(ttl)}
	c.cacheMu.Unlock()
	return v, nil
}

// InvalidateCache clears all cached entries, forcing the next request to
// fetch fresh data. This is called by the user via the `r` key.
func (c *Client) InvalidateCache() {
	c.cacheMu.Lock()
	c.cache = make(map[string]cacheEntry)
	c.cacheMu.Unlock()
}
