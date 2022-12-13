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

package cache

import (
	"container/list"
	"errors"
	"time"
)

type CacheNode struct {
	Key, Value interface{}
	Expiration int64
}

func (cnode *CacheNode) NewCacheNode(k, v interface{}, e int64) *CacheNode {
	return &CacheNode{k, v, e}
}

type LRUCache struct {
	Capacity int
	dlist    *list.List
	cacheMap map[interface{}]*list.Element
}

func NewLRUCache(cap int) *LRUCache {
	if cap <= 0 {
		return nil
	}

	return &LRUCache{
		Capacity: cap,
		dlist:    list.New(),
		cacheMap: make(map[interface{}]*list.Element)}
}

func (lru *LRUCache) Size() int {
	return lru.dlist.Len()
}

func (lru *LRUCache) Set(k, v interface{}) error {
	return lru.SetExpire(k, v, 0)
}

func (lru *LRUCache) SetExpire(k, v interface{}, d time.Duration) error {

	if lru.dlist == nil {
		return errors.New("not inited")
	}

	var e int64
	if d <= 0 {
		e = 0
	} else {
		e = time.Now().Add(d).UnixNano()
	}

	if pElement, ok := lru.cacheMap[k]; ok {
		lru.dlist.MoveToFront(pElement)
		pElement.Value.(*CacheNode).Value = v
		pElement.Value.(*CacheNode).Expiration = e
		return nil
	}

	newElement := lru.dlist.PushFront(&CacheNode{k, v, e})
	lru.cacheMap[k] = newElement

	if lru.dlist.Len() > lru.Capacity {
		lastElement := lru.dlist.Back()
		if lastElement == nil {
			return nil
		}
		cacheNode := lastElement.Value.(*CacheNode)
		delete(lru.cacheMap, cacheNode.Key)
		lru.dlist.Remove(lastElement)
	}
	return nil
}

func (lru *LRUCache) Get(k interface{}) (v interface{}, ret bool, err error) {

	if lru.cacheMap == nil {
		return v, false, errors.New("LRUCache not inited.")
	}

	if pElement, ok := lru.cacheMap[k]; ok {
		e := pElement.Value.(*CacheNode).Expiration
		if e == 0 || time.Now().UnixNano() < e {
			lru.dlist.MoveToFront(pElement)
			return pElement.Value.(*CacheNode).Value, true, nil
		}
	}
	return v, false, nil
}

func (lru *LRUCache) Remove(k interface{}) bool {

	if lru.cacheMap == nil {
		return false
	}

	if pElement, ok := lru.cacheMap[k]; ok {
		cacheNode := pElement.Value.(*CacheNode)
		delete(lru.cacheMap, cacheNode.Key)
		lru.dlist.Remove(pElement)
		return true
	}
	return false
}

func (lru *LRUCache) GetKeys(k interface{}, matchFunc func(cacheKey, k interface{}) bool) []interface{} {

	if lru.cacheMap == nil {
		return nil
	}

	rtKeys := make([]interface{}, 0)
	for cacheKey := range lru.cacheMap {
		if matchFunc(cacheKey, k) {
			rtKeys = append(rtKeys, cacheKey)
		}
	}

	return rtKeys
}
