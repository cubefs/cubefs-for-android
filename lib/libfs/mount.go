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
	"bytes"
	"encoding/json"
	"fmt"
	gohttp "net/http"
	"strings"
	"sync"
	"time"

	"github.com/cubefs/cubefs-for-android/lib/libfs/api"
	"github.com/cubefs/cubefs-for-android/lib/libfs/conf"
	"github.com/cubefs/cubefs-for-android/lib/libfs/consts"
	"github.com/cubefs/cubefs-for-android/lib/libfs/http"
	"github.com/cubefs/cubefs-for-android/lib/libfs/stat"
	. "github.com/cubefs/cubefs-for-android/lib/libfs/util"
	zap "github.com/cubefs/cubefs-for-android/lib/log"
	. "github.com/cubefs/cubefs-for-android/lib/proto"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	zapl "go.uber.org/zap"
)

type MountStu int

const (
	MountStuNew     MountStu = 0
	MountStuMounted MountStu = 1
	MountStuDel     MountStu = 2
)

type CfaMountInfo struct {
	Volume    string // "CFA"
	ClientId  string
	Path      string   // path prefix
	MountStu  MountStu // mount status
	User      *conf.UserInfo
	MountMode MountMode // Access permission of the mount path
	Cfg       conf.Config
	sync.Mutex

	log         *ClientLogger
	proxyClient *api.ProxyClient

	// file operation related
	OpenFileMax int    // max open files
	LsSize      uint16 // max items for a single 'ls' op, default 2000
	fileCache   *FileCache
	dentryCache *DentryCache
	rwCache     *RWCache
	wg          sync.WaitGroup
	st          *stat.Statistic
	stop        chan struct{}
}

func NewMount(cfg conf.Config) (*CfaMountInfo, error) {
	var cfaMntInfo *CfaMountInfo

	logCfg := cfg.BuildLogCfg()
	logCfg.LogFile = fmt.Sprintf("%s.log", logCfg.LogFile)

	// logger
	log := NewLogger(logCfg)
	log.Debug("log cfg", zap.Any("log", *logCfg))
	// Check path.
	path := cfg[consts.CfgPath]
	len := len(path)
	if (len == 0) || (path[0] != '/') || (path[len-1] == '/' && len > 1) {
		log.Info("path is not legal", zap.String("path", path))
		return nil, E_POSIX_EPERM
	}

	user, err := getUserInfo(cfg, log)
	if err != nil {
		log.Error("get user info failed", zap.Any("err", err))
		return nil, E_POSIX_EINVAL
	}

	// mount configs
	openFileMax := cfg.GetIntValue(consts.CfgOpenFileMax, 100000, log)

	// dentry Cache.
	dentryCacheSize := cfg.GetIntValue(consts.CfgDentryCacheSize, 0, log)
	dentryCacheExpireMs := cfg.GetIntValue(consts.CfgDentryCacheExpireMs, 500, log)

	// statistics log
	statLogPath := cfg.GetStringValue(consts.CfgStatLogPath, "./client_stat", log)
	statLogPath = fmt.Sprintf("%s", statLogPath)

	statLogNum := cfg.GetIntValue(consts.CfgStatLogNum, 10, log)
	statLogSize := int64(cfg.GetIntValue(consts.CfgStatLogMaxSize, 20000000, log))
	timeOutUs := [3]uint32{100000, 500000, 1000000}

	// init the mount object
	cfaMntInfo = &CfaMountInfo{
		ClientId:    genClientId(getExportPath(cfg)),
		Volume:      "CFA",
		Path:        cfg[consts.CfgPath],
		MountStu:    MountStuNew,
		User:        user,
		MountMode:   0,
		Cfg:         cfg,
		Mutex:       sync.Mutex{},
		log:         log,
		OpenFileMax: openFileMax,
		LsSize:      uint16(cfg.GetIntValue(consts.CfgLsSize, 2000, log)),
		fileCache:   NewFileCache(openFileMax),
		dentryCache: NewDentryCache(dentryCacheSize, dentryCacheExpireMs),
		rwCache:     nil,
		wg:          sync.WaitGroup{},

		st:   stat.NewStatistic(statLogPath, statLogSize, statLogNum, timeOutUs, true),
		stop: make(chan struct{}),
	}

	// init rw cache.
	rwCacheSize := cfg.GetIntValue(consts.CfgRwCacheCacheSize, 0, log)
	if rwCacheSize > 0 {
		ExpireTime := cfg.GetIntValue(consts.CfgRwCacheExpireMs, 500, log)
		SyncRoutine := cfg.GetIntValue(consts.CfgRwCacheSyncRoutine, 64, log)
		PrefetchRoutine := cfg.GetIntValue(consts.CfgRwCachePrefetchRoutine, 64, log)
		PrefetchTimes := cfg.GetIntValue(consts.CfgRwCachePrefetchTimes, 8, log)
		cfaMntInfo.rwCache = NewRWCache(cfaMntInfo, rwCacheSize, ExpireTime, SyncRoutine, PrefetchRoutine, PrefetchTimes)
	}

	cfaMntInfo.setProxyCli()

	ioCli := cfaMntInfo.proxyClient.GetIoClient()
	ioCli.UpdateHosts(cfg[consts.CfgProxyHosts])

	log.Info("NewMount success", zap.Any("MountInfo", &cfaMntInfo))
	return cfaMntInfo, nil
}

