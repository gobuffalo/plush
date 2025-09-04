package inmemory

import (
	"sync"

	"github.com/gobuffalo/plush/v5"
)

type MemoryCache struct {
	mu    sync.RWMutex
	store map[string]*plush.Template
}

func NewMemoryCache() *MemoryCache {
	return &MemoryCache{
		store: make(map[string]*plush.Template),
	}
}

func (c *MemoryCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.store = make(map[string]*plush.Template)
}

func (c *MemoryCache) Get(key string) (*plush.Template, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	t, ok := c.store[key]
	return t, ok
}

func (c *MemoryCache) Set(key string, t *plush.Template) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.store[key] = t
}

func (c *MemoryCache) Delete(key ...string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, k := range key {
		delete(c.store, k)
	}
}
