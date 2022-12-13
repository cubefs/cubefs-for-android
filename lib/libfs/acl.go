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

// change permission
func (mount *CfaMountInfo) Fchmod(fd int, mode uint32) error {
	log := CloneLogger(mount.log)

	fdEntry, err := mount.fileCache.GetFileEntry(uint32(fd))
	if err != nil {
		return E_POSIX_EBADF
	}

	if mount.checkMount(fdEntry.FilePath, MountModeWrite, log) != nil {
		return E_POSIX_EACCES
	}

	resp, err := mount.proxyClient.Chmod(fdEntry.FilePath, mode, log)

	if err != nil || resp.Code != 0 {
		log.Error("Invoke fchmod fail,",
			zap.String("Volume", mount.Volume),
			zap.String("path", fdEntry.FilePath),
			zap.Int("mode", int(mode)),
			zap.Any("code", resp.Code),
			zap.String("Message", resp.Message),
			zap.Any("err", err))
		return toErr(err, resp.Code)
	}

	// clean cache.
	mount.dentryCache.Remove(fdEntry.FilePath)

	return nil
}

// change permission
func (mount *CfaMountInfo) Chmod(path string, mode uint32) error {
	log := CloneLogger(mount.log)

	mode = mode & 0777
	if mount.checkMount(path, MountModeWrite, log) != nil {
		return E_POSIX_EACCES
	}

	resp, err := mount.proxyClient.Chmod(path, mode, log)

	if err != nil || resp.Code != 0 {
		log.Error("Invoke Chmod fail,",
			zap.String("Volume", mount.Volume),
			zap.String("path", path),
			zap.Int("mode", int(mode)),
			zap.Any("code", resp.Code),
			zap.String("Message", resp.Message),
			zap.Any("err", err))
		return toErr(err, resp.Code)
	}

	// clean cache.
	mount.dentryCache.Remove(path)

	return nil
}

func (mount *CfaMountInfo) Chown(path string, uid, gid uint32) error {
	log := CloneLogger(mount.log)

	if mount.checkMount(path, MountModeWrite, log) != nil {
		return E_POSIX_EACCES
	}

	resp, err := mount.proxyClient.Chown(path, uid, gid, log)

	if err != nil || resp.Code != 0 {
		log.Error("Invoke Chown fail,",
			zap.String("Volume", mount.Volume),
			zap.String("path", path),
			zap.Any("code", resp.Code),
			zap.String("Message", resp.Message),
			zap.Any("err", err))
		return toErr(err, resp.Code)
	}

	// clean cache.
	mount.dentryCache.Remove(path)

	return nil
}

// change owner, group
func (mount *CfaMountInfo) ChownEx(path string, owner, group string) error {
	log := CloneLogger(mount.log)

	if mount.checkMount(path, MountModeWrite, log) != nil {
		return E_POSIX_EACCES
	}

	resp, err := mount.proxyClient.ChownEx(path, owner, group, log)

	if err != nil || resp.Code != 0 {
		log.Error("Invoke ChownEx fail,",
			zap.String("Volume", mount.Volume),
			zap.String("path", path),
			zap.Any("code", resp.Code),
			zap.String("Message", resp.Message),
			zap.Any("err", err))
		return toErr(err, resp.Code)
	}

	// clean cache.
	mount.dentryCache.Remove(path)

	return nil
}
