// Package cache provides a small, dependency-free TTL + LRU cache.
//
// Keys are strings so that domain caches can be invalidated by prefix
// (e.g. "album:" after a star/unstar mutation). All operations are O(1)
// except prefix invalidation which is O(n) and only runs on writes.
package cache

import (
	"container/list"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type entry[V any] struct {
	key       string
	value     V
	expiresAt time.Time
}

// Cache is safe for concurrent use.
type Cache[V any] struct {
	mu       sync.RWMutex
	items    map[string]*list.Element
	order    *list.List // front = most recently used
	capacity int
	ttl      time.Duration
	now      func() time.Time

	hits   atomic.Uint64
	misses atomic.Uint64
}

// New creates a cache. capacity <= 0 means unbounded (TTL still applies),
// ttl <= 0 means entries never expire.
func New[V any](capacity int, ttl time.Duration) *Cache[V] {
	return &Cache[V]{
		items:    make(map[string]*list.Element),
		order:    list.New(),
		capacity: capacity,
		ttl:      ttl,
		now:      time.Now,
	}
}

// Get returns the value and whether it was a fresh hit.
func (c *Cache[V]) Get(key string) (V, bool) {
	var zero V
	c.mu.Lock()
	defer c.mu.Unlock()

	el, ok := c.items[key]
	if !ok {
		c.misses.Add(1)
		return zero, false
	}
	e := el.Value.(*entry[V])
	if c.ttl > 0 && c.now().After(e.expiresAt) {
		c.removeElement(el)
		c.misses.Add(1)
		return zero, false
	}
	c.order.MoveToFront(el)
	c.hits.Add(1)
	return e.value, true
}

// Set inserts or replaces a value, evicting the least recently used entry
// when the configured capacity is exceeded.
func (c *Cache[V]) Set(key string, value V) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var exp time.Time
	if c.ttl > 0 {
		exp = c.now().Add(c.ttl)
	}
	if el, ok := c.items[key]; ok {
		e := el.Value.(*entry[V])
		e.value = value
		e.expiresAt = exp
		c.order.MoveToFront(el)
		return
	}
	el := c.order.PushFront(&entry[V]{key: key, value: value, expiresAt: exp})
	c.items[key] = el
	c.evictLocked()
}

// GetOrLoad returns the cached value or computes and stores it with fetch.
// fetch may be called more than once on a cold key under contention; callers
// must therefore keep it idempotent (all usages are read-only API calls).
func (c *Cache[V]) GetOrLoad(key string, fetch func() (V, error)) (V, error) {
	if v, ok := c.Get(key); ok {
		return v, nil
	}
	v, err := fetch()
	if err != nil {
		return v, err
	}
	c.Set(key, v)
	return v, nil
}

// Delete removes a single key.
func (c *Cache[V]) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.items[key]; ok {
		c.removeElement(el)
	}
}

// InvalidatePrefix removes every key starting with prefix.
func (c *Cache[V]) InvalidatePrefix(prefix string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := 0
	for key, el := range c.items {
		if strings.HasPrefix(key, prefix) {
			c.removeElement(el)
			n++
		}
	}
	return n
}

// Invalidate removes keys for which keep returns false.
func (c *Cache[V]) Invalidate(keep func(key string) bool) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := 0
	for key, el := range c.items {
		if !keep(key) {
			c.removeElement(el)
			n++
		}
	}
	return n
}

// Clear empties the cache but keeps its configuration and statistics.
func (c *Cache[V]) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[string]*list.Element)
	c.order.Init()
}

// Len returns the number of live entries (expired entries are pruned first).
func (c *Cache[V]) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pruneExpiredLocked()
	return len(c.items)
}

// Stats returns cumulative hit and miss counters.
func (c *Cache[V]) Stats() (hits, misses uint64) {
	return c.hits.Load(), c.misses.Load()
}

func (c *Cache[V]) removeElement(el *list.Element) {
	e := el.Value.(*entry[V])
	delete(c.items, e.key)
	c.order.Remove(el)
}

func (c *Cache[V]) evictLocked() {
	if c.capacity <= 0 {
		return
	}
	for len(c.items) > c.capacity {
		el := c.order.Back()
		if el == nil {
			return
		}
		c.removeElement(el)
	}
}

func (c *Cache[V]) pruneExpiredLocked() {
	if c.ttl <= 0 {
		return
	}
	now := c.now()
	for key, el := range c.items {
		if now.After(el.Value.(*entry[V]).expiresAt) {
			c.removeElement(el)
			_ = key
		}
	}
}
