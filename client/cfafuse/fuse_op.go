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
	"bufio"
	"fmt"
	"io"
	"math"
	"os"
	"strings"
	"time"

	"github.com/cubefs/cubefs-for-android/client/conf"
	fuselib "github.com/cubefs/cubefs-for-android/client/lib"
	"github.com/cubefs/cubefs-for-android/lib/libfs"
	. "github.com/cubefs/cubefs-for-android/lib/proto"

	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/hanwen/go-fuse/v2/fuse/nodefs"
	"github.com/hanwen/go-fuse/v2/fuse/pathfs"
)

type cfaFS struct {
	pathfs.FileSystem
	options   *conf.FuseConf
	mountInfo *libfs.CfaMountInfo
}

func parseErr(err error) fuse.Status {
	if err == nil {
		return fuse.OK
	}

	sErr, ok := err.(Errno)
	if !ok {
		return fuse.EIO
	}

	return fuselib.ToFuseStatus(sErr)
}

func NewCfaFS(options *conf.FuseConf, mountInfo *libfs.CfaMountInfo) (pathfs.FileSystem, error) {
	g := &cfaFS{
		options:    options,
		FileSystem: pathfs.NewDefaultFileSystem(),
		mountInfo:  mountInfo,
	}
	return g, nil
}

func genPath(root, name string) string {
	if name == "" {
		return root
	}

	if strings.HasSuffix(root, "/") {
		root = root[:len(root)-1]
	}

	return fmt.Sprintf("%s/%s", root, name)
}

func removeRoot(root, path string) string {
	if path == root {
		return ""
	}

	if !strings.HasSuffix(root, "/") {
		root = root + "/"
	}

	return strings.TrimPrefix(path, root)
}

// cfaFS
func (fs *cfaFS) GetAttr(name string, context *fuse.Context) (*fuse.Attr, fuse.Status) {
	path := genPath(fs.options.GetPath(), name)

	dEntry, err := fs.mountInfo.Stat(path)
	if err != nil {
		return nil, parseErr(err)
	}

	attr := &fuse.Attr{}
	setFuseAttr(attr, dEntry)
	return attr, fuse.OK
}

func (fs *cfaFS) GetXAttr(name string, attr string, context *fuse.Context) ([]byte, fuse.Status) {
	path := genPath(fs.options.GetPath(), name)

	data, err := fs.mountInfo.Getxattr(path, attr)
	if err != nil {
		return nil, parseErr(err)
	}
	return data, fuse.OK
}

func (fs *cfaFS) SetXAttr(name string, attr string, data []byte, flags int, context *fuse.Context) fuse.Status {
	path := genPath(fs.options.GetPath(), name)

	err := fs.mountInfo.Setxattr(path, attr, data, SetXattrFlag(flags))
	if err != nil {
		return parseErr(err)
	}

	return fuse.OK
}

func (fs *cfaFS) ListXAttr(name string, context *fuse.Context) ([]string, fuse.Status) {
	path := genPath(fs.options.GetPath(), name)

	resp, err := fs.mountInfo.Listxattr(path)
	if err != nil {
		return nil, parseErr(err)
	}

	return resp, fuse.OK
}

func (fs *cfaFS) RemoveXAttr(name string, attr string, context *fuse.Context) fuse.Status {
	path := genPath(fs.options.GetPath(), name)

	err := fs.mountInfo.Removexattr(path, attr)
	if err != nil {
		return parseErr(err)
	}
	return fuse.OK
}

func (fs *cfaFS) Readlink(name string, context *fuse.Context) (string, fuse.Status) {
	path := genPath(fs.options.GetPath(), name)

	data, err := fs.mountInfo.Readlink(path)
	if err != nil {
		return "", parseErr(err)
	}

	// 去除root
	data = removeRoot(fs.options.GetPath(), data)

	return data, fuse.OK
}

