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

// reads a soft link file and returns the target file name
func (mount *CfaMountInfo) Readlink(path string) (string, error) {
	log := CloneLogger(mount.log)

	if mount.checkMount(path, MountModeRead, log) != nil {
		return "", E_POSIX_EACCES
	}

	resp, err := mount.proxyClient.Stat(path, log)

	if err != nil || resp.Code != 0 {
		log.Error("invoke Readlink fail,",
			zap.String("path", path),
			zap.Any("code", resp.Code),
			zap.String("Message", resp.Message),
			zap.Any("err", err))
		return "", toErr(err, resp.Code)
	}

	return resp.Data.Target, nil
}

// create hard link
func (mount *CfaMountInfo) Link(path, linkPath string) error {
	log := CloneLogger(mount.log)

	if mount.checkMount(path, MountModeWrite, log) != nil {
		return E_POSIX_EACCES
	}

	resp, err := mount.proxyClient.Link(path, linkPath, log)

	if err != nil || resp.Code != 0 {
		log.Error("Link fail,",
			zap.String("old path", path),
			zap.String("new path", linkPath),
			zap.Any("code", resp.Code),
			zap.String("Message", resp.Message),
			zap.Any("err", err))
		return toErr(err, resp.Code)
	}

	// clean cache.
	mount.dentryCache.Remove(path)

	return nil
}

// create soft link
func (mount *CfaMountInfo) Symlink(path, linkPath string) error {
	log := CloneLogger(mount.log)

	if mount.checkMount(path, MountModeWrite, log) != nil {
		return E_POSIX_EACCES
	}

	resp, err := mount.proxyClient.SymLink(path, linkPath, log)

	if err != nil || resp.Code != 0 {
		log.Error("invoke SymLink fail,",
			zap.String("path", path),
			zap.String("link path", linkPath),
			zap.Int("code", resp.Code),
			zap.String("Message", resp.Message),
			zap.Any("err", err))
		return toErr(err, resp.Code)
	}

	return nil
}
