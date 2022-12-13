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
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/cubefs/cubefs-for-android/lib/libfs/conf"
	"github.com/cubefs/cubefs-for-android/lib/libfs/http"
	"github.com/cubefs/cubefs-for-android/lib/libfs/stat"
	. "github.com/cubefs/cubefs-for-android/lib/libfs/util"
	zap "github.com/cubefs/cubefs-for-android/lib/log"
	. "github.com/cubefs/cubefs-for-android/lib/proto"
)

type ProxyClient struct {
	IoCli    *http.HttpClient
	RetryCfg *conf.RetryCfg
	St       *stat.Statistic
	sync.Mutex
}

func NewProxyClient(io *http.HttpClient, retry *conf.RetryCfg, st *stat.Statistic) *ProxyClient {
	return &ProxyClient{
		IoCli:    io,
		RetryCfg: retry,
		St:       st,
	}
}

func (c *ProxyClient) GetIoClient() *http.HttpClient {
	return c.IoCli
}

func (c *ProxyClient) Close() {
	if c.IoCli != nil {
		c.IoCli.Close()
	}
}

func (c *ProxyClient) toStatErrno(err error, code int) error {
	if err != nil {
		return err
	}

	if code == 0 {
		return nil
	}

	return Errno(code)
}

//request to proxy
func (c *ProxyClient) tryReqProxy(api_uri string, params interface{}, result ProxyResp, log *ClientLogger) error {
	var err error

	for i := 0; i <= c.RetryCfg.RetryTimes; i++ {
		bgTime := c.St.BeginStat()

		url := c.IoCli.Hosts[0] + api_uri
		err = c.IoCli.DoPostWithJson(url, params, result, log)

		c.St.EndStat(api_uri, c.toStatErrno(err, result.GetCode()), bgTime, 1)

		if !retry(err, result.GetCode(), log) {
			return err
		} /**/

		log.Debug("tryReqProxy",
			zap.Int("times", i+1),
			zap.Any("err", err),
			zap.Int("Code", result.GetCode()),
			zap.String("url", api_uri))

		time.Sleep(time.Duration(i)*c.RetryCfg.RetryFactor + c.RetryCfg.RetryGap)
	}

	return err
}

func retry(err error, code int, log *ClientLogger) bool {
	if err == nil {
		return false
	}

	if strings.Contains(err.Error(), "connection refused") {
		return true
	}

	if strings.Contains(err.Error(), "use of closed network connection") {
		return true
	}

	if strings.Contains(err.Error(), "EOF") {
		return true
	}

	if strings.Contains(err.Error(), "connection reset by peer") {
		return true
	}

	urlErr, ok := err.(*url.Error)
	if ok && (urlErr.Timeout() || urlErr.Temporary()) {
		return true
	}

	return false
}

//delete a file
func (c *ProxyClient) Unlink(path string, log *ClientLogger) (resp UnlinkResponse, err error) {
	req := UnlinkRequest{
		ProxyRequest: ProxyRequest{
			Path: path,
		},
	}
	log.Debug("ProxyClient Unlink run", zap.Any("req", req))

	err = c.tryReqProxy(UrlUnlink, req, &resp, log)
	return resp, err
}

func (c *ProxyClient) Link(path, dstpath string, log *ClientLogger) (resp LinkResponse, err error) {
	req := LinkRequest{
		ProxyRequest: ProxyRequest{
			Path: path,
		},
		Dstpath: dstpath,
	}
	log.Debug("ProxyClient Link run", zap.Any("req", req))

	err = c.tryReqProxy(UrlLink, req, &resp, log)
	return resp, err
}

func (c *ProxyClient) SymLink(path string, dstpath string, log *ClientLogger) (resp SymlinkResponse, err error) {
	req := SymlinkRequest{
		ProxyRequest: ProxyRequest{
			Path: path,
		},
		Dstpath: dstpath,
	}
	log.Debug("ProxyClient SymLink run", zap.Any("req", req))

	err = c.tryReqProxy(UrlSymLink, req, &resp, log)
	return resp, err
}

func (c *ProxyClient) MkDir(path string, mode uint32, log *ClientLogger) (resp MkdirResponse, err error) {
	req := MkdirRequest{
		ProxyRequest: ProxyRequest{
			Path: path,
		},
		Mode: mode,
	}
	log.Debug("ProxyClient MkDir run", zap.Any("req", req))

	err = c.tryReqProxy(UrlMkdir, req, &resp, log)
	return resp, err
}

