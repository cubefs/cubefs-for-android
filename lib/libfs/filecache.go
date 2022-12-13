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
	"fmt"
	"sync"
	"syscall"
	"time"
)

type FileEntry struct {
	sync.RWMutex
	Id        uint64 // file's dentry id
	BlockSize uint32 // block size
	Handle    int    // upper-layer file handle
	FilePath  string // file path
	OpenFlag  int
	Mode      uint32 // permission

	DirectIO bool // Direct IO

	Owner          string // Flock owner
	Pid            uint32 // Flock pid
	LastAccessTime time.Time
	close          chan struct{}
}

type fileRefer map[uint64]int

func (fr *fileRefer) AddRef(Id uint64) {
	_, ok := (*fr)[Id]
	if !ok {
		(*fr)[Id] = 1
	} else {
		(*fr)[Id]++
	}
}

func (fr *fileRefer) DelRef(Id uint64) {
	ref, ok := (*fr)[Id]
	Assert(ok && ref >= 1)
	if ref == 1 {
		delete(*fr, Id)
	} else {
		(*fr)[Id]--
	}
}

func (fr *fileRefer) LastRef(Id uint64) bool {
	ref, ok := (*fr)[Id]
	Assert(ok && ref >= 1)
	return ref == 1
}

// read write Cache
type FileCache struct {
	fileEntryIndex   uint32
	fileEntryVecSize uint32
	fileEntryVec     []*FileEntry // index: handle(linux): val: File RW Cache.
	fileRefer        fileRefer
	sync.RWMutex
}

func NewFileCache(openFileMax int) *FileCache {
	return &FileCache{
		fileEntryIndex:   0,
		fileEntryVecSize: uint32(openFileMax),
		fileEntryVec:     make([]*FileEntry, openFileMax),
		fileRefer:        make(map[uint64]int),
		RWMutex:          sync.RWMutex{},
	}
}

func (fileCache *FileCache) NewFile(fd *int, fileEntry *FileEntry) error {
	fileCache.Lock()
	defer fileCache.Unlock()

	DirectIO := false
	if fileEntry.OpenFlag&syscall.O_DIRECT > 0 {
		DirectIO = true
	}

	if fileCache.fileEntryIndex >= fileCache.fileEntryVecSize {
		return fmt.Errorf("NewFile fail! path %s, did %d", fileEntry.FilePath, fileEntry.Id)
	}

	if fileCache.fileEntryVec[fileCache.fileEntryIndex] != nil {
		panic("fileEntryVec not nil")
	}

	fileCache.fileEntryVec[fileCache.fileEntryIndex] = &FileEntry{
		RWMutex:        sync.RWMutex{},
		Id:             fileEntry.Id,
		BlockSize:      fileEntry.BlockSize,
		Handle:         int(fileCache.fileEntryIndex),
		FilePath:       fileEntry.FilePath,
		OpenFlag:       fileEntry.OpenFlag,
		Mode:           fileEntry.Mode,
		DirectIO:       DirectIO,
		LastAccessTime: time.Now(),
		close:          make(chan struct{}),
	}

	if !fileCache.fileEntryVec[fileCache.fileEntryIndex].DirectIO {
		fileCache.fileRefer.AddRef(fileEntry.Id)
	}

	*fd = int(fileCache.fileEntryIndex)
	fileCache.fileEntryIndex = fileCache.GetMinEntryIndex()

	return nil
}

func (fileCache *FileCache) GetFileEntry(handle uint32) (*FileEntry, error) {
	fileCache.RLock()
	defer fileCache.RUnlock()

	if handle >= fileCache.fileEntryVecSize ||
		fileCache.fileEntryVec[handle] == nil {
		return nil, fmt.Errorf("GetFileEntry invalid handle %d", handle)
	}

	fileCache.fileEntryVec[handle].LastAccessTime = time.Now()

	return fileCache.fileEntryVec[handle], nil
}

func (fileCache *FileCache) CloseFile(handle uint32) error {
	fileCache.Lock()
	defer fileCache.Unlock()

	if handle >= fileCache.fileEntryVecSize ||
		fileCache.fileEntryVec[handle] == nil {
		return fmt.Errorf("fileCache.CloseFile, invalid handle %d", handle)
	}

	if !fileCache.fileEntryVec[handle].DirectIO {
		fileCache.fileRefer.DelRef(fileCache.fileEntryVec[handle].Id)
	}

	close(fileCache.fileEntryVec[handle].close)
	fileCache.fileEntryVec[handle] = nil

	if handle < fileCache.fileEntryIndex {
		fileCache.fileEntryIndex = handle
	}

	return nil
}

func (fileCache *FileCache) GetMinEntryIndex() uint32 {
	next := fileCache.fileEntryIndex
	for ; next < fileCache.fileEntryVecSize; next++ {
		if fileCache.fileEntryVec[next] == nil {
			break
		}
	}
	return next
}

func (fileCache *FileCache) IsLastRefer(fileEntry *FileEntry) bool {
	fileCache.Lock()
	defer fileCache.Unlock()
	return fileCache.fileRefer.LastRef(fileEntry.Id)
}
