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
	"io/fs"
	"strings"

	. "github.com/cubefs/cubefs-for-android/lib/libfs/util"
	zap "github.com/cubefs/cubefs-for-android/lib/log"
	. "github.com/cubefs/cubefs-for-android/lib/proto"
)

// 目录的统计信息
type Statfs struct {
	BlockSize uint32 // block size of the current file system
	Blocks    uint32 // block number of the current file system
	Files     uint32 // file numbers of the current dir, non-recursive
	Folders   uint32 // sub dir numbers under the current dir, non-recursive
	Fbytes    uint64 // size of all files under the current dir, non-recursive
	Fubytes   uint64 // space occupied by files in the current directory, non-recursive
	Rfiles    uint64 // file numbers of the current dir, the whole subtree
	Rfolders  uint64 // sub dir numbers under the current dir, the whole subtree
	Rbytes    uint64 // size of all files under the current dir, the whole subtree
	Rubytes   uint64 // space occupied by files in the current directory, the whole subtree
}

// get file attributes
func (mount *CfaMountInfo) doStat(path string, log *ClientLogger) (*Dentry, error) {
	resp, err := mount.proxyClient.Stat(path, log)
	if err != nil || resp.Code != 0 {
		log.Warn("invoke stat fail,",
			zap.String("Volume", mount.Volume),
			zap.String("path", path),
			zap.Int("code", resp.Code),
			zap.String("Message", resp.Message),
			zap.Any("err", err))
		return nil, toErr(err, resp.Code)
	}

	item_name := "/"
	if item_name != path {
		path_slice := strings.Split(path, "/")
		item_name = path_slice[len(path_slice)-1]
	}

	var item_type uint32
	fs_mode := fs.FileMode(resp.Data.Mode)
	if fs_mode.IsDir() {
		item_type = uint32(DentryTypeDir)
	} else if fs_mode.IsRegular() {
		item_type = uint32(DentryTypeFile)
	} else {
		item_type = uint32(DentryTypeLink)
	}

	dentry := &Dentry{
		Inode: resp.Data.Inode,
		Name:  item_name,
		Type:  item_type,
		Info:  &(resp.Data),
	}

	return dentry, nil
}

func (mount *CfaMountInfo) Stat(path string) (*Dentry, error) {
	log := CloneLogger(mount.log)

	if mount.checkMount(path, MountModeRead, log) != nil {
		return nil, E_POSIX_EACCES
	}

	cacheData, hit, _ := mount.dentryCache.Get(path)
	if hit {
		dentry := cacheData.(Dentry)

		mount.st.EndStat("StatHit", nil, nil, 1)
		return &dentry, nil
	}

	dentry, err := mount.doStat(path, log)
	if err != nil {
		return nil, err
	}

	//  stat cache size.
	CacheSize := uint64(0)
	if mount.rwCache != nil {
		fe := mount.GetSimpleFileEntry(dentry)
		if fe != nil {
			CacheSize, err = mount.rwCache.wCache.StatSize(fe)
			if err != nil {
				// must stat again from disk.
				dentry, err = mount.doStat(path, log)
				if err != nil {
					return nil, err
				}
			} else if dentry.Info.Size < CacheSize {
				dentry.Info.Size = CacheSize
			}
		}
	}

	// add to cache.
	mount.dentryCache.Set(path, *dentry)

	log.Debug("Stat", zap.Uint64("Id", dentry.Inode),
		zap.String("path", path),
		zap.Any("dentry", dentry))

	return dentry, nil
}

func (mount *CfaMountInfo) Fstat(fd int) (*Dentry, error) {
	log := CloneLogger(mount.log)

	entry, err := mount.fileCache.GetFileEntry(uint32(fd))
	if err != nil {
		return nil, E_POSIX_EBADF
	}

	if mount.checkMount(entry.FilePath, MountModeRead, log) != nil {
		return nil, E_POSIX_EACCES
	}

	cacheData, hit, _ := mount.dentryCache.Get(entry.FilePath)
	if hit {
		dentry := cacheData.(Dentry)

		mount.st.EndStat("FStatHit", nil, nil, 1)
		return &dentry, nil
	}

	//  stat cache size.
	CacheSize := uint64(0)
	if mount.rwCache != nil {
		CacheSize, _ = mount.rwCache.wCache.StatSize(entry)
	}

	dentry, err := mount.doStat(entry.FilePath, log)
	if err != nil {
		return nil, err
	}

	if dentry.Info.Size < CacheSize {
		dentry.Info.Size = CacheSize
	}

	// add to cache.
	mount.dentryCache.Set(entry.FilePath, *dentry)

	log.Debug("FStat", zap.Uint64("Id", entry.Id),
		zap.String("path", entry.FilePath),
		zap.Any("dentry", dentry))

	return dentry, nil
}

func (mount *CfaMountInfo) Statfs(path string, stat *Statfs) error {
	log := CloneLogger(mount.log)

	if stat == nil {
		log.Warn("statfs is nil", zap.String("path", path))
		return E_POSIX_EPERM
	}

	if mount.checkMount(path, MountModeRead, log) != nil {
		return E_POSIX_EACCES
	}

	resp, err := mount.proxyClient.StatFs(path, log)
	if err != nil || resp.Code != 0 {
		log.Error("StatFs.invoke statfs dir fail,",
			zap.String("Volume", mount.Volume),
			zap.String("path", path),
			zap.Int("code", resp.Code),
			zap.String("Message", resp.Message),
			zap.Any("err", err))
		return toErr(err, resp.Code)
	}

	blkSize := uint64(4 * 1024)

	// assign data.
	stat.BlockSize = uint32(blkSize)
	stat.Blocks = uint32(resp.Data.Fbytes / blkSize)
	stat.Files = resp.Data.Files
	stat.Folders = resp.Data.Folders
	stat.Fbytes = resp.Data.Fbytes
	stat.Fubytes = resp.Data.Fubytes
	stat.Rfiles = resp.Data.Rfiles
	stat.Rfolders = resp.Data.Rfolders
	stat.Rbytes = resp.Data.Rbytes
	stat.Rubytes = resp.Data.Rubytes

	return nil
}