func getUserInfo(cfg conf.Config, log *ClientLogger) (u *conf.UserInfo, err error) {
	user := cfg.BuildUserInfo("UidNotSet", "TokenNotSet")

	pullType := cfg[consts.CfgThirdPartyUidPullType]
	if pullType == consts.UidPullTypeByUrl {
		// obtain third party's user info by url of account server
		userTmp, err := PullThirdPartyUser(cfg, log)
		if err != nil {
			log.Error("pull user info failed", zap.Any("err", err))
			return nil, err
		}
		user = userTmp
		log.Info("pulled user info by URL", zap.Any("userInfo", user))
		return user, nil
	} else if pullType == consts.UidPullTypeByConf {
		// obtain third party's user info by config file
		uid, uidInConf := cfg[consts.CfgUserId]
		token, tokenInConf := cfg[consts.CfgUserToken]
		if !uidInConf || !tokenInConf {
			errMsg := "configured to get user info by conf, but not " + consts.CfgUserId + " or " + consts.CfgUserToken + " in conf file"
			log.Error(errMsg, zap.Any(consts.CfgUserId, uid), zap.Any(consts.CfgUserToken, token))
			return nil, errors.New(errMsg)
		}

		user = cfg.BuildUserInfo(uid, token)
		log.Info("get user info by config", zap.Any("userInfo", user))
		return user, nil
	}

	log.Info("user not set", zap.Any("userInfo", user))
	return user, nil
}

//TODO: pull user info and update mem periodically
func PullThirdPartyUser(cfg conf.Config, log *ClientLogger) (u *conf.UserInfo, err error) {
	reqBody := &FetchRequest{}

	msg, err := json.Marshal(reqBody)
	if err != nil {
		log.Error("request body is not legal",
			zap.Any("reqBody", reqBody),
			zap.Any("err", err))
		return nil, err
	}

	req, err := gohttp.NewRequest("POST", cfg[consts.CfgThirdPartyUidPullUrl], bytes.NewReader(msg))
	if err != nil {
		log.Error("gen request fail", zap.Any("err", err))
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(ReqidKey, uuid.New().String())
	req.Header.Set(ReqClientTag, cfg[consts.CfgClientTag])
	req.Header.Set(ReqAccessKey, cfg[consts.CfgThirdPartyUidPullAccessKey])
	log.Debug("PullThirdPartyUser http req",
		      zap.String("URL", cfg[consts.CfgThirdPartyUidPullUrl]),
		      zap.Any("header", req.Header),
		      zap.Any("body", reqBody))

	client := &gohttp.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Error("PullThirdPartyUser failed send failed", zap.Any("err", err))
		return nil, err
	}
	defer resp.Body.Close()


	respBody := &FetchResponse{}
	err = json.NewDecoder(resp.Body).Decode(respBody)
	if err != nil || respBody == nil {
		log.Error("PullThirdPartyUser Decode resp failed", zap.String("err", err.Error()))
		return nil, err
	}
	if respBody.Code != 0 {
		log.Error("PullThirdPartyUser resp code is failed",
			zap.Int("code", respBody.Code),
			zap.String("msg", respBody.Msg))
		return nil, errors.New("account server return failed code")
	}
	if respBody.Data == nil {
		log.Error("PullThirdPartyUser resp data is nil",
			zap.Int("code", respBody.Code),
			zap.String("msg", respBody.Msg))
		return nil, errors.New("account server return data is nil")
	}

	log.Debug("PullThirdPartyUser fetched", zap.Any("rsp data", respBody.Data))

	return &conf.UserInfo{
		Uid:   respBody.Data.Uid,
		Token: respBody.Data.Token,
	}, nil
}

// flag > 0 turns up the level
func (mount *CfaMountInfo) SetLogLevel(flag int) {
	level := mount.log.Level()
	if flag < 0 {
		if level > zapl.DebugLevel {
			mount.log.SetLevel(level - 1)
		}
		return
	}

	if level < zapl.PanicLevel {
		mount.log.SetLevel(level + 1)
	}
}