func (c *ProxyClient) ReadDir(path string, log *ClientLogger) (resp ReaddirResponse, err error) {
	req := ReaddirRequest{
		ProxyRequest: ProxyRequest{
			Path: path,
		},
	}
	log.Debug("ProxyClient ReadDir run", zap.Any("req", req))

	err = c.tryReqProxy(UrlReaddir, req, &resp, log)
	return
}

func (c *ProxyClient) ReadDirEx(path, startKey string, count uint16, log *ClientLogger) (resp ReaddirExResponse, err error) {
	req := ReaddirExRequest{
		ProxyRequest: ProxyRequest{
			Path: path,
		},
		Count:    count,
		StartKey: startKey,
	}
	log.Debug("ProxyClient ReadDirEx run", zap.Any("req", req))

	err = c.tryReqProxy(UrlReaddirEx, req, &resp, log)
	return
}

func (c *ProxyClient) RmDir(path string, log *ClientLogger) (resp RmdirResponse, err error) {
	req := RmdirRequest{
		ProxyRequest: ProxyRequest{
			Path: path,
		},
	}
	log.Debug("ProxyClient RmDir run", zap.Any("req", req))

	err = c.tryReqProxy(UrlRmdir, req, &resp, log)
	return
}

func (c *ProxyClient) RmDirTree(path string, log *ClientLogger) (resp RmdirTreeResponse, err error) {
	req := RmdirTreeRequest{
		ProxyRequest: ProxyRequest{
			Path: path,
		},
	}
	log.Debug("ProxyClient RmDirTree run", zap.Any("req", req))

	err = c.tryReqProxy(UrlRmdirtree, req, &resp, log)
	return
}

// Create a new file or open an existing file
func (c *ProxyClient) Open(path string, flag uint32, mode uint32, log *ClientLogger) (resp OpenResponse, err error) {
	req := OpenRequest{
		ProxyRequest: ProxyRequest{
			Path: path,
		},
		Openflag: flag,
		Mode:     mode,
	}
	log.Debug("ProxyClient Open run", zap.Any("req", req))

	err = c.tryReqProxy(UrlOpen, req, &resp, log)
	return resp, err
}

// 查询文件信息
func (c *ProxyClient) Read(id, offset, length uint64, log *ClientLogger) (resp ReadResponse, err error) {
	log.Debug("ProxyClient Read run", zap.Uint64("id", id))
	req := ReadRequest{
		Id:     id,
		Offset: offset,
		Length: length,
	}

	err = c.tryReqProxy(UrlRead, req, &resp, log)
	return resp, err
}

func (c *ProxyClient) Write(id uint64, buffer []byte, offset uint64, log *ClientLogger) (resp WriteResponse, err error) {
	log.Debug("ProxyClient Write run", zap.Uint64("id", id))
	req := WriteRequest{
		Id:     id,
		Offset: offset,
		Length: uint64(len(buffer)),
		Data:   buffer,
	}

	err = c.tryReqProxy(UrlWrite, req, &resp, log)
	return resp, err
}

func (c *ProxyClient) Truncate(path string, length uint64, log *ClientLogger) (resp TruncateResponse, err error) {
	req := TruncateRequest{
		ProxyRequest: ProxyRequest{
			Path: path,
		},
		Length: length,
	}
	log.Debug("ProxyClient Truncate run", zap.Any("req", req))

	err = c.tryReqProxy(UrlTruncate, req, &resp, log)
	return resp, err
}

func (c *ProxyClient) SetXattr(path string, flag SetXattrFlag, name string, value []byte, log *ClientLogger) (resp SetxattrResponse, err error) {
	req := SetxattrRequest{
		ProxyRequest: ProxyRequest{
			Path: path,
		},
		Flag: flag,
		Name: name,
		Data: value,
	}
	log.Debug("ProxyClient SetXattr run", zap.Any("req", req))

	err = c.tryReqProxy(UrlSetxattr, req, &resp, log)
	return resp, err
}

func (c *ProxyClient) GetXattr(path, name string, log *ClientLogger) (resp GetxattrResponse, err error) {
	req := GetxattrRequest{
		ProxyRequest: ProxyRequest{
			Path: path,
		},
		Name: name,
	}
	log.Debug("ProxyClient GetXattr run", zap.Any("req", req))

	err = c.tryReqProxy(UrlGetxattr, req, &resp, log)
	return resp, err
}

