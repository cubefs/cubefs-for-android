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
	"container/list"
	"fmt"
	"sync"
	"time"

	"github.com/cubefs/cubefs-for-android/lib/libfs/consts"
	. "github.com/cubefs/cubefs-for-android/lib/libfs/util"
	zap "github.com/cubefs/cubefs-for-android/lib/log"
)

type RCache struct {
	sync.RWMutex
	Mount      *CfaMountInfo
	BufferMax  int
	FreeList   *list.List
	Cache      map[uint64]*FileBuffer
	LruList    *list.List
	log        *ClientLogger
	ExpireTime int

	PrefetchRoutine int
	PrefetchTimes   int
	chPrefetch      chan interface{}

	Stop bool
	wg   sync.WaitGroup
}

func NewRCache(mount *CfaMountInfo, BufferMax int, ExpireTime int, PrefetchRoutine int, PrefetchTimes int) *RCache {
	cache := &RCache{
		RWMutex:         sync.RWMutex{},
		Mount:           mount,
		BufferMax:       BufferMax,
		FreeList:        list.New(),
		Cache:           make(map[uint64]*FileBuffer),
		LruList:         list.New(),
		log:             mount.log,
		ExpireTime:      ExpireTime,
		PrefetchRoutine: PrefetchRoutine,
		PrefetchTimes:   PrefetchTimes,
		chPrefetch:      nil,
		Stop:            false,
		wg:              sync.WaitGroup{},
	}

	for i := 0; i < BufferMax; i++ {
		cache.FreeList.PushBack(NewBuffer(uint32(i)))
	}

	if PrefetchRoutine > 0 {
		cache.chPrefetch = make(chan interface{}, cache.PrefetchRoutine)
		for i := 0; i < PrefetchRoutine; i++ {
			cache.wg.Add(1)
			go cache.PrefetchRead()
		}
	}

	cache.wg.Add(1)
	go cache.CheckExpire()

	return cache
}

func (cache *RCache) AllocBuffer(fe *FileEntry, BlkIdx uint64) *Buffer {
	cache.Lock()
	defer cache.Unlock()

	if cache.FreeList.Len() == 0 {
		return nil
	}

	eL := cache.FreeList.Front()
	cache.FreeList.Remove(eL)

	buf := eL.Value.(*Buffer)
	buf.ReSet(fe, BlkIdx)

	return buf
}

func (cache *RCache) FreeBuffer(buf *Buffer) {
	cache.Lock()
	defer cache.Unlock()

	cache.FreeList.PushBack(buf)
}

func (cache *RCache) DoLru(buf *Buffer) {
	cache.Lock()
	defer cache.Unlock()

	buf.LastUpdateTime = time.Now()
	if buf.elm == nil {
		buf.elm = cache.LruList.PushFront(buf)
	} else {
		cache.LruList.MoveToFront(buf.elm)
	}
}

func (cache *RCache) RemoveLru(buf *Buffer) {
	cache.Lock()
	defer cache.Unlock()

	if buf.elm != nil {
		cache.LruList.Remove(buf.elm)
		buf.elm = nil
	}
}

func (cache *RCache) LruExpire() (fe *FileEntry, blkIdx uint64) {
	cache.Lock()
	defer cache.Unlock()

	if cache.LruList.Len() == 0 {
		return nil, 0
	}

	buffer := cache.LruList.Back().Value.(*Buffer)
	Assert(cache.LruList.Back() == buffer.elm)

	if time.Since(buffer.LastUpdateTime) < time.Millisecond*time.Duration(cache.ExpireTime) {
		return nil, 0
	}

	cache.LruList.Remove(buffer.elm)
	buffer.elm = nil

	return buffer.fe, buffer.BlkIdx
}

func (cache *RCache) GetFileBuffer(fe *FileEntry, bAlloc bool) *FileBuffer {
	cache.Lock()
	defer cache.Unlock()

	fileBuf, ok := cache.Cache[fe.Id]
	if !ok {
		if bAlloc {
			fileBuf = NewFileBuffer(fe)
			cache.Cache[fe.Id] = fileBuf
			return fileBuf
		} else {
			return nil
		}
	}

	return fileBuf
}

func (cache *RCache) DetachFileBuffer(fe *FileEntry) {
	cache.Lock()
	defer cache.Unlock()

	delete(cache.Cache, fe.Id)
}

func (cache *RCache) DirectRead(fe *FileEntry, buffer []byte, offset uint64) (int, error) {
	reader := NewReader(cache.Mount, fe)
	rdSize, rdErr := reader.Read(fe.Id, buffer, offset)
	return rdSize, rdErr
}