func (mount *CfaMountInfo) setProxyCli() {
	httpCfg := mount.Cfg.BuildHttpCfg(mount.log)

	// http client for communicating to proxy
	ioCli := http.NewHttpClient(httpCfg, mount.User, mount.ClientId, mount.log, &mount.Cfg)

	mount.proxyClient = api.NewProxyClient(ioCli, mount.Cfg.BuildRetryCfg(mount.log), mount.st)
}

func (mount *CfaMountInfo) GetProxyClient() *api.ProxyClient {
	return mount.proxyClient
}

func (mount *CfaMountInfo) GetLogger() *zap.Logger {
	return mount.log.Logger
}

func (mount *CfaMountInfo) DelMount() {
	mount.Lock()
	defer mount.Unlock()

	mount.log.Debug("start del cfa mount")

	if mount.rwCache != nil {
		mount.rwCache.CloseAll()
	}

	mount.MountStu = MountStuDel
	mount.proxyClient.Close()
	close(mount.stop)

	mount.log.Close()

	mount.wg.Wait()
}

func (mount *CfaMountInfo) Mount() error {

	log := CloneLogger(mount.log)

	if mount.MountStu != MountStuNew {
		return E_POSIX_EPERM
	}

	mount.Lock()
	defer mount.Unlock()

	if mount.MountStu == MountStuMounted {
		return E_POSIX_EPERM
	}

	mount.MountMode = MountModeRead | MountModeWrite | MountModeDel

	// mount compete.
	mount.MountStu = MountStuMounted

	_, err := mount.Stat(mount.Path)
	if err != nil {
		log.Error("mount stat fail", zap.String("path", mount.Path), zap.Error(err))
		return err
	}

	// stat log
	mount.wg.Add(1)
	go mount.StatTo()

	log.Info("Mount success",
		zap.Any("MountInfo", mount))

	return nil
}

func getExportPath(cfg conf.Config) string {
	exportPath := cfg[consts.CfgPath]
	if cfg[consts.ExportPath] != "" {
		exportPath = cfg[consts.ExportPath]
	}
	return exportPath
}

func (mount *CfaMountInfo) checkMount(path string, perm MountMode, log *ClientLogger) error {
	// check if mounted
	if mount.MountStu != MountStuMounted {
		return E_POSIX_EPERM
	}

	// check permission
	if perm == 0 || (perm&mount.MountMode) != perm {
		log.Error("checkMount fail, Invalid Perm",
			zap.String("MountPath", mount.Path),
			zap.String("Path", path),
			zap.Uint32("MountMode", uint32(mount.MountMode)),
			zap.Uint32("Perm", uint32(perm)))
		return E_POSIX_EPERM
	}

	// check path
	l := len(path)
	if (l == 0) || (path[0] != '/') || (path[l-1] == '/' && l > 1) {
		log.Error("checkMount fail, Invalid Path",
			zap.String("MountPath", mount.Path),
			zap.String("Path", path))
		return E_POSIX_EPERM
	}

	// if req path under mount path
	if !strings.HasPrefix(path, mount.Path) {
		log.Error("checkMount fail, Invalid Path",
			zap.String("MountPath", mount.Path), zap.String("Path", path))
		return E_POSIX_EACCES
	}

	return nil
}

// generate unique id for client instance
func genClientId(mntPath string) string {
	ip := GetIp()
	// format: "ip_uuid_pid"
	return ip + "_" + mntPath
}

func toErr(err error, code int) error {
	err = TryToErrno(err)
	if errno, ok := err.(Errno); ok {
		return errno
	}

	if err == nil && code == 0 {
		return nil
	}

	switch code {
	case 1001:
		return E_POSIX_EAGAIN
	case 1002:
		return E_POSIX_EEXIST
	case 1003:
		return E_POSIX_ENOENT
	case 1004:
		return E_POSIX_ENOTSUP
	case 1005:
		return E_POSIX_EIO
	case 1006:
		return E_POSIX_ENOTEMPTY
	case 1007:
		return E_POSIX_EPERM
	default:
		return E_POSIX_EIO
	}
}

// do the statistics log job
func (mount *CfaMountInfo) StatTo() {
	defer mount.wg.Done()

	lastStatTime := time.Now()
	statGap := time.Duration(mount.Cfg.GetIntValue(consts.CfgStatGap, 60, mount.log))

	// don't stat.
	if statGap == 0 {
		mount.st.CloseStat()
		return
	}

	// break if unmounted
	log := CloneLogger(mount.log)
	for mount.MountStu == MountStuMounted {

		if time.Since(lastStatTime) < (statGap * time.Second) {
			time.Sleep(1 * time.Second)
			continue
		}

		err := mount.st.WriteStat()
		if err != nil {
			log.Error("statLog", zap.Any("err", err))
		}

		mount.st.ClearStat()
		lastStatTime = time.Now()
	}
}
