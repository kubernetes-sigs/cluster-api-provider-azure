/*
Copyright 2020 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package ttllru

import (
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru"
	"github.com/pkg/errors"
)

type (
	// Cache is a TTL LRU cache which caches items with a max time to live and with
	// bounded length
	Cache struct {
		TimeToLive time.Duration
		cache      cacher
		mu         sync.Mutex
	}

	cacher interface {
		Get(key interface{}) (value interface{}, ok bool)
		Add(key interface{}, value interface{}) (evicted bool)
		Remove(key interface{}) (ok bool)
	}

	timeToLiveItem struct {
		LastTouch time.Time
		Value     interface{}
	}
)

// New creates a new TTL LRU cache which caches items with a max time to live and with
// bounded length
func New(size int, timeToLive time.Duration) (*Cache, error) {
	c, err := lru.New(size)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build new LRU cache")
	}

	return newCache(timeToLive, c)
}

func newCache(timeToLive time.Duration, cache cacher) (*Cache, error) {
	return &Cache{
		cache:      cache,
		TimeToLive: timeToLive,
	}, nil
}

// Get returns a value and a bool indicating the value was found for a given key
func (ttlCache *Cache) Get(key interface{}) (value interface{}, ok bool) {
	ttlCache.mu.Lock()
	defer ttlCache.mu.Unlock()

	val, ok := ttlCache.cache.Get(key)
	if !ok {
		return nil, false
	}

	ttlItem, ok := val.(*timeToLiveItem)
	if !ok {
		return nil, false
	}

	if time.Since(ttlItem.LastTouch) > ttlCache.TimeToLive {
		ttlCache.cache.Remove(key)
		return nil, false
	}

	ttlItem.LastTouch = time.Now()
	return ttlItem.Value, true
}

// Add will add a value for a given key
func (ttlCache *Cache) Add(key interface{}, val interface{}) {
	ttlCache.mu.Lock()
	defer ttlCache.mu.Unlock()

	ttlCache.cache.Add(key, &timeToLiveItem{
		Value:     val,
		LastTouch: time.Now(),
	})
}
