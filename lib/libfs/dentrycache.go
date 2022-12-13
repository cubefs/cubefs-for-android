// Copyright 2022 The CubeFS Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.

package libfs

import (
	"errors"
	"strings"
	"sync"
	"time"

	. "github.com/cubefs/cubefs-for-android/lib/cache"
)

type DentryCache struct {
	dentryCache *LRUCache
	expireMs    int
	sync.Mutex
}

func NewDentryCache(cacheSize int, expireTimeMs int) *DentryCache {
	return &DentryCache{
		dentryCache: NewLRUCache(cacheSize),
		expireMs:    expireTimeMs,
		Mutex:       sync.Mutex{},
	}
}

func (cache *DentryCache) Set(k, v interface{}) error {
	if cache.dentryCache == nil {
		return errors.New("0 Capacity")
	}

	cache.Lock()
	defer cache.Unlock()
	return cache.dentryCache.SetExpire(k, v, time.Duration(cache.expireMs)*time.Millisecond)
}

func (cache *DentryCache) Get(k interface{}) (v interface{}, ret bool, err error) {
	if cache.dentryCache == nil {
		return v, false, errors.New("0 Capacity")
	}

	cache.Lock()
	defer cache.Unlock()
	return cache.dentryCache.Get(k)
}

func (cache *DentryCache) Remove(k interface{}) bool {
	if cache.dentryCache == nil {
		return false
	}

	cache.Lock()
	defer cache.Unlock()
	return cache.dentryCache.Remove(k)
}

func (cache *DentryCache) RemovePrefix(prefix interface{}) error {

	if cache.dentryCache == nil {
		return nil
	}

	cache.Lock()
	defer cache.Unlock()

	rtKeys := cache.dentryCache.GetKeys(prefix, func(cacheKey, prefix interface{}) bool {
		if strings.HasPrefix(cacheKey.(string), prefix.(string)) {
			return true
		}
		return false
	})

	if rtKeys != nil {
		for _, k := range rtKeys {
			cache.dentryCache.Remove(k)
		}
	}

	return nil
}
