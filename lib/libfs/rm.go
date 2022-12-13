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

// remove an empty directory
func (mount *CfaMountInfo) Rmdir(path string) error {
	log := CloneLogger(mount.log)

	if mount.checkMount(path, MountModeDel, log) != nil {
		return E_POSIX_EACCES
	}

	resp, err := mount.proxyClient.RmDir(path, log)

	if err != nil || resp.Code != 0 {
		log.Error("invoke Rmdir fail,",
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

// force remove a dir and it's subdirs
func (mount *CfaMountInfo) RmDirTree(path string) error {
	log := CloneLogger(mount.log)

	if mount.checkMount(path, MountModeDel, log) != nil {
		return E_POSIX_EACCES
	}

	resp, err := mount.proxyClient.RmDirTree(path, log)

	if err != nil || resp.Code != 0 {
		log.Error("invoke RmDirTree dir fail,",
			zap.String("Volume", mount.Volume),
			zap.String("path", path),
			zap.Any("code", resp.Code),
			zap.String("Message", resp.Message),
			zap.Any("err", err))
		return toErr(err, resp.Code)
	}

	// clean cache.
	mount.dentryCache.RemovePrefix(path)

	return nil
}

// delete a file
func (mount *CfaMountInfo) UnLink(path string) error {
	log := CloneLogger(mount.log)

	if mount.checkMount(path, MountModeDel, log) != nil {
		return E_POSIX_EACCES
	}

	log.Debug("unlink req", zap.String(path, path))

	resp, err := mount.proxyClient.Unlink(path, log)

	if err != nil || resp.Code != 0 {
		log.Error("invoke Unlink fail,",
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
