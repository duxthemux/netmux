// Package memstore implements a simple in memory thread safe container for different needs we will have in this repo.
package memstore

import (
	"sync"

	"github.com/google/uuid"
)

// Map is our core implementation for the in memory storage.
type Map[T any] struct {
	sync.RWMutex
	items map[string]T
}

// Add will add a new item to storage, but considers that you dont have a key, and it will generate one for your.
func (c *Map[T]) Add(item T) string {
	c.Lock()
	defer c.Unlock()

	ret := uuid.NewString()

	c.items[ret] = item

	return ret
}

// Set is similar to add, but in this case you want to use your own key. This method will overwrite previous item in
// case of conflicting keys.
func (c *Map[T]) Set(key string, item T) {
	c.Lock()
	defer c.Unlock()

	c.items[key] = item
}

// Del will remove entry referred by the key from the map.
func (c *Map[T]) Del(keys ...string) {
	c.Lock()
	defer c.Unlock()

	for _, aKey := range keys {
		delete(c.items, aKey)
	}
}

// Get retrieves the item represented by the provided key.
//
//nolint:ireturn,nolintlint
func (c *Map[T]) Get(key string) T {
	c.RLock()
	defer c.RUnlock()

	return c.items[key]
}

// ForEach iterates over all items until they are over or when fn returns an error.
// Dont delete items from inside this function.
func (c *Map[T]) ForEach(fn func(k string, v T) error) error {
	c.RLock()
	defer c.RUnlock()

	for k, v := range c.items {
		if err := fn(k, v); err != nil {
			return err
		}
	}

	return nil
}

// New will create a new instance of a Map.
func New[T any]() *Map[T] {
	return &Map[T]{
		RWMutex: sync.RWMutex{},
		items:   map[string]T{},
	}
}