func (fs *cfaFS) Mknod(name string, mode uint32, dev uint32, context *fuse.Context) fuse.Status {
	path := genPath(fs.options.GetPath(), name)

	err := fs.mountInfo.Mknod(path, mode, int(dev))
	if err != nil {
		return parseErr(err)
	}

	return fuse.OK
}

func (fs *cfaFS) Mkdir(name string, mode uint32, context *fuse.Context) fuse.Status {
	path := genPath(fs.options.GetPath(), name)

	err := fs.mountInfo.Mkdir(path, mode)
	if err != nil {
		return parseErr(err)
	}

	return fuse.OK
}

func (fs *cfaFS) Unlink(name string, context *fuse.Context) (code fuse.Status) {
	path := genPath(fs.options.GetPath(), name)

	err := fs.mountInfo.UnLink(path)
	if err != nil {
		return parseErr(err)
	}

	return fuse.OK
}

func (fs *cfaFS) Rmdir(name string, context *fuse.Context) (code fuse.Status) {
	path := genPath(fs.options.GetPath(), name)

	err := fs.mountInfo.Rmdir(path)
	if err != nil {
		return parseErr(err)
	}
	return fuse.OK
}

// linkname: the new link file name, value: the file to be linked to
func (fs *cfaFS) Symlink(value string, linkName string, context *fuse.Context) (code fuse.Status) {
	path := genPath(fs.options.GetPath(), value)
	linkPath := genPath(fs.options.GetPath(), linkName)

	err := fs.mountInfo.Symlink(path, linkPath)
	if err != nil {
		return parseErr(err)
	}

	return fuse.OK
}

func (fs *cfaFS) Rename(oldName string, newName string, context *fuse.Context) (code fuse.Status) {
	path := genPath(fs.options.GetPath(), oldName)
	newPath := genPath(fs.options.GetPath(), newName)

	err := fs.mountInfo.Rename(path, newPath)
	if err != nil {
		return parseErr(err)
	}

	return fuse.OK
}

func (fs *cfaFS) Link(oldName string, newName string, context *fuse.Context) (code fuse.Status) {
	path := genPath(fs.options.GetPath(), oldName)
	newPath := genPath(fs.options.GetPath(), newName)

	err := fs.mountInfo.Link(path, newPath)
	if err != nil {
		return parseErr(err)
	}

	return fuse.OK
}

func (fs *cfaFS) Chmod(name string, mode uint32, context *fuse.Context) (code fuse.Status) {
	path := genPath(fs.options.GetPath(), name)

	err := fs.mountInfo.Chmod(path, mode)
	if err != nil {
		return parseErr(err)
	}

	return fuse.OK
}

func (fs *cfaFS) Chown(name string, uid uint32, gid uint32, context *fuse.Context) (code fuse.Status) {
	path := genPath(fs.options.GetPath(), name)

	err := fs.mountInfo.Chown(path, uid, gid)
	if err != nil {
		return parseErr(err)
	}

	return fuse.OK
}

func getGroups(pid uint32) ([]string, fuse.Status) {
	path := fmt.Sprintf("/proc/%d/task/%d/status", pid, pid)
	file, err := os.Open(path)
	if err != nil {
		fmt.Printf("get groups fail, %s, err %v \n", path, err)
		return nil, fuse.ENOENT
	}

	defer file.Close()

	br := bufio.NewReader(file)
	for {
		s, _, c := br.ReadLine()
		if c == io.EOF {
			break
		}

		line := string(s)
		if strings.HasPrefix(line, "Groups:") && len(line) > 8 {
			fmt.Printf("group %s\n", line)
			return strings.Split(strings.TrimSpace(line[8:]), " "), fuse.OK
		}
	}

	return nil, fuse.EINVAL
}

func (fs *cfaFS) Truncate(name string, offset uint64, context *fuse.Context) (code fuse.Status) {
	path := genPath(fs.options.GetPath(), name)

	err := fs.mountInfo.Truncate(path, offset)
	if err != nil {
		return parseErr(err)
	}

	return fuse.OK
}

