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
	. "github.com/cubefs/cubefs-for-android/lib/proto"
)

func (mount *CfaMountInfo) Setxattr(path string, key string, value []byte, flag SetXattrFlag) error {
	log := CloneLogger(mount.log)

	if mount.checkMount(path, MountModeWrite, log) != nil {
		return E_POSIX_EACCES
	}

	resp, err := mount.proxyClient.SetXattr(path, flag, key, value, log)

	if err != nil || resp.Code != 0 {
		log.Error("SetXAttr.invoke setXAttr fail,",
			zap.String("fsName", mount.Volume),
			zap.String("path", path),
			zap.String("key", key),
			zap.String("value", string(value)),
			zap.Int("code", resp.Code),
			zap.String("Message", resp.Message),
			zap.Any("err", err))
		return toErr(err, resp.Code)
	}

	// clean cache.
	mount.dentryCache.Remove(path)

	return nil
}

func (mount *CfaMountInfo) Getxattr(path string, key string) ([]byte, error) {
	log := CloneLogger(mount.log)

	if mount.checkMount(path, MountModeRead, log) != nil {
		return nil, E_POSIX_EACCES
	}

	resp, err := mount.proxyClient.GetXattr(path, key, log)

	if err != nil || resp.Code != 0 {
		log.Warn("GetXAttr.invoke getXAttr fail,",
			zap.String("fsName", mount.Volume),
			zap.String("path", path),
			zap.String("key", key),
			zap.Int("code", resp.Code),
			zap.String("Message", resp.Message),
			zap.Any("err", err))
		return resp.Data, toErr(err, resp.Code)
	}

	return resp.Data, nil
}

func (mount *CfaMountInfo) Listxattr(path string) ([]string, error) {
	log := CloneLogger(mount.log)

	if mount.checkMount(path, MountModeRead, log) != nil {
		return nil, E_POSIX_EACCES
	}

	resp, err := mount.proxyClient.ListXAttr(path, log)

	if err != nil || resp.Code != 0 {
		log.Warn("ListXAttr.invoke ListXAttr fail,",
			zap.String("fsName", mount.Volume),
			zap.String("path", path),
			zap.Int("code", resp.Code),
			zap.String("Message", resp.Message),
			zap.Any("err", err))
		return nil, toErr(err, resp.Code)
	}

	log.Debug("CfaMountInfo Listxattr", zap.Any("names", resp.Names))
	return resp.Names, nil
}

func (mount *CfaMountInfo) Removexattr(path string, key string) error {
	log := CloneLogger(mount.log)

	if mount.checkMount(path, MountModeWrite, log) != nil {
		return E_POSIX_EACCES
	}

	resp, err := mount.proxyClient.RmXAttr(path, key, log)
	if err != nil || resp.Code != 0 {
		log.Error("Removexattr.invoke RmXAttr fail,",
			zap.String("fsName", mount.Volume),
			zap.String("path", path),
			zap.String("key", key),
			zap.Int("code", resp.Code),
			zap.String("Message", resp.Message),
			zap.Any("err", err))
		return toErr(err, resp.Code)
	}

	// clean cache.
	mount.dentryCache.Remove(path)

	return nil
}

func (mount *CfaMountInfo) Fsetxattr(fd int, key string, value []byte, flag int) error {
	fdEntry, err := mount.fileCache.GetFileEntry(uint32(fd))
	if err != nil {
		return E_POSIX_EBADF
	}

	if fdEntry.OpenFlag&(O_RDWR|O_WRONLY) == 0 {
		return E_POSIX_EPERM
	}

	return mount.Setxattr(fdEntry.FilePath, key, value, uint32(flag))
}

func (mount *CfaMountInfo) Fgetxattr(fd int, key string) ([]byte, error) {
	fdEntry, err := mount.fileCache.GetFileEntry(uint32(fd))
	if err != nil {
		return nil, E_POSIX_EBADF
	}

	if fdEntry.OpenFlag&O_WRONLY == O_WRONLY {
		return nil, E_POSIX_EPERM
	}

	return mount.Getxattr(fdEntry.FilePath, key)
}

func (mount *CfaMountInfo) Flistxattr(fd int) ([]string, error) {
	fdEntry, err := mount.fileCache.GetFileEntry(uint32(fd))
	if err != nil {
		return nil, E_POSIX_EBADF
	}

	if fdEntry.OpenFlag&O_WRONLY == O_WRONLY {
		return nil, E_POSIX_EPERM
	}

	return mount.Listxattr(fdEntry.FilePath)
}

func (mount *CfaMountInfo) Fremovexattr(fd int, key string) error {
	fdEntry, err := mount.fileCache.GetFileEntry(uint32(fd))
	if err != nil {
		return E_POSIX_EBADF
	}

	if fdEntry.OpenFlag&(O_RDWR|O_WRONLY) == 0 {
		return E_POSIX_EPERM
	}

	return mount.Removexattr(fdEntry.FilePath, key)
}
