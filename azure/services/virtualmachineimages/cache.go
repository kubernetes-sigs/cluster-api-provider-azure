/*
Copyright 2022 The Kubernetes Authors.

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

package virtualmachineimages

import (
	"context"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"
	"github.com/pkg/errors"

	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/util/cache/ttllru"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// Key contains the fields necessary to locate a VM image list resource.
type Key struct {
	location          string
	publicGalleryName string
	galleryImageName  string
}

// Cache stores VM image list resources.
type Cache struct {
	client Client
	data   map[Key][]*armcompute.CommunityGalleryImageVersion
}

// Cacher allows getting items from and adding them to a cache.
type Cacher interface {
	Get(key interface{}) (value interface{}, ok bool)
	Add(key interface{}, value interface{}) bool
}

var (
	_           Client = &AzureClient{}
	doOnce      sync.Once
	clientCache Cacher
)

// newCache instantiates a cache.
func newCache(auth azure.Authorizer) (*Cache, error) {
	client, err := NewClient(auth)
	if err != nil {
		return nil, err
	}
	return &Cache{
		client: client,
	}, nil
}

// GetCache either creates a new VM images cache or returns the existing one.
func GetCache(auth azure.Authorizer) (*Cache, error) {
	var err error
	doOnce.Do(func() {
		clientCache, err = ttllru.New(128, 1*time.Hour)
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed creating LRU cache for VM images")
	}

	key := auth.HashKey()
	c, ok := clientCache.Get(key)
	if ok {
		return c.(*Cache), nil
	}

	c, err = newCache(auth)
	if err != nil {
		return nil, err
	}
	_ = clientCache.Add(key, c)
	return c.(*Cache), nil
}

// refresh fetches a VM image list resource from Azure and stores it in the cache.
func (c *Cache) refresh(ctx context.Context, key Key) error {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "virtualmachineimages.Cache.refresh")
	defer done()

	data, err := c.client.List(ctx, key.location, key.publicGalleryName, key.galleryImageName)
	if err != nil {
		return errors.Wrap(err, "failed to refresh VM images cache")
	}

	c.data[key] = data

	return nil
}

// Get returns a VM image list resource in a location given a publisher, offer, and sku.
func (c *Cache) Get(ctx context.Context, location, publicGalleryName, galleryImageName string) ([]*armcompute.CommunityGalleryImageVersion, error) {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "virtualmachineimages.Cache.Get")
	defer done()

	if c.data == nil {
		c.data = make(map[Key][]*armcompute.CommunityGalleryImageVersion)
	}

	key := Key{
		location:          location,
		publicGalleryName: publicGalleryName,
		galleryImageName:  galleryImageName,
	}

	if _, ok := c.data[key]; !ok {
		log.V(4).Info("VM images cache miss", "location", key.location, "publicGalleryName", key.publicGalleryName, "galleryImageName", key.galleryImageName)
		if err := c.refresh(ctx, key); err != nil {
			return []*armcompute.CommunityGalleryImageVersion{}, err
		}
	} else {
		log.V(4).Info("VM images cache hit", "location", key.location, "publicGalleryName", key.publicGalleryName, "galleryImageName", key.galleryImageName)
	}

	return c.data[key], nil
}