func (fs *cfaFS) Open(name string, flags uint32, context *fuse.Context) (file nodefs.File, code fuse.Status) {
	path := genPath(fs.options.GetPath(), name)

	//f := toOpenFlag(flags)
	handle, err := fs.mountInfo.Open(path, int(flags), SysPerm)
	if err != nil {
		return nil, parseErr(err)
	}

	node, err := fs.mountInfo.Stat(path)
	if err != nil {
		return nil, parseErr(err)
	}

	file = NewFile(handle, path, fs.mountInfo, node)
	return file, fuse.OK
}
func (fs *cfaFS) OpenDir(name string, context *fuse.Context) (stream []fuse.DirEntry, status fuse.Status) {
	path := genPath(fs.options.GetPath(), name)

	dir, err := fs.mountInfo.Opendir(path)
	if err != nil {
		return nil, parseErr(err)
	}

	defer dir.CloseDir()

	count := 10000
	if fs.options.ReadDirCount > 0 {
		count = fs.options.ReadDirCount
	}

	for true {
		entryArr, err := dir.Readdir(uint(count))
		if err != nil {
			return nil, parseErr(err)
		}

		for _, e := range entryArr {
			dirEnt := fuse.DirEntry{
				Mode: e.Info.Mode,
				Name: e.Name,
				Ino:  e.Inode,
			}
			stream = append(stream, dirEnt)
		}

		if len(entryArr) < count {
			return stream, fuse.OK
		}
	}

	return stream, fuse.OK
}

// Called after mount.
func (fs *cfaFS) OnMount(nodeFs *pathfs.PathNodeFs) {
}

// Called after unmount.
func (fs *cfaFS) OnUnmount() {
}

func (fs *cfaFS) Access(name string, mode uint32, context *fuse.Context) (code fuse.Status) {
	return fuse.OK
}

func (fs *cfaFS) Create(name string, flags uint32, mode uint32, context *fuse.Context) (file nodefs.File, code fuse.Status) {
	path := genPath(fs.options.GetPath(), name)

	fd, err := fs.mountInfo.Open(path, int(flags), mode)
	if err != nil {
		return nil, parseErr(err)
	}

	node, err := fs.mountInfo.Stat(path)
	if err != nil {
		return nil, parseErr(err)
	}

	file = NewFile(fd, path, fs.mountInfo, node)

	return file, fuse.OK
}

func (fs *cfaFS) Utimens(name string, Atime *time.Time, Mtime *time.Time, context *fuse.Context) (code fuse.Status) {
	path := genPath(fs.options.GetPath(), name)

	err := fs.mountInfo.Utime(path, uint64(Mtime.Unix()), uint64(Atime.Unix()))
	if err != nil {
		return parseErr(err)
	}

	return fuse.OK
}

func (fs *cfaFS) String() string {
	return "cfaFS"
}

func (fs *cfaFS) StatFs(name string) *fuse.StatfsOut {
	stats := libfs.Statfs{}
	path := genPath(fs.options.GetPath(), name)

	err := fs.mountInfo.Statfs(path, &stats)
	if err != nil {
		return nil
	}

	//TODO: currently proxy has no api to query total size, modify here if later proxy provides such api
	total := uint64(1024 * 1024 * 1024 * 1024)
	blkSize := uint64(stats.BlockSize)
	free := (total - stats.Fbytes - stats.Rbytes) / blkSize

	out := &fuse.StatfsOut{
		Blocks:  total / blkSize,
		Bfree:   free,
		Bavail:  free,
		Files:   stats.Rfiles + uint64(stats.Files),
		Ffree:   math.MaxInt64 - stats.Rfiles - uint64(stats.Files),
		Bsize:   uint32(blkSize),
		NameLen: 256,
		Frsize:  4096,
		Padding: 0,
		Spare:   [6]uint32{},
	}

	return out
}