func (cache *RCache) Slice(fe *FileEntry, buffer []byte, offset uint64) []RWBlockOp {
	// align buffer to block size
	BlockSize := fe.BlockSize
	Size := uint32(len(buffer))
	blockIndexStart := offset / uint64(BlockSize)
	blockIndexEnd := (offset + uint64(Size) + uint64(BlockSize) - 1) / uint64(BlockSize)

	// assign block rw parameters
	RWOffset := uint32(offset % uint64(BlockSize))
	leftRwSize := Size
	rwBufOffset := uint32(0)

	// deal each blocks.
	Ops := make([]RWBlockOp, 0)
	for blockIndexStart < blockIndexEnd {
		op := RWBlockOp{
			fe:       fe,
			Index:    blockIndexStart,
			RwOffset: 0,
			RwSize:   0,
			rwBuf:    nil,
		}

		op.RwOffset = RWOffset % BlockSize
		RWSize := BlockSize - op.RwOffset
		if RWSize > leftRwSize {
			RWSize = leftRwSize
		}

		op.RwSize = RWSize
		op.rwBuf = buffer[rwBufOffset : rwBufOffset+op.RwSize]

		RWOffset += RWSize
		leftRwSize -= RWSize
		rwBufOffset += RWSize

		Ops = append(Ops, op)

		blockIndexStart++
		Assert(blockIndexStart <= blockIndexEnd)
	}

	return Ops
}

func (cache *RCache) ReadOp(fe *FileEntry, fileBuf *FileBuffer, op RWBlockOp) (int, error) {
	buffer := fileBuf.GetBuffer(op.Index)

	// cache not exist.
	if buffer == nil {
		size, err := cache.DirectRead(fe, op.rwBuf, op.GetFileOffset())
		return size, err
	}

	Assert(buffer.Offset == 0 && buffer.Len != 0)
	if op.RwOffset >= buffer.Len {
		return 0, nil
	}

	Size := buffer.Len - op.RwOffset
	if Size > op.RwSize {
		Size = op.RwSize
	}

	copy(op.rwBuf, buffer.GetRangeData(op.RwOffset, op.RwOffset+Size))
	cache.Mount.st.EndStat("CacheReadHit", nil, nil, 1)
	return int(Size), nil
}

func (cache *RCache) GetReadWindow(fe *FileEntry, Size uint32, offset uint64) *PrefetchWindow {
	BlockSize := fe.BlockSize
	return &PrefetchWindow{
		BlkStart: offset / uint64(BlockSize),
		BlkEnd:   (offset + uint64(Size) + uint64(BlockSize) - 1) / uint64(BlockSize),
	}
}

func (cache *RCache) DoPrefetch(fileBuf *FileBuffer, Size uint32, offset uint64) {
	Assert(cache.chPrefetch != nil)

	rw := cache.GetReadWindow(fileBuf.fe, Size, offset)
	Assert(rw.Size() > 0)

	fileBuf.pwLock.Lock()
	if rw.GetStart() == 0 && fileBuf.pw.Size() == 0 {
		fileBuf.pw = rw
		fileBuf.pw.Move()
		fileBuf.pw.Extend(uint32(cache.PrefetchTimes))
	} else if rw.Cross(fileBuf.pw) {
		Assert(fileBuf.pw.Size() > 0)
		fileBuf.pw.Move()
	} else {
		fileBuf.pwLock.Unlock()
		return
	}

	blkStart := fileBuf.pw.GetStart()
	blkEnd := fileBuf.pw.GetEnd()
	fileBuf.pwLock.Unlock()

	// push into prefetch chan.
	for blkStart < blkEnd {
		op := &RWBlockOp{
			fe:       fileBuf.fe,
			Index:    blkStart,
			RwOffset: 0,
			RwSize:   fileBuf.fe.BlockSize,
			rwBuf:    make([]byte, fileBuf.fe.BlockSize),
		}

		fileBuf.LockBuffer(op.Index)

		// don't read until the cache expire.
		buffer := fileBuf.GetBuffer(op.Index)
		if buffer != nil {
			fileBuf.UnLockBuffer(op.Index)
		} else {
			cache.chPrefetch <- op
		}

		blkStart++
	}
}

func (cache *RCache) TryDoPrefetch(fileBuf *FileBuffer, Size uint32, offset uint64) {
	// do prefetch
	if cache.chPrefetch != nil {
		cache.DoPrefetch(fileBuf, Size, offset)
	}
}

func (cache *RCache) ReleaseBuffer(fileBuf *FileBuffer, Index uint64) error {
	fileBuf.LockBuffer(Index)
	defer fileBuf.UnLockBuffer(Index)

	buffer := fileBuf.GetBuffer(Index)
	if buffer == nil {
		return nil
	}

	// remove from lru list.
	cache.RemoveLru(buffer)

	// free buf.
	fileBuf.DetachBuffer(Index)
	cache.FreeBuffer(buffer)

	return nil
}

