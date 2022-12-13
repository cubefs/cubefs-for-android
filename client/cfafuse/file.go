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

package cfafuse

import (
	"io/fs"
	"os"
	"sync"
	"syscall"
	"time"

	"github.com/cubefs/cubefs-for-android/lib/libfs"
	. "github.com/cubefs/cubefs-for-android/lib/proto"

	"golang.org/x/sys/unix"

	"github.com/hanwen/go-fuse/v2/fuse"
	. "github.com/hanwen/go-fuse/v2/fuse/nodefs"
)

type file struct {
	node *Dentry
	sync.RWMutex
	mountInfo *libfs.CfaMountInfo
	fd        int
	path      string
}

func NewFile(fd int, path string, mount *libfs.CfaMountInfo, node *Dentry) File {
	return &file{
		node:      node,
		RWMutex:   sync.RWMutex{},
		mountInfo: mount,
		fd:        fd,
		path:      path,
	}
}

func (f *file) SetInode(*Inode) {
}

func (f *file) InnerFile() File {
	return nil
}

func (f *file) String() string {
	return "file"
}

func (f *file) Read(buf []byte, off int64) (fuse.ReadResult, fuse.Status) {
	size, err := f.mountInfo.Read(f.fd, buf, uint64(off))
	if err != nil {
		return nil, parseErr(err)
	}

	result := fuse.ReadResultData(buf[0:size])
	return result, fuse.OK
}

func (f *file) Write(data []byte, off int64) (uint32, fuse.Status) {
	size, err := f.mountInfo.Write(f.fd, data, uint64(off))
	if err != nil {
		return 0, parseErr(err)
	}
	return uint32(size), fuse.OK
}

func (f *file) GetLk(owner uint64, lk *fuse.FileLock, flags uint32, out *fuse.FileLock) (code fuse.Status) {
	return fuse.OK
}

func (f *file) SetLk(owner uint64, lk *fuse.FileLock, flags uint32) (code fuse.Status) {
	return fuse.OK
}

func (f *file) SetLkw(owner uint64, lk *fuse.FileLock, flags uint32) (code fuse.Status) {
	return fuse.OK
}

func (f *file) Flush() fuse.Status {
	err := f.mountInfo.Flush(f.fd)
	if err != nil {
		return parseErr(err)
	}

	return fuse.OK
}

func formAclFromNode(node *Dentry) *Acl {
	return &Acl{
		UseFlag: AclUseUidGid,
		Uid:     int(node.Info.Uid),
		Gid:     int(node.Info.Gid),
		Owner:   "",
		Group:   "",
	}
}

func (f *file) Release() {
	err := f.mountInfo.Close(f.fd)
	if err != nil {

	}
}

func (f *file) GetAttr(attr *fuse.Attr) fuse.Status {
	den, err := f.mountInfo.Fstat(f.fd)
	if err != nil {
		return parseErr(err)
	}

	setFuseAttr(attr, den)
	return fuse.OK
}

func osModeToSyscall(m uint32) uint32 {
	mode := fs.FileMode(m)
	typ := syscall.S_IFREG
	switch mode.Type() {
	case os.ModeDir:
		typ = syscall.S_IFDIR
	case os.ModeSymlink:
		typ = syscall.S_IFLNK
	default:
		typ = syscall.S_IFREG
	}

	typ |= int(mode.Perm())

	return uint32(typ)
}

func setFuseAttr(attr *fuse.Attr, node *Dentry) {
	attr.Size = node.Info.Size
	mode := node.Info.Mode
	attr.Mode = osModeToSyscall(mode)
	attr.Ino = node.Info.Inode
	attr.Size = node.Info.Size

	attr.Mtime = node.Info.MTime
	attr.Atime = node.Info.ATime
	attr.Ctime = node.Info.CTime

	attr.Mtimensec = 0
	attr.Atimensec = 0
	attr.Ctimensec = 0

	attr.Nlink = uint32(node.Info.Nlink)
	attr.Owner = fuse.Owner{
		Uid: node.Info.Uid,
		Gid: node.Info.Gid,
	}

	attr.Blocks = 0
	attr.Blksize = 0
	attr.Rdev = 0
}

func (f *file) Fsync(flags int) (code fuse.Status) {
	err := f.mountInfo.Fsync(f.fd, flags)
	if err != nil {
		return parseErr(err)
	}

	return fuse.OK
}

func (f *file) Utimens(atime *time.Time, mtime *time.Time) fuse.Status {
	err := f.mountInfo.Utime(f.path, uint64(mtime.Unix()), uint64(atime.Unix()))
	if err != nil {
		return parseErr(err)
	}

	return fuse.OK
}

func (f *file) Truncate(size uint64) fuse.Status {
	err := f.mountInfo.Truncate(f.path, size)
	if err != nil {
		return parseErr(err)
	}
	return fuse.OK
}

func (f *file) Chown(uid uint32, gid uint32) fuse.Status {
	err := f.mountInfo.Chown(f.path, uid, gid)
	if err != nil {
		return parseErr(err)
	}

	return fuse.OK
}

func (f *file) Chmod(perms uint32) fuse.Status {
	err := f.mountInfo.Chmod(f.path, perms)
	if err != nil {
		return parseErr(err)
	}

	return fuse.OK
}

func (f *file) Allocate(off uint64, size uint64, mode uint32) (code fuse.Status) {
	if mode == 0 {
		return f.Truncate(off + size)
	} else if mode == unix.FALLOC_FL_KEEP_SIZE {
		return fuse.OK
	} else {
		return fuse.ENOSYS
	}
}
