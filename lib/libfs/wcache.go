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
	. "github.com/cubefs/cubefs-for-android/lib/proto"
)

type WCache struct {
	sync.RWMutex
	Mount      *CfaMountInfo
	BufferMax  int
	FreeList   *list.List
	Cache      map[uint64]*FileBuffer
	LruList    *list.List
	log        *ClientLogger
	ExpireTime int

	SyncRoutine int
	chSync      chan interface{}

	Stop bool
	wg   sync.WaitGroup
}

func NewWCache(mount *CfaMountInfo, BufferMax int, ExpireTime int, SyncRoutine int) *WCache {
	cache := &WCache{
		RWMutex:     sync.RWMutex{},
		Mount:       mount,
		BufferMax:   BufferMax,
		FreeList:    list.New(),
		Cache:       make(map[uint64]*FileBuffer),
		LruList:     list.New(),
		log:         mount.log,
		ExpireTime:  ExpireTime,
		SyncRoutine: SyncRoutine,
		chSync:      nil,
		Stop:        false,
		wg:          sync.WaitGroup{},
	}

	for i := 0; i < BufferMax; i++ {
		cache.FreeList.PushBack(NewBuffer(uint32(i)))
	}

	if SyncRoutine > 0 {
		cache.chSync = make(chan interface{}, cache.SyncRoutine)
		for i := 0; i < SyncRoutine; i++ {
			cache.wg.Add(1)
			go cache.SyncWrite()
		}
	}

	defaultCheckExpireRoutine := 8
	for i := 0; i < defaultCheckExpireRoutine; i++ {
		cache.wg.Add(1)
		go cache.CheckExpire()
	}

	return cache
}

