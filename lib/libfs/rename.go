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
	"github.com/cubefs/cubefs-for-android/lib/libfs/util"
	zap "github.com/cubefs/cubefs-for-android/lib/log"
	. "github.com/cubefs/cubefs-for-android/lib/proto"
)

// the rename op
func (mount *CfaMountInfo) Rename(srcpath string, dstpath string) error {
	log := util.CloneLogger(mount.log)
	if mount.checkMount(srcpath, MountModeWrite, log) != nil {
		return E_POSIX_EACCES
	}

	resp, err := mount.proxyClient.Rename(srcpath, dstpath, log)
	if err != nil {
		return ToErrno(err)
	}

	if resp.Code != 0 {
		log.Error("Invoke Rename fail",
			zap.String("Volume", mount.Volume),
			zap.String("srcPath", srcpath),
			zap.String("dstpath", dstpath),
			zap.Int("code", resp.Code),
			zap.String("Message", resp.Message),
			zap.Any("err", err))
		return toErr(err, resp.Code)
	}

	// clean cache.
	mount.dentryCache.RemovePrefix(srcpath)
	mount.dentryCache.RemovePrefix(dstpath)

	return nil
}
