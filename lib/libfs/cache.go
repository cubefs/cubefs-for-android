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
	"sync"
	"time"
)

func Assert(bOk bool) {
	if !bOk {
		panic(nil)
	}
}

type Buffer struct {
	Id             uint32
	fe             *FileEntry
	BlkIdx         uint64
	Offset         uint32
	Len            uint32
	dataBuf        []byte
	LastUpdateTime time.Time
	elm            *list.Element
}

func NewBuffer(Id uint32) *Buffer {
	return &Buffer{
		Id:             Id,
		fe:             nil,
		BlkIdx:         0,
		Offset:         0,
		Len:            0,
		dataBuf:        nil,
		LastUpdateTime: time.Unix(0, 0),
		elm:            nil,
	}
}

func (buf *Buffer) ReSet(fe *FileEntry, BlkIdx uint64) {
	buf.fe = fe
	buf.BlkIdx = BlkIdx
	buf.Offset = 0
	buf.Len = 0

	if buf.dataBuf == nil || uint32(len(buf.dataBuf)) < fe.BlockSize {
		buf.dataBuf = make([]byte, fe.BlockSize, fe.BlockSize)
	}

	buf.LastUpdateTime = time.Unix(0, 0)

	Assert(buf.elm == nil)
	buf.elm = nil
}

func (buf *Buffer) CanMerge(Offset uint32) bool {
	return buf.Offset+buf.Len == Offset
}

func (buf *Buffer) MergeData(data []byte) {
	copy(buf.dataBuf[buf.Offset+buf.Len:], data)
	buf.Len += uint32(len(data))
	buf.LastUpdateTime = time.Now()
}

func (buf *Buffer) IsFull(BlockSize uint32) bool {
	Assert((buf.Offset + buf.Len) <= BlockSize)
	return buf.Offset+buf.Len == BlockSize
}

func (buf *Buffer) Skip(Offset uint32) {
	Assert(buf.Offset == 0)
	buf.Offset += Offset
}

func (buf *Buffer) GetData() []byte {
	return buf.dataBuf[buf.Offset : buf.Offset+buf.Len]
}

func (buf *Buffer) GetRangeData(Start uint32, End uint32) []byte {
	Assert(buf.Offset <= Start)
	Assert(End <= buf.Offset+buf.Len)
	return buf.dataBuf[Start:End]
}

func (buf *Buffer) GetAlignOffset() uint64 {
	return buf.BlkIdx * uint64(buf.fe.BlockSize)
}

func (buf *Buffer) GetFileOffset() uint64 {
	return buf.GetAlignOffset() + uint64(buf.Offset)
}

func (buf *Buffer) GetTailOffset() uint64 {
	return buf.GetAlignOffset() + uint64(buf.Offset) + uint64(buf.Len)
}

type RWBlockOp struct {
	fe       *FileEntry
	Index    uint64 //	fragment index
	RwOffset uint32 //	fragment internal offset corresponding to the write operation
	RwSize   uint32 //	data size corresponding to the Write operation
	rwBuf    []byte //  data corresponding to the Write operation
}

func (op *RWBlockOp) GetAlignOffset() uint64 {
	return op.Index * uint64(op.fe.BlockSize)
}

func (op *RWBlockOp) GetFileOffset() uint64 {
	return op.GetAlignOffset() + uint64(op.RwOffset)
}

func (op *RWBlockOp) IsFull() bool {
	Assert(op.RwSize == uint32(len(op.rwBuf)))
	Assert((op.RwOffset + op.RwSize) <= op.fe.BlockSize)
	return op.RwOffset+op.RwSize == op.fe.BlockSize
}

type PrefetchWindow struct {
	BlkStart uint64
	BlkEnd   uint64
}

func NewPrefetchWindow() *PrefetchWindow {
	return &PrefetchWindow{
		BlkStart: 0,
		BlkEnd:   0,
	}
}

func (pw *PrefetchWindow) Reset() {
	pw.BlkStart = 0
	pw.BlkEnd = 0
}

func (pw *PrefetchWindow) Size() uint32 {
	Assert(pw.BlkStart <= pw.BlkEnd)
	return uint32(pw.BlkEnd - pw.BlkStart)
}

func (pw *PrefetchWindow) Move() {
	Step := pw.BlkEnd - pw.BlkStart
	pw.Shift(Step)
}

func (pw *PrefetchWindow) Shift(Step uint64) {
	pw.BlkStart += Step
	pw.BlkEnd += Step
}

func (pw *PrefetchWindow) Extend(Times uint32) {
	Step := pw.BlkEnd - pw.BlkStart
	pw.BlkEnd = pw.BlkStart + Step*uint64(Times)
}

func (pw *PrefetchWindow) GetStart() uint64 {
	return pw.BlkStart
}

func (pw *PrefetchWindow) GetEnd() uint64 {
	return pw.BlkEnd
}

