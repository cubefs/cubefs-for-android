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

package lib

import (
	"syscall"

	"github.com/cubefs/cubefs-for-android/lib/proto"

	"github.com/hanwen/go-fuse/v2/fuse"
)

func ToFuseStatus(err proto.Errno) fuse.Status {
	if !err.IsPosixErrno() {
		return fuse.EIO
	}

	sysErr := syscall.EIO

	switch err {
	case proto.E_POSIX_EACCES:
		sysErr = syscall.EACCES
	case proto.E_POSIX_EBADF:
		sysErr = syscall.EBADF
	case proto.E_POSIX_EBUSY:
		sysErr = syscall.EBUSY
	case proto.E_POSIX_EEXIST:
		sysErr = syscall.EEXIST
	case proto.E_POSIX_EFAULT:
		sysErr = syscall.EFAULT
	case proto.E_POSIX_EFBIG:
		sysErr = syscall.EFBIG
	case proto.E_POSIX_EINVAL:
		sysErr = syscall.EINVAL
	case proto.E_POSIX_EIO:
		sysErr = syscall.EIO
	case proto.E_POSIX_EISDIR:
		sysErr = syscall.EISDIR
	case proto.E_POSIX_ELOOP:
		sysErr = syscall.ELOOP
	case proto.E_POSIX_EMLINK:
		sysErr = syscall.EMLINK
	case proto.E_POSIX_ENAMETOOLONG:
		sysErr = syscall.ENAMETOOLONG
	case proto.E_POSIX_ENODATA:
		sysErr = syscall.ENODATA
	case proto.E_POSIX_ENOENT:
		sysErr = syscall.ENOENT
	case proto.E_POSIX_ENOTCONN:
		sysErr = syscall.ENOTCONN
	case proto.E_POSIX_ENOTDIR:
		sysErr = syscall.ENOTDIR
	case proto.E_POSIX_ENOTEMPTY:
		sysErr = syscall.ENOTEMPTY
	case proto.E_POSIX_ENOTSUP:
		sysErr = syscall.ENOTSUP
	case proto.E_POSIX_EPERM:
		sysErr = syscall.EPERM
	case proto.E_POSIX_ERANEG:
		sysErr = syscall.ERANGE
	case proto.E_POSIX_EAGAIN:
		sysErr = syscall.EAGAIN
	default:
		sysErr = syscall.EIO
	}

	return fuse.Status(sysErr)
}
