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
	"os"
	"strings"
	"sync"

	"github.com/cubefs/cubefs-for-android/lib/libfs/util"
	zap "github.com/cubefs/cubefs-for-android/lib/log"
	. "github.com/cubefs/cubefs-for-android/lib/proto"
)

const InvalidHandle = int(-1)

func (mount *CfaMountInfo) Open(path string, flags int, mode uint32) (int, error) {
	log := util.CloneLogger(mount.log)

	log.Info("Open",
		zap.String("path", path),
		zap.Int("flags", flags),
		zap.Uint32("mode", mode))

	var perm = MountModeRead
	if flags&os.O_WRONLY > 0 {
		perm = MountModeWrite
	} else if flags&O_RDWR > 0 {
		perm |= MountModeWrite
	}

	if flags&(os.O_CREATE|os.O_TRUNC) > 0 {
		perm |= MountModeWrite
	}

	if mount.checkMount(path, perm, log) != nil {
		return InvalidHandle, E_POSIX_EACCES
	}

	if mode&S_IFMT != S_IFLNK {
		mode = mode&SysPerm | S_IFREG
	}

	var OpenFlag = OpenFlagOpen
	if flags&os.O_CREATE > 0 {
		OpenFlag = OpenFlagCreate
	}

	resp, err := mount.proxyClient.Open(path, OpenFlag, mode, log)
	if err != nil || resp.Code != 0 {
		log.Error("Invoke Open fail",
			zap.String("Path", path),
			zap.Uint32("OpenFlag", OpenFlag),
			zap.Uint32("Mode", mode),
			zap.Int("code", resp.Code),
			zap.String("Message", resp.Message),
			zap.Any("err", err))
		return InvalidHandle, toErr(err, resp.Code)
	}
	log.Debug("Open Success", zap.Any("resp", resp))

	// assign file info.
	fileEntry := &FileEntry{
		RWMutex:   sync.RWMutex{},
		Id:        resp.Id,
		BlockSize: DefaultBlockSize,
		Handle:    -1,
		FilePath:  path,
		OpenFlag:  flags,
		Mode:      mode,
	}

	// storage fd entry.
	var fd = InvalidHandle
	err = mount.fileCache.NewFile(&fd, fileEntry)
	if err != nil {
		log.Error("Open fail, file too many",
			zap.String("Path", path),
			zap.Uint64("Did", resp.Id))
		return InvalidHandle, E_POSIX_EBADF
	}

	fdEntry, errFile := mount.fileCache.GetFileEntry(uint32(fd))
	if errFile != nil {
		log.Panic("get file err", zap.Error(errFile))
	}

	log.Info("Open Success",
		zap.Any("FileEntry", *fdEntry))

	// clean cache.
	mount.dentryCache.Remove(path[:strings.LastIndex(path, "/")])
	return fd, nil
}

func (mount *CfaMountInfo) Close(fd int) error {
	log := util.CloneLogger(mount.log)
	log.Debug(" CfaMountInfo Close run", zap.Int("fd", fd))
	fdEntry, err := mount.fileCache.GetFileEntry(uint32(fd))
	if err != nil {
		return E_POSIX_EBADF
	}

	log.Debug("file close", zap.Int("fd", fd),
		zap.Uint64("Id", fdEntry.Id),
		zap.String("path", fdEntry.FilePath))

	bgTime := mount.st.BeginStat()

	// flush cache.
	if mount.UseRWCache(fdEntry) {
		if mount.fileCache.IsLastRefer(fdEntry) {
			err := mount.rwCache.Close(fdEntry)
			if err != nil {
				log.Error("rwCache Close fail",
					zap.Int32("fd", int32(fd)))
				return E_POSIX_EBADF
			}
		} else {
			err := mount.rwCache.Flush(fdEntry)
			if err != nil {
				log.Error("rwCache Flush fail",
					zap.Int32("fd", int32(fd)))
				return E_POSIX_EBADF
			}
		}
	}

	err = mount.fileCache.CloseFile(uint32(fd))
	if err != nil {
		log.Error("Close fail, fd not exit",
			zap.Int32("fd", int32(fd)))
		return E_POSIX_EBADF
	}

	mount.log.Debug("close file success", zap.Any("entry", *fdEntry))
	mount.st.EndStat("Close", err, bgTime, 1)
	return nil
}

func (mount *CfaMountInfo) Read(fd int, buffer []byte, offset uint64) (int, error) {
	mount.log.Debug("CfaMountInfo Read run", zap.Int("fd", fd), zap.Uint64("offset", offset))
	// 512MB
	if len(buffer) > 512*1024*1024 {
		return 0, E_POSIX_EIO
	}

	fdEntry, err := mount.fileCache.GetFileEntry(uint32(fd))
	if err != nil {
		return 0, ToErrno(err)
	}

	if fdEntry.OpenFlag&O_WRONLY == O_WRONLY {
		return 0, E_POSIX_EPERM
	}

	var rdSize int
	var rdErr error

	bgTime := mount.st.BeginStat()

	if mount.UseRWCache(fdEntry) {
		rdSize, rdErr = mount.rwCache.Read(fdEntry, buffer, offset)
	} else {
		reader := NewReader(mount, fdEntry)
		rdSize, rdErr = reader.Read(fdEntry.Id, buffer, offset)
	}

	mount.st.EndStat("Read", rdErr, bgTime, 1)
	mount.st.StatBandWidth("Read", uint32(rdSize))

	if rdErr != nil {
		rdErr = ToErrno(rdErr)
	}

	return rdSize, rdErr
}