func (pw *PrefetchWindow) Cross(rw *PrefetchWindow) bool {
	return !(pw.BlkEnd <= rw.BlkStart || rw.BlkEnd <= pw.BlkStart)
}

const (
	WriteLock int = 0
	ReadLock  int = 1
)

type CacheLock struct {
	tp    int
	refer int
}

type FileBuffer struct {
	sync.RWMutex
	fe        *FileEntry
	BufferMap map[uint64]*Buffer
	LockMap   map[uint64]*CacheLock

	// prefetch read
	pwLock sync.RWMutex
	pw     *PrefetchWindow
}

func NewFileBuffer(fe *FileEntry) *FileBuffer {
	return &FileBuffer{
		RWMutex:   sync.RWMutex{},
		fe:        fe,
		BufferMap: make(map[uint64]*Buffer),
		LockMap:   make(map[uint64]*CacheLock),
		pwLock:    sync.RWMutex{},
		pw:        NewPrefetchWindow(),
	}
}

func (fileBuf *FileBuffer) GetAllBufferIndex() []uint64 {
	fileBuf.RLock()
	defer fileBuf.RUnlock()

	bufIndexS := make([]uint64, 0)
	for Index, _ := range fileBuf.BufferMap {
		bufIndexS = append(bufIndexS, Index)
	}

	return bufIndexS
}

func (fileBuf *FileBuffer) UpperIdx() uint64 {
	fileBuf.RLock()
	defer fileBuf.RUnlock()

	TailIndex := uint64(0)
	for Index, _ := range fileBuf.BufferMap {
		if TailIndex < Index {
			TailIndex = Index
		}
	}

	return TailIndex
}

func (fileBuf *FileBuffer) IsZeroCache() bool {
	fileBuf.RLock()
	defer fileBuf.RUnlock()

	return len(fileBuf.BufferMap) == 0
}

func (fileBuf *FileBuffer) GetBuffer(blkIdx uint64) *Buffer {
	fileBuf.RLock()
	defer fileBuf.RUnlock()

	buf, ok := fileBuf.BufferMap[blkIdx]
	if !ok {
		return nil
	}
	return buf
}

func (fileBuf *FileBuffer) AttachBuffer(blkIdx uint64, buffer *Buffer) {
	fileBuf.Lock()
	defer fileBuf.Unlock()

	_, ok := fileBuf.BufferMap[blkIdx]
	Assert(!ok)
	fileBuf.BufferMap[blkIdx] = buffer

}

func (fileBuf *FileBuffer) DetachBuffer(blkIdx uint64) {
	fileBuf.Lock()
	defer fileBuf.Unlock()

	delete(fileBuf.BufferMap, blkIdx)
}

func (fileBuf *FileBuffer) LockBuffer(blkIdx uint64) {
	for {
		fileBuf.Lock()
		_, ok := fileBuf.LockMap[blkIdx]
		if ok {
			fileBuf.Unlock()
			time.Sleep(500 * time.Microsecond)
		} else {
			fileBuf.LockMap[blkIdx] = &CacheLock{
				tp:    WriteLock,
				refer: 0,
			}
			fileBuf.Unlock()
			break
		}
	}
}

func (fileBuf *FileBuffer) UnLockBuffer(blkIdx uint64) {
	fileBuf.Lock()
	defer fileBuf.Unlock()

	v, ok := fileBuf.LockMap[blkIdx]
	Assert(ok && v.tp == WriteLock && v.refer == 0)
	delete(fileBuf.LockMap, blkIdx)
}

func (fileBuf *FileBuffer) RLockBuffer(blkIdx uint64) {
	for {
		fileBuf.Lock()
		v, ok := fileBuf.LockMap[blkIdx]
		if ok && v.tp == WriteLock {
			fileBuf.Unlock()
			time.Sleep(500 * time.Microsecond)
		} else {
			if !ok {
				fileBuf.LockMap[blkIdx] = &CacheLock{
					tp:    ReadLock,
					refer: 1,
				}
			} else {
				fileBuf.LockMap[blkIdx].refer++
			}
			fileBuf.Unlock()
			break
		}
	}
}

func (fileBuf *FileBuffer) RUnLockBuffer(blkIdx uint64) {
	fileBuf.Lock()
	defer fileBuf.Unlock()

	v, ok := fileBuf.LockMap[blkIdx]
	Assert(ok && v.tp == ReadLock && v.refer >= 1)
	v.refer--
	if v.refer == 0 {
		delete(fileBuf.LockMap, blkIdx)
	}
}

func (fileBuf *FileBuffer) Wait() {
	for {
		fileBuf.RLock()

		if len(fileBuf.LockMap) > 0 {
			fileBuf.RUnlock()
			time.Sleep(500 * time.Microsecond)
		} else {
			fileBuf.RUnlock()
			break
		}
	}
}