func (c *ProxyClient) ListXAttr(path string, log *ClientLogger) (resp ListxattrResponse, err error) {
	req := GetxattrRequest{
		ProxyRequest: ProxyRequest{
			Path: path,
		},
	}
	log.Debug("ProxyClient ListXAttr run", zap.Any("req", req))

	err = c.tryReqProxy(UrlListxattr, req, &resp, log)
	return resp, err
}

func (c *ProxyClient) RmXAttr(path, name string, log *ClientLogger) (resp RemovexattrResponse, err error) {
	req := RemovexattrRequest{
		ProxyRequest: ProxyRequest{
			Path: path,
		},
		Name: name,
	}
	log.Debug("ProxyClient RmXAttr run", zap.Any("req", req))

	err = c.tryReqProxy(UrlRemovexattr, req, &resp, log)
	return resp, err
}

func (c *ProxyClient) Chmod(path string, mode uint32, log *ClientLogger) (resp SetAttrResponse, err error) {
	req := SetAttrRequest{
		ProxyRequest: ProxyRequest{
			Path: path,
		},
		Flag: SetAclFlagChangeMode,
		Mode: mode,
	}
	log.Debug("ProxyClient Chmod run", zap.Any("req", req))

	err = c.tryReqProxy(UrlSetattr, req, &resp, log)
	return resp, err
}

func (c *ProxyClient) Chown(path string, uid, gid uint32, log *ClientLogger) (resp SetAttrResponse, err error) {
	req := SetAttrRequest{
		ProxyRequest: ProxyRequest{
			Path: path,
		},
		Flag: SetAclFlagChangeUid | SetAclFlagChangeGid,
		Uid:  uid,
		Gid:  gid,
	}
	log.Debug("ProxyClient Chown run", zap.Any("req", req))

	err = c.tryReqProxy(UrlSetattr, req, &resp, log)
	return resp, err
}

func (c *ProxyClient) ChownEx(path string, owner, group string, log *ClientLogger) (resp SetAttrResponse, err error) {
	req := SetAttrRequest{
		ProxyRequest: ProxyRequest{
			Path: path,
		},
		Flag:  SetAclFlagChangeOwner | SetAclFlagChangeGroup,
		Owner: owner,
		Group: group,
	}
	log.Debug("ProxyClient ChownEx run", zap.Any("req", req))

	err = c.tryReqProxy(UrlSetattr, req, &resp, log)
	return resp, err
}

func (c *ProxyClient) Rename(path, dstPath string, log *ClientLogger) (resp RenameResponse, err error) {
	req := RenameRequest{
		ProxyRequest: ProxyRequest{
			Path: path,
		},
		Dstpath: dstPath,
	}
	log.Debug("ProxyClient Rename run", zap.Any("req", req))

	err = c.tryReqProxy(UrlRename, req, &resp, log)
	return resp, err
}

// get meta info
func (c *ProxyClient) Stat(path string, log *ClientLogger) (resp StatResponse, err error) {
	req := StatRequest{
		ProxyRequest: ProxyRequest{
			Path: path,
		},
	}
	log.Debug("ProxyClient Stat run", zap.Any("req", req))

	err = c.tryReqProxy(UrlStat, req, &resp, log)
	return resp, err
}

// Obtain the meta information about a file system or directory tree
func (c *ProxyClient) StatFs(path string, log *ClientLogger) (resp StatfsResponse, err error) {
	req := StatfsRequest{
		ProxyRequest: ProxyRequest{
			Path: path,
		},
	}
	log.Debug("ProxyClient StatFs run", zap.Any("req", req))

	err = c.tryReqProxy(UrlStatfs, req, &resp, log)
	return resp, err
}

// update timestamps of dir entry
func (c *ProxyClient) Utime(path string, utime, atime uint64, log *ClientLogger) (resp SetAttrResponse, err error) {
	req := SetAttrRequest{
		ProxyRequest: ProxyRequest{
			Path: path,
		},
		Flag:  SetAclFlagChangeATime | SetAclFlagChangeMTime,
		Mtime: utime,
		Atime: atime,
	}
	log.Debug("ProxyClient Utime run", zap.Any("req", req))

	err = c.tryReqProxy(UrlSetattr, req, &resp, log)
	return resp, err
}

func (c *ProxyClient) Fsync(id uint64, log *ClientLogger) (resp FsyncResponse, err error) {
	req := FsyncRequest{
		Id: id,
	}
	log.Debug("ProxyClient Fsync run", zap.Any("req", req))

	err = c.tryReqProxy(UrlFsync, req, &resp, log)
	return resp, err
}