func (mount *CfaMountInfo) Write(fd int, buffer []byte, offset uint64) (int, error) {
	mount.log.Debug("CfaMountInfo Write begin",
		zap.Int("fd", fd),
		zap.Int("offset", int(offset)),
		zap.Int("size", len(buffer)))

	// 512MB
	if len(buffer) > 512*1024*1024 {
		return 0, E_POSIX_EIO
	}

	fdEntry, err := mount.fileCache.GetFileEntry(uint32(fd))
	if err != nil {
		return 0, E_POSIX_EBADF
	}

	if fdEntry.OpenFlag&(O_RDWR|O_WRONLY) == 0 {
		return InvalidHandle, E_POSIX_EPERM
	}

	var wtSize int
	var wtErr error

	bgTime := mount.st.BeginStat()

	if mount.UseRWCache(fdEntry) {
		wtSize, wtErr = mount.rwCache.Write(fdEntry, buffer, offset)
	} else {
		writer := NewWriter(mount, fdEntry)
		wtSize, wtErr = writer.Write(fdEntry.Id, buffer, offset)
	}

	mount.st.EndStat("Write", wtErr, bgTime, 1)
	mount.st.StatBandWidth("Write", uint32(wtSize))

	// clean cache.
	mount.dentryCache.Remove(fdEntry.FilePath)

	if wtErr != nil {
		wtErr = ToErrno(wtErr)
	}

	mount.log.Debug("CfaMountInfo Write done",
		zap.Int("fd", fd),
		zap.Int("offset", int(offset)),
		zap.Int("size", len(buffer)))
	return wtSize, wtErr
}

func (mount *CfaMountInfo) Utime(path string, utime uint64, atime uint64) error {
	log := util.CloneLogger(mount.log)

	if mount.checkMount(path, MountModeWrite, log) != nil {
		return E_POSIX_EACCES
	}

	resp, err := mount.proxyClient.Utime(path, utime, atime, log)

	if err != nil || resp.Code != 0 {
		log.Error("Invoke Utime fail",
			zap.String("Path", path),
			zap.Int("code", resp.Code),
			zap.String("Message", resp.Message),
			zap.Any("err", err))
		return toErr(err, resp.Code)
	}

	// clean cache.
	mount.dentryCache.Remove(path)

	return nil
}

func (mount *CfaMountInfo) Truncate(path string, length uint64) error {
	log := util.CloneLogger(mount.log)

	if mount.checkMount(path, MountModeWrite, log) != nil {
		return E_POSIX_EACCES
	}

	log.Debug("Truncate", zap.String("path", path),
		zap.Uint64("len", length))

	// before truncate, flush first.
	if mount.rwCache != nil {
		dEntry, err := mount.doStat(path, log)
		if err != nil {
			return err
		}

		fe := mount.GetSimpleFileEntry(dEntry)
		if fe != nil {
			log.Debug("Cache::Truncate",
				zap.Uint64("id", fe.Id),
				zap.Uint64("len", length))

			// must flush data to disk.
			err = mount.rwCache.Flush(fe)
			if err != nil {
				return err
			}

			// clean read cache.
			err = mount.rwCache.Release(fe)
			if err != nil {
				return err
			}
		}
	}

	resp, err := mount.proxyClient.Truncate(path, length, log)

	if err != nil || resp.Code != 0 {
		log.Error("Invoke Truncate fail",
			zap.String("Path", path),
			zap.Uint64("size", length),
			zap.Int("code", resp.Code),
			zap.String("Message", resp.Message),
			zap.Any("err", err))
		return toErr(err, resp.Code)
	}

	// clean cache.
	mount.dentryCache.Remove(path)

	return nil
}

func (mount *CfaMountInfo) Flush(fd int) error {
	log := util.CloneLogger(mount.log)
	fdEntry, err := mount.fileCache.GetFileEntry(uint32(fd))
	if err != nil {
		return E_POSIX_EBADF
	}

	if mount.UseRWCache(fdEntry) {
		bgTime := mount.st.BeginStat()
		err = mount.rwCache.Flush(fdEntry)
		mount.st.EndStat("Flush", err, bgTime, 1)
	}

	resp, err := mount.proxyClient.Fsync(fdEntry.Id, log)
	if err != nil || resp.Code != 0 {
		log.Error("Invoke Flush fail",
			zap.Uint64("id", fdEntry.Id),
			zap.Int("code", resp.Code),
			zap.String("Message", resp.Message),
			zap.Any("err", err))
		return toErr(err, resp.Code)
	}
	return nil
}

func (mount *CfaMountInfo) Fsync(fd int, datasync int) error {
	return mount.Flush(fd)
}

func (mount *CfaMountInfo) Ftruncate(fd int, size uint64) error {
	fdEntry, err := mount.fileCache.GetFileEntry(uint32(fd))
	if err != nil {
		return E_POSIX_EBADF
	}

	if fdEntry.OpenFlag&(O_RDWR|O_WRONLY) == 0 {
		return E_POSIX_EPERM
	}

	return mount.Truncate(fdEntry.FilePath, size)
}

func (mount *CfaMountInfo) UseRWCache(fdEntry *FileEntry) bool {
	return !fdEntry.DirectIO && mount.rwCache != nil
}

func (mount *CfaMountInfo) GetSimpleFileEntry(dentry *Dentry) *FileEntry {

	if DentryType(dentry.Type) != DentryTypeFile {
		return nil
	}

	fdEntry := &FileEntry{
		Id:        dentry.Inode,
		BlockSize: DefaultBlockSize,
	}

	return fdEntry
}