func (cache *RCache) Release(fe *FileEntry) error {
	fileBuf := cache.GetFileBuffer(fe, false)
	if fileBuf == nil {
		return nil
	}

	// wait prefetch complete.
	fileBuf.Wait()

	// free cache.
	bufIndexS := fileBuf.GetAllBufferIndex()
	for _, Index := range bufIndexS {
		bgTime := cache.Mount.st.BeginStat()
		err := cache.ReleaseBuffer(fileBuf, Index)
		cache.Mount.st.EndStat("CacheRelease", err, bgTime, 1)
		if err != nil {
			return err
		}
	}

	return nil
}

func (cache *RCache) CheckExpire() error {
	lastCheckTime := time.Now()
	CheckGap := time.Millisecond * time.Duration(
		cache.Mount.Cfg.GetIntValue(consts.CfgRwCacheCheckGap, 300, cache.log))

	for {
		if cache.Stop {
			break
		}

		if time.Since(lastCheckTime) < CheckGap {
			time.Sleep(100 * time.Millisecond)
			continue
		}

		for {
			fe, blkIdx := cache.LruExpire()
			if fe == nil {
				break
			}

			fileBuf := cache.GetFileBuffer(fe, false)
			if fileBuf == nil {
				continue
			}

			bgTime := cache.Mount.st.BeginStat()
			cache.ReleaseBuffer(fileBuf, blkIdx)
			cache.Mount.st.EndStat("ExpireRelease", nil, bgTime, 1)
		}

		lastCheckTime = time.Now()
	}

	cache.wg.Done()
	return nil
}

func (cache *RCache) DoPrefetchRead(fileBuf *FileBuffer, op *RWBlockOp) error {
	size, err := cache.DirectRead(op.fe, op.rwBuf, op.GetFileOffset())
	if err != nil {
		return err
	}

	if size == 0 {
		return nil
	}

	Assert(size > 0)
	buffer := cache.AllocBuffer(op.fe, op.Index)
	if buffer == nil {
		cache.Mount.st.EndStat("RCacheAlloc", fmt.Errorf(""),
			nil, 1)

		cache.log.Error("RCache:: AllocRBuffer fail.",
			zap.Any("fd", op.fe.Id),
			zap.Any("op", op))
		return fmt.Errorf("AllocRBuffer")
	}

	buffer.Skip(op.RwOffset)
	Assert(buffer.CanMerge(op.RwOffset))
	buffer.MergeData(op.rwBuf)
	fileBuf.AttachBuffer(op.Index, buffer)
	cache.DoLru(buffer)

	return nil
}

func (cache *RCache) PrefetchRead() error {
	for v := range cache.chPrefetch {
		op, ok := v.(*RWBlockOp)
		if ok {
			fileBuf := cache.GetFileBuffer(op.fe, false)
			Assert(fileBuf != nil)

			bgTime := cache.Mount.st.BeginStat()
			err := cache.DoPrefetchRead(fileBuf, op)
			fileBuf.UnLockBuffer(op.Index)
			cache.Mount.st.EndStat("PrefetchRead", err, bgTime, 1)

		} else {
			Assert(false)
		}
	}

	cache.wg.Done()
	return nil
}

func (cache *RCache) Clean(fe *FileEntry, Size int, offset uint64) error {
	fileBuf := cache.GetFileBuffer(fe, false)
	if fileBuf == nil {
		return nil
	}

	rw := cache.GetReadWindow(fe, uint32(Size), offset)
	Assert(rw.Size() > 0)

	blkStart := rw.GetStart()
	blkEnd := rw.GetEnd()

	// clean cache.
	for blkStart < blkEnd {
		cache.ReleaseBuffer(fileBuf, blkStart)
		blkStart++
	}

	return nil
}

func (cache *RCache) Close(fe *FileEntry) error {
	err := cache.Release(fe)
	if err != nil {
		return err
	}

	cache.DetachFileBuffer(fe)
	return nil
}

func (cache *RCache) CloseAll() {
	// prefetch routine
	if cache.chPrefetch != nil {
		close(cache.chPrefetch)
	}

	// check expire routine
	cache.Stop = true

	feList := make([]*FileEntry, 0)

	cache.Lock()
	for _, fileBuf := range cache.Cache {
		feList = append(feList, fileBuf.fe)
	}
	cache.Unlock()

	for _, fe := range feList {
		cache.Close(fe)
	}

	cache.wg.Wait()
}
