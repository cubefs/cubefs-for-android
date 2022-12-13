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
	. "github.com/cubefs/cubefs-for-android/lib/libfs/util"
	zap "github.com/cubefs/cubefs-for-android/lib/log"
)

type RWCache struct {
	Mount  *CfaMountInfo
	log    *ClientLogger
	rCache *RCache
	wCache *WCache
}

func NewRWCache(mount *CfaMountInfo, BufferMax int, ExpireTime int, SyncRoutine int, PrefetchRoutine int, PrefetchTimes int) *RWCache {
	cache := &RWCache{
		Mount:  mount,
		log:    mount.log,
		rCache: NewRCache(mount, BufferMax, ExpireTime, PrefetchRoutine, PrefetchTimes),
		wCache: NewWCache(mount, BufferMax, ExpireTime, SyncRoutine),
	}

	return cache
}

func (cache *RWCache) doRead(fe *FileEntry, buffer []byte, offset uint64) (int, error) {
	dataBuf := make([]byte, len(buffer))
	fileBufRead := cache.rCache.GetFileBuffer(fe, true)
	Assert(fileBufRead != nil)
	cache.rCache.TryDoPrefetch(fileBufRead, uint32(len(dataBuf)), offset)

	fileBufWrite := cache.wCache.GetFileBuffer(fe, false)

	Size := 0
	Ops := cache.rCache.Slice(fe, dataBuf, offset)
	for _, op := range Ops {
		bgTime := cache.Mount.st.BeginStat()

		// first lock buffer, if lock success, then data is flush..
		if fileBufWrite != nil {
			fileBufWrite.RLockBuffer(op.Index)
		}

		// second read data from disk.
		fileBufRead.RLockBuffer(op.Index)
		opSize, err := cache.rCache.ReadOp(fe, fileBufRead, op)
		fileBufRead.RUnLockBuffer(op.Index)
		if err != nil {
			if fileBufWrite != nil {
				fileBufWrite.RUnLockBuffer(op.Index)
			}

			cache.Mount.st.EndStat("CacheRead", err, bgTime, 1)
			return opSize, err
		}

		// third merge data with write cache.
		if fileBufWrite != nil {
			opSize = cache.wCache.MergeOp(fileBufWrite, op, opSize)
			fileBufWrite.RUnLockBuffer(op.Index)
		}

		cache.Mount.st.EndStat("CacheRead", nil, bgTime, 1)
		if opSize != 0 {
			Size = int(op.GetFileOffset()-offset) + opSize
		}
	}

	Assert(Size <= len(buffer))
	copy(buffer, dataBuf[0:Size])

	return Size, nil
}

func (cache *RWCache) Read(fe *FileEntry, buffer []byte, offset uint64) (int, error) {
	Size, err := cache.doRead(fe, buffer, offset)

	cache.log.Info("cache::read done",
		zap.Int("handle", fe.Handle),
		zap.Uint64("id", fe.Id),
		zap.Uint64("offset", offset),
		zap.Uint32("size", uint32(len(buffer))),
		zap.Uint64("idx", offset/uint64(fe.BlockSize)),
		zap.Int("rspSize", Size),
		zap.Any("err", err))

	return Size, err
}

func (cache *RWCache) Write(fe *FileEntry, buffer []byte, offset uint64) (int, error) {
	Size, err := cache.wCache.Write(fe, buffer, offset)

	// clean cache.
	cache.rCache.Clean(fe, len(buffer), offset)

	cache.log.Info("cache::write done",
		zap.Int("handle", fe.Handle),
		zap.Uint64("id", fe.Id),
		zap.Uint64("offset", offset),
		zap.Uint32("size", uint32(len(buffer))),
		zap.Uint64("idx", offset/uint64(fe.BlockSize)),
		zap.Int("rspSize", Size),
		zap.Any("err", err))

	return Size, err
}

func (cache *RWCache) Release(fe *FileEntry) error {
	return cache.rCache.Release(fe)
}

func (cache *RWCache) Flush(fe *FileEntry) error {
	return cache.wCache.Flush(fe)
}

func (cache *RWCache) Close(fe *FileEntry) error {
	err := cache.rCache.Close(fe)
	if err != nil {
		return err
	}

	err = cache.wCache.Close(fe)
	if err != nil {
		return err
	}

	return nil
}

func (cache *RWCache) CloseAll() {
	cache.rCache.CloseAll()
	cache.wCache.CloseAll()
}
