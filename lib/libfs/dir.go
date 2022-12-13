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
	"log"
	"strings"
	"sync"
	"time"

	"github.com/cubefs/cubefs-for-android/lib/libfs/stat"
	"github.com/cubefs/cubefs-for-android/lib/libfs/util"
	zap "github.com/cubefs/cubefs-for-android/lib/log"
	. "github.com/cubefs/cubefs-for-android/lib/proto"
)

type LsStatus int

const (
	LsFetching LsStatus = iota
	LsDone
	LsError
	LsUserAbort
)

type Dir struct {
	dirList   list.List
	startKey  string
	lsSize    uint16
	lsStatus  LsStatus // 0.fetching. 1.done. 2.error
	cacheSize uint
	Error     error
	ch        chan bool
	btTime    time.Time
	st        *stat.Statistic
	log       *util.ClientLogger
	sync.Mutex
}

// mkdir
func (mount *CfaMountInfo) Mkdir(path string, mode uint32) error {
	log := util.CloneLogger(mount.log)
	log.Debug("CfaMountInfo Mkdir run", zap.String("path", path))

	if mount.checkMount(path, MountModeWrite, log) != nil {
		return E_POSIX_EACCES
	}

	mode = mode&SysPerm | S_IFDIR
	resp, err := mount.proxyClient.MkDir(path, mode, log)

	if err != nil || resp.Code != 0 {
		log.Error("Invoke Mkdir fail",
			zap.String("Path", path),
			zap.Uint32("Mode", mode),
			zap.Int("code", resp.Code),
			zap.String("Message", resp.Message),
			zap.Any("err", err))
		return toErr(err, resp.Code)
	}

	mount.dentryCache.Remove(path[:strings.LastIndex(path, "/")])

	return nil
}

//cmd line 'mkdir -p'
func (mount *CfaMountInfo) MkdirAll(path string, mode uint32) error {
	log := util.CloneLogger(mount.log)
	log.Debug("CfaMountInfo MkdirAll run", zap.String("path", path))
	if mount.checkMount(path, MountModeWrite, log) != nil {
		return E_POSIX_EACCES
	}

	mode = mode&SysPerm | S_IFDIR

	baseDir := mount.Path
	subDirVec := strings.Split(path[len(mount.Path):], "/")
	for _, subDir := range subDirVec {
		if len(subDir) > 0 {
			if baseDir == "/" {
				baseDir = baseDir + subDir
			} else {
				baseDir = baseDir + "/" + subDir
			}

			resp, err := mount.proxyClient.MkDir(baseDir, mode, log)
			if err != nil {
				return err
			}

			// 目录已经存在
			if Errno(resp.Code) == E_POSIX_EEXIST {
				continue
			}

			if resp.Code != 0 {
				log.Error("Invoke Mkdir fail",
					zap.String("Path", baseDir),
					zap.Uint32("Mode", mode),
					zap.Int("code", resp.Code),
					zap.String("Message", resp.Message))
				return Errno(resp.Code)
			}

			mount.dentryCache.Remove(path[:strings.LastIndex(path, "/")])
		}
	}

	return nil
}

func (mount *CfaMountInfo) Mknod(path string, mode uint32, dev int) error {
	log := util.CloneLogger(mount.log)
	log.Debug("CfaMountInfo Mknod run", zap.String("path", path))
	if mount.checkMount(path, MountModeWrite, log) != nil {
		return E_POSIX_EACCES
	}

	if (mode&S_IFIFO == 0 && mode&S_IFSOCK == 0) || dev != 0 {
		return E_POSIX_ENOTSUP
	}

	// only support fifo, socket.
	if mode&S_IFIFO != 0 {
		mode = mode&SysPerm | mode&S_IFIFO
	} else if mode&S_IFSOCK != 0 {
		mode = mode&SysPerm | mode&S_IFSOCK
	}

	openFlag := OpenFlagCreate
	openResp, err := mount.proxyClient.Open(path, openFlag, mode, log)

	if err != nil || openResp.Code != 0 {
		log.Error("Invoke Open fail",
			zap.String("Path", path),
			zap.Uint32("OpenFlag", uint32(openFlag)),
			zap.Uint32("Mode", uint32(mode)),
			zap.Int("code", openResp.Code),
			zap.String("Message", openResp.Message),
			zap.Any("err", err))
		return toErr(err, openResp.Code)
	}

	mount.dentryCache.Remove(path[:strings.LastIndex(path, "/")])
	return nil
}

