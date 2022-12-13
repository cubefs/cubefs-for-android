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
	"github.com/cubefs/cubefs-for-android/lib/libfs/api"
	"github.com/cubefs/cubefs-for-android/lib/libfs/util"
	. "github.com/cubefs/cubefs-for-android/lib/libfs/util"
	zap "github.com/cubefs/cubefs-for-android/lib/log"
	. "github.com/cubefs/cubefs-for-android/lib/proto"
)

type Reader struct {
	proxyClient *api.ProxyClient
	log         *ClientLogger
}

func NewReader(mount *CfaMountInfo, entry *FileEntry) *Reader {
	return &Reader{
		proxyClient: mount.proxyClient,
		log:         util.CloneLogger(mount.log),
	}
}

func (reader *Reader) Read(id uint64, buffer []byte, offset uint64) (int, error) {
	size := uint32(len(buffer))
	reader.log.Info("reader::read",
		zap.Uint64("id", id),
		zap.Uint64("offset", offset),
		zap.Uint32("size", size))

	resp, err := reader.proxyClient.Read(id, offset, uint64(size), reader.log)
	if err != nil {
		return 0, ToErrno(err)
	}

	if resp.Code != 0 {
		reader.log.Error("Invoke Read fail",
			zap.Uint64("id", id),
			zap.Uint64("offset", offset),
			zap.Uint32("size", size),
			zap.Int("code", resp.Code),
			zap.String("Message", resp.Message),
			zap.Any("err", err))
		return 0, toErr(err, resp.Code)
	}

	// copy data
	copy(buffer, resp.Data)

	reader.log.Info("reader::read done",
		zap.Uint64("id", id),
		zap.Uint64("offset", offset),
		zap.Uint32("size", size),
		zap.Int("rspSize", len(resp.Data)))

	return len(resp.Data), nil
}
