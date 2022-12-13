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

package proto

import (
	"github.com/pkg/errors"
)

// CubeFS proxy URL.
const UrlMkdir = "/api/v1/mkdir"
const UrlReaddir = "/api/v1/readdir"
const UrlReaddirEx = "/api/v1/readdirex"
const UrlRmdir = "/api/v1/rmdir"
const UrlRmdirtree = "/api/v1/rmdirtree"
const UrlOpen = "/api/v1/open"
const UrlRead = "/api/v1/read"
const UrlWrite = "/api/v1/write"
const UrlFsync = "/api/v1/fsync"
const UrlTruncate = "/api/v1/truncate"
const UrlUnlink = "/api/v1/unlink"
const UrlLink = "/api/v1/link"
const UrlSymLink = "/api/v1/symbol_link"
const UrlSetxattr = "/api/v1/setXattr"
const UrlGetxattr = "/api/v1/getXattr"
const UrlRemovexattr = "/api/v1/removeXattr"
const UrlListxattr = "/api/v1/listXattr"
const UrlSetattr = "/api/v1/setattr"
const UrlRename = "/api/v1/rename"
const UrlStat = "/api/v1/stat"
const UrlStatfs = "/api/v1/statDir"

// dentry type.
type DentryType uint8

func (d DentryType) String() string {
	switch d {
	case DentryTypeFile:
		return "regular"
	case DentryTypeDir:
		return "directory"
	case DentryTypeLink:
		return "link"
	case DentryTypeFifo:
		return "fifo"
	case DentryTypeSock:
		return "socket"
	default:
		return "unknown"
	}
}

const (
	DentryTypeFile DentryType = 1 // 文件
	DentryTypeDir  DentryType = 2 // 目录
	DentryTypeLink DentryType = 3 // 软链接
	DentryTypeFifo DentryType = 4 // FIFO
	DentryTypeSock DentryType = 5 // SOCK
)

// setxattr flag
type SetXattrFlag = uint32

const (
	SetXattrFlagCreate  SetXattrFlag = 1 // 创建
	SetXattrFlagReplace SetXattrFlag = 2 // 替换
)

// set acl flag
type SetAttrFlag uint32

const (
	SetAclFlagChangeMode  SetAttrFlag = 1 << 0 // 修改mode
	SetAclFlagChangeUid   SetAttrFlag = 1 << 1 // 修改uid
	SetAclFlagChangeGid   SetAttrFlag = 1 << 2 // 修改gid
	SetAclFlagChangeMTime SetAttrFlag = 1 << 3 // 修改mtime
	SetAclFlagChangeATime SetAttrFlag = 1 << 4 // 修改atime
	SetAclFlagChangeOwner SetAttrFlag = 1 << 5 // 修改Owner
	SetAclFlagChangeGroup SetAttrFlag = 1 << 6 // 修改Group
)

// mount mode
type MountMode uint32

const (
	MountModeRead  MountMode = 0x01 // mount 读
	MountModeWrite MountMode = 0x02 // mount 写
	MountModeDel   MountMode = 0x04 // mount 删
)

// ACL use flag
type AclUseFlag uint32

const (
	AclUseUidGid     AclUseFlag = 1 // ACL使用UID, GID
	AclUseOwnerGroup AclUseFlag = 2 // ACL 使用OWNER, GROUP
)

const InvalidUid = 4294967295
const InvalidGid = 4294967295

const DefaultBlockSize = 1 * 1024 * 1024

type Acl struct {
	UseFlag AclUseFlag
	Uid     int
	Gid     int
	Owner   string
	Group   string
	Groups  []string
}

type ProxyRequest struct {
	Path string `json:"path"`
}

type ProxyResponse struct {
	Code    int    `json:"code"`
	Message string `json:"msg"`
}

func (r ProxyResponse) GetError() error {
	if r.Code == SUCCESS {
		return nil
	} else {
		return errors.WithMessage(Errno(r.Code), r.Message)
	}
}

func (r ProxyResponse) GetCode() int {
	return r.Code
}

func (r ProxyResponse) Msg() string {
	return r.Message
}

type ProxyResp interface {
	GetCode() int
	Msg() string
}

// Mkdir
type MkdirRequest struct {
	ProxyRequest
	Mode uint32 `json:"mode"`
}

type MkdirResponse struct {
	ProxyResponse
}

type InoInfo struct {
	Inode  uint64 `json:"ino"`
	Mode   uint32 `json:"mode"`
	Nlink  uint32 `json:"nlink"`
	Size   uint64 `json:"sz"`
	Uid    uint32 `json:"uid"`
	Gid    uint32 `json:"gid"`
	MTime  uint64 `json:"mt"`
	CTime  uint64 `json:"ct"`
	ATime  uint64 `json:"at"`
	Target string `json:"target"`
	Owner  string `json:"owner"`
	Group  string `json:"group"`
}

