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

package api

import (
	"github.com/cubefs/cubefs-for-android/lib/libfs/http"
	. "github.com/cubefs/cubefs-for-android/lib/libfs/util"
	. "github.com/cubefs/cubefs-for-android/lib/proto"
)

type ProxyApi interface {
	Close()
	GetIoClient() *http.HttpClient
	GetThirdPartyClient() *http.HttpClient
	Unlink(path string, log *ClientLogger) (UnlinkResponse, error)
	Link(path, dstpath string, log *ClientLogger) (LinkResponse, error)
	MkDir(path string, mode uint32, log *ClientLogger) (MkdirResponse, error)
	ReadDir(path, log *ClientLogger) (ReaddirResponse, error)
	ReadDirEx(path, startKey string, count uint16, log *ClientLogger) (ReaddirExResponse, error)
	RmDir(path string, log *ClientLogger) (RmdirResponse, error)
	RmDirTree(path string, log *ClientLogger) (RmdirTreeResponse, error)
	Open(path string, flag int, mode uint32, log *ClientLogger) (OpenResponse, error)
	Read(id, offset, length uint64, log *ClientLogger) (ReadResponse, error)
	Write(req *WriteRequest, log *ClientLogger) (WriteResponse, error)
	Truncate(path string, length uint64, log *ClientLogger) (TruncateResponse, error)
	SetXattr(path string, flag SetXattrFlag, name string, value []byte, log *ClientLogger) (SetxattrResponse, error)
	GetXattr(path, name string, log *ClientLogger) (GetxattrResponse, error)
	ListXAttr(path string, log *ClientLogger) (ListxattrResponse, error)
	RmXAttr(path, name string, log *ClientLogger) (RemovexattrResponse, error)
	SetAttr(req SetAttrRequest, log *ClientLogger) (SetAttrResponse, error)
	Rename(path, dstPath string, log *ClientLogger) (RenameResponse, error)
	Stat(path string, log *ClientLogger) (StatResponse, error)
	Fstat(id uint64, log *ClientLogger) (StatResponse, error)
	StatFs(path string, log *ClientLogger) (StatfsResponse, error)
	Fsync(id uint64, log *ClientLogger) (FsyncResponse, error)
}