func (cache *WCache) AllocBuffer(fe *FileEntry, BlkIdx uint64) *Buffer {
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

func (cache *WCache) FreeBuffer(buf *Buffer) {
	cache.Lock()
	defer cache.Unlock()

	cache.FreeList.PushBack(buf)
}

func (cache *WCache) DoLru(buf *Buffer) {
	cache.Lock()
	defer cache.Unlock()

	buf.LastUpdateTime = time.Now()
	if buf.elm == nil {
		buf.elm = cache.LruList.PushFront(buf)
	} else {
		cache.LruList.MoveToFront(buf.elm)
	}
}

func (cache *WCache) RemoveLru(buf *Buffer) {
	cache.Lock()
	defer cache.Unlock()

	if buf.elm != nil {
		cache.LruList.Remove(buf.elm)
		buf.elm = nil
	}
}

func (cache *WCache) LruExpire() (fe *FileEntry, blkIdx uint64) {
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

// bAlloc 是否申请file buf
func (cache *WCache) GetFileBuffer(fe *FileEntry, bAlloc bool) *FileBuffer {
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

func (cache *WCache) DetachFileBuffer(fe *FileEntry) {
	cache.Lock()
	defer cache.Unlock()

	delete(cache.Cache, fe.Id)
}

func (cache *WCache) DirectWrite(fe *FileEntry, buffer []byte, offset uint64) (int, error) {
	writer := NewWriter(cache.Mount, fe)
	wtSize, wtErr := writer.Write(fe.Id, buffer, offset)
	return wtSize, wtErr
}

func (cache *WCache) SyncIo() bool {
	return cache.chSync == nil
}

func (cache *WCache) Slice(fe *FileEntry, buffer []byte, offset uint64) []RWBlockOp {
	// align buffer to block size
	blockSize := fe.BlockSize
	size := uint32(len(buffer))
	blockIndexStart := offset / uint64(blockSize)
	blockIndexEnd := (offset + uint64(size) + uint64(blockSize) - 1) / uint64(blockSize)

	// assign block rw parameters
	rwOffset := uint32(offset % uint64(blockSize))
	leftRwSize := size
	rwBufOffset := uint32(0)

	// deal each blocks.
	ops := make([]RWBlockOp, 0)
	for blockIndexStart < blockIndexEnd {
		op := RWBlockOp{
			fe:       fe,
			Index:    blockIndexStart,
			RwOffset: 0,
			RwSize:   0,
			rwBuf:    nil,
		}

		op.RwOffset = rwOffset % blockSize
		RWSize := blockSize - op.RwOffset
		if RWSize > leftRwSize {
			RWSize = leftRwSize
		}

		op.RwSize = RWSize
		op.rwBuf = buffer[rwBufOffset : rwBufOffset+op.RwSize]

		rwOffset += RWSize
		leftRwSize -= RWSize
		rwBufOffset += RWSize

		ops = append(ops, op)

		blockIndexStart++
	}

	return ops
}

func (cache *WCache) WriteOp(fe *FileEntry, fileBuf *FileBuffer, op RWBlockOp) (bool, int, error) {
	syncIo := true
	buffer := fileBuf.GetBuffer(op.Index)

	// cache not exist.
	if buffer == nil {
		if op.IsFull() && cache.SyncIo() {
			size, err := cache.DirectWrite(fe, op.rwBuf, op.GetFileOffset())
			return syncIo, size, err

		} else {
			buffer = cache.AllocBuffer(fe, op.Index)

			// alloc fail, so do direct io.
			if buffer == nil {
				cache.Mount.st.EndStat("WCacheAlloc", fmt.Errorf(""),
					nil, 1)

				cache.log.Error("WCache:: AllocBuffer fail.",
					zap.Any("fd", fe.Id),
					zap.Any("Op", op))

				size, err := cache.DirectWrite(fe, op.rwBuf, op.GetFileOffset())
				return syncIo, size, err

			} else {
				buffer.Skip(op.RwOffset)
				Assert(buffer.CanMerge(op.RwOffset))
				buffer.MergeData(op.rwBuf)
				fileBuf.AttachBuffer(op.Index, buffer)

				cache.log.Info("WCache:: New Buffer",
					zap.Any("Id", fe.Id),
					zap.Any("buffer", buffer),
					zap.Any("Op", op))

				if buffer.IsFull(fe.BlockSize) {
					Assert(op.IsFull() && !cache.SyncIo())
					syncIo = false
					cache.chSync <- buffer
					return syncIo, int(op.RwSize), nil

				} else {
					cache.DoLru(buffer)
					return syncIo, int(op.RwSize), nil

				}
			}
		}

	} else { // cache exist.
		cache.RemoveLru(buffer)

		if buffer.CanMerge(op.RwOffset) {
			buffer.MergeData(op.rwBuf)

			cache.log.Info("WCache:: Merged",
				zap.Any("fd", fe.Id),
				zap.Any("buffer", buffer),
				zap.Any("Op", op))

			// buffer is full, must do flush.
			if buffer.IsFull(fe.BlockSize) {
				if !cache.SyncIo() {
					syncIo = false
					cache.chSync <- buffer
					return syncIo, int(op.RwSize), nil
				} else {
					size, err := cache.DirectWrite(fe, buffer.GetData(), buffer.GetFileOffset())
					if err != nil {
						cache.DoLru(buffer)
						return syncIo, size, err
					}

					fileBuf.DetachBuffer(op.Index)
					cache.FreeBuffer(buffer)
				}
			} else {
				cache.DoLru(buffer)
			}

			return syncIo, int(op.RwSize), nil

		} else {

			// cache exist. flush first
			size, err := cache.DirectWrite(fe, buffer.GetData(), buffer.GetFileOffset())
			if err != nil {
				cache.DoLru(buffer)
				return syncIo, size, err
			}

			// can't merge, do direct io
			if op.IsFull() {
				// free prev write buf first.
				fileBuf.DetachBuffer(op.Index)
				cache.FreeBuffer(buffer)

				size, err := cache.DirectWrite(fe, op.rwBuf, op.GetFileOffset())
				return syncIo, size, err

			} else {
				buffer.ReSet(fe, op.Index)
				buffer.Skip(op.RwOffset)
				Assert(buffer.CanMerge(op.RwOffset))
				buffer.MergeData(op.rwBuf)
				cache.DoLru(buffer)

				cache.log.Info("WCache:: Reset Buffer",
					zap.Any("Id", fe.Id),
					zap.Any("buffer", buffer),
					zap.Any("Op", op))

				return syncIo, int(op.RwSize), nil
			}
		}
	}
}

func (cache *WCache) MergeOp(fileBuf *FileBuffer, op RWBlockOp, dataSize int) int {
	Assert(dataSize <= int(op.RwSize))
	buffer := fileBuf.GetBuffer(op.Index)
	if buffer == nil {
		// read hit hole.
		if dataSize < int(op.RwSize) && op.Index < fileBuf.UpperIdx() {
			return int(op.RwSize)
		} else {
			return dataSize
		}
	}

	// op and buffer not intersection.
	prevOpSize := dataSize
	if buffer.Offset+buffer.Len <= op.RwOffset ||
		op.RwOffset+op.RwSize <= buffer.Offset {
		// read hit hole.
		if dataSize < int(op.RwSize) && op.RwOffset < buffer.Offset {
			return int(op.RwSize)
		} else {
			return dataSize
		}
	}

	// only copy the min range.
	cpOffset := buffer.Offset
	if op.RwOffset > cpOffset {
		cpOffset = op.RwOffset
	}

	cpTailOffset := buffer.Offset + buffer.Len
	if op.RwOffset+op.RwSize < cpTailOffset {
		cpTailOffset = op.RwOffset + op.RwSize
	}

	Assert(cpOffset < cpTailOffset)
	Assert(cpTailOffset <= op.RwOffset+op.RwSize)
	copy(op.rwBuf[cpOffset-op.RwOffset:], buffer.GetRangeData(cpOffset, cpTailOffset))
	cache.Mount.st.StatBandWidth("CacheMerge", cpTailOffset-cpOffset)

	// reset read size.
	if dataSize < int(cpTailOffset-op.RwOffset) {
		dataSize = int(cpTailOffset - op.RwOffset)
	}

	cache.log.Info("WCache:: Read Hit",
		zap.Any("buffer", buffer),
		zap.Any("Op", op),
		zap.Any("prevOpSize", prevOpSize),
		zap.Any("dataSize", dataSize),
		zap.Any("Merge Size", cpTailOffset-cpOffset))

	return dataSize
}

func (cache *WCache) Write(fe *FileEntry, buffer []byte, offset uint64) (int, error) {
	fileBuf := cache.GetFileBuffer(fe, true)
	Assert(fileBuf != nil)

	var wtSize = 0
	Ops := cache.Slice(fe, buffer, offset)
	for _, op := range Ops {
		bgTime := cache.Mount.st.BeginStat()

		fileBuf.LockBuffer(op.Index)
		syncIo, size, err := cache.WriteOp(fe, fileBuf, op)
		if syncIo {
			fileBuf.UnLockBuffer(op.Index)
		}

		cache.Mount.st.EndStat("CacheWrite", err, bgTime, 1)

		if err != nil {
			return wtSize, err
		}

		wtSize += size
	}

	Assert(wtSize == len(buffer))
	return wtSize, nil
}

func (cache *WCache) FlushBuffer(fileBuf *FileBuffer, Index uint64) error {
	fileBuf.LockBuffer(Index)
	defer fileBuf.UnLockBuffer(Index)

	buffer := fileBuf.GetBuffer(Index)
	if buffer == nil {
		return nil
	}

	// remove from lru list.
	cache.RemoveLru(buffer)

	// cache exist. flush.
	_, err := cache.DirectWrite(buffer.fe, buffer.GetData(), buffer.GetFileOffset())
	if err != nil {
		code, ok := err.(Errno)
		if ok && E_POSIX_ENOENT.Equal(code) {
			cache.log.Error("WCache:: FlushBuffer fail, drop data.",
				zap.Any("Id", buffer.fe.Id),
				zap.Any("buffer", buffer),
				zap.Any("err", err))

			// fail, free buf.
			fileBuf.DetachBuffer(Index)
			cache.FreeBuffer(buffer)

			return code
		}

		cache.DoLru(buffer)
		return err
	}

	// free buf.
	fileBuf.DetachBuffer(Index)
	cache.FreeBuffer(buffer)

	return nil
}

func (cache *WCache) Flush(fe *FileEntry) error {
	fileBuf := cache.GetFileBuffer(fe, false)
	if fileBuf == nil {
		return nil
	}

	bufIndexS := fileBuf.GetAllBufferIndex()
	if len(bufIndexS) == 0 {
		return nil
	}

	mq := make(chan uint64, len(bufIndexS))
	rs := make(chan error, len(bufIndexS))

	defaultFlushRoutine := 8
	if len(bufIndexS) < defaultFlushRoutine {
		defaultFlushRoutine = len(bufIndexS)
	}

	for i := 0; i < defaultFlushRoutine; i++ {
		go func() {
			for {
				Index, ok := <-mq
				if !ok {
					break
				}

				bgTime := cache.Mount.st.BeginStat()
				err := cache.FlushBuffer(fileBuf, Index)
				cache.Mount.st.EndStat("CacheFlush", err, bgTime, 1)
				rs <- err
			}
		}()
	}

	for _, Index := range bufIndexS {
		mq <- Index
	}

	var flushErr error
	for i := 0; i < len(bufIndexS); i++ {
		err := <-rs
		if err != nil {
			flushErr = err // one buffer flush fail.
		}
	}

	close(mq)
	close(rs)

	return flushErr
}

func (cache *WCache) CheckExpire() error {
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
			err := cache.FlushBuffer(fileBuf, blkIdx)
			cache.Mount.st.EndStat("ExpireFlush", err, bgTime, 1)
		}

		lastCheckTime = time.Now()
	}

	cache.wg.Done()
	return nil
}

func (cache *WCache) SyncWrite() error {
	for v := range cache.chSync {
		buffer, ok := v.(*Buffer)
		if ok {
			fileBuf := cache.GetFileBuffer(buffer.fe, false)
			Assert(fileBuf != nil)

			_, err := cache.DirectWrite(buffer.fe, buffer.GetData(), buffer.GetFileOffset())
			if err != nil {
				cache.DoLru(buffer)
				fileBuf.UnLockBuffer(buffer.BlkIdx)
			} else {
				fileBuf.DetachBuffer(buffer.BlkIdx)
				cache.FreeBuffer(buffer)
				fileBuf.UnLockBuffer(buffer.BlkIdx)
			}
		} else {
			Assert(false)
		}
	}

	cache.wg.Done()
	return nil
}

func (cache *WCache) StatSize(fe *FileEntry) (uint64, error) {
	fileBuf := cache.GetFileBuffer(fe, false)
	if fileBuf == nil {
		return 0, nil
	}

	if fileBuf.IsZeroCache() {
		return 0, nil
	}

	UpperIdx := fileBuf.UpperIdx()
	fileBuf.LockBuffer(UpperIdx)
	defer fileBuf.UnLockBuffer(UpperIdx)

	buffer := fileBuf.GetBuffer(UpperIdx)
	if buffer == nil {
		return 0, E_POSIX_EIO
	}

	return buffer.GetTailOffset(), nil
}

func (cache *WCache) Close(fe *FileEntry) error {
	err := cache.Flush(fe)
	if err != nil {
		return err
	}

	cache.DetachFileBuffer(fe)
	return nil
}

func (cache *WCache) CloseAll() {
	// sync write routine
	if cache.chSync != nil {
		close(cache.chSync)
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