// Readdir
type Dentry struct {
	Name  string   `json:"name"`
	Inode uint64   `json:"ino"`
	Type  uint32   `json:"type"`
	Info  *InoInfo `json:"info"`
}

type ReaddirRequest struct {
	ProxyRequest
}

type ReaddirResponse struct {
	ProxyResponse
	Dentrys []Dentry `json:"data"`
}

type ReaddirExRequest struct {
	ProxyRequest
	StartKey string `json:"from"`
	Count    uint16 `json:"limit"`
}

type ReaddirExResponse struct {
	ProxyResponse
	Dentrys []Dentry `json:"data"`
}

// Rmdir
type RmdirRequest struct {
	ProxyRequest
}

type RmdirResponse struct {
	ProxyResponse
}

// Rmdirtree
type RmdirTreeRequest struct {
	ProxyRequest
}

type RmdirTreeResponse struct {
	ProxyResponse
}

const (
	OpenFlagCreate uint32 = 1 // 创建文件
	OpenFlagOpen   uint32 = 2 // 打开文件
)

// Open
type OpenRequest struct {
	ProxyRequest
	Openflag uint32 `json:"openflag"`
	Mode     uint32 `json:"mode"`
}

type OpenResponse struct {
	ProxyResponse
	Id uint64 `json:"data"`
}

// Read
type ReadRequest struct {
	Id     uint64 `json:"id"`
	Offset uint64 `json:"off"`
	Length uint64 `json:"len"`
}

type ReadResponse struct {
	ProxyResponse
	Data []byte `json:"data"`
}

type WriteRequest struct {
	Id     uint64 `json:"id"`
	Offset uint64 `json:"off"`
	Length uint64 `json:"len"`
	Data   []byte `json:"data"`
}

type WriteResponse struct {
	ProxyResponse
}

// Truncate
type TruncateRequest struct {
	ProxyRequest
	Length uint64 `json:"offset"`
}

type TruncateResponse struct {
	ProxyResponse
}

// Unlink
type UnlinkRequest struct {
	ProxyRequest
}

type UnlinkResponse struct {
	ProxyResponse
}

// LinkId
type LinkRequest struct {
	ProxyRequest
	Dstpath string `json:"newpath"`
}

type LinkResponse struct {
	ProxyResponse
}

// Symlink
type SymlinkRequest struct {
	ProxyRequest
	Dstpath string `json:"newpath"`
}

type SymlinkResponse struct {
	ProxyResponse
}

// Setxattr
type SetxattrRequest struct {
	ProxyRequest
	Flag SetXattrFlag `json:"flag"`
	Name string       `json:"name"`
	Data []byte       `json:"data"`
}

type SetxattrResponse struct {
	ProxyResponse
}

// Getxattr
type GetxattrRequest struct {
	ProxyRequest
	Name string `json:"name"`
}

type GetxattrResponse struct {
	ProxyResponse
	Data []byte `json:"data"`
}

// Removexattr
type RemovexattrRequest struct {
	ProxyRequest
	Name string `json:"name"`
}

type RemovexattrResponse struct {
	ProxyResponse
}

type ListxattrRequest struct {
	ProxyRequest
}

type ListxattrResponse struct {
	ProxyResponse
	Names []string `json:"data"`
}

// SetAttr
type SetAttrRequest struct {
	ProxyRequest
	Flag  SetAttrFlag `json:"flag"`
	Uid   uint32      `json:"uid"`
	Gid   uint32      `json:"gid"`
	Mtime uint64      `json:"mtime"`
	Atime uint64      `json:"atime"`
	Mode  uint32      `json:"mode"`
	Owner string      `json:"owner"`
	Group string      `json:"group"`
}

type SetAttrResponse struct {
	ProxyResponse
}

// Rename
type RenameRequest struct {
	ProxyRequest
	Dstpath string `json:"destpath"`
}

type RenameResponse struct {
	ProxyResponse
}

// Stat
type StatRequest struct {
	ProxyRequest
}

type StatResponse struct {
	ProxyResponse
	Data InoInfo `json:"data"`
}

// Statfs
type StatfsRequest struct {
	ProxyRequest
}

type StatfsInfo struct {
	Files    uint32 `json:"files"`
	Folders  uint32 `json:"folders"`
	Fbytes   uint64 `json:"fbytes"`
	Fubytes  uint64 `json:"fubytes"`
	Rfiles   uint64 `json:"rfiles"`
	Rfolders uint64 `json:"rfolders"`
	Rbytes   uint64 `json:"rbytes"`
	Rubytes  uint64 `json:"rubytes"`
}

type StatfsResponse struct {
	ProxyResponse
	Data StatfsInfo `json:"data"`
}

type FsyncRequest struct {
	Id uint64 `json:"id"`
}

type FsyncResponse struct {
	ProxyResponse
}
