package cache

import (
	"sync"
	"time"
)

//Cache represents a thread-safe map
type Cache struct {
	mx    sync.RWMutex
	cache map[string]interface{}
}

func NewCache(timeout time.Duration) *Cache {
	return &Cache{
		cache: make(map[string]interface{}),
	}
}

func (c *Cache) Get(key string) (interface{}, bool) {
	c.mx.RLock()
	defer c.mx.RUnlock()

	v, ok := c.cache[key]
	return v, ok
}

func (c *Cache) MustGet(key string) interface{} {
	c.mx.RLock()
	defer c.mx.RUnlock()

	v, ok := c.cache[key]
	if ok {
		return v
	}

	return nil
}

func (c *Cache) Set(key string, value interface{}) {
	c.mx.Lock()
	defer c.mx.Unlock()

	c.cache[key] = value
}

func (c *Cache) Delete(key string) {
	c.mx.Lock()
	defer c.mx.Unlock()

	delete(c.cache, key)
}

func (c *Cache) Len() int {
	c.mx.Lock()
	defer c.mx.Unlock()

	return len(c.cache)
}