// Opendir reads all items under the dir
func (mount *CfaMountInfo) Opendir(path string) (*Dir, error) {
	log := util.CloneLogger(mount.log)
	log.Debug("CfaMountInfo Opendir run", zap.String("path", path))
	if mount.checkMount(path, MountModeRead, log) != nil {
		return nil, E_POSIX_EACCES
	}

	newDir := &Dir{
		dirList:   list.List{},
		startKey:  "",
		lsSize:    mount.LsSize,
		lsStatus:  LsFetching,
		cacheSize: uint(mount.LsSize),
		Error:     nil,
		ch:        make(chan bool, 1),
		btTime:    time.Now(),
		st:        mount.st,
		Mutex:     sync.Mutex{},
		log:       log,
	}

	log.Debug("open dir", zap.Uint16("ls_size", newDir.lsSize), zap.String("path", path))

	go func(newDir *Dir) {
		var lastErr error
		for {
			newDir.Lock()
			// user abort.
			if newDir.lsStatus == LsUserAbort {
				newDir.Unlock()
				break
			}

			// cache full.
			if uint(newDir.dirList.Len()) > newDir.cacheSize {
				newDir.Unlock()
				time.Sleep(1 * time.Microsecond)
				continue
			}
			newDir.Unlock()

			resp, err := mount.proxyClient.ReadDirEx(path, newDir.startKey, newDir.lsSize, log)
			if err != nil || resp.Code != 0 {
				log.Error("Invoke ReadDirEx fail",
					zap.String("Path", path),
					zap.Int("code", resp.Code),
					zap.Error(err),
					zap.String("Message", resp.Message))
				lastErr = toErr(err, resp.Code)
				break
			}

			log.Debug("read dir success",
				zap.String("start", newDir.startKey),
				zap.Int("len", len(resp.Dentrys)))

			newDir.Lock()
			for _, entry := range resp.Dentrys {
				newDir.dirList.PushBack(entry)
			}

			len := len(resp.Dentrys)
			if len < int(newDir.lsSize) {
				newDir.lsStatus = LsDone
			} else {
				newDir.startKey = resp.Dentrys[len-1].Name
			}
			newDir.Unlock()

			// notify reader if has.
			select {
			case newDir.ch <- true:
			default:
			}

			// add to cache.
			for _, entry := range resp.Dentrys {
				mount.dentryCache.Set(path+"/"+entry.Name, entry)
			}

			// ls complete.
			if len < int(newDir.lsSize) {
				break
			}
		}

		if lastErr != nil {
			newDir.Lock()
			newDir.lsStatus = LsError
			newDir.Error = lastErr
			newDir.Unlock()

			// notify reader if has.
			select {
			case newDir.ch <- true:
			default:
			}
		}
	}(newDir)

	return newDir, nil
}

// read 'count' items under the dir
func (dir *Dir) Readdir(count uint) ([]Dentry, error) {
	dir.log.Debug("CfaMountInfo Readdir run", zap.Uint("count", count))

	dentrys := make([]Dentry, count)
	for {
		dir.Lock()

		dir.log.Debug("read dir", zap.Uint("count", count), zap.Uint("cache_size", dir.cacheSize),
			zap.Any("status", dir.lsStatus))

		// update cache size.
		if count*2 > dir.cacheSize {
			dir.cacheSize = count * 2
		}

		if dir.lsStatus == LsError || dir.lsStatus == LsUserAbort { // read error.
			dir.Unlock()
			return nil, dir.Error
		} else if dir.lsStatus == LsDone && dir.dirList.Len() == 0 { // fetching complete.
			dir.Unlock()
			return make([]Dentry, 0), nil
		} else if dir.lsStatus == LsFetching && uint(dir.dirList.Len()) < count { // fetching. go to wait.
			dir.Unlock()
			<-dir.ch
		} else {
			index := uint(0)
			for ; index < count && dir.dirList.Len() > 0; index++ {
				elm := dir.dirList.Front()
				dentrys[index] = elm.Value.(Dentry)
				dir.dirList.Remove(elm)
			}

			dir.Unlock()
			return dentrys[:index], nil
		}
	}
}

func (dir *Dir) CloseDir() error {
	if dir == nil {
		log.Panic("dir is empty")
	}

	dir.Lock()
	dir.lsStatus = LsUserAbort
	dir.Unlock()

	dir.st.EndStat("Ls", dir.Error, &dir.btTime, 1)
	return nil
}
